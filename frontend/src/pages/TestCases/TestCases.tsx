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
  Timeline,
  Badge,
  Row,
  Col,
  Statistic,
} from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  PlayCircleOutlined,
  EyeOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import { api } from '../../services/api';
import type { TestCase, Project, Environment, Device } from '../../types';
import type { ColumnsType } from 'antd/es/table';

const { Title, Text } = Typography;
const { TextArea } = Input;
const { Option } = Select;

const TestCases: React.FC = () => {
  const [testCases, setTestCases] = useState<TestCase[]>([]);
  const [projects, setProjects] = useState<Project[]>([]);
  const [environments, setEnvironments] = useState<Environment[]>([]);
  const [devices, setDevices] = useState<Device[]>([]);
  const [loading, setLoading] = useState(false);
  const [isModalVisible, setIsModalVisible] = useState(false);
  const [isDetailDrawerVisible, setIsDetailDrawerVisible] = useState(false);
  const [isExecuteModalVisible, setIsExecuteModalVisible] = useState(false);
  const [editingTestCase, setEditingTestCase] = useState<TestCase | null>(null);
  const [selectedTestCase, setSelectedTestCase] = useState<TestCase | null>(null);
  const [executingTestCase, setExecutingTestCase] = useState<TestCase | null>(null);
  const [form] = Form.useForm();
  const [pagination, setPagination] = useState({
    current: 1,
    pageSize: 10,
    total: 0,
  });

  useEffect(() => {
    loadTestCases();
    loadInitialData();
  }, [pagination.current, pagination.pageSize]);

  const loadInitialData = async () => {
    try {
      const [projectsData, environmentsData, devicesData] = await Promise.all([
        api.getProjects({ page: 1, page_size: 100 }),
        api.getEnvironments(),
        api.getDevices(),
      ]);
      setProjects(projectsData.list);
      setEnvironments(environmentsData);
      setDevices(devicesData);
    } catch (error) {
      console.error('Failed to load initial data:', error);
    }
  };

  const loadTestCases = async () => {
    setLoading(true);
    try {
      const response = await api.getTestCases({
        page: pagination.current,
        page_size: pagination.pageSize,
      });
      setTestCases(response.list);
      setPagination(prev => ({
        ...prev,
        total: response.total,
      }));
    } catch (error) {
      console.error('Failed to load test cases:', error);
      message.error('获取测试用例失败');
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = () => {
    setEditingTestCase(null);
    setIsModalVisible(true);
    form.resetFields();
  };

  const handleEdit = (testCase: TestCase) => {
    setEditingTestCase(testCase);
    setIsModalVisible(true);
    form.setFieldsValue({
      name: testCase.name,
      description: testCase.description,
      project_id: testCase.project_id,
      environment_id: testCase.environment_id,
      device_id: testCase.device_id,
      expected_result: testCase.expected_result,
      tags: testCase.tags,
      priority: testCase.priority,
    });
  };

  const handleDelete = async (id: number) => {
    try {
      await api.deleteTestCase(id);
      message.success('删除成功');
      loadTestCases();
    } catch (error) {
      console.error('Failed to delete test case:', error);
      message.error('删除失败');
    }
  };

  const handleExecute = (testCase: TestCase) => {
    setExecutingTestCase(testCase);
    setIsExecuteModalVisible(true);
  };

  const handleConfirmExecute = async (isVisual: boolean) => {
    if (!executingTestCase) return;
    
    try {
      const response = await api.executeTestCase(executingTestCase.id, { is_visual: isVisual });
      message.success(`测试执行已启动（${isVisual ? '可视化' : '后台'}模式）`);
      console.log('Execution started:', response);
      setIsExecuteModalVisible(false);
      setExecutingTestCase(null);
    } catch (error) {
      console.error('Failed to execute test case:', error);
      message.error('执行测试失败');
    }
  };

  const handleViewDetails = (testCase: TestCase) => {
    setSelectedTestCase(testCase);
    setIsDetailDrawerVisible(true);
  };

  const handleSave = async (values: any) => {
    try {
      if (editingTestCase) {
        await api.updateTestCase(editingTestCase.id, values);
        message.success('更新成功');
      } else {
        await api.createTestCase(values);
        message.success('创建成功');
      }
      setIsModalVisible(false);
      loadTestCases();
    } catch (error) {
      console.error('Failed to save test case:', error);
      message.error('保存失败');
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

  const renderSteps = (steps: string) => {
    try {
      const stepArray = JSON.parse(steps);
      return (
        <Timeline style={{ fontSize: '12px' }}>
          {stepArray.map((step: any, index: number) => (
            <Timeline.Item key={index}>
              <div>
                <Badge color={step.type === 'click' ? 'blue' : 'green'} text={step.type} />
                <div style={{ marginLeft: 16, marginTop: 4 }}>
                  <Text code>{step.selector}</Text>
                  {step.value && <Text style={{ marginLeft: 8 }}>值: {step.value}</Text>}
                </div>
              </div>
            </Timeline.Item>
          ))}
        </Timeline>
      );
    } catch {
      return <Text type="secondary">步骤数据格式错误</Text>;
    }
  };

  const columns: ColumnsType<TestCase> = [
    {
      title: '测试用例名称',
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
      dataIndex: ['environment', 'name'],
      key: 'environment',
      width: 100,
    },
    {
      title: '设备',
      dataIndex: ['device', 'name'],
      key: 'device',
      width: 120,
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
      title: '创建者',
      dataIndex: ['user', 'username'],
      key: 'user',
      width: 100,
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 150,
      render: (date: string) => new Date(date).toLocaleString(),
    },
    {
      title: '操作',
      key: 'action',
      width: 200,
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
          <Popconfirm
            title="确定删除这个测试用例吗？"
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
      <Title level={2}>测试用例管理</Title>
      
      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={6}>
          <Card>
            <Statistic title="总计" value={pagination.total} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="启用"
              value={testCases.filter(tc => tc.status === 1).length}
              valueStyle={{ color: '#3f8600' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="禁用"
              value={testCases.filter(tc => tc.status === 0).length}
              valueStyle={{ color: '#cf1322' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="高优先级"
              value={testCases.filter(tc => tc.priority === 3).length}
              valueStyle={{ color: '#fa541c' }}
            />
          </Card>
        </Col>
      </Row>

      <Card>
        <div style={{ marginBottom: 16 }}>
          <Space>
            <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
              新建测试用例
            </Button>
            <Button icon={<ReloadOutlined />} onClick={loadTestCases}>
              刷新
            </Button>
          </Space>
        </div>

        <Table
          columns={columns}
          dataSource={testCases}
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
        title={editingTestCase ? '编辑测试用例' : '新建测试用例'}
        open={isModalVisible}
        onCancel={() => setIsModalVisible(false)}
        onOk={() => form.submit()}
        width={600}
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={handleSave}
        >
          <Form.Item
            name="name"
            label="测试用例名称"
            rules={[
              { required: true, message: '请输入测试用例名称' },
              { min: 1, max: 200, message: '名称长度为1-200个字符' },
            ]}
          >
            <Input placeholder="请输入测试用例名称" />
          </Form.Item>

          <Form.Item
            name="description"
            label="描述"
            rules={[{ max: 1000, message: '描述不能超过1000个字符' }]}
          >
            <TextArea rows={3} placeholder="请输入测试用例描述" />
          </Form.Item>

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

          <Form.Item
            name="environment_id"
            label="测试环境"
            rules={[{ required: true, message: '请选择环境' }]}
          >
            <Select placeholder="请选择环境">
              {environments.map(env => (
                <Option key={env.id} value={env.id}>
                  {env.name} ({env.type})
                </Option>
              ))}
            </Select>
          </Form.Item>

          <Form.Item
            name="device_id"
            label="测试设备"
            rules={[{ required: true, message: '请选择设备' }]}
          >
            <Select placeholder="请选择设备">
              {devices.map(device => (
                <Option key={device.id} value={device.id}>
                  {device.name} ({device.width}x{device.height})
                </Option>
              ))}
            </Select>
          </Form.Item>


          <Form.Item
            name="expected_result"
            label="预期结果"
            rules={[{ max: 1000, message: '预期结果不能超过1000个字符' }]}
          >
            <TextArea rows={3} placeholder="请输入预期结果" />
          </Form.Item>

          <Form.Item name="tags" label="标签">
            <Input placeholder="请输入标签，多个标签用逗号分隔" />
          </Form.Item>

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
        </Form>
      </Modal>

      <Drawer
        title="测试用例详情"
        placement="right"
        size="large"
        onClose={() => setIsDetailDrawerVisible(false)}
        open={isDetailDrawerVisible}
      >
        {selectedTestCase && (
          <div>
            <Descriptions title="基本信息" bordered column={1} size="small">
              <Descriptions.Item label="名称">
                {selectedTestCase.name}
              </Descriptions.Item>
              <Descriptions.Item label="描述">
                {selectedTestCase.description || '无'}
              </Descriptions.Item>
              <Descriptions.Item label="所属项目">
                {selectedTestCase.project?.name}
              </Descriptions.Item>
              <Descriptions.Item label="测试环境">
                {selectedTestCase.environment?.name} ({selectedTestCase.environment?.type})
              </Descriptions.Item>
              <Descriptions.Item label="测试设备">
                {selectedTestCase.device?.name} ({selectedTestCase.device?.width}x{selectedTestCase.device?.height})
              </Descriptions.Item>
              <Descriptions.Item label="预期结果">
                {selectedTestCase.expected_result || '无'}
              </Descriptions.Item>
              <Descriptions.Item label="标签">
                {selectedTestCase.tags || '无'}
              </Descriptions.Item>
              <Descriptions.Item label="优先级">
                <Tag color={getPriorityColor(selectedTestCase.priority)}>
                  {getPriorityText(selectedTestCase.priority)}
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item label="状态">
                <Tag color={getStatusColor(selectedTestCase.status)}>
                  {getStatusText(selectedTestCase.status)}
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item label="创建者">
                {selectedTestCase.user?.username}
              </Descriptions.Item>
              <Descriptions.Item label="创建时间">
                {new Date(selectedTestCase.created_at).toLocaleString()}
              </Descriptions.Item>
              <Descriptions.Item label="更新时间">
                {new Date(selectedTestCase.updated_at).toLocaleString()}
              </Descriptions.Item>
            </Descriptions>

            <div style={{ marginTop: 24 }}>
              <Title level={4}>测试步骤</Title>
              {selectedTestCase.steps && renderSteps(selectedTestCase.steps)}
            </div>
          </div>
        )}
      </Drawer>

      <Modal
        title="选择执行模式"
        open={isExecuteModalVisible}
        onCancel={() => setIsExecuteModalVisible(false)}
        footer={null}
        width={400}
      >
        {executingTestCase && (
          <div>
            <p style={{ marginBottom: 24 }}>
              即将执行测试用例：<strong>{executingTestCase.name}</strong>
            </p>
            <div style={{ textAlign: 'center' }}>
              <Space direction="vertical" size="large" style={{ width: '100%' }}>
                <Button
                  type="primary"
                  size="large"
                  style={{ width: '100%', height: '60px' }}
                  onClick={() => handleConfirmExecute(true)}
                >
                  <div>
                    <div style={{ fontSize: '16px', fontWeight: 'bold' }}>可视化执行</div>
                    <div style={{ fontSize: '12px', color: '#666' }}>
                      浏览器界面可见，可以观察执行过程
                    </div>
                  </div>
                </Button>
                <Button
                  size="large"
                  style={{ width: '100%', height: '60px' }}
                  onClick={() => handleConfirmExecute(false)}
                >
                  <div>
                    <div style={{ fontSize: '16px', fontWeight: 'bold' }}>后台执行</div>
                    <div style={{ fontSize: '12px', color: '#666' }}>
                      浏览器后台运行，执行速度更快
                    </div>
                  </div>
                </Button>
              </Space>
            </div>
          </div>
        )}
      </Modal>
    </div>
  );
};

export default TestCases;