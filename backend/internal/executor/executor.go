package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	"webtestflow/backend/internal/models"
	"webtestflow/backend/pkg/chrome"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/device"
	"github.com/chromedp/cdproto/target"
)

type TestExecutor struct {
	ctx         context.Context
	cancel      context.CancelFunc
	device      models.Device
	maxWorkers  int
	workQueue   chan ExecutionJob
	wg          sync.WaitGroup
	mutex       sync.RWMutex
	running     map[uint]bool
	cancels     map[uint]context.CancelFunc // Store cancel functions for each execution
	completions map[uint]chan bool          // Store completion channels for each execution
	// 添加全局Chrome上下文管理
	globalAllocCtx    context.Context
	globalAllocCancel context.CancelFunc
}

type ExecutionJob struct {
	Execution    *models.TestExecution
	TestCase     *models.TestCase
	IsVisual     bool
	ResultChan   chan ExecutionResult
	CompleteChan chan bool // Added for proper cleanup coordination
}

type ExecutionResult struct {
	Success      bool
	ErrorMessage string
	Screenshots  []string
	Logs         []ExecutionLog
	Metrics      *models.PerformanceMetric
}

type ExecutionLog struct {
	Timestamp   time.Time `json:"timestamp"`
	Level       string    `json:"level"`
	Message     string    `json:"message"`
	StepIndex   int       `json:"step_index"`
	StepType    string    `json:"step_type,omitempty"`
	StepStatus  string    `json:"step_status,omitempty"` // success, failed, running
	Selector    string    `json:"selector,omitempty"`
	Value       string    `json:"value,omitempty"`
	Screenshot  string    `json:"screenshot,omitempty"`
	Duration    int64     `json:"duration,omitempty"` // milliseconds
	ErrorDetail string    `json:"error_detail,omitempty"`
}

var GlobalExecutor *TestExecutor
var chromeMutex sync.Mutex // Global mutex to serialize Chrome instance creation

func InitExecutor(maxWorkers int) {
	GlobalExecutor = &TestExecutor{
		maxWorkers:  maxWorkers,
		workQueue:   make(chan ExecutionJob, maxWorkers*2),
		running:     make(map[uint]bool),
		cancels:     make(map[uint]context.CancelFunc),
		completions: make(map[uint]chan bool),
	}

	// Start worker goroutines
	for i := 0; i < maxWorkers; i++ {
		go GlobalExecutor.worker()
	}

	log.Printf("Test executor initialized with %d workers", maxWorkers)
}

func (te *TestExecutor) worker() {
	for job := range te.workQueue {
		// Execute the test case
		result := te.executeTestCase(job.Execution.ID, job.TestCase, job.IsVisual)

		// Send result to handler FIRST
		job.ResultChan <- result

		// Log that result was sent
		log.Printf("✅ Worker sent execution result for %d (success=%v) to handler", job.Execution.ID, result.Success)

		// Wait for handler to confirm database update is complete
		select {
		case <-job.CompleteChan:
			log.Printf("✅ Handler confirmed database update for execution %d", job.Execution.ID)
		case <-time.After(10 * time.Second):
			log.Printf("⚠️ Timeout waiting for handler confirmation for execution %d, proceeding with cleanup", job.Execution.ID)
		}

		// Now mark execution as completed in internal state
		te.mutex.Lock()
		delete(te.running, job.Execution.ID)
		delete(te.cancels, job.Execution.ID)
		delete(te.completions, job.Execution.ID)
		te.mutex.Unlock()

		log.Printf("✅ Worker cleaned up internal state for execution %d", job.Execution.ID)
	}
}

func (te *TestExecutor) ExecuteTestCase(execution *models.TestExecution, testCase *models.TestCase) <-chan ExecutionResult {
	return te.ExecuteTestCaseWithOptions(execution, testCase, false)
}

func (te *TestExecutor) ExecuteTestCaseWithOptions(execution *models.TestExecution, testCase *models.TestCase, isVisual bool) <-chan ExecutionResult {
	te.mutex.Lock()
	te.running[execution.ID] = true
	// Create completion channel for this execution
	completeChan := make(chan bool, 1)
	te.completions[execution.ID] = completeChan
	te.mutex.Unlock()

	resultChan := make(chan ExecutionResult, 1)
	job := ExecutionJob{
		Execution:    execution,
		TestCase:     testCase,
		IsVisual:     isVisual,
		ResultChan:   resultChan,
		CompleteChan: completeChan,
	}

	te.workQueue <- job
	return resultChan
}

