package routes

import (
	"log"
	"os"
	"webtestflow/backend/internal/api/handlers"
	"webtestflow/backend/internal/api/middleware"
	"webtestflow/backend/internal/config"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(cfg *config.Config) *gin.Engine {
	router := gin.Default()

	// Global middleware
	router.Use(middleware.CORSMiddleware())
	router.Use(gin.Recovery())

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Public routes (no auth required)
		auth := v1.Group("/auth")
		{
			auth.POST("/login", handlers.Login)
			auth.POST("/register", handlers.Register)
		}

		// Health check
		v1.GET("/health", handlers.HealthCheck)

		// WebSocket endpoint (no auth middleware for WebSocket)
		v1.GET("/ws/recording", handlers.RecordingWebSocket)

		// Protected routes (auth required)
		protected := v1.Group("")
		protected.Use(middleware.AuthMiddleware())
		{
			// User management
			users := protected.Group("/users")
			{
				users.GET("/profile", handlers.GetProfile)
				users.PUT("/profile", handlers.UpdateProfile)
				users.GET("", handlers.GetUsers)
				users.PUT("/:id/password", handlers.AdminChangePassword) // Admin only
			}

			// Environment management
			environments := protected.Group("/environments")
			{
				environments.GET("", handlers.GetEnvironments)
				environments.POST("", handlers.CreateEnvironment)
				environments.GET("/:id", handlers.GetEnvironment)
				environments.PUT("/:id", handlers.UpdateEnvironment)
				environments.DELETE("/:id", handlers.DeleteEnvironment)
			}

			// Project management
			projects := protected.Group("/projects")
			{
				projects.GET("", handlers.GetProjects)
				projects.POST("", handlers.CreateProject)
				projects.GET("/:id", handlers.GetProject)
				projects.PUT("/:id", handlers.UpdateProject)
				projects.DELETE("/:id", handlers.DeleteProject)
			}

			// Device management
			devices := protected.Group("/devices")
			{
				devices.GET("", handlers.GetDevices)
				devices.POST("", handlers.CreateDevice)
				devices.GET("/:id", handlers.GetDevice)
				devices.PUT("/:id", handlers.UpdateDevice)
				devices.DELETE("/:id", handlers.DeleteDevice)
			}

			// Test case management
			testCases := protected.Group("/test-cases")
			{
				testCases.GET("", handlers.GetTestCases)
				testCases.POST("", handlers.CreateTestCase)
				testCases.GET("/:id", handlers.GetTestCase)
				testCases.PUT("/:id", handlers.UpdateTestCase)
				testCases.DELETE("/:id", handlers.DeleteTestCase)
				testCases.POST("/:id/execute", handlers.ExecuteTestCase)
			}

			// Test suite management
			testSuites := protected.Group("/test-suites")
			{
				testSuites.GET("", handlers.GetTestSuites)
				testSuites.POST("", handlers.CreateTestSuite)
				testSuites.GET("/:id", handlers.GetTestSuite)
				testSuites.PUT("/:id", handlers.UpdateTestSuite)
				testSuites.DELETE("/:id", handlers.DeleteTestSuite)
				testSuites.POST("/:id/execute", handlers.ExecuteTestSuite)
				testSuites.POST("/:id/stop", handlers.StopTestSuite)
				testSuites.GET("/:id/executions", handlers.GetTestSuiteExecutions)
			}

			// Test execution and reporting
			executions := protected.Group("/executions")
			{
				executions.GET("", handlers.GetExecutions)
				executions.GET("/statistics", handlers.GetExecutionStatistics)
				executions.GET("/:id", handlers.GetExecution)
				executions.GET("/:id/status", handlers.GetExecutionStatus)
				executions.DELETE("/:id", handlers.DeleteExecution)
				executions.POST("/:id/stop", handlers.StopExecution)
				executions.GET("/:id/logs", handlers.GetExecutionLogs)
				executions.GET("/:id/screenshots", handlers.GetExecutionScreenshots)
				executions.GET("/:id/batch", handlers.GetCurrentBatchExecutions)
				executions.GET("/:id/report/html", handlers.DownloadExecutionReportHTML)
				executions.GET("/:id/report/pdf", handlers.DownloadExecutionReportPDF)
			}

			// Test reports
			reports := protected.Group("/reports")
			{
				reports.GET("", handlers.GetReports)
				reports.GET("/:id", handlers.GetReport)
				reports.DELETE("/:id", handlers.DeleteReport)
				reports.POST("", handlers.CreateReport)
				reports.GET("/:id/export", handlers.ExportReport)
			}

			// Recording functionality
			recording := protected.Group("/recording")
			{
				recording.POST("/start", handlers.StartRecording)
				recording.POST("/stop", handlers.StopRecording)
				recording.GET("/status", handlers.GetRecordingStatus)
				recording.POST("/save", handlers.SaveRecording)
			}

			// WebSocket moved to public routes above
		}

		// API for serving screenshots (supports daily folders)
		router.GET("/api/v1/screenshots/*filepath", func(c *gin.Context) {
			filepath := c.Param("filepath")
			// Remove leading slash from wildcard param
			if len(filepath) > 0 && filepath[0] == '/' {
				filepath = filepath[1:]
			}
			fullPath := "../screenshots/" + filepath

			// Check if file exists
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				log.Printf("Screenshot file not found: %s", fullPath)
				c.JSON(404, gin.H{"error": "Screenshot not found"})
				return
			}

			log.Printf("Serving screenshot: %s", fullPath)
			c.File(fullPath)
		})
	}

	return router
}
