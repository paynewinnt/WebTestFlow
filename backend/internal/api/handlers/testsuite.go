package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"webtestflow/backend/internal/executor"
	"webtestflow/backend/internal/models"
	"webtestflow/backend/pkg/database"
	"webtestflow/backend/pkg/response"
	"webtestflow/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

func GetTestSuites(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	projectID := c.Query("project_id")
	name := c.Query("name")

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 10
	}

	var testSuites []models.TestSuite
	var total int64

	query := database.DB.Model(&models.TestSuite{}).Where("status = ?", 1)
	if projectID != "" {
		query = query.Where("project_id = ?", projectID)
	}
	if name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}

	// Count total
	query.Count(&total)

	// Get paginated test suites with relations
	offset := (page - 1) * pageSize
	err := query.Preload("Project").Preload("Environment").Preload("User").
		Preload("TestCases").Preload("TestCases.Environment").
		Offset(offset).Limit(pageSize).Find(&testSuites).Error
	if err != nil {
		response.InternalServerError(c, "获取测试套件列表失败")
		return
	}

	// Clear user passwords, set test case counts, and calculate environment info
	for i := range testSuites {
		testSuites[i].User.Password = ""
		testSuites[i].TestCaseCount = len(testSuites[i].TestCases)
		testSuites[i].EnvironmentInfo = testSuites[i].GetEnvironmentInfo()
	}

	// Calculate global statistics (not filtered by pagination)
	var statistics struct {
		Total           int64 `json:"total"`
		Enabled         int64 `json:"enabled"`
		Scheduled       int64 `json:"scheduled"`
		Parallel        int64 `json:"parallel"`
	}

	statQuery := database.DB.Model(&models.TestSuite{})
	if projectID != "" {
		statQuery = statQuery.Where("project_id = ?", projectID)
	}

	// Count enabled test suites
	statQuery.Where("status = ?", 1).Count(&statistics.Enabled)
	statistics.Total = statistics.Enabled

	// Count scheduled test suites (have cron_expression)
	statQuery.Where("status = ? AND cron_expression != ''", 1).Count(&statistics.Scheduled)

	// Count parallel execution test suites
	statQuery.Where("status = ? AND is_parallel = ?", 1, true).Count(&statistics.Parallel)

	// Create response with both data and statistics
	responseData := gin.H{
		"list": testSuites,
		"total": total,
		"page": page,
		"page_size": pageSize,
		"statistics": statistics,
	}

	c.JSON(200, gin.H{
		"code": 200,
		"data": responseData,
		"message": "success",
	})
}

