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
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
	"webtestflow/backend/internal/models"
	"webtestflow/backend/pkg/chrome"

	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/device"
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
		result := te.executeTestCase(job.Execution.ID, job.TestCase)

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
	return te.ExecuteTestCaseWithOptions(execution, testCase)
}

func (te *TestExecutor) ExecuteTestCaseWithOptions(execution *models.TestExecution, testCase *models.TestCase) <-chan ExecutionResult {
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
		IsVisual:     true, // Always visual execution
		ResultChan:   resultChan,
		CompleteChan: completeChan,
	}

	te.workQueue <- job
	return resultChan
}

// ExecuteTestCaseDirectly executes a test case directly without using the worker queue
// This method is safer for sequential execution and avoids ChromeDP concurrency issues
func (te *TestExecutor) ExecuteTestCaseDirectly(execution *models.TestExecution, testCase *models.TestCase) ExecutionResult {
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
	result = te.executeTestCase(execution.ID, testCase)

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

func (te *TestExecutor) executeTestCase(executionID uint, testCase *models.TestCase) ExecutionResult {
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

	// 检查是否有已存在的Chrome实例（可视化执行总是尝试复用）
	var port int
	existingPort := chrome.GlobalChromeManager.GetExistingPort(executionID, true)

	if existingPort > 0 {
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

	if existingPort == 0 {
		// 启动新的Chrome实例（可视化模式），直接加载目标URL避免空白页
		result.addLog("info", fmt.Sprintf("🚀 Starting Chrome in visual mode with target URL: %s", targetURL), -1)

		port, err = chrome.GlobalChromeManager.StartChromeWithURL(executionID, true, targetURL)
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
	defer func() {
		result.addLog("info", fmt.Sprintf("🧹 Starting Chrome cleanup for execution %d", executionID), -1)

		// Skip aggressive browser closing for visual executions to prevent page disruption
		// Since we now only support visual execution, keep browser open to preserve page functionality
		result.addLog("info", "🎬 Visual execution - keeping browser open to preserve page functionality", -1)

		// Step 2: Close ChromeDP contexts gently
		if chromeCleanup != nil {
			result.addLog("info", "🔄 Closing ChromeDP contexts...", -1)
			chromeCleanup()
		}

		// Step 3: Stop Chrome process (gracefully for visual, normally for non-visual)
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
		ID                   string `json:"id"`
		Type                 string `json:"type"`
		URL                  string `json:"url"`
		Title                string `json:"title"`
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
		ID                   string `json:"id"`
		Type                 string `json:"type"`
		URL                  string `json:"url"`
		Title                string `json:"title"`
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
		chromedp.WithTargetID(target.ID(targetTab.ID)),     // 连接到指定标签页
		chromedp.WithLogf(func(string, ...interface{}) {}), // 禁用ChromeDP的debug日志
	)

	// 保存Chrome上下文以便后续清理使用
	// Store Chrome context for graceful cleanup - removed for visual execution protection

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
		} else if existingPort > 0 && currentURL != "" && currentURL != "about:blank" {
			// 复用实例，检查是否需要切换到目标页面
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

	// Enhanced multi-stage page loading wait with dynamic content detection
	result.addLog("info", "🔍 Waiting for DOM and dynamic content...", -1)
	err = te.waitForPageStabilization(ctx)
	if err != nil {
		result.addLog("warn", fmt.Sprintf("⚠️ Page stabilization had some issues: %v, but continuing", err), -1)
	}
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
		
		// Check if step should be skipped
		if step.SkipStep {
			log.Printf("⏭️ 步骤 %d/%d - 已跳过: %s", i+1, totalSteps, detailedDesc)
			result.addStepLog("info", fmt.Sprintf("步骤 %d/%d 已跳过: %s", i+1, totalSteps, detailedDesc), i,
				step.Type, "skipped", step.Selector, step.Value, "", 0, "")
			continue
		}

		// Check if step needs wait before execution
		if step.WaitBefore > 0 {
			waitDuration := time.Duration(step.WaitBefore) * time.Second
			log.Printf("⏳ 步骤 %d/%d - 等待 %d 秒后执行: %s", i+1, totalSteps, step.WaitBefore, detailedDesc)
			result.addStepLog("info", fmt.Sprintf("等待 %d 秒后执行步骤 %d/%d", step.WaitBefore, i+1, totalSteps), i,
				"wait", "running", "", fmt.Sprintf("%d", step.WaitBefore), "", 0, "")
			
			// Context-aware wait
			select {
			case <-time.After(waitDuration):
				log.Printf("✅ 等待完成，开始执行步骤 %d/%d", i+1, totalSteps)
			case <-ctx.Done():
				result.ErrorMessage = fmt.Sprintf("步骤 %d 等待过程中被取消", i+1)
				return result
			}
		}

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
	case "touchstart", "touchend", "touchmove":
		return te.executeTouch(ctx, step)
	case "swipe":
		return te.executeSwipe(ctx, step)
	case "mousedrag":
		return te.executeMouseDrag(ctx, step)
	case "change":
		return te.executeChange(ctx, step)
	case "submit":
		return te.executeSubmit(ctx, step)
	case "navigate":
		return te.executeNavigate(ctx, step)
	case "cross_domain_navigation":
		return te.executeCrossDomainNavigation(ctx, step)
	case "back":
		return te.executeBack(ctx, step)
	case "beforeunload":
		return te.executeBeforeunload(ctx, step)
	case "popstate":
		return te.executePopstate(ctx, step)
	case "hashchange":
		return te.executeHashchange(ctx, step)
	default:
		return fmt.Errorf("unsupported step type: %s", step.Type)
	}
}

func (te *TestExecutor) executeClick(ctx context.Context, step models.TestStep) error {
	// Get fallback selectors from step options
	var fallbackSelectors []string
	if step.Options != nil {
		if fallbacks, ok := step.Options["fallbackSelectors"].([]interface{}); ok {
			for _, fb := range fallbacks {
				if sel, ok := fb.(string); ok {
					fallbackSelectors = append(fallbackSelectors, sel)
				}
			}
		}
	}

	// Generate intelligent fallback selectors
	smartFallbacks := te.generateSmartSelectors(step.Selector, step.Options)
	
	// Prepare all selectors to try (primary + manual fallbacks + smart fallbacks)
	selectorsToTry := []string{step.Selector}
	selectorsToTry = append(selectorsToTry, fallbackSelectors...)
	selectorsToTry = append(selectorsToTry, smartFallbacks...)

	// DEBUGGING: Log current page structure for the failed selector
	te.debugPageStructure(ctx, step.Selector)

	// Try each selector until one works
	for i, selector := range selectorsToTry {
		log.Printf("🔍 Trying selector %d/%d: %s", i+1, len(selectorsToTry), selector)

		// Handle special text-content selector
		if strings.Contains(selector, "[text-content=") {
			if err := te.executeClickByText(ctx, selector, step); err == nil {
				log.Printf("✅ Clicked successfully using text-content selector")
				return nil
			}
			log.Printf("❌ Text-content selector failed")
			continue
		}

		// Check if element exists in DOM first
		var exists bool
		err := chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`
			document.querySelector('%s') !== null
		`, selector), &exists))
		
		if err != nil {
			log.Printf("❌ Error checking element existence: %v", err)
			continue
		}
		
		if !exists {
			log.Printf("❌ Element does not exist in DOM: %s", selector)
			// Log similar elements for debugging
			te.findSimilarElements(ctx, selector)
			continue
		}

		log.Printf("✓ Element exists in DOM: %s", selector)

		// Enhanced element waiting with multiple strategies
		err = te.waitForElementSmart(ctx, selector)

		if err != nil {
			log.Printf("❌ Element not ready for interaction: %s, error: %v", selector, err)
			continue // Try next selector
		}

		log.Printf("✓ Element ready for interaction: %s", selector)

		// Add additional wait for dynamic content to stabilize
		time.Sleep(500 * time.Millisecond)

		// Try clicking with retry mechanism
		maxRetries := 3
		var clickErr error
		for attempt := 1; attempt <= maxRetries; attempt++ {
			clickErr = chromedp.Run(ctx,
				chromedp.Click(selector, chromedp.ByQuery),
				chromedp.Sleep(200*time.Millisecond),
			)

			if clickErr == nil {
				log.Printf("🎯 Successfully clicked element with selector: %s (attempt %d)", selector, attempt)
				return nil // Success
			}

			if attempt < maxRetries {
				log.Printf("⚠️ Click attempt %d failed for element %s: %v, retrying...", attempt, selector, clickErr)
				time.Sleep(time.Duration(attempt) * 500 * time.Millisecond) // Exponential backoff
			}
		}

		// If we got here, all click attempts failed for this selector
		log.Printf("❌ All click attempts failed for selector: %s", selector)
	}

	// If we got here, all selectors failed - provide detailed debugging info
	log.Printf("🚨 COMPLETE FAILURE: All %d selectors failed for click action", len(selectorsToTry))
	te.debugCompleteFailure(ctx, selectorsToTry)
	
	return fmt.Errorf("failed to click element with any selector (tried %d selectors)", len(selectorsToTry))
}

