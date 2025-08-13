package handlers

import (
	"encoding/json"
	"strconv"
	"webtestflow/backend/internal/executor"
	"webtestflow/backend/internal/models"
	"webtestflow/backend/pkg/database"
	"webtestflow/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

func GetExecutions(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	status := c.Query("status")

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 10
	}

	var executions []models.TestExecution
	var total int64

	query := database.DB.Model(&models.TestExecution{}).Where("execution_type != ?", "test_case_internal")
	if status != "" {
		query = query.Where("status = ?", status)
	}

	// Count total
	query.Count(&total)

	// Get paginated executions with relations
	offset := (page - 1) * pageSize
	err := query.Preload("TestCase").Preload("TestCase.Environment").Preload("TestCase.Project").
		Preload("TestSuite").Preload("TestSuite.Environment").Preload("TestSuite.Project").
		Preload("User").
		Order("created_at DESC").
		Offset(offset).Limit(pageSize).Find(&executions).Error
	if err != nil {
		response.InternalServerError(c, "获取执行记录失败")
		return
	}

	// Clear user passwords
	for i := range executions {
		executions[i].User.Password = ""
	}

	response.Page(c, executions, total, page, pageSize)
}

func GetExecutionStatistics(c *gin.Context) {
	// 获取查询参数
	projectID := c.Query("project_id")
	environmentID := c.Query("environment_id")
	status := c.Query("status")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	// 构建基础查询
	query := database.DB.Model(&models.TestExecution{}).Where("execution_type != ?", "test_case_internal")

	// 应用过滤条件
	if projectID != "" {
		query = query.Joins("LEFT JOIN test_cases ON test_executions.test_case_id = test_cases.id").
			Joins("LEFT JOIN test_suites ON test_executions.test_suite_id = test_suites.id").
			Where("test_cases.project_id = ? OR test_suites.project_id = ?", projectID, projectID)
	}
	if environmentID != "" {
		query = query.Joins("LEFT JOIN test_cases tc2 ON test_executions.test_case_id = tc2.id").
			Joins("LEFT JOIN test_suites ts2 ON test_executions.test_suite_id = ts2.id").
			Where("tc2.environment_id = ? OR ts2.environment_id = ?", environmentID, environmentID)
	}
	if status != "" {
		query = query.Where("test_executions.status = ?", status)
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
	if projectID != "" {
		baseQuery = baseQuery.Joins("LEFT JOIN test_cases ON test_executions.test_case_id = test_cases.id").
			Joins("LEFT JOIN test_suites ON test_executions.test_suite_id = test_suites.id").
			Where("test_cases.project_id = ? OR test_suites.project_id = ?", projectID, projectID)
	}
	if environmentID != "" {
		baseQuery = baseQuery.Joins("LEFT JOIN test_cases tc2 ON test_executions.test_case_id = tc2.id").
			Joins("LEFT JOIN test_suites ts2 ON test_executions.test_suite_id = ts2.id").
			Where("tc2.environment_id = ? OR ts2.environment_id = ?", environmentID, environmentID)
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
		Preload("TestSuite").Preload("User").
		First(&execution, id).Error
	if err != nil {
		response.NotFound(c, "执行记录不存在")
		return
	}

	execution.User.Password = ""
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
			"screenshot_count": len(screenshots),
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
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 50
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
