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
  List,
  InputNumber,
  Collapse,
  Tooltip,
  Dropdown,
} from 'antd';
import dayjs from 'dayjs';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  PlayCircleOutlined,
  EyeOutlined,
  ReloadOutlined,
  SaveOutlined,
  ClockCircleOutlined,
  MoreOutlined,
} from '@ant-design/icons';
import { api } from '../../services/api';
import type { TestCase, Project, Environment, Device, TestStep } from '../../types';
import type { ColumnsType } from 'antd/es/table';

const { Title, Text } = Typography;
const { TextArea } = Input;
const { Option } = Select;
const { Panel } = Collapse;

// Steps Editor Component
interface StepsEditorProps {
  visible: boolean;
  testCase: TestCase | null;
  onClose: () => void;
  onSave: (steps: any[]) => void;
}

const StepsEditor: React.FC<StepsEditorProps> = ({ visible, testCase, onClose, onSave }) => {
  const [steps, setSteps] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (testCase && visible) {
      try {
        const parsedSteps = testCase.steps ? JSON.parse(testCase.steps) : [];
        setSteps(parsedSteps);
      } catch (error) {
        console.error('Failed to parse steps:', error);
        setSteps([]);
      }
    }
  }, [testCase, visible]);

  const handleStepUpdate = (index: number, field: string, value: any) => {
    const newSteps = [...steps];
    newSteps[index] = { ...newSteps[index], [field]: value };
    setSteps(newSteps);
  };

  const handleSave = () => {
    setLoading(true);
    onSave(steps);
    setLoading(false);
  };

  const getStepTypeLabel = (type: string) => {
    const labels: Record<string, string> = {
      click: '点击',
      input: '输入',
      scroll: '滚动',
      navigate: '导航',
      keydown: '按键',
      touchstart: '触摸开始',
      touchend: '触摸结束',
      swipe: '滑动',
      change: '变更',
      submit: '提交',
      back: '返回',
    };
    return labels[type] || type;
  };

  const getStepDescription = (step: any) => {
    const { type, selector, value } = step;
    switch (type) {
      case 'click':
        return `点击元素: ${selector}`;
      case 'input':
        return `输入内容: "${value}" 到 ${selector}`;
      case 'scroll':
        return '滚动页面';
      case 'navigate':
        return `导航到: ${value}`;
      default:
        return `${getStepTypeLabel(type)}操作`;
    }
  };

  return (
    <Drawer
      title={`编辑测试步骤 - ${testCase?.name || ''}`}
      width={800}
      open={visible}
      onClose={onClose}
      extra={
        <Button type="primary" icon={<SaveOutlined />} loading={loading} onClick={handleSave}>
          保存
        </Button>
      }
    >
      <div style={{ marginBottom: 16 }}>
        <Text type="secondary">
          共 {steps.length} 个步骤，您可以为每个步骤设置执行前的等待时间
        </Text>
      </div>
      
      <Collapse ghost>
        {steps.map((step, index) => (
          <Panel
            key={index}
            header={
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', width: '100%' }}>
                <div style={{ flex: 1 }}>
                  <Badge
                    count={index + 1}
                    style={{ backgroundColor: '#1890ff', marginRight: 8 }}
                  />
                  <Tag color="blue">{getStepTypeLabel(step.type)}</Tag>
                  <Text>{getStepDescription(step)}</Text>
                </div>
                {step.wait_before > 0 && (
                  <Tooltip title={`等待 ${step.wait_before} 秒`}>
                    <ClockCircleOutlined style={{ color: '#faad14', marginLeft: 8 }} />
                  </Tooltip>
                )}
              </div>
            }
          >
            <div style={{ padding: '16px 0' }}>
              <Row gutter={16}>
                <Col span={12}>
                  <Space direction="vertical" style={{ width: '100%' }}>
                    <div>
                      <label style={{ fontWeight: 'bold', marginBottom: 4, display: 'block' }}>
                        步骤描述
                      </label>
                      <Input
                        placeholder="为此步骤添加描述（可选）"
                        value={step.description || ''}
                        onChange={(e) => handleStepUpdate(index, 'description', e.target.value)}
                      />
                    </div>
                    <div>
                      <label style={{ fontWeight: 'bold', marginBottom: 4, display: 'block' }}>
                        <ClockCircleOutlined style={{ marginRight: 4 }} />
                        执行前等待时间（秒）
                      </label>
                      <InputNumber
                        min={0}
                        max={300}
                        value={step.wait_before || 0}
                        onChange={(value) => handleStepUpdate(index, 'wait_before', value || 0)}
                        placeholder="0"
                        style={{ width: '100%' }}
                        addonAfter="秒"
                      />
                      <Text type="secondary" style={{ fontSize: '12px' }}>
                        设置大于0的值时，此步骤执行前会等待指定时间
                      </Text>
                    </div>
                  </Space>
                </Col>
                <Col span={12}>
                  <Space direction="vertical" style={{ width: '100%' }}>
                    <div>
                      <label style={{ fontWeight: 'bold', marginBottom: 4, display: 'block' }}>
                        操作类型
                      </label>
                      <Input value={getStepTypeLabel(step.type)} disabled />
                    </div>
                    <div>
                      <label style={{ fontWeight: 'bold', marginBottom: 4, display: 'block' }}>
                        元素选择器
                      </label>
                      <Input value={step.selector || '无'} disabled />
                    </div>
                    <div>
                      <label style={{ fontWeight: 'bold', marginBottom: 4, display: 'block' }}>
                        操作值
                      </label>
                      <Input value={step.value || '无'} disabled />
                    </div>
                  </Space>
                </Col>
              </Row>
            </div>
          </Panel>
        ))}
      </Collapse>
      
      {steps.length === 0 && (
        <div style={{ textAlign: 'center', padding: '40px 0', color: '#999' }}>
          该测试用例暂无步骤数据
        </div>
      )}
    </Drawer>
  );
};

