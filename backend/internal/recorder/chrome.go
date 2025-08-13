package recorder

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/page"
	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/gorilla/websocket"
	"webtestflow/backend/pkg/chrome"
)

type ChromeRecorder struct {
	ctx          context.Context
	cancel       context.CancelFunc
	isRecording  bool
	steps        []RecordStep
	mutex        sync.RWMutex
	wsConn       *websocket.Conn
	deviceInfo   DeviceInfo
	sessionID    string
	targetURL    string
	lastReinject time.Time // 防止重复重新注入
}

// DeviceInfo uses the chrome package's DeviceInfo
type DeviceInfo = chrome.DeviceInfo

type RecordStep struct {
	Type         string                 `json:"type"`
	Selector     string                 `json:"selector"`
	Value        string                 `json:"value"`
	Coordinates  map[string]interface{} `json:"coordinates"`
	Options      map[string]interface{} `json:"options"`
	Timestamp    int64                  `json:"timestamp"`
	Screenshot   string                 `json:"screenshot"`
	ChromedpCode string                 `json:"chromedpCode"` // Generated ChromeDP Go code
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

	// Get actual device configuration
	device, err := chrome.GetDeviceByName(r.deviceInfo.Name)
	if err != nil {
		log.Printf("⚠️ Device configuration error: %v, using default", err)
		device = chrome.PredefinedDevices["iPhone 12 Pro"]
	}

	r.deviceInfo = device // Update with complete device info

	// Create Chrome context with device emulation enabled
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(chromePath),
		chromedp.Flag("headless", false),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-web-security", true),
		chromedp.Flag("disable-features", "VizDisplayCompositor"),
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
		chromedp.Flag("allow-running-insecure-content", true),
		chromedp.Flag("disable-cookie-encryption", true),
		chromedp.Flag("disable-java", true),
		chromedp.Flag("no-first-run", true),
	)

	// Add device-specific Chrome flags
	deviceArgs := chrome.CreateDeviceEmulationChrome(device)
	for _, arg := range deviceArgs {
		if strings.Contains(arg, "=") {
			parts := strings.SplitN(arg, "=", 2)
			opts = append(opts, chromedp.Flag(strings.TrimPrefix(parts[0], "--"), parts[1]))
		}
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	var ctxCancel context.CancelFunc
	r.ctx, ctxCancel = chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))

	// Create a custom cancel function that closes browser first
	r.cancel = func() {
		r.closeBrowser()
		ctxCancel()
		allocCancel()
	}

	// Navigate to target URL and apply ChromeDP device emulation
	err = chromedp.Run(r.ctx,
		// Step 1: Apply ChromeDP's built-in device emulation
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("🎭 Applying ChromeDP device emulation for: %s", device.Name)
			return chrome.ApplyChromeDPDeviceEmulation(ctx, device.Name)
		}),

		// Step 2: Navigate to target URL
		chromedp.Navigate(targetURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(2*time.Second), // Wait for page to load

		// Step 3: Verify device emulation and inject recording script
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("✅ ChromeDP device emulation active: %s", device.Name)
			return nil
		}),
		chromedp.Evaluate(getRecordingScript(), nil),
	)

	if err != nil {
		r.cancel()
		return fmt.Errorf("failed to start recording: %w", err)
	}

	// Set up Chrome DevTools Protocol event listeners for enhanced cross-domain detection
	r.setupNavigationListeners()

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

	// Get any remaining events before stopping
	if r.ctx != nil {
		var allEvents []RecordStep
		err := chromedp.Run(r.ctx,
			chromedp.Evaluate(`window.autoUIRecorder && window.autoUIRecorder.getAllEvents()`, &allEvents),
		)

		if err == nil && len(allEvents) > 0 {
			// Replace steps with all events to ensure we have everything
			r.steps = allEvents
			log.Printf("Retrieved %d total events when stopping recording", len(allEvents))
		}
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

			// First try to get events with enhanced error handling and filtering
			err := chromedp.Run(r.ctx,
				chromedp.Evaluate(`
					(function() {
						try {
							if (window.autoUIRecorder && typeof window.autoUIRecorder.getEvents === 'function') {
								const events = window.autoUIRecorder.getEvents();
								console.log('Raw events before filtering:', events.length);
								
								// Ultra-aggressive multi-layer filter to completely block cookiePart
								const filteredEvents = [];
								
								for (const event of events) {
									try {
										// Skip null/undefined events immediately
										if (!event || typeof event !== 'object') {
											continue;
										}
										
										// Skip if event type is missing or invalid
										if (!event.type || typeof event.type !== 'string') {
											continue;
										}
										
										// Multi-layer deep inspection for problematic content
										function deepInspectForProblems(obj, depth = 0, path = '') {
											if (depth > 5 || !obj) return false; // Prevent infinite recursion
											
											try {
												// Check if this is a string and contains problematic content
												if (typeof obj === 'string') {
													const lowerStr = obj.toLowerCase();
													if (lowerStr.includes('cookiepart') ||
														lowerStr.includes('navigationreason') ||
														lowerStr.includes('clientnavigation') ||
														lowerStr.includes('initialframe') ||
														lowerStr.includes('framenavigation')) {
														console.warn('Found problematic string at path:', path, 'value:', obj.substring(0, 100));
														return true;
													}
												}
												
												// For objects, check all properties recursively
												if (typeof obj === 'object' && obj !== null) {
													for (const [key, value] of Object.entries(obj)) {
														// Check property name itself
														if (typeof key === 'string') {
															const lowerKey = key.toLowerCase();
															if (lowerKey.includes('cookiepart') ||
																lowerKey.includes('cookie') ||
																lowerKey.includes('navigation') ||
																lowerKey.includes('client')) {
																console.warn('Found problematic property name:', key);
																return true;
															}
														}
														
														// Recursively check property value
														if (deepInspectForProblems(value, depth + 1, path + '.' + key)) {
															return true;
														}
													}
												}
											} catch (e) {
												console.warn('Error during deep inspection at path:', path);
												return true; // If we can't inspect safely, assume it's problematic
											}
											
											return false;
										}
										
										// Layer 1: Deep object inspection
										if (deepInspectForProblems(event, 0, 'event')) {
											console.warn('Layer 1: Blocking event due to deep inspection failure');
											continue;
										}
										
										// Layer 2: JSON serialization test with error handling
										let eventJson;
										try {
											eventJson = JSON.stringify(event);
											if (!eventJson || eventJson === '{}' || eventJson === 'null' || eventJson.length === 0) {
												console.warn('Layer 2: Event produces empty/null JSON');
												continue;
											}
										} catch (jsonError) {
											console.warn('Layer 2: Event cannot be JSON serialized:', jsonError.message);
											continue;
										}
										
										// Layer 3: String content analysis (case-insensitive)
										const lowerJson = eventJson.toLowerCase();
										const problematicPatterns = [
											'cookiepart',
											'cookie.*part',
											'navigationreason',
											'clientnavigationreason', 
											'initialframenavigation',
											'framenavigation',
											'clientnavigation'
										];
										
										let foundProblematic = false;
										for (const pattern of problematicPatterns) {
											if (lowerJson.includes(pattern)) {
												console.warn('Layer 3: Found problematic pattern:', pattern);
												foundProblematic = true;
												break;
											}
										}
										
										if (foundProblematic) {
											continue;
										}
										
										// Layer 4: Size and safety checks
										if (eventJson.length > 10000) {
											console.warn('Layer 4: Event JSON too large:', eventJson.length, 'chars');
											continue;
										}
										
										// Layer 5: Create completely clean copy with only essential fields
										const cleanEvent = {};
										
										// Only copy safe, essential properties with validation
										if (event.type && typeof event.type === 'string' && event.type.length > 0 && event.type.length < 50) {
											cleanEvent.type = event.type;
										} else {
											console.warn('Layer 5: Invalid event type, skipping');
											continue;
										}
										
										if (event.selector && typeof event.selector === 'string' && event.selector.length < 500) {
											// Extra check: make sure selector doesn't contain problematic content
											if (!event.selector.toLowerCase().includes('cookiepart')) {
												cleanEvent.selector = event.selector;
											}
										}
										
										if (event.value && typeof event.value === 'string' && event.value.length < 1000) {
											// Extra check: make sure value doesn't contain problematic content  
											if (!event.value.toLowerCase().includes('cookiepart')) {
												cleanEvent.value = event.value;
											}
										}
										
										if (event.coordinates && typeof event.coordinates === 'object') {
											cleanEvent.coordinates = {
												x: isFinite(Number(event.coordinates.x)) ? Number(event.coordinates.x) : 0,
												y: isFinite(Number(event.coordinates.y)) ? Number(event.coordinates.y) : 0
											};
										}
										
										if (event.timestamp && isFinite(Number(event.timestamp))) {
											cleanEvent.timestamp = Number(event.timestamp);
										} else {
											cleanEvent.timestamp = Date.now();
										}
										
										if (event.chromedpCode && typeof event.chromedpCode === 'string' && event.chromedpCode.length < 2000) {
											if (!event.chromedpCode.toLowerCase().includes('cookiepart')) {
												cleanEvent.chromedpCode = event.chromedpCode;
											}
										}
										
										// Layer 6: Final validation of clean event
										try {
											const cleanJson = JSON.stringify(cleanEvent);
											if (cleanJson.toLowerCase().includes('cookiepart') ||
												cleanJson.toLowerCase().includes('navigationreason')) {
												console.warn('Layer 6: Clean event still contains problematic content, blocking');
												continue;
											}
											
											// Success! Event passed all 6 layers of filtering
											filteredEvents.push(cleanEvent);
											
										} catch (finalError) {
											console.warn('Layer 6: Failed to validate clean event:', finalError.message);
											continue;
										}
										
									} catch (processingError) {
										console.warn('Error in multi-layer event processing:', processingError.message);
										continue;
									}
								}
								
								console.log('Multi-layer filter: processed', events.length, 'raw events, passed', filteredEvents.length, 'clean events');
								return filteredEvents;
							}
							return [];
						} catch (e) {
							if (console && console.error) console.error('Critical error in enhanced getEvents:', e);
							return [];
						}
					})()
				`, &events),
			)

			if err != nil {
				// Check if it's a JSON parsing error
				if strings.Contains(err.Error(), "parse error") || strings.Contains(err.Error(), "unmarshal") {
					log.Printf("🔧 JSON parse error detected, applying aggressive cleanup: %v", err)
					
					// Aggressive cleanup: clear everything and reset the recorder state
					cleanupErr := chromedp.Run(r.ctx, chromedp.Evaluate(`
						if (window.autoUIRecorder) {
							if (console && console.warn) console.warn('🧹 Aggressive cleanup: clearing all recording state due to persistent JSON errors');
							
							// Clear all events and reset all tracking
							window.autoUIRecorder.events = [];
							window.autoUIRecorder.sentIndex = 0;
							window.autoUIRecorder.recentEvents.clear();
							window.autoUIRecorder.lastScrollTime = 0;
							window.autoUIRecorder.lastScrollTarget = null;
							window.autoUIRecorder.lastScrollX = -1;
							window.autoUIRecorder.lastScrollY = -1;
							window.autoUIRecorder.userIsScrolling = false;
							window.autoUIRecorder.lastUserInteraction = 0;
							
							// Force garbage collection if available
							if (window.gc) {
								try { window.gc(); } catch(e) {}
							}
							
							if (console && console.log) console.log('✅ All recording state cleared, continuing with fresh state');
						}
					`, nil))
					
					if cleanupErr != nil {
						log.Printf("❌ Failed to cleanup events: %v", cleanupErr)
						// If cleanup fails, try to reinject the script completely
						log.Printf("🔄 Attempting complete script re-injection")
						go r.reinjectRecordingScript()
					} else {
						log.Printf("✅ Aggressive cleanup completed successfully")
					}
					continue
				}

				log.Printf("Error getting events (possible page navigation or script missing): %v", err)
				// Enhanced cross-domain and script missing detection
				now := time.Now()
				isScriptMissingError := strings.Contains(err.Error(), "autoUIRecorder") || 
										strings.Contains(err.Error(), "undefined") ||
										strings.Contains(err.Error(), "Cannot read properties") ||
										strings.Contains(err.Error(), "ReferenceError")
				
				isCrossDomainError := strings.Contains(err.Error(), "Cannot access") ||
									  strings.Contains(err.Error(), "cross-origin") ||
									  strings.Contains(err.Error(), "different origin") ||
									  strings.Contains(err.Error(), "SecurityError")
				
				isNavigationError := strings.Contains(err.Error(), "target navigated") ||
									strings.Contains(err.Error(), "page navigated") ||
									strings.Contains(err.Error(), "document unloaded")
				
				// Immediate re-injection for critical scenarios
				shouldForceReinject := isScriptMissingError || isCrossDomainError || isNavigationError
				
				// Dynamic rate limiting based on error type
				var rateLimitTime time.Duration
				if shouldForceReinject {
					rateLimitTime = 500 * time.Millisecond // Very aggressive for critical errors
				} else {
					rateLimitTime = 3 * time.Second // Normal rate for other errors
				}
				
				if now.Sub(r.lastReinject) > rateLimitTime {
					r.lastReinject = now
					log.Printf("🌐 Enhanced re-injection trigger (script missing: %v, cross-domain: %v, navigation: %v)", 
						isScriptMissingError, isCrossDomainError, isNavigationError)
					
					// For cross-domain issues, use enhanced re-injection
					if isCrossDomainError || isNavigationError {
						go r.enhancedCrossDomainReinject()
					} else {
						go r.reinjectRecordingScript()
					}
				} else {
					log.Printf("⏭️ Skipping reinject (too soon, last: %v ago)", now.Sub(r.lastReinject))
				}
				continue
			}

			if len(events) > 0 {
				r.mutex.Lock()
				// Add new events (JavaScript now only returns new events)
				r.steps = append(r.steps, events...)
				r.mutex.Unlock()

				// Send new events via WebSocket if connected
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

// reinjectRecordingScript re-injects the recording script after page navigation
func (r *ChromeRecorder) reinjectRecordingScript() {
	log.Printf("🔄 Re-injecting recording script after page navigation...")

	// Wait a bit for the new page to load, but not too long
	time.Sleep(500 * time.Millisecond)

	// Multiple attempts to handle different page load stages
	maxAttempts := 3
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		log.Printf("🔄 Reinject attempt %d/%d", attempt, maxAttempts)
		
		// First check if recording script is already present and working
		var recorderExists bool
		err := chromedp.Run(r.ctx,
			chromedp.Evaluate(`(function() {
				try {
					return !!(window.autoUIRecorder && typeof window.autoUIRecorder.getEvents === 'function' && window.autoUIRecorder.isRecording);
				} catch (e) {
					return false;
				}
			})()`, &recorderExists),
		)

		if err == nil && recorderExists {
			log.Printf("✅ Recording script already present and working on attempt %d", attempt)
			return
		}

		// Try to inject the script
		err = chromedp.Run(r.ctx,
			// Wait for page to be in a good state
			chromedp.WaitReady("body", chromedp.ByQuery),

			// Check current URL and log it
			chromedp.ActionFunc(func(ctx context.Context) error {
				var currentURL string
				chromedp.Evaluate(`window.location.href`, &currentURL).Do(ctx)
				log.Printf("📍 Current URL during reinject: %s", currentURL)
				return nil
			}),

			// First clear any existing problematic state
			chromedp.Evaluate(`
				try {
					if (window.autoUIRecorder) {
						console.log('🧹 Clearing existing recorder state for reinject');
						delete window.autoUIRecorder;
					}
					if (window.getChromeDPCode) {
						delete window.getChromeDPCode;
					}
					// Force garbage collection if available
					if (window.gc) {
						try { window.gc(); } catch(e) {}
					}
				} catch (e) {
					console.warn('Error clearing existing state during reinject:', e);
				}
			`, nil),

			// Re-inject the recording script with fresh state
			chromedp.Evaluate(getRecordingScript(), nil),
		)

		if err != nil {
			log.Printf("❌ Failed to re-inject recording script (attempt %d): %v", attempt, err)

			// If there's a JavaScript syntax error, don't keep trying
			if strings.Contains(err.Error(), "SyntaxError") {
				log.Printf("🚫 JavaScript syntax error detected, stopping reinject attempts")
				return
			}
			
			// Wait before retrying
			if attempt < maxAttempts {
				log.Printf("⏱️ Waiting 1s before retry...")
				time.Sleep(1 * time.Second)
			}
			continue
		}

		// Verify the injection worked
		log.Printf("✅ Recording script re-injected successfully (attempt %d)", attempt)
		time.Sleep(300 * time.Millisecond)
		
		var verified bool
		chromedp.Run(r.ctx, chromedp.Evaluate(`(function() {
			try {
				return !!(window.autoUIRecorder && 
						 typeof window.autoUIRecorder.getEvents === 'function' &&
						 typeof window.autoUIRecorder.addEvent === 'function' &&
						 window.autoUIRecorder.isRecording === true);
			} catch (e) {
				console.error('Verification failed:', e);
				return false;
			}
		})()`, &verified))

		if verified {
			log.Printf("✅ Recording script injection verified successfully!")
			
			// Log current event count
			var eventCount int
			chromedp.Run(r.ctx, chromedp.Evaluate(`
				(function() {
					try {
						return window.autoUIRecorder ? window.autoUIRecorder.events.length : 0;
					} catch (e) {
						return -1;
					}
				})()
			`, &eventCount))
			log.Printf("📊 Current event count after reinject: %d", eventCount)
			return
		} else {
			log.Printf("⚠️ Recording script injection verification failed (attempt %d)", attempt)
			if attempt < maxAttempts {
				time.Sleep(1 * time.Second)
			}
		}
	}
	
	log.Printf("❌ All reinject attempts failed")
}

// enhancedCrossDomainReinject handles cross-domain navigation more aggressively
func (r *ChromeRecorder) enhancedCrossDomainReinject() {
	log.Printf("🌐 Enhanced cross-domain re-injection started")
	
	// Wait a bit longer for cross-domain navigation to complete
	time.Sleep(1 * time.Second)
	
	maxAttempts := 5
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		log.Printf("🌐 Cross-domain reinject attempt %d/%d", attempt, maxAttempts)
		
		// Step 1: Check current URL and domain
		var currentURL string
		err := chromedp.Run(r.ctx, 
			chromedp.Evaluate(`window.location.href`, &currentURL))
		
		if err != nil {
			log.Printf("❌ Cannot get current URL (attempt %d): %v", attempt, err)
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}
		
		log.Printf("📍 Cross-domain reinject: current URL is %s", currentURL)
		
		// Step 2: Wait for document ready state
		var readyState string
		err = chromedp.Run(r.ctx,
			chromedp.Evaluate(`document.readyState`, &readyState))
		
		if err != nil {
			log.Printf("❌ Cannot check document ready state (attempt %d): %v", attempt, err)
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}
		
		log.Printf("📄 Document ready state: %s", readyState)
		
		// Step 3: Wait for complete load if not ready
		if readyState != "complete" {
			log.Printf("⏳ Waiting for document to complete loading...")
			err = chromedp.Run(r.ctx,
				chromedp.WaitReady("body", chromedp.ByQuery),
				chromedp.Sleep(500*time.Millisecond))
			if err != nil {
				log.Printf("❌ Failed waiting for document ready (attempt %d): %v", attempt, err)
				time.Sleep(time.Duration(attempt) * time.Second)
				continue
			}
		}
		
		// Step 4: Force clear any existing recorder state
		err = chromedp.Run(r.ctx,
			chromedp.Evaluate(`
				try {
					// Remove any existing recorder instances
					if (window.autoUIRecorder) {
						console.log('🧹 Clearing existing cross-domain recorder state');
						delete window.autoUIRecorder;
					}
					if (window.getChromeDPCode) {
						delete window.getChromeDPCode;
					}
					
					// Clear any conflicting event listeners
					if (window.__uiRecorderListeners) {
						console.log('🧹 Removing existing event listeners');
						for (const listener of window.__uiRecorderListeners) {
							try {
								document.removeEventListener(listener.type, listener.handler, true);
							} catch (e) {}
						}
						delete window.__uiRecorderListeners;
					}
					
					console.log('🧹 Cross-domain cleanup completed');
					return true;
				} catch (e) {
					console.warn('Cross-domain cleanup error:', e);
					return false;
				}
			`, nil))
		
		if err != nil {
			log.Printf("❌ Failed to clear cross-domain state (attempt %d): %v", attempt, err)
		}
		
		// Step 5: Inject fresh recording script with cross-domain awareness
		err = chromedp.Run(r.ctx,
			chromedp.Evaluate(getRecordingScript(), nil))
		
		if err != nil {
			log.Printf("❌ Failed to inject cross-domain recording script (attempt %d): %v", attempt, err)
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}
		
		// Step 6: Verify injection worked
		var verified bool
		err = chromedp.Run(r.ctx, 
			chromedp.Evaluate(`
				(function() {
					try {
						const isRecorderReady = !!(window.autoUIRecorder && 
												  typeof window.autoUIRecorder.getEvents === 'function' &&
												  typeof window.autoUIRecorder.addEvent === 'function' &&
												  window.autoUIRecorder.isRecording === true);
						
						if (isRecorderReady) {
							console.log('✅ Cross-domain recorder verification successful');
							console.log('📊 Current domain:', window.location.hostname);
							console.log('📊 Current URL:', window.location.href);
							
							// Mark that this is a cross-domain injection
							window.autoUIRecorder._crossDomainInjected = true;
							window.autoUIRecorder._injectionDomain = window.location.hostname;
							window.autoUIRecorder._injectionTime = Date.now();
						}
						
						return isRecorderReady;
					} catch (e) {
						console.error('Cross-domain verification failed:', e);
						return false;
					}
				})()
			`, &verified))
		
		if err != nil {
			log.Printf("❌ Failed to verify cross-domain injection (attempt %d): %v", attempt, err)
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}
		
		if verified {
			log.Printf("✅ Cross-domain recorder injection successful after %d attempts", attempt)
			log.Printf("🎯 Now monitoring domain: %s", currentURL)
			
			// Log current event count
			var eventCount int
			chromedp.Run(r.ctx, chromedp.Evaluate(`
				(function() {
					try {
						return window.autoUIRecorder ? window.autoUIRecorder.events.length : 0;
					} catch (e) {
						return -1;
					}
				})()
			`, &eventCount))
			log.Printf("📊 Current event count after cross-domain reinject: %d", eventCount)
			return
		}
		
		log.Printf("⚠️ Cross-domain verification failed (attempt %d), retrying...", attempt)
		time.Sleep(time.Duration(attempt) * time.Second)
	}
	
	log.Printf("❌ All cross-domain reinject attempts failed")
}

