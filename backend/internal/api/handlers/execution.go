package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
	"webtestflow/backend/internal/executor"
	"webtestflow/backend/internal/models"
	"webtestflow/backend/pkg/chrome"
	"webtestflow/backend/pkg/database"
	"webtestflow/backend/pkg/response"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/gin-gonic/gin"
)

func GetExecutions(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	name := c.Query("name")
	status := c.Query("status")
	projectID := c.Query("project_id")
	environmentID := c.Query("environment_id")
	testCaseID := c.Query("test_case_id")
	executionType := c.Query("execution_type")
	includeInternal := c.Query("include_internal") // 新参数：是否包含内部执行记录
	parentExecutionID := c.Query("parent_execution_id") // 新参数：父执行ID过滤
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	// 对于有parent_execution_id的查询，允许更大的页面大小以获取所有子执行记录
	if parentExecutionID != "" && pageSize > 1000 {
		pageSize = 1000
	} else if parentExecutionID == "" && pageSize > 100 {
		pageSize = 10
	}

	var executions []models.TestExecution
	var total int64

	query := database.DB.Model(&models.TestExecution{})
	// 根据参数决定是否排除内部执行记录
	if includeInternal != "true" {
		query = query.Where("execution_type != ?", "test_case_internal")
	}
	
	// Apply filters
	if name != "" {
		// Filter by test case name or test suite name
		query = query.Where(
			"(test_case_id IN (SELECT id FROM test_cases WHERE name LIKE ?)) OR " +
			"(test_suite_id IN (SELECT id FROM test_suites WHERE name LIKE ?))",
			"%"+name+"%", "%"+name+"%",
		)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if projectID != "" {
		// Filter by project through test case or test suite
		query = query.Where(
			"(test_case_id IN (SELECT id FROM test_cases WHERE project_id = ?)) OR " +
			"(test_suite_id IN (SELECT id FROM test_suites WHERE project_id = ?))",
			projectID, projectID,
		)
	}
	if environmentID != "" {
		// Filter by environment through test case or test suite
		query = query.Where(
			"(test_case_id IN (SELECT id FROM test_cases WHERE environment_id = ?)) OR " +
			"(test_suite_id IN (SELECT id FROM test_suites WHERE environment_id = ?))",
			environmentID, environmentID,
		)
	}
	if testCaseID != "" {
		// Filter by specific test case
		query = query.Where("test_case_id = ?", testCaseID)
	}
	if executionType != "" {
		query = query.Where("execution_type = ?", executionType)
	}
	if parentExecutionID != "" {
		// Filter by parent execution ID
		query = query.Where("parent_execution_id = ?", parentExecutionID)
	}
	if startDate != "" && endDate != "" {
		query = query.Where("DATE(start_time) BETWEEN ? AND ?", startDate, endDate)
	}

	// Count total
	query.Count(&total)

	// Get paginated executions with relations
	offset := (page - 1) * pageSize
	err := query.Preload("TestCase").Preload("TestCase.Environment").Preload("TestCase.Project").
		Preload("TestSuite").Preload("TestSuite.Environment").Preload("TestSuite.Project").
		Preload("TestSuite.TestCases").Preload("TestSuite.TestCases.Environment").
		Preload("User").
		Order("created_at DESC").
		Offset(offset).Limit(pageSize).Find(&executions).Error
	if err != nil {
		response.InternalServerError(c, "获取执行记录失败")
		return
	}

	// Clear user passwords and calculate environment info for test suites
	for i := range executions {
		executions[i].User.Password = ""
		// Calculate environment info for test suite executions
		if executions[i].ExecutionType == "test_suite" && executions[i].TestSuite.ID != 0 {
			executions[i].TestSuite.EnvironmentInfo = executions[i].TestSuite.GetEnvironmentInfo()
			
			// Dynamically calculate test suite execution statistics
			// Use raw SQL to get the latest status for each unique test case
			type TestCaseStatus struct {
				TestCaseID uint
				Status     string
			}
			
			var latestStatuses []TestCaseStatus
			err := database.DB.Raw(`
				SELECT DISTINCT t1.test_case_id, t1.status
				FROM test_executions t1
				WHERE t1.parent_execution_id = ? 
				AND t1.test_case_id IS NOT NULL
				AND t1.created_at = (
					SELECT MAX(t2.created_at) 
					FROM test_executions t2 
					WHERE t2.parent_execution_id = ? 
					AND t2.test_case_id = t1.test_case_id
				)
			`, executions[i].ID, executions[i].ID).Scan(&latestStatuses).Error
			
			if err == nil && len(latestStatuses) > 0 {
				// Calculate statistics based on unique test cases with their latest status
				var realPassedCount, realFailedCount, realTotalCount int
				var realStatus string = "passed" // default
				
				for _, testCaseStatus := range latestStatuses {
					realTotalCount++
					switch testCaseStatus.Status {
					case "passed":
						realPassedCount++
					case "failed":
						realFailedCount++
						realStatus = "failed" // if any test case failed, parent should be failed
					case "running":
						realStatus = "running" // if any test case running, parent should be running
					case "pending":
						if realStatus != "failed" && realStatus != "running" {
							realStatus = "partial" // some are done, some are pending
						}
					}
				}
				
				// Update the execution record with real statistics
				executions[i].TotalCount = realTotalCount
				executions[i].PassedCount = realPassedCount
				executions[i].FailedCount = realFailedCount
				
				// Update status based on test case results
				if realFailedCount > 0 {
					executions[i].Status = "failed"
				} else if realPassedCount == realTotalCount {
					executions[i].Status = "passed"
				} else if realStatus == "running" {
					executions[i].Status = "running"
				} else {
					executions[i].Status = "partial"
				}
			}
		}
	}

	response.Page(c, executions, total, page, pageSize)
}

func GetExecutionStatistics(c *gin.Context) {
	// 获取查询参数
	name := c.Query("name")
	projectID := c.Query("project_id")
	environmentID := c.Query("environment_id")
	status := c.Query("status")
	executionType := c.Query("execution_type")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	// 构建基础查询
	query := database.DB.Model(&models.TestExecution{}).Where("execution_type != ?", "test_case_internal")

	// 判断是否需要JOIN表
	needJoin := name != "" || projectID != "" || environmentID != ""
	if needJoin {
		query = query.Joins("LEFT JOIN test_cases ON test_executions.test_case_id = test_cases.id").
			Joins("LEFT JOIN test_suites ON test_executions.test_suite_id = test_suites.id")
	}

	// 应用过滤条件
	if name != "" {
		query = query.Where("test_cases.name LIKE ? OR test_suites.name LIKE ?", "%"+name+"%", "%"+name+"%")
	}
	if projectID != "" {
		query = query.Where("test_cases.project_id = ? OR test_suites.project_id = ?", projectID, projectID)
	}
	if environmentID != "" {
		query = query.Where("test_cases.environment_id = ? OR test_suites.environment_id = ?", environmentID, environmentID)
	}
	if status != "" {
		query = query.Where("test_executions.status = ?", status)
	}
	if executionType != "" {
		query = query.Where("test_executions.execution_type = ?", executionType)
	}
	if startDate != "" && endDate != "" {
		query = query.Where("test_executions.created_at BETWEEN ? AND ?", startDate+" 00:00:00", endDate+" 23:59:59")
	}

	// 获取总执行次数
	var totalExecutions int64
	query.Count(&totalExecutions)

	// 获取各状态的执行次数
	var passedCount, failedCount, runningCount, pendingCount int64

	baseQuery := database.DB.Model(&models.TestExecution{}).Where("execution_type != ?", "test_case_internal")
	if needJoin {
		baseQuery = baseQuery.Joins("LEFT JOIN test_cases ON test_executions.test_case_id = test_cases.id").
			Joins("LEFT JOIN test_suites ON test_executions.test_suite_id = test_suites.id")
	}
	if name != "" {
		baseQuery = baseQuery.Where("test_cases.name LIKE ? OR test_suites.name LIKE ?", "%"+name+"%", "%"+name+"%")
	}
	if projectID != "" {
		baseQuery = baseQuery.Where("test_cases.project_id = ? OR test_suites.project_id = ?", projectID, projectID)
	}
	if environmentID != "" {
		baseQuery = baseQuery.Where("test_cases.environment_id = ? OR test_suites.environment_id = ?", environmentID, environmentID)
	}
	if executionType != "" {
		baseQuery = baseQuery.Where("test_executions.execution_type = ?", executionType)
	}
	if startDate != "" && endDate != "" {
		baseQuery = baseQuery.Where("test_executions.created_at BETWEEN ? AND ?", startDate+" 00:00:00", endDate+" 23:59:59")
	}

	baseQuery.Where("test_executions.status = ?", "passed").Count(&passedCount)

	baseQuery2 := database.DB.Model(&models.TestExecution{}).Where("execution_type != ?", "test_case_internal")
	if projectID != "" {
		baseQuery2 = baseQuery2.Joins("LEFT JOIN test_cases ON test_executions.test_case_id = test_cases.id").
			Joins("LEFT JOIN test_suites ON test_executions.test_suite_id = test_suites.id").
			Where("test_cases.project_id = ? OR test_suites.project_id = ?", projectID, projectID)
	}
	if environmentID != "" {
		baseQuery2 = baseQuery2.Joins("LEFT JOIN test_cases tc2 ON test_executions.test_case_id = tc2.id").
			Joins("LEFT JOIN test_suites ts2 ON test_executions.test_suite_id = ts2.id").
			Where("tc2.environment_id = ? OR ts2.environment_id = ?", environmentID, environmentID)
	}
	if startDate != "" && endDate != "" {
		baseQuery2 = baseQuery2.Where("test_executions.created_at BETWEEN ? AND ?", startDate+" 00:00:00", endDate+" 23:59:59")
	}
	baseQuery2.Where("test_executions.status = ?", "failed").Count(&failedCount)

	baseQuery3 := database.DB.Model(&models.TestExecution{}).Where("execution_type != ?", "test_case_internal")
	if projectID != "" {
		baseQuery3 = baseQuery3.Joins("LEFT JOIN test_cases ON test_executions.test_case_id = test_cases.id").
			Joins("LEFT JOIN test_suites ON test_executions.test_suite_id = test_suites.id").
			Where("test_cases.project_id = ? OR test_suites.project_id = ?", projectID, projectID)
	}
	if environmentID != "" {
		baseQuery3 = baseQuery3.Joins("LEFT JOIN test_cases tc2 ON test_executions.test_case_id = tc2.id").
			Joins("LEFT JOIN test_suites ts2 ON test_executions.test_suite_id = ts2.id").
			Where("tc2.environment_id = ? OR ts2.environment_id = ?", environmentID, environmentID)
	}
	if startDate != "" && endDate != "" {
		baseQuery3 = baseQuery3.Where("test_executions.created_at BETWEEN ? AND ?", startDate+" 00:00:00", endDate+" 23:59:59")
	}
	baseQuery3.Where("test_executions.status = ?", "running").Count(&runningCount)

	baseQuery4 := database.DB.Model(&models.TestExecution{}).Where("execution_type != ?", "test_case_internal")
	if projectID != "" {
		baseQuery4 = baseQuery4.Joins("LEFT JOIN test_cases ON test_executions.test_case_id = test_cases.id").
			Joins("LEFT JOIN test_suites ON test_executions.test_suite_id = test_suites.id").
			Where("test_cases.project_id = ? OR test_suites.project_id = ?", projectID, projectID)
	}
	if environmentID != "" {
		baseQuery4 = baseQuery4.Joins("LEFT JOIN test_cases tc2 ON test_executions.test_case_id = tc2.id").
			Joins("LEFT JOIN test_suites ts2 ON test_executions.test_suite_id = ts2.id").
			Where("tc2.environment_id = ? OR ts2.environment_id = ?", environmentID, environmentID)
	}
	if startDate != "" && endDate != "" {
		baseQuery4 = baseQuery4.Where("test_executions.created_at BETWEEN ? AND ?", startDate+" 00:00:00", endDate+" 23:59:59")
	}
	baseQuery4.Where("test_executions.status = ?", "pending").Count(&pendingCount)

	// 计算成功率
	var successRate float64
	if totalExecutions > 0 {
		successRate = float64(passedCount) / float64(totalExecutions) * 100
	}

	// 计算平均执行时长
	var avgDuration float64
	database.DB.Model(&models.TestExecution{}).
		Where("execution_type != ? AND duration > 0", "test_case_internal").
		Select("AVG(duration) as avg_duration").
		Pluck("avg_duration", &avgDuration)

	response.Success(c, gin.H{
		"total_executions": totalExecutions,
		"passed_count":     passedCount,
		"failed_count":     failedCount,
		"running_count":    runningCount,
		"pending_count":    pendingCount,
		"success_rate":     successRate,
		"avg_duration":     avgDuration,
	})
}

func GetExecution(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的执行记录ID")
		return
	}

	var execution models.TestExecution
	err = database.DB.Preload("TestCase").Preload("TestCase.Project").
		Preload("TestCase.Environment").Preload("TestCase.Device").
		Preload("TestSuite").Preload("TestSuite.Environment").Preload("TestSuite.Project").
		Preload("TestSuite.TestCases").Preload("TestSuite.TestCases.Environment").
		Preload("User").
		First(&execution, id).Error
	if err != nil {
		response.NotFound(c, "执行记录不存在")
		return
	}

	execution.User.Password = ""
	// Calculate environment info for test suite executions
	if execution.ExecutionType == "test_suite" && execution.TestSuite.ID != 0 {
		execution.TestSuite.EnvironmentInfo = execution.TestSuite.GetEnvironmentInfo()
	}
	response.Success(c, execution)
}