const TestCases: React.FC = () => {
  const [testCases, setTestCases] = useState<TestCase[]>([]);
  const [projects, setProjects] = useState<Project[]>([]);
  const [environments, setEnvironments] = useState<Environment[]>([]);
  const [devices, setDevices] = useState<Device[]>([]);
  const [loading, setLoading] = useState(false);
  const [isModalVisible, setIsModalVisible] = useState(false);
  const [isDetailDrawerVisible, setIsDetailDrawerVisible] = useState(false);
  const [isExecuteModalVisible, setIsExecuteModalVisible] = useState(false);
  const [isStepsEditorVisible, setIsStepsEditorVisible] = useState(false);
  const [editingTestCase, setEditingTestCase] = useState<TestCase | null>(null);
  const [selectedTestCase, setSelectedTestCase] = useState<TestCase | null>(null);
  const [executingTestCase, setExecutingTestCase] = useState<TestCase | null>(null);
  const [stepsEditingTestCase, setStepsEditingTestCase] = useState<TestCase | null>(null);
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

  const handleConfirmExecute = async () => {
    if (!executingTestCase) return;
    
    try {
      const response = await api.executeTestCase(executingTestCase.id, { is_visual: true });
      message.success('测试执行已启动（可视化模式）');
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

  const handleEditSteps = (testCase: TestCase) => {
    setStepsEditingTestCase(testCase);
    setIsStepsEditorVisible(true);
  };

  const handleStepsSave = async (steps: TestStep[]) => {
    if (!stepsEditingTestCase) return;
    
    try {
      const values = {
        name: stepsEditingTestCase.name,
        description: stepsEditingTestCase.description,
        project_id: stepsEditingTestCase.project_id,
        environment_id: stepsEditingTestCase.environment_id,
        device_id: stepsEditingTestCase.device_id,
        expected_result: stepsEditingTestCase.expected_result,
        tags: stepsEditingTestCase.tags,
        priority: stepsEditingTestCase.priority,
        steps: JSON.stringify(steps)
      };
      
      await api.updateTestCase(stepsEditingTestCase.id, values);
      message.success('测试步骤更新成功');
      setIsStepsEditorVisible(false);
      loadTestCases();
    } catch (error) {
      console.error('Failed to update test steps:', error);
      message.error('更新测试步骤失败');
    }
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
      render: (date: string) => dayjs(date).format('YYYY/M/D HH:mm:ss'),
    },
    {
      title: '操作',
      key: 'action',
      width: 120,
      fixed: 'right',
      render: (_, record) => {
        const items = [
          {
            key: 'view',
            label: '查看详情',
            icon: <EyeOutlined />,
          },
          {
            key: 'execute',
            label: '执行测试',
            icon: <PlayCircleOutlined />,
          },
          {
            key: 'edit',
            label: '编辑信息',
            icon: <EditOutlined />,
          },
          {
            key: 'editSteps',
            label: '编辑步骤',
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
            case 'view':
              handleViewDetails(record);
              break;
            case 'execute':
              handleExecute(record);
              break;
            case 'edit':
              handleEdit(record);
              break;
            case 'editSteps':
              handleEditSteps(record);
              break;
            case 'delete':
              Modal.confirm({
                title: '确定删除这个测试用例吗？',
                onOk: () => handleDelete(record.id),
                okText: '确定',
                cancelText: '取消',
              });
              break;
          }
        };

        return (
          <Dropdown
            menu={{ 
              items,
              onClick: handleMenuClick
            }}
            trigger={['click']}
          >
            <Button type="text" icon={<MoreOutlined />} />
          </Dropdown>
        );
      },
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
          scroll={{ x: 1200 }}
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
                {dayjs(selectedTestCase.created_at).format('YYYY/M/D HH:mm:ss')}
              </Descriptions.Item>
              <Descriptions.Item label="更新时间">
                {dayjs(selectedTestCase.updated_at).format('YYYY/M/D HH:mm:ss')}
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
        title="确认执行测试用例"
        open={isExecuteModalVisible}
        onCancel={() => setIsExecuteModalVisible(false)}
        onOk={handleConfirmExecute}
        okText="确定执行"
        cancelText="取消"
        width={400}
      >
        {executingTestCase && (
          <div>
            <p style={{ marginBottom: 24 }}>
              即将执行测试用例：<strong>{executingTestCase.name}</strong>
            </p>
            <div style={{ padding: '16px', backgroundColor: '#f6ffed', border: '1px solid #b7eb8f', borderRadius: '6px' }}>
              <div style={{ fontSize: '16px', fontWeight: 'bold', color: '#52c41a', marginBottom: '8px' }}>可视化执行模式</div>
              <div style={{ fontSize: '14px', color: '#666' }}>
                浏览器界面可见，可以实时观察执行过程和页面交互
              </div>
            </div>
          </div>
        )}
      </Modal>

      {/* Steps Editor Drawer */}
      <StepsEditor
        visible={isStepsEditorVisible}
        testCase={stepsEditingTestCase}
        onClose={() => {
          setIsStepsEditorVisible(false);
          setStepsEditingTestCase(null);
        }}
        onSave={handleStepsSave}
      />
    </div>
  );
};

export default TestCases;