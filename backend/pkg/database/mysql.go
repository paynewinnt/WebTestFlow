package database

import (
	"fmt"
	"log"
	"webtestflow/backend/internal/config"
	"webtestflow/backend/internal/models"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func InitDatabase(cfg *config.Config) error {
	var err error

	dsn := cfg.GetDSN()

	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})

	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql.DB: %w", err)
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)

	if err = sqlDB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	log.Println("Database connected successfully")

	return AutoMigrate()
}

func AutoMigrate() error {
	err := DB.AutoMigrate(
		&models.User{},
		&models.Environment{},
		&models.Project{},
		&models.Device{},
		&models.TestCase{},
		&models.TestSuite{},
		&models.TestExecution{},
		&models.TestReport{},
		&models.PerformanceMetric{},
		&models.Screenshot{},
	)

	if err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	log.Println("Database migration completed")

	return SeedDefaultData()
}

func SeedDefaultData() error {
	// Seed default devices
	devices := []models.Device{
		{
			Name:      "iPhone 12 Pro",
			Width:     390,
			Height:    844,
			UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 14_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0 Mobile/15E148 Safari/604.1",
			IsDefault: true,
			Status:    1,
		},
		{
			Name:      "iPad Pro",
			Width:     1024,
			Height:    1366,
			UserAgent: "Mozilla/5.0 (iPad; CPU OS 14_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0 Mobile/15E148 Safari/604.1",
			IsDefault: false,
			Status:    1,
		},
		{
			Name:      "Samsung Galaxy S21",
			Width:     360,
			Height:    800,
			UserAgent: "Mozilla/5.0 (Linux; Android 11; SM-G991B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/87.0.4280.141 Mobile Safari/537.36",
			IsDefault: false,
			Status:    1,
		},
		{
			Name:      "Desktop 1920x1080",
			Width:     1920,
			Height:    1080,
			UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
			IsDefault: false,
			Status:    1,
		},
	}

	for _, device := range devices {
		var existingDevice models.Device
		if err := DB.Where("name = ?", device.Name).First(&existingDevice).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				if err := DB.Create(&device).Error; err != nil {
					return fmt.Errorf("failed to create device %s: %w", device.Name, err)
				}
			}
		}
	}

	// Seed default environments
	environments := []models.Environment{
		{
			Name:        "Test Environment",
			Description: "Testing environment for development",
			BaseURL:     "https://test.example.com",
			Type:        "test",
			Headers:     `{"Content-Type": "application/json"}`,
			Variables:   `{"timeout": 30000}`,
			Status:      1,
		},
		{
			Name:        "Production Environment",
			Description: "Production environment",
			BaseURL:     "https://www.example.com",
			Type:        "product",
			Headers:     `{"Content-Type": "application/json"}`,
			Variables:   `{"timeout": 10000}`,
			Status:      1,
		},
	}

	for _, env := range environments {
		var existingEnv models.Environment
		if err := DB.Where("name = ? AND type = ?", env.Name, env.Type).First(&existingEnv).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				if err := DB.Create(&env).Error; err != nil {
					return fmt.Errorf("failed to create environment %s: %w", env.Name, err)
				}
			}
		}
	}

	log.Println("Default data seeded successfully")
	return nil
}
