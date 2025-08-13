import React, { useEffect, useState } from 'react';
import {
  Card,
  Table,
  Button,
  Space,
  message,
  Tag,
  Typography,
  Descriptions,
  Drawer,
  Row,
  Col,
  Statistic,
  Progress,
  Timeline,
  Image,
  List,
  Badge,
  DatePicker,
  Select,
  Empty,
} from 'antd';
import {
  EyeOutlined,
  ReloadOutlined,
  DownloadOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  ClockCircleOutlined,
} from '@ant-design/icons';
import dayjs from 'dayjs';
import { api } from '../../services/api';
import type { TestExecution, TestReport, Project, Environment } from '../../types';
import type { ColumnsType } from 'antd/es/table';

const { Title, Text } = Typography;
const { RangePicker } = DatePicker;
const { Option } = Select;

const Reports: React.FC = () => {
  const [executions, setExecutions] = useState<TestExecution[]>([]);
  // const [reports, setReports] = useState<TestReport[]>([]);
  const [projects, setProjects] = useState<Project[]>([]);
  const [environments, setEnvironments] = useState<Environment[]>([]);
  const [statistics, setStatistics] = useState({
    total_executions: 0,
    passed_count: 0,
    failed_count: 0,
    running_count: 0,
    pending_count: 0,
    success_rate: 0,
    avg_duration: 0,
  });
  const [loading, setLoading] = useState(false);
  const [isDetailDrawerVisible, setIsDetailDrawerVisible] = useState(false);
  const [selectedExecution, setSelectedExecution] = useState<TestExecution | null>(null);
  const [executionLogs, setExecutionLogs] = useState<any[]>([]);
  const [executionScreenshots, setExecutionScreenshots] = useState<any[]>([]);
  const [showTestSuiteDetails, setShowTestSuiteDetails] = useState(false);
  const [suiteTestCases, setSuiteTestCases] = useState<TestExecution[]>([]);
  const [selectedTestCaseFromSuite, setSelectedTestCaseFromSuite] = useState<TestExecution | null>(null);
  const [filters, setFilters] = useState<{
    project_id?: number;
    environment_id?: number;
    status?: string;
    date_range?: any;
  }>({
    project_id: undefined,
    environment_id: undefined,
    status: undefined,
    date_range: null,
  });
  const [pagination, setPagination] = useState({
    current: 1,
    pageSize: 10,
    total: 0,
  });

  useEffect(() => {
    loadExecutions();
    loadStatistics();
    // loadReports();
    loadInitialData();
  }, [pagination.current, pagination.pageSize, filters]);

  const loadInitialData = async () => {
    try {
      const [projectsData, environmentsData] = await Promise.all([
        api.getProjects({ page: 1, page_size: 100 }),
        api.getEnvironments(),
      ]);
      setProjects(projectsData.list);
      setEnvironments(environmentsData);
    } catch (error) {
      console.error('Failed to load initial data:', error);
    }
  };

  const loadStatistics = async () => {
    try {
      const params: any = {};
      if (filters.project_id) params.project_id = filters.project_id;
      if (filters.environment_id) params.environment_id = filters.environment_id;
      if (filters.status) params.status = filters.status;
      if (filters.date_range && filters.date_range.length === 2 && filters.date_range[0] && filters.date_range[1]) {
        params.start_date = dayjs(filters.date_range[0]).format('YYYY-MM-DD');
        params.end_date = dayjs(filters.date_range[1]).format('YYYY-MM-DD');
      }

      const stats = await api.getExecutionStatistics(params);
      setStatistics(stats);
    } catch (error) {
      console.error('Failed to load statistics:', error);
    }
  };

  const loadExecutions = async () => {
    setLoading(true);
    try {
      const params: any = {
        page: pagination.current,
        page_size: pagination.pageSize,
      };

      if (filters.project_id) params.project_id = filters.project_id;
      if (filters.environment_id) params.environment_id = filters.environment_id;
      if (filters.status) params.status = filters.status;
      if (filters.date_range && filters.date_range.length === 2 && filters.date_range[0] && filters.date_range[1]) {
        params.start_date = dayjs(filters.date_range[0]).format('YYYY-MM-DD');
        params.end_date = dayjs(filters.date_range[1]).format('YYYY-MM-DD');
      }

      const response = await api.getExecutions(params);
      setExecutions(response.list);
      setPagination(prev => ({
        ...prev,
        total: response.total,
      }));
    } catch (error) {
      console.error('Failed to load executions:', error);
      message.error('获取执行记录失败');
    } finally {
      setLoading(false);
    }
  };

  // const loadReports = async () => {
  //   try {
  //     const response = await api.getReports();
  //     setReports(response.list || []);
  //   } catch (error) {
  //     console.error('Failed to load reports:', error);
  //   }
  // };

  const handleViewDetails = async (execution: TestExecution) => {
    try {
      // 重置状态
      setShowTestSuiteDetails(false);
      setSuiteTestCases([]);
      setSelectedTestCaseFromSuite(null);
      
      setSelectedExecution(execution);
      
      // 如果是测试套件执行，获取当前批次的测试用例执行情况
      if (execution.execution_type === 'test_suite') {
        try {
          const suiteExecutionsResponse = await api.getCurrentBatchExecutions(execution.id);
          setSuiteTestCases(suiteExecutionsResponse.list || []);
          setShowTestSuiteDetails(true);
          setIsDetailDrawerVisible(true);
          return;
        } catch (error) {
          console.error('Failed to load test suite executions:', error);
          const errorMessage = error instanceof Error ? error.message : String(error);
          message.error('获取测试套件执行详情失败: ' + errorMessage);
          return;
        }
      }
      
      // 单个测试用例执行，获取日志和截图
      const [logsResponse, screenshotsResponse] = await Promise.all([
        api.getExecutionLogs(execution.id),
        api.getExecutionScreenshots(execution.id),
      ]);
      
      setExecutionLogs(logsResponse.logs || []);
      
      // 处理截图数据
      const processedScreenshots: any[] = [];
      
      if (screenshotsResponse.screenshots && screenshotsResponse.screenshots.length > 0) {
        screenshotsResponse.screenshots.forEach((screenshot: any) => {
          processedScreenshots.push({
            ...screenshot,
            url: `/api/v1/screenshots/${screenshot.file_name}`,
            description: `${screenshot.type === 'error' ? '错误' : screenshot.type === 'before' ? '执行前' : '执行后'}截图`,
            timestamp: screenshot.created_at
          });
        });
      }
      
      if (screenshotsResponse.execution_screenshots && Array.isArray(screenshotsResponse.execution_screenshots) && screenshotsResponse.execution_screenshots.length > 0) {
        screenshotsResponse.execution_screenshots.forEach((filename: string, index: number) => {
          processedScreenshots.push({
            id: `exec_${index}`,
            step_index: index,
            file_name: filename,
            url: `/api/v1/screenshots/${filename}`,
            description: `步骤截图`,
            timestamp: execution.start_time,
            type: 'step'
          });
        });
      }
      
      setExecutionScreenshots(processedScreenshots);
      setIsDetailDrawerVisible(true);
    } catch (error) {
      console.error('Failed to load execution details:', error);
      message.error('获取执行详情失败');
    }
  };

  const handleViewTestCaseDetails = async (testCaseExecution: TestExecution) => {
    try {
      const [logsResponse, screenshotsResponse] = await Promise.all([
        api.getExecutionLogs(testCaseExecution.id),
        api.getExecutionScreenshots(testCaseExecution.id),
      ]);
      
      setSelectedTestCaseFromSuite(testCaseExecution);
      setExecutionLogs(logsResponse.logs || []);
      
      // 处理截图数据
      const processedScreenshots: any[] = [];
      
      if (screenshotsResponse.screenshots && screenshotsResponse.screenshots.length > 0) {
        screenshotsResponse.screenshots.forEach((screenshot: any) => {
          processedScreenshots.push({
            ...screenshot,
            url: `/api/v1/screenshots/${screenshot.file_name}`,
            description: `${screenshot.type === 'error' ? '错误' : screenshot.type === 'before' ? '执行前' : '执行后'}截图`,
            timestamp: screenshot.created_at
          });
        });
      }
      
      if (screenshotsResponse.execution_screenshots && screenshotsResponse.execution_screenshots.length > 0) {
        screenshotsResponse.execution_screenshots.forEach((screenshot: string, index: number) => {
          processedScreenshots.push({
            url: `/api/v1/screenshots/${screenshot}`,
            description: `执行截图 ${index + 1}`,
            timestamp: testCaseExecution.created_at
          });
        });
      }
      
      setExecutionScreenshots(processedScreenshots);
      setShowTestSuiteDetails(false); // 切换到步骤详情视图
      
    } catch (error) {
      console.error('Failed to load test case execution details:', error);
      message.error('获取测试用例执行详情失败');
    }
  };
  
  const handleBackToSuiteDetails = () => {
    setShowTestSuiteDetails(true);
    setSelectedTestCaseFromSuite(null);
    setExecutionLogs([]);
    setExecutionScreenshots([]);
  };

  const handleDownloadReport = async (execution: TestExecution) => {
    try {
      const testName = execution.test_case?.name || execution.test_suite?.name;
      const projectName = execution.test_case?.project?.name || execution.test_suite?.project?.name;
      const environmentName = execution.test_case?.environment?.name || execution.test_suite?.environment?.name;
      
      // Validate required fields
      if (!testName || !projectName) {
        message.error('缺少必要的测试信息，无法生成报告');
        return;
      }
      
      message.loading('正在生成测试报告...', 0);
      
      // Generate HTML report content
      const htmlContent = generateHTMLReport({
        execution,
        testName,
        projectName,
        environmentName,
      });
      
      // Create blob and download
      const blob = new Blob([htmlContent], { type: 'text/html; charset=utf-8' });
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      
      // Generate filename with timestamp
      const timestamp = new Date().toISOString().slice(0, 19).replace(/[T:]/g, '-');
      link.download = `测试报告-${testName}-${timestamp}.html`;
      
      // Trigger download
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(url);
      
      message.destroy();
      message.success('测试报告已生成，请选择保存位置');
      
    } catch (error) {
      message.destroy();
      console.error('Failed to generate report:', error);
      message.error('生成报告失败');
    }
  };

  const generateHTMLReport = ({ execution, testName, projectName, environmentName }: {
    execution: TestExecution;
    testName: string;
    projectName: string;
    environmentName?: string;
  }) => {
    const startTime = new Date(execution.start_time).toLocaleString();
    const endTime = execution.end_time ? new Date(execution.end_time).toLocaleString() : '未结束';
    const duration = Math.round(execution.duration / 1000);
    const status = execution.status;
    const statusText = getStatusText(status);
    
    // Parse execution logs
    let logs: any[] = [];
    try {
      logs = JSON.parse(execution.execution_logs || '[]');
    } catch (e) {
      console.warn('Failed to parse execution logs:', e);
    }
    
    // Parse screenshots
    let screenshots: string[] = [];
    try {
      screenshots = JSON.parse(execution.screenshots || '[]');
    } catch (e) {
      console.warn('Failed to parse screenshots:', e);
    }

    const successRate = execution.total_count > 0 ? 
      Math.round((execution.passed_count / execution.total_count) * 100) : 0;

    return `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>${testName} - 测试报告</title>
    <style>
        body { font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; margin: 0; padding: 40px; background-color: #f5f7fa; }
        .container { max-width: 1200px; margin: 0 auto; background: white; padding: 40px; border-radius: 12px; box-shadow: 0 4px 20px rgba(0,0,0,0.1); }
        .header { text-align: center; margin-bottom: 40px; border-bottom: 3px solid #1890ff; padding-bottom: 30px; }
        .title { color: #1890ff; margin-bottom: 15px; font-size: 2.2em; font-weight: 600; }
        .subtitle { color: #666; margin: 8px 0; font-size: 1.1em; }
        .stats { display: flex; justify-content: space-around; margin: 40px 0; gap: 20px; }
        .stat-item { text-align: center; padding: 25px; background: linear-gradient(135deg, #f8f9fa, #e9ecef); border-radius: 10px; min-width: 140px; box-shadow: 0 2px 8px rgba(0,0,0,0.1); }
        .stat-number { font-size: 2.5em; font-weight: bold; margin-bottom: 8px; }
        .stat-label { color: #666; font-size: 1em; font-weight: 500; }
        .passed { color: #52c41a; }
        .failed { color: #ff4d4f; }
        .total { color: #1890ff; }
        .section { margin: 40px 0; }
        .section-title { color: #1890ff; border-bottom: 2px solid #e8e8e8; padding-bottom: 15px; margin-bottom: 25px; font-size: 1.5em; font-weight: 600; }
        .info-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(250px, 1fr)); gap: 20px; margin: 25px 0; }
        .info-item { background: #f8f9fa; padding: 20px; border-radius: 8px; border-left: 4px solid #1890ff; }
        .info-label { font-weight: bold; color: #666; font-size: 0.9em; margin-bottom: 8px; }
        .info-value { font-size: 1.1em; color: #333; }
        .status { padding: 8px 16px; border-radius: 20px; color: white; font-size: 0.9em; font-weight: 500; }
        .status.passed { background: linear-gradient(135deg, #52c41a, #73d13d); }
        .status.failed { background: linear-gradient(135deg, #ff4d4f, #ff7875); }
        .status.running { background: linear-gradient(135deg, #1890ff, #40a9ff); }
        .status.pending { background: linear-gradient(135deg, #fa8c16, #ffa940); }
        .logs-section { margin-top: 30px; }
        .log-item { margin: 10px 0; padding: 15px; background: #f8f9fa; border-radius: 8px; border-left: 4px solid #ddd; }
        .log-timestamp { color: #999; font-size: 0.85em; margin-bottom: 5px; }
        .log-message { color: #333; }
        .log-success { border-left-color: #52c41a; }
        .log-error { border-left-color: #ff4d4f; background: #fff2f0; }
        .log-info { border-left-color: #1890ff; }
        .footer { margin-top: 50px; text-align: center; color: #999; font-size: 0.9em; border-top: 1px solid #e8e8e8; padding-top: 30px; }
        .error-section { background: #fff2f0; border: 1px solid #ffccc7; border-radius: 8px; padding: 20px; margin: 20px 0; }
        .error-title { color: #ff4d4f; font-weight: bold; margin-bottom: 10px; }
        @media (max-width: 768px) {
            body { padding: 20px; }
            .container { padding: 20px; }
            .stats { flex-direction: column; }
            .info-grid { grid-template-columns: 1fr; }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1 class="title">${testName} - 测试报告</h1>
            <div class="subtitle">项目: ${projectName}</div>
            <div class="subtitle">生成时间: ${new Date().toLocaleString()}</div>
            <div class="subtitle">执行时间: ${startTime} - ${endTime}</div>
        </div>

        <div class="stats">
            <div class="stat-item">
                <div class="stat-number total">${execution.total_count}</div>
                <div class="stat-label">总测试数</div>
            </div>
            <div class="stat-item">
                <div class="stat-number passed">${execution.passed_count}</div>
                <div class="stat-label">通过</div>
            </div>
            <div class="stat-item">
                <div class="stat-number failed">${execution.failed_count}</div>
                <div class="stat-label">失败</div>
            </div>
            <div class="stat-item">
                <div class="stat-number">${successRate}%</div>
                <div class="stat-label">成功率</div>
            </div>
        </div>

        <div class="section">
            <h2 class="section-title">执行详情</h2>
            <div class="info-grid">
                <div class="info-item">
                    <div class="info-label">执行状态</div>
                    <div class="info-value">
                        <span class="status ${status}">${statusText}</span>
                    </div>
                </div>
                <div class="info-item">
                    <div class="info-label">执行时长</div>
                    <div class="info-value">${duration} 秒</div>
                </div>
                <div class="info-item">
                    <div class="info-label">测试环境</div>
                    <div class="info-value">${environmentName || '未知'}</div>
                </div>
                <div class="info-item">
                    <div class="info-label">执行类型</div>
                    <div class="info-value">${execution.execution_type === 'test_case' ? '单个测试用例' : '测试套件'}</div>
                </div>
            </div>
        </div>

        ${execution.error_message ? `
        <div class="error-section">
            <div class="error-title">错误信息</div>
            <div>${execution.error_message}</div>
        </div>
        ` : ''}

        ${logs.length > 0 ? `
        <div class="section">
            <h2 class="section-title">执行日志 (${logs.length} 条)</h2>
            <div class="logs-section">
                ${logs.map((log: any) => `
                    <div class="log-item ${log.level === 'error' ? 'log-error' : log.step_status === 'success' ? 'log-success' : 'log-info'}">
                        <div class="log-timestamp">${new Date(log.timestamp).toLocaleString()}</div>
                        <div class="log-message">${log.message}</div>
                        ${log.duration ? `<div style="color: #999; font-size: 0.8em; margin-top: 5px;">耗时: ${log.duration}ms</div>` : ''}
                    </div>
                `).join('')}
            </div>
        </div>
        ` : ''}

        ${screenshots.length > 0 ? `
        <div class="section">
            <h2 class="section-title">截图记录 (${screenshots.length} 张)</h2>
            <div style="color: #666; font-style: italic;">
                注意: 截图文件需要从服务器单独下载查看
            </div>
            <div style="margin-top: 15px;">
                ${screenshots.map((screenshot: string, index: number) => `
                    <div style="margin: 5px 0; padding: 8px; background: #f0f0f0; border-radius: 4px;">
                        ${index + 1}. ${screenshot}
                    </div>
                `).join('')}
            </div>
        </div>
        ` : ''}

        <div class="footer">
            <p>此报告由 WebTestFlow 自动生成 | 生成时间: ${new Date().toLocaleString()}</p>
        </div>
    </div>
</body>
</html>`;
  };

  const getStatusColor = (status: string) => {
    const colors: Record<string, string> = {
      passed: 'green',
      failed: 'red',
      running: 'blue',
      pending: 'orange',
      cancelled: 'gray',
    };
    return colors[status] || 'default';
  };

  const getStatusText = (status: string) => {
    const texts: Record<string, string> = {
      passed: '通过',
      failed: '失败',
      running: '运行中',
      pending: '等待中',
      cancelled: '已取消',
    };
    return texts[status] || status;
  };

  const getStatusIcon = (status: string) => {
    const icons: Record<string, React.ReactNode> = {
      passed: <CheckCircleOutlined style={{ color: '#52c41a' }} />,
      failed: <CloseCircleOutlined style={{ color: '#ff4d4f' }} />,
      running: <ClockCircleOutlined style={{ color: '#1890ff' }} />,
      pending: <ClockCircleOutlined style={{ color: '#fa8c16' }} />,
    };
    return icons[status] || <ClockCircleOutlined />;
  };


  const columns: ColumnsType<TestExecution> = [
    {
      title: '执行ID',
      dataIndex: 'id',
      key: 'id',
      width: 80,
    },
    {
      title: '执行类型',
      dataIndex: 'execution_type',
      key: 'execution_type',
      width: 100,
      render: (type: string) => (
        <Tag color={type === 'test_case' ? 'blue' : 'green'}>
          {type === 'test_case' ? '测试用例' : '测试套件'}
        </Tag>
      ),
    },
    {
      title: '名称',
      key: 'name',
      width: 200,
      ellipsis: true,
      render: (_, record) => (
        <div>
          <div>{record.test_case?.name || record.test_suite?.name}</div>
          <Text type="secondary" style={{ fontSize: '12px' }}>
            {record.test_case?.project?.name || record.test_suite?.project?.name}
          </Text>
        </div>
      ),
    },
    {
      title: '环境',
      key: 'environment',
      width: 100,
      render: (_, record) => (
        record.test_case?.environment?.name || record.test_suite?.environment?.name
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (status: string) => (
        <Space>
          {getStatusIcon(status)}
          <Tag color={getStatusColor(status)}>
            {getStatusText(status)}
          </Tag>
        </Space>
      ),
    },
    {
      title: '成功/总数',
      key: 'test_results',
      width: 100,
      render: (_, record) => (
        <div>
          <Text>{record.passed_count}/{record.total_count}</Text>
          <Progress
            percent={record.total_count > 0 ? Math.round((record.passed_count / record.total_count) * 100) : 0}
            size="small"
            showInfo={false}
          />
        </div>
      ),
    },
    {
      title: '执行时长',
      dataIndex: 'duration',
      key: 'duration',
      width: 100,
      render: (duration: number) => `${Math.round(duration / 1000)}s`,
    },
    {
      title: '开始时间',
      dataIndex: 'start_time',
      key: 'start_time',
      width: 150,
      render: (date: string) => new Date(date).toLocaleString(),
    },
    {
      title: '操作',
      key: 'action',
      width: 150,
      render: (_, record) => (
        <Space size="small">
          <Button
            type="link"
            size="small"
            icon={<EyeOutlined />}
            onClick={() => handleViewDetails(record)}
          >
            详情
          </Button>
          <Button
            type="link"
            size="small"
            icon={<DownloadOutlined />}
            onClick={() => handleDownloadReport(record)}
            disabled={record.status !== 'passed' && record.status !== 'failed'}
          >
            报告
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Title level={2}>测试报告</Title>
      
      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={6}>
          <Card>
            <Statistic title="总执行次数" value={statistics.total_executions} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="成功率"
              value={Math.round(statistics.success_rate)}
              suffix="%"
              valueStyle={{ color: statistics.success_rate >= 80 ? '#3f8600' : '#cf1322' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="运行中"
              value={statistics.running_count}
              valueStyle={{ color: '#1890ff' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="失败次数"
              value={statistics.failed_count}
              valueStyle={{ color: '#cf1322' }}
            />
          </Card>
        </Col>
      </Row>

      <Card>
        <div style={{ marginBottom: 16 }}>
          <Space wrap>
            <Select
              placeholder="选择项目"
              style={{ width: 200 }}
              allowClear
              value={filters.project_id}
              onChange={(value) => setFilters({ ...filters, project_id: value })}
            >
              {projects.map(project => (
                <Option key={project.id} value={project.id}>
                  {project.name}
                </Option>
              ))}
            </Select>
            
            <Select
              placeholder="选择环境"
              style={{ width: 150 }}
              allowClear
              value={filters.environment_id}
              onChange={(value) => setFilters({ ...filters, environment_id: value })}
            >
              {environments.map(env => (
                <Option key={env.id} value={env.id}>
                  {env.name}
                </Option>
              ))}
            </Select>
            
            <Select
              placeholder="选择状态"
              style={{ width: 120 }}
              allowClear
              value={filters.status}
              onChange={(value) => setFilters({ ...filters, status: value })}
            >
              <Option value="passed">通过</Option>
              <Option value="failed">失败</Option>
              <Option value="running">运行中</Option>
              <Option value="pending">等待中</Option>
              <Option value="cancelled">已取消</Option>
            </Select>
            
            <RangePicker
              value={filters.date_range}
              onChange={(dates) => setFilters({ ...filters, date_range: dates })}
              placeholder={['开始日期', '结束日期']}
            />
            
            <Button icon={<ReloadOutlined />} onClick={() => { loadExecutions(); loadStatistics(); }}>
              刷新
            </Button>
          </Space>
        </div>

        <Table
          columns={columns}
          dataSource={executions}
          rowKey="id"
          loading={loading}
          pagination={{
            ...pagination,
            showSizeChanger: true,
            showQuickJumper: true,
            showTotal: (total, range) =>
              `第 ${range[0]}-${range[1]} 条，共 ${total} 条`,
            onChange: (page, pageSize) => {
              setPagination({ ...pagination, current: page, pageSize: pageSize || 10 });
            },
          }}
        />
      </Card>

      <Drawer
        title="执行详情"
        placement="right"
        size="large"
        onClose={() => {
          setIsDetailDrawerVisible(false);
          setSelectedExecution(null);
          setExecutionLogs([]);
          setExecutionScreenshots([]);
          setShowTestSuiteDetails(false);
          setSuiteTestCases([]);
          setSelectedTestCaseFromSuite(null);
        }}
        open={isDetailDrawerVisible}
      >
        {selectedExecution && (
          <div>
            {/* 测试套件详情 - 显示用例列表 */}
            {showTestSuiteDetails && (
              <div>
                <Descriptions title="测试套件基本信息" bordered column={1} size="small">
                  <Descriptions.Item label="套件名称">
                    {selectedExecution.test_suite?.name}
                  </Descriptions.Item>
                  <Descriptions.Item label="所属项目">
                    {selectedExecution.test_suite?.project?.name}
                  </Descriptions.Item>
                  <Descriptions.Item label="测试环境">
                    {selectedExecution.test_suite?.environment?.name}
                  </Descriptions.Item>
                  <Descriptions.Item label="执行状态">
                    <Space>
                      {getStatusIcon(selectedExecution.status)}
                      <Tag color={getStatusColor(selectedExecution.status)}>
                        {getStatusText(selectedExecution.status)}
                      </Tag>
                    </Space>
                  </Descriptions.Item>
                  <Descriptions.Item label="执行时长">
                    {Math.round(selectedExecution.duration / 1000)} 秒
                  </Descriptions.Item>
                  <Descriptions.Item label="开始时间">
                    {new Date(selectedExecution.start_time).toLocaleString()}
                  </Descriptions.Item>
                </Descriptions>

                <div style={{ marginTop: 24 }}>
                  <Title level={4}>测试用例执行情况 ({suiteTestCases.length}个)</Title>
                  <Table
                    dataSource={suiteTestCases}
                    rowKey="id"
                    pagination={false}
                    size="small"
                  >
                    <Table.Column
                      title="用例名称"
                      key="name"
                      render={(_, record: TestExecution) => record.test_case?.name || '未知用例'}
                    />
                    <Table.Column
                      title="执行状态"
                      dataIndex="status"
                      key="status"
                      render={(status) => (
                        <Space>
                          {getStatusIcon(status)}
                          <Tag color={getStatusColor(status)}>
                            {getStatusText(status)}
                          </Tag>
                        </Space>
                      )}
                    />
                    <Table.Column
                      title="执行时长"
                      dataIndex="duration"
                      key="duration"
                      render={(duration) => `${Math.round(duration / 1000)}秒`}
                    />
                    <Table.Column
                      title="开始时间"
                      dataIndex="start_time"
                      key="start_time"
                      render={(time) => new Date(time).toLocaleTimeString()}
                    />
                    <Table.Column
                      title="操作"
                      key="action"
                      render={(_, record: TestExecution) => (
                        <Button
                          type="link"
                          size="small"
                          onClick={() => handleViewTestCaseDetails(record)}
                        >
                          查看步骤详情
                        </Button>
                      )}
                    />
                  </Table>
                </div>
              </div>
            )}

            {/* 单个用例详情或从套件中点击的用例详情 */}
            {!showTestSuiteDetails && (
              <div>
                {/* 如果是从测试套件点击进来的，显示返回按钮 */}
                {selectedTestCaseFromSuite && (
                  <div style={{ marginBottom: 16 }}>
                    <Button 
                      icon={<span>←</span>} 
                      onClick={handleBackToSuiteDetails}
                      size="small"
                    >
                      返回套件详情
                    </Button>
                  </div>
                )}

                <Descriptions title="基本信息" bordered column={1} size="small">
                  <Descriptions.Item label="执行ID">
                    {selectedTestCaseFromSuite?.id || selectedExecution.id}
                  </Descriptions.Item>
                  <Descriptions.Item label="执行类型">
                    <Tag color={selectedExecution.execution_type === 'test_case' ? 'blue' : 'green'}>
                      {selectedExecution.execution_type === 'test_case' ? '测试用例' : '测试套件'}
                    </Tag>
                  </Descriptions.Item>
                  <Descriptions.Item label="用例名称">
                    {selectedTestCaseFromSuite?.test_case?.name || selectedExecution.test_case?.name || selectedExecution.test_suite?.name}
                  </Descriptions.Item>
                  <Descriptions.Item label="所属项目">
                    {selectedTestCaseFromSuite?.test_case?.project?.name || selectedExecution.test_case?.project?.name || selectedExecution.test_suite?.project?.name}
                  </Descriptions.Item>
                  <Descriptions.Item label="测试环境">
                    {selectedTestCaseFromSuite?.test_case?.environment?.name || selectedExecution.test_case?.environment?.name || selectedExecution.test_suite?.environment?.name}
                  </Descriptions.Item>
                  <Descriptions.Item label="执行状态">
                    <Space>
                      {getStatusIcon(selectedTestCaseFromSuite?.status || selectedExecution.status)}
                      <Tag color={getStatusColor(selectedTestCaseFromSuite?.status || selectedExecution.status)}>
                        {getStatusText(selectedTestCaseFromSuite?.status || selectedExecution.status)}
                      </Tag>
                    </Space>
                  </Descriptions.Item>
                  <Descriptions.Item label="执行时长">
                    {Math.round((selectedTestCaseFromSuite?.duration || selectedExecution.duration) / 1000)} 秒
                  </Descriptions.Item>
                  <Descriptions.Item label="开始时间">
                    {new Date(selectedTestCaseFromSuite?.start_time || selectedExecution.start_time).toLocaleString()}
                  </Descriptions.Item>
                  <Descriptions.Item label="结束时间">
                    {(selectedTestCaseFromSuite?.end_time || selectedExecution.end_time) ? 
                      new Date(selectedTestCaseFromSuite?.end_time || selectedExecution.end_time!).toLocaleString() : '未结束'}
                  </Descriptions.Item>
                  {(selectedTestCaseFromSuite?.error_message || selectedExecution.error_message) && (
                    <Descriptions.Item label="错误信息">
                      <Text type="danger">{selectedTestCaseFromSuite?.error_message || selectedExecution.error_message}</Text>
                    </Descriptions.Item>
                  )}
                </Descriptions>
              </div>
            )}

            {executionLogs.length > 0 && (
              <div style={{ marginTop: 24 }}>
                <Title level={4}>执行日志</Title>
                <Timeline>
                  {executionLogs.map((log: any, index: number) => {
                    const getTimelineColor = () => {
                      if (log.step_status === 'failed') return 'red';
                      if (log.step_status === 'running') return 'orange';
                      if (log.step_status === 'success') return 'green';
                      return log.level === 'error' ? 'red' : log.level === 'warn' ? 'orange' : 'blue';
                    };

                    const getBadgeColor = (): 'success' | 'processing' | 'default' | 'error' | 'warning' => {
                      if (log.step_status === 'failed') return 'error';
                      if (log.step_status === 'running') return 'processing';
                      if (log.step_status === 'success') return 'success';
                      return log.level === 'error' ? 'error' : log.level === 'warn' ? 'warning' : 'default';
                    };

                    return (
                      <Timeline.Item
                        key={index}
                        color={getTimelineColor()}
                      >
                        <div>
                          <Badge
                            status={getBadgeColor()}
                            text={log.step_status ? log.step_status.toUpperCase() : log.level.toUpperCase()}
                          />
                          <Text style={{ marginLeft: 8, fontSize: '12px', color: '#999' }}>
                            {new Date(log.timestamp).toLocaleTimeString()}
                          </Text>
                          {log.duration && (
                            <Text style={{ marginLeft: 8, fontSize: '12px', color: '#999' }}>
                              ({log.duration}ms)
                            </Text>
                          )}
                        </div>
                        <div style={{ marginTop: 4 }}>
                          <Text strong={log.step_status === 'running'}>{log.message}</Text>
                        </div>
                        
                        {/* Enhanced step details */}
                        {(log.step_type || log.selector || log.value) && (
                          <div style={{ marginTop: 8 }}>
                            {log.step_type && (
                              <Tag color="blue" style={{ marginBottom: 4 }}>
                                {log.step_type}
                              </Tag>
                            )}
                            {log.selector && (
                              <div style={{ marginTop: 4 }}>
                                <Text type="secondary" style={{ fontSize: '12px' }}>选择器: </Text>
                                <Text code style={{ fontSize: '12px' }}>{log.selector}</Text>
                              </div>
                            )}
                            {log.value && (
                              <div style={{ marginTop: 4 }}>
                                <Text type="secondary" style={{ fontSize: '12px' }}>值: </Text>
                                <Text style={{ fontSize: '12px' }}>{log.value}</Text>
                              </div>
                            )}
                          </div>
                        )}

                        {log.error_detail && (
                          <div style={{ marginTop: 8, padding: 8, background: '#fff2f0', borderRadius: 4, border: '1px solid #ffccc7' }}>
                            <Text type="danger" style={{ fontSize: '12px' }}>{log.error_detail}</Text>
                          </div>
                        )}

                        {log.screenshot && (
                          <div style={{ marginTop: 8 }}>
                            <Text type="secondary" style={{ fontSize: '12px' }}>截图: </Text>
                            <Text code style={{ fontSize: '12px' }}>{log.screenshot}</Text>
                          </div>
                        )}

                        {log.details && (
                          <div style={{ marginTop: 4, padding: 8, background: '#f5f5f5', borderRadius: 4 }}>
                            <Text code style={{ fontSize: '12px' }}>{JSON.stringify(log.details, null, 2)}</Text>
                          </div>
                        )}
                      </Timeline.Item>
                    );
                  })}
                </Timeline>
              </div>
            )}

            {executionScreenshots.length > 0 && (
              <div style={{ marginTop: 24 }}>
                <Title level={4}>截图记录 ({executionScreenshots.length}张)</Title>
                <List
                  grid={{ gutter: 16, column: 2 }}
                  dataSource={executionScreenshots}
                  renderItem={(screenshot: any, index: number) => (
                    <List.Item>
                      <Card
                        hoverable
                        cover={
                          <div style={{ position: 'relative' }}>
                            <Image
                              alt={`screenshot-${screenshot.file_name || index}`}
                              src={screenshot.url}
                              preview={{
                                mask: <EyeOutlined />,
                              }}
                              style={{ height: 150, objectFit: 'cover', width: '100%' }}
                              onError={(e) => {
                                console.error('图片加载失败:', screenshot.url);
                                message.error(`截图加载失败: ${screenshot.file_name}`);
                              }}
                              fallback="data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iNjQiIGhlaWdodD0iNjQiIHZpZXdCb3g9IjAgMCA2NCA2NCIgZmlsbD0ibm9uZSIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj4KPHJlY3Qgd2lkdGg9IjY0IiBoZWlnaHQ9IjY0IiBmaWxsPSIjZjVmNWY1Ii8+CjxwYXRoIGQ9Im0zMiAyMC02IDZoNHY4aDJ2LThoNGwtNi02eiIgZmlsbD0iIzk5OTk5OSIvPgo8L3N2Zz4="
                            />
                            {screenshot.type === 'error' && (
                              <Tag color="red" style={{ position: 'absolute', top: 8, right: 8 }}>
                                错误
                              </Tag>
                            )}
                          </div>
                        }
                      >
                        <Card.Meta
                          title={
                            <div>
                              <Text strong>步骤 {screenshot.step_index !== undefined ? screenshot.step_index + 1 : '未知'}</Text>
                              {screenshot.type && (
                                <Tag 
                                  color={screenshot.type === 'error' ? 'red' : screenshot.type === 'before' ? 'blue' : 'green'}
                                  style={{ marginLeft: 8 }}
                                >
                                  {screenshot.type}
                                </Tag>
                              )}
                            </div>
                          }
                          description={
                            <div>
                              <div style={{ marginBottom: 4 }}>
                                <Text>{screenshot.description || '截图'}</Text>
                              </div>
                              <div>
                                <Text type="secondary" style={{ fontSize: '12px' }}>
                                  文件: {screenshot.file_name}
                                </Text>
                              </div>
                              <div>
                                <Text type="secondary" style={{ fontSize: '12px' }}>
                                  时间: {new Date(screenshot.timestamp || screenshot.created_at).toLocaleString()}
                                </Text>
                              </div>
                            </div>
                          }
                        />
                      </Card>
                    </List.Item>
                  )}
                />
              </div>
            )}

            {executionScreenshots.length === 0 && (
              <div style={{ marginTop: 24 }}>
                <Title level={4}>截图记录</Title>
                <Empty 
                  description="暂无截图记录"
                  style={{ marginTop: 20 }}
                />
              </div>
            )}

            {executionLogs.length === 0 && executionScreenshots.length === 0 && (
              <Empty 
                description="暂无详细信息" 
                style={{ marginTop: 40 }}
              />
            )}
          </div>
        )}
      </Drawer>
    </div>
  );
};

export default Reports;