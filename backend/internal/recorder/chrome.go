package recorder

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/gorilla/websocket"
	"webtestflow/backend/pkg/chrome"
)

type ChromeRecorder struct {
	ctx        context.Context
	cancel     context.CancelFunc
	isRecording bool
	steps      []RecordStep
	mutex      sync.RWMutex
	wsConn     *websocket.Conn
	deviceInfo DeviceInfo
	sessionID  string
	targetURL  string
}

type DeviceInfo struct {
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	UserAgent string `json:"user_agent"`
}

type RecordStep struct {
	Type        string                 `json:"type"`
	Selector    string                 `json:"selector"`
	Value       string                 `json:"value"`
	Coordinates map[string]interface{} `json:"coordinates"`
	Options     map[string]interface{} `json:"options"`
	Timestamp   int64                  `json:"timestamp"`
	Screenshot  string                 `json:"screenshot"`
}

type RecorderManager struct {
	recorders map[string]*ChromeRecorder
	mutex     sync.RWMutex
}

var Manager = &RecorderManager{
	recorders: make(map[string]*ChromeRecorder),
}

func NewChromeRecorder(sessionID string, device DeviceInfo) *ChromeRecorder {
	return &ChromeRecorder{
		isRecording: false,
		steps:       make([]RecordStep, 0),
		deviceInfo:  device,
		sessionID:   sessionID,
	}
}

func (r *ChromeRecorder) StartRecording(targetURL string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.isRecording {
		return fmt.Errorf("recording is already in progress")
	}

	// Check if Chrome is available
	chromePath := chrome.GetChromePath()
	if chromePath == "" {
		// Try flatpak Chrome
		chromePath = chrome.GetFlatpakChromePath()
		if chromePath == "" {
			return fmt.Errorf("Chrome browser not found. Please install Google Chrome or Chromium")
		}
	}

	// Create Chrome context with device emulation
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(chromePath),
		chromedp.Flag("headless", false),
		chromedp.Flag("disable-web-security", true),
		chromedp.Flag("disable-features", "VizDisplayCompositor"),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-plugins", true),
		chromedp.Flag("disable-images", false),
		chromedp.Flag("disable-background-timer-throttling", true),
		chromedp.Flag("disable-backgrounding-occluded-windows", true),
		chromedp.Flag("disable-renderer-backgrounding", true),
		chromedp.Flag("ignore-certificate-errors", true),
		chromedp.Flag("ignore-ssl-errors", true),
		chromedp.Flag("ignore-certificate-errors-spki-list", true),
		chromedp.Flag("ignore-ssl-errors-spki-list", true),
		chromedp.Flag("allow-running-insecure-content", true),
		chromedp.Flag("disable-ipc-flooding-protection", true),
		chromedp.Flag("excludeSwitches", "enable-automation"),
		chromedp.Flag("useAutomationExtension", false),
		// Fix cookie parsing issues
		chromedp.Flag("disable-cookie-encryption", true),
		chromedp.Flag("disable-java", true),
		chromedp.Flag("no-first-run", true),
		// Add flags to help with browser process cleanup and force closure
		chromedp.Flag("force-device-scale-factor", "1"),
		chromedp.Flag("aggressive-cache-discard", true),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-component-update", true),
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("no-pings", true),
		chromedp.Flag("no-crash-upload", true),
		// Don't set window size here - we'll use device emulation instead
		chromedp.UserAgent(r.deviceInfo.UserAgent),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	var ctxCancel context.CancelFunc
	r.ctx, ctxCancel = chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
	
	// Create a custom cancel function that closes browser first
	r.cancel = func() {
		r.closeBrowser()
		ctxCancel()
		allocCancel()
	}

	// Navigate to target URL and setup device emulation
	err := chromedp.Run(r.ctx,
		// Enable device emulation using DevTools (equivalent to Ctrl+Shift+M)
		chromedp.EmulateViewport(int64(r.deviceInfo.Width), int64(r.deviceInfo.Height)),
		chromedp.Navigate(targetURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // Wait for dynamic content to load
		chromedp.Evaluate(getRecordingScript(), nil),
	)

	if err != nil {
		r.cancel()
		return fmt.Errorf("failed to start recording: %w", err)
	}

	r.isRecording = true
	r.steps = make([]RecordStep, 0)

	// Start listening for events
	go r.listenForEvents()

	return nil
}

func (r *ChromeRecorder) StopRecording() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if !r.isRecording {
		return fmt.Errorf("no recording in progress")
	}

	if r.cancel != nil {
		r.cancel()
	}

	r.isRecording = false
	return nil
}

