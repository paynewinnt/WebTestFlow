package handlers

import (
	"encoding/json"
	"strconv"
	"webtestflow/backend/internal/models"
	"webtestflow/backend/pkg/database"
	"webtestflow/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

func GetEnvironments(c *gin.Context) {
	var environments []models.Environment
	err := database.DB.Where("status = ?", 1).Order("id ASC").Find(&environments).Error
	if err != nil {
		response.InternalServerError(c, "获取环境列表失败")
		return
	}

	response.Success(c, environments)
}

func CreateEnvironment(c *gin.Context) {
	var req struct {
		Name        string      `json:"name" binding:"required,min=1,max=100"`
		Description string      `json:"description" binding:"max=500"`
		BaseURL     string      `json:"base_url" binding:"required,url"`
		Type        string      `json:"type" binding:"required,oneof=test product"`
		Headers     interface{} `json:"headers"`   // 改为 interface{} 类型
		Variables   interface{} `json:"variables"` // 改为 interface{} 类型
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Check if environment name and type combination exists
	var existingEnv models.Environment
	err := database.DB.Where("name = ? AND type = ? AND status = ?", req.Name, req.Type, 1).
		First(&existingEnv).Error
	if err == nil {
		response.BadRequest(c, "相同类型的环境名称已存在")
		return
	}

	// Convert headers to JSON string
	headersJSON := "{}"
	if req.Headers != nil {
		switch v := req.Headers.(type) {
		case string:
			// 如果是字符串，验证是否为有效JSON
			var temp interface{}
			if err := json.Unmarshal([]byte(v), &temp); err == nil {
				headersJSON = v
			} else {
				response.BadRequest(c, "Headers格式不正确，请输入有效的JSON")
				return
			}
		case map[string]interface{}:
			// 如果是对象，转换为JSON字符串
			if data, err := json.Marshal(v); err == nil {
				headersJSON = string(data)
			}
		default:
			// 其他类型，尝试转换为JSON
			if data, err := json.Marshal(v); err == nil {
				headersJSON = string(data)
			}
		}
	}

	// Convert variables to JSON string
	variablesJSON := "{}"
	if req.Variables != nil {
		switch v := req.Variables.(type) {
		case string:
			// 如果是字符串，验证是否为有效JSON
			var temp interface{}
			if err := json.Unmarshal([]byte(v), &temp); err == nil {
				variablesJSON = v
			} else {
				response.BadRequest(c, "Variables格式不正确，请输入有效的JSON")
				return
			}
		case map[string]interface{}:
			// 如果是对象，转换为JSON字符串
			if data, err := json.Marshal(v); err == nil {
				variablesJSON = string(data)
			}
		default:
			// 其他类型，尝试转换为JSON
			if data, err := json.Marshal(v); err == nil {
				variablesJSON = string(data)
			}
		}
	}

	environment := models.Environment{
		Name:        req.Name,
		Description: req.Description,
		BaseURL:     req.BaseURL,
		Type:        req.Type,
		Headers:     headersJSON,
		Variables:   variablesJSON,
		Status:      1,
	}

	err = database.DB.Create(&environment).Error
	if err != nil {
		response.InternalServerError(c, "创建环境失败")
		return
	}

	response.SuccessWithMessage(c, "创建成功", environment)
}

func GetEnvironment(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的环境ID")
		return
	}

	var environment models.Environment
	err = database.DB.Where("status = ?", 1).First(&environment, id).Error
	if err != nil {
		response.NotFound(c, "环境不存在")
		return
	}

	response.Success(c, environment)
}

func UpdateEnvironment(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的环境ID")
		return
	}

	var req struct {
		Name        string      `json:"name" binding:"omitempty,min=1,max=100"`
		Description string      `json:"description" binding:"max=500"`
		BaseURL     string      `json:"base_url" binding:"omitempty,url"`
		Type        string      `json:"type" binding:"omitempty,oneof=test product"`
		Headers     interface{} `json:"headers"`   // 改为 interface{} 类型
		Variables   interface{} `json:"variables"` // 改为 interface{} 类型
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	var environment models.Environment
	err = database.DB.Where("status = ?", 1).First(&environment, id).Error
	if err != nil {
		response.NotFound(c, "环境不存在")
		return
	}

	// Check name and type uniqueness if updating
	if req.Name != "" && req.Type != "" && (req.Name != environment.Name || req.Type != environment.Type) {
		var existingEnv models.Environment
		err := database.DB.Where("name = ? AND type = ? AND id != ? AND status = ?",
			req.Name, req.Type, id, 1).First(&existingEnv).Error
		if err == nil {
			response.BadRequest(c, "相同类型的环境名称已存在")
			return
		}
	}

	// Update basic fields
	if req.Name != "" {
		environment.Name = req.Name
	}
	if req.Description != "" {
		environment.Description = req.Description
	}
	if req.BaseURL != "" {
		environment.BaseURL = req.BaseURL
	}
	if req.Type != "" {
		environment.Type = req.Type
	}

	// Update headers if provided
	if req.Headers != nil {
		switch v := req.Headers.(type) {
		case string:
			// 如果是字符串，验证是否为有效JSON
			var temp interface{}
			if err := json.Unmarshal([]byte(v), &temp); err == nil {
				environment.Headers = v
			} else {
				response.BadRequest(c, "Headers格式不正确，请输入有效的JSON")
				return
			}
		case map[string]interface{}:
			// 如果是对象，转换为JSON字符串
			if data, err := json.Marshal(v); err == nil {
				environment.Headers = string(data)
			}
		default:
			// 其他类型，尝试转换为JSON
			if data, err := json.Marshal(v); err == nil {
				environment.Headers = string(data)
			}
		}
	}

	// Update variables if provided
	if req.Variables != nil {
		switch v := req.Variables.(type) {
		case string:
			// 如果是字符串，验证是否为有效JSON
			var temp interface{}
			if err := json.Unmarshal([]byte(v), &temp); err == nil {
				environment.Variables = v
			} else {
				response.BadRequest(c, "Variables格式不正确，请输入有效的JSON")
				return
			}
		case map[string]interface{}:
			// 如果是对象，转换为JSON字符串
			if data, err := json.Marshal(v); err == nil {
				environment.Variables = string(data)
			}
		default:
			// 其他类型，尝试转换为JSON
			if data, err := json.Marshal(v); err == nil {
				environment.Variables = string(data)
			}
		}
	}

	err = database.DB.Save(&environment).Error
	if err != nil {
		response.InternalServerError(c, "更新环境失败")
		return
	}

	response.SuccessWithMessage(c, "更新成功", environment)
}

func DeleteEnvironment(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的环境ID")
		return
	}

	var environment models.Environment
	err = database.DB.Where("status = ?", 1).First(&environment, id).Error
	if err != nil {
		response.NotFound(c, "环境不存在")
		return
	}

	// Check if environment is being used by test cases
	var testCaseCount int64
	database.DB.Model(&models.TestCase{}).Where("environment_id = ? AND status = ?", id, 1).Count(&testCaseCount)
	if testCaseCount > 0 {
		response.BadRequest(c, "该环境正在被测试用例使用，无法删除")
		return
	}

	// Check if environment is being used by test suites
	var testSuiteCount int64
	database.DB.Model(&models.TestSuite{}).Where("environment_id = ? AND status = ?", id, 1).Count(&testSuiteCount)
	if testSuiteCount > 0 {
		response.BadRequest(c, "该环境正在被测试套件使用，无法删除")
		return
	}

	// Soft delete
	environment.Status = 0
	err = database.DB.Save(&environment).Error
	if err != nil {
		response.InternalServerError(c, "删除环境失败")
		return
	}

	response.SuccessWithMessage(c, "删除成功", nil)
}
