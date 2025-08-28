package chrome

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"gorm.io/gorm"
)

// DeviceInfo represents mobile device specifications for emulation
type DeviceInfo struct {
	Name             string  `json:"name"`
	Width            int64   `json:"width"`
	Height           int64   `json:"height"`
	UserAgent        string  `json:"user_agent"`
	DevicePixelRatio float64 `json:"device_pixel_ratio"`
	Mobile           bool    `json:"mobile"`
	Touch            bool    `json:"touch"`
}

// Device represents a device configuration from database  
type Device struct {
	ID        uint   `json:"id" gorm:"primarykey"`
	Name      string `json:"name" gorm:"size:100;not null"`
	Width     int    `json:"width" gorm:"not null"`  
	Height    int    `json:"height" gorm:"not null"`
	UserAgent string `json:"user_agent" gorm:"size:500"`
	IsDefault bool   `json:"is_default" gorm:"default:false"`
	Status    int    `json:"status" gorm:"default:1"`
}

// DynamicDeviceManager manages devices loaded from database
type DynamicDeviceManager struct {
	db           *gorm.DB
	deviceCache  map[string]DeviceInfo
	cacheMutex   sync.RWMutex
	cacheLoaded  bool
}

var deviceManager *DynamicDeviceManager

// InitializeDeviceManager initializes the dynamic device manager
func InitializeDeviceManager(db *gorm.DB) {
	deviceManager = &DynamicDeviceManager{
		db:          db,
		deviceCache: make(map[string]DeviceInfo),
	}
}

// GetDeviceManager returns the global device manager instance
func GetDeviceManager() *DynamicDeviceManager {
	return deviceManager
}

// determineDeviceType automatically determines if a device is mobile or desktop based on size and name
func determineDeviceType(name string, width, height int) (bool, bool) {
	nameLower := strings.ToLower(name)
	
	// Check name patterns for explicit type detection
	if strings.Contains(nameLower, "desktop") || strings.Contains(nameLower, "laptop") {
		return false, false // Desktop: not mobile, no touch
	}
	
	if strings.Contains(nameLower, "iphone") || strings.Contains(nameLower, "android") || 
	   strings.Contains(nameLower, "mobile") || strings.Contains(nameLower, "phone") {
		return true, true // Phone: mobile with touch
	}
	
	if strings.Contains(nameLower, "ipad") || strings.Contains(nameLower, "tablet") {
		return true, true // Tablet: mobile with touch
	}
	
	// Use size-based heuristics as fallback
	// Mobile devices are typically <= 768px width
	if width <= 768 {
		return true, true // Mobile with touch
	}
	
	// Desktop/laptop devices are typically > 768px width
	return false, false // Desktop without touch
}

// convertDatabaseDeviceToDeviceInfo converts a database Device to DeviceInfo
func convertDatabaseDeviceToDeviceInfo(dbDevice Device) DeviceInfo {
	mobile, touch := determineDeviceType(dbDevice.Name, dbDevice.Width, dbDevice.Height)
	
	// Use pixel ratio of 1.0 for all devices to avoid scaling issues in UI automation testing
	pixelRatio := 1.0
	
	return DeviceInfo{
		Name:             dbDevice.Name,
		Width:            int64(dbDevice.Width),
		Height:           int64(dbDevice.Height),
		UserAgent:        dbDevice.UserAgent,
		DevicePixelRatio: pixelRatio,
		Mobile:           mobile,
		Touch:            touch,
	}
}