func (r *ChromeRecorder) GetSteps() []RecordStep {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	return append([]RecordStep(nil), r.steps...)
}

func (r *ChromeRecorder) IsRecording() bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	return r.isRecording
}

func (r *ChromeRecorder) listenForEvents() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			if !r.isRecording {
				return
			}

			var events []RecordStep
			err := chromedp.Run(r.ctx,
				chromedp.Evaluate(`window.autoUIRecorder && window.autoUIRecorder.getEvents()`, &events),
			)

			if err != nil {
				log.Printf("Error getting events: %v", err)
				continue
			}

			if len(events) > 0 {
				r.mutex.Lock()
				r.steps = append(r.steps, events...)
				r.mutex.Unlock()

				// Send events via WebSocket if connected
				if r.wsConn != nil {
					for _, event := range events {
						eventData, _ := json.Marshal(event)
						r.wsConn.WriteMessage(websocket.TextMessage, eventData)
					}
				}
			}
		}
	}
}

func (r *ChromeRecorder) SetWebSocketConnection(conn *websocket.Conn) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.wsConn = conn
}

func (rm *RecorderManager) StartRecording(sessionID, targetURL string, device DeviceInfo) error {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	if _, exists := rm.recorders[sessionID]; exists {
		return fmt.Errorf("recording session %s already exists", sessionID)
	}

	recorder := NewChromeRecorder(sessionID, device)
	err := recorder.StartRecording(targetURL)
	if err != nil {
		return err
	}

	rm.recorders[sessionID] = recorder
	return nil
}

func (rm *RecorderManager) StopRecording(sessionID string) error {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	recorder, exists := rm.recorders[sessionID]
	if !exists {
		return fmt.Errorf("recording session %s not found", sessionID)
	}

	err := recorder.StopRecording()
	if err != nil {
		return err
	}

	// Don't delete the session here - keep it for saving
	// The session will be cleaned up when saving is complete
	return nil
}

func (rm *RecorderManager) GetRecorder(sessionID string) (*ChromeRecorder, bool) {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	recorder, exists := rm.recorders[sessionID]
	return recorder, exists
}

func (rm *RecorderManager) GetRecordingStatus(sessionID string) (bool, []RecordStep, error) {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	recorder, exists := rm.recorders[sessionID]
	if !exists {
		return false, nil, fmt.Errorf("recording session %s not found", sessionID)
	}

	return recorder.IsRecording(), recorder.GetSteps(), nil
}

func (rm *RecorderManager) CleanupRecording(sessionID string) error {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	if _, exists := rm.recorders[sessionID]; exists {
		delete(rm.recorders, sessionID)
	}
	return nil
}