// ExecuteTestCaseDirectly executes a test case directly without using the worker queue
// This method is safer for sequential execution and avoids ChromeDP concurrency issues
func (te *TestExecutor) ExecuteTestCaseDirectly(execution *models.TestExecution, testCase *models.TestCase, isVisual bool) ExecutionResult {
	te.mutex.Lock()
	te.running[execution.ID] = true
	te.mutex.Unlock()

	defer func() {
		te.mutex.Lock()
		delete(te.running, execution.ID)
		te.mutex.Unlock()
	}()

	// Add panic recovery to prevent service crash
	var result ExecutionResult
	var panicRecovered bool
	
	defer func() {
		if r := recover(); r != nil {
			panicRecovered = true
			log.Printf("🚨 PANIC recovered in ExecuteTestCaseDirectly for execution %d: %v", execution.ID, r)
			
			// Force cleanup of any stuck Chrome processes
			go func() {
				time.Sleep(2 * time.Second)
				te.forceKillChromeProcesses()
			}()
			
			result = ExecutionResult{
				Success:      false,
				ErrorMessage: fmt.Sprintf("ChromeDP panic recovered: %v", r),
				Screenshots:  []string{},
				Logs: []ExecutionLog{
					{
						Timestamp: time.Now(),
						Level:     "error",
						Message:   fmt.Sprintf("Execution failed due to ChromeDP panic: %v", r),
						StepIndex: -1,
					},
				},
				Metrics: nil,
			}
		}
	}()

	// Add execution isolation to prevent Chrome instance conflicts
	// 为每个执行添加短暂的隔离延迟，避免Chrome实例冲突
	time.Sleep(500 * time.Millisecond)

	// Execute directly without worker queue
	result = te.executeTestCase(execution.ID, testCase, isVisual)

	if !panicRecovered {
		log.Printf("✅ Direct execution completed for %d (success=%v)", execution.ID, result.Success)
	}
	return result
}

func (te *TestExecutor) IsRunning(executionID uint) bool {
	te.mutex.RLock()
	defer te.mutex.RUnlock()
	return te.running[executionID]
}

func (te *TestExecutor) GetRunningCount() int {
	te.mutex.RLock()
	defer te.mutex.RUnlock()
	return len(te.running)
}

// NotifyExecutionComplete signals the executor that the handler has finished updating the database
func (te *TestExecutor) NotifyExecutionComplete(executionID uint) {
	te.mutex.RLock()
	completeChan, exists := te.completions[executionID]
	te.mutex.RUnlock()

	if exists {
		select {
		case completeChan <- true:
			log.Printf("✅ Notified executor that database update is complete for execution %d", executionID)
		default:
			// Channel already closed or worker has timed out, no need to log
		}
	}
}