// generateSmartSelectors creates intelligent fallback selectors based on the original selector
func (te *TestExecutor) generateSmartSelectors(originalSelector string, options map[string]interface{}) []string {
	var smartSelectors []string
	
	// Strategy 1: Remove nth-of-type constraints (most common fix)
	relaxedSelector := te.relaxSelector(originalSelector)
	if relaxedSelector != originalSelector {
		smartSelectors = append(smartSelectors, relaxedSelector)
	}
	
	// Strategy 2: Use only the last class in the chain (deepest element)
	if strings.Contains(originalSelector, ".") && strings.Contains(originalSelector, ">") {
		parts := strings.Split(originalSelector, ">")
		if len(parts) > 0 {
			lastPart := strings.TrimSpace(parts[len(parts)-1])
			if strings.Contains(lastPart, ".") {
				// Extract just the class name
				if classMatch := strings.Split(lastPart, "."); len(classMatch) > 1 {
					justClass := "." + classMatch[1]
					smartSelectors = append(smartSelectors, justClass)
				}
			}
		}
	}
	
	// Strategy 3: Extract all individual class selectors
	classSelectors := te.extractClassSelectors(originalSelector)
	smartSelectors = append(smartSelectors, classSelectors...)
	
	// Strategy 4: Create attribute-based selectors from class names
	if strings.Contains(originalSelector, "Protectthechild") {
		smartSelectors = append(smartSelectors, `[class*="Protectthechild"]`)
		smartSelectors = append(smartSelectors, `div[class*="Protectthechild"]`)
	}
	if strings.Contains(originalSelector, "edit") {
		smartSelectors = append(smartSelectors, `[class*="edit"]`)
		smartSelectors = append(smartSelectors, `div[class*="edit"]`)
	}
	if strings.Contains(originalSelector, "icon") {
		smartSelectors = append(smartSelectors, `[class*="icon"]`)
		smartSelectors = append(smartSelectors, `div[class*="icon"]`)
	}
	
	// Extract element text if available for text-based selection
	if options != nil {
		if elementText, ok := options["elementText"].(string); ok && elementText != "" {
			// Create selectors based on text content
			smartSelectors = append(smartSelectors, 
				fmt.Sprintf("*[text-content=\"%s\"]", elementText),
				fmt.Sprintf("*:contains('%s')", elementText),
			)
		}
		
		if tagName, ok := options["tagName"].(string); ok && tagName != "" {
			// Create tag-based selectors
			if elementText, ok := options["elementText"].(string); ok && elementText != "" {
				smartSelectors = append(smartSelectors,
					fmt.Sprintf("%s[text-content=\"%s\"]", tagName, elementText),
				)
			}
		}
	}
	
	return smartSelectors
}