func getRecordingScript() string {
	return `
(function() {
	if (window.autoUIRecorder) return;
	
	window.autoUIRecorder = {
		events: [],
		isRecording: true,
		
		addEvent: function(event) {
			if (this.isRecording) {
				this.events.push(event);
			}
		},
		
		getEvents: function() {
			const events = [...this.events];
			this.events = [];
			return events;
		},
		
		getSelector: function(element) {
			if (element.id) {
				return '#' + element.id;
			}
			
			let path = [];
			while (element && element.nodeType === Node.ELEMENT_NODE) {
				let selector = element.nodeName.toLowerCase();
				if (element.className) {
					selector += '.' + element.className.trim().split(/\s+/).join('.');
				}
				path.unshift(selector);
				element = element.parentNode;
			}
			return path.join(' > ');
		},
		
		getCoordinates: function(event) {
			const rect = event.target.getBoundingClientRect();
			return {
				x: event.clientX - rect.left,
				y: event.clientY - rect.top,
				pageX: event.pageX,
				pageY: event.pageY
			};
		}
	};
	
	// Click events
	document.addEventListener('click', function(event) {
		if (event.isTrusted) {
			window.autoUIRecorder.addEvent({
				type: 'click',
				selector: window.autoUIRecorder.getSelector(event.target),
				coordinates: window.autoUIRecorder.getCoordinates(event),
				timestamp: Date.now(),
				options: {
					button: event.button,
					detail: event.detail
				}
			});
		}
	}, true);
	
	// Input events
	document.addEventListener('input', function(event) {
		if (event.isTrusted && event.target.tagName) {
			const tagName = event.target.tagName.toLowerCase();
			if (tagName === 'input' || tagName === 'textarea') {
				window.autoUIRecorder.addEvent({
					type: 'input',
					selector: window.autoUIRecorder.getSelector(event.target),
					value: event.target.value,
					timestamp: Date.now(),
					options: {
						inputType: event.inputType
					}
				});
			}
		}
	}, true);
	
	// Key events
	document.addEventListener('keydown', function(event) {
		if (event.isTrusted) {
			window.autoUIRecorder.addEvent({
				type: 'keydown',
				selector: window.autoUIRecorder.getSelector(event.target),
				value: event.key,
				timestamp: Date.now(),
				options: {
					keyCode: event.keyCode,
					ctrlKey: event.ctrlKey,
					shiftKey: event.shiftKey,
					altKey: event.altKey,
					metaKey: event.metaKey
				}
			});
		}
	}, true);
	
	// Touch events for mobile simulation
	document.addEventListener('touchstart', function(event) {
		if (event.isTrusted) {
			const touch = event.touches[0];
			window.autoUIRecorder.addEvent({
				type: 'touchstart',
				selector: window.autoUIRecorder.getSelector(event.target),
				coordinates: {
					x: touch.clientX,
					y: touch.clientY,
					pageX: touch.pageX,
					pageY: touch.pageY
				},
				timestamp: Date.now(),
				options: {
					touchCount: event.touches.length
				}
			});
		}
	}, true);
	
	document.addEventListener('touchend', function(event) {
		if (event.isTrusted) {
			window.autoUIRecorder.addEvent({
				type: 'touchend',
				selector: window.autoUIRecorder.getSelector(event.target),
				timestamp: Date.now(),
				options: {
					touchCount: event.changedTouches.length
				}
			});
		}
	}, true);
	
	// Scroll events
	document.addEventListener('scroll', function(event) {
		if (event.isTrusted) {
			window.autoUIRecorder.addEvent({
				type: 'scroll',
				selector: window.autoUIRecorder.getSelector(event.target),
				coordinates: {
					scrollX: window.scrollX,
					scrollY: window.scrollY
				},
				timestamp: Date.now()
			});
		}
	}, true);
	
	// Form submission
	document.addEventListener('submit', function(event) {
		if (event.isTrusted) {
			window.autoUIRecorder.addEvent({
				type: 'submit',
				selector: window.autoUIRecorder.getSelector(event.target),
				timestamp: Date.now()
			});
		}
	}, true);
	
	// Select changes
	document.addEventListener('change', function(event) {
		if (event.isTrusted && event.target.tagName) {
			const tagName = event.target.tagName.toLowerCase();
			if (tagName === 'select' || tagName === 'input') {
				window.autoUIRecorder.addEvent({
					type: 'change',
					selector: window.autoUIRecorder.getSelector(event.target),
					value: event.target.value,
					timestamp: Date.now(),
					options: {
						type: event.target.type
					}
				});
			}
		}
	}, true);
	
	console.log('AutoUI Recorder initialized');
})();
`
}

// closeBrowser forcefully closes the entire Chrome browser process
func (r *ChromeRecorder) closeBrowser() {
	if r.ctx == nil {
		return
	}
	
	log.Printf("Attempting to close Chrome recording browser completely...")
	
	// Method 1: Try to close the entire browser using JavaScript
	err := chromedp.Run(r.ctx, chromedp.Evaluate(`
		try {
			// Close all windows and exit the browser entirely
			if (window.chrome && window.chrome.runtime) {
				window.chrome.runtime.exit();
			} else if (window.external && window.external.msIsSiteMode) {
				window.external.msIsSiteMode();
			} else {
				// Force close by calling window.close multiple times
				for (let i = 0; i < 10; i++) {
					setTimeout(() => {
						try { 
							window.close(); 
							if (window.parent) window.parent.close();
						} catch(e) {}
					}, i * 100);
				}
			}
		} catch(e) {
			console.log('Recording browser close attempt failed:', e);
		}
	`, nil))
	
	if err != nil {
		log.Printf("JavaScript recording browser close failed: %v", err)
	}
	
	// Method 2: Give a brief moment for graceful close, then force terminate
	time.Sleep(500 * time.Millisecond)
	
	log.Printf("Chrome recording browser close sequence initiated - context will be cancelled to force process termination")
	
	// Method 3: Force terminate Chrome processes as last resort
	go func() {
		time.Sleep(2 * time.Second) // Give graceful close some time
		forceKillChromeProcesses()
	}()
}

// forceKillChromeProcesses terminates all Chrome processes related to automation
func forceKillChromeProcesses() {
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
			log.Printf("Force killed Chrome processes with automation flags on Linux")
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