func (te *TestExecutor) executeTestCase(executionID uint, testCase *models.TestCase, isVisual bool) ExecutionResult {
	result := ExecutionResult{
		Screenshots: make([]string, 0),
		Logs:        make([]ExecutionLog, 0),
	}

	// Add panic recovery to prevent ChromeDP crashes from killing the service
	defer func() {
		if r := recover(); r != nil {
			log.Printf("🚨 PANIC recovered in executeTestCase for execution %d: %v", executionID, r)
			result.Success = false
			result.ErrorMessage = fmt.Sprintf("ChromeDP execution panic: %v", r)
			result.addLog("error", fmt.Sprintf("Execution panic recovered: %v", r), -1)
		}
	}()

	// Parse test steps
	steps, err := testCase.GetSteps()
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to parse test steps: %v", err)
		return result
	}

	// Check if Chrome is available
	result.addLog("info", fmt.Sprintf("Current working directory: %s", getCurrentDir()), -1)

	chromePath := chrome.GetChromePath()
	result.addLog("info", fmt.Sprintf("GetChromePath() returned: '%s'", chromePath), -1)

	if chromePath == "" {
		// Try flatpak Chrome
		chromePath = chrome.GetFlatpakChromePath()
		result.addLog("info", fmt.Sprintf("GetFlatpakChromePath() returned: '%s'", chromePath), -1)

		if chromePath == "" {
			result.Success = false
			result.ErrorMessage = "Chrome browser not found. Please install Google Chrome or Chromium"
			result.addLog("error", "Chrome not found in system", -1)
			return result
		}
		result.addLog("info", "Using Flatpak Chrome", -1)
	}

	result.addLog("info", fmt.Sprintf("Using Chrome path: %s", chromePath), -1)

	// Test if Chrome executable exists and is accessible
	if _, err := os.Stat(chromePath); err != nil {
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("Chrome executable not accessible: %v", err)
		result.addLog("error", fmt.Sprintf("Chrome path not accessible: %v", err), -1)
		return result
	}

	// ChromeDP v0.9.2有已知的"close of closed channel"bug
	// 使用最保守的方式避免触发这个bug
	log.Printf("🚀 Creating Chrome context for execution %d with path: %s", executionID, chromePath)

	// 使用专用的Chrome管理器避免ChromeDP v0.9.2的channel竞争问题
	targetURL := testCase.Environment.BaseURL
	
	// 对于可视化执行，检查是否有已存在的Chrome实例
	var port int
	existingPort := chrome.GlobalChromeManager.GetExistingPort(executionID, isVisual)
	
	if isVisual && existingPort > 0 {
		// 尝试复用已存在的Chrome实例
		result.addLog("info", fmt.Sprintf("🔄 Attempting to reuse existing Chrome instance for execution %d on port %d", executionID, existingPort), -1)
		port = existingPort
		
		// 验证连接是否可用 - 如果不可用，将启动新实例
		debugURL := fmt.Sprintf("http://localhost:%d/json/version", port)
		client := &http.Client{Timeout: 2 * time.Second}
		resp, connErr := client.Get(debugURL)
		if connErr != nil {
			result.addLog("warn", fmt.Sprintf("⚠️ Existing Chrome instance not responsive: %v, starting new instance", connErr), -1)
			// 清理失效的Chrome实例引用
			chrome.GlobalChromeManager.StopVisualInstance()
			existingPort = 0 // 重置，强制启动新实例
		} else {
			resp.Body.Close()
			result.addLog("info", fmt.Sprintf("✅ Successfully connected to existing Chrome instance on port %d", port), -1)
		}
	}
	
	if !isVisual || existingPort == 0 {
		// 启动新的Chrome实例，直接加载目标URL避免空白页
		result.addLog("info", fmt.Sprintf("🚀 Starting Chrome with target URL: %s", targetURL), -1)
		
		port, err = chrome.GlobalChromeManager.StartChromeWithURL(executionID, isVisual, targetURL)
		if err != nil {
			result.Success = false
			result.ErrorMessage = fmt.Sprintf("Failed to start Chrome: %v", err)
			result.addLog("error", fmt.Sprintf("❌ Chrome startup failed: %v", err), -1)
			return result
		}
		result.addLog("info", fmt.Sprintf("✅ Chrome started successfully on port %d with target page loaded", port), -1)
	}
	
	// 确保Chrome进程在函数退出时被完全关闭
	var chromeCleanup func()
	var chromeContext context.Context
	defer func() {
		result.addLog("info", fmt.Sprintf("🧹 Starting Chrome cleanup for execution %d", executionID), -1)
		
		// Step 1: Try to gracefully close browser tabs first (for visual executions)
		if chromeContext != nil && isVisual {
			result.addLog("info", "🔄 Attempting graceful browser close...", -1)
			te.closeBrowser(chromeContext)
		}
		
		// Step 2: Close ChromeDP contexts
		if chromeCleanup != nil {
			result.addLog("info", "🔄 Closing ChromeDP contexts...", -1)
			chromeCleanup()
		}
		
		// Step 3: Stop Chrome process (gracefully first, then force if needed)
		result.addLog("info", fmt.Sprintf("🛑 Stopping Chrome process for execution %d", executionID), -1)
		chrome.GlobalChromeManager.StopChrome(executionID)
		result.addLog("info", fmt.Sprintf("✅ Chrome cleanup completed for execution %d", executionID), -1)
	}()
	
	// Chrome启动时已经包含动态就绪检测，无需额外等待
	result.addLog("info", "✅ Chrome is ready for connection", -1)
	
	// 连接到已运行的Chrome实例
	debugURL := fmt.Sprintf("http://localhost:%d", port)
	result.addLog("info", fmt.Sprintf("🔗 Connecting to Chrome at %s", debugURL), -1)
	
	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	result.addLog("info", "📋 Created main context with timeout", -1)
	
	// 连接到已运行的Chrome实例
	result.addLog("info", "🔌 Creating remote allocator connection...", -1)
	allocCtx, cancel2 := chromedp.NewRemoteAllocator(ctx, debugURL)
	defer cancel2()
	result.addLog("info", "✅ Remote allocator created successfully", -1)
	
	// 获取Chrome中已存在的标签页，连接到第一个而不是创建新的
	result.addLog("info", "📄 Looking for existing tabs to connect to...", -1)
	
	// 等待Chrome完全准备就绪
	time.Sleep(200 * time.Millisecond)
	
	// 使用HTTP直接获取标签页列表（更可靠的方法）
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%d/json", port))
	if err != nil {
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("Failed to get Chrome tabs list: %v", err)
		result.addLog("error", fmt.Sprintf("❌ Failed to get tabs: %v", err), -1)
		return result
	}
	defer resp.Body.Close()
	
	// 解析标签页列表
	var tabs []struct {
		ID             string `json:"id"`
		Type           string `json:"type"`
		URL            string `json:"url"`
		Title          string `json:"title"`
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&tabs); err != nil {
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("Failed to parse Chrome tabs: %v", err)
		result.addLog("error", fmt.Sprintf("❌ Failed to parse tabs: %v", err), -1)
		return result
	}
	
	// 查找第一个页面类型的标签页
	var targetTab *struct {
		ID             string `json:"id"`
		Type           string `json:"type"`
		URL            string `json:"url"`
		Title          string `json:"title"`
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	
	for i := range tabs {
		if tabs[i].Type == "page" {
			targetTab = &tabs[i]
			result.addLog("info", fmt.Sprintf("🎯 Found existing tab: %s (URL: %s, Title: %s)", targetTab.ID, targetTab.URL, targetTab.Title), -1)
			break
		}
	}
	
	if targetTab == nil {
		result.Success = false
		result.ErrorMessage = "No existing page tab found to connect to"
		result.addLog("error", "❌ No page tab found", -1)
		return result
	}
	
	result.addLog("info", fmt.Sprintf("📊 Total tabs found: %d, connecting to first page tab", len(tabs)), -1)
	
	// 连接到指定的已存在标签页
	ctx, cancel3 := chromedp.NewContext(allocCtx,
		chromedp.WithTargetID(target.ID(targetTab.ID)), // 连接到指定标签页
		chromedp.WithLogf(func(string, ...interface{}) {}), // 禁用ChromeDP的debug日志
	)
	
	// 保存Chrome上下文以便后续清理使用
	chromeContext = ctx
	
	// 测试连接是否成功 - 尝试获取当前页面标题
	var pageTitle string
	testErr := chromedp.Run(ctx, chromedp.Title(&pageTitle))
	if testErr != nil {
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("Failed to connect to Chrome tab: %v", testErr)
		result.addLog("error", fmt.Sprintf("❌ Chrome connection test failed: %v", testErr), -1)
		return result
	}
	result.addLog("info", fmt.Sprintf("✅ Successfully connected to existing tab (title: '%s')", pageTitle), -1)
	
	// 设置清理函数，确保上下文在进程关闭前被关闭
	chromeCleanup = func() {
		if cancel3 != nil {
			cancel3()
		}
		if cancel2 != nil {
			cancel2()
		}
		if cancel != nil {
			cancel()
		}
	}
	
	result.addLog("info", "✅ Execution context created", -1)

	startTime := time.Now()

	// 设置设备模拟
	result.addLog("info", fmt.Sprintf("📱 Configuring device emulation: %s (%dx%d)", testCase.Device.Name, testCase.Device.Width, testCase.Device.Height), -1)
	
	// Enable device emulation with mobile parameters
	deviceInfo := device.Info{
		Name:      testCase.Device.Name,
		UserAgent: testCase.Device.UserAgent,
		Width:     int64(testCase.Device.Width),
		Height:    int64(testCase.Device.Height),
		Scale:     1.0,
		Landscape: false, // Portrait mode for mobile devices
		Mobile:    true,  // Enable mobile mode
		Touch:     true,  // Enable touch events
	}
	
	// Apply device emulation
	err = chromedp.Run(ctx, chromedp.Emulate(deviceInfo))
	if err != nil {
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("Failed to setup device emulation: %v", err)
		result.addLog("error", fmt.Sprintf("❌ Device emulation failed: %v", err), -1)
		return result
	}
	result.addLog("info", fmt.Sprintf("✅ Device emulation (%s) configured successfully", testCase.Device.Name), -1)
	
	// 检查当前页面URL，智能决定是否需要导航
	var currentURL string
	urlErr := chromedp.Run(ctx, chromedp.Location(&currentURL))
	
	// 智能导航逻辑：Chrome启动时已加载目标URL，检查是否需要导航
	needNavigation := false
	if urlErr == nil {
		if currentURL == targetURL {
			// 当前页面已经是目标页面（Chrome启动时已加载），无需导航
			result.addLog("info", fmt.Sprintf("✅ Target page already loaded at startup: %s", currentURL), -1)
			needNavigation = false
		} else if isVisual && existingPort > 0 && currentURL != "" && currentURL != "about:blank" {
			// 可视化执行复用实例，检查是否需要切换到目标页面
			result.addLog("info", fmt.Sprintf("🔄 Current page in reused instance: %s, checking if navigation needed", currentURL), -1)
			needNavigation = (currentURL != targetURL)
		} else {
			// 其他情况需要导航到目标页面
			result.addLog("info", fmt.Sprintf("📄 Current page: %s, will navigate to target: %s", currentURL, targetURL), -1)
			needNavigation = true
		}
	} else {
		// 获取URL失败，尝试导航
		result.addLog("warn", fmt.Sprintf("⚠️ Failed to get current URL: %v, will attempt navigation", urlErr), -1)
		needNavigation = true
	}
	
	// 在当前标签页中导航到目标页面（仅在必要时）
	if needNavigation {
		result.addLog("info", fmt.Sprintf("🔄 Navigating current tab to target page: %s", targetURL), -1)
		
		// 使用chromedp.Navigate确保在当前标签页中导航
		err = chromedp.Run(ctx, 
			chromedp.Navigate(targetURL),
			chromedp.WaitReady("body", chromedp.ByQuery), // 等待页面基本加载
		)
		if err != nil {
			result.Success = false
			result.ErrorMessage = fmt.Sprintf("Failed to navigate current tab to target page: %v", err)
			result.addLog("error", fmt.Sprintf("❌ Tab navigation failed: %v", err), -1)
			return result
		}
		result.addLog("info", "✅ Successfully navigated current tab to target page", -1)
	} else {
		result.addLog("info", "✅ Target page is already loaded, no navigation needed", -1)
	}

	// Enhanced page load waiting for better dynamic content handling
	result.addLog("info", "⏳ Waiting for page to load...", -1)

	// Multi-stage page loading wait
	result.addLog("info", "🔍 Waiting for DOM body element...", -1)
	err = chromedp.Run(ctx,
		chromedp.WaitReady("body", chromedp.ByQuery),   // Wait for basic DOM
		chromedp.Sleep(2*time.Second),                  // Initial wait for JS to start
		chromedp.WaitVisible("body", chromedp.ByQuery), // Wait for body to be visible
		chromedp.Sleep(3*time.Second),                  // Additional wait for dynamic content
	)
	if err != nil {
		// If body is not ready, try to get page title and current URL for debugging
		result.addLog("warn", fmt.Sprintf("⚠️ Page loading issues: %v", err), -1)
		var title, currentURL string
		titleErr := chromedp.Run(ctx, chromedp.Title(&title))
		urlErr := chromedp.Run(ctx, chromedp.Location(&currentURL))

		debugInfo := fmt.Sprintf("🔍 Debug info - Title: '%s', URL: '%s', TitleErr: %v, URLErr: %v",
			title, currentURL, titleErr, urlErr)
		result.addLog("info", debugInfo, -1)

		// Continue execution even if page is not fully loaded
		result.addLog("warn", "⚠️ Page not fully loaded, continuing with execution", -1)
	} else {
		result.addLog("info", "✅ Page loaded successfully", -1)
	}

	// Additional check: get page URL for verification
	var pageURL string
	chromedp.Run(ctx, chromedp.Title(&pageTitle))
	chromedp.Run(ctx, chromedp.Location(&pageURL))
	result.addLog("info", fmt.Sprintf("Page info - Title: '%s', URL: '%s'", pageTitle, pageURL), -1)

	// Take initial screenshot
	screenshotPath := te.takeScreenshot(ctx, "initial", 0, testCase.Name)
	if screenshotPath != "" {
		result.Screenshots = append(result.Screenshots, screenshotPath)
	}

	// Execute test steps with enhanced logging
	totalSteps := len(steps)
	log.Printf("🏁 开始执行测试用例: %s (共 %d 个步骤)", testCase.Name, totalSteps)

	for i, step := range steps {
		stepStartTime := time.Now()
		detailedDesc := te.getDetailedStepDescription(step, i, totalSteps)

		// Enhanced step start logging
		log.Printf("🔄 %s - 开始执行...", detailedDesc)
		result.addStepLog("info", fmt.Sprintf("开始执行步骤 %d/%d: %s", i+1, totalSteps, detailedDesc), i,
			step.Type, "running", step.Selector, step.Value, "", 0, "")

		// Pre-execution validation logging
		if step.Selector != "" {
			log.Printf("🔍 步骤 %d/%d - 查找元素: %s", i+1, totalSteps, step.Selector)
		}

		err = te.executeStep(ctx, step, i)
		stepDuration := time.Since(stepStartTime).Milliseconds()

		if err != nil {
			result.ErrorMessage = fmt.Sprintf("步骤 %d 执行失败: %v", i+1, err)

			// Enhanced error logging
			log.Printf("❌ 步骤 %d/%d 执行失败 (耗时: %dms): %s - 错误: %v",
				i+1, totalSteps, stepDuration, detailedDesc, err)

			// Take error screenshot
			screenshotPath := te.takeScreenshot(ctx, "error", i, testCase.Name)
			screenshotFile := ""
			if screenshotPath != "" {
				result.Screenshots = append(result.Screenshots, screenshotPath)
				screenshotFile = screenshotPath
				log.Printf("📷 已拍摄错误截图: %s", screenshotPath)
			}

			// Log step failure with detailed info
			result.addStepLog("error", fmt.Sprintf("步骤 %d/%d 执行失败: %s - 错误: %v (耗时: %dms)",
				i+1, totalSteps, detailedDesc, err, stepDuration), i,
				step.Type, "failed", step.Selector, step.Value, screenshotFile, stepDuration, err.Error())

			return result
		}

		// Take screenshot for key steps
		screenshotFile := ""
		if te.shouldTakeScreenshot(step) {
			screenshotPath := te.takeScreenshot(ctx, "step", i, testCase.Name)
			if screenshotPath != "" {
				result.Screenshots = append(result.Screenshots, screenshotPath)
				screenshotFile = screenshotPath
				log.Printf("📷 已拍摄步骤截图: %s", screenshotPath)
			}
		}

		// Enhanced success logging with timing info
		log.Printf("✅ 步骤 %d/%d 执行成功 (耗时: %dms): %s",
			i+1, totalSteps, stepDuration, detailedDesc)

		result.addStepLog("info", fmt.Sprintf("步骤 %d/%d 执行成功: %s (耗时: %dms)",
			i+1, totalSteps, detailedDesc, stepDuration), i,
			step.Type, "success", step.Selector, step.Value, screenshotFile, stepDuration, "")

		// Progress indicator for console
		progressPercent := int(float64(i+1) / float64(totalSteps) * 100)
		log.Printf("📊 执行进度: %d%% (%d/%d 步骤已完成)", progressPercent, i+1, totalSteps)

		// Small delay between steps
		chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond))
	}

	// Take final screenshot
	screenshotPath = te.takeScreenshot(ctx, "final", len(steps), testCase.Name)
	if screenshotPath != "" {
		result.Screenshots = append(result.Screenshots, screenshotPath)
	}

	// Collect performance metrics
	result.Metrics = te.collectPerformanceMetrics(ctx)
	result.Metrics.PageLoadTime = int(time.Since(startTime).Milliseconds())

	// Check if context was cancelled before marking as successful
	select {
	case <-ctx.Done():
		result.Success = false
		result.ErrorMessage = "Execution was cancelled"
		result.addLog("info", "Test case execution was cancelled", -1)
		log.Printf("⚠️ 测试用例执行被取消: %s", testCase.Name)
	default:
		result.Success = true
		result.addLog("info", "Test case execution completed successfully", -1)
		log.Printf("🎉 测试用例执行成功完成: %s (共执行 %d 个步骤, 耗时: %.2f秒)",
			testCase.Name, totalSteps, time.Since(startTime).Seconds())
	}

	return result
}

