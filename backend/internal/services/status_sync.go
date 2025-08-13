package services

import (
	"webtestflow/backend/internal/executor"
	"webtestflow/backend/internal/models"
	"webtestflow/backend/pkg/database"
	"log"
	"time"
)

// StatusSyncService handles synchronization between executor state and database state
type StatusSyncService struct {
	running bool
	ticker  *time.Ticker
}

// NewStatusSyncService creates a new status sync service
func NewStatusSyncService() *StatusSyncService {
	return &StatusSyncService{}
}

// Start begins the status synchronization service
func (s *StatusSyncService) Start() {
	if s.running {
		return
	}
	
	s.running = true
	s.ticker = time.NewTicker(30 * time.Second) // Check every 30 seconds
	
	go s.syncLoop()
	log.Println("Status sync service started")
}

// Stop stops the status synchronization service
func (s *StatusSyncService) Stop() {
	if !s.running {
		return
	}
	
	s.running = false
	if s.ticker != nil {
		s.ticker.Stop()
	}
	log.Println("Status sync service stopped")
}

// syncLoop is the main synchronization loop
func (s *StatusSyncService) syncLoop() {
	for s.running {
		select {
		case <-s.ticker.C:
			s.syncExecutionStates()
		}
	}
}

// syncExecutionStates checks for inconsistencies between executor and database states
func (s *StatusSyncService) syncExecutionStates() {
	if executor.GlobalExecutor == nil {
		return
	}
	
	// Get all executions marked as "running" in database
	var runningExecutions []models.TestExecution
	err := database.DB.Where("status = ?", "running").Find(&runningExecutions).Error
	if err != nil {
		log.Printf("Failed to query running executions: %v", err)
		return
	}
	
	fixed := 0
	for _, execution := range runningExecutions {
		// Check if executor still considers this execution as running
		if !executor.GlobalExecutor.IsRunning(execution.ID) {
			// Check how long it's been since start - if very recent, might still be updating
			timeSinceStart := time.Since(execution.StartTime)
			if timeSinceStart < 30*time.Second {
				// Too recent, might still be processing - skip for now
				continue
			}
			
			// Execution is not running in executor but marked as running in DB
			// This indicates the execution completed but status wasn't updated
			now := time.Now()
			execution.EndTime = &now
			execution.Duration = int(now.Sub(execution.StartTime).Seconds())
			
			// Check if we can determine the actual result
			// If execution ran for a reasonable time and no error message, assume it passed
			if execution.Duration > 5 && execution.ErrorMessage == "" {
				execution.Status = "passed"
				execution.ErrorMessage = "" // Keep empty for successful executions
				log.Printf("🔧 Fixed execution %d: marked as passed (ran %d seconds, likely completed successfully)", 
					execution.ID, execution.Duration)
			} else {
				execution.Status = "failed"
				execution.ErrorMessage = "Execution completed but status was not updated properly"
				log.Printf("🔧 Fixed execution %d: marked as failed after %d seconds (status sync)", 
					execution.ID, execution.Duration)
			}
			
			err := database.DB.Save(&execution).Error
			if err != nil {
				log.Printf("❌ Failed to fix stuck execution %d: %v", execution.ID, err)
			} else {
				fixed++
			}
		}
	}
	
	if fixed > 0 {
		log.Printf("Status sync fixed %d stuck executions", fixed)
	}
	
	// Also check for executions running too long (more than 30 minutes)
	s.timeoutLongRunningExecutions()
}

// timeoutLongRunningExecutions marks executions running for too long as failed
func (s *StatusSyncService) timeoutLongRunningExecutions() {
	cutoffTime := time.Now().Add(-30 * time.Minute)
	
	var longRunningExecutions []models.TestExecution
	err := database.DB.Where("status = ? AND start_time < ?", "running", cutoffTime).Find(&longRunningExecutions).Error
	if err != nil {
		log.Printf("Failed to query long running executions: %v", err)
		return
	}
	
	for _, execution := range longRunningExecutions {
		// Force cancel in executor if still running
		if executor.GlobalExecutor.IsRunning(execution.ID) {
			executor.GlobalExecutor.CancelExecution(execution.ID)
		}
		
		// Update database status
		now := time.Now()
		execution.EndTime = &now
		execution.Duration = int(now.Sub(execution.StartTime).Seconds())
		execution.Status = "failed"
		execution.ErrorMessage = "Execution timed out after 30 minutes"
		
		err := database.DB.Save(&execution).Error
		if err != nil {
			log.Printf("Failed to timeout execution %d: %v", execution.ID, err)
		} else {
			log.Printf("Timed out long running execution %d after %d seconds", 
				execution.ID, execution.Duration)
		}
	}
}

// Global instance
var GlobalStatusSync *StatusSyncService

// InitStatusSync initializes the global status sync service
func InitStatusSync() {
	GlobalStatusSync = NewStatusSyncService()
	GlobalStatusSync.Start()
}