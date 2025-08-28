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
  Dropdown,
  Modal,
  Input,
} from 'antd';
import {
  EyeOutlined,
  ReloadOutlined,
  DownloadOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  ClockCircleOutlined,
  FileTextOutlined,
  FilePdfOutlined,
  PlayCircleOutlined,
} from '@ant-design/icons';
import dayjs from 'dayjs';
import { api } from '../../services/api';
import type { TestExecution, Project, Environment } from '../../types';
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
    name?: string;
    project_id?: number;
    environment_id?: number;
    status?: string;
    date_range?: any;
  }>({
    name: undefined,
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
  
  // 继续执行确认弹窗相关状态
  const [isContinueModalVisible, setIsContinueModalVisible] = useState(false);
  const [continueExecutionData, setContinueExecutionData] = useState<{
    execution: TestExecution | null;
    testCases: any[];
    loading: boolean;
  }>({
    execution: null,
    testCases: [],
    loading: false,
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
      if (filters.name) params.name = filters.name;
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

      if (filters.name) params.name = filters.name;
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

  const handleDownloadReport = async (execution: TestExecution, format: 'html' | 'pdf' = 'html') => {
    try {
      const testName = execution.test_case?.name || execution.test_suite?.name;
      
      // Validate required fields
      if (!testName) {
        message.error('缺少必要的测试信息，无法生成报告');
        return;
      }
      
      const formatName = format === 'pdf' ? 'PDF' : 'HTML';
      message.loading(`正在生成${formatName}测试报告...`, 0);
      
      // Get auth token
      const token = localStorage.getItem('token');
      
      // Download report from backend
      const response = await fetch(`/api/v1/executions/${execution.id}/report/${format}`, {
        method: 'GET',
        headers: {
          'Authorization': `Bearer ${token}`,
        },
      });
      
      // Check if response is successful
      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`HTTP ${response.status}: ${errorText}`);
      }
      
      // Verify content type
      const contentType = response.headers.get('content-type');
      const expectedContentType = format === 'pdf' ? 'application/pdf' : 'text/html';
      if (!contentType || !contentType.includes(expectedContentType)) {
        console.warn('Unexpected content type:', contentType);
      }
      
      // Get blob from response
      const blob = await response.blob();
      
      // Verify blob size
      if (blob.size === 0) {
        throw new Error(`${formatName}文件为空`);
      }
      
      // Create download link
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      
      // Generate filename with timestamp (use safe characters only)
      const timestamp = new Date().toISOString().slice(0, 19).replace(/[T:]/g, '-');
      const safeTestName = testName.replace(/[<>:"/\\|?*]/g, '_'); // Replace unsafe characters
      const extension = format === 'pdf' ? 'pdf' : 'html';
      link.download = `TestReport-${safeTestName}-${timestamp}.${extension}`;
      
      // Trigger download
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(url);
      
      message.destroy();
      message.success(`${formatName}测试报告已生成，请选择保存位置`);
      
    } catch (error) {
      message.destroy();
      console.error('Failed to generate report:', error);
      message.error(`生成${format === 'pdf' ? 'PDF' : 'HTML'}报告失败`);
    }
  };

  // 显示继续执行确认弹窗
  const handleContinueExecution = async (execution: TestExecution) => {
    setContinueExecutionData(prev => ({ ...prev, loading: true }));
    setIsContinueModalVisible(true);
    
    try {
      if (execution.execution_type === 'test_suite') {
        // 获取测试套件中所有测试用例的状态
        if (!execution.test_suite_id) {
          message.error('测试套件ID无效，无法继续执行');
          setIsContinueModalVisible(false);
          return;
        }
        
        // 获取测试套件详情，包含所有测试用例
        const suiteDetails = await api.getTestSuite(execution.test_suite_id);
        const testCases = suiteDetails.test_cases || [];
        
        // 获取该套件执行的所有子执行记录
        const suiteExecutions = await api.getExecutions({
          parent_execution_id: execution.id,
          include_internal: true,
          page_size: 1000, // 确保获取所有子执行
        });
        
        // 为每个测试用例匹配对应的执行结果
        const testCasesWithStatus = testCases.map(testCase => {
          const testCaseExecution = suiteExecutions.list?.find(
            exec => exec.test_case_id === testCase.id
          );
          
          return {
            ...testCase,
            latest_status: testCaseExecution?.status || 'not_executed',
            latest_execution_time: testCaseExecution?.start_time || null,
            should_execute: !testCaseExecution || testCaseExecution.status !== 'passed',
          };
        });
        
        setContinueExecutionData({
          execution,
          testCases: testCasesWithStatus,
          loading: false,
        });
      } else if (execution.execution_type === 'test_case') {
        // 单个测试用例
        if (execution.status === 'passed') {
          message.info('该测试用例已通过，无需重新执行');
          setIsContinueModalVisible(false);
          return;
        }
        
        setContinueExecutionData({
          execution,
          testCases: [{
            id: execution.test_case_id || 0,
            name: execution.test_case?.name || '未知测试用例',
            latest_status: execution.status,
            latest_execution_time: execution.start_time,
            should_execute: (execution.status as any) !== 'passed',
          } as any],
          loading: false,
        });
      } else {
        message.error('不支持的执行类型');
        setIsContinueModalVisible(false);
      }
    } catch (error) {
      console.error('Failed to load execution details:', error);
      message.error('获取执行详情失败');
      setIsContinueModalVisible(false);
      setContinueExecutionData(prev => ({ ...prev, loading: false }));
    }
  };

  // 确认继续执行
  const handleConfirmContinueExecution = async () => {
    const { execution } = continueExecutionData;
    if (!execution) return;
    
    try {
      if (execution.execution_type === 'test_suite') {
        const response = await api.executeTestSuite(execution.test_suite_id!, {
          is_visual: true,
          continue_failed_only: true, // 只执行失败的测试用例
          parent_execution_id: execution.id, // 传递原始执行ID用于判断哪些测试用例需要重新执行
        });
        
        message.success('测试套件继续执行已启动，将重新执行未通过的测试用例');
        console.log('Suite continue execution started:', response);
      } else if (execution.execution_type === 'test_case') {
        const response = await api.executeTestCase(execution.test_case_id!, {
          is_visual: true,
        });
        
        message.success('测试用例重新执行已启动');
        console.log('Test case re-execution started:', response);
      }
      
      // 关闭弹窗并刷新列表
      setIsContinueModalVisible(false);
      setContinueExecutionData({
        execution: null,
        testCases: [],
        loading: false,
      });
      loadExecutions();
    } catch (error) {
      console.error('Failed to continue execution:', error);
      message.error('继续执行失败');
    }
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
      width: 120,
      render: (_, record) => {
        const tagStyle = {
          whiteSpace: 'normal' as const,
          wordBreak: 'break-word' as const,
          height: 'auto',
          padding: '4px 8px',
          lineHeight: '1.2',
          display: 'inline-block',
          maxWidth: '150px'
        };

        // 如果是测试用例执行，显示用例的环境
        if (record.execution_type === 'test_case' && record.test_case?.environment?.name) {
          return <Tag color="blue" style={tagStyle}>{record.test_case.environment.name}</Tag>;
        }
        
        // 如果是测试套件执行，显示套件的环境信息
        if (record.execution_type === 'test_suite' && record.test_suite) {
          if (record.test_suite.environment_info) {
            const { type, summary } = record.test_suite.environment_info;
            const color = type === 'single' ? 'blue' : type === 'multiple' ? 'orange' : 'gray';
            return <Tag color={color} style={tagStyle}>{summary}</Tag>;
          } else if (record.test_suite.environment?.name) {
            // 向后兼容：如果没有environment_info但有environment
            return <Tag color="blue" style={tagStyle}>{record.test_suite.environment.name}</Tag>;
          }
        }
        
        return <Tag color="gray" style={tagStyle}>未知环境</Tag>;
      },
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
      render: (date: string) => dayjs(date).format('YYYY/M/D HH:mm:ss'),
    },
    {
      title: '操作',
      key: 'action',
      width: 280,
      render: (_, record) => {
        const reportMenuItems = [
          {
            key: 'html',
            label: 'HTML报告',
            icon: <FileTextOutlined />,
            onClick: () => handleDownloadReport(record, 'html'),
          },
          // PDF功能暂时隐藏，保留函数
          // {
          //   key: 'pdf',
          //   label: 'PDF报告',
          //   icon: <FilePdfOutlined />,
          //   onClick: () => handleDownloadReport(record, 'pdf'),
          // },
        ];

        return (
          <Space size="small">
            <Button
              type="link"
              size="small"
              icon={<EyeOutlined />}
              onClick={() => handleViewDetails(record)}
            >
              详情
            </Button>
            <Dropdown
              menu={{ items: reportMenuItems }}
              placement="bottomRight"
              disabled={record.status !== 'passed' && record.status !== 'failed'}
            >
              <Button
                type="link"
                size="small"
                icon={<DownloadOutlined />}
                disabled={record.status !== 'passed' && record.status !== 'failed'}
              >
                导出报告
              </Button>
            </Dropdown>
            {record.status !== 'passed' && (
              <Button
                type="link"
                size="small"
                icon={<PlayCircleOutlined />}
                onClick={() => handleContinueExecution(record)}
                disabled={record.status === 'running' || record.status === 'pending'}
              >
                继续执行
              </Button>
            )}
          </Space>
        );
      },
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
            <Input.Search
              placeholder="请输入名称"
              style={{ width: 200 }}
              allowClear
              value={filters.name}
              onChange={(e) => setFilters({ ...filters, name: e.target.value })}
              onSearch={(value) => setFilters({ ...filters, name: value })}
              enterButton="查询"
            />
            
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
          <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
            {/* 可滚动内容区域 */}
            <div style={{ flex: 1, overflow: 'auto', paddingBottom: selectedTestCaseFromSuite ? '60px' : '0' }}>
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
                  <Descriptions.Item label="环境信息">
                    {selectedExecution.test_suite?.environment_info ? (
                      <div>
                        <Tag 
                          color={selectedExecution.test_suite.environment_info.type === 'single' ? 'blue' : 
                                selectedExecution.test_suite.environment_info.type === 'multiple' ? 'orange' : 'gray'}
                          style={{
                            whiteSpace: 'normal',
                            wordBreak: 'break-word',
                            height: 'auto',
                            padding: '4px 8px',
                            lineHeight: '1.2',
                            display: 'inline-block',
                            maxWidth: '200px'
                          }}
                        >
                          {selectedExecution.test_suite.environment_info.summary}
                        </Tag>
                        {selectedExecution.test_suite.environment_info.type === 'multiple' && (
                          <div style={{ marginTop: 8 }}>
                            {selectedExecution.test_suite.environment_info.environments.map(env => (
                              <Tag 
                                key={env.id} 
                                color="blue" 
                                style={{ 
                                  marginTop: 4,
                                  whiteSpace: 'normal',
                                  wordBreak: 'break-word',
                                  height: 'auto',
                                  padding: '4px 8px',
                                  lineHeight: '1.2',
                                  display: 'inline-block',
                                  maxWidth: '180px'
                                }}
                              >
                                {env.name} ({env.type})
                              </Tag>
                            ))}
                          </div>
                        )}
                      </div>
                    ) : (
                      <Tag 
                        color="blue"
                        style={{
                          whiteSpace: 'normal',
                          wordBreak: 'break-word',
                          height: 'auto',
                          padding: '4px 8px',
                          lineHeight: '1.2',
                          display: 'inline-block',
                          maxWidth: '200px'
                        }}
                      >
                        {selectedExecution.test_suite?.environment?.name || '未设置环境'}
                      </Tag>
                    )}
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
                    {dayjs(selectedExecution.start_time).format('YYYY/M/D HH:mm:ss')}
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
                    {selectedTestCaseFromSuite?.test_case?.environment?.name || 
                     selectedExecution.test_case?.environment?.name || 
                     (selectedExecution.test_suite?.environment_info?.summary) ||
                     selectedExecution.test_suite?.environment?.name ||
                     '未知环境'}
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
                    {dayjs(selectedTestCaseFromSuite?.start_time || selectedExecution.start_time).format('YYYY/M/D HH:mm:ss')}
                  </Descriptions.Item>
                  <Descriptions.Item label="结束时间">
                    {(selectedTestCaseFromSuite?.end_time || selectedExecution.end_time) ? 
                      dayjs(selectedTestCaseFromSuite?.end_time || selectedExecution.end_time!).format('YYYY/M/D HH:mm:ss') : '未结束'}
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
                                  时间: {dayjs(screenshot.timestamp || screenshot.created_at).format('YYYY/M/D HH:mm:ss')}
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

            {/* 固定在底部的按钮区域 */}
            {selectedTestCaseFromSuite && !showTestSuiteDetails && (
              <div style={{
                position: 'absolute',
                bottom: 0,
                left: 0,
                right: 0,
                padding: '16px 24px',
                backgroundColor: '#fff',
                borderTop: '1px solid #f0f0f0',
                boxShadow: '0 -2px 8px rgba(0, 0, 0, 0.1)'
              }}>
                <Button 
                  icon={<span>←</span>} 
                  onClick={handleBackToSuiteDetails}
                  type="primary"
                  block
                >
                  返回套件详情
                </Button>
              </div>
            )}
          </div>
        )}
      </Drawer>

      {/* 继续执行确认弹窗 */}
      <Modal
        title="继续执行确认"
        open={isContinueModalVisible}
        onOk={handleConfirmContinueExecution}
        onCancel={() => {
          setIsContinueModalVisible(false);
          setContinueExecutionData({
            execution: null,
            testCases: [],
            loading: false,
          });
        }}
        width={800}
        okText="确认执行"
        cancelText="取消"
        confirmLoading={continueExecutionData.loading}
      >
        {continueExecutionData.loading ? (
          <div style={{ textAlign: 'center', padding: '40px 0' }}>
            <span>正在加载测试用例信息...</span>
          </div>
        ) : (
          <div>
            <div style={{ marginBottom: 16 }}>
              <Text strong>
                {continueExecutionData.execution?.execution_type === 'test_suite' 
                  ? '即将重新执行以下未通过的测试用例：' 
                  : '即将重新执行该测试用例：'}
              </Text>
            </div>
            
            <Table
              dataSource={continueExecutionData.testCases}
              rowKey="id"
              pagination={false}
              size="middle"
            >
              <Table.Column
                title="测试用例"
                dataIndex="name"
                key="name"
                width={200}
              />
              <Table.Column
                title="上次状态"
                dataIndex="latest_status"
                key="latest_status"
                width={120}
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
                title="上次执行时间"
                dataIndex="latest_execution_time"
                key="latest_execution_time"
                width={150}
                render={(time) => time ? dayjs(time).format('YYYY/M/D HH:mm:ss') : '未执行'}
              />
              <Table.Column
                title="是否执行"
                dataIndex="should_execute"
                key="should_execute"
                width={100}
                render={(shouldExecute) => (
                  <Tag color={shouldExecute ? 'orange' : 'green'}>
                    {shouldExecute ? '将执行' : '跳过'}
                  </Tag>
                )}
              />
            </Table>
            
            {continueExecutionData.testCases.filter(tc => tc.should_execute).length === 0 && (
              <div style={{ textAlign: 'center', padding: '20px 0', color: '#999' }}>
                所有测试用例都已通过，无需重新执行
              </div>
            )}
          </div>
        )}
      </Modal>
    </div>
  );
};

export default Reports;