func (te *TestExecutor) executeStep(ctx context.Context, step models.TestStep, stepIndex int) error {
	switch step.Type {
	case "click":
		return te.executeClick(ctx, step)
	case "input":
		return te.executeInput(ctx, step)
	case "keydown":
		return te.executeKeydown(ctx, step)
	case "scroll":
		return te.executeScroll(ctx, step)
	case "touchstart", "touchend":
		return te.executeTouch(ctx, step)
	case "change":
		return te.executeChange(ctx, step)
	case "submit":
		return te.executeSubmit(ctx, step)
	default:
		return fmt.Errorf("unsupported step type: %s", step.Type)
	}
}

func (te *TestExecutor) executeClick(ctx context.Context, step models.TestStep) error {
	// Enhanced approach with better element waiting and retry mechanism

	// First, wait for the element to be visible and enabled
	err := chromedp.Run(ctx,
		chromedp.WaitVisible(step.Selector, chromedp.ByQuery),
		chromedp.WaitEnabled(step.Selector, chromedp.ByQuery),
	)

	if err != nil {
		return fmt.Errorf("failed to wait for element %s to be visible and enabled: %v", step.Selector, err)
	}

	// Add additional wait for dynamic content to stabilize
	time.Sleep(500 * time.Millisecond)

	// Try clicking with retry mechanism
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err = chromedp.Run(ctx,
			chromedp.Click(step.Selector, chromedp.ByQuery),
			chromedp.Sleep(200*time.Millisecond),
		)

		if err == nil {
			return nil // Success
		}

		if attempt < maxRetries {
			log.Printf("Click attempt %d failed for element %s: %v, retrying...", attempt, step.Selector, err)
			time.Sleep(time.Duration(attempt) * 500 * time.Millisecond) // Exponential backoff
		}
	}

	return fmt.Errorf("failed to click element %s after %d attempts: %v", step.Selector, maxRetries, err)
}

