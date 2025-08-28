package executor

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
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
		chromedp.Sleep(2*time.Second), // 等待图片完全加载
	)
	if err != nil {
		return fmt.Errorf("验证码图片不可见: %w", err)
	}
	
	// 使用JavaScript检查图片是否真正加载完成
	var imageLoaded bool
	checkImageScript := fmt.Sprintf(`
		(function() {
			const img = document.querySelector('%s');
			if (!img) return false;
			
			// 检查图片是否存在且有尺寸
			if (img.naturalWidth === 0 || img.naturalHeight === 0) {
				console.log('Image not loaded yet, naturalWidth/Height is 0');
				return false;
			}
			
			// 检查是否是有效的图片源
			if (!img.src || img.src === '' || img.src.includes('data:image/svg+xml') || img.src.includes('placeholder')) {
				console.log('Image src is invalid or placeholder:', img.src);
				return false;
			}
			
			console.log('Image loaded successfully:', img.src, img.naturalWidth + 'x' + img.naturalHeight);
			return true;
		})();
	`, step.CaptchaSelector)
	
	// 等待图片加载完成，最多等待10秒
	for attempts := 0; attempts < 10; attempts++ {
		err = chromedp.Run(ctx,
			chromedp.Evaluate(checkImageScript, &imageLoaded),
		)
		if err != nil {
			log.Printf("⚠️ Failed to check image loaded status: %v", err)
			break
		}
		
		if imageLoaded {
			log.Printf("✅ Captcha image loaded successfully")
			break
		}
		
		log.Printf("⏳ Waiting for captcha image to load... (attempt %d/10)", attempts+1)
		err = chromedp.Run(ctx, chromedp.Sleep(1*time.Second))
		if err != nil {
			return fmt.Errorf("等待图片加载失败: %w", err)
		}
	}
	
	// 截取验证码图片，带重试机制
	var buf []byte
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		err = chromedp.Run(ctx,
			chromedp.Screenshot(step.CaptchaSelector, &buf, chromedp.NodeVisible, chromedp.ByQuery),
		)
		if err != nil {
			log.Printf("⚠️ Screenshot attempt %d failed: %v", i+1, err)
			if i < maxRetries-1 {
				chromedp.Run(ctx, chromedp.Sleep(1*time.Second))
				continue
			}
			return fmt.Errorf("截取验证码失败: %w", err)
		}
		
		// 检查截取的图片是否有效（不是空白图片）
		if len(buf) < 1000 { // 太小的图片可能是空白的
			log.Printf("⚠️ Screenshot too small (%d bytes), retrying...", len(buf))
			if i < maxRetries-1 {
				chromedp.Run(ctx, chromedp.Sleep(2*time.Second))
				continue
			}
		}
		
		log.Printf("📸 Captured captcha image, size: %d bytes", len(buf))
		break
	}
	
	// 保存验证码图片用于调试
	debugPath := fmt.Sprintf("screenshots/debug_captcha_%d.png", time.Now().Unix())
	if err := os.WriteFile(debugPath, buf, 0644); err == nil {
		log.Printf("🐛 Debug captcha image saved to: %s", debugPath)
	} else {
		log.Printf("⚠️ Failed to save debug captcha image: %v", err)
	}
	
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
	
	// 检查识别结果是否为空
	if code == "" {
		return fmt.Errorf("OCR识别结果为空，可能是验证码图片质量问题或不支持的验证码类型")
	}
	
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
	
	// 方案1：先尝试获取图片src属性
	var imgSrc string
	err = chromedp.Run(ctx,
		chromedp.AttributeValue(backgroundSelector, "src", &imgSrc, nil, chromedp.ByQuery),
	)
	
	var bgBuf []byte
	
	if err == nil && imgSrc != "" && strings.HasPrefix(imgSrc, "http") {
		// 如果有图片URL，直接下载图片
		log.Printf("📸 Found image URL: %s", imgSrc)
		resp, err := http.Get(imgSrc)
		if err == nil {
			defer resp.Body.Close()
			bgBuf, _ = io.ReadAll(resp.Body)
			log.Printf("📸 Downloaded image from URL, size: %d bytes", len(bgBuf))
		}
	}
	
	// 方案2：如果URL方式失败，尝试截图方式
	if len(bgBuf) == 0 {
		// 等待图片完全加载
		err = chromedp.Run(ctx,
			chromedp.WaitReady(backgroundSelector, chromedp.ByQuery),
			chromedp.Sleep(500*time.Millisecond), // 额外等待确保图片渲染完成
		)
		if err != nil {
			log.Printf("⚠️ Wait for image ready failed: %v", err)
		}
		
		// 尝试使用不同的截图方式
		err = chromedp.Run(ctx,
			chromedp.Screenshot(backgroundSelector, &bgBuf, chromedp.NodeVisible, chromedp.ByQuery),
		)
		if err != nil {
			// 如果节点截图失败，尝试全页面截图后裁剪
			var fullScreenshot []byte
			var box Box
			err = chromedp.Run(ctx,
				chromedp.Evaluate(fmt.Sprintf(`
					(function() {
						const elem = document.querySelector('%s');
						const rect = elem.getBoundingClientRect();
						return {
							x: rect.left + window.scrollX,
							y: rect.top + window.scrollY,
							width: rect.width,
							height: rect.height
						};
					})()
				`, backgroundSelector), &box),
				chromedp.CaptureScreenshot(&fullScreenshot),
			)
			if err == nil && len(fullScreenshot) > 0 {
				// 这里需要裁剪图片，暂时使用全图
				bgBuf = fullScreenshot
				log.Printf("📸 Using full screenshot, size: %d bytes", len(bgBuf))
			} else {
				return fmt.Errorf("截取滑块背景图失败: %w", err)
			}
		} else {
			log.Printf("📸 Captured background image via screenshot, size: %d bytes", len(bgBuf))
		}
	}
	
	// 验证图片数据
	if len(bgBuf) < 100 {
		return fmt.Errorf("截取的图片数据太小: %d bytes", len(bgBuf))
	}
	
	// 保存截图用于调试
	debugPath := fmt.Sprintf("../screenshots/debug_sliding_%d.png", time.Now().Unix())
	if err := os.WriteFile(debugPath, bgBuf, 0644); err == nil {
		log.Printf("📸 Debug image saved to: %s", debugPath)
	}
	
	// 调用OCR服务识别滑块位置
	ocrClient := captcha.NewOCRClient(os.Getenv("OCR_SERVICE_URL"))
	distance, err := ocrClient.RecognizeSliding(bgBuf, nil)
	if err != nil {
		return fmt.Errorf("滑块位置识别失败: %w", err)
	}
	
	log.Printf("📏 Sliding distance detected: %d pixels", distance)
	
	// 获取滑块和背景的位置信息
	err = chromedp.Run(ctx,
		chromedp.Evaluate(fmt.Sprintf(`
			(function() {
				// 获取实际的滑块元素
				const sliderSelectors = [
					'taro-view-core.slide.text-center',     
					'taro-view-core[class*="slide"]',
					'%s'
				];
				let slider = null;
				for(let sel of sliderSelectors) {
					const elem = document.querySelector(sel);
					if(elem && elem.getBoundingClientRect().width > 0) {
						slider = elem;
						break;
					}
				}
				
				// 获取背景图
				const bg = document.querySelector('%s');
				
				if(!slider || !bg) {
					return null;
				}
				
				const sliderRect = slider.getBoundingClientRect();
				const bgRect = bg.getBoundingClientRect();
				
				console.log('Slider rect:', sliderRect);
				console.log('Background rect:', bgRect);
				
				return {
					slider: {
						x: sliderRect.left,
						y: sliderRect.top,
						width: sliderRect.width,
						height: sliderRect.height
					},
					background: {
						x: bgRect.left,
						y: bgRect.top,
						width: bgRect.width,
						height: bgRect.height
					}
				};
			})()
		`, sliderSelector, backgroundSelector), &map[string]interface{}{}),
	)
	
	// 解析返回的坐标信息
	var boxInfo map[string]interface{}
	err = chromedp.Run(ctx,
		chromedp.Evaluate(fmt.Sprintf(`
			(function() {
				const sliderSelectors = ['taro-view-core.slide.text-center', 'taro-view-core[class*="slide"]', '%s'];
				let slider = null;
				for(let sel of sliderSelectors) {
					const elem = document.querySelector(sel);
					if(elem && elem.getBoundingClientRect().width > 0) {
						slider = elem;
						console.log('Found slider:', sel, elem.textContent);
						break;
					}
				}
				
				const bg = document.querySelector('%s');
				if(!slider || !bg) {
					return {error: 'Elements not found'};
				}
				
				const sliderRect = slider.getBoundingClientRect();
				const bgRect = bg.getBoundingClientRect();
				
				return {
					sliderX: sliderRect.left,
					sliderY: sliderRect.top,
					sliderWidth: sliderRect.width,
					sliderHeight: sliderRect.height,
					backgroundX: bgRect.left,
					backgroundY: bgRect.top,
					backgroundWidth: bgRect.width,
					backgroundHeight: bgRect.height
				};
			})()
		`, sliderSelector, backgroundSelector), &boxInfo),
	)
	if err != nil {
		return fmt.Errorf("获取位置信息失败: %w", err)
	}
	
	if errorMsg, exists := boxInfo["error"]; exists {
		return fmt.Errorf("元素查找失败: %v", errorMsg)
	}
	
	// 计算滑动坐标
	sliderX := boxInfo["sliderX"].(float64)
	sliderY := boxInfo["sliderY"].(float64)
	sliderWidth := boxInfo["sliderWidth"].(float64)
	sliderHeight := boxInfo["sliderHeight"].(float64)
	backgroundWidth := boxInfo["backgroundWidth"].(float64)
	
	startX := sliderX + sliderWidth/2
	startY := sliderY + sliderHeight/2
	
	// 智能计算实际滑动距离 - 关键修复
	// OCR返回的distance是基于背景图坐标的，需要映射到滑块轨道
	var actualDistance float64
	
	// 获取滑块轨道的实际可滑动宽度
	var trackInfo map[string]interface{}
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(function() {
				// 查找滑块轨道容器
				const trackSelectors = [
					'taro-view-core.slide-box',
					'.slide-track',
					'[class*="slide-box"]',
					'[class*="track"]'
				];
				
				let track = null;
				for(let sel of trackSelectors) {
					const elem = document.querySelector(sel);
					if(elem && elem.getBoundingClientRect().width > 0) {
						track = elem;
						console.log('Found track:', sel, elem);
						break;
					}
				}
				
				if(track) {
					const trackRect = track.getBoundingClientRect();
					return {
						trackX: trackRect.left,
						trackY: trackRect.top,
						trackWidth: trackRect.width,
						trackHeight: trackRect.height,
						found: true
					};
				}
				
				return {found: false};
			})()
		`, &trackInfo),
	)
	
	if err == nil && trackInfo["found"] == true {
		// 找到滑块轨道，计算精确映射
		trackWidth := trackInfo["trackWidth"].(float64)
		
		// OCR距离是基于背景图的比例，映射到滑块轨道
		// OCR距离 / 背景图宽度 = 实际滑动距离 / 轨道宽度
		distanceRatio := float64(distance) / backgroundWidth
		actualDistance = trackWidth * distanceRatio
		
		log.Printf("🎯 智能距离计算: OCR距离=%dpx, 背景宽度=%.0fpx, 轨道宽度=%.0fpx, 实际距离=%.0fpx", 
			distance, backgroundWidth, trackWidth, actualDistance)
	} else {
		// 备用方案：使用原OCR距离，但限制最大值
		actualDistance = float64(distance)
		if backgroundWidth > 0 {
			maxDistance := backgroundWidth * 0.8
			if actualDistance > maxDistance {
				actualDistance = maxDistance
			}
		}
		log.Printf("⚠️ 使用备用距离计算: OCR距离=%dpx, 实际距离=%.0fpx", distance, actualDistance)
	}
	
	endX := startX + actualDistance
	
	log.Printf("🎯 Sliding from (%.0f, %.0f) to (%.0f, %.0f), distance: %.0f", startX, startY, endX, startY, actualDistance)
	
	// 智能查找滑块元素 - 专门针对Taro框架
	var actualSlider string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(fmt.Sprintf(`
			(function() {
				// 针对Taro框架的滑块选择器优化
				const selectors = [
					'taro-view-core.slide.text-center',     // Taro滑块按钮
					'taro-view-core[class*="slide"]',       // 包含slide的Taro组件
					'%s',                                   // 原始选择器
					'[class*="slide-btn"]',                 // 滑块按钮类名
					'[class*="slider"]',                    // 包含slider的类名
					'div[class*="slide"]',                  // 包含slide的div
					'*:contains("拖动")',                   // 包含"拖动"文本的元素
					'*:contains(">>")',                     // 包含>>的元素
				];
				
				for(let selector of selectors) {
					try {
						let elements;
						if(selector.includes(':contains(')) {
							// 手动实现contains选择器
							const text = selector.match(/contains\("([^"]*)"\)/)[1];
							elements = Array.from(document.querySelectorAll('*')).filter(el => 
								el.textContent && el.textContent.includes(text) && el.offsetWidth > 0
							);
						} else {
							elements = document.querySelectorAll(selector);
						}
						
						for(let elem of elements) {
							const rect = elem.getBoundingClientRect();
							if(rect.width > 0 && rect.height > 0) {
								console.log('Found slider element:', selector, elem.textContent);
								return selector.includes(':contains(') ? 
									elem.tagName.toLowerCase() + (elem.className ? '.' + elem.className.split(' ').join('.') : '') :
									selector;
							}
						}
					} catch(e) {
						console.log('Selector error:', selector, e);
					}
				}
				return '%s'; // 返回原始选择器作为备用
			})()
		`, sliderSelector, sliderSelector), &actualSlider),
	)
	
	log.Printf("🎚️ Using slider selector: %s", actualSlider)
	
	// 使用Touch事件模拟滑动（适用于移动端Taro框架）
	err = chromedp.Run(ctx,
		chromedp.Evaluate(fmt.Sprintf(`
			(function() {
				console.log('Starting touch-based sliding operation...');
				const slider = document.querySelector('%s');
				if (!slider) {
					return "未找到滑块元素: %s";
				}
				
				const startX = %.0f;
				const startY = %.0f;
				const endX = %.0f;
				
				console.log('Touch slide from', startX, startY, 'to', endX, startY);
				console.log('Slider element:', slider.tagName, slider.className, slider.textContent);
				
				// 创建Touch对象
				function createTouch(x, y, identifier = 0) {
					return new Touch({
						identifier: identifier,
						target: slider,
						clientX: x,
						clientY: y,
						pageX: x,
						pageY: y,
						screenX: x,
						screenY: y,
						radiusX: 10,
						radiusY: 10,
						rotationAngle: 0,
						force: 1
					});
				}
				
				// 创建TouchList
				function createTouchList(touches) {
					const touchList = [];
					touches.forEach(touch => touchList.push(touch));
					touchList.item = function(index) { return this[index] || null; };
					touchList.length = touches.length;
					return touchList;
				}
				
				// 事件序列
				const events = [];
				
				// 1. 触摸开始
				const startTouch = createTouch(startX, startY);
				events.push(new TouchEvent('touchstart', {
					bubbles: true, cancelable: true, view: window,
					touches: createTouchList([startTouch]),
					targetTouches: createTouchList([startTouch]),
					changedTouches: createTouchList([startTouch])
				}));
				
				// 2. 分段移动
				const steps = 20;
				for(let i = 1; i <= steps; i++) {
					const currentX = startX + (endX - startX) * i / steps;
					const moveTouch = createTouch(currentX, startY);
					events.push(new TouchEvent('touchmove', {
						bubbles: true, cancelable: true, view: window,
						touches: createTouchList([moveTouch]),
						targetTouches: createTouchList([moveTouch]),
						changedTouches: createTouchList([moveTouch])
					}));
				}
				
				// 3. 触摸结束
				const endTouch = createTouch(endX, startY);
				events.push(new TouchEvent('touchend', {
					bubbles: true, cancelable: true, view: window,
					touches: createTouchList([]),
					targetTouches: createTouchList([]),
					changedTouches: createTouchList([endTouch])
				}));
				
				// 同时添加鼠标事件作为备用
				events.push(new MouseEvent('mousedown', {
					bubbles: true, cancelable: true, view: window,
					clientX: startX, clientY: startY, buttons: 1, button: 0
				}));
				
				for(let i = 1; i <= steps; i++) {
					const currentX = startX + (endX - startX) * i / steps;
					events.push(new MouseEvent('mousemove', {
						bubbles: true, cancelable: true, view: window,
						clientX: currentX, clientY: startY, buttons: 1
					}));
				}
				
				events.push(new MouseEvent('mouseup', {
					bubbles: true, cancelable: true, view: window,
					clientX: endX, clientY: startY, buttons: 0, button: 0
				}));
				
				// 依次触发事件
				let eventIndex = 0;
				const triggerNext = () => {
					if (eventIndex < events.length) {
						const event = events[eventIndex++];
						console.log('Triggering:', event.type, 'at', event.clientX || event.changedTouches?.[0]?.clientX, event.clientY || event.changedTouches?.[0]?.clientY);
						slider.dispatchEvent(event);
						setTimeout(triggerNext, 30); // 30ms间隔
					} else {
						console.log('All events triggered, checking result...');
						// 验证滑动结果
						setTimeout(() => {
							const newBox = slider.getBoundingClientRect();
							console.log('Slider position after slide:', newBox.left, newBox.top);
						}, 500);
					}
				};
				triggerNext();
				
				return "Touch滑动操作已启动";
			})()
		`, actualSlider, actualSlider, startX, startY, endX), nil),
		chromedp.Sleep(3*time.Second), // 等待滑动动画完成
	)
	if err != nil {
		// 如果JavaScript方式失败，尝试使用CDP原生方式
		log.Printf("⚠️ JavaScript滑动失败，尝试CDP方式: %v", err)
		
		// 使用CDP Input domain 方式 - 使用正确的参数
		err = chromedp.Run(ctx,
			// 按下鼠标
			chromedp.MouseEvent("mousePressed", startX, startY, chromedp.ButtonLeft),
			chromedp.Sleep(100*time.Millisecond),
		)
		if err != nil {
			return fmt.Errorf("鼠标按下失败: %w", err)
		}
		
		// 分段滑动
		steps := 10
		for i := 1; i <= steps; i++ {
			currentX := startX + (endX-startX)*float64(i)/float64(steps)
			err = chromedp.Run(ctx,
				chromedp.MouseEvent("mouseMoved", currentX, startY),
				chromedp.Sleep(30*time.Millisecond),
			)
			if err != nil {
				log.Printf("⚠️ Mouse move failed at step %d: %v", i, err)
			}
		}
		
		// 滑动结束前截图（松开鼠标前）
		beforeReleaseScreenshot := fmt.Sprintf("../screenshots/sliding_before_release_%d.png", time.Now().Unix())
		var beforeReleaseBuf []byte
		err = chromedp.Run(ctx, chromedp.CaptureScreenshot(&beforeReleaseBuf))
		if err == nil {
			if err := os.WriteFile(beforeReleaseScreenshot, beforeReleaseBuf, 0644); err == nil {
				log.Printf("📸 Sliding before release screenshot saved: %s", beforeReleaseScreenshot)
			}
		}
		
		// 释放鼠标
		err = chromedp.Run(ctx,
			chromedp.MouseEvent("mouseReleased", endX, startY, chromedp.ButtonLeft),
			chromedp.Sleep(1*time.Second),
		)
		if err != nil {
			return fmt.Errorf("鼠标释放失败: %w", err)
		}
	}
	
	// 拍摄滑动完成后的截图
	screenshotPath := fmt.Sprintf("../screenshots/sliding_completed_%d.png", time.Now().Unix())
	var screenshotBuf []byte
	err = chromedp.Run(ctx, chromedp.CaptureScreenshot(&screenshotBuf))
	if err == nil {
		if err := os.WriteFile(screenshotPath, screenshotBuf, 0644); err == nil {
			log.Printf("📸 Sliding completed screenshot saved: %s", screenshotPath)
		}
	}
	
	// 等待验证结果，可能需要额外的确认步骤
	log.Printf("⏳ Waiting for captcha verification...")
	err = chromedp.Run(ctx, chromedp.Sleep(3*time.Second))
	if err != nil {
		log.Printf("⚠️ Wait after sliding failed: %v", err)
	}
	
	// 检查是否需要点击确认按钮
	var confirmExists bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(function() {
				const confirmSelectors = [
					'button[type="submit"]',
					'button:contains("确定")',
					'button:contains("确认")', 
					'button:contains("提交")',
					'.confirm-btn',
					'.submit-btn',
					'[class*="confirm"]',
					'[class*="submit"]'
				];
				
				for(let selector of confirmSelectors) {
					const elements = document.querySelectorAll(selector);
					for(let elem of elements) {
						const rect = elem.getBoundingClientRect();
						if(rect.width > 0 && rect.height > 0 && 
						   elem.offsetParent !== null) {
							console.log('Found confirm button:', selector, elem);
							return true;
						}
					}
				}
				return false;
			})()
		`, &confirmExists),
	)
	
	if err == nil && confirmExists {
		log.Printf("🔘 Found confirm button, attempting to click...")
		// 尝试点击确认按钮
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`
				(function() {
					const confirmSelectors = [
						'button[type="submit"]',
						'button:contains("确定")',
						'button:contains("确认")', 
						'button:contains("提交")',
						'.confirm-btn',
						'.submit-btn',
						'[class*="confirm"]',
						'[class*="submit"]'
					];
					
					for(let selector of confirmSelectors) {
						const elements = document.querySelectorAll(selector);
						for(let elem of elements) {
							const rect = elem.getBoundingClientRect();
							if(rect.width > 0 && rect.height > 0 && 
							   elem.offsetParent !== null) {
								elem.click();
								return 'Clicked confirm button: ' + selector;
							}
						}
					}
					return 'No confirm button found';
				})()
			`, nil),
			chromedp.Sleep(1*time.Second),
		)
		if err != nil {
			log.Printf("⚠️ Click confirm button failed: %v", err)
		} else {
			log.Printf("✅ Confirm button clicked")
		}
	}
	
	log.Printf("✅ Sliding captcha handled successfully - Distance: %d pixels", distance)
	return nil
}