// loadDevicesFromDatabase loads all active devices from database
func (dm *DynamicDeviceManager) loadDevicesFromDatabase() error {
	if dm.db == nil {
		return fmt.Errorf("database connection not initialized")
	}
	
	var devices []Device
	err := dm.db.Where("status = ?", 1).Find(&devices).Error
	if err != nil {
		return fmt.Errorf("failed to load devices from database: %w", err)
	}
	
	dm.cacheMutex.Lock()
	defer dm.cacheMutex.Unlock()
	
	// Clear existing cache
	dm.deviceCache = make(map[string]DeviceInfo)
	
	// Convert database devices to DeviceInfo and cache them
	for _, device := range devices {
		deviceInfo := convertDatabaseDeviceToDeviceInfo(device)
		dm.deviceCache[device.Name] = deviceInfo
		
		log.Printf("✅ Loaded dynamic device: %s (%dx%d) - Type: %s", 
			device.Name, device.Width, device.Height, 
			map[bool]string{true: "mobile", false: "desktop"}[deviceInfo.Mobile])
	}
	
	dm.cacheLoaded = true
	log.Printf("📱 Dynamic device manager loaded %d devices from database", len(devices))
	return nil
}

// GetAllDevices returns all available devices (database + predefined)
func (dm *DynamicDeviceManager) GetAllDevices() (map[string]DeviceInfo, error) {
	if !dm.cacheLoaded {
		if err := dm.loadDevicesFromDatabase(); err != nil {
			log.Printf("⚠️ Failed to load devices from database, using predefined only: %v", err)
			return PredefinedDevices, nil
		}
	}
	
	dm.cacheMutex.RLock()
	defer dm.cacheMutex.RUnlock()
	
	// Merge database devices with predefined devices  
	// Database devices take precedence over predefined ones with same name
	allDevices := make(map[string]DeviceInfo)
	
	// Add predefined devices first
	for name, device := range PredefinedDevices {
		allDevices[name] = device
	}
	
	// Override with database devices
	for name, device := range dm.deviceCache {
		allDevices[name] = device
	}
	
	return allDevices, nil
}

// RefreshDevices reloads devices from database
func (dm *DynamicDeviceManager) RefreshDevices() error {
	return dm.loadDevicesFromDatabase()
}

// GetDevice returns a specific device by name
func (dm *DynamicDeviceManager) GetDevice(name string) (DeviceInfo, error) {
	allDevices, err := dm.GetAllDevices()
	if err != nil {
		return DeviceInfo{}, err
	}
	
	device, exists := allDevices[name]
	if !exists {
		return DeviceInfo{}, fmt.Errorf("device '%s' not found", name)
	}
	
	return device, nil
}

// PredefinedDevices contains common mobile device configurations
var PredefinedDevices = map[string]DeviceInfo{
	"iPhone 12 Pro": {
		Name:             "iPhone 12 Pro",
		Width:            390,
		Height:           844,
		DevicePixelRatio: 1.0, // 设为1.0避免页面过度放大，适合UI自动化测试
		Mobile:           true,
		Touch:            true,
		UserAgent:        "Mozilla/5.0 (iPhone; CPU iPhone OS 15_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.0 Mobile/15E148 Safari/604.1",
	},
	"iPhone 11 Pro Max": {
		Name:             "iPhone 11 Pro Max",
		Width:            414,
		Height:           896,
		DevicePixelRatio: 1.0, // 设为1.0避免页面过度放大，适合UI自动化测试
		Mobile:           true,
		Touch:            true,
		UserAgent:        "Mozilla/5.0 (iPhone; CPU iPhone OS 13_2_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.0.3 Mobile/15E148 Safari/604.1",
	},
	"iPhone X": {
		Name:             "iPhone X",
		Width:            375,
		Height:           812,
		DevicePixelRatio: 1.0, // 设为1.0避免页面过度放大，适合UI自动化测试
		Mobile:           true,
		Touch:            true,
		UserAgent:        "Mozilla/5.0 (iPhone; CPU iPhone OS 11_0 like Mac OS X) AppleWebKit/604.1.38 (KHTML, like Gecko) Version/11.0 Mobile/15A372 Safari/604.1",
	},
	"Galaxy S20": {
		Name:             "Galaxy S20",
		Width:            360,
		Height:           800,
		DevicePixelRatio: 1.0, // 设为1.0避免页面过度放大，适合UI自动化测试
		Mobile:           true,
		Touch:            true,
		UserAgent:        "Mozilla/5.0 (Linux; Android 10; SM-G981B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/80.0.3987.162 Mobile Safari/537.36",
	},
	"iPad Pro": {
		Name:             "iPad Pro",
		Width:            768,
		Height:           1024,
		DevicePixelRatio: 2.0,
		Mobile:           true,
		Touch:            true,
		UserAgent:        "Mozilla/5.0 (iPad; CPU OS 13_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/87.0.4280.77 Mobile/15E148 Safari/604.1",
	},
	"Responsive": {
		Name:             "Responsive",
		Width:            1200,
		Height:           800,
		DevicePixelRatio: 1.0,
		Mobile:           false,
		Touch:            false,
		UserAgent:        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
	},
	"Desktop 1280x800": {
		Name:             "Desktop 1280x800",
		Width:            1280,
		Height:           800,
		DevicePixelRatio: 1.0,
		Mobile:           false,
		Touch:            false,
		UserAgent:        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
	},
	"Desktop 1920x1080": {
		Name:             "Desktop 1920x1080",
		Width:            1920,
		Height:           1080,
		DevicePixelRatio: 1.0,
		Mobile:           false,
		Touch:            false,
		UserAgent:        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
	},
	"Desktop 960x700": {
		Name:             "Desktop 960x700",
		Width:            960,
		Height:           700,
		DevicePixelRatio: 1.0,
		Mobile:           false,
		Touch:            false,
		UserAgent:        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
	},
}