func DeleteExecution(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的执行记录ID")
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var execution models.TestExecution
	err = database.DB.Where("id = ? AND user_id = ?", id, userID).First(&execution).Error
	if err != nil {
		response.NotFound(c, "执行记录不存在或无权限")
		return
	}

	// Don't allow deleting running executions
	if execution.Status == "running" || execution.Status == "pending" {
		response.BadRequest(c, "不能删除正在运行的执行记录")
		return
	}

	// Delete related performance metrics first
	database.DB.Where("execution_id = ?", id).Delete(&models.PerformanceMetric{})

	// Delete related screenshots
	database.DB.Where("execution_id = ?", id).Delete(&models.Screenshot{})

	// Delete execution record
	err = database.DB.Delete(&execution).Error
	if err != nil {
		response.InternalServerError(c, "删除执行记录失败")
		return
	}

	response.SuccessWithMessage(c, "删除成功", nil)
}

func GetExecutionLogs(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的执行记录ID")
		return
	}

	var execution models.TestExecution
	err = database.DB.Select("execution_logs").First(&execution, id).Error
	if err != nil {
		response.NotFound(c, "执行记录不存在")
		return
	}

	// Parse logs JSON
	var logs []map[string]interface{}
	if execution.ExecutionLogs != "" && execution.ExecutionLogs != "[]" {
		err = json.Unmarshal([]byte(execution.ExecutionLogs), &logs)
		if err != nil {
			response.InternalServerError(c, "解析执行日志失败")
			return
		}
	}

	response.Success(c, gin.H{
		"logs": logs,
	})
}