// setupNavigationListeners sets up Chrome DevTools Protocol listeners for page navigation events
func (r *ChromeRecorder) setupNavigationListeners() {
	log.Printf("🔧 Setting up Chrome DevTools Protocol navigation listeners")
	
	// Enable page domain events
	chromedp.ListenTarget(r.ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *page.EventFrameNavigated:
			if ev.Frame.URL != "" {
				log.Printf("🌐 CDP: Frame navigated to %s", ev.Frame.URL)
				// Trigger immediate script re-injection for frame navigation
				go func() {
					time.Sleep(1 * time.Second) // Wait for navigation to complete
					r.enhancedCrossDomainReinject()
				}()
			}
			
		case *page.EventNavigatedWithinDocument:
			log.Printf("🔄 CDP: Navigation within document to %s", ev.URL)
			// For SPA navigation, also try re-injection
			go func() {
				time.Sleep(500 * time.Millisecond)
				r.reinjectRecordingScript()
			}()
			
		case *page.EventDomContentEventFired:
			log.Printf("📄 CDP: DOM content loaded")
			// DOM loaded, check if script needs re-injection
			go func() {
				time.Sleep(300 * time.Millisecond)
				r.verifyAndReinjectIfNeeded()
			}()
			
		case *page.EventLoadEventFired:
			log.Printf("✅ CDP: Page load complete")
			// Page fully loaded, ensure script is present
			go func() {
				time.Sleep(500 * time.Millisecond)
				r.verifyAndReinjectIfNeeded()
			}()
			
		case *cdpruntime.EventConsoleAPICalled:
			// Monitor console messages for potential script errors
			if len(ev.Args) > 0 {
				for _, arg := range ev.Args {
					if arg.Value != nil {
						message := string(arg.Value)
						if strings.Contains(message, "autoUIRecorder") || 
						   strings.Contains(message, "Cross-domain") {
							log.Printf("🔍 CDP Console: %s", message)
						}
					}
				}
			}
		}
	})
	
	// Enable the page domain to receive events
	err := chromedp.Run(r.ctx,
		page.Enable(),
		cdpruntime.Enable(),
	)
	
	if err != nil {
		log.Printf("❌ Failed to enable CDP event listeners: %v", err)
	} else {
		log.Printf("✅ CDP navigation listeners enabled successfully")
	}
}

