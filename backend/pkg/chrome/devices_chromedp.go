package chrome

import (
	"context"
	"log"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/device"
)

// ChromeDPDeviceEmulator uses ChromeDP's built-in device emulation
type ChromeDPDeviceEmulator struct{}

// PredefinedChromeDPDevices contains device.Info for common devices
var PredefinedChromeDPDevices = map[string]device.Info{
	"iPhone 12 Pro": {
		Name:      "iPhone 12 Pro",
		UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 14_7_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.1.2 Mobile/15E148 Safari/604.1",
		Width:     390,
		Height:    844,
		Scale:     1.0, // Use 1.0 to avoid text being too large
		Landscape: false,
		Mobile:    true,
		Touch:     true,
	},
	"iPhone 12 Pro Max": {
		Name:      "iPhone 12 Pro Max",
		UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 14_7_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.1.2 Mobile/15E148 Safari/604.1",
		Width:     428,
		Height:    926,
		Scale:     1.0,
		Landscape: false,
		Mobile:    true,
		Touch:     true,
	},
	"iPhone X": {
		Name:      "iPhone X",
		UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 11_0 like Mac OS X) AppleWebKit/604.1.38 (KHTML, like Gecko) Version/11.0 Mobile/15A372 Safari/604.1",
		Width:     375,
		Height:    812,
		Scale:     1.0,
		Landscape: false,
		Mobile:    true,
		Touch:     true,
	},
	"iPad Pro": {
		Name:      "iPad Pro",
		UserAgent: "Mozilla/5.0 (iPad; CPU OS 13_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/87.0.4280.77 Mobile/15E148 Safari/604.1",
		Width:     1024,
		Height:    1366,
		Scale:     1.0,
		Landscape: false,
		Mobile:    true,
		Touch:     true,
	},
	"Galaxy S5": {
		Name:      "Galaxy S5",
		UserAgent: "Mozilla/5.0 (Linux; Android 5.0; SM-G900P Build/LRX21T) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Mobile Safari/537.36",
		Width:     360,
		Height:    640,
		Scale:     1.0,
		Landscape: false,
		Mobile:    true,
		Touch:     true,
	},
	"Desktop 1280x800": {
		Name:      "Desktop 1280x800",
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		Width:     1280,
		Height:    800,
		Scale:     1.0,
		Landscape: false,
		Mobile:    false,
		Touch:     false,
	},
	"Desktop 1920x1080": {
		Name:      "Desktop 1920x1080",
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		Width:     1920,
		Height:    1080,
		Scale:     1.0,
		Landscape: false,
		Mobile:    false,
		Touch:     false,
	},
	"Desktop 960x700": {
		Name:      "Desktop 960x700",
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		Width:     960,
		Height:    700,
		Scale:     1.0,
		Landscape: false,
		Mobile:    false,
		Touch:     false,
	},
}

// convertDeviceInfoToChromeDPDevice converts DeviceInfo to ChromeDP device.Info
func convertDeviceInfoToChromeDPDevice(deviceInfo DeviceInfo) device.Info {
	return device.Info{
		Name:      deviceInfo.Name,
		UserAgent: deviceInfo.UserAgent,
		Width:     int64(deviceInfo.Width),
		Height:    int64(deviceInfo.Height),
		Scale:     deviceInfo.DevicePixelRatio,
		Landscape: false,
		Mobile:    deviceInfo.Mobile,
		Touch:     deviceInfo.Touch,
	}
}

// GetChromeDPDevice returns a ChromeDP device.Info by name (supports dynamic devices)
func GetChromeDPDevice(deviceName string) (device.Info, error) {
	// Try to get from dynamic device manager first
	if deviceManager != nil {
		if deviceInfo, err := deviceManager.GetDevice(deviceName); err == nil {
			return convertDeviceInfoToChromeDPDevice(deviceInfo), nil
		}
	}
	
	// Fallback to predefined ChromeDP devices
	if dev, exists := PredefinedChromeDPDevices[deviceName]; exists {
		return dev, nil
	}

	// Default fallback to iPhone 12 Pro
	log.Printf("⚠️ Device '%s' not found in ChromeDP devices, using iPhone 12 Pro", deviceName)
	return PredefinedChromeDPDevices["iPhone 12 Pro"], nil
}

// ApplyChromeDPDeviceEmulation applies device emulation using ChromeDP's built-in devices
func ApplyChromeDPDeviceEmulation(ctx context.Context, deviceName string) error {
	dev, err := GetChromeDPDevice(deviceName)
	if err != nil {
		return err
	}

	log.Printf("🎭 Applying ChromeDP device emulation: %s (%dx%d, Scale=%.1f, Mobile=%t)",
		dev.Name, dev.Width, dev.Height, dev.Scale, dev.Mobile)

	// Use ChromeDP's built-in device emulation
	return chromedp.Run(ctx, chromedp.Emulate(dev))
}

// ListAvailableChromeDPDevices returns all available ChromeDP devices
func ListAvailableChromeDPDevices() []string {
	devices := make([]string, 0, len(PredefinedChromeDPDevices))
	for name := range PredefinedChromeDPDevices {
		devices = append(devices, name)
	}
	return devices
}

// GetDeviceInfo returns device information for display purposes
func GetDeviceInfo(deviceName string) (map[string]interface{}, error) {
	dev, err := GetChromeDPDevice(deviceName)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"name":       dev.Name,
		"width":      dev.Width,
		"height":     dev.Height,
		"user_agent": dev.UserAgent,
		"scale":      dev.Scale,
		"mobile":     dev.Mobile,
		"touch":      dev.Touch,
		"landscape":  dev.Landscape,
	}, nil
}