func GetExecutionScreenshots(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的执行记录ID")
		return
	}

	// Get screenshots from database
	var screenshots []models.Screenshot
	err = database.DB.Where("execution_id = ?", id).Order("step_index ASC").Find(&screenshots).Error
	if err != nil {
		response.InternalServerError(c, "获取截图记录失败")
		return
	}

	// Also get screenshots from execution record
	var execution models.TestExecution
	err = database.DB.Select("screenshots").First(&execution, id).Error
	if err != nil {
		response.NotFound(c, "执行记录不存在")
		return
	}

	// Parse screenshots JSON from execution record
	var executionScreenshots []string
	if execution.Screenshots != "" && execution.Screenshots != "[]" {
		err = json.Unmarshal([]byte(execution.Screenshots), &executionScreenshots)
		if err != nil {
			response.InternalServerError(c, "解析截图数据失败")
			return
		}
	}

	response.Success(c, gin.H{
		"screenshots":           screenshots,
		"execution_screenshots": executionScreenshots,
		"debug_info": gin.H{
			"screenshot_count":           len(screenshots),
			"execution_screenshot_count": len(executionScreenshots),
		},
	})
}

func GetExecutionStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的执行记录ID")
		return
	}

	var execution models.TestExecution
	err = database.DB.Select("id, status, start_time, end_time").First(&execution, id).Error
	if err != nil {
		response.NotFound(c, "执行记录不存在")
		return
	}

	// Check executor status
	executorRunning := false
	if executor.GlobalExecutor != nil {
		executorRunning = executor.GlobalExecutor.IsRunning(execution.ID)
	}

	response.Success(c, gin.H{
		"id":               execution.ID,
		"database_status":  execution.Status,
		"executor_running": executorRunning,
		"start_time":       execution.StartTime,
		"end_time":         execution.EndTime,
		"consistent":       (execution.Status == "running") == executorRunning,
	})
}

func StopExecution(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的执行记录ID")
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var execution models.TestExecution
	err = database.DB.Where("id = ? AND user_id = ?", id, userID).First(&execution).Error
	if err != nil {
		response.NotFound(c, "执行记录不存在或无权限")
		return
	}

	// Only allow stopping running or pending executions
	if execution.Status != "running" && execution.Status != "pending" {
		response.BadRequest(c, "只能停止运行中或等待中的执行记录")
		return
	}

	// Cancel the actual execution and close browser
	if executor.GlobalExecutor != nil {
		if execution.Status == "running" {
			executor.GlobalExecutor.CancelExecution(execution.ID)
		}
	}

	// Update execution status to cancelled
	err = database.DB.Model(&execution).Updates(models.TestExecution{
		Status: "cancelled",
	}).Error
	if err != nil {
		response.InternalServerError(c, "停止执行失败")
		return
	}

	response.SuccessWithMessage(c, "停止执行成功", nil)
}

func GetCurrentBatchExecutions(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的执行ID")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "1000")) // 默认获取更多记录

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 1000 // 设置更大的默认值
	}
	// 对于测试套件执行详情，允许更大的页面大小以显示所有测试用例
	if pageSize > 1000 {
		pageSize = 1000
	}

	// 获取当前执行记录
	var currentExecution models.TestExecution
	err = database.DB.First(&currentExecution, id).Error
	if err != nil {
		response.NotFound(c, "执行记录不存在")
		return
	}

	// 确保这是一个测试套件执行记录
	if currentExecution.ExecutionType != "test_suite" || currentExecution.TestSuiteID == nil {
		response.BadRequest(c, "该记录不是测试套件执行记录")
		return
	}

	var executions []models.TestExecution
	var total int64

	// 查找当前套件执行的内部用例执行记录：优先使用parent_execution_id精确匹配
	query := database.DB.Model(&models.TestExecution{}).
		Where("parent_execution_id = ? AND execution_type = ?",
			currentExecution.ID, "test_case_internal")

	// Count total for new approach
	query.Count(&total)

	// 如果没有找到记录（历史数据），回退到时间范围查询
	if total == 0 {
		startTimeFrom := currentExecution.StartTime.Add(-1 * 60 * 1000000000) // 1分钟前
		startTimeTo := currentExecution.StartTime.Add(1 * 60 * 1000000000)    // 1分钟后

		query = database.DB.Model(&models.TestExecution{}).
			Where("test_suite_id = ? AND execution_type = ? AND start_time BETWEEN ? AND ?",
				currentExecution.TestSuiteID, "test_case_internal", startTimeFrom, startTimeTo)

		// Recount for fallback approach
		query.Count(&total)
	}

	// Get paginated executions with relations
	offset := (page - 1) * pageSize
	err = query.Preload("TestCase").Preload("TestCase.Project").Preload("TestCase.Environment").Preload("TestCase.Device").Preload("User").
		Order("created_at ASC").Offset(offset).Limit(pageSize).Find(&executions).Error
	if err != nil {
		response.InternalServerError(c, "获取批次执行记录失败")
		return
	}

	// Clear user passwords
	for i := range executions {
		executions[i].User.Password = ""
	}

	response.Page(c, executions, total, page, pageSize)
}