func CreateTestSuite(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var req struct {
		Name           string `json:"name" binding:"required,min=1,max=200"`
		Description    string `json:"description" binding:"max=1000"`
		ProjectID      uint   `json:"project_id" binding:"required"`
		EnvironmentID  *uint  `json:"environment_id"` // Made optional
		TestCaseIDs    []uint `json:"test_case_ids"`
		Tags           string `json:"tags" binding:"max=500"`
		Priority       int    `json:"priority" binding:"min=1,max=3"`
		CronExpression string `json:"cron_expression" binding:"max=100"`
		IsParallel     bool   `json:"is_parallel"`
		TimeoutMinutes int    `json:"timeout_minutes" binding:"min=1,max=1440"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Verify project exists and user has permission (admin has full access)
	var project models.Project
	err := database.DB.Where("id = ? AND status = ?", req.ProjectID, 1).First(&project).Error
	if err != nil {
		response.NotFound(c, "项目不存在")
		return
	}

	if !utils.HasPermissionOnProject(userID.(uint), req.ProjectID) {
		response.Forbidden(c, "无权限在该项目中创建测试套件")
		return
	}

	// Verify environment exists (if provided)
	if req.EnvironmentID != nil {
		var environment models.Environment
		err = database.DB.Where("id = ? AND status = ?", *req.EnvironmentID, 1).First(&environment).Error
		if err != nil {
			response.NotFound(c, "环境不存在")
			return
		}
	}

	// Check if test suite name exists in the project
	var existingTestSuite models.TestSuite
	err = database.DB.Where("name = ? AND project_id = ? AND status = ?", req.Name, req.ProjectID, 1).
		First(&existingTestSuite).Error
	if err == nil {
		response.BadRequest(c, "测试套件名称在该项目中已存在")
		return
	}

	// Verify test cases exist and belong to the same project (removed environment consistency check)
	var testCases []models.TestCase
	if len(req.TestCaseIDs) > 0 {
		err = database.DB.Where("id IN ? AND project_id = ? AND status = ?", req.TestCaseIDs, req.ProjectID, 1).
			Find(&testCases).Error
		if err != nil || len(testCases) != len(req.TestCaseIDs) {
			response.BadRequest(c, "部分测试用例不存在或不属于该项目")
			return
		}
	}

	testSuite := models.TestSuite{
		Name:           req.Name,
		Description:    req.Description,
		ProjectID:      req.ProjectID,
		EnvironmentID:  req.EnvironmentID, // Now nullable
		Tags:           req.Tags,
		Priority:       req.Priority,
		CronExpression: req.CronExpression,
		IsParallel:     req.IsParallel,
		TimeoutMinutes: req.TimeoutMinutes,
		Status:         1,
		UserID:         userID.(uint),
		TestCases:      testCases,
	}

	err = database.DB.Create(&testSuite).Error
	if err != nil {
		response.InternalServerError(c, "创建测试套件失败")
		return
	}

	// Associate test cases
	if len(testCases) > 0 {
		err = database.DB.Model(&testSuite).Association("TestCases").Replace(testCases)
		if err != nil {
			response.InternalServerError(c, "关联测试用例失败")
			return
		}
	}

	// Load relations for response
	database.DB.Preload("Project").Preload("Environment").Preload("User").
		Preload("TestCases").Preload("TestCases.Environment").
		First(&testSuite, testSuite.ID)
	testSuite.User.Password = ""
	testSuite.TestCaseCount = len(testSuite.TestCases)
	testSuite.EnvironmentInfo = testSuite.GetEnvironmentInfo()

	response.SuccessWithMessage(c, "创建成功", testSuite)
}

func GetTestSuite(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的测试套件ID")
		return
	}

	var testSuite models.TestSuite
	err = database.DB.Preload("Project").Preload("Environment").Preload("User").
		Preload("TestCases").Preload("TestCases.Environment").
		Where("status = ?", 1).First(&testSuite, id).Error
	if err != nil {
		response.NotFound(c, "测试套件不存在")
		return
	}

	testSuite.User.Password = ""
	testSuite.TestCaseCount = len(testSuite.TestCases)
	testSuite.EnvironmentInfo = testSuite.GetEnvironmentInfo()
	response.Success(c, testSuite)
}

func UpdateTestSuite(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的测试套件ID")
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var req struct {
		Name           string `json:"name" binding:"omitempty,min=1,max=200"`
		Description    string `json:"description" binding:"max=1000"`
		EnvironmentID  *uint  `json:"environment_id"` // Made optional
		TestCaseIDs    []uint `json:"test_case_ids"`
		Tags           string `json:"tags" binding:"max=500"`
		Priority       int    `json:"priority" binding:"min=1,max=3"`
		CronExpression string `json:"cron_expression" binding:"max=100"`
		IsParallel     bool   `json:"is_parallel"`
		TimeoutMinutes int    `json:"timeout_minutes" binding:"min=1,max=1440"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	var testSuite models.TestSuite
	err = database.DB.Where("id = ? AND status = ?", id, 1).First(&testSuite).Error
	if err != nil {
		response.NotFound(c, "测试套件不存在")
		return
	}

	if !utils.HasPermissionOnProject(userID.(uint), testSuite.ProjectID) {
		response.Forbidden(c, "无权限修改该测试套件")
		return
	}

	// Check name uniqueness if updating
	if req.Name != "" && req.Name != testSuite.Name {
		var existingTestSuite models.TestSuite
		err := database.DB.Where("name = ? AND project_id = ? AND id != ? AND status = ?",
			req.Name, testSuite.ProjectID, id, 1).First(&existingTestSuite).Error
		if err == nil {
			response.BadRequest(c, "测试套件名称在该项目中已存在")
			return
		}
		testSuite.Name = req.Name
	}

	// Update other fields
	if req.Description != "" {
		testSuite.Description = req.Description
	}
	if req.EnvironmentID != nil {
		// Verify environment exists
		var environment models.Environment
		err := database.DB.Where("id = ? AND status = ?", *req.EnvironmentID, 1).First(&environment).Error
		if err != nil {
			response.NotFound(c, "环境不存在")
			return
		}
		testSuite.EnvironmentID = req.EnvironmentID
	}
	if req.Tags != "" {
		testSuite.Tags = req.Tags
	}
	if req.Priority != 0 {
		testSuite.Priority = req.Priority
	}
	if req.CronExpression != "" {
		testSuite.CronExpression = req.CronExpression
	}
	testSuite.IsParallel = req.IsParallel
	if req.TimeoutMinutes != 0 {
		testSuite.TimeoutMinutes = req.TimeoutMinutes
	}

	// Update test cases if provided (removed environment consistency check)
	if req.TestCaseIDs != nil {
		var testCases []models.TestCase
		if len(req.TestCaseIDs) > 0 {
			err = database.DB.Where("id IN ? AND project_id = ? AND status = ?",
				req.TestCaseIDs, testSuite.ProjectID, 1).Find(&testCases).Error
			if err != nil || len(testCases) != len(req.TestCaseIDs) {
				response.BadRequest(c, "部分测试用例不存在或不属于该项目")
				return
			}
		}

		err = database.DB.Model(&testSuite).Association("TestCases").Replace(testCases)
		if err != nil {
			response.InternalServerError(c, "更新测试用例关联失败")
			return
		}
	}

	err = database.DB.Save(&testSuite).Error
	if err != nil {
		response.InternalServerError(c, "更新测试套件失败")
		return
	}

	// Load relations for response
	database.DB.Preload("Project").Preload("Environment").Preload("User").
		Preload("TestCases").Preload("TestCases.Environment").
		First(&testSuite, testSuite.ID)
	testSuite.User.Password = ""
	testSuite.TestCaseCount = len(testSuite.TestCases)
	testSuite.EnvironmentInfo = testSuite.GetEnvironmentInfo()

	response.SuccessWithMessage(c, "更新成功", testSuite)
}

func DeleteTestSuite(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的测试套件ID")
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var testSuite models.TestSuite
	err = database.DB.Where("id = ? AND status = ?", id, 1).First(&testSuite).Error
	if err != nil {
		response.NotFound(c, "测试套件不存在")
		return
	}

	if !utils.HasPermissionOnProject(userID.(uint), testSuite.ProjectID) {
		response.Forbidden(c, "无权限删除该测试套件")
		return
	}

	// Remove test case associations first
	err = database.DB.Model(&testSuite).Association("TestCases").Clear()
	if err != nil {
		response.InternalServerError(c, "清除测试用例关联失败")
		return
	}

	// Soft delete
	testSuite.Status = 0
	err = database.DB.Save(&testSuite).Error
	if err != nil {
		response.InternalServerError(c, "删除测试套件失败")
		return
	}

	response.SuccessWithMessage(c, "删除成功", nil)
}

func ExecuteTestSuite(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的测试套件ID")
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "用户未登录")
		return
	}

	// Parse request body for execution options (force visual execution)
	var req struct {
		IsVisual           bool  `json:"is_visual"`
		ContinueFailedOnly bool  `json:"continue_failed_only"` // 只执行未通过的测试用例
		ParentExecutionID  *uint `json:"parent_execution_id"`  // 原始执行ID，用于继续执行时判断测试用例状态
	}
	c.ShouldBindJSON(&req)
	// Force visual execution only
	req.IsVisual = true

	var testSuite models.TestSuite
	err = database.DB.Preload("Project").
		Where("id = ? AND status = ?", id, 1).First(&testSuite).Error
	if err != nil {
		response.NotFound(c, "测试套件不存在")
		return
	}

	// Manually load test cases with proper association to ensure correct data
	err = database.DB.Model(&testSuite).Association("TestCases").Find(&testSuite.TestCases, "status = ?", 1)
	if err != nil {
		response.InternalServerError(c, "加载测试用例失败")
		return
	}

	// Debug: Log loaded test cases
	log.Printf("Loaded %d test cases for suite ID=%d:", len(testSuite.TestCases), testSuite.ID)
	for i, tc := range testSuite.TestCases {
		log.Printf("  Test case %d: ID=%d, Name=%s", i+1, tc.ID, tc.Name)
	}

	// Check if user has permission to execute (admin has full access)
	if !utils.HasPermissionOnProject(userID.(uint), testSuite.ProjectID) {
		response.Forbidden(c, "无权限执行该测试套件")
		return
	}

	if len(testSuite.TestCases) == 0 {
		response.BadRequest(c, "测试套件中没有测试用例")
		return
	}

	// 如果是继续执行模式，过滤掉已经通过的测试用例
	var testCasesToExecute []models.TestCase
	if req.ContinueFailedOnly {
		log.Printf("Continue failed only mode: filtering passed test cases")
		
		if req.ParentExecutionID != nil {
			// 基于指定的原始执行ID来判断测试用例状态
			log.Printf("Using parent execution ID %d to determine test case status", *req.ParentExecutionID)
			for _, testCase := range testSuite.TestCases {
				// 查找该测试用例在指定套件执行中的执行记录
				var suiteExecution models.TestExecution
				err := database.DB.Where("test_case_id = ? AND parent_execution_id = ?", testCase.ID, *req.ParentExecutionID).
					First(&suiteExecution).Error
				
				if err != nil {
					// 如果没有找到执行记录，需要执行
					testCasesToExecute = append(testCasesToExecute, testCase)
					log.Printf("Test case %s (ID=%d): not executed in parent suite, will execute", testCase.Name, testCase.ID)
				} else if suiteExecution.Status != "passed" {
					// 如果状态不是通过，需要重新执行
					testCasesToExecute = append(testCasesToExecute, testCase)
					log.Printf("Test case %s (ID=%d): status=%s in parent suite, will execute", testCase.Name, testCase.ID, suiteExecution.Status)
				} else {
					// 如果状态是通过，跳过
					log.Printf("Test case %s (ID=%d): status=passed in parent suite, will skip", testCase.Name, testCase.ID)
				}
			}
		} else {
			// 兼容旧逻辑：基于全局最新独立执行记录判断
			log.Printf("No parent execution ID provided, using global latest execution records")
			for _, testCase := range testSuite.TestCases {
				// 查找该测试用例的最新执行记录
				var latestExecution models.TestExecution
				err := database.DB.Where("test_case_id = ? AND execution_type = 'test_case'", testCase.ID).
					Order("start_time DESC").First(&latestExecution).Error
				
				if err != nil {
					// 如果没有找到执行记录，说明从未执行过，需要执行
					testCasesToExecute = append(testCasesToExecute, testCase)
					log.Printf("Test case %s (ID=%d): no previous execution, will execute", testCase.Name, testCase.ID)
				} else if latestExecution.Status != "passed" {
					// 如果最新执行状态不是通过，需要重新执行
					testCasesToExecute = append(testCasesToExecute, testCase)
					log.Printf("Test case %s (ID=%d): last status=%s, will execute", testCase.Name, testCase.ID, latestExecution.Status)
				} else {
					// 如果最新执行状态是通过，跳过
					log.Printf("Test case %s (ID=%d): last status=passed, will skip", testCase.Name, testCase.ID)
				}
			}
		}
		
		if len(testCasesToExecute) == 0 {
			response.BadRequest(c, "所有测试用例都已通过，无需重新执行")
			return
		}
		
		log.Printf("Continue mode: will execute %d out of %d test cases", len(testCasesToExecute), len(testSuite.TestCases))
	} else {
		// 正常执行模式，执行所有用例
		testCasesToExecute = testSuite.TestCases
		log.Printf("Normal mode: will execute all %d test cases", len(testCasesToExecute))
	}

	// Check if executor is available
	if executor.GlobalExecutor == nil {
		response.InternalServerError(c, "测试执行引擎未初始化")
		return
	}

	runningCount := executor.GlobalExecutor.GetRunningCount()
	maxWorkers := executor.GlobalExecutor.GetMaxWorkers()
	
	// 检查并发容量：串行执行只需要1个worker，并行执行需要足够的worker数量
	requiredWorkers := 1 // 串行执行默认需要1个worker
	if testSuite.IsParallel {
		requiredWorkers = len(testCasesToExecute) // 并行执行需要与实际执行用例数量相等的worker
	}
	
	if runningCount+requiredWorkers > maxWorkers {
		executionMode := "串行"
		if testSuite.IsParallel {
			executionMode = "并行"
		}
		response.BadRequest(c, fmt.Sprintf("当前并发执行数不足以运行%s测试套件，请稍后再试。当前运行: %d, %s执行需要: %d, 最大容量: %d", 
			executionMode, runningCount, executionMode, requiredWorkers, maxWorkers))
		return
	}

	// Create main suite execution record (this will be shown in reports)
	suiteExecution := models.TestExecution{
		TestSuiteID:   &testSuite.ID,
		ExecutionType: "test_suite",
		Status:        "pending",
		StartTime:     time.Now(),
		TotalCount:    len(testSuite.TestCases), // 使用所有用例数量，包括已通过的
		PassedCount:   0,
		FailedCount:   0,
		UserID:        userID.(uint),
		ErrorMessage:  "",
		ExecutionLogs: "[]",
		Screenshots:   "[]",
	}

	err = database.DB.Create(&suiteExecution).Error
	if err != nil {
		response.InternalServerError(c, "创建套件执行记录失败")
		return
	}

	// Create individual test case execution records (internal tracking only)
	var executions []models.TestExecution
	var passedExecutions []models.TestExecution // 已通过的执行记录，不需要重新执行
	
	log.Printf("Creating execution records for ALL %d test cases in suite %d (continue mode: %v)", len(testSuite.TestCases), testSuite.ID, req.ContinueFailedOnly)
	
	for i, testCase := range testSuite.TestCases {
		// Fix: Create a local copy of the ID to avoid the range loop variable pointer issue
		testCaseID := testCase.ID
		log.Printf("Creating execution record %d/%d for test case ID=%d, Name=%s", i+1, len(testSuite.TestCases), testCaseID, testCase.Name)
		
		// 检查是否为继续执行模式且该用例已通过
		isAlreadyPassed := false
		var previousExecution models.TestExecution
		
		if req.ContinueFailedOnly && req.ParentExecutionID != nil {
			// 查找该测试用例在原始执行中的记录
			err := database.DB.Where("test_case_id = ? AND parent_execution_id = ?", testCase.ID, *req.ParentExecutionID).
				First(&previousExecution).Error
			if err == nil && previousExecution.Status == "passed" {
				isAlreadyPassed = true
				log.Printf("Test case %s (ID=%d): already passed in parent execution, will copy result", testCase.Name, testCase.ID)
			}
		}
		
		execution := models.TestExecution{
			TestCaseID:        &testCaseID, // Use pointer to local copy
			TestSuiteID:       &testSuite.ID,
			ParentExecutionID: &suiteExecution.ID,   // 关联到套件执行记录
			ExecutionType:     "test_case_internal", // 标记为内部记录
			UserID:            userID.(uint),
		}
		
		if isAlreadyPassed {
			// 复制之前通过的执行结果
			execution.Status = "passed"
			execution.StartTime = previousExecution.StartTime
			execution.EndTime = previousExecution.EndTime
			execution.Duration = previousExecution.Duration
			execution.TotalCount = previousExecution.TotalCount
			execution.PassedCount = previousExecution.PassedCount
			execution.FailedCount = previousExecution.FailedCount
			execution.ExecutionLogs = previousExecution.ExecutionLogs
			execution.Screenshots = previousExecution.Screenshots
			execution.ErrorMessage = ""
			log.Printf("Test case %s (ID=%d): copied passed result from previous execution", testCase.Name, testCase.ID)
		} else {
			// 设置为待执行状态
			execution.Status = "pending"
			execution.StartTime = time.Now()
			execution.ErrorMessage = ""
			execution.ExecutionLogs = "[]"
			execution.Screenshots = "[]"
			log.Printf("Test case %s (ID=%d): set to pending for execution", testCase.Name, testCase.ID)
		}

		err = database.DB.Create(&execution).Error
		if err != nil {
			log.Printf("Failed to create execution record for test case ID=%d: %v", testCaseID, err)
			response.InternalServerError(c, "创建用例执行记录失败")
			return
		}

		log.Printf("Successfully created execution record ID=%d for test case ID=%d", execution.ID, testCaseID)
		
		if isAlreadyPassed {
			passedExecutions = append(passedExecutions, execution)
		} else {
			executions = append(executions, execution)
		}
	}

	// Update suite execution status to running
	suiteExecution.Status = "running"
	database.DB.Save(&suiteExecution)

	// Execute all test cases asynchronously with comprehensive panic protection
	go func() {
		// 顶层panic恢复 - 防止ChromeDP的goroutine panic影响主流程
		defer func() {
			if r := recover(); r != nil {
				log.Printf("🚨 TOP-LEVEL PANIC recovered in test suite execution for suite %d: %v", testSuite.ID, r)
				// 确保套件状态被正确更新
				suiteExecution.Status = "failed"
				suiteExecution.ErrorMessage = fmt.Sprintf("Suite execution panic: %v", r)
				now := time.Now()
				suiteExecution.EndTime = &now
				suiteExecution.Duration = int(now.Sub(suiteExecution.StartTime).Milliseconds())
				database.DB.Save(&suiteExecution)
				log.Printf("🛡️ Service continues running despite ChromeDP panic")
			}
		}()

		passedCount := len(passedExecutions) // 初始化为已通过的测试用例数量
		failedCount := 0
		var allLogs []interface{}
		var allScreenshots []string
		completedExecutions := make(map[uint]bool)
		
		// 将已通过的测试用例标记为完成
		for _, passedExec := range passedExecutions {
			completedExecutions[passedExec.ID] = true
			log.Printf("Pre-marked test case execution %d as completed (already passed)", passedExec.ID)
		}

		defer func() {
			// Update suite execution record with final results
			now := time.Now()
			suiteExecution.EndTime = &now
			suiteExecution.Duration = int(now.Sub(suiteExecution.StartTime).Milliseconds())
			suiteExecution.PassedCount = passedCount
			suiteExecution.FailedCount = failedCount

			if failedCount > 0 {
				suiteExecution.Status = "failed"
				suiteExecution.ErrorMessage = fmt.Sprintf("套件执行完成，%d个用例失败", failedCount)
			} else {
				suiteExecution.Status = "passed"
			}

			// Save aggregated logs and screenshots
			if logsJSON, err := json.Marshal(allLogs); err == nil {
				suiteExecution.ExecutionLogs = string(logsJSON)
			}
			if screenshotsJSON, err := json.Marshal(allScreenshots); err == nil {
				suiteExecution.Screenshots = string(screenshotsJSON)
			}

			database.DB.Save(&suiteExecution)
			log.Printf("Test suite %d execution completed: %d passed, %d failed", testSuite.ID, passedCount, failedCount)

			// Check for stuck executions
			for _, exec := range executions {
				if !completedExecutions[exec.ID] {
					var finalExecution models.TestExecution
					if err := database.DB.First(&finalExecution, exec.ID).Error; err == nil {
						if finalExecution.Status == "running" {
							finalExecution.Status = "failed"
							finalExecution.ErrorMessage = "Test case execution did not complete properly"
							now := time.Now()
							finalExecution.EndTime = &now
							finalExecution.Duration = int(now.Sub(finalExecution.StartTime).Milliseconds())
							database.DB.Save(&finalExecution)
							log.Printf("Fixed stuck test case execution %d status from 'running' to 'failed'", exec.ID)
						}
					}
				}
			}
		}()

		if testSuite.IsParallel {
			// 并行执行模式
			log.Printf("Starting PARALLEL execution of %d test cases for suite %d", len(executions), testSuite.ID)
			
			var wg sync.WaitGroup
			var mu sync.Mutex  // 用于保护共享变量
			
			// 启动所有测试用例的并行执行
			for i, execution := range executions {
				// Check if the suite execution has been cancelled before starting each test case
				var currentSuiteExecution models.TestExecution
				if err := database.DB.First(&currentSuiteExecution, suiteExecution.ID).Error; err == nil {
					if currentSuiteExecution.Status == "cancelled" {
						log.Printf("Suite execution %d has been cancelled, stopping parallel execution launch", suiteExecution.ID)
						// Mark all remaining pending executions as cancelled
						for j := i; j < len(executions); j++ {
							executions[j].Status = "cancelled"
							executions[j].ErrorMessage = "测试套件执行被用户停止"
							now := time.Now()
							executions[j].EndTime = &now
							database.DB.Save(&executions[j])
						}
						break
					}
				}
				
				wg.Add(1)
				go func(exec models.TestExecution, index int) {
					defer wg.Done()
					defer func() {
						if r := recover(); r != nil {
							log.Printf("🚨 PANIC recovered in parallel test case execution %d: %v", exec.ID, r)
							mu.Lock()
							failedCount++
							completedExecutions[exec.ID] = true
							mu.Unlock()
							
							// Save failed execution to database
							exec.Status = "failed"
							exec.ErrorMessage = fmt.Sprintf("Test case execution panic: %v", r)
							now := time.Now()
							exec.EndTime = &now
							exec.Duration = int(now.Sub(exec.StartTime).Milliseconds())
							database.DB.Save(&exec)
						}
					}()

					// Check again if suite is cancelled before starting execution
					var currentSuiteExecution models.TestExecution
					if err := database.DB.First(&currentSuiteExecution, suiteExecution.ID).Error; err == nil {
						if currentSuiteExecution.Status == "cancelled" {
							log.Printf("Suite execution %d has been cancelled, skipping test case %d", suiteExecution.ID, exec.ID)
							exec.Status = "cancelled"
							exec.ErrorMessage = "测试套件执行被用户停止"
							now := time.Now()
							exec.EndTime = &now
							database.DB.Save(&exec)
							return
						}
					}

					exec.Status = "running"
					database.DB.Save(&exec)

					// Load test case with relations
					var testCase models.TestCase
					database.DB.Preload("Environment").Preload("Device").
						First(&testCase, *exec.TestCaseID)

					// Add small random delay to prevent Chrome port conflicts
					if index > 0 {
						time.Sleep(time.Duration(index) * 500 * time.Millisecond)
					}

					log.Printf("Starting PARALLEL execution of test case %d: %s", index+1, testCase.Name)

					// Execute test case
					result := executor.GlobalExecutor.ExecuteTestCaseDirectly(&exec, &testCase)

					log.Printf("Completed PARALLEL execution of test case %d: %s (Success: %v)", index+1, testCase.Name, result.Success)

					// Update execution with result (thread-safe)
					mu.Lock()
					if result.Success {
						exec.Status = "passed"
						passedCount++
					} else {
						exec.Status = "failed"
						failedCount++
						exec.ErrorMessage = result.ErrorMessage
					}

					now := time.Now()
					exec.EndTime = &now
					exec.Duration = int(now.Sub(exec.StartTime).Milliseconds())

					// Save logs and screenshots
					if logsJSON, err := json.Marshal(result.Logs); err == nil {
						exec.ExecutionLogs = string(logsJSON)
						// Convert logs to interface{} for aggregation
						for _, logItem := range result.Logs {
							allLogs = append(allLogs, logItem)
						}
					}
					if screenshotsJSON, err := json.Marshal(result.Screenshots); err == nil {
						exec.Screenshots = string(screenshotsJSON)
						allScreenshots = append(allScreenshots, result.Screenshots...)
					}

					database.DB.Save(&exec)

					// Notify executor that database update is complete
					if executor.GlobalExecutor != nil {
						executor.GlobalExecutor.NotifyExecutionComplete(exec.ID)
					}

					// Save performance metrics if available
					if result.Metrics != nil {
						result.Metrics.ExecutionID = exec.ID
						database.DB.Create(result.Metrics)
					}

					completedExecutions[exec.ID] = true
					mu.Unlock()

					log.Printf("PARALLEL test case execution %d completed with status: %s", exec.ID, exec.Status)
				}(execution, i)
			}

			// 等待所有并行执行完成
			wg.Wait()
			log.Printf("All PARALLEL test case executions completed for suite %d", testSuite.ID)

		} else {
			// 串行执行模式（原有逻辑）
			log.Printf("Starting SERIAL execution of %d test cases for suite %d", len(executions), testSuite.ID)
			
			for i, execution := range executions {
				// Check if the suite execution has been cancelled before starting each test case
				var currentSuiteExecution models.TestExecution
				if err := database.DB.First(&currentSuiteExecution, suiteExecution.ID).Error; err == nil {
					if currentSuiteExecution.Status == "cancelled" {
						log.Printf("Suite execution %d has been cancelled, stopping execution of remaining test cases", suiteExecution.ID)
						// Mark all remaining pending executions as cancelled
						for j := i; j < len(executions); j++ {
							executions[j].Status = "cancelled"
							executions[j].ErrorMessage = "测试套件执行被用户停止"
							now := time.Now()
							executions[j].EndTime = &now
							database.DB.Save(&executions[j])
						}
						break
					}
				}

				execution.Status = "running"
				database.DB.Save(&execution)

				// Load test case with relations
				var testCase models.TestCase
				database.DB.Preload("Environment").Preload("Device").
					First(&testCase, *execution.TestCaseID)

				// Add isolation delay between test cases to prevent interference
				if i > 0 {
					log.Printf("Adding isolation delay before executing test case %d/%d", i+1, len(executions))
					time.Sleep(2 * time.Second) // Give previous execution time to fully complete
				}

				log.Printf("Starting SERIAL execution of test case %d/%d: %s", i+1, len(executions), testCase.Name)

				// Execute with panic recovery for each individual test case
				var result executor.ExecutionResult
				func() {
					defer func() {
						if r := recover(); r != nil {
							log.Printf("🚨 PANIC recovered in individual test case execution %d: %v", execution.ID, r)
							result = executor.ExecutionResult{
								Success:      false,
								ErrorMessage: fmt.Sprintf("Test case execution panic: %v", r),
								Screenshots:  []string{},
								Logs: []executor.ExecutionLog{
									{
										Timestamp: time.Now(),
										Level:     "error",
										Message:   fmt.Sprintf("Test case panic recovered: %v", r),
										StepIndex: -1,
									},
								},
								Metrics: nil,
							}
						}
					}()

					// Use direct execution to avoid ChromeDP concurrency issues
					result = executor.GlobalExecutor.ExecuteTestCaseDirectly(&execution, &testCase)
				}()

				// Update execution with result
				if result.Success {
					execution.Status = "passed"
					passedCount++
				} else {
					execution.Status = "failed"
					failedCount++
					execution.ErrorMessage = result.ErrorMessage
				}

				now := time.Now()
				execution.EndTime = &now
				execution.Duration = int(now.Sub(execution.StartTime).Milliseconds())

				// Save logs and screenshots
				if logsJSON, err := json.Marshal(result.Logs); err == nil {
					execution.ExecutionLogs = string(logsJSON)
					// Convert logs to interface{} for aggregation
					for _, logItem := range result.Logs {
						allLogs = append(allLogs, logItem)
					}
				}
				if screenshotsJSON, err := json.Marshal(result.Screenshots); err == nil {
					execution.Screenshots = string(screenshotsJSON)
					allScreenshots = append(allScreenshots, result.Screenshots...)
				}

				database.DB.Save(&execution)
				log.Printf("SERIAL test case execution %d completed with status: %s", execution.ID, execution.Status)

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
				// Mark this execution as completed
				completedExecutions[execution.ID] = true

				log.Printf("Completed SERIAL test case %d/%d: %s", i+1, len(executions), testCase.Name)
			}
		}
	}()

	// Load suite execution with relations for response
	database.DB.Preload("TestSuite").Preload("TestSuite.Project").Preload("User").First(&suiteExecution, suiteExecution.ID)
	suiteExecution.User.Password = ""

	response.SuccessWithMessage(c, "测试套件执行已启动", suiteExecution)
}

