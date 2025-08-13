package handlers

import (
	"webtestflow/backend/internal/models"
	"webtestflow/backend/pkg/database"
	"webtestflow/backend/pkg/response"
	"webtestflow/backend/pkg/utils"
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func GetReports(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	projectID := c.Query("project_id")

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 10
	}

	var reports []models.TestReport
	var total int64

	query := database.DB.Model(&models.TestReport{})
	if projectID != "" {
		query = query.Where("project_id = ?", projectID)
	}

	// Count total
	query.Count(&total)

	// Get paginated reports with relations
	offset := (page - 1) * pageSize
	err := query.Preload("Project").Preload("TestSuite").Preload("User").
		Order("created_at DESC").
		Offset(offset).Limit(pageSize).Find(&reports).Error
	if err != nil {
		response.InternalServerError(c, "获取测试报告失败")
		return
	}

	// Clear user passwords
	for i := range reports {
		reports[i].User.Password = ""
	}

	response.Page(c, reports, total, page, pageSize)
}

func GetReport(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的报告ID")
		return
	}

	var report models.TestReport
	err = database.DB.Preload("Project").Preload("TestSuite").Preload("User").
		Preload("Executions").Preload("Executions.TestCase").
		First(&report, id).Error
	if err != nil {
		response.NotFound(c, "测试报告不存在")
		return
	}

	report.User.Password = ""
	response.Success(c, report)
}