func (te *TestExecutor) executeInput(ctx context.Context, step models.TestStep) error {
	return chromedp.Run(ctx,
		chromedp.Clear(step.Selector),
		chromedp.SendKeys(step.Selector, step.Value),
		chromedp.Sleep(200*time.Millisecond),
	)
}

func (te *TestExecutor) executeKeydown(ctx context.Context, step models.TestStep) error {
	return chromedp.Run(ctx,
		chromedp.KeyEvent(step.Value),
		chromedp.Sleep(200*time.Millisecond),
	)
}

func (te *TestExecutor) executeScroll(ctx context.Context, step models.TestStep) error {
	if coords, ok := step.Coordinates["scrollY"].(float64); ok {
		return chromedp.Run(ctx,
			chromedp.Evaluate(fmt.Sprintf("window.scrollTo(0, %f)", coords), nil),
			chromedp.Sleep(200*time.Millisecond),
		)
	}
	return nil
}

func (te *TestExecutor) executeTouch(ctx context.Context, step models.TestStep) error {
	// For touch events, we simulate them as clicks for now
	if step.Type == "touchstart" {
		return te.executeClick(ctx, step)
	}
	return nil
}

func (te *TestExecutor) executeChange(ctx context.Context, step models.TestStep) error {
	return chromedp.Run(ctx,
		chromedp.SetValue(step.Selector, step.Value),
		chromedp.Sleep(200*time.Millisecond),
	)
}