// relaxSelector removes nth-of-type and nth-child constraints to make selector more flexible
func (te *TestExecutor) relaxSelector(selector string) string {
	// Remove nth-of-type(n) patterns
	re1 := regexp.MustCompile(`:nth-of-type\(\d+\)`)
	relaxed := re1.ReplaceAllString(selector, "")
	
	// Remove nth-child(n) patterns  
	re2 := regexp.MustCompile(`:nth-child\(\d+\)`)
	relaxed = re2.ReplaceAllString(relaxed, "")
	
	// Clean up any double spaces or trailing/leading spaces
	relaxed = regexp.MustCompile(`\s+`).ReplaceAllString(strings.TrimSpace(relaxed), " ")
	
	return relaxed
}

// extractClassSelectors extracts class-based selectors from the original selector
func (te *TestExecutor) extractClassSelectors(selector string) []string {
	var classSelectors []string
	
	// Extract class names using regex
	classRe := regexp.MustCompile(`\.([a-zA-Z][a-zA-Z0-9_-]*)`)
	matches := classRe.FindAllStringSubmatch(selector, -1)
	
	for _, match := range matches {
		if len(match) > 1 {
			className := match[1]
			// Create simple class selector
			classSelectors = append(classSelectors, fmt.Sprintf(".%s", className))
		}
	}
	
	return classSelectors
}

// waitForElementSmart uses multiple strategies to wait for element availability
func (te *TestExecutor) waitForElementSmart(ctx context.Context, selector string) error {
	// Strategy 1: Standard wait for visible and enabled (shorter timeout for first attempt)
	ctxShort, cancel1 := context.WithTimeout(ctx, 5*time.Second)
	defer cancel1()
	
	err := chromedp.Run(ctxShort,
		chromedp.WaitVisible(selector, chromedp.ByQuery),
		chromedp.WaitEnabled(selector, chromedp.ByQuery),
	)
	
	if err == nil {
		return nil // Success with standard approach
	}
	
	log.Printf("Standard wait failed for %s, trying extended strategies: %v", selector, err)
	
	// Strategy 2: Wait for DOM presence first, then visibility (longer timeout)
	ctxLong, cancel2 := context.WithTimeout(ctx, 15*time.Second)
	defer cancel2()
	
	// First wait for element to exist in DOM
	err = chromedp.Run(ctxLong, chromedp.WaitReady(selector, chromedp.ByQuery))
	if err != nil {
		log.Printf("Element not in DOM: %s, error: %v", selector, err)
		return err
	}
	
	// Then wait for visibility with polling
	for i := 0; i < 30; i++ { // 30 attempts * 500ms = 15 seconds max
		var visible bool
		err = chromedp.Run(ctxLong, 
			chromedp.Evaluate(fmt.Sprintf(`
				(function() {
					const el = document.querySelector('%s');
					if (!el) return false;
					const rect = el.getBoundingClientRect();
					const style = window.getComputedStyle(el);
					return rect.width > 0 && rect.height > 0 && 
					       style.visibility !== 'hidden' && 
					       style.display !== 'none' &&
					       !el.disabled;
				})();
			`, selector), &visible),
		)
		
		if err == nil && visible {
			log.Printf("Element became visible after %d attempts: %s", i+1, selector)
			return nil
		}
		
		time.Sleep(500 * time.Millisecond)
	}
	
	return fmt.Errorf("element %s not visible after extended wait", selector)
}

// debugPageStructure logs the current page structure to help debug selector issues
func (te *TestExecutor) debugPageStructure(ctx context.Context, originalSelector string) {
	log.Printf("🔍 DEBUG: Analyzing page structure for selector: %s", originalSelector)
	
	// Get page URL
	var currentURL string
	chromedp.Run(ctx, chromedp.Evaluate(`window.location.href`, &currentURL))
	log.Printf("🔍 Current URL: %s", currentURL)
	
	// Get page title
	var title string
	chromedp.Run(ctx, chromedp.Evaluate(`document.title`, &title))
	log.Printf("🔍 Page title: %s", title)
	
	// Check if the main class exists in the selector
	if strings.Contains(originalSelector, "Protectthechild-head") {
		var hasClass bool
		chromedp.Run(ctx, chromedp.Evaluate(`
			document.querySelector('.Protectthechild-head') !== null ||
			document.querySelector('[class*="Protectthechild-head"]') !== null
		`, &hasClass))
		log.Printf("🔍 Protectthechild-head class exists: %t", hasClass)
		
		// Get all elements with similar class names
		var similarElements string
		chromedp.Run(ctx, chromedp.Evaluate(`
			Array.from(document.querySelectorAll('[class*="Protectthechild"], [class*="head"]')).map(el => el.className).join(', ')
		`, &similarElements))
		log.Printf("🔍 Similar classes found: %s", similarElements)
	}
	
	// Get DOM depth and complexity
	var bodyHTML string
	err := chromedp.Run(ctx, chromedp.Evaluate(`document.body ? document.body.innerHTML.substring(0, 1000) : 'NO BODY'`, &bodyHTML))
	if err == nil {
		log.Printf("🔍 Body HTML sample (first 1000 chars): %s", bodyHTML)
	}
}

// findSimilarElements finds elements with similar selectors to help debug
func (te *TestExecutor) findSimilarElements(ctx context.Context, selector string) {
	log.Printf("🔍 Looking for similar elements to: %s", selector)
	
	// Extract class names from the selector
	classNames := te.extractClassSelectors(selector)
	for _, className := range classNames {
		var count int
		cleanClass := strings.TrimPrefix(className, ".")
		chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`
			document.querySelectorAll('%s').length
		`, className), &count))
		log.Printf("🔍 Elements with class '%s': %d", cleanClass, count)
		
		if count > 0 {
			// Get first few elements' details
			var details string
			chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`
				Array.from(document.querySelectorAll('%s')).slice(0, 3).map(el => 
					el.tagName + (el.id ? '#' + el.id : '') + '.' + el.className.replace(/ /g, '.')
				).join(' | ')
			`, className), &details))
			log.Printf("🔍 Sample elements: %s", details)
		}
	}
}