func CreateReport(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var req struct {
		Name         string `json:"name" binding:"required,min=1,max=200"`
		ProjectID    uint   `json:"project_id" binding:"required"`
		TestSuiteID  *uint  `json:"test_suite_id"`
		ExecutionIDs []uint `json:"execution_ids" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Verify project exists and user has permission
	if !utils.HasPermissionOnProject(userID.(uint), req.ProjectID) {
		response.NotFound(c, "项目不存在或无权限")
		return
	}

	var project models.Project
	err := database.DB.Where("id = ? AND status = ?", req.ProjectID, 1).
		First(&project).Error
	if err != nil {
		response.NotFound(c, "项目不存在")
		return
	}

	// Verify test suite if provided
	if req.TestSuiteID != nil {
		var testSuite models.TestSuite
		err := database.DB.Where("id = ? AND project_id = ? AND status = ?", 
			*req.TestSuiteID, req.ProjectID, 1).First(&testSuite).Error
		if err != nil {
			response.NotFound(c, "测试套件不存在或不属于该项目")
			return
		}
	}

	// Verify executions exist and calculate statistics
	var executions []models.TestExecution
	err = database.DB.Where("id IN ?", req.ExecutionIDs).Find(&executions).Error
	if err != nil || len(executions) != len(req.ExecutionIDs) {
		response.BadRequest(c, "部分执行记录不存在")
		return
	}

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

	// Determine status
	status := "completed"
	for _, execution := range executions {
		if execution.Status == "running" || execution.Status == "pending" {
			status = "running"
			break
		}
	}

	// Create report
	report := models.TestReport{
		Name:        req.Name,
		ProjectID:   req.ProjectID,
		TestSuiteID: req.TestSuiteID,
		TotalCases:  totalCases,
		PassedCases: passedCases,
		FailedCases: failedCases,
		ErrorCases:  errorCases,
		StartTime:   minStartTime,
		EndTime:     maxEndTime,
		Duration:    reportDuration,
		Status:      status,
		UserID:      userID.(uint),
	}

	err = database.DB.Create(&report).Error
	if err != nil {
		response.InternalServerError(c, "创建测试报告失败")
		return
	}

	// Associate executions with report
	err = database.DB.Model(&report).Association("Executions").Replace(executions)
	if err != nil {
		response.InternalServerError(c, "关联执行记录失败")
		return
	}

	// Load relations for response
	database.DB.Preload("Project").Preload("TestSuite").Preload("User").
		Preload("Executions").First(&report, report.ID)
	report.User.Password = ""

	response.SuccessWithMessage(c, "创建成功", report)
}

func ExportReport(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的报告ID")
		return
	}

	format := c.DefaultQuery("format", "html")
	if format != "html" && format != "json" {
		response.BadRequest(c, "不支持的导出格式，仅支持 html 或 json")
		return
	}

	// Get report with all relations
	var report models.TestReport
	err = database.DB.Preload("Project").Preload("TestSuite").Preload("User").
		Preload("Executions").Preload("Executions.TestCase").
		Preload("Executions.TestCase.Project").Preload("Executions.TestCase.Environment").
		First(&report, id).Error
	if err != nil {
		response.NotFound(c, "测试报告不存在")
		return
	}

	if format == "json" {
		// Export as JSON
		report.User.Password = ""
		for i := range report.Executions {
			report.Executions[i].User.Password = ""
		}
		
		filename := fmt.Sprintf("test-report-%d-%s.json", report.ID, time.Now().Format("20060102-150405"))
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
		c.Header("Content-Type", "application/json")
		c.JSON(200, report)
		return
	}

	// Export as HTML
	htmlContent := generateHTMLReport(report)
	filename := fmt.Sprintf("test-report-%d-%s.html", report.ID, time.Now().Format("20060102-150405"))
	
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(200, htmlContent)
}

func generateHTMLReport(report models.TestReport) string {
	html := fmt.Sprintf(`
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; background-color: #f5f5f5; }
        .container { max-width: 1200px; margin: 0 auto; background: white; padding: 30px; border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        .header { text-align: center; margin-bottom: 30px; border-bottom: 2px solid #1890ff; padding-bottom: 20px; }
        .title { color: #1890ff; margin-bottom: 10px; }
        .subtitle { color: #666; margin: 5px 0; }
        .stats { display: flex; justify-content: space-around; margin: 30px 0; }
        .stat-item { text-align: center; padding: 20px; background: #f8f9fa; border-radius: 6px; min-width: 120px; }
        .stat-number { font-size: 2em; font-weight: bold; margin-bottom: 5px; }
        .stat-label { color: #666; font-size: 0.9em; }
        .passed { color: #52c41a; }
        .failed { color: #ff4d4f; }
        .total { color: #1890ff; }
        .section { margin: 30px 0; }
        .section-title { color: #1890ff; border-bottom: 1px solid #e8e8e8; padding-bottom: 10px; margin-bottom: 20px; }
        .execution-item { margin: 15px 0; padding: 15px; border: 1px solid #e8e8e8; border-radius: 6px; }
        .execution-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px; }
        .execution-name { font-weight: bold; }
        .status { padding: 4px 8px; border-radius: 4px; color: white; font-size: 0.85em; }
        .status.passed { background-color: #52c41a; }
        .status.failed { background-color: #ff4d4f; }
        .status.running { background-color: #1890ff; }
        .status.pending { background-color: #fa8c16; }
        .info-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 15px; margin-top: 15px; }
        .info-item { background: #f8f9fa; padding: 10px; border-radius: 4px; }
        .info-label { font-weight: bold; color: #666; font-size: 0.85em; }
        .info-value { margin-top: 5px; }
        .footer { margin-top: 40px; text-align: center; color: #999; font-size: 0.9em; border-top: 1px solid #e8e8e8; padding-top: 20px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1 class="title">%s</h1>
            <div class="subtitle">项目: %s</div>
            <div class="subtitle">生成时间: %s</div>
            <div class="subtitle">执行时间: %s - %s</div>
        </div>

        <div class="stats">
            <div class="stat-item">
                <div class="stat-number total">%d</div>
                <div class="stat-label">总用例数</div>
            </div>
            <div class="stat-item">
                <div class="stat-number passed">%d</div>
                <div class="stat-label">通过</div>
            </div>
            <div class="stat-item">
                <div class="stat-number failed">%d</div>
                <div class="stat-label">失败</div>
            </div>
            <div class="stat-item">
                <div class="stat-number">%.1f%%</div>
                <div class="stat-label">成功率</div>
            </div>
        </div>

        <div class="section">
            <h2 class="section-title">执行详情</h2>
            %s
        </div>

        <div class="footer">
            <p>此报告由 WebTestFlow 自动生成</p>
        </div>
    </div>
</body>
</html>`,
		report.Name,
		report.Name,
		report.Project.Name,
		time.Now().Format("2006-01-02 15:04:05"),
		report.StartTime.Format("2006-01-02 15:04:05"),
		report.EndTime.Format("2006-01-02 15:04:05"),
		report.TotalCases,
		report.PassedCases,
		report.FailedCases,
		float64(report.PassedCases)/float64(report.TotalCases)*100,
		generateExecutionDetails(report.Executions))

	return html
}

func generateExecutionDetails(executions []models.TestExecution) string {
	var executionsHTML string
	
	for _, execution := range executions {
		statusClass := execution.Status
		statusText := map[string]string{
			"passed":    "通过",
			"failed":    "失败",
			"running":   "运行中",
			"pending":   "等待中",
			"cancelled": "已取消",
		}[execution.Status]

		testName := "未知测试"
		if execution.TestCase.Name != "" {
			testName = execution.TestCase.Name
		} else if execution.TestSuite.Name != "" {
			testName = execution.TestSuite.Name + " (套件)"
		}

		duration := fmt.Sprintf("%.1f秒", float64(execution.Duration)/1000.0)
		
		executionsHTML += fmt.Sprintf(`
			<div class="execution-item">
				<div class="execution-header">
					<span class="execution-name">%s</span>
					<span class="status %s">%s</span>
				</div>
				<div class="info-grid">
					<div class="info-item">
						<div class="info-label">执行时长</div>
						<div class="info-value">%s</div>
					</div>
					<div class="info-item">
						<div class="info-label">开始时间</div>
						<div class="info-value">%s</div>
					</div>
					<div class="info-item">
						<div class="info-label">结束时间</div>
						<div class="info-value">%s</div>
					</div>
					<div class="info-item">
						<div class="info-label">执行环境</div>
						<div class="info-value">%s</div>
					</div>
				</div>
				%s
			</div>`,
			testName,
			statusClass,
			statusText,
			duration,
			execution.StartTime.Format("2006-01-02 15:04:05"),
			func() string {
				if execution.EndTime != nil {
					return execution.EndTime.Format("2006-01-02 15:04:05")
				}
				return "未结束"
			}(),
			func() string {
				if execution.TestCase.Environment.Name != "" {
					return execution.TestCase.Environment.Name
				}
				return "未知"
			}(),
			func() string {
				if execution.ErrorMessage != "" {
					return fmt.Sprintf(`<div style="margin-top: 10px; padding: 10px; background: #fff2f0; border: 1px solid #ffccc7; border-radius: 4px;"><strong>错误信息:</strong> %s</div>`, execution.ErrorMessage)
				}
				return ""
			}())
	}
	
	return executionsHTML
}

func DeleteReport(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的报告ID")
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var report models.TestReport
	err = database.DB.Where("id = ? AND user_id = ?", id, userID).First(&report).Error
	if err != nil {
		response.NotFound(c, "测试报告不存在或无权限")
		return
	}

	// Remove execution associations first
	err = database.DB.Model(&report).Association("Executions").Clear()
	if err != nil {
		response.InternalServerError(c, "清除执行记录关联失败")
		return
	}

	// Delete report
	err = database.DB.Delete(&report).Error
	if err != nil {
		response.InternalServerError(c, "删除测试报告失败")
		return
	}

	response.SuccessWithMessage(c, "删除成功", nil)
}