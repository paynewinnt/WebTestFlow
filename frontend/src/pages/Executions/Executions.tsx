import React, { useEffect, useState } from 'react';
import {
  Card,
  Table,
  Button,
  Space,
  message,
  Tag,
  Typography,
  Row,
  Col,
  Statistic,
  Progress,
  DatePicker,
  Select,
  Popconfirm,
  Switch,
  Tooltip,
  Input,
} from 'antd';
import dayjs from 'dayjs';
import {
  ReloadOutlined,
  DeleteOutlined,
  StopOutlined,
} from '@ant-design/icons';
import { api } from '../../services/api';
import type { TestExecution, Project, Environment } from '../../types';
import type { ColumnsType } from 'antd/es/table';

const { Title, Text } = Typography;
const { RangePicker } = DatePicker;
const { Option } = Select;

const Executions: React.FC = () => {
  const [executions, setExecutions] = useState<TestExecution[]>([]);
  const [projects, setProjects] = useState<Project[]>([]);
  const [environments, setEnvironments] = useState<Environment[]>([]);
  const [loading, setLoading] = useState(false);
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [runningExecutions, setRunningExecutions] = useState<Set<number>>(new Set());
  const [statistics, setStatistics] = useState({
    total_executions: 0,
    passed_count: 0,
    failed_count: 0,
    running_count: 0,
    pending_count: 0,
    success_rate: 0,
    avg_duration: 0,
  });
  const [filters, setFilters] = useState<{
    name?: string;
    project_id?: number;
    environment_id?: number;
    status?: string;
    execution_type?: string;
    date_range?: any;
  }>({
    name: undefined,
    project_id: undefined,
    environment_id: undefined,
    status: undefined,
    execution_type: undefined,
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
    loadInitialData();
  }, [pagination.current, pagination.pageSize, filters]);

  // Auto refresh effect for running executions
  useEffect(() => {
    if (!autoRefresh) return;

    const interval = setInterval(() => {
      // Check if there are any running executions
      const hasRunning = executions.some(e => e.status === 'running' || e.status === 'pending');
      if (hasRunning) {
        loadExecutions();
        loadStatistics();
      }
    }, 5000); // Refresh every 5 seconds

    return () => clearInterval(interval);
  }, [executions, autoRefresh]);

  // Track running executions and check their status
  useEffect(() => {
    const newRunning = new Set(
      executions.filter(e => e.status === 'running' || e.status === 'pending').map(e => e.id)
    );
    
    // For newly completed executions, check status consistency
    runningExecutions.forEach(async (executionId) => {
      if (!newRunning.has(executionId)) {
        // This execution just completed, verify status consistency
        try {
          const statusInfo = await api.getExecutionStatus(executionId);
          if (!statusInfo.consistent) {
            console.warn(`Status inconsistency detected for execution ${executionId}:`, statusInfo);
            // Trigger a refresh to get updated status
            setTimeout(() => loadExecutions(), 1000);
          }
        } catch (error) {
          console.error('Failed to check execution status:', error);
        }
      }
    });
    
    setRunningExecutions(newRunning);
  }, [executions]);

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
      if (filters.execution_type) params.execution_type = filters.execution_type;
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
      if (filters.execution_type) params.execution_type = filters.execution_type;
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

  const handleDelete = async (id: number) => {
    try {
      await api.deleteExecution(id);
      message.success('删除成功');
      loadExecutions();
      loadStatistics();
    } catch (error) {
      console.error('Failed to delete execution:', error);
      message.error('删除失败');
    }
  };

  const handleStop = async (id: number) => {
    try {
      // Assuming there's a stop execution API
      await api.stopExecution(id);
      message.success('停止成功');
      loadExecutions();
      loadStatistics();
    } catch (error) {
      console.error('Failed to stop execution:', error);
      message.error('停止失败');
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


  const columns: ColumnsType<TestExecution> = [
    {
      title: '执行ID',
      dataIndex: 'id',
      key: 'id',
      width: 80,
      sorter: (a, b) => a.id - b.id,
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
      render: (status: string, record) => {
        const isRunning = status === 'running' || status === 'pending';
        return (
          <div>
            <Tag color={getStatusColor(status)}>
              {getStatusText(status)}
            </Tag>
            {isRunning && (
              <Tooltip title="实时监控中">
                <div style={{ 
                  width: '8px', 
                  height: '8px', 
                  backgroundColor: '#52c41a', 
                  borderRadius: '50%',
                  display: 'inline-block',
                  marginLeft: '4px',
                  animation: 'pulse 2s infinite'
                }} />
              </Tooltip>
            )}
          </div>
        );
      },
    },
    {
      title: '测试结果',
      key: 'test_results',
      width: 120,
      render: (_, record) => (
        <div>
          <div>
            <Text style={{ color: '#52c41a' }}>✓ {record.passed_count}</Text>
            <Text style={{ margin: '0 4px' }}>/</Text>
            <Text style={{ color: '#ff4d4f' }}>✗ {record.failed_count}</Text>
            <Text style={{ margin: '0 4px' }}>/</Text>
            <Text>总 {record.total_count}</Text>
          </div>
          <Progress
            percent={record.total_count > 0 ? Math.round((record.passed_count / record.total_count) * 100) : 0}
            size="small"
            showInfo={false}
            strokeColor={record.passed_count === record.total_count ? '#52c41a' : '#ff4d4f'}
          />
        </div>
      ),
    },
    {
      title: '执行时长',
      dataIndex: 'duration',
      key: 'duration',
      width: 100,
      render: (duration: number) => {
        if (duration === 0) return '-';
        const seconds = Math.round(duration / 1000);
        if (seconds < 60) return `${seconds}s`;
        const minutes = Math.floor(seconds / 60);
        const remainingSeconds = seconds % 60;
        return `${minutes}m ${remainingSeconds}s`;
      },
      sorter: (a, b) => a.duration - b.duration,
    },
    {
      title: '开始时间',
      dataIndex: 'start_time',
      key: 'start_time',
      width: 150,
      render: (date: string) => dayjs(date).format('YYYY/M/D HH:mm:ss'),
      sorter: (a, b) => new Date(a.start_time).getTime() - new Date(b.start_time).getTime(),
      defaultSortOrder: 'descend' as const,
    },
    {
      title: '结束时间',
      dataIndex: 'end_time',
      key: 'end_time',
      width: 150,
      render: (date: string | null) => date ? dayjs(date).format('YYYY/M/D HH:mm:ss') : '-',
    },
    {
      title: '操作',
      key: 'action',
      width: 120,
      render: (_, record) => (
        <Space size="small">
          {record.status === 'running' && (
            <Popconfirm
              title="确定要停止这个执行吗？"
              onConfirm={() => handleStop(record.id)}
              okText="确定"
              cancelText="取消"
            >
              <Button
                type="link"
                size="small"
                danger
                icon={<StopOutlined />}
              >
                停止
              </Button>
            </Popconfirm>
          )}
          <Popconfirm
            title="确定删除这条执行记录吗？"
            onConfirm={() => handleDelete(record.id)}
            okText="确定"
            cancelText="取消"
          >
            <Button
              type="link"
              size="small"
              danger
              icon={<DeleteOutlined />}
              disabled={record.status === 'running'}
            >
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <style>{`
        @keyframes pulse {
          0% { opacity: 1; }
          50% { opacity: 0.5; }
          100% { opacity: 1; }
        }
      `}</style>
      <Title level={2}>执行记录</Title>
      
      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={6}>
          <Card>
            <Statistic 
              title="总执行次数" 
              value={statistics.total_executions} 
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="成功率"
              value={Math.round(statistics.success_rate)}
              suffix="%"
              valueStyle={{ 
                color: statistics.success_rate >= 80 ? '#3f8600' : '#cf1322' 
              }}
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
              title="平均时长"
              value={Math.round(statistics.avg_duration / 1000)}
              suffix="s"
              valueStyle={{ color: '#722ed1' }}
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
              placeholder="执行类型"
              style={{ width: 120 }}
              allowClear
              value={filters.execution_type}
              onChange={(value) => setFilters({ ...filters, execution_type: value })}
            >
              <Option value="test_case">测试用例</Option>
              <Option value="test_suite">测试套件</Option>
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
            
            <Space>
              <span>自动刷新:</span>
              <Switch 
                checked={autoRefresh}
                onChange={setAutoRefresh}
                size="small"
              />
              {autoRefresh && runningExecutions.size > 0 && (
                <Text type="secondary" style={{ fontSize: '12px' }}>
                  监控 {runningExecutions.size} 个运行中的任务
                </Text>
              )}
            </Space>
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
          scroll={{ x: 1300 }}
          onChange={(pagination, filters, sorter) => {
            // Prevent table column filters from interfering with our API filters
            // Only handle pagination changes here
          }}
        />
      </Card>
    </div>
  );
};

export default Executions;