// debugCompleteFailure provides comprehensive debugging when all selectors fail
func (te *TestExecutor) debugCompleteFailure(ctx context.Context, selectorsToTry []string) {
	log.Printf("🚨 DEBUGGING COMPLETE SELECTOR FAILURE")
	
	// Get current page state
	var readyState, loadState string
	chromedp.Run(ctx, chromedp.Evaluate(`document.readyState`, &readyState))
	chromedp.Run(ctx, chromedp.Evaluate(`document.querySelector('body') ? 'body-exists' : 'no-body'`, &loadState))
	log.Printf("🔍 Page state: readyState=%s, bodyExists=%s", readyState, loadState)
	
	// Check for common UI framework indicators
	var frameworks []string
	var hasReact, hasVue, hasAngular, hasUniApp bool
	chromedp.Run(ctx, chromedp.Evaluate(`typeof React !== 'undefined'`, &hasReact))
	chromedp.Run(ctx, chromedp.Evaluate(`typeof Vue !== 'undefined'`, &hasVue))
	chromedp.Run(ctx, chromedp.Evaluate(`typeof angular !== 'undefined'`, &hasAngular))
	chromedp.Run(ctx, chromedp.Evaluate(`typeof uni !== 'undefined' || document.querySelector('[class*="uni-"]') !== null`, &hasUniApp))
	
	if hasReact { frameworks = append(frameworks, "React") }
	if hasVue { frameworks = append(frameworks, "Vue") }
	if hasAngular { frameworks = append(frameworks, "Angular") }
	if hasUniApp { frameworks = append(frameworks, "UniApp") }
	
	log.Printf("🔍 Detected frameworks: %v", frameworks)
	
	// Try to find the closest matching elements
	originalSelector := selectorsToTry[0]
	log.Printf("🔍 Searching for elements similar to original selector: %s", originalSelector)
	
	// Get all divs with any class
	var divCount int
	chromedp.Run(ctx, chromedp.Evaluate(`document.querySelectorAll('div[class]').length`, &divCount))
	log.Printf("🔍 Total divs with classes: %d", divCount)
	
	// Look for partial matches
	if strings.Contains(originalSelector, "Protectthechild") {
		var partialMatches string
		chromedp.Run(ctx, chromedp.Evaluate(`
			Array.from(document.querySelectorAll('div')).filter(div => 
				div.className && (
					div.className.includes('Protectthechild') ||
					div.className.includes('head') ||
					div.className.includes('edit') ||
					div.className.includes('icon')
				)
			).map(div => div.className).slice(0, 10).join(' | ')
		`, &partialMatches))
		log.Printf("🔍 Partial class matches: %s", partialMatches)
	}
	
	// Take a screenshot for manual debugging
	screenshotPath := te.takeScreenshot(ctx, "debug_failure", 0, "selector_debug")
	if screenshotPath != "" {
		log.Printf("🔍 Debug screenshot saved: %s", screenshotPath)
	}
}

// waitForPageStabilization waits for page to be fully loaded and stable
func (te *TestExecutor) waitForPageStabilization(ctx context.Context) error {
	// Stage 1: Wait for basic DOM
	log.Printf("🔍 Stage 1: Waiting for basic DOM structure...")
	err := chromedp.Run(ctx,
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
	)
	if err != nil {
		return fmt.Errorf("basic DOM not ready: %v", err)
	}
	
	// Stage 2: Wait for body visibility
	log.Printf("🔍 Stage 2: Waiting for body visibility...")
	err = chromedp.Run(ctx, chromedp.WaitVisible("body", chromedp.ByQuery))
	if err != nil {
		return fmt.Errorf("body not visible: %v", err)
	}
	
	// Stage 3: Wait for JavaScript frameworks and dynamic content
	log.Printf("🔍 Stage 3: Waiting for JavaScript frameworks...")
	time.Sleep(3 * time.Second) // Increased from 2 to 3 seconds
	
	// Stage 3.5: Check if we're dealing with a SPA that needs more time
	var isSPA bool
	chromedp.Run(ctx, chromedp.Evaluate(`
		typeof React !== 'undefined' || 
		typeof Vue !== 'undefined' || 
		typeof angular !== 'undefined' || 
		typeof uni !== 'undefined' ||
		document.querySelector('[class*="uni-"]') !== null ||
		document.querySelector('[data-reactroot]') !== null ||
		document.querySelector('.v-application') !== null
	`, &isSPA))
	
	if isSPA {
		log.Printf("🔍 Stage 3.5: SPA detected, waiting additional time for components to mount...")
		time.Sleep(2 * time.Second) // Extra wait for SPA components
	}
	
	// Stage 4: Check for common loading indicators and wait for them to disappear
	log.Printf("🔍 Stage 4: Checking for loading indicators...")
	loadingSelectors := []string{
		".loading", ".spinner", ".loader", "[data-loading]",
		".loading-overlay", ".loading-spinner", ".loading-indicator",
		"uni-loading", ".uni-loading", // UniApp specific
	}
	
	for _, selector := range loadingSelectors {
		// Wait for loading indicator to disappear (short timeout)
		ctxShort, cancel := context.WithTimeout(ctx, 3*time.Second)
		err := chromedp.Run(ctxShort, 
			chromedp.WaitNotPresent(selector, chromedp.ByQuery),
		)
		cancel()
		
		if err == nil {
			log.Printf("✅ Loading indicator %s disappeared", selector)
		}
		// Don't return error here, just log and continue
	}
	
	// Stage 5: Final stability check - wait for network idle
	log.Printf("🔍 Stage 5: Final stability check...")
	time.Sleep(2 * time.Second)
	
	// Stage 6: Check document ready state
	var readyState string
	err = chromedp.Run(ctx, chromedp.Evaluate(`document.readyState`, &readyState))
	if err == nil {
		log.Printf("📋 Document ready state: %s", readyState)
	}
	
	log.Printf("✅ Page stabilization completed")
	return nil
}

