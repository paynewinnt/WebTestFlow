package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"webtestflow/backend/internal/api/routes"
	"webtestflow/backend/internal/config"
	"webtestflow/backend/internal/executor"
	"webtestflow/backend/internal/services"
	"webtestflow/backend/pkg/auth"
	"webtestflow/backend/pkg/chrome"
	"webtestflow/backend/pkg/database"

	"github.com/gin-gonic/gin"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal("Failed to load configuration:", err)
	}

	// Initialize JWT
	auth.InitJWT(cfg.JWT.Secret)

	// Initialize database
	if err := database.InitDatabase(cfg); err != nil {
		log.Fatal("Failed to initialize database:", err)
	}

	// Initialize dynamic device manager
	chrome.InitializeDeviceManager(database.DB)
	log.Println("✅ Dynamic device manager initialized")

	// Initialize test executor
	executor.InitExecutor(cfg.Chrome.MaxInstances)

	// Initialize scheduler service
	if err := services.InitScheduler(); err != nil {
		log.Fatal("Failed to initialize scheduler:", err)
	}

	// Initialize status sync service
	services.InitStatusSync()

	// Set Gin mode
	gin.SetMode(cfg.Server.Mode)

	// Initialize router
	router := routes.SetupRoutes(cfg)

	// Setup graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		log.Println("Shutting down server...")

		// Stop scheduler
		if services.GlobalScheduler != nil {
			services.GlobalScheduler.Stop()
		}

		// Stop status sync service
		if services.GlobalStatusSync != nil {
			services.GlobalStatusSync.Stop()
		}

		log.Println("Server shutdown complete")
		os.Exit(0)
	}()

	// Start server
	addr := fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Server starting on %s", addr)

	if err := router.Run(addr); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
