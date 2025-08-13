package handlers

import (
	"strconv"
	"webtestflow/backend/internal/models"
	"webtestflow/backend/pkg/database"
	"webtestflow/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

func GetDevices(c *gin.Context) {
	var devices []models.Device
	err := database.DB.Where("status = ?", 1).Order("is_default DESC, id ASC").Find(&devices).Error
	if err != nil {
		response.InternalServerError(c, "获取设备列表失败")
		return
	}

	response.Success(c, devices)
}

func CreateDevice(c *gin.Context) {
	var req struct {
		Name      string `json:"name" binding:"required,min=1,max=100"`
		Width     int    `json:"width" binding:"required,min=100,max=4000"`
		Height    int    `json:"height" binding:"required,min=100,max=4000"`
		UserAgent string `json:"user_agent" binding:"required,min=10,max=500"`
		IsDefault bool   `json:"is_default"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Check if device name exists
	var existingDevice models.Device
	err := database.DB.Where("name = ? AND status = ?", req.Name, 1).First(&existingDevice).Error
	if err == nil {
		response.BadRequest(c, "设备名称已存在")
		return
	}

	// If setting as default, remove default from other devices
	if req.IsDefault {
		database.DB.Model(&models.Device{}).Where("is_default = ? AND status = ?", true, 1).
			Update("is_default", false)
	}

	device := models.Device{
		Name:      req.Name,
		Width:     req.Width,
		Height:    req.Height,
		UserAgent: req.UserAgent,
		IsDefault: req.IsDefault,
		Status:    1,
	}

	err = database.DB.Create(&device).Error
	if err != nil {
		response.InternalServerError(c, "创建设备失败")
		return
	}

	response.SuccessWithMessage(c, "创建成功", device)
}

func GetDevice(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的设备ID")
		return
	}

	var device models.Device
	err = database.DB.Where("status = ?", 1).First(&device, id).Error
	if err != nil {
		response.NotFound(c, "设备不存在")
		return
	}

	response.Success(c, device)
}

func UpdateDevice(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的设备ID")
		return
	}

	var req struct {
		Name      string `json:"name" binding:"omitempty,min=1,max=100"`
		Width     int    `json:"width" binding:"omitempty,min=100,max=4000"`
		Height    int    `json:"height" binding:"omitempty,min=100,max=4000"`
		UserAgent string `json:"user_agent" binding:"omitempty,min=10,max=500"`
		IsDefault *bool  `json:"is_default"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	var device models.Device
	err = database.DB.Where("status = ?", 1).First(&device, id).Error
	if err != nil {
		response.NotFound(c, "设备不存在")
		return
	}

	// Check name uniqueness if updating
	if req.Name != "" && req.Name != device.Name {
		var existingDevice models.Device
		err := database.DB.Where("name = ? AND id != ? AND status = ?", req.Name, id, 1).
			First(&existingDevice).Error
		if err == nil {
			response.BadRequest(c, "设备名称已存在")
			return
		}
		device.Name = req.Name
	}

	// Update fields
	if req.Width > 0 {
		device.Width = req.Width
	}
	if req.Height > 0 {
		device.Height = req.Height
	}
	if req.UserAgent != "" {
		device.UserAgent = req.UserAgent
	}

	// Handle default setting
	if req.IsDefault != nil {
		if *req.IsDefault && !device.IsDefault {
			// Remove default from other devices
			database.DB.Model(&models.Device{}).Where("is_default = ? AND status = ?", true, 1).
				Update("is_default", false)
		}
		device.IsDefault = *req.IsDefault
	}

	err = database.DB.Save(&device).Error
	if err != nil {
		response.InternalServerError(c, "更新设备失败")
		return
	}

	response.SuccessWithMessage(c, "更新成功", device)
}

func DeleteDevice(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的设备ID")
		return
	}

	var device models.Device
	err = database.DB.Where("status = ?", 1).First(&device, id).Error
	if err != nil {
		response.NotFound(c, "设备不存在")
		return
	}

	// Check if device is being used by test cases
	var testCaseCount int64
	database.DB.Model(&models.TestCase{}).Where("device_id = ? AND status = ?", id, 1).Count(&testCaseCount)
	if testCaseCount > 0 {
		response.BadRequest(c, "该设备正在被测试用例使用，无法删除")
		return
	}

	// Don't allow deleting default device if it's the only one
	if device.IsDefault {
		var deviceCount int64
		database.DB.Model(&models.Device{}).Where("status = ?", 1).Count(&deviceCount)
		if deviceCount <= 1 {
			response.BadRequest(c, "至少需要保留一个设备")
			return
		}

		// Set another device as default
		var newDefaultDevice models.Device
		database.DB.Where("id != ? AND status = ?", id, 1).First(&newDefaultDevice)
		newDefaultDevice.IsDefault = true
		database.DB.Save(&newDefaultDevice)
	}

	// Soft delete
	device.Status = 0
	err = database.DB.Save(&device).Error
	if err != nil {
		response.InternalServerError(c, "删除设备失败")
		return
	}

	response.SuccessWithMessage(c, "删除成功", nil)
}
