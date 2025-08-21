package sms

import (
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ADBService ADB服务管理
type ADBService struct {
	deviceID     string
	mu           sync.Mutex
	lastSMSTime  time.Time
	smsCache     map[string]*SMSMessage
	monitoring   bool
	stopMonitor  chan bool
}

// SMSMessage 短信消息
type SMSMessage struct {
	Phone   string
	Content string
	Code    string
	Time    time.Time
}

var (
	// 全局ADB服务实例
	globalADBService *ADBService
	adbMutex         sync.Mutex
)

// GetADBService 获取ADB短信服务实例
func GetADBService() (SMSServiceInterface, error) {
	adbMutex.Lock()
	defer adbMutex.Unlock()
	
	if globalADBService == nil {
		service, err := NewADBService()
		if err != nil {
			return nil, err
		}
		globalADBService = service
		log.Printf("✅ ADB短信服务初始化成功")
	}
	
	return globalADBService, nil
}

// SMSServiceInterface 统一的SMS服务接口
type SMSServiceInterface interface {
	GetLatestSMSCode(phone string, timeout time.Duration) (string, error)
	GetSMSCodeWithRetry(phone string, timeout time.Duration, retries int) (string, error)
	CheckPermissions() error
}

// NewADBService 创建ADB服务
func NewADBService() (*ADBService, error) {
	// 检查ADB是否安装
	if _, err := exec.LookPath("adb"); err != nil {
		return nil, fmt.Errorf("ADB未安装，请先安装Android SDK: %v", err)
	}
	
	// 获取连接的设备
	deviceID, err := getConnectedDevice()
	if err != nil {
		return nil, err
	}
	
	service := &ADBService{
		deviceID:    deviceID,
		smsCache:    make(map[string]*SMSMessage),
		stopMonitor: make(chan bool),
	}
	
	// 不再自动启动监控，主要使用直接查询
	log.Printf("ADB服务就绪，设备: %s", deviceID)
	
	return service, nil
}

// getConnectedDevice 获取连接的Android设备
func getConnectedDevice() (string, error) {
	cmd := exec.Command("adb", "devices")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("执行adb devices失败: %v", err)
	}
	
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		// 跳过标题行和空行
		if strings.Contains(line, "List of devices") || strings.TrimSpace(line) == "" {
			continue
		}
		
		// 查找已连接的设备
		if strings.Contains(line, "device") && !strings.Contains(line, "offline") {
			parts := strings.Fields(line)
			if len(parts) >= 2 && parts[1] == "device" {
				log.Printf("找到Android设备: %s", parts[0])
				return parts[0], nil
			}
		}
	}
	
	return "", fmt.Errorf("未找到已连接的Android设备，请确保: 1)手机已连接 2)已开启USB调试 3)已授权此电脑")
}

// startMonitoring 启动短信监控
func (s *ADBService) startMonitoring() {
	s.monitoring = true
	log.Println("开始监控短信...")
	
	for s.monitoring {
		select {
		case <-s.stopMonitor:
			s.monitoring = false
			return
		case <-time.After(2 * time.Second):
			// 每2秒检查一次新短信
			s.checkNewSMS()
		}
	}
}

// checkNewSMS 检查新短信
func (s *ADBService) checkNewSMS() {
	// 使用直接的content query命令获取最近的短信（修复的版本）
	cmd := exec.Command("adb", "-s", s.deviceID, "shell", 
		"content query --uri content://sms/inbox --projection address:body:date --sort 'date DESC' | head -5")
	
	output, err := cmd.Output()
	if err != nil {
		// 静默处理错误，避免日志刷屏
		return
	}
	
	// 解析短信内容
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Row:") {
			s.parseSMSRow(line)
		}
	}
}

// parseSMSRow 解析短信行
func (s *ADBService) parseSMSRow(row string) {
	// 提取手机号和内容
	phoneReg := regexp.MustCompile(`address=([^,\s]+)`)
	bodyReg := regexp.MustCompile(`body=(.+?)(,\s*date=|$)`)
	
	phoneMatch := phoneReg.FindStringSubmatch(row)
	bodyMatch := bodyReg.FindStringSubmatch(row)
	
	if len(phoneMatch) > 1 && len(bodyMatch) > 1 {
		phone := phoneMatch[1]
		body := bodyMatch[1]
		
		// 提取验证码
		code := extractVerificationCode(body)
		if code != "" {
			s.mu.Lock()
			// 缓存短信
			s.smsCache[phone] = &SMSMessage{
				Phone:   phone,
				Content: body,
				Code:    code,
				Time:    time.Now(),
			}
			s.mu.Unlock()
			
			log.Printf("收到验证码短信 - 号码: %s, 验证码: %s", phone, code)
		}
	}
}

// GetLatestSMSCodeDirect 直接查询最新短信验证码（不依赖监控）
func (s *ADBService) GetLatestSMSCodeDirect(phone string) (string, error) {
	log.Printf("🔍 直接查询手机号 %s 的最新验证码...", phone)
	
	// 首先等待10秒，让短信有时间到达
	log.Printf("⏰ 等待10秒让短信到达...")
	time.Sleep(10 * time.Second)
	
	// 执行ADB查询命令获取最新短信
	cmd := exec.Command("adb", "-s", s.deviceID, "shell", 
		"content query --uri content://sms/inbox --projection address:body:date --sort 'date DESC' | head -3")
	
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("ADB查询失败: %v", err)
	}
	
	log.Printf("📱 ADB查询结果:\n%s", string(output))
	
	// 解析输出，查找验证码
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Row:") && strings.Contains(line, "body=") {
			// 解析这行短信
			code := s.parseDirectSMSRow(line)
			if code != "" {
				log.Printf("✅ 找到最新验证码: %s", code)
				return code, nil
			}
		}
	}
	
	return "", fmt.Errorf("未找到验证码")
}