func StopTestSuite(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的测试套件ID")
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var testSuite models.TestSuite
	err = database.DB.Where("id = ? AND status = ?", id, 1).First(&testSuite).Error
	if err != nil {
		response.NotFound(c, "测试套件不存在")
		return
	}

	// Check if user has permission to stop (admin has full access)
	if !utils.HasPermissionOnProject(userID.(uint), testSuite.ProjectID) {
		response.Forbidden(c, "无权限停止该测试套件")
		return
	}

	// Find all running or pending executions for this test suite
	var executions []models.TestExecution
	err = database.DB.Where("test_suite_id = ? AND (status = ? OR status = ?)",
		id, "running", "pending").Find(&executions).Error
	if err != nil {
		response.InternalServerError(c, "查询执行记录失败")
		return
	}

	if len(executions) == 0 {
		response.BadRequest(c, "没有正在运行的执行记录")
		return
	}

	// Cancel running executions in the executor
	cancelledCount := 0
	if executor.GlobalExecutor != nil {
		for _, execution := range executions {
			if execution.Status == "running" {
				success := executor.GlobalExecutor.CancelExecution(execution.ID)
				if success {
					cancelledCount++
					log.Printf("Successfully cancelled execution %d", execution.ID)
				}
			}
		}
	}

	// Stop all running/pending executions in database
	err = database.DB.Model(&models.TestExecution{}).
		Where("test_suite_id = ? AND (status = ? OR status = ?)", id, "running", "pending").
		Updates(models.TestExecution{Status: "cancelled"}).Error
	if err != nil {
		response.InternalServerError(c, "停止测试套件执行失败")
		return
	}

	log.Printf("Stopped test suite %d: %d executions cancelled in executor, %d executions marked as cancelled in database", 
		id, cancelledCount, len(executions))

	response.SuccessWithMessage(c, "测试套件执行已停止", gin.H{
		"stopped_count": len(executions),
		"cancelled_count": cancelledCount,
	})
}

