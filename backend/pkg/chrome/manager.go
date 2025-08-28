package chrome

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"
)

// ChromeManager manages Chrome instances to avoid ChromeDP v0.9.2 concurrency issues
type ChromeManager struct {
	mutex          sync.Mutex
	processes      map[string]*ChromeProcess
	visualInstance *ChromeProcess // Shared instance for visual executions
}

type ChromeProcess struct {
	Command *exec.Cmd
	Port    int
	PID     int
}

var GlobalChromeManager = &ChromeManager{
	processes: make(map[string]*ChromeProcess),
}

// StartChrome starts a new Chrome instance and returns the debugging port
func (cm *ChromeManager) StartChrome(executionID uint, isVisual bool) (int, error) {
	return cm.StartChromeWithURL(executionID, isVisual, "")
}

// StartChromeWithURL starts a new Chrome instance with optional target URL and returns the debugging port
func (cm *ChromeManager) StartChromeWithURL(executionID uint, isVisual bool, targetURL string) (int, error) {
	return cm.StartChromeWithDevice(executionID, isVisual, targetURL, "")
}

// StartChromeWithDevice starts a new Chrome instance with device-specific configuration
func (cm *ChromeManager) StartChromeWithDevice(executionID uint, isVisual bool, targetURL string, deviceName string) (int, error) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	key := fmt.Sprintf("exec-%d", executionID)

	// Find available port
	port := cm.findAvailablePort()
	if port == 0 {
		return 0, fmt.Errorf("no available port found")
	}

	// Get Chrome path
	chromePath := GetChromePath()
	if chromePath == "" {
		chromePath = GetFlatpakChromePath()
		if chromePath == "" {
			return 0, fmt.Errorf("Chrome not found")
		}
	}

	// Chrome arguments - base arguments
	args := []string{
		"--remote-debugging-port=" + strconv.Itoa(port),
		"--user-data-dir=" + fmt.Sprintf("/tmp/chrome-data-%d", executionID),
		"--enable-features=OverlayScrollbar",
		"--disable-web-security",
		"--disable-features=VizDisplayCompositor",
	}

	// Add device-specific arguments
	if deviceName != "" {
		if deviceManager := GetDeviceManager(); deviceManager != nil {
			if deviceInfo, err := deviceManager.GetDevice(deviceName); err == nil {
				log.Printf("🎭 Configuring Chrome startup for device: %s (%dx%d, mobile=%t, DPR=%.1f)", 
					deviceInfo.Name, deviceInfo.Width, deviceInfo.Height, deviceInfo.Mobile, deviceInfo.DevicePixelRatio)
				
				if deviceInfo.Mobile {
					// Mobile device arguments
					args = append(args,
						"--user-agent="+deviceInfo.UserAgent,
						"--touch-events=enabled",
						"--enable-viewport-meta",
						fmt.Sprintf("--window-size=%d,%d", deviceInfo.Width, deviceInfo.Height),
						fmt.Sprintf("--force-device-scale-factor=%.1f", deviceInfo.DevicePixelRatio),
					)
				} else {
					// Desktop device arguments - ensure proper desktop rendering
					args = append(args,
						"--user-agent="+deviceInfo.UserAgent,
						"--touch-events=disabled",
						fmt.Sprintf("--window-size=%d,%d", deviceInfo.Width, deviceInfo.Height),
						fmt.Sprintf("--force-device-scale-factor=%.1f", deviceInfo.DevicePixelRatio),
						// Additional desktop-specific flags to prevent mobile-like rendering
						"--disable-features=VizDisplayCompositor",
						"--disable-viewport-meta",
						"--enable-desktop-site",
						fmt.Sprintf("--force-screen-size=%d,%d", deviceInfo.Width, deviceInfo.Height),
					)
				}
			} else {
				log.Printf("⚠️ Device '%s' not found, using default desktop configuration", deviceName)
				// Default desktop configuration
				args = append(args,
					"--user-agent=Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
					"--touch-events=disabled",
					"--window-size=1920,1080",
					"--disable-features=VizDisplayCompositor",
					"--disable-viewport-meta",
					"--enable-desktop-site",
					"--force-screen-size=1920,1080",
				)
			}
		} else {
			log.Printf("⚠️ Device manager not available, using default desktop configuration")
			// Default desktop configuration  
			args = append(args,
				"--user-agent=Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
				"--touch-events=disabled",
				"--window-size=1920,1080",
				"--disable-features=VizDisplayCompositor",
				"--disable-viewport-meta",
				"--enable-desktop-site",
				"--force-screen-size=1920,1080",
			)
		}
	} else {
		// No device specified, use default mobile configuration for backward compatibility
		args = append(args,
			"--user-agent=Mozilla/5.0 (iPhone; CPU iPhone OS 15_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.0 Mobile/15E148 Safari/604.1",
			"--touch-events=enabled",
			"--enable-viewport-meta",
			"--window-size=390,844",
		)
	}

	if !isVisual {
		args = append(args, "--headless")
	} else {
		// For visual mode, start maximized for better user experience
		args = append(args,
			"--start-minimized", // Start minimized to reduce visual impact
		)
	}

	// Add target URL if provided
	if targetURL != "" {
		args = append(args, targetURL)
		log.Printf("🚀 Starting Chrome for execution %d on port %d with target URL: %s", executionID, port, targetURL)
	} else {
		log.Printf("🚀 Starting Chrome for execution %d on port %d", executionID, port)
	}

	cmd := exec.Command(chromePath, args...)
	cmd.Stderr = nil // Suppress Chrome error output
	cmd.Stdout = nil

	// Start Chrome process
	log.Printf("📋 Executing Chrome command: %s %v", chromePath, args)
	if err := cmd.Start(); err != nil {
		log.Printf("❌ Failed to start Chrome process: %v", err)
		return 0, fmt.Errorf("failed to start Chrome: %v", err)
	}

	process := &ChromeProcess{
		Command: cmd,
		Port:    port,
		PID:     cmd.Process.Pid,
	}

	cm.processes[key] = process

	// For visual executions, also store as shared visual instance
	if isVisual {
		cm.visualInstance = process
		log.Printf("📝 Chrome process registered as visual instance: PID=%d, Port=%d", process.PID, port)
	} else {
		log.Printf("📝 Chrome process registered: PID=%d, Port=%d", process.PID, port)
	}

	// Give Chrome time to start up
	log.Printf("⏳ Waiting 2 seconds for Chrome to initialize...")
	time.Sleep(2 * time.Second)

	// 检查进程是否仍在运行
	if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
		log.Printf("❌ Chrome process exited unexpectedly: %s", cmd.ProcessState.String())
		return 0, fmt.Errorf("Chrome process exited unexpectedly")
	}

	// Wait for Chrome to be ready with dynamic detection
	if err := cm.waitForChromeReady(port, 15*time.Second); err != nil {
		// Cleanup on failure
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		delete(cm.processes, key)
		return 0, fmt.Errorf("Chrome failed to start properly: %v", err)
	}

	log.Printf("✅ Chrome started successfully for execution %d (PID: %d, Port: %d)", executionID, process.PID, port)

	return port, nil
}

