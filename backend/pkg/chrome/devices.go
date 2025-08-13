package chrome

import (
	"context"
	"fmt"
	"log"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
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

// PredefinedDevices contains common mobile device configurations
var PredefinedDevices = map[string]DeviceInfo{
	"iPhone 12 Pro": {
		Name:             "iPhone 12 Pro",
		Width:            390,
		Height:           844,
		DevicePixelRatio: 1.0, // 使用1.0避免文字过大，Chrome会自动处理缩放
		Mobile:           true,
		Touch:            true,
		UserAgent:        "Mozilla/5.0 (iPhone; CPU iPhone OS 14_7_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.1.2 Mobile/15E148 Safari/604.1",
	},
	"iPhone 11 Pro Max": {
		Name:             "iPhone 11 Pro Max",
		Width:            414,
		Height:           896,
		DevicePixelRatio: 1.0, // 调整为1.0避免文字过大
		Mobile:           true,
		Touch:            true,
		UserAgent:        "Mozilla/5.0 (iPhone; CPU iPhone OS 13_2_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.0.3 Mobile/15E148 Safari/604.1",
	},
	"iPhone X": {
		Name:             "iPhone X",
		Width:            375,
		Height:           812,
		DevicePixelRatio: 1.0, // 调整为1.0避免文字过大
		Mobile:           true,
		Touch:            true,
		UserAgent:        "Mozilla/5.0 (iPhone; CPU iPhone OS 11_0 like Mac OS X) AppleWebKit/604.1.38 (KHTML, like Gecko) Version/11.0 Mobile/15A372 Safari/604.1",
	},
	"Galaxy S20": {
		Name:             "Galaxy S20",
		Width:            360,
		Height:           800,
		DevicePixelRatio: 1.0, // 调整为1.0避免文字过大
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
}

// GetDeviceByName returns a device configuration by name
func GetDeviceByName(name string) (DeviceInfo, error) {
	if device, exists := PredefinedDevices[name]; exists {
		return device, nil
	}

	// Default to iPhone 12 Pro if device not found
	log.Printf("⚠️ Device '%s' not found, using iPhone 12 Pro as default", name)
	return PredefinedDevices["iPhone 12 Pro"], nil
}

// ApplyDeviceEmulation applies complete device emulation using ChromeDP
func ApplyDeviceEmulation(ctx context.Context, device DeviceInfo) error {
	log.Printf("🎭 Applying device emulation: %s (%dx%d)", device.Name, device.Width, device.Height)

	return chromedp.Run(ctx,
		// Step 1: Set device metrics override (equivalent to DevTools device mode)
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("📐 Setting device metrics: %dx%d, DPR: %.1f, Mobile: %t",
				device.Width, device.Height, device.DevicePixelRatio, device.Mobile)
			return emulation.SetDeviceMetricsOverride(
				device.Width,            // width
				device.Height,           // height
				device.DevicePixelRatio, // deviceScaleFactor
				device.Mobile,           // mobile
			).Do(ctx)
		}),

		// Step 2: Set user agent override
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("🌐 Setting user agent: %s", device.UserAgent)
			return emulation.SetUserAgentOverride(device.UserAgent).Do(ctx)
		}),

		// Step 3: Enable touch emulation if device supports touch
		chromedp.ActionFunc(func(ctx context.Context) error {
			if device.Touch {
				log.Printf("👆 Enabling touch emulation")
				return emulation.SetTouchEmulationEnabled(true).Do(ctx)
			} else {
				log.Printf("🖱️ Disabling touch emulation")
				return emulation.SetTouchEmulationEnabled(false).Do(ctx)
			}
		}),

		// Step 4: Inject mobile device detection JavaScript
		chromedp.ActionFunc(func(ctx context.Context) error {
			if device.Mobile {
				script := fmt.Sprintf(`
					// Override device detection properties for mobile emulation
					Object.defineProperty(navigator, 'platform', {
						get: function() { return '%s'; }
					});
					Object.defineProperty(screen, 'width', {
						get: function() { return %d; }
					});
					Object.defineProperty(screen, 'height', {
						get: function() { return %d; }
					});
					// 不覆盖devicePixelRatio，让Chrome自动处理缩放
					// Add ontouchstart event support
					if (!('ontouchstart' in window)) {
						window.ontouchstart = null;
						window.ontouchmove = null;
						window.ontouchend = null;
					}
					// Override max touch points
					Object.defineProperty(navigator, 'maxTouchPoints', {
						get: function() { return %d; }
					});
					console.log('✅ Mobile device emulation applied: %s (DPR preserved)');
				`,
					getDevicePlatform(device.Name),
					device.Width, device.Height,
					getTouchPoints(device.Touch),
					device.Name)

				_, exp, err := runtime.Evaluate(script).Do(ctx)
				_ = exp // Ignore exception
				if err != nil {
					log.Printf("⚠️ Failed to inject mobile detection script: %v", err)
				} else {
					log.Printf("✅ Mobile detection script injected successfully")
				}
				return err
			}
			return nil
		}),

		// Step 5: Force viewport update
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Printf("🔄 Forcing viewport update")
			// Force a window resize to ensure viewport changes take effect
			return chromedp.Evaluate(`
				window.dispatchEvent(new Event('resize'));
				document.documentElement.style.width = '100%';
				document.documentElement.style.height = '100%';
			`, nil).Do(ctx)
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
		// Essential device emulation flags - remove forced scaling to avoid large text
		"--user-agent=" + device.UserAgent,
	}

	if device.Mobile {
		args = append(args,
			"--enable-features=OverlayScrollbar", // Mobile-style scrollbars
			"--simulate-outdated-no-au='Tue, 31 Dec 2099 23:59:59 GMT'",
		)
	}

	if device.Touch {
		args = append(args, "--touch-events=enabled")
	} else {
		args = append(args, "--touch-events=disabled")
	}

	return args
}