func GetTestSuiteExecutions(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的测试套件ID")
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

	// Verify test suite exists
	var testSuite models.TestSuite
	err = database.DB.Where("id = ? AND status = ?", id, 1).First(&testSuite).Error
	if err != nil {
		response.NotFound(c, "测试套件不存在")
		return
	}

	var executions []models.TestExecution
	var total int64

	// Query executions for this test suite
	query := database.DB.Model(&models.TestExecution{}).Where("test_suite_id = ?", id)

	// Count total
	query.Count(&total)

	// Get paginated executions with relations
	offset := (page - 1) * pageSize
	err = query.Preload("TestCase").Preload("TestCase.Project").Preload("TestCase.Environment").Preload("TestCase.Device").Preload("User").
		Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&executions).Error
	if err != nil {
		response.InternalServerError(c, "获取测试套件执行记录失败")
		return
	}

	// Clear user passwords
	for i := range executions {
		executions[i].User.Password = ""
	}

	response.Page(c, executions, total, page, pageSize)
}

// GetLatestTestSuiteReport generates a report with latest test results for all test cases in a suite
func GetLatestTestSuiteReport(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的测试套件ID")
		return
	}

	var testSuite models.TestSuite
	err = database.DB.Preload("Project").
		Where("id = ? AND status = ?", id, 1).First(&testSuite).Error
	if err != nil {
		response.NotFound(c, "测试套件不存在")
		return
	}

	// Load test cases with relations
	err = database.DB.Model(&testSuite).Association("TestCases").Find(&testSuite.TestCases, "status = ?", 1)
	if err != nil {
		response.InternalServerError(c, "加载测试用例失败")
		return
	}

	// For each test case, get its latest execution result
	var latestExecutions []models.TestExecution
	totalCount := len(testSuite.TestCases)
	passedCount := 0
	failedCount := 0

	// Define the cutoff time for "not executed" status (7 days ago)
	sevenDaysAgo := time.Now().AddDate(0, 0, -7)

	for _, testCase := range testSuite.TestCases {
		var latestValidExecution models.TestExecution
		found := false
		
		// Strategy 1: Look for the latest valid execution across ALL test suite runs for this test case
		// This approach finds the most recent execution regardless of which suite execution it belongs to
		// Include both 'test_case' and 'test_case_internal' execution types
		err := database.DB.Preload("TestCase").Preload("TestCase.Environment").Preload("TestCase.Device").
			Where("test_case_id = ? AND execution_type IN ('test_case', 'test_case_internal') AND status IN ('passed', 'failed')", testCase.ID).
			Order("start_time DESC").First(&latestValidExecution).Error
		
		if err == nil {
			found = true
		}
		
		if !found {
			// Strategy 2: Look for standalone test case executions (individual test case runs)
			err := database.DB.Preload("TestCase").Preload("TestCase.Environment").Preload("TestCase.Device").
				Where("test_case_id = ? AND parent_execution_id IS NULL AND execution_type = 'test_case' AND status IN ('passed', 'failed')", testCase.ID).
				Order("start_time DESC").First(&latestValidExecution).Error
			
			if err == nil {
				found = true
			}
		}
		
		if !found {
			// Strategy 3: Check if there's any execution within 7 days
			var anyRecentExecution models.TestExecution
			recentErr := database.DB.Preload("TestCase").Preload("TestCase.Environment").Preload("TestCase.Device").
				Where("test_case_id = ? AND execution_type IN ('test_case', 'test_case_internal') AND start_time > ?", testCase.ID, sevenDaysAgo).
				Order("start_time DESC").First(&anyRecentExecution).Error
			
			if recentErr != nil {
				// No execution within 7 days, mark as not_executed
				latestValidExecution = models.TestExecution{
					TestCaseID:    &testCase.ID,
					TestCase:      testCase,
					ExecutionType: "test_case",
					Status:        "not_executed",
					ErrorMessage:  "该用例在过去7天内未执行",
					TotalCount:    0,
					PassedCount:   0,
					FailedCount:   0,
					Duration:      0,
					ExecutionLogs: "[]",
					Screenshots:   "[]",
					UserID:        1, // Default user
				}
			} else {
				// Found recent execution but it doesn't have passed/failed status
				// This means it might be pending, cancelled, etc.
				// Check if there's any historical valid execution
				var historicalValidExecution models.TestExecution
				historyErr := database.DB.Preload("TestCase").Preload("TestCase.Environment").Preload("TestCase.Device").
					Where("test_case_id = ? AND execution_type IN ('test_case', 'test_case_internal') AND status IN ('passed', 'failed')", testCase.ID).
					Order("start_time DESC").First(&historicalValidExecution).Error
				
				if historyErr != nil {
					// No valid execution found in history, mark as not_executed
					latestValidExecution = models.TestExecution{
						TestCaseID:    &testCase.ID,
						TestCase:      testCase,
						ExecutionType: "test_case",
						Status:        "not_executed",
						ErrorMessage:  "该用例最近的执行未产生有效结果",
						TotalCount:    0,
						PassedCount:   0,
						FailedCount:   0,
						Duration:      0,
						ExecutionLogs: "[]",
						Screenshots:   "[]",
						UserID:        1, // Default user
					}
				} else {
					// Use the historical valid execution
					latestValidExecution = historicalValidExecution
				}
			}
		}
		
		// Count results based on valid executions only
		if latestValidExecution.Status == "passed" {
			passedCount++
		} else if latestValidExecution.Status == "failed" {
			failedCount++
		}
		
		latestExecutions = append(latestExecutions, latestValidExecution)
	}

	// Create a virtual suite execution for reporting
	now := time.Now()
	virtualSuiteExecution := models.TestExecution{
		TestSuiteID:   &testSuite.ID,
		TestSuite:     testSuite,
		ExecutionType: "test_suite_latest",
		Status:        "completed",
		StartTime:     now,
		EndTime:       &now,
		Duration:      0,
		TotalCount:    totalCount,
		PassedCount:   passedCount,
		FailedCount:   failedCount,
		ErrorMessage:  "",
		ExecutionLogs: "[]",
		Screenshots:   "[]",
		UserID:        1, // Default user
	}

	// Determine overall status
	if failedCount > 0 {
		virtualSuiteExecution.Status = "failed"
	} else if passedCount == totalCount {
		virtualSuiteExecution.Status = "passed"
	} else {
		virtualSuiteExecution.Status = "partial"
	}

	response.Success(c, gin.H{
		"suite_execution": virtualSuiteExecution,
		"test_suite":      testSuite,
		"executions":      latestExecutions,
		"summary": gin.H{
			"total_count":  totalCount,
			"passed_count": passedCount,
			"failed_count": failedCount,
			"not_executed": totalCount - passedCount - failedCount,
		},
	})
}