func (te *TestExecutor) executeSubmit(ctx context.Context, step models.TestStep) error {
	return chromedp.Run(ctx,
		chromedp.Submit(step.Selector),
		chromedp.Sleep(1*time.Second),
	)
}

func (te *TestExecutor) takeScreenshot(ctx context.Context, stepType string, stepIndex int, testCaseName string) string {
	now := time.Now()
	dateFolder := now.Format("2006-01-02")
	timeStamp := now.Format("15:04:05")
	filename := fmt.Sprintf("%s_%s_%d_%s.png", testCaseName, stepType, stepIndex, timeStamp)

	// Create daily screenshots directory if not exists
	screenshotDir := filepath.Join("../screenshots", dateFolder)
	if err := os.MkdirAll(screenshotDir, 0755); err != nil {
		log.Printf("Failed to create screenshots directory: %v", err)
		return ""
	}

	fullPath := filepath.Join(screenshotDir, filename)

	var buf []byte
	err := chromedp.Run(ctx, chromedp.CaptureScreenshot(&buf))
	if err != nil {
		log.Printf("Failed to take screenshot: %v", err)
		return ""
	}

	// Save screenshot to file
	err = ioutil.WriteFile(fullPath, buf, 0644)
	if err != nil {
		log.Printf("Failed to save screenshot file: %v", err)
		return ""
	}

	log.Printf("📸 Screenshot saved: %s (step %d, type: %s)", filename, stepIndex, stepType)
	return filepath.Join(dateFolder, filename)
}

func (te *TestExecutor) shouldTakeScreenshot(step models.TestStep) bool {
	// Take screenshots for key interaction types
	keyTypes := []string{"click", "submit", "change"}
	for _, keyType := range keyTypes {
		if step.Type == keyType {
			return true
		}
	}
	return false
}