// GetDeviceByName returns a device configuration by name (uses dynamic device manager)
func GetDeviceByName(name string) (DeviceInfo, error) {
	// Try to use dynamic device manager first
	if deviceManager != nil {
		if device, err := deviceManager.GetDevice(name); err == nil {
			return device, nil
		}
	}
	
	// Fallback to predefined devices if dynamic manager not available
	if device, exists := PredefinedDevices[name]; exists {
		return device, nil
	}

	// Default to iPhone 12 Pro if device not found
	log.Printf("⚠️ Device '%s' not found, using iPhone 12 Pro as default", name)
	return PredefinedDevices["iPhone 12 Pro"], nil
}

// ApplyDeviceEmulation applies complete device emulation using ChromeDP
func ApplyDeviceEmulation(ctx context.Context, device DeviceInfo) error {
	deviceType := "desktop"
	if device.Mobile {
		deviceType = "mobile"
	}
	
	// For desktop devices, skip emulation and rely on Chrome startup window size
	if !device.Mobile && (device.Width >= 1024 || strings.Contains(strings.ToLower(device.Name), "desktop")) {
		log.Printf("🖥️ Skipping device emulation for desktop device: %s (%dx%d) - using Chrome window size", 
			device.Name, device.Width, device.Height)
		
		// Set desktop User Agent and viewport to ensure proper page rendering
		return chromedp.Run(ctx,
			chromedp.ActionFunc(func(ctx context.Context) error {
				log.Printf("🌐 Setting desktop User Agent for: %s", device.Name)
				return emulation.SetUserAgentOverride(device.UserAgent).Do(ctx)
			}),
			// Inject JavaScript to set proper viewport and screen dimensions for desktop
			chromedp.ActionFunc(func(ctx context.Context) error {
				log.Printf("📐 Setting desktop viewport and screen dimensions: %dx%d", device.Width, device.Height)
				viewportJS := fmt.Sprintf(`
					// Override screen dimensions for desktop rendering
					Object.defineProperty(screen, 'width', { value: %d, writable: false });
					Object.defineProperty(screen, 'height', { value: %d, writable: false });
					Object.defineProperty(screen, 'availWidth', { value: %d, writable: false });
					Object.defineProperty(screen, 'availHeight', { value: %d, writable: false });
					
					// Remove any existing viewport meta tag
					var existingViewport = document.querySelector('meta[name="viewport"]');
					if (existingViewport) {
						existingViewport.remove();
					}
					
					// Create desktop-friendly viewport meta tag
					var viewportMeta = document.createElement('meta');
					viewportMeta.name = 'viewport';
					viewportMeta.content = 'width=%d, initial-scale=1.0, user-scalable=yes';
					document.head.appendChild(viewportMeta);
					
					// Override window dimensions
					Object.defineProperty(window, 'innerWidth', { value: %d, writable: false });
					Object.defineProperty(window, 'innerHeight', { value: %d, writable: false });
					Object.defineProperty(window, 'outerWidth', { value: %d, writable: false });
					Object.defineProperty(window, 'outerHeight', { value: %d, writable: false });
				`, device.Width, device.Height, device.Width, device.Height, device.Width, device.Width, device.Height, device.Width, device.Height)
				
				_, _, err := runtime.Evaluate(viewportJS).Do(ctx)
				return err
			}),
		)
	}
	
	log.Printf("🖥️ Applying %s device emulation: %s (%dx%d, DPR: %.1f)", 
		deviceType, device.Name, device.Width, device.Height, device.DevicePixelRatio)

	return chromedp.Run(ctx,
		// Step 1: 设置设备指标覆盖 - 设定窗口尺寸和设备特性
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("📐 Setting device metrics override: %dx%d, DPR: %.1f, Mobile: %t",
				device.Width, device.Height, device.DevicePixelRatio, device.Mobile)
			return emulation.SetDeviceMetricsOverride(
				device.Width,            // width
				device.Height,           // height  
				device.DevicePixelRatio, // deviceScaleFactor
				device.Mobile,           // mobile - 这个参数很重要，告诉Chrome这是移动设备
			).Do(ctx)
		}),

		// Step 2: 设置用户代理覆盖
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("🌐 Setting user agent override")
			return emulation.SetUserAgentOverride(device.UserAgent).Do(ctx)
		}),

		// Step 3: 启用触摸事件模拟
		chromedp.ActionFunc(func(ctx context.Context) error {
			if device.Touch {
				log.Printf("👆 Enabling touch emulation")
				return emulation.SetTouchEmulationEnabled(true).Do(ctx)
			}
			log.Printf("🖱️ Touch emulation not needed")
			return nil
		}),

		// Step 4: 注入完整的移动设备检测脚本
		chromedp.ActionFunc(func(ctx context.Context) error {
			if device.Mobile {
				script := `
					console.log('🔧 Starting mobile device emulation injection...');
					
					// 1. 确保移动viewport meta标签
					let viewport = document.querySelector('meta[name="viewport"]');
					if (!viewport) {
						viewport = document.createElement('meta');
						viewport.name = 'viewport';
						viewport.content = 'width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no';
						document.getElementsByTagName('head')[0].appendChild(viewport);
						console.log('✅ Added viewport meta tag');
					}

					// 2. 覆盖screen对象属性
					Object.defineProperty(screen, 'width', {
						get: function() { return ` + fmt.Sprintf("%d", device.Width) + `; }
					});
					Object.defineProperty(screen, 'height', {
						get: function() { return ` + fmt.Sprintf("%d", device.Height) + `; }
					});
					Object.defineProperty(screen, 'availWidth', {
						get: function() { return ` + fmt.Sprintf("%d", device.Width) + `; }
					});
					Object.defineProperty(screen, 'availHeight', {
						get: function() { return ` + fmt.Sprintf("%d", device.Height) + `; }
					});

					// 3. 覆盖navigator对象属性
					Object.defineProperty(navigator, 'platform', {
						get: function() { return 'iPhone'; }
					});
					Object.defineProperty(navigator, 'maxTouchPoints', {
						get: function() { return 5; }
					});

					// 4. 添加触摸事件支持
					if (!('ontouchstart' in window)) {
						window.ontouchstart = null;
						window.ontouchmove = null;
						window.ontouchend = null;
					}

					// 5. 设置CSS媒体查询支持
					Object.defineProperty(window, 'innerWidth', {
						get: function() { return ` + fmt.Sprintf("%d", device.Width) + `; }
					});
					Object.defineProperty(window, 'innerHeight', {
						get: function() { return ` + fmt.Sprintf("%d", device.Height) + `; }
					});

					// 6. 强制触发resize事件，让响应式CSS生效
					setTimeout(function() {
						window.dispatchEvent(new Event('resize'));
						window.dispatchEvent(new Event('orientationchange'));
						console.log('✅ Mobile device emulation complete - triggered resize events');
					}, 100);

					console.log('✅ Mobile device properties injected successfully');
				`
				
				_, _, err := runtime.Evaluate(script).Do(ctx)
				if err != nil {
					log.Printf("⚠️ Failed to inject mobile device script: %v", err)
				} else {
					log.Printf("✅ Mobile device detection script injected successfully")
				}
				return nil // 不因为脚本失败而中断执行
			}
			return nil
		}),

		// Step 5: 强制刷新页面以确保CSS媒体查询重新计算
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("🔄 Refreshing page to apply device emulation")
			return chromedp.Reload().Do(ctx)
		}),

		// Step 6: 等待页面重新加载完成
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("⏳ Waiting for page reload after device emulation")
			// 等待页面DOM加载完成
			return chromedp.WaitReady("body", chromedp.ByQuery).Do(ctx)
		}),
	)
}