// DownloadLatestTestSuiteReportHTML exports HTML report with latest test results
func DownloadLatestTestSuiteReportHTML(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的测试套件ID")
		return
	}

	var testSuite models.TestSuite
	err = database.DB.Preload("Project").
		Where("id = ? AND status = ?", id, 1).First(&testSuite).Error
	if err != nil {
		response.NotFound(c, "测试套件不存在")
		return
	}

	// Load test cases with relations
	err = database.DB.Model(&testSuite).Association("TestCases").Find(&testSuite.TestCases, "status = ?", 1)
	if err != nil {
		response.InternalServerError(c, "加载测试用例失败")
		return
	}

	// Get latest executions for all test cases
	var latestExecutions []models.TestExecution
	totalCount := len(testSuite.TestCases)
	passedCount := 0
	failedCount := 0

	// Use the same logic as GetLatestTestSuiteReport for consistency
	sevenDaysAgo := time.Now().AddDate(0, 0, -7)
	
	for _, testCase := range testSuite.TestCases {
		var latestValidExecution models.TestExecution
		found := false
		
		// Strategy 1: Look for the latest valid execution across ALL test suite runs for this test case
		// Include both 'test_case' and 'test_case_internal' execution types
		err := database.DB.Preload("TestCase").Preload("TestCase.Environment").Preload("TestCase.Device").
			Where("test_case_id = ? AND execution_type IN ('test_case', 'test_case_internal') AND status IN ('passed', 'failed')", testCase.ID).
			Order("start_time DESC").First(&latestValidExecution).Error
		
		if err == nil {
			found = true
		}
		
		if !found {
			// Strategy 2: Look for standalone test case executions
			err := database.DB.Preload("TestCase").Preload("TestCase.Environment").Preload("TestCase.Device").
				Where("test_case_id = ? AND parent_execution_id IS NULL AND execution_type IN ('test_case', 'test_case_internal') AND status IN ('passed', 'failed')", testCase.ID).
				Order("start_time DESC").First(&latestValidExecution).Error
			
			if err == nil {
				found = true
			}
		}
		
		if !found {
			// Strategy 3: Check if there's any execution within 7 days
			var anyRecentExecution models.TestExecution
			recentErr := database.DB.Preload("TestCase").Preload("TestCase.Environment").Preload("TestCase.Device").
				Where("test_case_id = ? AND execution_type IN ('test_case', 'test_case_internal') AND start_time > ?", testCase.ID, sevenDaysAgo).
				Order("start_time DESC").First(&anyRecentExecution).Error
			
			if recentErr != nil {
				// No execution within 7 days, mark as not_executed
				latestValidExecution = models.TestExecution{
					TestCaseID:    &testCase.ID,
					TestCase:      testCase,
					ExecutionType: "test_case",
					Status:        "not_executed",
					ErrorMessage:  "该用例在过去7天内未执行",
					TotalCount:    0,
					PassedCount:   0,
					FailedCount:   0,
					Duration:      0,
					ExecutionLogs: "[]",
					Screenshots:   "[]",
					// Don't set StartTime for not_executed cases
					UserID:        1, // Default user
				}
				// Don't set EndTime for not_executed cases
			} else {
				// Found recent execution but it doesn't have passed/failed status
				// Check if there's any historical valid execution
				var historicalValidExecution models.TestExecution
				historyErr := database.DB.Preload("TestCase").Preload("TestCase.Environment").Preload("TestCase.Device").
					Where("test_case_id = ? AND execution_type IN ('test_case', 'test_case_internal') AND status IN ('passed', 'failed')", testCase.ID).
					Order("start_time DESC").First(&historicalValidExecution).Error
				
				if historyErr != nil {
					// No valid execution found in history, mark as not_executed
					latestValidExecution = models.TestExecution{
						TestCaseID:    &testCase.ID,
						TestCase:      testCase,
						ExecutionType: "test_case",
						Status:        "not_executed",
						ErrorMessage:  "该用例最近的执行未产生有效结果",
						TotalCount:    0,
						PassedCount:   0,
						FailedCount:   0,
						Duration:      0,
						ExecutionLogs: "[]",
						Screenshots:   "[]",
						// Don't set StartTime for not_executed cases
						UserID:        1, // Default user
					}
					// Don't set EndTime for not_executed cases
				} else {
					// Use the historical valid execution
					latestValidExecution = historicalValidExecution
				}
			}
		}
		
		// Count results based on valid executions only
		if latestValidExecution.Status == "passed" {
			passedCount++
		} else if latestValidExecution.Status == "failed" {
			failedCount++
		}
		
		latestExecutions = append(latestExecutions, latestValidExecution)
	}

	// Create virtual suite execution
	now := time.Now()
	virtualSuiteExecution := models.TestExecution{
		TestSuiteID:   &testSuite.ID,
		TestSuite:     testSuite,
		ExecutionType: "test_suite_latest",
		Status:        "completed",
		StartTime:     now,
		EndTime:       &now,
		Duration:      0,
		TotalCount:    totalCount,
		PassedCount:   passedCount,
		FailedCount:   failedCount,
		ErrorMessage:  "",
		ExecutionLogs: "[]",
		Screenshots:   "[]",
		UserID:        1, // Default user
	}

	if failedCount > 0 {
		virtualSuiteExecution.Status = "failed"
	} else if passedCount == totalCount {
		virtualSuiteExecution.Status = "passed"
	} else {
		virtualSuiteExecution.Status = "partial"
	}

	// Generate HTML report
	htmlContent := generateTestSuiteLatestHTML(virtualSuiteExecution, testSuite, latestExecutions)

	// Set response headers for file download
	filename := fmt.Sprintf("%s_测试报告_%s.html", testSuite.Name, now.Format("20060102150405"))
	c.Header("Content-Type", "text/html; charset=utf-8")
	// 使用同时支持 filename 和 filename* 格式
	encodedFilename := url.QueryEscape(filename)
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, filename, encodedFilename))

	c.String(http.StatusOK, htmlContent)
}