func (te *TestExecutor) collectPerformanceMetrics(ctx context.Context) *models.PerformanceMetric {
	metric := &models.PerformanceMetric{}

	// Collect performance timing data using string evaluation
	var performanceDataStr string
	err := chromedp.Run(ctx,
		chromedp.Evaluate(`
			JSON.stringify({
				domContentLoaded: performance.timing.domContentLoadedEventEnd - performance.timing.navigationStart,
				firstPaint: performance.getEntriesByType('paint').find(entry => entry.name === 'first-paint')?.startTime || 0,
				firstContentfulPaint: performance.getEntriesByType('paint').find(entry => entry.name === 'first-contentful-paint')?.startTime || 0,
				memoryUsage: performance.memory ? performance.memory.usedJSHeapSize / 1024 / 1024 : 0,
				networkRequests: performance.getEntriesByType('resource').length,
				networkTime: performance.getEntriesByType('navigation')[0] ? performance.getEntriesByType('navigation')[0].loadEventEnd - performance.getEntriesByType('navigation')[0].fetchStart : 0,
				jsHeapSize: performance.memory ? performance.memory.totalJSHeapSize / 1024 / 1024 : 0
			})
		`, &performanceDataStr),
	)

	if err != nil {
		log.Printf("Failed to collect performance metrics: %v", err)
		return metric
	}

	// Parse the JSON string manually
	performanceDataStr = strings.Trim(performanceDataStr, "\"")
	performanceDataStr = strings.ReplaceAll(performanceDataStr, "\\", "")

	// Extract values using string parsing (simple implementation)
	if strings.Contains(performanceDataStr, "domContentLoaded") {
		if idx := strings.Index(performanceDataStr, "domContentLoaded\":"); idx != -1 {
			valueStr := performanceDataStr[idx+17:]
			if commaIdx := strings.Index(valueStr, ","); commaIdx != -1 {
				valueStr = valueStr[:commaIdx]
			}
			if val := parseFloat(valueStr); val > 0 {
				metric.DOMContentLoaded = int(val)
			}
		}
	}

	if strings.Contains(performanceDataStr, "memoryUsage") {
		if idx := strings.Index(performanceDataStr, "memoryUsage\":"); idx != -1 {
			valueStr := performanceDataStr[idx+13:]
			if commaIdx := strings.Index(valueStr, ","); commaIdx != -1 {
				valueStr = valueStr[:commaIdx]
			}
			if val := parseFloat(valueStr); val > 0 {
				metric.MemoryUsage = val
			}
		}
	}

	if strings.Contains(performanceDataStr, "networkRequests") {
		if idx := strings.Index(performanceDataStr, "networkRequests\":"); idx != -1 {
			valueStr := performanceDataStr[idx+17:]
			if commaIdx := strings.Index(valueStr, ","); commaIdx != -1 {
				valueStr = valueStr[:commaIdx]
			} else if closeIdx := strings.Index(valueStr, "}"); closeIdx != -1 {
				valueStr = valueStr[:closeIdx]
			}
			if val := parseFloat(valueStr); val > 0 {
				metric.NetworkRequests = int(val)
			}
		}
	}

	return metric
}

// Simple float parsing helper
func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	var result float64 = 0
	var decimal float64 = 0.1
	var isDecimal bool = false

	for _, char := range s {
		if char >= '0' && char <= '9' {
			digit := float64(char - '0')
			if isDecimal {
				result += digit * decimal
				decimal *= 0.1
			} else {
				result = result*10 + digit
			}
		} else if char == '.' && !isDecimal {
			isDecimal = true
		} else {
			break
		}
	}
	return result
}

func (result *ExecutionResult) addLog(level, message string, stepIndex int) {
	result.Logs = append(result.Logs, ExecutionLog{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		StepIndex: stepIndex,
	})
}

func (result *ExecutionResult) addStepLog(level, message string, stepIndex int, stepType, stepStatus, selector, value, screenshot string, duration int64, errorDetail string) {
	result.Logs = append(result.Logs, ExecutionLog{
		Timestamp:   time.Now(),
		Level:       level,
		Message:     message,
		StepIndex:   stepIndex,
		StepType:    stepType,
		StepStatus:  stepStatus,
		Selector:    selector,
		Value:       value,
		Screenshot:  screenshot,
		Duration:    duration,
		ErrorDetail: errorDetail,
	})
}

func (te *TestExecutor) getStepDescription(step models.TestStep) string {
	switch step.Type {
	case "click":
		return fmt.Sprintf("点击元素 %s", step.Selector)
	case "input":
		return fmt.Sprintf("在 %s 输入: %s", step.Selector, step.Value)
	case "keydown":
		return fmt.Sprintf("按键: %s", step.Value)
	case "scroll":
		return "页面滚动"
	case "touchstart", "touchend":
		return fmt.Sprintf("触摸操作: %s", step.Selector)
	case "change":
		return fmt.Sprintf("更改 %s 的值为: %s", step.Selector, step.Value)
	case "submit":
		return fmt.Sprintf("提交表单: %s", step.Selector)
	default:
		return fmt.Sprintf("执行 %s 操作", step.Type)
	}
}

// getDetailedStepDescription returns enhanced step description with progress info
func (te *TestExecutor) getDetailedStepDescription(step models.TestStep, stepIndex, totalSteps int) string {
	progress := fmt.Sprintf("[%d/%d]", stepIndex+1, totalSteps)

	switch step.Type {
	case "click":
		return fmt.Sprintf("%s 🔘 点击元素: %s", progress, step.Selector)
	case "input":
		if len(step.Value) > 50 {
			return fmt.Sprintf("%s ⌨️ 输入文本到 %s (长度: %d字符)", progress, step.Selector, len(step.Value))
		}
		return fmt.Sprintf("%s ⌨️ 输入文本到 %s: %s", progress, step.Selector, step.Value)
	case "keydown":
		return fmt.Sprintf("%s ⌨️ 按键操作: %s", progress, step.Value)
	case "scroll":
		if coords, ok := step.Coordinates["scrollY"].(float64); ok {
			return fmt.Sprintf("%s 📜 页面滚动到位置: Y=%.0f", progress, coords)
		}
		return fmt.Sprintf("%s 📜 页面滚动操作", progress)
	case "touchstart", "touchend":
		return fmt.Sprintf("%s 👆 触摸操作: %s", progress, step.Selector)
	case "change":
		return fmt.Sprintf("%s 🔄 更改元素值 %s → %s", progress, step.Selector, step.Value)
	case "submit":
		return fmt.Sprintf("%s ✅ 提交表单: %s", progress, step.Selector)
	default:
		return fmt.Sprintf("%s ⚙️ 执行 %s 操作: %s", progress, step.Type, step.Selector)
	}
}

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(result)
}

