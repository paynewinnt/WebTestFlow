package handlers

import (
	"webtestflow/backend/internal/models"
	"webtestflow/backend/pkg/database"
	"webtestflow/backend/pkg/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

func GetProjects(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 10
	}

	var projects []models.Project
	var total int64

	// Count total
	database.DB.Model(&models.Project{}).Where("status = ?", 1).Count(&total)

	// Get paginated projects with user info
	offset := (page - 1) * pageSize
	err := database.DB.Preload("User").Where("status = ?", 1).
		Offset(offset).Limit(pageSize).Find(&projects).Error
	if err != nil {
		response.InternalServerError(c, "获取项目列表失败")
		return
	}

	// Clear user passwords
	for i := range projects {
		projects[i].User.Password = ""
	}

	response.Page(c, projects, total, page, pageSize)
}

func CreateProject(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var req struct {
		Name        string `json:"name" binding:"required,min=1,max=100"`
		Description string `json:"description" binding:"max=500"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Check if project name exists for this user
	var existingProject models.Project
	err := database.DB.Where("name = ? AND user_id = ? AND status = ?", req.Name, userID, 1).
		First(&existingProject).Error
	if err == nil {
		response.BadRequest(c, "项目名称已存在")
		return
	}

	project := models.Project{
		Name:        req.Name,
		Description: req.Description,
		UserID:      userID.(uint),
		Status:      1,
	}

	err = database.DB.Create(&project).Error
	if err != nil {
		response.InternalServerError(c, "创建项目失败")
		return
	}

	// Load user info
	database.DB.Preload("User").First(&project, project.ID)
	project.User.Password = ""

	response.SuccessWithMessage(c, "创建成功", project)
}

func GetProject(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的项目ID")
		return
	}

	var project models.Project
	err = database.DB.Preload("User").Where("status = ?", 1).First(&project, id).Error
	if err != nil {
		response.NotFound(c, "项目不存在")
		return
	}

	project.User.Password = ""
	response.Success(c, project)
}

func UpdateProject(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的项目ID")
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var req struct {
		Name        string `json:"name" binding:"omitempty,min=1,max=100"`
		Description string `json:"description" binding:"max=500"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	var project models.Project
	err = database.DB.Where("id = ? AND user_id = ? AND status = ?", id, userID, 1).
		First(&project).Error
	if err != nil {
		response.NotFound(c, "项目不存在或无权限")
		return
	}

	// Check name uniqueness if updating
	if req.Name != "" && req.Name != project.Name {
		var existingProject models.Project
		err := database.DB.Where("name = ? AND user_id = ? AND id != ? AND status = ?", 
			req.Name, userID, id, 1).First(&existingProject).Error
		if err == nil {
			response.BadRequest(c, "项目名称已存在")
			return
		}
		project.Name = req.Name
	}

	if req.Description != "" {
		project.Description = req.Description
	}

	err = database.DB.Save(&project).Error
	if err != nil {
		response.InternalServerError(c, "更新项目失败")
		return
	}

	// Load user info
	database.DB.Preload("User").First(&project, project.ID)
	project.User.Password = ""

	response.SuccessWithMessage(c, "更新成功", project)
}

func DeleteProject(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的项目ID")
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var project models.Project
	err = database.DB.Where("id = ? AND user_id = ? AND status = ?", id, userID, 1).
		First(&project).Error
	if err != nil {
		response.NotFound(c, "项目不存在或无权限")
		return
	}

	// Soft delete by setting status to 0
	project.Status = 0
	err = database.DB.Save(&project).Error
	if err != nil {
		response.InternalServerError(c, "删除项目失败")
		return
	}

	response.SuccessWithMessage(c, "删除成功", nil)
}