// DownloadLatestTestSuiteReportPDF exports PDF report with latest test results
func DownloadLatestTestSuiteReportPDF(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的测试套件ID")
		return
	}

	var testSuite models.TestSuite
	err = database.DB.Preload("Project").
		Where("id = ? AND status = ?", id, 1).First(&testSuite).Error
	if err != nil {
		response.NotFound(c, "测试套件不存在")
		return
	}

	// Load test cases
	err = database.DB.Model(&testSuite).Association("TestCases").Find(&testSuite.TestCases, "status = ?", 1)
	if err != nil {
		response.InternalServerError(c, "加载测试用例失败")
		return
	}

	// Get latest executions
	var latestExecutions []models.TestExecution
	totalCount := len(testSuite.TestCases)
	passedCount := 0
	failedCount := 0

	// Use the same logic as GetLatestTestSuiteReport for consistency
	sevenDaysAgo := time.Now().AddDate(0, 0, -7)
	
	for _, testCase := range testSuite.TestCases {
		var latestValidExecution models.TestExecution
		found := false
		
		// Strategy 1: Look for the latest valid execution across ALL test suite runs for this test case
		// Include both 'test_case' and 'test_case_internal' execution types
		err := database.DB.Preload("TestCase").Preload("TestCase.Environment").Preload("TestCase.Device").
			Where("test_case_id = ? AND execution_type IN ('test_case', 'test_case_internal') AND status IN ('passed', 'failed')", testCase.ID).
			Order("start_time DESC").First(&latestValidExecution).Error
		
		if err == nil {
			found = true
		}
		
		if !found {
			// Strategy 2: Look for standalone test case executions
			err := database.DB.Preload("TestCase").Preload("TestCase.Environment").Preload("TestCase.Device").
				Where("test_case_id = ? AND parent_execution_id IS NULL AND execution_type IN ('test_case', 'test_case_internal') AND status IN ('passed', 'failed')", testCase.ID).
				Order("start_time DESC").First(&latestValidExecution).Error
			
			if err == nil {
				found = true
			}
		}
		
		if !found {
			// Strategy 3: Check if there's any execution within 7 days
			var anyRecentExecution models.TestExecution
			recentErr := database.DB.Preload("TestCase").Preload("TestCase.Environment").Preload("TestCase.Device").
				Where("test_case_id = ? AND execution_type IN ('test_case', 'test_case_internal') AND start_time > ?", testCase.ID, sevenDaysAgo).
				Order("start_time DESC").First(&anyRecentExecution).Error
			
			if recentErr != nil {
				// No execution within 7 days, mark as not_executed
				latestValidExecution = models.TestExecution{
					TestCaseID:    &testCase.ID,
					TestCase:      testCase,
					ExecutionType: "test_case",
					Status:        "not_executed",
					ErrorMessage:  "该用例在过去7天内未执行",
					Duration:      0,
					// Don't set StartTime for not_executed cases
					UserID:        1, // Default user
				}
				// Don't set EndTime for not_executed cases
			} else {
				// Found recent execution but it doesn't have passed/failed status
				// Check if there's any historical valid execution
				var historicalValidExecution models.TestExecution
				historyErr := database.DB.Preload("TestCase").Preload("TestCase.Environment").Preload("TestCase.Device").
					Where("test_case_id = ? AND execution_type IN ('test_case', 'test_case_internal') AND status IN ('passed', 'failed')", testCase.ID).
					Order("start_time DESC").First(&historicalValidExecution).Error
				
				if historyErr != nil {
					// No valid execution found in history, mark as not_executed
					latestValidExecution = models.TestExecution{
						TestCaseID:    &testCase.ID,
						TestCase:      testCase,
						ExecutionType: "test_case",
						Status:        "not_executed",
						ErrorMessage:  "该用例最近的执行未产生有效结果",
						Duration:      0,
						// Don't set StartTime for not_executed cases
						UserID:        1, // Default user
					}
					// Don't set EndTime for not_executed cases
				} else {
					// Use the historical valid execution
					latestValidExecution = historicalValidExecution
				}
			}
		}
		
		// Count results based on valid executions only
		if latestValidExecution.Status == "passed" {
			passedCount++
		} else if latestValidExecution.Status == "failed" {
			failedCount++
		}
		
		latestExecutions = append(latestExecutions, latestValidExecution)
	}

	// Create virtual suite execution
	now := time.Now()
	virtualSuiteExecution := models.TestExecution{
		TestSuiteID:   &testSuite.ID,
		TestSuite:     testSuite,
		ExecutionType: "test_suite_latest",
		Status:        "completed",
		StartTime:     now,
		EndTime:       &now,
		Duration:      0,
		TotalCount:    totalCount,
		PassedCount:   passedCount,
		FailedCount:   failedCount,
		ErrorMessage:  "",
		ExecutionLogs: "[]",
		Screenshots:   "[]",
		UserID:        1, // Default user
	}

	if failedCount > 0 {
		virtualSuiteExecution.Status = "failed"
	} else if passedCount == totalCount {
		virtualSuiteExecution.Status = "passed"
	} else {
		virtualSuiteExecution.Status = "partial"
	}

	// Generate HTML first
	htmlContent := generateTestSuiteLatestHTML(virtualSuiteExecution, testSuite, latestExecutions)

	// Convert to PDF using Chrome
	pdfData, err := generatePDFFromHTML(htmlContent)
	if err != nil {
		response.InternalServerError(c, "生成PDF报告失败: "+err.Error())
		return
	}

	// Set response headers for PDF download
	filename := fmt.Sprintf("%s_测试报告_%s.pdf", testSuite.Name, now.Format("20060102150405"))
	c.Header("Content-Type", "application/pdf")
	// 使用同时支持 filename 和 filename* 格式
	encodedFilename := url.QueryEscape(filename)
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, filename, encodedFilename))
	c.Header("Content-Length", strconv.Itoa(len(pdfData)))

	c.Data(http.StatusOK, "application/pdf", pdfData)
}


// generateTestSuiteLatestHTML generates HTML report for test suite with latest results
func generateTestSuiteLatestHTML(execution models.TestExecution, testSuite models.TestSuite, latestExecutions []models.TestExecution) string {
	// Get server base URL
	baseURL := getServerBaseURL()

	// Generate test case results table
	var resultsHTML string
	for _, exec := range latestExecutions {
		statusClass := "status-" + exec.Status
		statusText := exec.Status
		switch exec.Status {
		case "passed":
			statusText = "通过"
		case "failed":
			statusText = "失败"
		case "not_executed":
			statusText = "未执行"
		}
		
		environmentName := ""
		deviceName := ""
		if exec.TestCase.Environment.Name != "" {
			environmentName = exec.TestCase.Environment.Name
		}
		if exec.TestCase.Device.Name != "" {
			deviceName = fmt.Sprintf("%s (%dx%d)", exec.TestCase.Device.Name, exec.TestCase.Device.Width, exec.TestCase.Device.Height)
		}
		
		resultsHTML += fmt.Sprintf(`
		<tr>
			<td>%s</td>
			<td>%s</td>
			<td>%s</td>
			<td><span class="%s">%s</span></td>
			<td>%s</td>
		</tr>`,
			exec.TestCase.Name,
			environmentName,
			deviceName,
			statusClass,
			statusText,
			exec.StartTime.Format("2006-01-02 15:04:05"),
		)
	}

	// Calculate overall pass rate
	passRate := float64(0)
	if execution.TotalCount > 0 {
		passRate = float64(execution.PassedCount) / float64(execution.TotalCount) * 100
	}

	// Generate HTML report
	html := fmt.Sprintf(`
<!DOCTYPE html>
<html lang="zh-CN">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>测试套件最新结果报告 - %s</title>
	<style>
		body { font-family: Arial, sans-serif; margin: 20px; }
		.header { border-bottom: 2px solid #007bff; padding-bottom: 20px; margin-bottom: 30px; }
		.title { color: #007bff; font-size: 28px; margin: 0; }
		.subtitle { color: #6c757d; font-size: 16px; margin: 5px 0; }
		.info-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 20px; margin-bottom: 30px; }
		.info-section { background: #f8f9fa; padding: 15px; border-radius: 8px; }
		.info-section h3 { color: #495057; margin: 0 0 10px 0; font-size: 16px; }
		.info-row { margin: 5px 0; }
		.info-label { font-weight: bold; color: #495057; }
		.summary { background: linear-gradient(135deg, #007bff, #0056b3); color: white; padding: 20px; border-radius: 10px; margin-bottom: 30px; }
		.summary-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 20px; text-align: center; }
		.summary-item { background: rgba(255, 255, 255, 0.1); padding: 15px; border-radius: 8px; }
		.summary-number { font-size: 24px; font-weight: bold; margin-bottom: 5px; }
		.summary-label { font-size: 14px; opacity: 0.9; }
		table { width: 100%%; border-collapse: collapse; margin-top: 20px; }
		th, td { border: 1px solid #dee2e6; padding: 12px; text-align: left; }
		th { background-color: #007bff; color: white; font-weight: bold; }
		tr:nth-child(even) { background-color: #f8f9fa; }
		.status-passed { color: #28a745; font-weight: bold; }
		.status-failed { color: #dc3545; font-weight: bold; }
		.status-not_executed { color: #6c757d; font-weight: bold; }
		.footer { margin-top: 30px; text-align: center; color: #6c757d; font-size: 12px; }
	</style>
</head>
<body>
	<div class="header">
		<h1 class="title">测试套件最新结果报告</h1>
		<div class="subtitle">%s - %s</div>
		<div class="subtitle">报告生成时间: %s</div>
	</div>
	
	<div class="info-grid">
		<div class="info-section">
			<h3>测试套件信息</h3>
			<div class="info-row"><span class="info-label">套件名称:</span> %s</div>
			<div class="info-row"><span class="info-label">项目名称:</span> %s</div>
			<div class="info-row"><span class="info-label">套件描述:</span> %s</div>
		</div>
		<div class="info-section">
			<h3>执行概要</h3>
			<div class="info-row"><span class="info-label">执行类型:</span> 最新结果汇总</div>
			<div class="info-row"><span class="info-label">数据来源:</span> 各用例最新执行</div>
			<div class="info-row"><span class="info-label">通过率:</span> %.1f%%</div>
		</div>
	</div>
	
	<div class="summary">
		<div class="summary-grid">
			<div class="summary-item">
				<div class="summary-number">%d</div>
				<div class="summary-label">总用例数</div>
			</div>
			<div class="summary-item">
				<div class="summary-number">%d</div>
				<div class="summary-label">通过</div>
			</div>
			<div class="summary-item">
				<div class="summary-number">%d</div>
				<div class="summary-label">失败</div>
			</div>
			<div class="summary-item">
				<div class="summary-number">%d</div>
				<div class="summary-label">未执行</div>
			</div>
		</div>
	</div>
	
	<h2>测试用例结果详情</h2>
	<table>
		<thead>
			<tr>
				<th>用例名称</th>
				<th>测试环境</th>
				<th>设备</th>
				<th>执行状态</th>
				<th>最后执行时间</th>
			</tr>
		</thead>
		<tbody>
			%s
		</tbody>
	</table>
	
	<div class="footer">
		<p>此报告由 WebTestFlow 自动生成 | %s</p>
		<p>报告基于各测试用例的最新执行结果汇总而成</p>
	</div>
</body>
</html>`,
		testSuite.Name,
		testSuite.Project.Name,
		testSuite.Name,
		time.Now().Format("2006-01-02 15:04:05"),
		testSuite.Name,
		testSuite.Project.Name,
		testSuite.Description,
		passRate,
		execution.TotalCount,
		execution.PassedCount,
		execution.FailedCount,
		execution.TotalCount-execution.PassedCount-execution.FailedCount,
		resultsHTML,
		baseURL,
	)

	return html
}