func getCurrentDir() string {
	if dir, err := os.Getwd(); err == nil {
		return dir
	}
	return "unknown"
}

// Stop gracefully shuts down the executor
func (te *TestExecutor) Stop() {
	te.mutex.Lock()
	defer te.mutex.Unlock()

	if te.workQueue != nil {
		close(te.workQueue)
	}

	if te.cancel != nil {
		te.cancel()
	}

	log.Println("Test executor stopped")
}

// GetExecutionStatus returns the current status of an execution
func (te *TestExecutor) GetExecutionStatus(executionID uint) string {
	te.mutex.RLock()
	defer te.mutex.RUnlock()

	if te.running[executionID] {
		return "running"
	}
	return "completed"
}

// CancelExecution cancels a running execution
func (te *TestExecutor) CancelExecution(executionID uint) bool {
	te.mutex.Lock()
	defer te.mutex.Unlock()

	if te.running[executionID] {
		// Call the cancel function to close browser and cancel context
		if cancelFunc, exists := te.cancels[executionID]; exists {
			log.Printf("Cancelling execution %d and closing browser", executionID)
			cancelFunc()
		}

		// Clean up all tracking maps
		delete(te.running, executionID)
		delete(te.cancels, executionID)
		delete(te.completions, executionID)
		log.Printf("Execution %d cancelled", executionID)
		return true
	}
	return false
}

// closeBrowser gracefully closes all tabs and then the entire Chrome browser process
func (te *TestExecutor) closeBrowser(ctx context.Context) {
	if ctx == nil {
		return
	}

	log.Printf("Attempting to close Chrome browser gracefully...")

	// Method 1: First close all tabs one by one
	err := chromedp.Run(ctx, chromedp.Evaluate(`
		try {
			// Get all window references
			const allWindows = [];
			
			// Close current window first
			if (window.self !== window.top) {
				// If in iframe, close parent
				try { window.parent.close(); } catch(e) {}
			}
			
			// Try to close current window
			window.close();
			
			// Additional cleanup for any remaining windows
			if (window.chrome && window.chrome.runtime) {
				window.chrome.runtime.exit();
			}
			
			console.log('All tabs closing sequence initiated');
		} catch(e) {
			console.log('Tab close attempt failed:', e);
		}
	`, nil))

	if err != nil {
		log.Printf("JavaScript tab close failed: %v", err)
	}

	// Method 2: Wait briefly for graceful close
	time.Sleep(300 * time.Millisecond)

	// Method 3: Try ChromeDP's built-in browser close
	log.Printf("Attempting ChromeDP browser close...")
	browserCloseErr := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		// Use Chrome DevTools Protocol to close browser
		return chromedp.Evaluate(`
			if (window.chrome && window.chrome.runtime) {
				window.chrome.runtime.exit();
			} else {
				window.close();
			}
		`, nil).Do(ctx)
	}))

	if browserCloseErr != nil {
		log.Printf("ChromeDP browser close failed: %v", browserCloseErr)
	}

	// Method 4: Give a brief moment for graceful close, then force terminate
	time.Sleep(500 * time.Millisecond)

	log.Printf("Chrome browser close sequence completed - context will be cancelled to force process termination")

	// Method 5: Force terminate Chrome processes as last resort
	go func() {
		time.Sleep(2 * time.Second) // Give graceful close some time
		te.forceKillChromeProcesses()
	}()
}

// ensureExecutionCompleted ensures that an execution is marked as completed in database
// This is a safety net to prevent executions from staying in "running" state
func (te *TestExecutor) ensureExecutionCompleted(executionID uint) {
	// Create a callback channel for status verification
	go func() {
		// Wait a short time for normal completion to process
		time.Sleep(2 * time.Second)

		// Check if execution is still marked as running in our internal state
		te.mutex.RLock()
		stillRunning := te.running[executionID]
		te.mutex.RUnlock()

		if !stillRunning {
			log.Printf("Browser cleanup completed for execution %d - execution no longer in running state", executionID)
		} else {
			log.Printf("WARNING: Browser cleanup completed for execution %d but still marked as running internally", executionID)
		}
	}()
}

// forceKillChromeProcesses terminates all Chrome processes related to automation
func (te *TestExecutor) forceKillChromeProcesses() {
	switch runtime.GOOS {
	case "linux":
		// Kill Chrome processes that might be related to automation
		cmd := exec.Command("pkill", "-f", "chrome.*automation")
		if err := cmd.Run(); err == nil {
			log.Printf("Force killed Chrome automation processes on Linux")
		}

		// Also try killing any chrome processes with our specific flags
		cmd2 := exec.Command("pkill", "-f", "chrome.*disable-blink-features.*AutomationControlled")
		if err := cmd2.Run(); err == nil {
			log.Printf("Force killed Chrome processes with automation flags")
		}

	case "darwin":
		// Kill Chrome processes on macOS
		cmd := exec.Command("pkill", "-f", "Google Chrome.*automation")
		if err := cmd.Run(); err == nil {
			log.Printf("Force killed Chrome automation processes on macOS")
		}

	case "windows":
		// Kill Chrome processes on Windows
		cmd := exec.Command("taskkill", "/F", "/IM", "chrome.exe")
		if err := cmd.Run(); err == nil {
			log.Printf("Force killed Chrome processes on Windows")
		}
	}
}
