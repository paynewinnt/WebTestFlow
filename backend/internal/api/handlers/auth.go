package handlers

import (
	"net/http"
	"webtestflow/backend/internal/config"
	"webtestflow/backend/internal/models"
	"webtestflow/backend/pkg/auth"
	"webtestflow/backend/pkg/database"
	"webtestflow/backend/pkg/response"
	"webtestflow/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type LoginRequest struct {
	Username string `json:"username" binding:"required,min=3"`
	Password string `json:"password" binding:"required,min=6"`
}

type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type LoginResponse struct {
	Token string      `json:"token"`
	User  models.User `json:"user"`
}

func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	var user models.User
	err := database.DB.Where("username = ? OR email = ?", req.Username, req.Username).First(&user).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Unauthorized(c, "用户名或密码错误")
		} else {
			response.InternalServerError(c, "数据库查询失败")
		}
		return
	}

	if !utils.CheckPassword(req.Password, user.Password) {
		response.Unauthorized(c, "用户名或密码错误")
		return
	}

	if user.Status != 1 {
		response.Forbidden(c, "账户已被禁用")
		return
	}

	// Load config for JWT expiration time
	cfg, _ := config.LoadConfig()
	token, err := auth.GenerateToken(user.ID, user.Username, cfg.JWT.ExpireTime)
	if err != nil {
		response.InternalServerError(c, "生成令牌失败")
		return
	}

	// Clear password from response
	user.Password = ""

	loginResp := LoginResponse{
		Token: token,
		User:  user,
	}

	response.SuccessWithMessage(c, "登录成功", loginResp)
}

func Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Check if username exists
	var existingUser models.User
	err := database.DB.Where("username = ?", req.Username).First(&existingUser).Error
	if err == nil {
		response.BadRequest(c, "用户名已存在")
		return
	}

	// Check if email exists
	err = database.DB.Where("email = ?", req.Email).First(&existingUser).Error
	if err == nil {
		response.BadRequest(c, "邮箱已被注册")
		return
	}

	// Hash password
	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		response.InternalServerError(c, "密码加密失败")
		return
	}

	// Create user
	user := models.User{
		Username: req.Username,
		Email:    req.Email,
		Password: hashedPassword,
		Status:   1,
	}

	err = database.DB.Create(&user).Error
	if err != nil {
		response.InternalServerError(c, "创建用户失败")
		return
	}

	// Clear password from response
	user.Password = ""

	response.SuccessWithMessage(c, "注册成功", user)
}

func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"status":    "healthy",
			"timestamp": gin.H{},
		},
	})
}