// DownloadLatestTestSuiteReportHTMLWithScreenshots exports HTML report with latest test results and screenshots
func DownloadLatestTestSuiteReportHTMLWithScreenshots(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的测试套件ID")
		return
	}

	var testSuite models.TestSuite
	err = database.DB.Preload("Project").
		Where("id = ? AND status = ?", id, 1).First(&testSuite).Error
	if err != nil {
		response.NotFound(c, "测试套件不存在")
		return
	}

	// Load test cases with relations
	err = database.DB.Model(&testSuite).Association("TestCases").Find(&testSuite.TestCases, "status = ?", 1)
	if err != nil {
		response.InternalServerError(c, "加载测试用例失败")
		return
	}

	// Get latest executions for all test cases
	var latestExecutions []models.TestExecution
	totalCount := len(testSuite.TestCases)
	passedCount := 0
	failedCount := 0

	// Use the same logic as GetLatestTestSuiteReport for consistency
	sevenDaysAgo := time.Now().AddDate(0, 0, -7)
	
	for _, testCase := range testSuite.TestCases {
		var latestValidExecution models.TestExecution
		found := false
		
		// Strategy 1: Look for the latest valid execution across ALL test suite runs for this test case
		// Include both 'test_case' and 'test_case_internal' execution types
		err := database.DB.Preload("TestCase").Preload("TestCase.Environment").Preload("TestCase.Device").
			Where("test_case_id = ? AND execution_type IN ('test_case', 'test_case_internal') AND status IN ('passed', 'failed')", testCase.ID).
			Order("start_time DESC").First(&latestValidExecution).Error
		
		if err == nil {
			found = true
		}
		
		if !found {
			// Strategy 2: Look for standalone test case executions
			err := database.DB.Preload("TestCase").Preload("TestCase.Environment").Preload("TestCase.Device").
				Where("test_case_id = ? AND parent_execution_id IS NULL AND execution_type IN ('test_case', 'test_case_internal') AND status IN ('passed', 'failed')", testCase.ID).
				Order("start_time DESC").First(&latestValidExecution).Error
			
			if err == nil {
				found = true
			}
		}
		
		if !found {
			// Strategy 3: Check if there's any execution within 7 days
			var anyRecentExecution models.TestExecution
			recentErr := database.DB.Preload("TestCase").Preload("TestCase.Environment").Preload("TestCase.Device").
				Where("test_case_id = ? AND execution_type IN ('test_case', 'test_case_internal') AND start_time > ?", testCase.ID, sevenDaysAgo).
				Order("start_time DESC").First(&anyRecentExecution).Error
			
			if recentErr != nil {
				// No execution within 7 days, mark as not_executed
				latestValidExecution = models.TestExecution{
					TestCaseID:    &testCase.ID,
					TestCase:      testCase,
					ExecutionType: "test_case",
					Status:        "not_executed",
					ErrorMessage:  "该用例在过去7天内未执行",
					TotalCount:    0,
					PassedCount:   0,
					FailedCount:   0,
					Duration:      0,
					ExecutionLogs: "[]",
					Screenshots:   "[]",
					// Don't set StartTime for not_executed cases
					UserID:        1, // Default user
				}
				// Don't set EndTime for not_executed cases
			} else {
				// Found recent execution but it doesn't have passed/failed status
				// Check if there's any historical valid execution
				var historicalValidExecution models.TestExecution
				historyErr := database.DB.Preload("TestCase").Preload("TestCase.Environment").Preload("TestCase.Device").
					Where("test_case_id = ? AND execution_type IN ('test_case', 'test_case_internal') AND status IN ('passed', 'failed')", testCase.ID).
					Order("start_time DESC").First(&historicalValidExecution).Error
				
				if historyErr != nil {
					// No valid execution found in history, mark as not_executed
					latestValidExecution = models.TestExecution{
						TestCaseID:    &testCase.ID,
						TestCase:      testCase,
						ExecutionType: "test_case",
						Status:        "not_executed",
						ErrorMessage:  "该用例最近的执行未产生有效结果",
						TotalCount:    0,
						PassedCount:   0,
						FailedCount:   0,
						Duration:      0,
						ExecutionLogs: "[]",
						Screenshots:   "[]",
						// Don't set StartTime for not_executed cases
						UserID:        1, // Default user
					}
					// Don't set EndTime for not_executed cases
				} else {
					// Use the historical valid execution
					latestValidExecution = historicalValidExecution
				}
			}
		}
		
		// Count results based on valid executions only
		if latestValidExecution.Status == "passed" {
			passedCount++
		} else if latestValidExecution.Status == "failed" {
			failedCount++
		}
		
		latestExecutions = append(latestExecutions, latestValidExecution)
	}

	// Create virtual suite execution
	now := time.Now()
	virtualSuiteExecution := models.TestExecution{
		TestSuiteID:   &testSuite.ID,
		TestSuite:     testSuite,
		ExecutionType: "test_suite_latest_with_screenshots",
		Status:        "completed",
		StartTime:     now,
		EndTime:       &now,
		Duration:      0,
		TotalCount:    totalCount,
		PassedCount:   passedCount,
		FailedCount:   failedCount,
		ErrorMessage:  "",
		ExecutionLogs: "[]",
		Screenshots:   "[]",
		UserID:        1, // Default user
	}

	// Determine overall status
	if failedCount > 0 {
		virtualSuiteExecution.Status = "failed"
	} else if passedCount == totalCount {
		virtualSuiteExecution.Status = "passed"
	} else {
		virtualSuiteExecution.Status = "partial"
	}

	// Generate HTML report with screenshots
	htmlContent := generateTestSuiteLatestHTMLWithScreenshots(virtualSuiteExecution, testSuite, latestExecutions)

	// Set response headers for file download
	filename := fmt.Sprintf("%s_测试报告带截图_%s.html", testSuite.Name, now.Format("20060102150405"))
	c.Header("Content-Type", "text/html; charset=utf-8")
	// 使用同时支持 filename 和 filename* 格式
	encodedFilename := url.QueryEscape(filename)
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, filename, encodedFilename))

	c.String(http.StatusOK, htmlContent)
}

// DownloadLatestTestSuiteReportPDFWithScreenshots exports PDF report with latest test results and screenshots
func DownloadLatestTestSuiteReportPDFWithScreenshots(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的测试套件ID")
		return
	}

	var testSuite models.TestSuite
	err = database.DB.Preload("Project").
		Where("id = ? AND status = ?", id, 1).First(&testSuite).Error
	if err != nil {
		response.NotFound(c, "测试套件不存在")
		return
	}

	// Load test cases
	err = database.DB.Model(&testSuite).Association("TestCases").Find(&testSuite.TestCases, "status = ?", 1)
	if err != nil {
		response.InternalServerError(c, "加载测试用例失败")
		return
	}

	// Get latest executions
	var latestExecutions []models.TestExecution
	totalCount := len(testSuite.TestCases)
	passedCount := 0
	failedCount := 0

	// Use the same logic as GetLatestTestSuiteReport for consistency
	sevenDaysAgo := time.Now().AddDate(0, 0, -7)
	
	for _, testCase := range testSuite.TestCases {
		var latestValidExecution models.TestExecution
		found := false
		
		// Strategy 1: Look for the latest valid execution across ALL test suite runs for this test case
		// Include both 'test_case' and 'test_case_internal' execution types
		err := database.DB.Preload("TestCase").Preload("TestCase.Environment").Preload("TestCase.Device").
			Where("test_case_id = ? AND execution_type IN ('test_case', 'test_case_internal') AND status IN ('passed', 'failed')", testCase.ID).
			Order("start_time DESC").First(&latestValidExecution).Error
		
		if err == nil {
			found = true
		}
		
		if !found {
			// Strategy 2: Look for standalone test case executions
			err := database.DB.Preload("TestCase").Preload("TestCase.Environment").Preload("TestCase.Device").
				Where("test_case_id = ? AND parent_execution_id IS NULL AND execution_type IN ('test_case', 'test_case_internal') AND status IN ('passed', 'failed')", testCase.ID).
				Order("start_time DESC").First(&latestValidExecution).Error
			
			if err == nil {
				found = true
			}
		}
		
		if !found {
			// Strategy 3: Check if there's any execution within 7 days
			var anyRecentExecution models.TestExecution
			recentErr := database.DB.Preload("TestCase").Preload("TestCase.Environment").Preload("TestCase.Device").
				Where("test_case_id = ? AND execution_type IN ('test_case', 'test_case_internal') AND start_time > ?", testCase.ID, sevenDaysAgo).
				Order("start_time DESC").First(&anyRecentExecution).Error
			
			if recentErr != nil {
				// No execution within 7 days, mark as not_executed
				latestValidExecution = models.TestExecution{
					TestCaseID:    &testCase.ID,
					TestCase:      testCase,
					ExecutionType: "test_case",
					Status:        "not_executed",
					ErrorMessage:  "该用例在过去7天内未执行",
					Duration:      0,
					// Don't set StartTime for not_executed cases
					UserID:        1, // Default user
				}
				// Don't set EndTime for not_executed cases
			} else {
				// Found recent execution but it doesn't have passed/failed status
				// Check if there's any historical valid execution
				var historicalValidExecution models.TestExecution
				historyErr := database.DB.Preload("TestCase").Preload("TestCase.Environment").Preload("TestCase.Device").
					Where("test_case_id = ? AND execution_type IN ('test_case', 'test_case_internal') AND status IN ('passed', 'failed')", testCase.ID).
					Order("start_time DESC").First(&historicalValidExecution).Error
				
				if historyErr != nil {
					// No valid execution found in history, mark as not_executed
					latestValidExecution = models.TestExecution{
						TestCaseID:    &testCase.ID,
						TestCase:      testCase,
						ExecutionType: "test_case",
						Status:        "not_executed",
						ErrorMessage:  "该用例最近的执行未产生有效结果",
						Duration:      0,
						// Don't set StartTime for not_executed cases
						UserID:        1, // Default user
					}
					// Don't set EndTime for not_executed cases
				} else {
					// Use the historical valid execution
					latestValidExecution = historicalValidExecution
				}
			}
		}
		
		// Count results based on valid executions only
		if latestValidExecution.Status == "passed" {
			passedCount++
		} else if latestValidExecution.Status == "failed" {
			failedCount++
		}
		
		latestExecutions = append(latestExecutions, latestValidExecution)
	}

	// Create virtual suite execution
	now := time.Now()
	virtualSuiteExecution := models.TestExecution{
		TestSuiteID:   &testSuite.ID,
		TestSuite:     testSuite,
		ExecutionType: "test_suite_latest_with_screenshots",
		Status:        "completed",
		StartTime:     now,
		EndTime:       &now,
		Duration:      0,
		TotalCount:    totalCount,
		PassedCount:   passedCount,
		FailedCount:   failedCount,
		ErrorMessage:  "",
		ExecutionLogs: "[]",
		Screenshots:   "[]",
		UserID:        1, // Default user
	}

	// Determine overall status
	if failedCount > 0 {
		virtualSuiteExecution.Status = "failed"
	} else if passedCount == totalCount {
		virtualSuiteExecution.Status = "passed"
	} else {
		virtualSuiteExecution.Status = "partial"
	}

	// Generate HTML report with optimized screenshots
	htmlContent := generateTestSuiteLatestHTMLWithScreenshots(virtualSuiteExecution, testSuite, latestExecutions)

	// Convert to PDF using Chrome with optimized content
	pdfData, err := generatePDFFromHTML(htmlContent)
	if err != nil {
		response.InternalServerError(c, "生成PDF报告失败: "+err.Error())
		return
	}

	// Set response headers for PDF download
	filename := fmt.Sprintf("%s_测试报告带截图_%s.pdf", testSuite.Name, now.Format("20060102150405"))
	c.Header("Content-Type", "application/pdf")
	// 使用同时支持 filename 和 filename* 格式
	encodedFilename := url.QueryEscape(filename)
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, filename, encodedFilename))
	c.Header("Content-Length", strconv.Itoa(len(pdfData)))

	c.Data(http.StatusOK, "application/pdf", pdfData)
}