// waitForChromeReady waits for Chrome to be ready by checking the debugging endpoint
func (cm *ChromeManager) waitForChromeReady(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	debugURL := fmt.Sprintf("http://localhost:%d/json", port)

	log.Printf("⏳ Waiting for Chrome to be ready on port %d...", port)

	for time.Now().Before(deadline) {
		resp, err := http.Get(debugURL)
		if err == nil {
			resp.Body.Close()
			log.Printf("✅ Chrome debugging endpoint is ready on port %d", port)
			return nil
		}
		time.Sleep(200 * time.Millisecond) // Check every 200ms
	}

	return fmt.Errorf("Chrome debugging endpoint not ready within %v", timeout)
}

// StopChrome stops the Chrome instance for the given execution
func (cm *ChromeManager) StopChrome(executionID uint) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	key := fmt.Sprintf("exec-%d", executionID)
	process, exists := cm.processes[key]

	if !exists {
		return
	}

	// Check if this is the visual instance - if so, keep it alive for reuse
	if cm.visualInstance != nil && process == cm.visualInstance {
		log.Printf("🔄 Keeping visual Chrome instance alive for execution %d (PID: %d)", executionID, process.PID)
		// Remove from processes map but keep visualInstance reference
		delete(cm.processes, key)
		return
	}

	log.Printf("🛑 Stopping Chrome for execution %d (PID: %d)", executionID, process.PID)

	if process.Command.Process != nil {
		// First try graceful termination for visual executions
		log.Printf("🔄 Attempting graceful Chrome termination for process %d", process.PID)

		// Send SIGTERM for graceful shutdown (allows Chrome to close tabs properly)
		err := process.Command.Process.Signal(os.Interrupt)
		if err != nil {
			log.Printf("⚠️ Failed to send SIGTERM to Chrome process %d: %v", process.PID, err)
		} else {
			// Wait up to 3 seconds for graceful shutdown
			done := make(chan error, 1)
			go func() {
				done <- process.Command.Wait()
			}()

			select {
			case err := <-done:
				if err != nil {
					log.Printf("Chrome process %d ended with error: %v", process.PID, err)
				} else {
					log.Printf("✅ Chrome process %d terminated gracefully", process.PID)
				}
			case <-time.After(3 * time.Second):
				// If graceful shutdown didn't work, force kill
				log.Printf("🔨 Graceful shutdown timeout, force killing Chrome process %d", process.PID)
				killErr := process.Command.Process.Kill()
				if killErr != nil {
					log.Printf("⚠️ Failed to force kill Chrome process %d: %v", process.PID, killErr)
				} else {
					process.Command.Wait()
					log.Printf("✅ Chrome process %d force terminated", process.PID)
				}
			}
		}
	}

	// Cleanup user data directory
	userDataDir := fmt.Sprintf("/tmp/chrome-data-%d", executionID)
	if err := os.RemoveAll(userDataDir); err != nil {
		log.Printf("⚠️ Failed to cleanup user data dir for execution %d: %v", executionID, err)
	}

	delete(cm.processes, key)
	log.Printf("🧹 Cleanup completed for Chrome execution %d", executionID)
}

