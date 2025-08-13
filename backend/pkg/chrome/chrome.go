package chrome

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// GetChromePath returns the path to Chrome executable
func GetChromePath() string {
	// Common Chrome paths for different systems
	var chromePaths []string

	switch runtime.GOOS {
	case "linux":
		chromePaths = []string{
			"/usr/bin/google-chrome-stable",
			"/usr/bin/google-chrome",
			"/usr/bin/chromium-browser",
			"/usr/bin/chromium",
			"/snap/bin/chromium",
			"/opt/google/chrome/google-chrome",
		}
	case "darwin":
		chromePaths = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		}
	case "windows":
		chromePaths = []string{
			"C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe",
			"C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe",
			"C:\\Users\\%USERNAME%\\AppData\\Local\\Google\\Chrome\\Application\\chrome.exe",
		}
	}

	// Check each path
	for _, path := range chromePaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Try to find in PATH
	if path, err := exec.LookPath("google-chrome"); err == nil {
		return path
	}
	if path, err := exec.LookPath("google-chrome-stable"); err == nil {
		return path
	}
	if path, err := exec.LookPath("chromium-browser"); err == nil {
		return path
	}
	if path, err := exec.LookPath("chromium"); err == nil {
		return path
	}

	return "" // Not found
}

// GetFlatpakChromePath returns the path for flatpak Chrome if available
func GetFlatpakChromePath() string {
	if !isFlatpakChromeAvailable() {
		return ""
	}

	// 使用相对路径
	wrapperPath := "./scripts/chrome-flatpak-wrapper.sh"

	// 检查包装脚本是否存在
	if _, err := os.Stat(wrapperPath); err == nil {
		return wrapperPath
	}

	return ""
}

// isFlatpakChromeAvailable checks if Chrome is available via Flatpak
func isFlatpakChromeAvailable() bool {
	// Check if flatpak command exists
	if _, err := exec.LookPath("flatpak"); err != nil {
		return false
	}

	// Check if Chrome is installed via flatpak
	cmd := exec.Command("flatpak", "list", "--app", "--columns=application")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	outputStr := string(output)
	return strings.Contains(outputStr, "com.google.Chrome") || strings.Contains(outputStr, "org.chromium.Chromium")
}
