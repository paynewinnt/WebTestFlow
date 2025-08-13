package models

import (
	"time"
	"encoding/json"
	"gorm.io/gorm"
)

type BaseModel struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

type User struct {
	BaseModel
	Username string `json:"username" gorm:"uniqueIndex;size:100;not null"`
	Email    string `json:"email" gorm:"uniqueIndex;size:100;not null"`
	Password string `json:"-" gorm:"size:255;not null"`
	Avatar   string `json:"avatar" gorm:"size:255"`
	Status   int    `json:"status" gorm:"default:1"` // 1:active, 0:inactive
}

type Environment struct {
	BaseModel
	Name        string `json:"name" gorm:"size:100;not null"`
	Description string `json:"description" gorm:"size:500"`
	BaseURL     string `json:"base_url" gorm:"size:500;not null"`
	Type        string `json:"type" gorm:"size:20;not null"` // test, product
	Headers     string `json:"headers" gorm:"type:text"`     // JSON format
	Variables   string `json:"variables" gorm:"type:text"`   // JSON format
	Status      int    `json:"status" gorm:"default:1"`
}

type Project struct {
	BaseModel
	Name        string `json:"name" gorm:"size:100;not null"`
	Description string `json:"description" gorm:"size:500"`
	UserID      uint   `json:"user_id" gorm:"not null"`
	User        User   `json:"user" gorm:"foreignKey:UserID"`
	Status      int    `json:"status" gorm:"default:1"`
}

type Device struct {
	BaseModel
	Name      string `json:"name" gorm:"size:100;not null"`
	Width     int    `json:"width" gorm:"not null"`
	Height    int    `json:"height" gorm:"not null"`
	UserAgent string `json:"user_agent" gorm:"size:500"`
	IsDefault bool   `json:"is_default" gorm:"default:false"`
	Status    int    `json:"status" gorm:"default:1"`
}

type TestStep struct {
	Type        string                 `json:"type"`        // click, input, scroll, drag, etc.
	Selector    string                 `json:"selector"`    // CSS selector
	Value       string                 `json:"value"`       // Input value for input type
	Coordinates map[string]interface{} `json:"coordinates"` // x, y coordinates
	Options     map[string]interface{} `json:"options"`     // Additional options
	Timestamp   int64                  `json:"timestamp"`
	Screenshot  string                 `json:"screenshot"`  // Screenshot path if needed
}

type TestCase struct {
	BaseModel
	Name            string    `json:"name" gorm:"size:200;not null"`
	Description     string    `json:"description" gorm:"size:1000"`
	ProjectID       uint      `json:"project_id" gorm:"not null"`
	Project         Project   `json:"project" gorm:"foreignKey:ProjectID"`
	EnvironmentID   uint      `json:"environment_id" gorm:"not null"`
	Environment     Environment `json:"environment" gorm:"foreignKey:EnvironmentID"`
	DeviceID        uint      `json:"device_id" gorm:"not null"`
	Device          Device    `json:"device" gorm:"foreignKey:DeviceID"`
	Steps           string    `json:"steps" gorm:"type:longtext"` // JSON format TestStep array
	ExpectedResult  string    `json:"expected_result" gorm:"size:1000"`
	Tags            string    `json:"tags" gorm:"size:500"`
	Priority        int       `json:"priority" gorm:"default:1"` // 1:low, 2:medium, 3:high
	Status          int       `json:"status" gorm:"default:1"`   // 1:active, 0:inactive
	UserID          uint      `json:"user_id" gorm:"not null"`
	User            User      `json:"user" gorm:"foreignKey:UserID"`
}

func (tc *TestCase) GetSteps() ([]TestStep, error) {
	var steps []TestStep
	if tc.Steps == "" {
		return steps, nil
	}
	err := json.Unmarshal([]byte(tc.Steps), &steps)
	return steps, err
}


type TestSuite struct {
	BaseModel
	Name            string      `json:"name" gorm:"size:200;not null"`
	Description     string      `json:"description" gorm:"size:1000"`
	ProjectID       uint        `json:"project_id" gorm:"not null"`
	Project         Project     `json:"project" gorm:"foreignKey:ProjectID"`
	EnvironmentID   uint        `json:"environment_id" gorm:"not null"`
	Environment     Environment `json:"environment" gorm:"foreignKey:EnvironmentID"`
	TestCases       []TestCase  `json:"test_cases" gorm:"many2many:test_suite_cases;"`
	TestCaseCount   int         `json:"test_case_count" gorm:"-"` // Virtual field for count
	CronExpression  string      `json:"cron_expression" gorm:"size:100"` // New cron field
	IsParallel      bool        `json:"is_parallel" gorm:"default:false"`
	TimeoutMinutes  int         `json:"timeout_minutes" gorm:"default:60"`
	Tags            string      `json:"tags" gorm:"size:500"`
	Priority        int         `json:"priority" gorm:"default:2"` // 1:low, 2:medium, 3:high
	Status          int         `json:"status" gorm:"default:1"`
	UserID          uint        `json:"user_id" gorm:"not null"`
	User            User        `json:"user" gorm:"foreignKey:UserID"`
}