// generateTestSuiteLatestHTMLWithScreenshots generates HTML report with same styling as standard reports
func generateTestSuiteLatestHTMLWithScreenshots(execution models.TestExecution, testSuite models.TestSuite, latestExecutions []models.TestExecution) string {
	// Get server base URL
	baseURL := getServerBaseURL()
	
	// Generate test case results with card-style layout (same as standard reports)
	var resultsHTML string
	for i, exec := range latestExecutions {
		statusClass := exec.Status
		statusText := exec.Status
		switch exec.Status {
		case "passed":
			statusText = "通过"
		case "failed":
			statusText = "失败"
		case "not_executed":
			statusText = "未执行"
		}
		
		environmentName := ""
		deviceName := ""
		if exec.TestCase.Environment.Name != "" {
			environmentName = exec.TestCase.Environment.Name
		}
		if exec.TestCase.Device.Name != "" {
			deviceName = fmt.Sprintf("%s (%dx%d)", exec.TestCase.Device.Name, exec.TestCase.Device.Width, exec.TestCase.Device.Height)
		}
		
		// Calculate duration
		duration := ""
		if exec.EndTime != nil && !exec.EndTime.IsZero() {
			duration = exec.EndTime.Sub(exec.StartTime).String()
		}
		
		// Generate test case HTML using card layout (same style as standard reports)
		resultsHTML += fmt.Sprintf(`<div class="testcase-item %s">
                <div class="testcase-title">测试用例 %d: %s</div>
                <div class="testcase-info">
                    <span>执行时间: %s - %s</span>
                    <span class="testcase-status %s">%s</span>
                </div>
                <div class="testcase-info">
                    <span>持续时间: %s | 环境: %s | 设备: %s</span>
                </div>`, 
			statusClass, i+1, exec.TestCase.Name,
			exec.StartTime.Format("15:04:05"),
			func() string {
				if exec.EndTime != nil && !exec.EndTime.IsZero() {
					return exec.EndTime.Format("15:04:05")
				}
				return "未结束"
			}(),
			statusClass, statusText, duration, environmentName, deviceName)
		
		// Parse screenshots from this test case execution and add to this test case
		screenshotPaths := parseScreenshotPaths(exec.Screenshots)
		if len(screenshotPaths) > 0 {
			// Show all screenshots for HTML report
			
			resultsHTML += `<div class="testcase-screenshots">
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
				
				resultsHTML += fmt.Sprintf(`
                        <div class="screenshot-item">
                            <div class="screenshot-title">截图 %d</div>
                            <img src="%s/api/v1/screenshots/%s" alt="%s" loading="lazy" style="max-width: 200px; height: auto;" onerror="this.style.display='none'"/>
                            <div class="screenshot-description">%s<br>%s</div>
                        </div>`,
					j+1, baseURL, encodedPath, typeDesc, typeDesc, stepInfo)
			}
			
			resultsHTML += `</div></div>`
		}
		
		resultsHTML += `</div>` // Close testcase-item
	}

	// Calculate overall pass rate
	passRate := float64(0)
	if execution.TotalCount > 0 {
		passRate = float64(execution.PassedCount) / float64(execution.TotalCount) * 100
	}

	// Generate HTML report with same styling as standard reports
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>测试套件最新结果报告（带截图） - %s</title>
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
        .testcase-item {
            margin: 15px 0;
            padding: 15px;
            border: 1px solid #ddd;
            border-radius: 8px;
            background: #f9f9f9;
            page-break-inside: avoid;
            page-break-after: auto;
        }
        .testcase-item.passed {
            border-left: 5px solid #4CAF50;
        }
        .testcase-item.failed {
            border-left: 5px solid #f44336;
        }
        .testcase-item.not_executed {
            border-left: 5px solid #9e9e9e;
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
        .testcase-status.not_executed {
            color: #9e9e9e;
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
        .screenshots-grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
            gap: 20px;
            margin: 15px 0;
        }
        .screenshot-item {
            text-align: center;
            border: 1px solid #ddd;
            border-radius: 6px;
            padding: 10px;
            background: #f8f9fa;
            box-shadow: 0 1px 3px rgba(0,0,0,0.1);
            page-break-inside: avoid;
        }
        .screenshot-item img {
            max-width: 200px;
            height: auto;
            border-radius: 4px;
            box-shadow: 0 1px 4px rgba(0,0,0,0.2);
            cursor: pointer;
            transition: transform 0.2s;
        }
        .screenshot-item img:hover {
            transform: scale(1.02);
        }
        .screenshot-note {
            color: #666;
            font-size: 12px;
            font-style: italic;
            padding: 10px;
            background: #f0f0f0;
            border-radius: 4px;
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
        .footer {
            text-align: center;
            margin-top: 40px;
            padding: 20px;
            background: white;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            color: #666;
        }
    </style>
</head>
<body>
    <div class="header">
        <div class="title">测试套件最新结果报告（带截图）</div>
        <div class="subtitle">测试名称: %s</div>
        <div class="subtitle">项目名称: %s</div>
        <div class="subtitle">生成时间: %s</div>
    </div>

    <div class="section">
        <div class="section-title">1. 基本信息</div>
        <table class="info-table">
            <tr><th>项目名称</th><td>%s</td></tr>
            <tr><th>套件名称</th><td>%s</td></tr>
            <tr><th>套件描述</th><td>%s</td></tr>
            <tr><th>执行类型</th><td>最新结果汇总（含截图）</td></tr>
            <tr><th>数据来源</th><td>各用例最新执行</td></tr>
            <tr><th>报告生成时间</th><td>%s</td></tr>
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

    <div class="section">
        <div class="section-title">4. 环境配置</div>
        <table class="info-table">
            <tr><th>浏览器</th><td>Google Chrome (可视化模式)</td></tr>
            <tr><th>操作系统</th><td>Linux</td></tr>
            <tr><th>平台</th><td>WebTestFlow 自动化测试平台</td></tr>
            <tr><th>报告类型</th><td>测试套件最新结果报告（带截图）</td></tr>
        </table>
    </div>

    <div class="footer">
        <p>本报告由 WebTestFlow 自动化测试平台生成 | 生成时间: %s</p>
    </div>
</body>
</html>`,
		testSuite.Name,
		testSuite.Name, testSuite.Project.Name, time.Now().Format("2006-01-02 15:04:05"),
		testSuite.Project.Name,
		testSuite.Name,
		testSuite.Description,
		time.Now().Format("2006-01-02 15:04:05"),
		execution.TotalCount, execution.PassedCount, execution.FailedCount, passRate,
		resultsHTML,
		time.Now().Format("2006-01-02 15:04:05"))

	return html
}


// optimizeScreenshotsForPDF reduces the number of screenshots for PDF generation
// Prioritizes initial, final, and error screenshots, limits step screenshots
func optimizeScreenshotsForPDF(screenshotPaths []string) []string {
	if len(screenshotPaths) == 0 {
		return screenshotPaths
	}

	var optimizedPaths []string
	var initialShots, stepShots, finalShots, errorShots []string
	
	// Categorize screenshots by type
	for _, path := range screenshotPaths {
		filename := path
		if idx := strings.LastIndex(path, "/"); idx >= 0 {
			filename = path[idx+1:]
		}
		
		if strings.Contains(filename, "_initial_") {
			initialShots = append(initialShots, path)
		} else if strings.Contains(filename, "_final_") {
			finalShots = append(finalShots, path)
		} else if strings.Contains(filename, "_error_") {
			errorShots = append(errorShots, path)
		} else if strings.Contains(filename, "_step_") {
			stepShots = append(stepShots, path)
		}
	}
	
	// Always include initial screenshots (first one)
	if len(initialShots) > 0 {
		optimizedPaths = append(optimizedPaths, initialShots[0])
	}
	
	// Include limited step screenshots (max 3, evenly distributed)
	if len(stepShots) > 0 {
		maxSteps := 3
		if len(stepShots) <= maxSteps {
			optimizedPaths = append(optimizedPaths, stepShots...)
		} else {
			// Select evenly distributed steps
			step := len(stepShots) / maxSteps
			for i := 0; i < maxSteps; i++ {
				idx := i * step
				if idx < len(stepShots) {
					optimizedPaths = append(optimizedPaths, stepShots[idx])
				}
			}
		}
	}
	
	// Always include error screenshots (all of them as they're important)
	optimizedPaths = append(optimizedPaths, errorShots...)
	
	// Always include final screenshots (first one)
	if len(finalShots) > 0 {
		optimizedPaths = append(optimizedPaths, finalShots[0])
	}
	
	// Cap the total number to maximum 8 screenshots per test case
	maxTotal := 8
	if len(optimizedPaths) > maxTotal {
		optimizedPaths = optimizedPaths[:maxTotal]
	}
	
	return optimizedPaths
}
