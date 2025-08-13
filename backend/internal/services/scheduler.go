package services

import (
	"encoding/json"
	"log"
	"time"
	"webtestflow/backend/internal/executor"
	"webtestflow/backend/internal/models"
	"webtestflow/backend/pkg/database"

	"github.com/robfig/cron/v3"
)

type SchedulerService struct {
	cron *cron.Cron
}

var GlobalScheduler *SchedulerService

func InitScheduler() error {
	GlobalScheduler = &SchedulerService{
		cron: cron.New(cron.WithSeconds()),
	}

	// Load existing scheduled test suites
	err := GlobalScheduler.loadScheduledTestSuites()
	if err != nil {
		return err
	}

	// Start the cron scheduler
	GlobalScheduler.cron.Start()
	log.Println("Scheduler service initialized")

	return nil
}

func (s *SchedulerService) loadScheduledTestSuites() error {
	var testSuites []models.TestSuite
	err := database.DB.Preload("TestCases", "status = ?", 1).
		Where("schedule != '' AND schedule IS NOT NULL AND status = ?", 1).
		Find(&testSuites).Error
	if err != nil {
		return err
	}

	for _, testSuite := range testSuites {
		err := s.AddTestSuiteSchedule(testSuite)
		if err != nil {
			log.Printf("Failed to add schedule for test suite %d: %v", testSuite.ID, err)
		}
	}

	log.Printf("Loaded %d scheduled test suites", len(testSuites))
	return nil
}

func (s *SchedulerService) AddTestSuiteSchedule(testSuite models.TestSuite) error {
	if testSuite.CronExpression == "" {
		return nil
	}

	// Remove existing schedule if any
	s.RemoveTestSuiteSchedule(testSuite.ID)

	// Add new schedule
	entryID, err := s.cron.AddFunc(testSuite.CronExpression, func() {
		s.executeScheduledTestSuite(testSuite.ID)
	})
	if err != nil {
		return err
	}

	log.Printf("Added schedule for test suite %d (entry %d): %s", testSuite.ID, entryID, testSuite.CronExpression)
	return nil
}

func (s *SchedulerService) RemoveTestSuiteSchedule(testSuiteID uint) {
	// In a real implementation, you would store the entry IDs mapped to test suite IDs
	// For simplicity, we'll just log this action
	log.Printf("Removing schedule for test suite %d", testSuiteID)
}