// getDevicePlatform returns the appropriate platform string for device
func getDevicePlatform(deviceName string) string {
	switch {
	case contains(deviceName, "iPhone"):
		return "iPhone"
	case contains(deviceName, "iPad"):
		return "iPad"
	case contains(deviceName, "Galaxy") || contains(deviceName, "Android"):
		return "Linux armv7l"
	default:
		return "iPhone"
	}
}

// getTouchPoints returns touch points count for device
func getTouchPoints(hasTouch bool) int {
	if hasTouch {
		return 5
	}
	return 0
}

// contains checks if string contains substring (case-insensitive)
func contains(str, substr string) bool {
	return len(str) >= len(substr) &&
		(str == substr ||
			len(str) > len(substr) &&
				(containsIgnoreCase(str, substr)))
}

func containsIgnoreCase(str, substr string) bool {
	for i := 0; i <= len(str)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if toLower(str[i+j]) != toLower(substr[j]) {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func toLower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 32
	}
	return c
}

// CreateDeviceEmulationChrome returns Chrome arguments for device emulation
func CreateDeviceEmulationChrome(device DeviceInfo) []string {
	args := []string{
		// Essential device emulation flags
		"--user-agent=" + device.UserAgent,
		// Set window size for all devices
		fmt.Sprintf("--window-size=%d,%d", device.Width, device.Height),
	}

	if device.Mobile {
		args = append(args,
			"--enable-features=OverlayScrollbar", // Mobile-style scrollbars
			"--simulate-outdated-no-au='Tue, 31 Dec 2099 23:59:59 GMT'",
			"--enable-viewport-meta",
			fmt.Sprintf("--force-device-scale-factor=%.1f", device.DevicePixelRatio),
		)
	} else {
		// Desktop device arguments - ensure proper desktop rendering and lock window size
		args = append(args,
			"--disable-features=VizDisplayCompositor",
			"--disable-viewport-meta",
			"--enable-desktop-site",
			fmt.Sprintf("--force-screen-size=%d,%d", device.Width, device.Height),
			fmt.Sprintf("--force-device-scale-factor=%.1f", device.DevicePixelRatio),
			fmt.Sprintf("--window-position=0,0"),  // Set window position
		)
	}

	if device.Touch {
		args = append(args, "--touch-events=enabled")
	} else {
		args = append(args, "--touch-events=disabled")
	}

	return args
}