// DownloadExecutionReportHTML generates and downloads an HTML report for a test execution
func DownloadExecutionReportHTML(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的执行记录ID")
		return
	}

	// Get execution with all relations
	var execution models.TestExecution
	err = database.DB.
		Preload("TestCase").
		Preload("TestCase.Project").
		Preload("TestCase.Environment").
		Preload("TestCase.Device").
		Preload("TestSuite").
		Preload("TestSuite.Project").
		Preload("TestSuite.Environment").
		Preload("User").
		First(&execution, id).Error
	
	if err != nil {
		response.NotFound(c, "执行记录不存在")
		return
	}

	// Generate HTML report
	htmlContent := generateHTMLReport(&execution)
	
	// Get test name for filename
	testName := ""
	if execution.TestCaseID != nil {
		testName = execution.TestCase.Name
	} else if execution.TestSuiteID != nil {
		testName = execution.TestSuite.Name
	}
	
	// Generate filename with timestamp - sanitize name for filename safety
	timestamp := time.Now().Format("20060102-150405")
	safeTestName := strings.ReplaceAll(testName, " ", "_")
	safeTestName = strings.ReplaceAll(safeTestName, "/", "_")
	safeTestName = strings.ReplaceAll(safeTestName, "\\", "_")
	filename := fmt.Sprintf("TestReport-%s-%s.html", safeTestName, timestamp)
	
	// Set response headers for HTML download
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Header("Content-Length", fmt.Sprintf("%d", len(htmlContent)))
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	
	// Write HTML to response
	c.Data(200, "text/html; charset=utf-8", []byte(htmlContent))
}

// DownloadExecutionReportPDF generates and downloads a PDF report for a test execution
func DownloadExecutionReportPDF(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的执行记录ID")
		return
	}

	// Get execution with all relations
	var execution models.TestExecution
	err = database.DB.
		Preload("TestCase").
		Preload("TestCase.Project").
		Preload("TestCase.Environment").
		Preload("TestCase.Device").
		Preload("TestSuite").
		Preload("TestSuite.Project").
		Preload("TestSuite.Environment").
		Preload("User").
		First(&execution, id).Error
	
	if err != nil {
		response.NotFound(c, "执行记录不存在")
		return
	}

	// Generate HTML content for PDF conversion
	htmlContent := generateHTMLReport(&execution)
	
	// Generate PDF from HTML using Chrome
	pdfData, err := generatePDFFromHTML(htmlContent)
	if err != nil {
		response.InternalServerError(c, "生成PDF报告失败: "+err.Error())
		return
	}
	
	// Get test name for filename
	testName := ""
	if execution.TestCaseID != nil {
		testName = execution.TestCase.Name
	} else if execution.TestSuiteID != nil {
		testName = execution.TestSuite.Name
	}
	
	// Generate filename with timestamp - sanitize name for filename safety
	timestamp := time.Now().Format("20060102-150405")
	safeTestName := strings.ReplaceAll(testName, " ", "_")
	safeTestName = strings.ReplaceAll(safeTestName, "/", "_")
	safeTestName = strings.ReplaceAll(safeTestName, "\\", "_")
	filename := fmt.Sprintf("TestReport-%s-%s.pdf", safeTestName, timestamp)
	
	// Set response headers for PDF download
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Header("Content-Length", fmt.Sprintf("%d", len(pdfData)))
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	
	// Write PDF to response
	c.Data(200, "application/pdf", pdfData)
}

