package captcha

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OCRClient OCR识别客户端
type OCRClient struct {
	serviceURL string
	client     *http.Client
}

// OCRResponse OCR识别响应
type OCRResponse struct {
	Success    bool    `json:"success"`
	Code       string  `json:"code"`
	Confidence float64 `json:"confidence"`
	Error      string  `json:"error,omitempty"`
}

// SlideResponse 滑块识别响应
type SlideResponse struct {
	Success  bool   `json:"success"`
	Distance int    `json:"distance"`
	Error    string `json:"error,omitempty"`
}

// NewOCRClient 创建OCR客户端
func NewOCRClient(serviceURL string) *OCRClient {
	if serviceURL == "" {
		serviceURL = "http://localhost:8888"
	}
	return &OCRClient{
		serviceURL: serviceURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// RecognizeImage 识别图片验证码
func (c *OCRClient) RecognizeImage(imageData []byte) (string, error) {
	// 将图片转换为base64
	imageBase64 := base64.StdEncoding.EncodeToString(imageData)
	
	// 构建请求
	reqBody := map[string]string{
		"image": imageBase64,
	}
	
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %v", err)
	}
	
	// 发送请求
	resp, err := c.client.Post(
		c.serviceURL+"/recognize",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return "", fmt.Errorf("请求OCR服务失败: %v", err)
	}
	defer resp.Body.Close()
	
	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %v", err)
	}
	
	// 解析响应
	var ocrResp OCRResponse
	if err := json.Unmarshal(body, &ocrResp); err != nil {
		return "", fmt.Errorf("解析响应失败: %v", err)
	}
	
	if !ocrResp.Success {
		return "", fmt.Errorf("OCR识别失败: %s", ocrResp.Error)
	}
	
	return ocrResp.Code, nil
}

// RecognizeBase64 识别base64格式的验证码
func (c *OCRClient) RecognizeBase64(imageBase64 string) (string, error) {
	// 解码base64
	imageData, err := base64.StdEncoding.DecodeString(imageBase64)
	if err != nil {
		// 尝试去除data:image前缀后再解码
		if idx := bytes.Index([]byte(imageBase64), []byte(",")); idx > 0 {
			imageBase64 = imageBase64[idx+1:]
			imageData, err = base64.StdEncoding.DecodeString(imageBase64)
			if err != nil {
				return "", fmt.Errorf("解码base64失败: %v", err)
			}
		} else {
			return "", fmt.Errorf("解码base64失败: %v", err)
		}
	}
	
	return c.RecognizeImage(imageData)
}

// RecognizeSliding 识别滑块验证码
func (c *OCRClient) RecognizeSliding(backgroundData, sliderData []byte) (int, error) {
	// 构建请求
	reqBody := map[string]string{
		"background": base64.StdEncoding.EncodeToString(backgroundData),
	}
	
	if sliderData != nil {
		reqBody["slider"] = base64.StdEncoding.EncodeToString(sliderData)
	}
	
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return 0, fmt.Errorf("序列化请求失败: %v", err)
	}
	
	// 发送请求
	resp, err := c.client.Post(
		c.serviceURL+"/recognize/sliding",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return 0, fmt.Errorf("请求OCR服务失败: %v", err)
	}
	defer resp.Body.Close()
	
	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("读取响应失败: %v", err)
	}
	
	// 解析响应
	var slideResp SlideResponse
	if err := json.Unmarshal(body, &slideResp); err != nil {
		return 0, fmt.Errorf("解析响应失败: %v", err)
	}
	
	if !slideResp.Success {
		return 0, fmt.Errorf("滑块识别失败: %s", slideResp.Error)
	}
	
	return slideResp.Distance, nil
}

// HealthCheck 健康检查
func (c *OCRClient) HealthCheck() error {
	resp, err := c.client.Get(c.serviceURL + "/health")
	if err != nil {
		return fmt.Errorf("OCR服务不可用: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("OCR服务状态异常: %d", resp.StatusCode)
	}
	
	return nil
}