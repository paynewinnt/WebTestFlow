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
  Spin,
  Alert,
  Dropdown,
} from 'antd';
import dayjs from 'dayjs';
import type { TransferDirection } from 'antd/es/transfer';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  PlayCircleOutlined,
  EyeOutlined,
  ReloadOutlined,
  FileTextOutlined,
  FilePdfOutlined,
  FileImageOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  ClockCircleOutlined,
  MinusCircleOutlined,
  SearchOutlined,
  MoreOutlined,
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
  const [searchKeyword, setSearchKeyword] = useState<string>('');
  const [isLatestReportModalVisible, setIsLatestReportModalVisible] = useState(false);
  const [latestReportData, setLatestReportData] = useState<any>(null);
  const [loadingLatestReport, setLoadingLatestReport] = useState(false);

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
      const params: any = {
        page: pagination.current,
        page_size: pagination.pageSize,
      };
      if (searchKeyword.trim()) {
        params.name = searchKeyword.trim();
      }
      const response = await api.getTestSuites(params);
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

  const handleSearch = () => {
    setPagination(prev => ({ ...prev, current: 1 }));
    loadTestSuites();
  };

  const handleSearchReset = () => {
    setSearchKeyword('');
    setPagination(prev => ({ ...prev, current: 1 }));
    setTimeout(() => {
      loadTestSuites();
    }, 0);
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
  const handleDownloadLatestReport = async (testSuite: TestSuite) => {
    setLoadingLatestReport(true);
    try {
      const response = await api.getTestSuiteLatestReport(testSuite.id);
      setLatestReportData(response);
      setIsLatestReportModalVisible(true);
    } catch (error) {
      console.error('Failed to get latest report:', error);
      message.error('获取最新报告失败');
    } finally {
      setLoadingLatestReport(false);
    }
  };


  // 处理直接导出报告（新的按钮式导出）
  const handleDirectExport = async (format: 'html' | 'pdf' | 'html_with_screenshots' | 'pdf_with_screenshots') => {
    if (!latestReportData || !latestReportData.test_suite) {
      message.error('没有可导出的数据');
      return;
    }
    const testSuiteId = latestReportData.test_suite?.id;
    if (!testSuiteId) {
      message.error('无效的测试套件ID');
      return;
    }

    try {
      const formatName = format === 'html' ? 'HTML报告' : 
                        format === 'pdf' ? 'PDF报告' :
                        format === 'html_with_screenshots' ? 'HTML报告（带截图）' :
                        'PDF报告（带截图）';
      message.loading(`正在生成${formatName}...`, 0);
      
      // 根据选择的格式调用相应的API
      let endpoint: string;
      switch (format) {
        case 'html':
          endpoint = `test-suites/${testSuiteId}/latest-report-html`;
          break;
        case 'pdf':
          endpoint = `test-suites/${testSuiteId}/latest-report-pdf`;
          break;
        case 'html_with_screenshots':
          endpoint = `test-suites/${testSuiteId}/latest-report-html-with-screenshots`;
          break;
        case 'pdf_with_screenshots':
          endpoint = `test-suites/${testSuiteId}/latest-report-pdf-with-screenshots`;
          break;
        default:
          endpoint = `test-suites/${testSuiteId}/latest-report-html`;
      }
      
      // 直接调用API导出
      const response = await fetch(`/api/v1/${endpoint}`, {
        method: 'GET',
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
          'Content-Type': 'application/json',
        },
      });
      
      message.destroy(); // 清除loading消息
      
      if (!response.ok) {
        throw new Error(`导出失败: ${response.status}`);
      }
      
      // 获取文件blob并下载
      const blob = await response.blob();
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      
      // 尝试从响应头获取文件名
      let fileName = '';
      const contentDisposition = response.headers.get('Content-Disposition');
      if (contentDisposition) {
        // 优先尝试解析 filename*=UTF-8''encoded_filename 格式（支持中文）
        let matches = contentDisposition.match(/filename\*=UTF-8''([^;]+)/);
        if (matches && matches[1]) {
          try {
            fileName = decodeURIComponent(matches[1]);
          } catch (e) {
            console.warn('Failed to decode filename*:', e);
          }
        }
        
        // 如果上面失败，尝试解析普通 filename="..." 格式
        if (!fileName) {
          matches = contentDisposition.match(/filename="([^"]+)"/);
          if (matches && matches[1]) {
            fileName = matches[1];
          }
        }
      }
      
      // 如果无法从响应头获取文件名，使用默认格式
      if (!fileName) {
        const fileExtension = format.includes('html') ? 'html' : 'pdf';
        fileName = format.includes('with_screenshots') 
          ? `test_suite_${testSuiteId}_latest_report_with_screenshots.${fileExtension}`
          : `test_suite_${testSuiteId}_latest_report.${fileExtension}`;
      }
      
      link.download = fileName;
      link.style.display = 'none';
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(url);
      
      message.success(`${formatName}已成功导出`);
    } catch (error) {
      message.destroy(); // 清除loading消息
      console.error('Export error:', error);
      message.error(`导出失败: ${error instanceof Error ? error.message : '未知错误'}`);
    }
  };

  // 执行单个测试用例
  const handleExecuteTestCase = async (testCaseId: number, testCaseName: string) => {
    try {
      const response = await api.executeTestCase(testCaseId, { is_visual: true });
      message.success(`测试用例"${testCaseName}"执行已启动（可视化模式）`);
      console.log('Test case execution started:', response);
    } catch (error) {
      console.error('Failed to execute test case:', error);
      message.error(`执行测试用例"${testCaseName}"失败`);
    }
  };

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
      render: (_, record) => {
        const items = [
          {
            key: 'execute',
            label: '执行测试',
            icon: <PlayCircleOutlined />,
          },
          {
            key: 'edit',
            label: '编辑',
            icon: <EditOutlined />,
          },
          {
            type: 'divider' as const,
          },
          {
            key: 'delete',
            label: '删除',
            icon: <DeleteOutlined />,
            danger: true,
          },
        ];

        const handleMenuClick = ({ key }: { key: string }) => {
          switch (key) {
            case 'execute':
              handleExecute(record);
              break;
            case 'edit':
              handleEdit(record);
              break;
            case 'delete':
              Modal.confirm({
                title: '确定删除这个测试套件吗？',
                onOk: () => handleDelete(record.id),
                okText: '确定',
                cancelText: '取消',
              });
              break;
          }
        };

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
            <Button
              type="link"
              size="small"
              icon={<FileTextOutlined />}
              loading={loadingLatestReport}
              onClick={() => handleDownloadLatestReport(record)}
            >
              最新报告
            </Button>
            <Dropdown
              menu={{ 
                items,
                onClick: handleMenuClick
              }}
              trigger={['click']}
            >
              <Button type="link" size="small">更多 ...</Button>
            </Dropdown>
          </Space>
        );
      },
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
            <Input.Search
              placeholder="请输入测试套件名称"
              value={searchKeyword}
              onChange={(e) => setSearchKeyword(e.target.value)}
              onSearch={handleSearch}
              onPressEnter={handleSearch}
              style={{ width: 300 }}
              enterButton={
                <Button type="primary" icon={<SearchOutlined />}>
                  搜索
                </Button>
              }
            />
            {searchKeyword && (
              <Button onClick={handleSearchReset}>
                清空搜索
              </Button>
            )}
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

      {/* 最新报告弹窗 */}
      <Modal
        title="测试套件最新执行报告"
        open={isLatestReportModalVisible}
        onCancel={() => setIsLatestReportModalVisible(false)}
        footer={[
          <div key="export-buttons" style={{ display: 'flex', justifyContent: 'center', gap: '12px', marginBottom: '8px' }}>
            <Button 
              type="primary" 
              icon={<FileTextOutlined />}
              onClick={() => handleDirectExport('html')}
            >
              HTML报告
            </Button>
            <Button 
              type="primary" 
              icon={<FilePdfOutlined />}
              onClick={() => handleDirectExport('pdf')}
            >
              PDF报告
            </Button>
            <Button 
              type="primary" 
              icon={<FileImageOutlined />}
              onClick={() => handleDirectExport('html_with_screenshots')}
            >
              HTML报告（带截图）
            </Button>
            {/* PDF带截图功能暂时隐藏 */}
            {/* <Button 
              type="primary" 
              icon={<FilePdfOutlined />}
              onClick={() => handleDirectExport('pdf_with_screenshots')}
            >
              PDF报告（带截图）
            </Button> */}
          </div>,
          <Button key="close" onClick={() => setIsLatestReportModalVisible(false)}>
            关闭
          </Button>
        ]}
        width={900}
        styles={{ body: { maxHeight: '600px', overflow: 'auto' } }}
      >
        {loadingLatestReport ? (
          <div style={{ textAlign: 'center', padding: '50px' }}>
            <Spin size="large" tip="正在加载最新报告..." />
          </div>
        ) : latestReportData ? (
          <div>
            {/* 汇总统计 */}
            <Card style={{ marginBottom: 16 }}>
              <Row gutter={16}>
                <Col span={6}>
                  <Statistic
                    title="总用例数"
                    value={latestReportData.summary?.total_count || 0}
                    valueStyle={{ color: '#1890ff' }}
                  />
                </Col>
                <Col span={6}>
                  <Statistic
                    title="通过数"
                    value={latestReportData.summary?.passed_count || 0}
                    valueStyle={{ color: '#52c41a' }}
                    prefix={<CheckCircleOutlined />}
                  />
                </Col>
                <Col span={6}>
                  <Statistic
                    title="失败数"
                    value={latestReportData.summary?.failed_count || 0}
                    valueStyle={{ color: '#ff4d4f' }}
                    prefix={<CloseCircleOutlined />}
                  />
                </Col>
                <Col span={6}>
                  <Statistic
                    title="未执行"
                    value={latestReportData.summary?.not_executed || 0}
                    valueStyle={{ color: '#8c8c8c' }}
                    prefix={<MinusCircleOutlined />}
                  />
                </Col>
              </Row>
            </Card>

            {/* 用例执行详情列表 */}
            <Card title="测试用例执行详情">
              <List
                dataSource={latestReportData.executions || []}
                renderItem={(item: any, index: number) => (
                  <List.Item
                    actions={
                      (item.status === 'failed' || item.status === 'not_executed') && item.test_case?.id ? [
                        <Button
                          key="execute"
                          type="link"
                          size="small"
                          icon={<PlayCircleOutlined />}
                          onClick={() => handleExecuteTestCase(item.test_case.id, item.test_case.name)}
                        >
                          执行测试
                        </Button>
                      ] : undefined
                    }
                  >
                    <List.Item.Meta
                      avatar={
                        item.status === 'passed' ? (
                          <CheckCircleOutlined style={{ fontSize: 24, color: '#52c41a' }} />
                        ) : item.status === 'failed' ? (
                          <CloseCircleOutlined style={{ fontSize: 24, color: '#ff4d4f' }} />
                        ) : item.status === 'not_executed' ? (
                          <MinusCircleOutlined style={{ fontSize: 24, color: '#8c8c8c' }} />
                        ) : (
                          <ClockCircleOutlined style={{ fontSize: 24, color: '#faad14' }} />
                        )
                      }
                      title={
                        <Space>
                          <Text strong>{index + 1}. {item.test_case?.name || '未知用例'}</Text>
                          {item.test_case?.environment?.name && (
                            <Tag color="blue">{item.test_case.environment.name}</Tag>
                          )}
                          {item.test_case?.device?.name && (
                            <Tag color="green">{item.test_case.device.name}</Tag>
                          )}
                        </Space>
                      }
                      description={
                        <div>
                          <div>
                            状态: <Tag color={
                              item.status === 'passed' ? 'success' :
                              item.status === 'failed' ? 'error' :
                              item.status === 'not_executed' ? 'default' :
                              'warning'
                            }>
                              {item.status === 'passed' ? '通过' :
                               item.status === 'failed' ? '失败' :
                               item.status === 'not_executed' ? '未执行' :
                               item.status}
                            </Tag>
                            {item.duration > 0 && (
                              <span style={{ marginLeft: 16 }}>
                                耗时: {(item.duration / 1000).toFixed(2)}秒
                              </span>
                            )}
                          </div>
                          {item.error_message && (
                            <Alert
                              message={item.error_message}
                              type="error"
                              showIcon
                              style={{ marginTop: 8 }}
                            />
                          )}
                          {item.start_time && dayjs(item.start_time).isValid() && dayjs(item.start_time).year() > 1900 && (
                            <div style={{ marginTop: 8, color: '#8c8c8c', fontSize: 12 }}>
                              执行时间: {dayjs(item.start_time).format('YYYY-MM-DD HH:mm:ss')}
                            </div>
                          )}
                        </div>
                      }
                    />
                  </List.Item>
                )}
              />
            </Card>

          </div>
        ) : (
          <Alert
            message="暂无数据"
            description="无法获取最新报告数据"
            type="warning"
            showIcon
          />
        )}
      </Modal>
    </div>
  );
};

export default TestSuites;