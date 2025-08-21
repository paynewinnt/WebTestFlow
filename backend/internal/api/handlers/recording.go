package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"webtestflow/backend/internal/models"
	"webtestflow/backend/internal/recorder"
	"webtestflow/backend/pkg/chrome"
	"webtestflow/backend/pkg/database"
	"webtestflow/backend/pkg/response"
	"webtestflow/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func StartRecording(c *gin.Context) {
	var req struct {
		EnvironmentID uint `json:"environment_id" binding:"required"`
		DeviceID      uint `json:"device_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Get environment info
	var environment models.Environment
	err := database.DB.Where("status = ?", 1).First(&environment, req.EnvironmentID).Error
	if err != nil {
		response.NotFound(c, "测试环境不存在")
		return
	}

	// Get device info
	var device models.Device
	err = database.DB.Where("status = ?", 1).First(&device, req.DeviceID).Error
	if err != nil {
		response.NotFound(c, "设备不存在")
		return
	}

	// Generate session ID
	sessionID := uuid.New().String()

	// Use ChromeDP's built-in device info - just pass device name
	deviceInfo := chrome.DeviceInfo{
		Name: device.Name, // ChromeDP will handle the rest
	}

	// Start recording with environment's base URL
	err = recorder.Manager.StartRecording(sessionID, environment.BaseURL, deviceInfo)
	if err != nil {
		response.InternalServerError(c, "启动录制失败: "+err.Error())
		return
	}

	response.SuccessWithMessage(c, "录制已启动", gin.H{
		"session_id": sessionID,
	})
}

func StopRecording(c *gin.Context) {
	var req struct {
		SessionID string `json:"session_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	err := recorder.Manager.StopRecording(req.SessionID)
	if err != nil {
		response.InternalServerError(c, "停止录制失败: "+err.Error())
		return
	}

	response.SuccessWithMessage(c, "录制已停止", nil)
}

func GetRecordingStatus(c *gin.Context) {
	sessionID := c.Query("session_id")
	if sessionID == "" {
		response.BadRequest(c, "session_id is required")
		return
	}

	isRecording, steps, err := recorder.Manager.GetRecordingStatus(sessionID)
	if err != nil {
		response.NotFound(c, "录制会话不存在")
		return
	}

	// Ensure steps is never nil
	if steps == nil {
		steps = make([]recorder.RecordStep, 0)
	}

	response.Success(c, gin.H{
		"is_recording": isRecording,
		"steps":        steps,
	})
}

func SaveRecording(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var req struct {
		SessionID      string              `json:"session_id" binding:"required"`
		Name           string              `json:"name" binding:"required,min=1,max=200"`
		Description    string              `json:"description" binding:"max=1000"`
		ProjectID      uint                `json:"project_id" binding:"required"`
		EnvironmentID  uint                `json:"environment_id" binding:"required"`
		DeviceID       uint                `json:"device_id" binding:"required"`
		ExpectedResult string              `json:"expected_result" binding:"max=1000"`
		Tags           string              `json:"tags" binding:"max=500"`
		Priority       int                 `json:"priority" binding:"min=1,max=3"`
		Steps          []models.TestStep   `json:"steps"` // 支持接收包含验证码标记的步骤
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Verify project exists and user has permission
	if !utils.HasPermissionOnProject(userID.(uint), req.ProjectID) {
		response.NotFound(c, "项目不存在或无权限")
		return
	}

	var project models.Project
	err := database.DB.Where("id = ? AND status = ?", req.ProjectID, 1).First(&project).Error
	if err != nil {
		response.NotFound(c, "项目不存在")
		return
	}

	// Verify environment exists
	var environment models.Environment
	err = database.DB.Where("id = ? AND status = ?", req.EnvironmentID, 1).First(&environment).Error
	if err != nil {
		response.NotFound(c, "环境不存在")
		return
	}

	// Verify device exists
	var device models.Device
	err = database.DB.Where("id = ? AND status = ?", req.DeviceID, 1).First(&device).Error
	if err != nil {
		response.NotFound(c, "设备不存在")
		return
	}

	var stepsJSON []byte
	
	// 优先使用前端传递的包含验证码标记的步骤数据
	if len(req.Steps) > 0 {
		log.Printf("📋 使用前端传递的步骤数据，共 %d 个步骤（包含验证码标记）", len(req.Steps))
		var err error
		stepsJSON, err = json.Marshal(req.Steps)
		if err != nil {
			response.InternalServerError(c, "处理步骤数据失败")
			return
		}
	} else {
		// 回退到从录制管理器获取原始步骤数据
		log.Printf("📋 前端未传递步骤数据，从录制管理器获取原始步骤")
		isRecording, steps, err := recorder.Manager.GetRecordingStatus(req.SessionID)
		if err != nil {
			response.NotFound(c, "录制会话不存在")
			return
		}

		if isRecording {
			response.BadRequest(c, "请先停止录制")
			return
		}

		// Ensure steps is never nil
		if steps == nil {
			steps = make([]recorder.RecordStep, 0)
		}

		if len(steps) == 0 {
			response.BadRequest(c, "没有录制到任何操作步骤")
			return
		}

		// Convert steps to JSON
		stepsJSON, err = json.Marshal(steps)
		if err != nil {
			response.InternalServerError(c, "保存步骤数据失败")
			return
		}
	}

	// Create test case
	testCase := models.TestCase{
		Name:           req.Name,
		Description:    req.Description,
		ProjectID:      req.ProjectID,
		EnvironmentID:  req.EnvironmentID,
		DeviceID:       req.DeviceID,
		Steps:          string(stepsJSON),
		ExpectedResult: req.ExpectedResult,
		Tags:           req.Tags,
		Priority:       req.Priority,
		Status:         1,
		UserID:         userID.(uint),
	}

	err = database.DB.Create(&testCase).Error
	if err != nil {
		response.InternalServerError(c, "保存测试用例失败")
		return
	}

	// Load relations for response
	database.DB.Preload("Project").Preload("Environment").Preload("Device").Preload("User").
		First(&testCase, testCase.ID)

	// Clear user password
	testCase.User.Password = ""

	// Clean up recording session
	recorder.Manager.CleanupRecording(req.SessionID)

	response.SuccessWithMessage(c, "测试用例保存成功", testCase)
}

func RecordingWebSocket(c *gin.Context) {
	sessionID := c.Query("session_id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id is required"})
		return
	}

	// For WebSocket connections, we can skip authentication since the session itself
	// is created by authenticated users and serves as implicit authorization

	// Upgrade connection to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	// Get recorder for session
	chromeRecorder, exists := recorder.Manager.GetRecorder(sessionID)
	if !exists {
		conn.WriteJSON(gin.H{"error": "Recording session not found"})
		return
	}

	// Set WebSocket connection
	chromeRecorder.SetWebSocketConnection(conn)

	// Keep connection alive and handle messages
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			break
		}
	}
}