// verifyAndReinjectIfNeeded checks if the recording script is present and re-injects if needed
func (r *ChromeRecorder) verifyAndReinjectIfNeeded() {
	var recorderPresent bool
	err := chromedp.Run(r.ctx,
		chromedp.Evaluate(`!!(window.autoUIRecorder && window.autoUIRecorder.isRecording)`, &recorderPresent))
	
	if err != nil || !recorderPresent {
		log.Printf("🔄 CDP: Recorder script missing, triggering re-injection")
		r.enhancedCrossDomainReinject()
	}
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
		sentIndex: 0,
		isRecording: true,
		lastScrollTime: 0,
		lastScrollTarget: null,
		lastScrollX: -1,
		lastScrollY: -1,
		recentEvents: new Map(), // Track recent events to prevent duplicates
		userIsScrolling: false, // Track if user is actively scrolling
		lastUserInteraction: 0, // Timestamp of last user interaction

		// Generate ChromeDP code for an event
		generateChromeDPCode: function(event) {
			switch (event.type) {
				case 'click':
					return 'chromedp.Click("' + event.selector + '")';
					
				case 'input':
					const value = (event.value || '').replace(/"/g, '\\\\"'); // Escape quotes
					return 'chromedp.SendKeys("' + event.selector + '", "' + value + '")';
					
				case 'scroll':
					if (event.options && event.options.isWindow) {
						return 'chromedp.Evaluate("window.scrollTo(' + (event.coordinates.scrollX || 0) + ', ' + (event.coordinates.scrollY || 0) + ')", nil)';
					} else {
						return 'chromedp.Evaluate("document.querySelector(\\"' + event.selector + '\\").scrollTop = ' + (event.coordinates.scrollY || 0) + '", nil)';
					}
					
				case 'swipe':
					const direction = event.value || 'down';
					const distance = event.coordinates.distance || 50;
					return '// Swipe ' + direction + ' (distance: ' + distance + 'px)\\nchromedp.Evaluate("window.scrollBy(0, ' + (direction === 'down' ? distance : -distance) + ')", nil)';
					
				case 'keydown':
					const key = event.value || '';
					if (key === 'Enter') {
						return 'chromedp.KeyEvent("' + event.selector + '", kb.Enter)';
					} else if (key === 'Escape') {
						return 'chromedp.KeyEvent("' + event.selector + '", kb.Escape)';
					} else {
						return 'chromedp.KeyEvent("' + event.selector + '", "' + key + '")';
					}
					
				case 'submit':
					return 'chromedp.Submit("' + event.selector + '")';
					
				case 'change':
					const changeValue = (event.value || '').replace(/"/g, '\\\\"');
					return 'chromedp.SetValue("' + event.selector + '", "' + changeValue + '")';
					
				case 'navigate':
					return 'chromedp.Navigate("' + event.value + '")';
					
				case 'touchstart':
					return '// Touch start at (' + event.coordinates.x + ', ' + event.coordinates.y + ')\\nchromedp.Click("' + event.selector + '")';
					
				case 'mousedrag':
					return '// Mouse drag to (' + event.coordinates.x + ', ' + event.coordinates.y + ')';
					
				default:
					return '// ' + event.type + ': ' + event.selector;
			}
		},
		
		// Mark user interaction
		markUserInteraction: function() {
			this.lastUserInteraction = Date.now();
			this.userIsScrolling = true;
			
			// Clear the flag after 1 second if no more interactions
			clearTimeout(this.scrollTimeout);
			this.scrollTimeout = setTimeout(() => {
				this.userIsScrolling = false;
			}, 1000);
		},
		
		// Check if scroll is likely user-initiated
		isUserInitiatedScroll: function() {
			const now = Date.now();
			// Consider scroll user-initiated if there was user interaction in the last 2 seconds
			return (now - this.lastUserInteraction) < 2000;
		},

		// Check if element should be ignored for recording
		shouldIgnoreElement: function(element) {
			if (!element) return true;
			
			// Ignore resize sensors and other internal components
			const ignoredElements = [
				'uni-resize-sensor',
				'resize-sensor',
				'__vue__',
				'__vnode__'
			];
			
			// Check element tag name
			const tagName = element.tagName ? element.tagName.toLowerCase() : '';
			if (ignoredElements.includes(tagName)) {
				return true;
			}
			
			// Check element classes
			const className = element.className || '';
			if (typeof className === 'string') {
				for (const ignored of ignoredElements) {
					if (className.includes(ignored)) {
						return true;
					}
				}
			}
			
			// Check if element is a resize sensor by checking its parent hierarchy
			let parent = element.parentElement;
			let depth = 0;
			while (parent && depth < 3) {
				const parentTag = parent.tagName ? parent.tagName.toLowerCase() : '';
				const parentClass = parent.className || '';
				
				if (ignoredElements.includes(parentTag) || 
					(typeof parentClass === 'string' && ignoredElements.some(ignored => parentClass.includes(ignored)))) {
					return true;
				}
				
				parent = parent.parentElement;
				depth++;
			}
			
			return false;
		},

		// Check if event is duplicate within recent timeframe
		isDuplicateEvent: function(event) {
			const now = Date.now();
			const eventKey = event.type + '|' + (event.selector || '') + '|' + (event.value || '');
			const lastTime = this.recentEvents.get(eventKey);
			
			// Consider it duplicate if same event happened within 100ms
			if (lastTime && (now - lastTime) < 100) {
				return true;
			}
			
			// Update the timestamp for this event type
			this.recentEvents.set(eventKey, now);
			
			// Clean up old entries (keep only last 1 minute)
			for (const [key, time] of this.recentEvents.entries()) {
				if (now - time > 60000) {
					this.recentEvents.delete(key);
				}
			}
			
			return false;
		},

		addEvent: function(event) {
			if (this.isRecording) {
				try {
					// Pre-filter check: reject problematic events before adding
					if (event && event.type) {
						// Check if event has problematic content
						if (this.hasProblematicContent(event)) {
							console.warn('Blocking problematic event from being added:', event.type);
							return;
						}
						
						// Additional safety: check if event can be safely stringified
						try {
							JSON.stringify(event);
						} catch (e) {
							console.warn('Event cannot be stringified, blocking:', event.type);
							return;
						}
						
						event.chromedpCode = this.generateChromeDPCode(event);
						this.events.push(event);
						console.log('✅ Added event:', event.type, 'selector:', event.selector, 'total events:', this.events.length);
						console.log('ChromeDP code:', event.chromedpCode);
					}
				} catch (e) {
					console.warn('Failed to add event:', e);
				}
			}
		},
		
		getEvents: function() {
			// Return only new events since last call
			const newEvents = this.events.slice(this.sentIndex);
			this.sentIndex = this.events.length;
			
			// Clean and validate events before returning
			return newEvents.map(event => this.cleanEvent(event)).filter(event => event !== null);
		},
		
		// Clean event data to ensure JSON serialization safety
		cleanEvent: function(event) {
			try {
				if (!event || typeof event !== 'object') {
					return null;
				}
				
				// Early rejection: check for cookiePart in original event
				if (this.hasProblematicContent(event)) {
					console.warn('Rejecting event with problematic content before cleaning');
					return null;
				}
				
				// Create a clean copy with only basic, safe properties
				const cleanedEvent = {
					type: this.safeString(event.type, ''),
					selector: this.safeString(event.selector, ''),
					value: this.safeString(event.value, ''),
					coordinates: this.cleanCoordinates(event.coordinates),
					timestamp: this.safeNumber(event.timestamp, Date.now()),
					options: this.cleanOptions(event.options)
				};
				
				// Additional validation - reject events with problematic content
				const eventStr = JSON.stringify(cleanedEvent);
				
				// Check for problematic patterns that cause JSON parse errors
				if (eventStr.includes('cookiePart') || 
					eventStr.includes('NavigationReason') ||
					eventStr.includes('ClientNavigationReason') ||
					eventStr.includes('initialFrameNavigation') ||
					eventStr.includes('frameNavigation') ||
					eventStr.includes('navigationReason') ||
					eventStr.includes('clientNavigation') ||
					eventStr.length > 50000) { // Reject overly large events
					console.warn('Rejecting event with problematic content after cleaning:', cleanedEvent.type);
					return null;
				}
				
				return cleanedEvent;
			} catch (e) {
				console.warn('Failed to clean event, skipping:', e, event?.type || 'unknown');
				return null;
			}
		},
		
		// Check if event has problematic content that causes JSON errors
		hasProblematicContent: function(event) {
			if (!event || typeof event !== 'object') {
				return false;
			}
			
			try {
				// Quick property check
				for (const [key, value] of Object.entries(event)) {
					if (typeof key === 'string' && (key.includes('cookiePart') || key.includes('cookie'))) {
						return true;
					}
					if (typeof value === 'string' && (value.includes('cookiePart') || value.includes('NavigationReason'))) {
						return true;
					}
				}
				
				// Stringify check
				const eventStr = JSON.stringify(event);
				return eventStr.includes('cookiePart') || 
					   eventStr.includes('NavigationReason') ||
					   eventStr.includes('ClientNavigationReason');
			} catch (e) {
				// If we can't stringify, it's likely problematic
				return true;
			}
		},
		
		// Safe string conversion with aggressive filtering
		safeString: function(value, defaultValue) {
			if (value === null || value === undefined) {
				return defaultValue || '';
			}
			
			try {
				let str = String(value);
				
				// Aggressive filtering of problematic patterns
				str = str.replace(/cookiePart[^\s\n\r]*/gi, ''); // Remove cookiePart and following content
				str = str.replace(/cookie.*Part[^\s\n\r]*/gi, ''); // Handle variations
				str = str.replace(/NavigationReason[^\s\n\r]*/gi, '');
				str = str.replace(/ClientNavigationReason[^\s\n\r]*/gi, '');
				str = str.replace(/initialFrameNavigation/gi, '');
				str = str.replace(/frameNavigation/gi, '');
				str = str.replace(/navigationReason/gi, '');
				str = str.replace(/clientNavigation/gi, '');
				
				// Remove any remaining JSON-like structures that might contain these patterns
				str = str.replace(/\{[^{}]*cookiePart[^{}]*\}/gi, '');
				str = str.replace(/\{[^{}]*NavigationReason[^{}]*\}/gi, '');
				
				// Remove control characters that might cause JSON issues
				str = str.replace(/[\x00-\x1F\x7F]/g, '');
				
				// Limit length more aggressively
				if (str.length > 500) {
					str = str.substring(0, 500);
				}
				
				// Final check - if still contains problematic content, return empty
				if (str.includes('cookiePart') || str.includes('NavigationReason')) {
					console.warn('String still contains problematic content after cleaning, returning empty');
					return defaultValue || '';
				}
				
				return str;
			} catch (e) {
				console.warn('Failed to convert to safe string:', value);
				return defaultValue || '';
			}
		},
		
		// Safe number conversion
		safeNumber: function(value, defaultValue) {
			if (typeof value === 'number' && isFinite(value)) {
				return value;
			}
			
			const num = Number(value);
			if (isFinite(num)) {
				return num;
			}
			
			return defaultValue || 0;
		},
		
		// Clean coordinates object
		cleanCoordinates: function(coords) {
			if (!coords || typeof coords !== 'object') return {};
			
			const cleaned = {};
			const validKeys = ['x', 'y', 'pageX', 'pageY', 'startX', 'startY', 'endX', 'endY', 'deltaX', 'deltaY', 'distance'];
			
			for (let key of validKeys) {
				if (key in coords && typeof coords[key] === 'number' && isFinite(coords[key])) {
					cleaned[key] = coords[key];
				}
			}
			
			return cleaned;
		},
		
		// Clean options object
		cleanOptions: function(options) {
			if (!options || typeof options !== 'object') return {};
			
			const cleaned = {};
			const safeKeys = ['button', 'detail', 'tagName', 'elementText', 'inputType', 'keyCode', 
							  'ctrlKey', 'shiftKey', 'altKey', 'metaKey', 'touchCount', 'duration',
							  'direction', 'moves', 'inputType', 'type', 'isWindow', 'scrollHeight',
							  'clientHeight', 'deltaY', 'method', 'action', 'trigger', 'fromURL', 'toURL'];
			
			for (let key of safeKeys) {
				if (key in options) {
					const value = options[key];
					
					// Skip functions, undefined, null, and complex objects
					if (typeof value === 'function' || value === undefined || value === null) {
						continue;
					}
					
					// Handle different value types safely
					if (typeof value === 'string') {
						// Use safeString to filter problematic content
						const cleanStr = this.safeString(value, '');
						if (cleanStr.length > 0 && cleanStr.length <= 500) {
							cleaned[key] = cleanStr;
						}
					} else if (typeof value === 'number' && isFinite(value)) {
						cleaned[key] = value;
					} else if (typeof value === 'boolean') {
						cleaned[key] = value;
					} else if (Array.isArray(value)) {
						// Only include simple arrays with safe content
						const cleanArray = [];
						for (let i = 0; i < Math.min(value.length, 5); i++) {
							const item = value[i];
							if (typeof item === 'string') {
								const cleanItem = this.safeString(item, '');
								if (cleanItem.length > 0 && cleanItem.length <= 100) {
									cleanArray.push(cleanItem);
								}
							} else if (typeof item === 'number' && isFinite(item)) {
								cleanArray.push(item);
							}
						}
						if (cleanArray.length > 0) {
							cleaned[key] = cleanArray;
						}
					} else {
						// For other types, convert to string if safe
						try {
							const safeStr = this.safeString(value, '');
							if (safeStr.length > 0 && safeStr.length <= 200) {
								cleaned[key] = safeStr;
							}
						} catch (e) {
							// Skip values that can't be safely converted
							console.warn('Skipping non-serializable option:', key);
						}
					}
				}
			}
			
			return cleaned;
		},
		
		getAllEvents: function() {
			// Return all events (used when stopping recording)
			return this.events.map(event => this.cleanEvent(event)).filter(event => event !== null);
		},

		// Generate complete ChromeDP test code
		generateChromeDPTestCode: function() {
			const events = this.getAllEvents();
			if (events.length === 0) {
				return '// No events recorded';
			}

			let code = 'package main\\n\\n' +
				'import (\\n' +
				'\\t"context"\\n' +
				'\\t"log"\\n' +
				'\\t"time"\\n\\n' +
				'\\t"github.com/chromedp/chromedp"\\n' +
				'\\t"github.com/chromedp/chromedp/kb"\\n' +
				')\\n\\n' +
				'func main() {\\n' +
				'\\t// create chrome instance\\n' +
				'\\tctx, cancel := chromedp.NewContext(context.Background())\\n' +
				'\\tdefer cancel()\\n\\n' +
				'\\t// run task list\\n' +
				'\\terr := chromedp.Run(ctx,\\n' +
				'\\t\\t// Navigate to the initial page\\n' +
				'\\t\\tchromedp.Navigate("' + window.location.href + '"),\\n\\n' +
				'\\t\\t// Wait for page to load\\n' +
				'\\t\\tchromedp.WaitReady("body"),\\n\\n' +
				'\\t\\t// Recorded actions:\\n';

			// Add each event as a ChromeDP action
			events.forEach((event, index) => {
				if (event.chromedpCode) {
					code += '\\t\\t' + event.chromedpCode;
					if (index < events.length - 1) {
						code += ',';
					}
					code += '\\n';
				}
			});

			code += '\\t)\\n' +
				'\\tif err != nil {\\n' +
				'\\t\\tlog.Fatal(err)\\n' +
				'\\t}\\n' +
				'}\\n\\n' +
				'// Generated from WebTestFlow recording\\n' +
				'// Total events: ' + events.length + '\\n' +
				'// Generated at: ' + new Date().toISOString();

			return code;
		},
		
		getSelector: function(element) {
			if (element.id && !this.isDynamicId(element.id)) {
				return '#' + element.id;
			}
			
			let path = [];
			let current = element;
			
			while (current && current.nodeType === Node.ELEMENT_NODE && current !== document.body) {
				let selector = current.nodeName.toLowerCase();
				let classes = [];
				
				// Handle class names, filter out dynamic ones
				if (current.className) {
					const classList = current.className.trim().split(/\s+/);
					classes = classList.filter(cls => !this.isDynamicClass(cls));
				}
				
				// Add stable classes if any
				if (classes.length > 0) {
					selector += '.' + classes.join('.');
				}
				
				// Add position-based selector if needed for disambiguation
				if (this.needsPositionSelector(current, selector)) {
					const siblings = Array.from(current.parentNode.children).filter(
						child => child.nodeName.toLowerCase() === current.nodeName.toLowerCase()
					);
					if (siblings.length > 1) {
						const index = siblings.indexOf(current) + 1;
						selector += ':nth-of-type(' + index + ')';
					}
				}
				
				path.unshift(selector);
				current = current.parentNode;
			}
			
			return path.join(' > ');
		},
		
		// Check if ID contains timestamp or dynamic content
		isDynamicId: function(id) {
			// Check for timestamp patterns (10+ digits)
			return /\d{10,}/.test(id) || 
				   /^[a-f0-9]{8,}$/i.test(id) || // hex IDs
				   /-\d{13,}/.test(id); // timestamp suffixes
		},
		
		// Check if class name contains dynamic content
		isDynamicClass: function(className) {
			// Pattern for picker-view-column-1755069349387 style classes
			return /picker-view-column-\d{10,}/.test(className) ||
				   /-\d{10,}$/.test(className) ||
				   /^[a-f0-9]{8,}$/i.test(className) ||
				   /timestamp|uuid|guid|hash/i.test(className);
		},
		
		// Determine if element needs position-based selector
		needsPositionSelector: function(element, selector) {
			if (!element.parentNode) return false;
			
			// Check if there are similar elements that would match the same selector
			const similar = element.parentNode.querySelectorAll(selector);
			return similar.length > 1;
		},
		
		// Enhanced selector with fallback strategies
		getSelectorWithFallbacks: function(element) {
			const selectors = [];
			
			// Strategy 1: Standard selector
			selectors.push(this.getSelector(element));
			
			// Strategy 2: Text content based (for clickable elements)
			if (element.textContent && element.textContent.trim()) {
				const text = element.textContent.trim();
				if (text.length < 50) { // Reasonable text length
					selectors.push('*[text-content="' + text + '"]');
				}
			}
			
			// Strategy 3: Attribute-based
			const attrs = ['data-id', 'data-value', 'value', 'name', 'title'];
			for (let attr of attrs) {
				if (element.hasAttribute(attr) && !this.isDynamicValue(element.getAttribute(attr))) {
					selectors.push(element.nodeName.toLowerCase() + '[' + attr + '="' + element.getAttribute(attr) + '"]');
				}
			}
			
			// Strategy 4: Position in parent with stable classes
			const parent = element.parentNode;
			if (parent && parent.className) {
				const parentClasses = parent.className.trim().split(/\s+/).filter(cls => !this.isDynamicClass(cls));
				if (parentClasses.length > 0) {
					const siblings = Array.from(parent.children);
					const index = siblings.indexOf(element);
					selectors.push('.' + parentClasses.join('.') + ' > *:nth-child(' + (index + 1) + ')');
				}
			}
			
			return {
				primary: selectors[0],
				fallbacks: selectors.slice(1)
			};
		},
		
		isDynamicValue: function(value) {
			return /\d{10,}/.test(value) || /^[a-f0-9-]{20,}$/i.test(value);
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
	
	// Check if we're in mobile device emulation mode
	let isInDeviceMode = (window.innerWidth <= 768) || 
						navigator.userAgent.includes('Mobile') || 
						navigator.userAgent.includes('iPhone') || 
						navigator.userAgent.includes('Android');
	
	console.log('Device mode detection:', isInDeviceMode, 'width:', window.innerWidth, 'UA:', navigator.userAgent);
	
	// Function to show touch feedback
	function showTouchFeedback(x, y) {
		const ripple = document.createElement('div');
		ripple.className = 'touch-ripple';
		ripple.style.left = x + 'px';
		ripple.style.top = y + 'px';
		document.body.appendChild(ripple);
		
		setTimeout(() => {
			if (ripple.parentNode) {
				ripple.parentNode.removeChild(ripple);
			}
		}, 600);
	}
	
	// Function to create synthetic touch events from mouse events
	function createSyntheticTouchEvent(type, mouseEvent) {
		try {
			const touch = new Touch({
				identifier: 1,
				target: mouseEvent.target,
				clientX: mouseEvent.clientX,
				clientY: mouseEvent.clientY,
				pageX: mouseEvent.pageX,
				pageY: mouseEvent.pageY,
				screenX: mouseEvent.screenX,
				screenY: mouseEvent.screenY,
				radiusX: 20,
				radiusY: 20,
				rotationAngle: 0,
				force: 1
			});
			
			// Create touch list
			const touchList = [];
			touchList.push(touch);
			touchList.length = 1;
			touchList.item = function(index) { return this[index]; };
			
			const emptyTouchList = [];
			emptyTouchList.length = 0;
			emptyTouchList.item = function(index) { return this[index]; };
			
			const touchEvent = new TouchEvent(type, {
				bubbles: true,
				cancelable: true,
				composed: true,
				touches: type === 'touchend' ? emptyTouchList : touchList,
				targetTouches: type === 'touchend' ? emptyTouchList : touchList,
				changedTouches: touchList,
				ctrlKey: mouseEvent.ctrlKey,
				shiftKey: mouseEvent.shiftKey,
				altKey: mouseEvent.altKey,
				metaKey: mouseEvent.metaKey
			});
			
			return touchEvent;
		} catch (e) {
			console.warn('Failed to create TouchEvent, falling back to custom event:', e);
			// Fallback to custom event if TouchEvent is not supported
			const customEvent = new CustomEvent(type, {
				bubbles: true,
				cancelable: true,
				detail: {
					clientX: mouseEvent.clientX,
					clientY: mouseEvent.clientY,
					pageX: mouseEvent.pageX,
					pageY: mouseEvent.pageY,
					screenX: mouseEvent.screenX,
					screenY: mouseEvent.screenY
				}
			});
			return customEvent;
		}
	}
	
	// Helper function to check if event contains problematic data
	function hasProblematicContent(obj) {
		// Quick primitive checks first
		if (!obj || typeof obj !== 'object') {
			return false;
		}
		
		// Only check for the most critical patterns that we know cause issues
		const problematicPatterns = ['cookiePart', 'ClientNavigationReason', 'NavigationReason'];
		
		// Simple and fast check - only stringify if object seems reasonable
		try {
			// First check if object has reasonable size properties
			if (obj && typeof obj === 'object') {
				const keys = Object.keys(obj);
				if (keys.length > 100) {
					console.warn('Object has too many properties:', keys.length);
					return true;
				}
			}
			
			const str = JSON.stringify(obj);
			
			// Check size first
			if (str.length > 20000) {
				console.warn('Object too large for event:', str.length, 'characters');
				return true;
			}
			
			// Check for known problematic patterns
			for (const pattern of problematicPatterns) {
				if (str.includes(pattern)) {
					console.warn('Found problematic pattern:', pattern);
					return true;
				}
			}
			
			return false;
		} catch (e) {
			// If we can't stringify, it might be circular reference or other issue
			console.warn('Cannot stringify event, skipping:', e);
			return true;
		}
	}

	// Click events
	document.addEventListener('click', function(event) {
		console.log('Click detected on:', event.target.tagName, 'className:', event.target.className);
		console.log('Event trusted:', event.isTrusted);
		
		// Minimal check - only verify event is trusted
		if (event.isTrusted) {
			console.log('✅ Recording click event');
			// Mark user interaction
			window.autoUIRecorder.markUserInteraction();
			
			// Show touch feedback animation
			showTouchFeedback(event.pageX, event.pageY);
			
			const selectorInfo = window.autoUIRecorder.getSelectorWithFallbacks(event.target);
			
			window.autoUIRecorder.addEvent({
				type: 'click',
				selector: selectorInfo.primary,
				coordinates: window.autoUIRecorder.getCoordinates(event),
				timestamp: Date.now(),
				options: {
					button: event.button,
					detail: event.detail,
					fallbackSelectors: selectorInfo.fallbacks,
					elementText: event.target.textContent ? event.target.textContent.trim().substring(0, 100) : '',
					tagName: event.target.tagName.toLowerCase()
				}
			});
			
			console.log('Click recorded with selector:', selectorInfo.primary, 'fallbacks:', selectorInfo.fallbacks.length);
			
			// Check if click might cause navigation
			const target = event.target;
			const href = target.href || target.closest('a')?.href;
			if (href && href !== window.location.href) {
				// Record potential navigation
				setTimeout(() => {
					if (window.location.href !== href && window.location.href.includes(href.split('#')[0])) {
						console.log('Navigation detected after click:', window.location.href);
					}
				}, 100);
			}
		}
	}, true);
	
	// Input events
	document.addEventListener('input', function(event) {
		if (event.isTrusted && event.target.tagName && !hasProblematicContent(event)) {
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
		if (event.isTrusted && !hasProblematicContent(event)) {
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
	let touchStartData = null;
	let touchMoveData = [];
	
	document.addEventListener('touchstart', function(event) {
		if (event.isTrusted && !hasProblematicContent(event)) {
			// Mark user interaction
			window.autoUIRecorder.markUserInteraction();
			
			const touch = event.touches[0];
			
			// Show touch feedback animation
			showTouchFeedback(touch.pageX, touch.pageY);
			
			touchStartData = {
				x: touch.clientX,
				y: touch.clientY,
				pageX: touch.pageX,
				pageY: touch.pageY,
				timestamp: Date.now(),
				target: event.target
			};
			touchMoveData = [];
			
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
	
	document.addEventListener('touchmove', function(event) {
		if (event.isTrusted && touchStartData && !hasProblematicContent(event)) {
			const touch = event.touches[0];
			touchMoveData.push({
				x: touch.clientX,
				y: touch.clientY,
				pageX: touch.pageX,
				pageY: touch.pageY,
				timestamp: Date.now()
			});
			
			// Record touchmove events (throttled to avoid too many events)
			if (touchMoveData.length % 5 === 0) { // Record every 5th move event (reduced frequency)
				window.autoUIRecorder.addEvent({
					type: 'touchmove',
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
		}
	}, true);
	
	document.addEventListener('touchend', function(event) {
		if (event.isTrusted && touchStartData && !hasProblematicContent(event)) {
			const touch = event.changedTouches[0];
			const endTime = Date.now();
			const startTime = touchStartData.timestamp;
			const duration = endTime - startTime;
			
			// Calculate swipe distance and direction
			const deltaX = touch.clientX - touchStartData.x;
			const deltaY = touch.clientY - touchStartData.y;
			const distance = Math.sqrt(deltaX * deltaX + deltaY * deltaY);
			
			// If it's a swipe (moved more than 30px and duration > 50ms)
			if (distance > 30 && duration > 50) {
				let direction = 'unknown';
				const absDeltaX = Math.abs(deltaX);
				const absDeltaY = Math.abs(deltaY);
				
				if (absDeltaX > absDeltaY) {
					direction = deltaX > 0 ? 'right' : 'left';
				} else {
					direction = deltaY > 0 ? 'down' : 'up';
				}
				
				window.autoUIRecorder.addEvent({
					type: 'swipe',
					selector: window.autoUIRecorder.getSelector(touchStartData.target),
					coordinates: {
						startX: touchStartData.x,
						startY: touchStartData.y,
						endX: touch.clientX,
						endY: touch.clientY,
						deltaX: deltaX,
						deltaY: deltaY,
						distance: distance
					},
					timestamp: endTime,
					value: direction,
					options: {
						duration: duration,
						direction: direction,
						moves: touchMoveData.length
					}
				});
			}
			
			window.autoUIRecorder.addEvent({
				type: 'touchend',
				selector: window.autoUIRecorder.getSelector(event.target),
				coordinates: {
					x: touch.clientX,
					y: touch.clientY,
					pageX: touch.pageX,
					pageY: touch.pageY
				},
				timestamp: endTime,
				options: {
					touchCount: event.changedTouches.length,
					duration: duration
				}
			});
			
			// Reset touch data
			touchStartData = null;
			touchMoveData = [];
		}
	}, true);
	
	// Scroll events - capture both window and element scrolls (with throttling)
	document.addEventListener('scroll', function(event) {
		if (event.isTrusted && !hasProblematicContent(event)) {
			const target = event.target;
			const isWindow = target === document || target === window;
			
			// Skip scroll events from internal components like resize sensors
			if (!isWindow && window.autoUIRecorder.shouldIgnoreElement(target)) {
				console.log('Ignoring scroll event from internal component:', target.tagName, target.className);
				return;
			}
			
			// Only record scroll events for meaningful containers
			// Skip small internal elements that aren't user-scrollable areas
			if (!isWindow) {
				const tagName = target.tagName ? target.tagName.toLowerCase() : '';
				const className = target.className || '';
				
				// Skip scroll events from small internal elements
				if (target.clientHeight < 50 || target.clientWidth < 50) {
					console.log('Ignoring scroll event from small element:', tagName, 'size:', target.clientWidth, 'x', target.clientHeight);
					return;
				}
				
				// Only record scroll on meaningful scrollable containers
				const meaningfulScrollContainers = ['uni-scroll-view', 'scroll-view', 'uni-page-body', 'body', 'html'];
				const isScrollContainer = meaningfulScrollContainers.some(container => 
					tagName.includes(container) || className.includes(container)
				);
				
				if (!isScrollContainer && target !== document.documentElement && target !== document.body) {
					console.log('Ignoring scroll event from non-scrollable container:', tagName, className);
					return;
				}
			}
			
			// Only record scroll if it's user-initiated
			if (!window.autoUIRecorder.isUserInitiatedScroll()) {
				console.log('Ignoring non-user-initiated scroll event');
				return;
			}
			
			const now = Date.now();
			
			// Get current scroll position
			const currentScrollX = isWindow ? window.scrollX : target.scrollLeft;
			const currentScrollY = isWindow ? window.scrollY : target.scrollTop;
			
			// Throttle scroll events - only record if:
			// 1. More than 300ms since last scroll event
			// 2. Different target than last scroll
			// 3. Significant scroll distance change (>20px)
			const timeSinceLastScroll = now - window.autoUIRecorder.lastScrollTime;
			const scrollDistanceX = Math.abs(currentScrollX - window.autoUIRecorder.lastScrollX);
			const scrollDistanceY = Math.abs(currentScrollY - window.autoUIRecorder.lastScrollY);
			const significantChange = scrollDistanceX > 20 || scrollDistanceY > 20;
			const differentTarget = window.autoUIRecorder.lastScrollTarget !== target;
			
			if (timeSinceLastScroll > 300 || differentTarget || significantChange) {
				console.log('Recording scroll event on:', isWindow ? 'window' : target.tagName, 
					'className:', target.className, 'distance:', scrollDistanceY);
				
				window.autoUIRecorder.addEvent({
					type: 'scroll',
					selector: window.autoUIRecorder.getSelector(isWindow ? document.body : target),
					coordinates: {
						scrollX: currentScrollX,
						scrollY: currentScrollY
					},
					timestamp: now,
					options: {
						isWindow: isWindow,
						scrollHeight: isWindow ? document.body.scrollHeight : target.scrollHeight,
						clientHeight: isWindow ? window.innerHeight : target.clientHeight
					}
				});
				
				// Update tracking variables
				window.autoUIRecorder.lastScrollTime = now;
				window.autoUIRecorder.lastScrollTarget = target;
				window.autoUIRecorder.lastScrollX = currentScrollX;
				window.autoUIRecorder.lastScrollY = currentScrollY;
			} else {
				console.log('Throttling scroll event (time:', timeSinceLastScroll, 'ms, distance:', scrollDistanceY, 'px');
			}
		}
	}, true);
	
	// Wheel events for scroll detection (important for modal/popup scrolling) - with throttling
	let lastWheelTime = 0;
	let accumulatedDeltaY = 0;
	
	document.addEventListener('wheel', function(event) {
		if (event.isTrusted && !hasProblematicContent(event)) {
			// Mark user interaction for wheel events
			window.autoUIRecorder.markUserInteraction();
			
			const target = event.target;
			const now = Date.now();
			
			// Accumulate wheel delta
			accumulatedDeltaY += event.deltaY;
			
			// Only record wheel events every 150ms and if accumulated delta is significant
			if (now - lastWheelTime > 150 && Math.abs(accumulatedDeltaY) > 30) {
				console.log('Wheel event on:', target.tagName, 'className:', target.className, 'accumulated deltaY:', accumulatedDeltaY);
				
				const direction = accumulatedDeltaY > 0 ? 'down' : 'up';
				const distance = Math.min(Math.abs(accumulatedDeltaY), 200); // Cap distance
				
				window.autoUIRecorder.addEvent({
					type: 'swipe',
					selector: window.autoUIRecorder.getSelector(target),
					value: direction,
					coordinates: {
						startX: event.clientX,
						startY: event.clientY,
						endX: event.clientX,
						endY: event.clientY + (direction === 'down' ? distance : -distance),
						deltaX: 0,
						deltaY: direction === 'down' ? distance : -distance,
						distance: distance
					},
					timestamp: now,
					options: {
						inputType: 'wheel',
						direction: direction,
						deltaY: accumulatedDeltaY,
						duration: 100
					}
				});
				
				console.log('Wheel converted to swipe:', direction, 'distance:', distance, 'on element:', target.tagName);
				
				// Reset tracking variables
				lastWheelTime = now;
				accumulatedDeltaY = 0;
			}
		}
	}, true);
	
	// Form submission
	document.addEventListener('submit', function(event) {
		if (event.isTrusted && !hasProblematicContent(event)) {
			window.autoUIRecorder.addEvent({
				type: 'submit',
				selector: window.autoUIRecorder.getSelector(event.target),
				timestamp: Date.now()
			});
		}
	}, true);
	
	// Select changes
	document.addEventListener('change', function(event) {
		if (event.isTrusted && event.target.tagName && !hasProblematicContent(event)) {
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
	
	// Mouse events for drag simulation (when in device mode)
	let mouseStartData = null;
	let mouseMoveData = [];
	let isDragging = false;
	
	document.addEventListener('mousedown', function(event) {
		if (event.isTrusted && event.button === 0 && !hasProblematicContent(event)) { // Left mouse button only
			// Mark user interaction
			window.autoUIRecorder.markUserInteraction();
			
			// Show touch feedback for mobile simulation
			showTouchFeedback(event.pageX, event.pageY);
			
			mouseStartData = {
				x: event.clientX,
				y: event.clientY,
				pageX: event.pageX,
				pageY: event.pageY,
				timestamp: Date.now(),
				target: event.target
			};
			mouseMoveData = [];
			isDragging = false;
			console.log('Mouse down registered on:', event.target.tagName, 'className:', event.target.className, 'at position:', event.clientX, event.clientY);
			
			// 合成触摸开始事件发送给页面
			if (isInDeviceMode) {
				const touchEvent = createSyntheticTouchEvent('touchstart', event);
				event.target.dispatchEvent(touchEvent);
				console.log('Synthetic touchstart event dispatched');
			}
		}
	}, true);
	
	document.addEventListener('mousemove', function(event) {
		if (event.isTrusted && mouseStartData && event.buttons === 1 && !hasProblematicContent(event)) { // Left button is pressed
			if (!isDragging) {
				// Check if we've moved enough to consider it a drag
				const deltaX = event.clientX - mouseStartData.x;
				const deltaY = event.clientY - mouseStartData.y;
				const distance = Math.sqrt(deltaX * deltaX + deltaY * deltaY);
				
				if (distance > 3) { // Start considering it a drag after 3px movement (further lowered)
					isDragging = true;
					console.log('Mouse drag started, distance:', Math.round(distance), 'target:', event.target.tagName, 'className:', event.target.className);
				}
			}
			
			if (isDragging) {
				mouseMoveData.push({
					x: event.clientX,
					y: event.clientY,
					pageX: event.pageX,
					pageY: event.pageY,
					timestamp: Date.now()
				});
				
				// 合成触摸移动事件发送给页面
				if (isInDeviceMode) {
					const touchEvent = createSyntheticTouchEvent('touchmove', event);
					event.target.dispatchEvent(touchEvent);
				}
				
				// Record mousemove events (throttled)
				if (mouseMoveData.length % 5 === 0) { // Record every 5th move event (reduced frequency)
					window.autoUIRecorder.addEvent({
						type: 'mousedrag',
						selector: window.autoUIRecorder.getSelector(event.target),
						coordinates: {
							x: event.clientX,
							y: event.clientY,
							pageX: event.pageX,
							pageY: event.pageY
						},
						timestamp: Date.now(),
						options: {
							buttons: event.buttons
						}
					});
				}
			}
		}
	}, true);
	
	document.addEventListener('mouseup', function(event) {
		if (event.isTrusted && mouseStartData && event.button === 0 && !hasProblematicContent(event)) {
			const endTime = Date.now();
			const startTime = mouseStartData.timestamp;
			const duration = endTime - startTime;
			
			// Calculate drag distance and direction
			const deltaX = event.clientX - mouseStartData.x;
			const deltaY = event.clientY - mouseStartData.y;
			const distance = Math.sqrt(deltaX * deltaX + deltaY * deltaY);
			
			// If it's a drag (moved more than 3px, duration > 30ms, and was dragging)
			if (distance > 3 && duration > 30 && isDragging) {
				let direction = 'unknown';
				const absDeltaX = Math.abs(deltaX);
				const absDeltaY = Math.abs(deltaY);
				
				if (absDeltaX > absDeltaY) {
					direction = deltaX > 0 ? 'right' : 'left';
				} else {
					direction = deltaY > 0 ? 'down' : 'up';
				}
				
				// Record as swipe for mobile compatibility
				window.autoUIRecorder.addEvent({
					type: 'swipe',
					selector: window.autoUIRecorder.getSelector(mouseStartData.target),
					coordinates: {
						startX: mouseStartData.x,
						startY: mouseStartData.y,
						endX: event.clientX,
						endY: event.clientY,
						deltaX: deltaX,
						deltaY: deltaY,
						distance: distance
					},
					timestamp: endTime,
					value: direction,
					options: {
						duration: duration,
						direction: direction,
						moves: mouseMoveData.length,
						inputType: 'mouse' // Distinguish from touch
					}
				});
				
				console.log('Mouse drag detected as swipe:', direction, 'distance:', Math.round(distance), 'deltaX:', deltaX, 'deltaY:', deltaY, 'duration:', duration, 'selector:', window.autoUIRecorder.getSelector(mouseStartData.target));
			} else {
				console.log('Mouse up: drag too short or brief. Distance:', Math.round(distance), 'Duration:', duration, 'Was dragging:', isDragging);
			}
			
			// 合成触摸结束事件发送给页面
			if (isInDeviceMode && mouseStartData) {
				const touchEvent = createSyntheticTouchEvent('touchend', event);
				mouseStartData.target.dispatchEvent(touchEvent);
				console.log('Synthetic touchend event dispatched');
			}
			
			// Reset mouse data
			mouseStartData = null;
			mouseMoveData = [];
			isDragging = false;
		}
	}, true);
	
	// Force touch device behavior
	if ('ontouchstart' in window || navigator.maxTouchPoints > 0) {
		console.log('Touch device detected - enabling touch simulation');
	} else {
		console.log('Non-touch device - adding touch simulation for mobile emulation');
		
		// Override navigator properties for better mobile detection
		Object.defineProperty(navigator, 'maxTouchPoints', {value: 5, writable: false});
		Object.defineProperty(navigator, 'msMaxTouchPoints', {value: 5, writable: false});
		
		// Add synthetic touch events to window
		window.ontouchstart = null;
		window.ontouchmove = null;
		window.ontouchend = null;
		
		// Create enhanced touch event support
		if (typeof window.TouchEvent === 'undefined') {
			window.TouchEvent = function(type, params) {
				const event = document.createEvent('Event');
				event.initEvent(type, true, true);
				Object.assign(event, params);
				return event;
			};
		}
		
		if (typeof window.Touch === 'undefined') {
			window.Touch = function(params) {
				return Object.assign({
					identifier: 0,
					target: null,
					clientX: 0,
					clientY: 0,
					pageX: 0,
					pageY: 0,
					screenX: 0,
					screenY: 0,
					radiusX: 1,
					radiusY: 1,
					rotationAngle: 0,
					force: 1
				}, params);
			};
		}
	}
	
	// Add minimal mobile-specific CSS (let Chrome DevTools handle the rest)
	const mobileStyle = document.createElement('style');
	mobileStyle.textContent = '' +
		'* {' +
		'	-webkit-tap-highlight-color: rgba(0,0,0,0.1);' +
		'	-webkit-touch-callout: none;' +
		'}' +
		'' +
		'/* Make touch targets more accessible */' +
		'button, [role="button"], a, input, select, textarea {' +
		'	min-height: 44px !important;' +
		'	min-width: 44px !important;' +
		'}' +
		'' +
		'/* Touch feedback animation */' +
		'@keyframes touchRipple {' +
		'	from {' +
		'		transform: scale(0);' +
		'		opacity: 1;' +
		'	}' +
		'	to {' +
		'		transform: scale(4);' +
		'		opacity: 0;' +
		'	}' +
		'}' +
		'' +
		'.touch-ripple {' +
		'	position: absolute;' +
		'	border-radius: 50%;' +
		'	background: rgba(0,123,255,0.3);' +
		'	width: 20px;' +
		'	height: 20px;' +
		'	margin: -10px 0 0 -10px;' +
		'	animation: touchRipple 0.6s linear;' +
		'	pointer-events: none;' +
		'	z-index: 9999;' +
		'}';
	document.head.appendChild(mobileStyle);
	
	if (isInDeviceMode) {
		console.log('Device mode detected - enabling mouse-to-touch conversion');
		
		// Override mouse events to trigger touch events as well
		const originalAddEventListener = Element.prototype.addEventListener;
		Element.prototype.addEventListener = function(type, listener, options) {
			// Call original method
			originalAddEventListener.call(this, type, listener, options);
			
			// Add touch event equivalents for mouse events
			if (type === 'mousedown') {
				originalAddEventListener.call(this, 'touchstart', function(e) {
					if (e.isTrusted) return; // Only convert synthetic events
					listener.call(this, e);
				}, options);
			} else if (type === 'mousemove') {
				originalAddEventListener.call(this, 'touchmove', function(e) {
					if (e.isTrusted) return; // Only convert synthetic events
					listener.call(this, e);
				}, options);
			} else if (type === 'mouseup') {
				originalAddEventListener.call(this, 'touchend', function(e) {
					if (e.isTrusted) return; // Only convert synthetic events
					listener.call(this, e);
				}, options);
			}
		};
		
		// Force mobile viewport behavior
		const viewport = document.querySelector('meta[name="viewport"]');
		if (!viewport) {
			const newViewport = document.createElement('meta');
			newViewport.name = 'viewport';
			newViewport.content = 'width=device-width, initial-scale=1.0, user-scalable=no';
			document.head.appendChild(newViewport);
		}
	}
	
	// Monitor page navigation and URL changes
	let currentURL = window.location.href;
	
	// Function to record navigation event
	function recordNavigation(newURL, trigger) {
		if (newURL !== currentURL) {
			console.log('Recording navigation from', currentURL, 'to', newURL, 'trigger:', trigger);
			
			window.autoUIRecorder.addEvent({
				type: 'navigate',
				selector: '',
				value: newURL,
				coordinates: {},
				timestamp: Date.now(),
				options: {
					fromURL: currentURL,
					toURL: newURL,
					trigger: trigger
				}
			});
			
			currentURL = newURL;
		}
	}
	
	// Listen for popstate events (back/forward navigation)
	window.addEventListener('popstate', function(event) {
		if (!hasProblematicContent(event)) {
			console.log('Popstate event detected');
			recordNavigation(window.location.href, 'popstate');
		}
	}, true);
	
	// Listen for hashchange events  
	window.addEventListener('hashchange', function(event) {
		if (!hasProblematicContent(event)) {
			console.log('Hash change detected:', event.newURL);
			recordNavigation(event.newURL, 'hashchange');
		}
	}, true);
	
	// Monitor URL changes with polling (backup method)
	function checkURLChange() {
		const newURL = window.location.href;
		if (newURL !== currentURL) {
			console.log('URL change detected via polling:', newURL);
			recordNavigation(newURL, 'polling');
		}
	}
	
	// Check URL every 500ms
	setInterval(checkURLChange, 500);
	
	// Listen for beforeunload to handle page changes
	window.addEventListener('beforeunload', function(event) {
		if (!hasProblematicContent(event)) {
			console.log('Page is about to unload, current steps:', window.autoUIRecorder?.events?.length || 0);
			// Try to capture any final state before navigation
			if (window.autoUIRecorder && window.autoUIRecorder.isRecording) {
				// Mark that we're about to navigate
				window.autoUIRecorder.addEvent({
					type: 'beforeunload',
					selector: '',
					value: window.location.href,
					coordinates: {},
					timestamp: Date.now(),
					options: {
						url: window.location.href,
						trigger: 'beforeunload'
					}
				});
				
				// Force immediate event flush before page unloads
				try {
					console.log('Forcing immediate event flush before navigation');
					// This will be picked up by the next polling cycle
				} catch (e) {
					console.warn('Error during beforeunload event handling:', e);
				}
			}
		}
	}, true);
	
	// Enhanced DOMContentLoaded listener for page navigation detection
	if (document.readyState === 'loading') {
		document.addEventListener('DOMContentLoaded', function() {
			console.log('DOMContentLoaded detected - page may have navigated');
			// Re-announce recorder presence
			if (window.autoUIRecorder) {
				console.log('Recorder still present after DOMContentLoaded');
			} else {
				console.warn('Recorder missing after DOMContentLoaded - may need reinjection');
			}
		});
	}
	
	// Enhanced cross-domain navigation detection
	let lastDomain = window.location.hostname;
	let domainCheckInterval = setInterval(function() {
		try {
			const currentDomain = window.location.hostname;
			if (currentDomain !== lastDomain) {
				console.log('🌐 Cross-domain navigation detected:', lastDomain, '->', currentDomain);
				lastDomain = currentDomain;
				
				// Record cross-domain navigation event
				if (window.autoUIRecorder && window.autoUIRecorder.isRecording) {
					window.autoUIRecorder.addEvent({
						type: 'cross_domain_navigation',
						selector: '',
						value: window.location.href,
						coordinates: { x: 0, y: 0 },
						timestamp: Date.now(),
						options: { 
							from_domain: lastDomain, 
							to_domain: currentDomain,
							full_url: window.location.href
						}
					});
				}
			}
		} catch (e) {
			// Cross-domain error expected, clear interval
			console.warn('Cross-domain check failed (expected):', e.message);
		}
	}, 1000); // Check every second for domain changes
	
	// Store interval ID for cleanup
	if (!window.__crossDomainIntervals) {
		window.__crossDomainIntervals = [];
	}
	window.__crossDomainIntervals.push(domainCheckInterval);
	
	// Override history methods to catch programmatic navigation
	const originalPushState = history.pushState;
	const originalReplaceState = history.replaceState;
	
	history.pushState = function() {
		originalPushState.apply(history, arguments);
		setTimeout(() => {
			console.log('PushState navigation detected');
			recordNavigation(window.location.href, 'pushState');
		}, 10);
	};
	
	history.replaceState = function() {
		originalReplaceState.apply(history, arguments);
		setTimeout(() => {
			console.log('ReplaceState navigation detected');
			recordNavigation(window.location.href, 'replaceState');
		}, 10);
	};
	

	// Use safe console logging
	if (console && console.log) {
		console.log('AutoUI Recorder initialized with enhanced mobile simulation, mouse drag support, and navigation tracking');
		console.log('Current URL being monitored:', currentURL);
		console.log('Available commands:');
		console.log('- window.autoUIRecorder.getEvents() - Get recorded events');
		console.log('- window.autoUIRecorder.getAllEvents() - Get all events');  
		console.log('- window.autoUIRecorder.generateChromeDPTestCode() - Generate ChromeDP Go code');
		console.log('- window.getChromeDPCode() - Quick access to ChromeDP code');
	}
	
	// Make ChromeDP code generation globally available
	window.getChromeDPCode = function() {
		const code = window.autoUIRecorder.generateChromeDPTestCode();
		console.log('Generated ChromeDP test code:');
		console.log(code);
		return code;
	};
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