// generatePDFFromHTML converts HTML content to PDF using Chrome headless mode (optimized for completeness)
func generatePDFFromHTML(htmlContent string) ([]byte, error) {
	// Create context with reasonable timeout - 120 seconds for reports with many screenshots
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Get Chrome path using existing logic
	chromePath := chrome.GetChromePath()
	if chromePath == "" {
		chromePath = chrome.GetFlatpakChromePath()
		if chromePath == "" {
			return nil, fmt.Errorf("Chrome executable not found")
		}
	}

	// Launch Chrome with optimized headless options
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(chromePath),
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-web-security", true),
		chromedp.Flag("disable-features", "VizDisplayCompositor"),
		chromedp.Flag("run-all-compositor-stages-before-draw", true),
		chromedp.Flag("disable-background-timer-throttling", true),
		chromedp.Flag("disable-renderer-backgrounding", true),
		chromedp.Flag("disable-backgrounding-occluded-windows", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-plugins", true),
		chromedp.Flag("disable-images", false), // Keep images enabled but optimize loading
		chromedp.Flag("aggressive-cache-discard", true), // Save memory
		chromedp.WindowSize(1920, 1080), // Set viewport size
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)
	defer allocCancel()

	// Create Chrome context
	chromeCtx, chromeCancel := chromedp.NewContext(allocCtx)
	defer chromeCancel()

	// Create data URL from HTML using base64 encoding to avoid encoding issues
	htmlContentEncoded := base64.StdEncoding.EncodeToString([]byte(htmlContent))
	dataURL := "data:text/html;charset=utf-8;base64," + htmlContentEncoded

	var pdfData []byte
	
	// Optimized PDF generation with reduced wait times
	err := chromedp.Run(chromeCtx,
		chromedp.Navigate(dataURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		// Smart image loading check - quickly identify and handle broken images
		chromedp.ActionFunc(func(ctx context.Context) error {
			// First, quickly mark all images with onerror handlers to fail fast
			chromedp.Evaluate(`
				(function() {
					const images = document.querySelectorAll('img');
					images.forEach(img => {
						// If image already has onerror that hides it, trigger it for non-loading images
						if (!img.complete && img.onerror) {
							// Set a short timeout to trigger error handler for stuck images
							setTimeout(() => {
								if (!img.complete || img.naturalWidth === 0) {
									img.style.display = 'none';
								}
							}, 2000);
						}
					});
				})()
			`, nil).Do(ctx)
			
			// Wait briefly for error handlers to trigger
			time.Sleep(2 * time.Second)
			
			// Now count only visible images
			var visibleCount int
			chromedp.Evaluate(`
				Array.from(document.querySelectorAll('img')).filter(img => 
					img.style.display !== 'none' && img.offsetParent !== null
				).length
			`, &visibleCount).Do(ctx)
			
			if visibleCount > 0 {
				log.Printf("PDF generation: Waiting for %d visible images to load...", visibleCount)
				
				// Initial wait for images to start loading
				time.Sleep(3 * time.Second)
				
				// More attempts for reports with many images
				maxAttempts := 20
				if visibleCount > 100 {
					maxAttempts = 30 // Even more attempts for large reports
				}
				
				lastLoadedCount := 0
				noProgressCount := 0
				pendingCount := 0
				
				// Check for image loading with progressive retries
				for attempt := 0; attempt < maxAttempts; attempt++ {
					var loadStatus map[string]interface{}
					err := chromedp.Evaluate(`
						(function() {
							const images = Array.from(document.querySelectorAll('img')).filter(img => 
								img.style.display !== 'none' && img.offsetParent !== null
							);
							if (images.length === 0) return {loaded: 0, total: 0, complete: true, realTotal: 0};
							
							let loaded = 0;
							let failed = 0;
							let pending = 0;
							let total = images.length;
							
							for (let img of images) {
								if (img.complete) {
									if (img.naturalWidth > 0) {
										loaded++;
									} else if (!img.src || img.src === '' || img.src === window.location.href) {
										// No valid source, mark as failed
										img.style.display = 'none';
										failed++;
									} else {
										// Has source but failed to load
										failed++;
									}
								} else {
									// Still loading
									pending++;
								}
							}
							
							// Real total is the count of images that actually have a chance to load
							const realTotal = loaded + pending;
							
							// Consider complete if all pending images are done loading
							const complete = pending === 0;
							const percentLoaded = total > 0 ? (loaded / total) : 1;
							
							return {
								loaded: loaded, 
								failed: failed,
								pending: pending,
								total: total,
								realTotal: realTotal,
								complete: complete,
								percentLoaded: Math.round(percentLoaded * 100)
							};
						})()
					`, &loadStatus).Do(ctx)
					
					if err == nil {
						loaded := int(loadStatus["loaded"].(float64))
						failed := int(loadStatus["failed"].(float64))
						pending := int(loadStatus["pending"].(float64))
						total := int(loadStatus["total"].(float64))
						realTotal := int(loadStatus["realTotal"].(float64))
						complete := loadStatus["complete"].(bool)
						percentLoaded := int(loadStatus["percentLoaded"].(float64))
						
						// Update pending count for wait time calculation
						pendingCount = pending
						
						log.Printf("PDF generation: Image loading progress - %d/%d loaded (%d%%), %d pending, %d failed (attempt %d/%d)", 
							loaded, total, percentLoaded, pending, failed, attempt+1, maxAttempts)
						
						// Track progress
						if loaded > lastLoadedCount {
							lastLoadedCount = loaded
							noProgressCount = 0
						} else {
							noProgressCount++
						}
						
						// If no progress for 5 attempts and we have some images, proceed
						if noProgressCount >= 5 && loaded > 0 {
							log.Printf("PDF generation: No more progress after 5 attempts, proceeding with %d images", loaded)
							break
						}
						
						// Accept if all pending images are done
						if complete || pending == 0 {
							log.Printf("PDF generation: All images processed - %d loaded, %d failed", loaded, failed)
							break
						}
						
						// For large reports, accept if we have most of the real images
						if realTotal > 0 && loaded >= realTotal {
							log.Printf("PDF generation: All valid images loaded (%d/%d)", loaded, realTotal)
							break
						}
					}
					
					// Progressive wait - longer waits for more pending images
					waitTime := 2
					if pendingCount > 50 {
						waitTime = 3
					}
					if pendingCount > 100 {
						waitTime = 4
					}
					time.Sleep(time.Duration(waitTime) * time.Second)
				}
				
				// Final wait for rendering
				time.Sleep(2 * time.Second)
			} else {
				log.Printf("PDF generation: No visible images to wait for")
			}
			return nil
		}),
		// Additional wait for final rendering and layout stabilization
		chromedp.Sleep(3*time.Second),
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Configure PDF printing options - no headers/footers
			printParams := page.PrintToPDF().
				WithPrintBackground(true).      // Include background colors and images
				WithMarginTop(0.5).            // Top margin
				WithMarginBottom(0.5).         // Bottom margin  
				WithMarginLeft(0.5).           // Left margin
				WithMarginRight(0.5).          // Right margin
				WithPaperWidth(8.27).          // A4 width in inches
				WithPaperHeight(11.69).        // A4 height in inches
				WithDisplayHeaderFooter(false).// No headers/footers
				WithPreferCSSPageSize(false).  // Use our paper size
				WithScale(1.0)                 // Full scale

			var err error
			pdfData, _, err = printParams.Do(ctx)
			return err
		}),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to generate PDF: %w", err)
	}

	if len(pdfData) == 0 {
		return nil, fmt.Errorf("generated PDF is empty")
	}

	return pdfData, nil
}

// getServerBaseURL returns the base URL for the server using actual IP
func getServerBaseURL() string {
	// Try to get the server's actual IP address
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		// Fallback to localhost if can't determine IP
		return "http://localhost:8080"
	}
	defer conn.Close()
	
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	ip := localAddr.IP.String()
	
	return fmt.Sprintf("http://%s:8080", ip)
}