type TestExecution struct {
	BaseModel
	TestCaseID     *uint      `json:"test_case_id"` // nullable for test suite executions
	TestCase       TestCase   `json:"test_case" gorm:"foreignKey:TestCaseID"`
	TestSuiteID    *uint      `json:"test_suite_id"` // nullable for single test execution
	TestSuite      TestSuite  `json:"test_suite" gorm:"foreignKey:TestSuiteID"`
	ParentExecutionID *uint   `json:"parent_execution_id"` // For test_case_internal, points to suite execution
	ExecutionType  string     `json:"execution_type"` // test_case, test_suite, test_case_internal
	Status         string     `json:"status"`         // pending, running, passed, failed, cancelled
	StartTime      time.Time  `json:"start_time"`
	EndTime        *time.Time `json:"end_time"`
	Duration       int        `json:"duration"`       // in milliseconds
	TotalCount     int        `json:"total_count"`    // For test suite executions
	PassedCount    int        `json:"passed_count"`   // For test suite executions
	FailedCount    int        `json:"failed_count"`   // For test suite executions
	ErrorMessage   string     `json:"error_message" gorm:"type:text"`
	ExecutionLogs  string     `json:"execution_logs" gorm:"type:longtext"` // JSON format
	Screenshots    string     `json:"screenshots" gorm:"type:text"`        // JSON array of screenshot paths
	UserID         uint       `json:"user_id" gorm:"not null"`
	User           User       `json:"user" gorm:"foreignKey:UserID"`
}

type TestReport struct {
	BaseModel
	Name          string         `json:"name" gorm:"size:200;not null"`
	ProjectID     uint           `json:"project_id" gorm:"not null"`
	Project       Project        `json:"project" gorm:"foreignKey:ProjectID"`
	TestSuiteID   *uint          `json:"test_suite_id"`
	TestSuite     TestSuite      `json:"test_suite" gorm:"foreignKey:TestSuiteID"`
	Executions    []TestExecution `json:"executions" gorm:"many2many:test_report_executions;"`
	TotalCases    int            `json:"total_cases"`
	PassedCases   int            `json:"passed_cases"`
	FailedCases   int            `json:"failed_cases"`
	ErrorCases    int            `json:"error_cases"`
	StartTime     time.Time      `json:"start_time"`
	EndTime       time.Time      `json:"end_time"`
	Duration      int            `json:"duration"` // in seconds
	Status        string         `json:"status"`   // completed, running, error
	UserID        uint           `json:"user_id" gorm:"not null"`
	User          User           `json:"user" gorm:"foreignKey:UserID"`
}

type PerformanceMetric struct {
	BaseModel
	ExecutionID       uint    `json:"execution_id" gorm:"not null"`
	Execution         TestExecution `json:"execution" gorm:"foreignKey:ExecutionID"`
	PageLoadTime      int     `json:"page_load_time"`      // milliseconds
	DOMContentLoaded  int     `json:"dom_content_loaded"`  // milliseconds
	FirstPaint        int     `json:"first_paint"`         // milliseconds
	FirstContentfulPaint int  `json:"first_contentful_paint"` // milliseconds
	MemoryUsage       float64 `json:"memory_usage"`        // MB
	NetworkRequests   int     `json:"network_requests"`
	NetworkTime       int     `json:"network_time"`        // milliseconds
	JSHeapSize        float64 `json:"js_heap_size"`        // MB
}

type Screenshot struct {
	BaseModel
	ExecutionID uint          `json:"execution_id" gorm:"not null"`
	Execution   TestExecution `json:"execution" gorm:"foreignKey:ExecutionID"`
	StepIndex   int           `json:"step_index"`           // Which step this screenshot belongs to
	Type        string        `json:"type"`                 // before, after, error
	FilePath    string        `json:"file_path" gorm:"size:500;not null"`
	FileName    string        `json:"file_name" gorm:"size:255;not null"`
	FileSize    int64         `json:"file_size"`
}