import React, { useEffect, useState } from 'react';
import {
  Card,
  Table,
  Button,
  Space,
  Modal,
  Form,
  Input,
  Select,
  message,
  Popconfirm,
  Tag,
  Typography,
  Descriptions,
  Drawer,
  List,
  Badge,
  Row,
  Col,
  Statistic,
  Transfer,
  Divider,
  Dropdown,
} from 'antd';
import dayjs from 'dayjs';
import type { TransferDirection } from 'antd/es/transfer';
import type { MenuProps } from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  PlayCircleOutlined,
  EyeOutlined,
  ReloadOutlined,
  DownloadOutlined,
  FileTextOutlined,
  FilePdfOutlined,
} from '@ant-design/icons';
import { api } from '../../services/api';
import type { TestSuite, Project, Environment, TestCase, TestSuiteStatistics } from '../../types';
import type { ColumnsType } from 'antd/es/table';

const { Title, Text } = Typography;
const { TextArea } = Input;
const { Option } = Select;


const TestSuites: React.FC = () => {
  const [testSuites, setTestSuites] = useState<TestSuite[]>([]);
  const [projects, setProjects] = useState<Project[]>([]);
  const [environments, setEnvironments] = useState<Environment[]>([]);
  const [testCases, setTestCases] = useState<TestCase[]>([]);
  const [loading, setLoading] = useState(false);
  const [isModalVisible, setIsModalVisible] = useState(false);
  const [isDetailDrawerVisible, setIsDetailDrawerVisible] = useState(false);
  const [isExecuteModalVisible, setIsExecuteModalVisible] = useState(false);
  const [editingTestSuite, setEditingTestSuite] = useState<TestSuite | null>(null);
  const [selectedTestSuite, setSelectedTestSuite] = useState<TestSuite | null>(null);
  const [executingTestSuite, setExecutingTestSuite] = useState<TestSuite | null>(null);
  const [form] = Form.useForm();
  const [executeForm] = Form.useForm();
  const [transferData, setTransferData] = useState<any[]>([]);
  const [targetKeys, setTargetKeys] = useState<string[]>([]);
  const [pagination, setPagination] = useState({
    current: 1,
    pageSize: 10,
    total: 0,
  });
  const [statistics, setStatistics] = useState<TestSuiteStatistics>({
    total: 0,
    enabled: 0,
    scheduled: 0,
    parallel: 0,
  });

  useEffect(() => {
    loadTestSuites();
    loadInitialData();
  }, [pagination.current, pagination.pageSize]);

  const loadInitialData = async () => {
    try {
      const [projectsData, environmentsData, testCasesData] = await Promise.all([
        api.getProjects({ page: 1, page_size: 100 }),
        api.getEnvironments(),
        api.getTestCases({ page: 1, page_size: 1000 }),
      ]);
      setProjects(projectsData.list);
      setEnvironments(environmentsData);
      setTestCases(testCasesData.list);
      
      // Prepare transfer data
      const transferItems = testCasesData.list.map(tc => ({
        key: tc.id.toString(),
        title: tc.name,
        description: `${tc.project?.name} - ${tc.environment?.name}`,
        disabled: tc.status === 0,
      }));
      setTransferData(transferItems);
    } catch (error) {
      console.error('Failed to load initial data:', error);
    }
  };

  const loadTestSuites = async () => {
    setLoading(true);
    try {
      const response = await api.getTestSuites({
        page: pagination.current,
        page_size: pagination.pageSize,
      });
      setTestSuites(response.list);
      setPagination(prev => ({
        ...prev,
        total: response.total,
      }));
      
      // Update statistics if available
      if (response.statistics) {
        setStatistics(response.statistics as TestSuiteStatistics);
      }
    } catch (error) {
      console.error('Failed to load test suites:', error);
      message.error('获取测试套件失败');
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = () => {
    setEditingTestSuite(null);
    setIsModalVisible(true);
    setTargetKeys([]);
    form.resetFields();
  };

  const handleEdit = async (testSuite: TestSuite) => {
    setEditingTestSuite(testSuite);
    setIsModalVisible(true);
    form.setFieldsValue({
      name: testSuite.name,
      description: testSuite.description,
      project_id: testSuite.project_id,
      environment_id: testSuite.environment_id,
      tags: testSuite.tags,
      priority: testSuite.priority,
      cron_expression: testSuite.cron_expression,
      is_parallel: testSuite.is_parallel,
      timeout_minutes: testSuite.timeout_minutes,
    });

    // Get test suite details to load test cases
    try {
      const suiteDetails = await api.getTestSuite(testSuite.id);
      const testCaseIds = suiteDetails.test_cases?.map((tc: any) => tc.id.toString()) || [];
      setTargetKeys(testCaseIds);
    } catch (error) {
      console.error('Failed to load test suite details:', error);
    }
  };

  const handleDelete = async (id: number) => {
    try {
      await api.deleteTestSuite(id);
      message.success('删除成功');
      loadTestSuites();
    } catch (error) {
      console.error('Failed to delete test suite:', error);
      message.error('删除失败');
    }
  };

  const handleExecute = (testSuite: TestSuite) => {
    setExecutingTestSuite(testSuite);
    executeForm.resetFields();
    setIsExecuteModalVisible(true);
  };

  const handleConfirmExecute = async () => {
    if (!executingTestSuite) return;
    
    try {
      const response = await api.executeTestSuite(executingTestSuite.id, {
        is_visual: true,
      });
      message.success('测试套件执行已启动（可视化模式）');
      console.log('Execution started:', response);
      setIsExecuteModalVisible(false);
      setExecutingTestSuite(null);
    } catch (error) {
      console.error('Failed to execute test suite:', error);
      message.error('执行测试套件失败');
    }
  };

  const handleViewDetails = async (testSuite: TestSuite) => {
    try {
      const suiteDetails = await api.getTestSuite(testSuite.id);
      setSelectedTestSuite(suiteDetails);
      setIsDetailDrawerVisible(true);
    } catch (error) {
      console.error('Failed to load test suite details:', error);
      message.error('获取测试套件详情失败');
    }
  };

  const handleSave = async (values: any) => {
    try {
      const formData = {
        ...values,
        test_case_ids: targetKeys.map(key => parseInt(key)),
      };

      if (editingTestSuite) {
        await api.updateTestSuite(editingTestSuite.id, formData);
        message.success('更新成功');
      } else {
        await api.createTestSuite(formData);
        message.success('创建成功');
      }
      setIsModalVisible(false);
      setTargetKeys([]);
      loadTestSuites();
    } catch (error) {
      console.error('Failed to save test suite:', error);
      message.error('保存失败');
    }
  };

  const handleTransferChange = (newTargetKeys: React.Key[], direction: TransferDirection, moveKeys: React.Key[]) => {
    setTargetKeys(newTargetKeys as string[]);
  };

  // 获取最新测试报告
  const handleDownloadLatestReport = (testSuite: TestSuite, format: 'html' | 'pdf') => {
    const url = format === 'html' 
      ? `/api/v1/test-suite-latest-report-html/${testSuite.id}`
      : `/api/v1/test-suite-latest-report-pdf/${testSuite.id}`;
    
    // 创建隐藏的链接进行下载
    const link = document.createElement('a');
    link.href = url;
    link.style.display = 'none';
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    
    message.success(`正在下载${format.toUpperCase()}格式的最新测试报告...`);
  };

  // 生成最新测试报告下拉菜单
  const getLatestReportMenuItems = (testSuite: TestSuite): MenuProps['items'] => [
    {
      key: 'html',
      label: 'HTML报告',
      icon: <FileTextOutlined />,
      onClick: () => handleDownloadLatestReport(testSuite, 'html'),
    },
    {
      key: 'pdf',
      label: 'PDF报告',
      icon: <FilePdfOutlined />,
      onClick: () => handleDownloadLatestReport(testSuite, 'pdf'),
    },
  ];

  const getPriorityColor = (priority: number) => {
    const colors = { 1: 'blue', 2: 'orange', 3: 'red' };
    return colors[priority as keyof typeof colors] || 'default';
  };

  const getPriorityText = (priority: number) => {
    const texts = { 1: '低', 2: '中', 3: '高' };
    return texts[priority as keyof typeof texts] || '未知';
  };

  const getStatusColor = (status: number) => {
    const colors = { 0: 'red', 1: 'green' };
    return colors[status as keyof typeof colors] || 'default';
  };

  const getStatusText = (status: number) => {
    const texts = { 0: '禁用', 1: '启用' };
    return texts[status as keyof typeof texts] || '未知';
  };

  const columns: ColumnsType<TestSuite> = [
    {
      title: '测试套件名称',
      dataIndex: 'name',
      key: 'name',
      width: 200,
      ellipsis: true,
    },
    {
      title: '所属项目',
      dataIndex: ['project', 'name'],
      key: 'project',
      width: 120,
    },
    {
      title: '环境',
      key: 'environment',
      width: 120,
      render: (record: TestSuite) => {
        const tagStyle = {
          whiteSpace: 'normal' as const,
          wordBreak: 'break-word' as const,
          height: 'auto',
          padding: '4px 8px',
          lineHeight: '1.2',
          display: 'inline-block',
          maxWidth: '150px'
        };

        // Use environment_info if available, fallback to old logic
        if (record.environment_info) {
          const { type, summary } = record.environment_info;
          const color = type === 'single' ? 'blue' : type === 'multiple' ? 'orange' : 'gray';
          return <Tag color={color} style={tagStyle}>{summary}</Tag>;
        }
        // Fallback for backward compatibility
        return record.environment?.name ? (
          <Tag color="blue" style={tagStyle}>{record.environment.name}</Tag>
        ) : (
          <Tag color="gray" style={tagStyle}>未设置</Tag>
        );
      },
    },
    {
      title: '测试用例数',
      dataIndex: 'test_case_count',
      key: 'test_case_count',
      width: 100,
      render: (count: number) => (
        <Badge count={count} style={{ backgroundColor: '#52c41a' }} />
      ),
    },
    {
      title: '执行方式',
      dataIndex: 'is_parallel',
      key: 'is_parallel',
      width: 100,
      render: (isParallel: boolean) => (
        <Tag color={isParallel ? 'green' : 'blue'}>
          {isParallel ? '并行' : '串行'}
        </Tag>
      ),
    },
    {
      title: '优先级',
      dataIndex: 'priority',
      key: 'priority',
      width: 80,
      render: (priority: number) => (
        <Tag color={getPriorityColor(priority)}>
          {getPriorityText(priority)}
        </Tag>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 80,
      render: (status: number) => (
        <Tag color={getStatusColor(status)}>
          {getStatusText(status)}
        </Tag>
      ),
    },
    {
      title: '定时表达式',
      dataIndex: 'cron_expression',
      key: 'cron_expression',
      width: 120,
      ellipsis: true,
      render: (cron: string) => cron || '手动执行',
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 150,
      render: (date: string) => dayjs(date).format('YYYY/M/D HH:mm:ss'),
    },
    {
      title: '操作',
      key: 'action',
      width: 280,
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
            icon={<PlayCircleOutlined />}
            onClick={() => handleExecute(record)}
          >
            执行
          </Button>
          <Button
            type="link"
            size="small"
            icon={<EditOutlined />}
            onClick={() => handleEdit(record)}
          >
            编辑
          </Button>
          <Dropdown
            menu={{ items: getLatestReportMenuItems(record) }}
            trigger={['click']}
          >
            <Button
              type="link"
              size="small"
              icon={<DownloadOutlined />}
            >
              最新报告
            </Button>
          </Dropdown>
          <Popconfirm
            title="确定删除这个测试套件吗？"
            onConfirm={() => handleDelete(record.id)}
            okText="确定"
            cancelText="取消"
          >
            <Button
              type="link"
              size="small"
              danger
              icon={<DeleteOutlined />}
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
      <Title level={2}>测试套件管理</Title>
      
      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={6}>
          <Card>
            <Statistic title="总计" value={statistics.total} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="启用"
              value={statistics.enabled}
              valueStyle={{ color: '#3f8600' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="定时任务"
              value={statistics.scheduled}
              valueStyle={{ color: '#1890ff' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="并行执行"
              value={statistics.parallel}
              valueStyle={{ color: '#fa8c16' }}
            />
          </Card>
        </Col>
      </Row>

      <Card>
        <div style={{ marginBottom: 16 }}>
          <Space>
            <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
              新建测试套件
            </Button>
            <Button icon={<ReloadOutlined />} onClick={loadTestSuites}>
              刷新
            </Button>
          </Space>
        </div>

        <Table
          columns={columns}
          dataSource={testSuites}
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

      <Modal
        title={editingTestSuite ? '编辑测试套件' : '新建测试套件'}
        open={isModalVisible}
        onCancel={() => {
          setIsModalVisible(false);
          setTargetKeys([]);
        }}
        onOk={() => form.submit()}
        width={800}
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={handleSave}
        >
          <Form.Item
            name="name"
            label="测试套件名称"
            rules={[
              { required: true, message: '请输入测试套件名称' },
              { min: 1, max: 200, message: '名称长度为1-200个字符' },
            ]}
          >
            <Input placeholder="请输入测试套件名称" />
          </Form.Item>

          <Form.Item
            name="description"
            label="描述"
            rules={[{ max: 1000, message: '描述不能超过1000个字符' }]}
          >
            <TextArea rows={3} placeholder="请输入测试套件描述" />
          </Form.Item>

          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="project_id"
                label="所属项目"
                rules={[{ required: true, message: '请选择项目' }]}
              >
                <Select placeholder="请选择项目">
                  {projects.map(project => (
                    <Option key={project.id} value={project.id}>
                      {project.name}
                    </Option>
                  ))}
                </Select>
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="environment_id"
                label="默认环境"
                help="可选，套件中的测试用例将使用各自的环境设置"
              >
                <Select placeholder="请选择默认环境（可不选）" allowClear>
                  {environments.map(env => (
                    <Option key={env.id} value={env.id}>
                      {env.name} ({env.type})
                    </Option>
                  ))}
                </Select>
              </Form.Item>
            </Col>
          </Row>

          <Row gutter={16}>
            <Col span={8}>
              <Form.Item
                name="priority"
                label="优先级"
                initialValue={2}
                rules={[{ required: true, message: '请选择优先级' }]}
              >
                <Select>
                  <Option value={1}>低</Option>
                  <Option value={2}>中</Option>
                  <Option value={3}>高</Option>
                </Select>
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item
                name="is_parallel"
                label="执行方式"
                initialValue={false}
                rules={[{ required: true, message: '请选择执行方式' }]}
              >
                <Select>
                  <Option value={false}>串行执行</Option>
                  <Option value={true}>并行执行</Option>
                </Select>
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item
                name="timeout_minutes"
                label="超时时间(分钟)"
                initialValue={60}
                rules={[{ required: true, message: '请输入超时时间' }]}
              >
                <Input type="number" min={1} max={1440} placeholder="60" />
              </Form.Item>
            </Col>
          </Row>

          <Form.Item name="tags" label="标签">
            <Input placeholder="请输入标签，多个标签用逗号分隔" />
          </Form.Item>

          <Form.Item name="cron_expression" label="定时表达式">
            <Input placeholder="如：0 0 2 * * ? (每天凌晨2点执行，留空表示手动执行)" />
          </Form.Item>

          <Divider>选择测试用例</Divider>
          <Transfer
            dataSource={transferData}
            titles={['可选测试用例', '已选测试用例']}
            targetKeys={targetKeys}
            onChange={handleTransferChange}
            render={item => (
              <div>
                <div>{item.title}</div>
                <Text type="secondary" style={{ fontSize: '12px' }}>
                  {item.description}
                </Text>
              </div>
            )}
            showSearch
            style={{ marginBottom: 16 }}
          />
        </Form>
      </Modal>

      <Drawer
        title="测试套件详情"
        placement="right"
        size="large"
        onClose={() => setIsDetailDrawerVisible(false)}
        open={isDetailDrawerVisible}
      >
        {selectedTestSuite && (
          <div>
            <Descriptions title="基本信息" bordered column={1} size="small">
              <Descriptions.Item label="名称">
                {selectedTestSuite.name}
              </Descriptions.Item>
              <Descriptions.Item label="描述">
                {selectedTestSuite.description || '无'}
              </Descriptions.Item>
              <Descriptions.Item label="所属项目">
                {selectedTestSuite.project?.name}
              </Descriptions.Item>
              <Descriptions.Item label="环境信息">
                {selectedTestSuite.environment_info ? (
                  <div>
                    <Tag color={selectedTestSuite.environment_info.type === 'single' ? 'blue' : 
                                selectedTestSuite.environment_info.type === 'multiple' ? 'orange' : 'gray'}>
                      {selectedTestSuite.environment_info.summary}
                    </Tag>
                    {selectedTestSuite.environment_info.type === 'multiple' && (
                      <div style={{ marginTop: 8 }}>
                        {selectedTestSuite.environment_info.environments.map(env => (
                          <Tag key={env.id} color="blue" style={{ marginTop: 4 }}>
                            {env.name} ({env.type})
                          </Tag>
                        ))}
                      </div>
                    )}
                  </div>
                ) : (
                  selectedTestSuite.environment?.name ? 
                    `${selectedTestSuite.environment.name} (${selectedTestSuite.environment.type})` : 
                    '未设置环境'
                )}
              </Descriptions.Item>
              <Descriptions.Item label="执行方式">
                <Tag color={selectedTestSuite.is_parallel ? 'green' : 'blue'}>
                  {selectedTestSuite.is_parallel ? '并行' : '串行'}
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item label="超时时间">
                {selectedTestSuite.timeout_minutes} 分钟
              </Descriptions.Item>
              <Descriptions.Item label="定时表达式">
                {selectedTestSuite.cron_expression || '手动执行'}
              </Descriptions.Item>
              <Descriptions.Item label="标签">
                {selectedTestSuite.tags || '无'}
              </Descriptions.Item>
              <Descriptions.Item label="优先级">
                <Tag color={getPriorityColor(selectedTestSuite.priority)}>
                  {getPriorityText(selectedTestSuite.priority)}
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item label="状态">
                <Tag color={getStatusColor(selectedTestSuite.status)}>
                  {getStatusText(selectedTestSuite.status)}
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item label="创建时间">
                {dayjs(selectedTestSuite.created_at).format('YYYY/M/D HH:mm:ss')}
              </Descriptions.Item>
              <Descriptions.Item label="更新时间">
                {dayjs(selectedTestSuite.updated_at).format('YYYY/M/D HH:mm:ss')}
              </Descriptions.Item>
            </Descriptions>

            <div style={{ marginTop: 24 }}>
              <Title level={4}>包含的测试用例</Title>
              {selectedTestSuite.environment_info?.type === 'multiple' ? (
                // Group by environment for multiple environments
                (() => {
                  const groupedByEnv = (selectedTestSuite.test_cases || []).reduce((acc, testCase) => {
                    const envName = testCase.environment?.name || '未知环境';
                    if (!acc[envName]) acc[envName] = [];
                    acc[envName].push(testCase);
                    return acc;
                  }, {} as Record<string, TestCase[]>);

                  return Object.entries(groupedByEnv).map(([envName, testCases]) => (
                    <div key={envName} style={{ marginBottom: 16 }}>
                      <div style={{ 
                        padding: '8px 12px', 
                        backgroundColor: '#f0f9ff', 
                        borderLeft: '4px solid #1890ff',
                        marginBottom: 8 
                      }}>
                        <Text strong>{envName} ({testCases.length}个用例)</Text>
                      </div>
                      <List
                        dataSource={testCases}
                        renderItem={(testCase: TestCase, index) => (
                          <List.Item style={{ paddingLeft: 16 }}>
                            <List.Item.Meta
                              avatar={<Badge count={index + 1} style={{ backgroundColor: '#52c41a' }} />}
                              title={testCase.name}
                              description={
                                <Space>
                                  <Tag>{testCase.project?.name}</Tag>
                                  <Tag color={getPriorityColor(testCase.priority)}>
                                    {getPriorityText(testCase.priority)}
                                  </Tag>
                                </Space>
                              }
                            />
                          </List.Item>
                        )}
                      />
                    </div>
                  ));
                })()
              ) : (
                // Standard list for single environment or empty
                <List
                  dataSource={selectedTestSuite.test_cases || []}
                  renderItem={(testCase: TestCase, index) => (
                    <List.Item>
                      <List.Item.Meta
                        avatar={<Badge count={index + 1} style={{ backgroundColor: '#1890ff' }} />}
                        title={testCase.name}
                        description={
                          <Space>
                            <Tag>{testCase.project?.name}</Tag>
                            <Tag>{testCase.environment?.name}</Tag>
                            <Tag color={getPriorityColor(testCase.priority)}>
                              {getPriorityText(testCase.priority)}
                            </Tag>
                          </Space>
                        }
                      />
                    </List.Item>
                  )}
                />
              )}
            </div>
          </div>
        )}
      </Drawer>

      <Modal
        title="确认执行测试套件"
        open={isExecuteModalVisible}
        onCancel={() => {
          setIsExecuteModalVisible(false);
          setExecutingTestSuite(null);
        }}
        onOk={handleConfirmExecute}
        okText="开始执行"
        cancelText="取消"
        width={500}
      >
        {executingTestSuite && (
          <div>
            <Descriptions title="执行信息" bordered column={1} size="small">
              <Descriptions.Item label="测试套件">
                {executingTestSuite.name}
              </Descriptions.Item>
              <Descriptions.Item label="包含用例数">
                {executingTestSuite.test_case_count} 个
              </Descriptions.Item>
              <Descriptions.Item label="执行方式">
                <Tag color={executingTestSuite.is_parallel ? 'green' : 'blue'}>
                  {executingTestSuite.is_parallel ? '并行执行' : '串行执行'}
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item label="超时时间">
                {executingTestSuite.timeout_minutes} 分钟
              </Descriptions.Item>
            </Descriptions>
            
            <div style={{ marginTop: 16, padding: '16px', backgroundColor: '#f6ffed', border: '1px solid #b7eb8f', borderRadius: '6px' }}>
              <div style={{ fontSize: '16px', fontWeight: 'bold', color: '#52c41a', marginBottom: '8px' }}>可视化执行模式</div>
              <div style={{ fontSize: '14px', color: '#666' }}>
                浏览器界面可见，可以实时观察执行过程和页面交互
              </div>
            </div>
          </div>
        )}
      </Modal>
    </div>
  );
};

export default TestSuites;