func (s *SchedulerService) executeScheduledTestSuite(testSuiteID uint) {
	log.Printf("Executing scheduled test suite %d", testSuiteID)

	// Load test suite with test cases
	var testSuite models.TestSuite
	err := database.DB.Preload("TestCases", "status = ?", 1).
		Where("id = ? AND status = ?", testSuiteID, 1).First(&testSuite).Error
	if err != nil {
		log.Printf("Failed to load test suite %d: %v", testSuiteID, err)
		return
	}

	if len(testSuite.TestCases) == 0 {
		log.Printf("Test suite %d has no test cases", testSuiteID)
		return
	}

	// Check if executor is available
	if executor.GlobalExecutor == nil {
		log.Printf("Test executor not available for scheduled execution")
		return
	}

	runningCount := executor.GlobalExecutor.GetRunningCount()
	if runningCount+len(testSuite.TestCases) > 10 {
		log.Printf("Insufficient capacity for scheduled test suite %d (need %d, available %d)",
			testSuiteID, len(testSuite.TestCases), 10-runningCount)
		return
	}

	var executions []models.TestExecution

	// Create execution records for all test cases
	for _, testCase := range testSuite.TestCases {
		execution := models.TestExecution{
			TestCaseID:    &testCase.ID,
			TestSuiteID:   &testSuite.ID,
			ExecutionType: "test_case_internal", // 标记为内部记录
			Status:        "pending",
			StartTime:     time.Now(),
			UserID:        testSuite.UserID, // Use test suite owner as executor
			ErrorMessage:  "",
			ExecutionLogs: "[]",
			Screenshots:   "[]",
		}

		err = database.DB.Create(&execution).Error
		if err != nil {
			log.Printf("Failed to create execution record for test case %d: %v", testCase.ID, err)
			continue
		}

		executions = append(executions, execution)
	}

	// Execute all test cases asynchronously
	go func() {
		for i, execution := range executions {
			execution.Status = "running"
			database.DB.Save(&execution)

			// Load test case with relations
			var testCase models.TestCase
			database.DB.Preload("Environment").Preload("Device").
				First(&testCase, *execution.TestCaseID)

			resultChan := executor.GlobalExecutor.ExecuteTestCase(&execution, &testCase)
			result := <-resultChan

			// Update execution with result
			if result.Success {
				execution.Status = "passed"
			} else {
				execution.Status = "failed"
				execution.ErrorMessage = result.ErrorMessage
			}

			now := time.Now()
			execution.EndTime = &now
			execution.Duration = int(now.Sub(execution.StartTime).Seconds())

			// Save logs and screenshots
			if logsJSON, err := json.Marshal(result.Logs); err == nil {
				execution.ExecutionLogs = string(logsJSON)
			}
			if screenshotsJSON, err := json.Marshal(result.Screenshots); err == nil {
				execution.Screenshots = string(screenshotsJSON)
			}

			database.DB.Save(&execution)

			// Notify executor that database update is complete
			if executor.GlobalExecutor != nil {
				executor.GlobalExecutor.NotifyExecutionComplete(execution.ID)
			}

			// Save performance metrics if available
			if result.Metrics != nil {
				result.Metrics.ExecutionID = execution.ID
				database.DB.Create(result.Metrics)
			}

			executions[i] = execution
		}

		// Create test report for scheduled execution
		s.createScheduledTestReport(testSuite, executions)
	}()

	log.Printf("Started scheduled execution for test suite %d with %d test cases", testSuiteID, len(executions))
}

func (s *SchedulerService) createScheduledTestReport(testSuite models.TestSuite, executions []models.TestExecution) {
	// Calculate statistics
	var totalCases, passedCases, failedCases, errorCases int
	var minStartTime, maxEndTime time.Time
	var totalDuration int

	for i, execution := range executions {
		totalCases++

		switch execution.Status {
		case "passed":
			passedCases++
		case "failed":
			failedCases++
		case "error":
			errorCases++
		}

		if i == 0 || execution.StartTime.Before(minStartTime) {
			minStartTime = execution.StartTime
		}

		if execution.EndTime != nil {
			if i == 0 || execution.EndTime.After(maxEndTime) {
				maxEndTime = *execution.EndTime
			}
		}

		totalDuration += execution.Duration
	}

	// Set default end time if no executions have end time
	if maxEndTime.IsZero() {
		maxEndTime = time.Now()
	}

	// Calculate total duration
	reportDuration := int(maxEndTime.Sub(minStartTime).Seconds())
	if reportDuration <= 0 {
		reportDuration = totalDuration
	}

	// Create report
	reportName := testSuite.Name + " - 定时执行 " + time.Now().Format("2006-01-02 15:04:05")
	report := models.TestReport{
		Name:        reportName,
		ProjectID:   testSuite.ProjectID,
		TestSuiteID: &testSuite.ID,
		TotalCases:  totalCases,
		PassedCases: passedCases,
		FailedCases: failedCases,
		ErrorCases:  errorCases,
		StartTime:   minStartTime,
		EndTime:     maxEndTime,
		Duration:    reportDuration,
		Status:      "completed",
		UserID:      testSuite.UserID,
	}

	err := database.DB.Create(&report).Error
	if err != nil {
		log.Printf("Failed to create scheduled test report: %v", err)
		return
	}

	// Associate executions with report
	err = database.DB.Model(&report).Association("Executions").Replace(executions)
	if err != nil {
		log.Printf("Failed to associate executions with report: %v", err)
	}

	log.Printf("Created scheduled test report %d for test suite %d", report.ID, testSuite.ID)
}

func (s *SchedulerService) Stop() {
	if s.cron != nil {
		s.cron.Stop()
		log.Println("Scheduler service stopped")
	}
}

// Public functions for managing schedules
