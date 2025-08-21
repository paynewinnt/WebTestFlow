package executor

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"
	"webtestflow/backend/internal/models"
	"webtestflow/backend/pkg/captcha"
	"webtestflow/backend/pkg/sms"

	"github.com/chromedp/chromedp"
)

// Box 表示元素的位置和大小
type Box struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// handleCaptcha 处理验证码步骤
func (te *TestExecutor) handleCaptcha(ctx context.Context, step models.TestStep) error {
	log.Printf("🔐 Processing captcha step - Type: %s", step.CaptchaType)
	
	switch step.CaptchaType {
	case "image_ocr":
		return te.handleImageCaptcha(ctx, step)
	case "sms":
		return te.handleSMSCaptcha(ctx, step)
	case "sliding":
		return te.handleSlidingCaptcha(ctx, step)
	default:
		return fmt.Errorf("unsupported captcha type: %s", step.CaptchaType)
	}
}

// handleImageCaptcha 处理图形验证码
func (te *TestExecutor) handleImageCaptcha(ctx context.Context, step models.TestStep) error {
	log.Printf("🖼️ Handling image captcha - Selector: %s", step.CaptchaSelector)
	
	// 确保验证码图片可见
	err := chromedp.Run(ctx,
		chromedp.WaitVisible(step.CaptchaSelector, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond), // 等待图片完全加载
	)
	if err != nil {
		return fmt.Errorf("验证码图片不可见: %w", err)
	}
	
	// 截取验证码图片
	var buf []byte
	err = chromedp.Run(ctx,
		chromedp.Screenshot(step.CaptchaSelector, &buf, chromedp.NodeVisible, chromedp.ByQuery),
	)
	if err != nil {
		return fmt.Errorf("截取验证码失败: %w", err)
	}
	
	log.Printf("📸 Captured captcha image, size: %d bytes", len(buf))
	
	// 调用OCR服务识别
	ocrClient := captcha.NewOCRClient(os.Getenv("OCR_SERVICE_URL"))
	
	// 检查OCR服务健康状态
	if err := ocrClient.HealthCheck(); err != nil {
		log.Printf("⚠️ OCR service health check failed: %v", err)
		// 可以选择继续或返回错误
	}
	
	code, err := ocrClient.RecognizeImage(buf)
	if err != nil {
		return fmt.Errorf("OCR识别失败: %w", err)
	}
	
	log.Printf("✅ OCR recognized code: %s", code)
	
	// 输入识别的验证码
	inputSelector := step.CaptchaInputSelector
	if inputSelector == "" {
		// 如果没有指定输入框，使用原始选择器或查找常见的验证码输入框
		if step.Selector != "" {
			inputSelector = step.Selector
		} else {
			inputSelector = "input[type='text'][placeholder*='验证码'], input[name*='captcha'], input[id*='captcha']"
		}
	}
	
	err = chromedp.Run(ctx,
		chromedp.Clear(inputSelector, chromedp.ByQuery),
		chromedp.SendKeys(inputSelector, code, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
	)
	if err != nil {
		return fmt.Errorf("输入验证码失败: %w", err)
	}
	
	log.Printf("✅ Image captcha handled successfully - Code: %s", code)
	return nil
}

// handleSMSCaptcha 处理短信验证码
func (te *TestExecutor) handleSMSCaptcha(ctx context.Context, step models.TestStep) error {
	log.Printf("📱 Handling SMS captcha - Phone: %s", step.CaptchaPhone)
	
	// 如果有发送验证码按钮，先点击
	if step.CaptchaSelector != "" {
		log.Printf("🔘 Clicking SMS send button: %s", step.CaptchaSelector)
		err := chromedp.Run(ctx,
			chromedp.Click(step.CaptchaSelector, chromedp.ByQuery),
			chromedp.Sleep(2*time.Second), // 等待短信发送
		)
		if err != nil {
			log.Printf("⚠️ Failed to click SMS button: %v", err)
		}
	}
	
	// 设置超时时间
	timeout := time.Duration(step.CaptchaTimeout) * time.Second
	if timeout == 0 {
		timeout = 60 * time.Second // 默认60秒超时
	}
	
	log.Printf("⏳ Waiting for SMS code (timeout: %v)...", timeout)
	
	var code string
	var err error
	
	// 使用直接ADB查询获取短信验证码
	log.Printf("📱 使用ADB直接查询获取短信验证码")
	adbService, adbErr := sms.GetADBService()
	if adbErr != nil {
		return fmt.Errorf("ADB服务初始化失败: %w", adbErr)
	}
	
	// 检查权限
	if err := adbService.CheckPermissions(); err != nil {
		log.Printf("⚠️ ADB permissions check failed: %v", err)
	}
	
	// 使用直接ADB查询（包含10秒等待）
	if adbSrv, ok := adbService.(*sms.ADBService); ok {
		code, err = adbSrv.GetLatestSMSCodeDirect(step.CaptchaPhone)
	} else {
		// fallback to original method
		code, err = adbService.GetSMSCodeWithRetry(step.CaptchaPhone, timeout, 3)
	}
	
	if err != nil {
		return fmt.Errorf("获取短信验证码失败: %w", err)
	}
	
	log.Printf("✅ SMS code received: %s", code)
	
	// 输入验证码
	inputSelector := step.CaptchaInputSelector
	if inputSelector == "" {
		if step.Selector != "" {
			inputSelector = step.Selector
		} else {
			// 尝试查找常见的短信验证码输入框
			inputSelector = "input[type='text'][placeholder*='短信'], input[type='text'][placeholder*='验证码'], input[name*='sms'], input[id*='sms']"
		}
	}
	
	err = chromedp.Run(ctx,
		chromedp.Clear(inputSelector, chromedp.ByQuery),
		chromedp.SendKeys(inputSelector, code, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
	)
	if err != nil {
		return fmt.Errorf("输入短信验证码失败: %w", err)
	}
	
	log.Printf("✅ SMS captcha handled successfully - Code: %s", code)
	return nil
}

// handleSlidingCaptcha 处理滑块验证码
func (te *TestExecutor) handleSlidingCaptcha(ctx context.Context, step models.TestStep) error {
	log.Printf("🎚️ Handling sliding captcha")
	
	// 等待滑块验证码组件加载
	backgroundSelector := step.CaptchaSelector // 背景图选择器
	sliderSelector := step.Selector            // 滑块选择器
	
	if backgroundSelector == "" {
		backgroundSelector = ".captcha-bg, .slide-bg, canvas"
	}
	if sliderSelector == "" {
		sliderSelector = ".captcha-slider, .slide-btn, .slider-btn"
	}
	
	// 确保滑块组件可见
	err := chromedp.Run(ctx,
		chromedp.WaitVisible(backgroundSelector, chromedp.ByQuery),
		chromedp.WaitVisible(sliderSelector, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
	)
	if err != nil {
		return fmt.Errorf("滑块验证码组件不可见: %w", err)
	}
	
	// 截取背景图
	var bgBuf []byte
	err = chromedp.Run(ctx,
		chromedp.Screenshot(backgroundSelector, &bgBuf, chromedp.NodeVisible, chromedp.ByQuery),
	)
	if err != nil {
		return fmt.Errorf("截取滑块背景图失败: %w", err)
	}
	
	log.Printf("📸 Captured background image, size: %d bytes", len(bgBuf))
	
	// 调用OCR服务识别滑块位置
	ocrClient := captcha.NewOCRClient(os.Getenv("OCR_SERVICE_URL"))
	distance, err := ocrClient.RecognizeSliding(bgBuf, nil)
	if err != nil {
		return fmt.Errorf("滑块位置识别失败: %w", err)
	}
	
	log.Printf("📏 Sliding distance detected: %d pixels", distance)
	
	// 获取滑块位置信息
	var box Box
	err = chromedp.Run(ctx,
		chromedp.Evaluate(fmt.Sprintf(`
			(function() {
				const elem = document.querySelector('%s');
				const rect = elem.getBoundingClientRect();
				return {
					x: rect.left,
					y: rect.top,
					width: rect.width,
					height: rect.height
				};
			})()
		`, sliderSelector), &box),
	)
	if err != nil {
		return fmt.Errorf("获取滑块位置失败: %w", err)
	}
	
	// 执行滑动操作
	startX := box.X + box.Width/2
	startY := box.Y + box.Height/2
	endX := startX + float64(distance)
	
	log.Printf("🎯 Sliding from (%.0f, %.0f) to (%.0f, %.0f)", startX, startY, endX, startY)
	
	// 模拟人工滑动（带有轻微抖动）
	err = chromedp.Run(ctx,
		chromedp.MouseClickXY(startX, startY, chromedp.ButtonLeft),
		chromedp.Sleep(100*time.Millisecond),
	)
	if err != nil {
		return fmt.Errorf("鼠标按下失败: %w", err)
	}
	
	// 分段滑动，模拟人工操作
	steps := 10
	for i := 1; i <= steps; i++ {
		currentX := startX + (endX-startX)*float64(i)/float64(steps)
		// 添加轻微的Y轴抖动
		jitter := float64(i%2) * 2 - 1 // -1 or 1
		currentY := startY + jitter
		
		err = chromedp.Run(ctx,
			chromedp.MouseEvent("mousemove", currentX, currentY),
			chromedp.Sleep(50*time.Millisecond),
		)
		if err != nil {
			log.Printf("⚠️ Mouse move failed at step %d: %v", i, err)
		}
	}
	
	// 释放鼠标
	err = chromedp.Run(ctx,
		chromedp.MouseEvent("mouseup", endX, startY),
		chromedp.Sleep(1*time.Second), // 等待验证结果
	)
	if err != nil {
		return fmt.Errorf("鼠标释放失败: %w", err)
	}
	
	log.Printf("✅ Sliding captcha handled successfully - Distance: %d pixels", distance)
	return nil
}