// generateHTMLReport generates an HTML report for a test execution
func generateHTMLReport(execution *models.TestExecution) string {
	// Get server base URL
	baseURL := getServerBaseURL()
	
	// Get basic info
	testName := ""
	projectName := ""
	environmentName := ""
	deviceName := ""
	
	if execution.TestCaseID != nil {
		testName = execution.TestCase.Name
		projectName = execution.TestCase.Project.Name
		environmentName = execution.TestCase.Environment.Name
		deviceName = fmt.Sprintf("%s (%dx%d)",
			execution.TestCase.Device.Name,
			execution.TestCase.Device.Width,
			execution.TestCase.Device.Height)
	} else if execution.TestSuiteID != nil {
		testName = execution.TestSuite.Name
		projectName = execution.TestSuite.Project.Name
		environmentName = execution.TestSuite.Environment.Name
		deviceName = "默认设备"
	}

	// Calculate duration
	duration := ""
	if execution.EndTime != nil && !execution.EndTime.IsZero() {
		duration = execution.EndTime.Sub(execution.StartTime).String()
	}

	// Get tester name
	testerName := "系统"
	if execution.UserID != 0 {
		testerName = execution.User.Username
	}

	// Calculate pass rate
	passRate := float64(0)
	if execution.TotalCount > 0 {
		passRate = float64(execution.PassedCount) / float64(execution.TotalCount) * 100
	}

	// Parse execution details based on type
	var steps string
	var screenshotPaths []string
	
	var showSeparateScreenshots bool = false // 不显示单独的截图部分，因为截图已包含在详细测试结果中
	
	if execution.TestCaseID != nil {
		// Single test case execution
		steps = parseExecutionLogs(execution.ExecutionLogs)
		// Parse screenshots from execution record
		screenshotPaths = parseScreenshotPaths(execution.Screenshots)
	} else if execution.TestSuiteID != nil {
		// Test suite execution - get detailed test case results
		steps, screenshotPaths = parseTestSuiteExecutionNew(execution, baseURL)
		showSeparateScreenshots = false // 截图已包含在详细测试结果中，不需要单独显示
	} else {
		steps = parseExecutionLogs(execution.ExecutionLogs)
		screenshotPaths = parseScreenshotPaths(execution.Screenshots)
	}

	// Generate HTML
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>测试执行报告 - %s</title>
    <style>
        body {
            font-family: "Microsoft YaHei", "SimHei", Arial, sans-serif;
            font-size: 14px;
            line-height: 1.6;
            color: #333;
            margin: 20px;
            background-color: #f9f9f9;
        }
        .header {
            text-align: center;
            margin-bottom: 30px;
            background: white;
            padding: 20px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .title {
            font-size: 28px;
            font-weight: bold;
            margin-bottom: 20px;
            color: #2c5aa0;
        }
        .subtitle {
            font-size: 16px;
            margin: 8px 0;
            color: #666;
        }
        .section {
            margin: 30px 0;
            background: white;
            padding: 20px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .section-title {
            font-size: 20px;
            font-weight: bold;
            border-bottom: 3px solid #4CAF50;
            padding-bottom: 8px;
            margin-bottom: 20px;
            color: #2c5aa0;
        }
        .info-table {
            width: 100%%;
            border-collapse: collapse;
            margin-top: 15px;
        }
        .info-table th, .info-table td {
            padding: 12px 15px;
            text-align: left;
            border: 1px solid #ddd;
        }
        .info-table th {
            background-color: #f2f2f2;
            font-weight: bold;
            width: 25%%;
            color: #2c5aa0;
        }
        .info-table tr:nth-child(even) {
            background-color: #f9f9f9;
        }
        .summary-stats {
            display: flex;
            justify-content: space-around;
            margin: 25px 0;
            gap: 20px;
        }
        .stat-box {
            text-align: center;
            padding: 20px;
            border: 1px solid #ddd;
            border-radius: 8px;
            min-width: 140px;
            background: linear-gradient(135deg, #f5f7fa 0%%, #c3cfe2 100%%);
            box-shadow: 0 4px 6px rgba(0,0,0,0.1);
        }
        .stat-number {
            font-size: 32px;
            font-weight: bold;
            color: #4CAF50;
            margin-bottom: 8px;
        }
        .stat-label {
            font-size: 14px;
            color: #666;
            font-weight: 500;
        }
        .passed { color: #4CAF50; }
        .failed { color: #f44336; }
        .step {
            margin: 15px 0;
            padding: 15px;
            border-left: 4px solid #4CAF50;
            background-color: #f9f9f9;
            border-radius: 0 8px 8px 0;
        }
        .step.failed {
            border-left-color: #f44336;
            background-color: #fdf2f2;
        }
        .step-title {
            font-weight: bold;
            margin-bottom: 8px;
        }
        .step-error {
            color: #f44336;
            font-style: italic;
            margin-top: 8px;
        }
        .step-duration {
            color: #666;
            font-size: 12px;
            margin-top: 5px;
        }
        .footer {
            text-align: center;
            margin-top: 40px;
            padding: 20px;
            background: white;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            color: #666;
        }
        .screenshots-grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
            gap: 20px;
            margin: 20px 0;
        }
        .screenshot-item {
            text-align: center;
            border: 1px solid #ddd;
            border-radius: 8px;
            padding: 15px;
            background: white;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .screenshot-item img {
            max-width: 300px;
            width: 100%;
            height: auto;
            border-radius: 4px;
            box-shadow: 0 2px 8px rgba(0,0,0,0.2);
            cursor: pointer;
            transition: transform 0.2s;
        }
        .screenshot-item img:hover {
            transform: scale(1.02);
        }
        .screenshot-title {
            font-weight: bold;
            margin-bottom: 8px;
            color: #2c5aa0;
        }
        .screenshot-description {
            font-size: 12px;
            color: #666;
            margin-top: 8px;
        }
        .testcase-item {
            margin: 20px 0;
            padding: 20px;
            border: 1px solid #ddd;
            border-radius: 8px;
            background: #f9f9f9;
        }
        .testcase-item.passed {
            border-left: 5px solid #4CAF50;
        }
        .testcase-item.failed {
            border-left: 5px solid #f44336;
        }
        .testcase-title {
            font-size: 18px;
            font-weight: bold;
            margin-bottom: 10px;
        }
        .testcase-info {
            display: flex;
            justify-content: space-between;
            margin: 10px 0;
            font-size: 14px;
        }
        .testcase-status {
            font-weight: bold;
        }
        .testcase-status.passed {
            color: #4CAF50;
        }
        .testcase-status.failed {
            color: #f44336;
        }
        .testcase-screenshots {
            margin-top: 15px;
            padding-top: 15px;
            border-top: 1px solid #eee;
        }
        .testcase-screenshots h4 {
            margin: 0 0 10px 0;
            color: #2c5aa0;
            font-size: 16px;
        }
        .testcase-screenshots .screenshots-grid {
            margin: 10px 0 0 0;
        }
        .testcase-screenshots .screenshot-item {
            background: #f8f9fa;
        }
    </style>
</head>
<body>
    <div class="header">
        <div class="title">测试执行报告</div>
        <div class="subtitle">测试名称: %s</div>
        <div class="subtitle">项目名称: %s</div>
        <div class="subtitle">生成时间: %s</div>
    </div>

    <div class="section">
        <div class="section-title">1. 基本信息</div>
        <table class="info-table">
            <tr><th>项目名称</th><td>%s</td></tr>
            <tr><th>测试环境</th><td>%s</td></tr>
            <tr><th>测试设备</th><td>%s</td></tr>
            <tr><th>执行时间</th><td>%s - %s</td></tr>
            <tr><th>持续时间</th><td>%s</td></tr>
            <tr><th>执行人员</th><td>%s</td></tr>
            <tr><th>执行模式</th><td>可视化</td></tr>
            <tr><th>执行状态</th><td><span class="%s">%s</span></td></tr>
        </table>
    </div>

    <div class="section">
        <div class="section-title">2. 执行汇总</div>
        <div class="summary-stats">
            <div class="stat-box">
                <div class="stat-number">%d</div>
                <div class="stat-label">总用例数</div>
            </div>
            <div class="stat-box">
                <div class="stat-number passed">%d</div>
                <div class="stat-label">通过</div>
            </div>
            <div class="stat-box">
                <div class="stat-number failed">%d</div>
                <div class="stat-label">失败</div>
            </div>
            <div class="stat-box">
                <div class="stat-number">%.1f%%</div>
                <div class="stat-label">通过率</div>
            </div>
        </div>
    </div>

    <div class="section">
        <div class="section-title">3. 详细测试结果</div>
        %s
    </div>

    %s

    <div class="section">
        <div class="section-title">4. 环境配置</div>
        <table class="info-table">
            <tr><th>浏览器</th><td>Google Chrome (可视化模式)</td></tr>
            <tr><th>操作系统</th><td>Linux</td></tr>
            <tr><th>平台</th><td>WebTestFlow 自动化测试平台</td></tr>
            <tr><th>环境名称</th><td>%s</td></tr>
            <tr><th>设备类型</th><td>%s</td></tr>
        </table>
    </div>

    <div class="footer">
        <p>本报告由 WebTestFlow 自动化测试平台生成 | 生成时间: %s</p>
    </div>
</body>
</html>`,
		testName, testName, projectName, time.Now().Format("2006-01-02 15:04:05"),
		projectName, environmentName, deviceName,
		execution.StartTime.Format("2006-01-02 15:04:05"),
		func() string {
			if execution.EndTime != nil && !execution.EndTime.IsZero() {
				return execution.EndTime.Format("2006-01-02 15:04:05")
			}
			return "未结束"
		}(),
		duration, testerName,
		func() string {
			if execution.Status == "passed" {
				return "passed"
			} else if execution.Status == "failed" {
				return "failed"
			}
			return ""
		}(),
		execution.Status,
		execution.TotalCount, execution.PassedCount, execution.FailedCount, passRate,
		steps, func() string {
			if showSeparateScreenshots {
				return generateScreenshotsHTMLFromPaths(screenshotPaths, baseURL)
			}
			return ""
		}(), environmentName, deviceName, time.Now().Format("2006-01-02 15:04:05"))

	return html
}

// parseExecutionLogs parses execution logs to extract test steps for HTML display
func parseExecutionLogs(logs string) string {
	if logs == "" {
		return `<div class="step">
            <div class="step-title">测试执行完成</div>
            <div class="step-duration">执行状态: 完成</div>
        </div>`
	}

	stepsHTML := ""
	lines := strings.Split(logs, "\n")
	stepIndex := 1

	for _, line := range lines {
		if strings.Contains(line, "Executing step") || strings.Contains(line, "Step") {
			status := "passed"
			if strings.Contains(line, "failed") || strings.Contains(line, "error") {
				status = "failed"
			}

			stepsHTML += fmt.Sprintf(`<div class="step %s">
                <div class="step-title">步骤 %d: %s</div>`, status, stepIndex, line)

			if status == "failed" {
				stepsHTML += `<div class="step-error">执行失败</div>`
			}

			stepsHTML += `</div>`
			stepIndex++
		}
	}

	// If no structured steps found, create a summary
	if stepsHTML == "" {
		status := "passed"
		if strings.Contains(logs, "failed") || strings.Contains(logs, "error") {
			status = "failed"
		}

		stepsHTML = fmt.Sprintf(`<div class="step %s">
            <div class="step-title">测试执行完成</div>
            <div class="step-duration">执行状态: %s</div>
        </div>`, status, func() string {
			if status == "failed" {
				return "执行失败，请查看日志了解详情"
			}
			return "执行成功"
		}())
	}

	return stepsHTML
}

// parseTestSuiteExecutionNew parses test suite execution to get detailed test case results with screenshots
func parseTestSuiteExecutionNew(execution *models.TestExecution, baseURL string) (string, []string) {
	// Get test case executions for this test suite execution
	var testCaseExecutions []models.TestExecution
	var allScreenshotPaths []string
	
	// First try to find by parent_execution_id (new approach)
	err := database.DB.
		Preload("TestCase").
		Preload("TestCase.Project").
		Preload("TestCase.Environment").
		Preload("TestCase.Device").
		Where("parent_execution_id = ? AND execution_type = ?", execution.ID, "test_case_internal").
		Order("created_at ASC").
		Find(&testCaseExecutions).Error
		
	if err != nil || len(testCaseExecutions) == 0 {
		// Fallback to time range query for historical data
		startTimeFrom := execution.StartTime.Add(-1 * time.Minute)
		startTimeTo := execution.StartTime.Add(1 * time.Minute)
		
		database.DB.
			Preload("TestCase").
			Preload("TestCase.Project").
			Preload("TestCase.Environment").
			Preload("TestCase.Device").
			Where("test_suite_id = ? AND execution_type = ? AND start_time BETWEEN ? AND ?",
				execution.TestSuiteID, "test_case_internal", startTimeFrom, startTimeTo).
			Order("created_at ASC").
			Find(&testCaseExecutions)
	}
	
	stepsHTML := ""
	
	if len(testCaseExecutions) == 0 {
		stepsHTML = `<div class="step">
            <div class="step-title">测试套件执行完成</div>
            <div class="step-duration">执行状态: 执行成功</div>
        </div>`
		// Use main execution screenshots
		allScreenshotPaths = parseScreenshotPaths(execution.Screenshots)
	} else {
		for i, testCaseExec := range testCaseExecutions {
			status := "passed"
			statusText := "通过"
			if testCaseExec.Status == "failed" {
				status = "failed" 
				statusText = "失败"
			}
			
			// Calculate duration for this test case
			duration := ""
			if testCaseExec.EndTime != nil && !testCaseExec.EndTime.IsZero() {
				duration = testCaseExec.EndTime.Sub(testCaseExec.StartTime).String()
			}
			
			// Get test case name and device info
			testCaseName := "未知测试用例"
			deviceInfo := ""
			if testCaseExec.TestCase.ID != 0 {
				testCaseName = testCaseExec.TestCase.Name
				if testCaseExec.TestCase.Device.ID != 0 {
					deviceInfo = fmt.Sprintf(" | 设备: %s (%dx%d)", 
						testCaseExec.TestCase.Device.Name,
						testCaseExec.TestCase.Device.Width,
						testCaseExec.TestCase.Device.Height)
				}
			}
			
			// Generate test case HTML with its screenshots
			stepsHTML += fmt.Sprintf(`<div class="testcase-item %s">
                <div class="testcase-title">测试用例 %d: %s</div>
                <div class="testcase-info">
                    <span>执行时间: %s - %s</span>
                    <span class="testcase-status %s">%s</span>
                </div>
                <div class="testcase-info">
                    <span>持续时间: %s%s</span>
                </div>`, 
				status, i+1, testCaseName,
				testCaseExec.StartTime.Format("15:04:05"),
				func() string {
					if testCaseExec.EndTime != nil && !testCaseExec.EndTime.IsZero() {
						return testCaseExec.EndTime.Format("15:04:05")
					}
					return "未结束"
				}(),
				status, statusText, duration, deviceInfo)
			
			// Parse screenshots from this test case execution and add to this test case
			screenshotPaths := parseScreenshotPaths(testCaseExec.Screenshots)
			if len(screenshotPaths) > 0 {
				stepsHTML += `<div class="testcase-screenshots">
                    <h4>执行截图:</h4>
                    <div class="screenshots-grid">`
				
				for j, path := range screenshotPaths {
					// Extract info from path
					filename := path
					if idx := strings.LastIndex(path, "/"); idx >= 0 {
						filename = path[idx+1:]
					}
					
					// Determine screenshot type from filename
					typeDesc := "执行截图"
					if strings.Contains(filename, "_initial_") {
						typeDesc = "初始截图"
					} else if strings.Contains(filename, "_step_") {
						typeDesc = "步骤截图"
					} else if strings.Contains(filename, "_final_") {
						typeDesc = "最终截图"
					} else if strings.Contains(filename, "_error_") {
						typeDesc = "错误截图"
					}
					
					// Extract step info from filename
					stepInfo := "未知步骤"
					if strings.Contains(filename, "_initial_") {
						stepInfo = "初始状态"
					} else if strings.Contains(filename, "_final_") {
						stepInfo = "最终状态"
					} else {
						// Try to extract step number
						if stepStart := strings.Index(filename, "_step_"); stepStart >= 0 {
							stepPart := filename[stepStart+6:]
							if stepEnd := strings.Index(stepPart, "_"); stepEnd >= 0 {
								stepInfo = "步骤 " + stepPart[:stepEnd]
							}
						}
					}
					
					// URL encode the path
					encodedPath := strings.ReplaceAll(url.QueryEscape(path), "%2F", "/")
					
					stepsHTML += fmt.Sprintf(`
                        <div class="screenshot-item">
                            <div class="screenshot-title">截图 %d</div>
                            <img src="%s/api/v1/screenshots/%s" alt="%s" onerror="this.style.display='none'"/>
                            <div class="screenshot-description">%s<br>%s</div>
                        </div>`,
						j+1, baseURL, encodedPath, typeDesc, typeDesc, stepInfo)
				}
				
				stepsHTML += `</div></div>`
			}
			
			stepsHTML += `</div>` // Close testcase-item
			
			// Collect all screenshots for the return value (for compatibility)
			allScreenshotPaths = append(allScreenshotPaths, screenshotPaths...)
		}
	}
	
	return stepsHTML, allScreenshotPaths
}

// generateScreenshotsHTML generates HTML for screenshots section
func generateScreenshotsHTML(screenshots []models.Screenshot, baseURL string) string {
	if len(screenshots) == 0 {
		return ""
	}
	
	screenshotsHTML := `<div class="section">
        <div class="section-title">5. 执行截图</div>
        <div class="screenshots-grid">`
	
	for i, screenshot := range screenshots {
		// Determine screenshot type and description
		typeDesc := "执行截图"
		switch screenshot.Type {
		case "error":
			typeDesc = "错误截图"
		case "before":
			typeDesc = "执行前截图"
		case "after":
			typeDesc = "执行后截图"
		}
		
		// URL encode the filename for proper handling of Chinese characters, but preserve slashes
		encodedFileName := strings.ReplaceAll(url.QueryEscape(screenshot.FileName), "%2F", "/")
		
		screenshotsHTML += fmt.Sprintf(`
            <div class="screenshot-item">
                <div class="screenshot-title">截图 %d</div>
                <img src="%s/api/v1/screenshots/%s" alt="%s" onerror="this.style.display='none'"/>
                <div class="screenshot-description">%s<br>步骤 %d | %s</div>
            </div>`,
			i+1, baseURL, encodedFileName, typeDesc, typeDesc, screenshot.StepIndex, 
			screenshot.CreatedAt.Format("2006-01-02 15:04:05"))
	}
	
	screenshotsHTML += `
        </div>
    </div>`
	
	return screenshotsHTML
}

// parseScreenshotPaths parses screenshots JSON string to extract file paths
func parseScreenshotPaths(screenshotsJSON string) []string {
	if screenshotsJSON == "" || screenshotsJSON == "[]" {
		return []string{}
	}
	
	var paths []string
	err := json.Unmarshal([]byte(screenshotsJSON), &paths)
	if err != nil {
		return []string{}
	}
	
	return paths
}

// generateScreenshotsHTMLFromPaths generates HTML for screenshots section from file paths
func generateScreenshotsHTMLFromPaths(screenshotPaths []string, baseURL string) string {
	if len(screenshotPaths) == 0 {
		return ""
	}
	
	screenshotsHTML := `<div class="section">
        <div class="section-title">5. 执行截图</div>
        <div class="screenshots-grid">`
	
	for i, path := range screenshotPaths {
		// Extract info from path - format: "2025-08-19/首页_正常登录_step_1_17:03:02.png"
		filename := path
		if idx := strings.LastIndex(path, "/"); idx >= 0 {
			filename = path[idx+1:]
		}
		
		// Determine screenshot type from filename
		typeDesc := "执行截图"
		if strings.Contains(filename, "_initial_") {
			typeDesc = "初始截图"
		} else if strings.Contains(filename, "_step_") {
			typeDesc = "步骤截图"
		} else if strings.Contains(filename, "_final_") {
			typeDesc = "最终截图"
		} else if strings.Contains(filename, "_error_") {
			typeDesc = "错误截图"
		}
		
		// Extract step info from filename
		stepInfo := "未知步骤"
		if strings.Contains(filename, "_initial_") {
			stepInfo = "初始状态"
		} else if strings.Contains(filename, "_final_") {
			stepInfo = "最终状态"
		} else {
			// Try to extract step number
			if stepStart := strings.Index(filename, "_step_"); stepStart >= 0 {
				stepPart := filename[stepStart+6:]
				if stepEnd := strings.Index(stepPart, "_"); stepEnd >= 0 {
					stepInfo = "步骤 " + stepPart[:stepEnd]
				}
			}
		}
		
		// URL encode the path for proper handling of Chinese characters, but preserve slashes
		encodedPath := strings.ReplaceAll(url.QueryEscape(path), "%2F", "/")
		
		screenshotsHTML += fmt.Sprintf(`
            <div class="screenshot-item">
                <div class="screenshot-title">截图 %d</div>
                <img src="%s/api/v1/screenshots/%s" alt="%s" onerror="this.style.display='none'"/>
                <div class="screenshot-description">%s<br>%s</div>
            </div>`,
			i+1, baseURL, encodedPath, typeDesc, typeDesc, stepInfo)
	}
	
	screenshotsHTML += `
        </div>
    </div>`
	
	return screenshotsHTML
}