func (te *TestExecutor) executeClickByText(ctx context.Context, selector string, step models.TestStep) error {
	// Extract text content from selector like *[text-content="some text"]
	textPattern := `\[text-content="([^"]+)"\]`
	re := regexp.MustCompile(textPattern)
	matches := re.FindStringSubmatch(selector)

	if len(matches) < 2 {
		return fmt.Errorf("invalid text-content selector: %s", selector)
	}

	targetText := matches[1]

	// Use JavaScript to find element by text content
	clickScript := fmt.Sprintf(`
		(function() {
			function findElementByText(text) {
				const walker = document.createTreeWalker(
					document.body,
					NodeFilter.SHOW_TEXT,
					null,
					false
				);

				let node;
				while (node = walker.nextNode()) {
					if (node.textContent.trim() === text.trim()) {
						let element = node.parentElement;
						while (element && element.tagName) {
							if (element.offsetWidth > 0 && element.offsetHeight > 0) {
								return element;
							}
							element = element.parentElement;
						}
					}
				}
				return null;
			}

			const element = findElementByText('%s');
			if (element) {
				element.click();
				return true;
			}
			return false;
		})();
	`, targetText)

	var success bool
	err := chromedp.Run(ctx,
		chromedp.Evaluate(clickScript, &success),
	)

	if err != nil {
		return fmt.Errorf("failed to execute text-based click: %v", err)
	}

	if !success {
		return fmt.Errorf("could not find element with text: %s", targetText)
	}

	return nil
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
	// touchmove and touchend are usually handled as part of swipe
	return nil
}

func (te *TestExecutor) executeSwipe(ctx context.Context, step models.TestStep) error {
	// Extract swipe coordinates
	startX, startXOk := step.Coordinates["startX"].(float64)
	startY, startYOk := step.Coordinates["startY"].(float64)
	endX, endXOk := step.Coordinates["endX"].(float64)
	endY, endYOk := step.Coordinates["endY"].(float64)

	if !startXOk || !startYOk || !endXOk || !endYOk {
		// Fallback: try to determine swipe based on direction and element
		direction := step.Value
		if direction == "" {
			return fmt.Errorf("swipe coordinates or direction not available")
		}

		// Use a simple scroll based on direction
		return te.executeDirectionalSwipe(ctx, direction, step.Selector)
	}

	// Calculate swipe distance and duration
	deltaX := endX - startX
	deltaY := endY - startY

	// Use JavaScript to simulate the swipe
	swipeScript := fmt.Sprintf(`
		(function() {
			const element = document.querySelector('%s');
			if (!element) {
				window.scrollBy(%f, %f);
				return;
			}
			
			// Create touch events
			const startEvent = new TouchEvent('touchstart', {
				bubbles: true,
				cancelable: true,
				touches: [new Touch({
					identifier: 0,
					target: element,
					clientX: %f,
					clientY: %f,
					pageX: %f,
					pageY: %f
				})]
			});
			
			const moveEvent = new TouchEvent('touchmove', {
				bubbles: true,
				cancelable: true,
				touches: [new Touch({
					identifier: 0,
					target: element,
					clientX: %f,
					clientY: %f,
					pageX: %f,
					pageY: %f
				})]
			});
			
			const endEvent = new TouchEvent('touchend', {
				bubbles: true,
				cancelable: true,
				changedTouches: [new Touch({
					identifier: 0,
					target: element,
					clientX: %f,
					clientY: %f,
					pageX: %f,
					pageY: %f
				})]
			});
			
			// Dispatch events with timing
			element.dispatchEvent(startEvent);
			setTimeout(() => {
				element.dispatchEvent(moveEvent);
				setTimeout(() => {
					element.dispatchEvent(endEvent);
				}, 50);
			}, 50);
			
			// Also trigger scroll if it's a vertical swipe
			if (Math.abs(%f) > Math.abs(%f)) {
				window.scrollBy(0, %f);
			}
		})();
	`, step.Selector, deltaX, deltaY,
		startX, startY, startX, startY,
		endX, endY, endX, endY,
		endX, endY, endX, endY,
		deltaY, deltaX, deltaY)

	return chromedp.Run(ctx,
		chromedp.Evaluate(swipeScript, nil),
		chromedp.Sleep(300*time.Millisecond),
	)
}

func (te *TestExecutor) executeDirectionalSwipe(ctx context.Context, direction, selector string) error {
	var scrollScript string

	switch direction {
	case "up":
		scrollScript = "window.scrollBy(0, -300);"
	case "down":
		scrollScript = "window.scrollBy(0, 300);"
	case "left":
		scrollScript = "window.scrollBy(-300, 0);"
	case "right":
		scrollScript = "window.scrollBy(300, 0);"
	default:
		return fmt.Errorf("unsupported swipe direction: %s", direction)
	}

	return chromedp.Run(ctx,
		chromedp.Evaluate(scrollScript, nil),
		chromedp.Sleep(300*time.Millisecond),
	)
}