// findAvailablePort finds an available port for Chrome debugging
func (cm *ChromeManager) findAvailablePort() int {
	usedPorts := make(map[int]bool)
	for _, process := range cm.processes {
		usedPorts[process.Port] = true
	}

	// Try ports from 9222 to 9322
	for port := 9222; port <= 9322; port++ {
		if !usedPorts[port] {
			return port
		}
	}

	return 0
}

// GetDebugURL returns the Chrome debugging URL for the given execution
func (cm *ChromeManager) GetDebugURL(executionID uint) string {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	key := fmt.Sprintf("exec-%d", executionID)
	if process, exists := cm.processes[key]; exists {
		return fmt.Sprintf("http://localhost:%d", process.Port)
	}

	return ""
}

// CleanupAll stops all Chrome instances (for shutdown)
func (cm *ChromeManager) CleanupAll() {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	totalProcesses := len(cm.processes)
	if cm.visualInstance != nil {
		totalProcesses++
	}

	log.Printf("🧹 Cleaning up all Chrome instances (%d total)", totalProcesses)

	// Clean up regular processes
	for key, process := range cm.processes {
		if process.Command.Process != nil {
			log.Printf("🛑 Stopping Chrome process %s (PID: %d)", key, process.PID)
			process.Command.Process.Kill()
		}
	}

	// Clean up visual instance
	if cm.visualInstance != nil {
		if cm.visualInstance.Command.Process != nil {
			log.Printf("🛑 Stopping visual Chrome instance (PID: %d)", cm.visualInstance.PID)
			cm.visualInstance.Command.Process.Kill()
		}
		cm.visualInstance = nil
	}

	cm.processes = make(map[string]*ChromeProcess)
}

// KeepChromeAlive marks a Chrome instance to be kept alive
func (cm *ChromeManager) KeepChromeAlive(executionID uint) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	key := fmt.Sprintf("exec-%d", executionID)
	if _, exists := cm.processes[key]; exists {
		log.Printf("✅ Marking Chrome process for execution %d to be kept alive", executionID)
	}
}

// GetExistingPort returns an existing Chrome port for visual executions if available
func (cm *ChromeManager) GetExistingPort(executionID uint, isVisual bool) int {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// For visual executions, try to reuse the shared visual instance
	if isVisual && cm.visualInstance != nil {
		// Check if the visual instance is still running and responsive
		if cm.visualInstance.Command != nil && cm.visualInstance.Command.ProcessState == nil {
			// Additional check: verify the debugging endpoint is still responsive
			if cm.isPortResponsive(cm.visualInstance.Port) {
				log.Printf("🔄 Found existing visual Chrome instance on port %d", cm.visualInstance.Port)
				return cm.visualInstance.Port
			} else {
				log.Printf("🧹 Visual Chrome instance port %d is not responsive, cleaning up", cm.visualInstance.Port)
				cm.visualInstance = nil
			}
		} else {
			// Clean up dead visual instance
			log.Printf("🧹 Cleaning up dead visual Chrome instance")
			cm.visualInstance = nil
		}
	}

	return 0
}

// isPortResponsive checks if a Chrome debugging port is responsive
func (cm *ChromeManager) isPortResponsive(port int) bool {
	debugURL := fmt.Sprintf("http://localhost:%d/json/version", port)
	client := &http.Client{Timeout: 1 * time.Second}
	resp, err := client.Get(debugURL)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}

// StopVisualInstance forcefully stops the shared visual Chrome instance
func (cm *ChromeManager) StopVisualInstance() {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if cm.visualInstance == nil {
		return
	}

	log.Printf("🛑 Forcefully stopping visual Chrome instance (PID: %d)", cm.visualInstance.PID)

	if cm.visualInstance.Command.Process != nil {
		// Force kill the visual instance
		killErr := cm.visualInstance.Command.Process.Kill()
		if killErr != nil {
			log.Printf("⚠️ Failed to force kill visual Chrome process %d: %v", cm.visualInstance.PID, killErr)
		} else {
			cm.visualInstance.Command.Wait()
			log.Printf("✅ Visual Chrome process %d force terminated", cm.visualInstance.PID)
		}
	}

	cm.visualInstance = nil
	log.Printf("🧹 Visual Chrome instance cleanup completed")
}