// parseDirectSMSRow 解析直接查询的短信行
func (s *ADBService) parseDirectSMSRow(row string) string {
	// 直接提取body内容
	bodyReg := regexp.MustCompile(`body=([^,]+(?:，[^,]*)*)`)
	bodyMatch := bodyReg.FindStringSubmatch(row)
	
	if len(bodyMatch) > 1 {
		body := bodyMatch[1]
		log.Printf("📝 短信内容: %s", body)
		
		// 提取验证码
		code := extractVerificationCode(body)
		if code != "" {
			log.Printf("🔍 提取到验证码: %s", code)
			return code
		}
	}
	
	return ""
}

// GetLatestSMSCode 获取最新的短信验证码
func (s *ADBService) GetLatestSMSCode(phone string, timeout time.Duration) (string, error) {
	startTime := time.Now()
	
	// 清理号码格式（去除+86等前缀）
	phone = normalizePhoneNumber(phone)
	
	log.Printf("等待手机号 %s 的短信验证码...", phone)
	
	for {
		// 检查缓存
		s.mu.Lock()
		for cachedPhone, msg := range s.smsCache {
			normalizedCached := normalizePhoneNumber(cachedPhone)
			// 匹配手机号并且短信时间在开始等待之后
			if normalizedCached == phone && msg.Time.After(startTime) {
				code := msg.Code
				// 使用后删除缓存
				delete(s.smsCache, cachedPhone)
				s.mu.Unlock()
				log.Printf("获取到验证码: %s", code)
				return code, nil
			}
		}
		s.mu.Unlock()
		
		// 检查超时
		if time.Since(startTime) > timeout {
			return "", fmt.Errorf("等待短信验证码超时（%v）", timeout)
		}
		
		// 主动查询一次
		s.checkNewSMS()
		
		time.Sleep(1 * time.Second)
	}
}

// GetSMSCodeWithRetry 带重试的获取短信验证码
func (s *ADBService) GetSMSCodeWithRetry(phone string, timeout time.Duration, retries int) (string, error) {
	var lastErr error
	
	for i := 0; i < retries; i++ {
		if i > 0 {
			log.Printf("第 %d 次重试获取验证码...", i+1)
			time.Sleep(2 * time.Second)
		}
		
		code, err := s.GetLatestSMSCode(phone, timeout)
		if err == nil {
			return code, nil
		}
		lastErr = err
	}
	
	return "", fmt.Errorf("获取验证码失败（重试%d次）: %v", retries, lastErr)
}

// extractVerificationCode 提取验证码
func extractVerificationCode(smsBody string) string {
	// 常见的验证码模式
	patterns := []string{
		`验证码[:：]?\s*(\d{4,6})`,
		`验证码是[:：]?\s*(\d{4,6})`,
		`验证码为[:：]?\s*(\d{4,6})`,
		`码[:：]?\s*(\d{4,6})`,
		`code[:：]?\s*(\d{4,6})`,
		`(\d{4,6})\s*为您的验证码`,
		`(\d{4,6})\s*是您的验证码`,
		`您的验证码是[:：]?\s*(\d{4,6})`,
		`【[^】]+】\s*(\d{4,6})`,
		`\[.*?\]\s*(\d{4,6})`,
	}
	
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(smsBody)
		if len(matches) > 1 {
			return matches[1]
		}
	}
	
	// 如果没有匹配到特定模式，尝试查找任意4-6位数字
	digitReg := regexp.MustCompile(`\b(\d{4,6})\b`)
	matches := digitReg.FindStringSubmatch(smsBody)
	if len(matches) > 1 {
		return matches[1]
	}
	
	return ""
}

// normalizePhoneNumber 标准化手机号码
func normalizePhoneNumber(phone string) string {
	// 移除所有非数字字符
	re := regexp.MustCompile(`\D`)
	phone = re.ReplaceAllString(phone, "")
	
	// 移除国家代码
	if strings.HasPrefix(phone, "86") && len(phone) == 13 {
		phone = phone[2:]
	}
	if strings.HasPrefix(phone, "0086") && len(phone) == 15 {
		phone = phone[4:]
	}
	
	return phone
}

// ClearSMSCache 清理短信缓存
func (s *ADBService) ClearSMSCache() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.smsCache = make(map[string]*SMSMessage)
	log.Println("短信缓存已清理")
}

// Stop 停止ADB服务
func (s *ADBService) Stop() {
	if s.monitoring {
		s.stopMonitor <- true
		s.monitoring = false
		log.Println("ADB短信监控已停止")
	}
}

// 确保ADBService实现SMSServiceInterface接口
var _ SMSServiceInterface = (*ADBService)(nil)

// CheckPermissions 检查权限
func (s *ADBService) CheckPermissions() error {
	// 检查READ_SMS权限
	cmd := exec.Command("adb", "-s", s.deviceID, "shell",
		"pm", "list", "permissions", "-g", "-d")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("检查权限失败: %v", err)
	}
	
	if !strings.Contains(string(output), "android.permission.READ_SMS") {
		// 尝试授予权限
		log.Println("尝试授予READ_SMS权限...")
		grantCmd := exec.Command("adb", "-s", s.deviceID, "shell",
			"pm", "grant", "com.android.shell", "android.permission.READ_SMS")
		if err := grantCmd.Run(); err != nil {
			return fmt.Errorf("授予READ_SMS权限失败，请在手机上手动授权: %v", err)
		}
	}
	
	return nil
}