func (te *TestExecutor) executeMouseDrag(ctx context.Context, step models.TestStep) error {
	// Extract coordinates
	x, xOk := step.Coordinates["x"].(float64)
	y, yOk := step.Coordinates["y"].(float64)

	if !xOk || !yOk {
		return fmt.Errorf("mouse drag coordinates not available")
	}

	// For mousedrag events, we simulate a click at the position
	// This is useful for tracking intermediate drag positions
	dragScript := fmt.Sprintf(`
		(function() {
			const element = document.querySelector('%s');
			if (element) {
				const event = new MouseEvent('mousemove', {
					bubbles: true,
					cancelable: true,
					clientX: %f,
					clientY: %f,
					button: 0,
					buttons: 1
				});
				element.dispatchEvent(event);
			}
		})();
	`, step.Selector, x, y)

	return chromedp.Run(ctx,
		chromedp.Evaluate(dragScript, nil),
		chromedp.Sleep(50*time.Millisecond), // Short delay for drag
	)
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
	log.Printf("🔍 [DEBUG] Starting takeScreenshot: stepType=%s, stepIndex=%d, testCase=%s", stepType, stepIndex, testCaseName)
	
	now := time.Now()
	dateFolder := now.Format("2006-01-02")
	timeStamp := now.Format("15:04:05")
	
	// Sanitize test case name for file system - replace problematic characters
	sanitizedTestCaseName := strings.ReplaceAll(testCaseName, "/", "_")
	sanitizedTestCaseName = strings.ReplaceAll(sanitizedTestCaseName, "\\", "_")
	sanitizedTestCaseName = strings.ReplaceAll(sanitizedTestCaseName, ":", "_")
	sanitizedTestCaseName = strings.ReplaceAll(sanitizedTestCaseName, "*", "_")
	sanitizedTestCaseName = strings.ReplaceAll(sanitizedTestCaseName, "?", "_")
	sanitizedTestCaseName = strings.ReplaceAll(sanitizedTestCaseName, "\"", "_")
	sanitizedTestCaseName = strings.ReplaceAll(sanitizedTestCaseName, "<", "_")
	sanitizedTestCaseName = strings.ReplaceAll(sanitizedTestCaseName, ">", "_")
	sanitizedTestCaseName = strings.ReplaceAll(sanitizedTestCaseName, "|", "_")
	
	filename := fmt.Sprintf("%s_%s_%d_%s.png", sanitizedTestCaseName, stepType, stepIndex, timeStamp)

	// Create daily screenshots directory if not exists
	screenshotDir := filepath.Join("../screenshots", dateFolder)
	log.Printf("🔍 [DEBUG] Screenshot directory: %s", screenshotDir)
	
	if err := os.MkdirAll(screenshotDir, 0755); err != nil {
		log.Printf("❌ Failed to create screenshots directory: %v", err)
		return ""
	}

	fullPath := filepath.Join(screenshotDir, filename)
	log.Printf("🔍 [DEBUG] Full screenshot path: %s", fullPath)

	var buf []byte
	err := chromedp.Run(ctx, chromedp.CaptureScreenshot(&buf))
	if err != nil {
		log.Printf("❌ Failed to capture screenshot: %v", err)
		return ""
	}
	
	log.Printf("✅ [DEBUG] Screenshot captured successfully, buffer size: %d bytes", len(buf))

	// Save screenshot to file
	err = ioutil.WriteFile(fullPath, buf, 0644)
	if err != nil {
		log.Printf("❌ Failed to save screenshot file: %v", err)
		return ""
	}

	relativePath := filepath.Join(dateFolder, filename)
	log.Printf("📸 Screenshot saved successfully: %s (step %d, type: %s)", filename, stepIndex, stepType)
	log.Printf("🔍 [DEBUG] Returning relative path: %s", relativePath)
	return relativePath
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
	case "touchstart":
		return fmt.Sprintf("触摸开始: %s", step.Selector)
	case "touchend":
		return fmt.Sprintf("触摸结束: %s", step.Selector)
	case "touchmove":
		return fmt.Sprintf("触摸移动: %s", step.Selector)
	case "swipe":
		return fmt.Sprintf("滑动操作: %s (%s)", step.Selector, step.Value)
	case "mousedrag":
		return fmt.Sprintf("鼠标拖动: %s", step.Selector)
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
	case "touchstart":
		return fmt.Sprintf("%s 👆 触摸开始: %s", progress, step.Selector)
	case "touchend":
		return fmt.Sprintf("%s 👆 触摸结束: %s", progress, step.Selector)
	case "touchmove":
		return fmt.Sprintf("%s 👆 触摸移动: %s", progress, step.Selector)
	case "swipe":
		if direction := step.Value; direction != "" {
			return fmt.Sprintf("%s 👆 滑动操作: %s (方向: %s)", progress, step.Selector, direction)
		}
		return fmt.Sprintf("%s 👆 滑动操作: %s", progress, step.Selector)
	case "mousedrag":
		return fmt.Sprintf("%s 🖱️ 鼠标拖动: %s", progress, step.Selector)
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

// executeNavigate handles page navigation steps with enhanced stability
func (te *TestExecutor) executeNavigate(ctx context.Context, step models.TestStep) error {
	targetURL := step.Value
	if targetURL == "" {
		return fmt.Errorf("navigate step requires a URL in value field")
	}
	
	log.Printf("🌐 Executing enhanced navigation to: %s", targetURL)
	
	// Get current URL for comparison
	var currentURL string
	chromedp.Run(ctx, chromedp.Location(&currentURL))
	log.Printf("📍 Current URL before navigation: %s", currentURL)
	
	// Multi-stage navigation with enhanced error handling
	maxRetries := 3
	var lastError error
	
	for attempt := 1; attempt <= maxRetries; attempt++ {
		log.Printf("🔄 Navigation attempt %d/%d to: %s", attempt, maxRetries, targetURL)
		
		// Progressive timeout based on attempt number
		timeout := time.Duration(10+attempt*5) * time.Second
		navCtx, cancel := context.WithTimeout(ctx, timeout)
		
		err := chromedp.Run(navCtx,
			// Enhanced navigation sequence
			chromedp.Navigate(targetURL),
			chromedp.Sleep(1*time.Second), // Initial wait for navigation to start
			chromedp.WaitReady("body", chromedp.ByQuery), // Wait for basic DOM
		)
		
		cancel()
		
		if err != nil {
			lastError = err
			log.Printf("❌ Navigation attempt %d failed: %v", attempt, err)
			
			// If not the last attempt, wait before retry
			if attempt < maxRetries {
				waitTime := time.Duration(attempt*2) * time.Second
				log.Printf("⏳ Waiting %v before retry...", waitTime)
				time.Sleep(waitTime)
				continue
			}
		} else {
			// Verify navigation succeeded
			var newURL string
			chromedp.Run(ctx, chromedp.Location(&newURL))
			log.Printf("📍 URL after navigation: %s", newURL)
			
			// Additional stability wait with document ready check
			err = te.enhancedPageStabilization(ctx, targetURL)
			if err != nil {
				log.Printf("⚠️ Page stabilization had issues: %v", err)
			}
			
			log.Printf("✅ Successfully navigated to: %s", targetURL)
			return nil
		}
	}
	
	return fmt.Errorf("navigation failed after %d attempts. Last error: %w", maxRetries, lastError)
}

// executeCrossDomainNavigation handles cross-domain navigation steps with enhanced stability
func (te *TestExecutor) executeCrossDomainNavigation(ctx context.Context, step models.TestStep) error {
	targetURL := step.Value
	if targetURL == "" {
		return fmt.Errorf("cross_domain_navigation step requires a URL in value field")
	}
	
	log.Printf("🌐 Executing enhanced cross-domain navigation to: %s", targetURL)
	
	// Get current domain for logging
	var currentURL string
	chromedp.Run(ctx, chromedp.Location(&currentURL))
	log.Printf("📍 Current URL before cross-domain navigation: %s", currentURL)
	
	// Get additional options for domain info
	var fromDomain, toDomain string
	if step.Options != nil {
		if from, ok := step.Options["from_domain"].(string); ok {
			fromDomain = from
		}
		if to, ok := step.Options["to_domain"].(string); ok {
			toDomain = to
		}
	}
	
	if fromDomain != "" && toDomain != "" {
		log.Printf("🔄 Cross-domain navigation: %s -> %s", fromDomain, toDomain)
	}
	
	// Enhanced cross-domain navigation with multiple retries
	maxRetries := 5 // More retries for cross-domain navigation
	var lastError error
	
	for attempt := 1; attempt <= maxRetries; attempt++ {
		log.Printf("🔄 Cross-domain attempt %d/%d to: %s", attempt, maxRetries, targetURL)
		
		// Progressive timeout for cross-domain navigation
		timeout := time.Duration(15+attempt*10) * time.Second // Longer timeouts for cross-domain
		navCtx, cancel := context.WithTimeout(ctx, timeout)
		
		err := chromedp.Run(navCtx,
			// Enhanced cross-domain navigation sequence
			chromedp.Navigate(targetURL),
			chromedp.Sleep(2*time.Second), // Longer initial wait for cross-domain
			chromedp.WaitReady("body", chromedp.ByQuery),
		)
		
		cancel()
		
		if err != nil {
			lastError = err
			log.Printf("❌ Cross-domain attempt %d failed: %v", attempt, err)
			
			// Progressive backoff for cross-domain retries
			if attempt < maxRetries {
				waitTime := time.Duration(attempt*3) * time.Second // Longer wait between retries
				log.Printf("⏳ Cross-domain retry waiting %v...", waitTime)
				time.Sleep(waitTime)
				continue
			}
		} else {
			// Enhanced verification for cross-domain navigation
			var newURL string
			chromedp.Run(ctx, chromedp.Location(&newURL))
			log.Printf("📍 URL after cross-domain navigation: %s", newURL)
			
			// Extended stabilization for cross-domain pages
			err = te.enhancedCrossDomainStabilization(ctx, targetURL, toDomain)
			if err != nil {
				log.Printf("⚠️ Cross-domain stabilization had issues: %v", err)
			}
			
			log.Printf("✅ Cross-domain navigation complete. Final URL: %s", newURL)
			return nil
		}
	}
	
	return fmt.Errorf("cross-domain navigation failed after %d attempts. Last error: %w", maxRetries, lastError)
}

// executeBack handles browser back navigation
func (te *TestExecutor) executeBack(ctx context.Context, step models.TestStep) error {
	log.Printf("🔙 Executing browser back navigation")
	
	err := chromedp.Run(ctx, chromedp.NavigateBack())
	if err != nil {
		return fmt.Errorf("failed to navigate back: %w", err)
	}
	
	// Wait for page to load after going back
	err = chromedp.Run(ctx,
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
	)
	if err != nil {
		log.Printf("Warning: failed to wait after back navigation: %v", err)
	}
	
	// Get current URL for logging
	var currentURL string
	chromedp.Run(ctx, chromedp.Location(&currentURL))
	log.Printf("✅ Back navigation complete. Current URL: %s", currentURL)
	
	return nil
}

// enhancedPageStabilization provides comprehensive page stabilization after navigation
func (te *TestExecutor) enhancedPageStabilization(ctx context.Context, targetURL string) error {
	log.Printf("🔧 Starting enhanced page stabilization for: %s", targetURL)
	
	// Stage 1: Basic DOM ready check
	err := chromedp.Run(ctx,
		chromedp.WaitReady("html", chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
	)
	if err != nil {
		log.Printf("⚠️ Stage 1 stabilization failed: %v", err)
	}
	
	// Stage 2: Document ready state verification
	var readyState string
	for attempt := 1; attempt <= 5; attempt++ {
		chromedp.Run(ctx, chromedp.Evaluate(`document.readyState`, &readyState))
		log.Printf("🔍 Document ready state (attempt %d): %s", attempt, readyState)
		
		if readyState == "complete" {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	
	// Stage 3: Wait for potential JavaScript frameworks to initialize
	err = chromedp.Run(ctx,
		chromedp.Sleep(2*time.Second), // Allow time for JS frameworks
		chromedp.Evaluate(`
			// Check if common frameworks are initializing
			if (typeof window.jQuery !== 'undefined' && window.jQuery.isReady === false) {
				return 'jquery_loading';
			}
			if (typeof window.Vue !== 'undefined' || typeof window.React !== 'undefined') {
				return 'framework_detected';
			}
			return 'ready';
		`, &readyState),
	)
	
	log.Printf("📊 Page framework status: %s", readyState)
	
	// Stage 4: Final verification
	var pageTitle string
	var currentURL string
	chromedp.Run(ctx, chromedp.Title(&pageTitle))
	chromedp.Run(ctx, chromedp.Location(&currentURL))
	
	log.Printf("✅ Page stabilization complete - Title: '%s', URL: '%s'", pageTitle, currentURL)
	return nil
}

// enhancedCrossDomainStabilization provides extended stabilization for cross-domain navigation
func (te *TestExecutor) enhancedCrossDomainStabilization(ctx context.Context, targetURL, toDomain string) error {
	log.Printf("🌐 Starting enhanced cross-domain stabilization for: %s (domain: %s)", targetURL, toDomain)
	
	// Extended stabilization for cross-domain pages
	err := chromedp.Run(ctx,
		chromedp.Sleep(3*time.Second), // Longer initial wait for cross-domain
		chromedp.WaitReady("html", chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Additional wait for cross-domain resources
	)
	
	if err != nil {
		log.Printf("⚠️ Cross-domain basic stabilization failed: %v", err)
	}
	
	// Verify domain change
	var currentURL string
	chromedp.Run(ctx, chromedp.Location(&currentURL))
	
	if toDomain != "" && !strings.Contains(currentURL, toDomain) {
		log.Printf("⚠️ Expected domain '%s' but current URL is '%s'", toDomain, currentURL)
	}
	
	// Extended document ready verification for cross-domain
	var readyState string
	maxWait := 10 // Wait up to 10 attempts for cross-domain pages
	for attempt := 1; attempt <= maxWait; attempt++ {
		chromedp.Run(ctx, chromedp.Evaluate(`document.readyState`, &readyState))
		log.Printf("🔍 Cross-domain ready state (attempt %d/%d): %s", attempt, maxWait, readyState)
		
		if readyState == "complete" {
			break
		}
		time.Sleep(time.Duration(attempt) * 500 * time.Millisecond) // Progressive wait
	}
	
	// Additional stabilization for potential cross-domain security/loading delays
	err = chromedp.Run(ctx,
		chromedp.Sleep(4*time.Second), // Extended wait for cross-domain content
		chromedp.Evaluate(`
			// Check for common cross-domain loading indicators
			var loadingElements = document.querySelectorAll('.loading, .spinner, [data-loading="true"]');
			return loadingElements.length === 0 ? 'no_loading' : 'loading_detected';
		`, &readyState),
	)
	
	log.Printf("📊 Cross-domain loading status: %s", readyState)
	
	var pageTitle string
	chromedp.Run(ctx, chromedp.Title(&pageTitle))
	chromedp.Run(ctx, chromedp.Location(&currentURL))
	
	log.Printf("✅ Cross-domain stabilization complete - Title: '%s', Final URL: '%s'", pageTitle, currentURL)
	return nil
}

// executeBeforeunload handles beforeunload events (page about to unload)
func (te *TestExecutor) executeBeforeunload(ctx context.Context, step models.TestStep) error {
	log.Printf("⚠️ Processing beforeunload event - page is about to navigate away")
	
	// beforeunload is typically a notification that navigation is about to happen
	// In test execution, we just acknowledge it and prepare for potential navigation
	
	// Add a small wait to simulate the brief moment before navigation
	err := chromedp.Run(ctx, chromedp.Sleep(200*time.Millisecond))
	if err != nil {
		log.Printf("Warning: failed to wait during beforeunload: %v", err)
	}
	
	log.Printf("✅ Beforeunload event processed - ready for navigation")
	return nil
}

// executePopstate handles popstate events (browser history navigation)
func (te *TestExecutor) executePopstate(ctx context.Context, step models.TestStep) error {
	log.Printf("🔙 Processing popstate event - browser history navigation")
	
	// popstate events are triggered by back/forward navigation
	// In test execution, we don't need to trigger it - it's informational
	
	// Wait for the popstate navigation to complete
	err := chromedp.Run(ctx,
		chromedp.Sleep(1*time.Second),
		chromedp.WaitReady("body", chromedp.ByQuery),
	)
	
	if err != nil {
		log.Printf("Warning: popstate stabilization failed: %v", err)
	}
	
	// Get current URL after popstate
	var currentURL string
	chromedp.Run(ctx, chromedp.Location(&currentURL))
	log.Printf("✅ Popstate event processed - Current URL: %s", currentURL)
	
	return nil
}

// executeHashchange handles hashchange events (URL hash changes)
func (te *TestExecutor) executeHashchange(ctx context.Context, step models.TestStep) error {
	log.Printf("🔗 Processing hashchange event")
	
	targetHash := step.Value
	if targetHash != "" {
		log.Printf("🎯 Hash changing to: %s", targetHash)
		
		// If we have a target hash, navigate to it
		var currentURL string
		chromedp.Run(ctx, chromedp.Location(&currentURL))
		
		// Remove existing hash and add new one
		baseURL := strings.Split(currentURL, "#")[0]
		newURL := baseURL + "#" + targetHash
		
		err := chromedp.Run(ctx,
			chromedp.Navigate(newURL),
			chromedp.Sleep(500*time.Millisecond), // Brief wait for hash change
		)
		
		if err != nil {
			return fmt.Errorf("failed to navigate to hash %s: %w", targetHash, err)
		}
		
		log.Printf("✅ Hash navigation complete: %s", newURL)
	} else {
		// Just acknowledge the hash change
		err := chromedp.Run(ctx, chromedp.Sleep(200*time.Millisecond))
		if err != nil {
			log.Printf("Warning: failed to wait during hashchange: %v", err)
		}
		
		log.Printf("✅ Hashchange event processed")
	}
	
	return nil
}
