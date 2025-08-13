import React, { useEffect, useState } from 'react';
import {
  Card,
  Form,
  Input,
  Select,
  Button,
  Steps,
  Space,
  message,
  Spin,
  Typography,
  Alert,
  Divider,
  List,
  Tag,
} from 'antd';
import {
  PlayCircleOutlined,
  StopOutlined,
  SaveOutlined,
  MonitorOutlined,
} from '@ant-design/icons';
import { api } from '../../services/api';
import type { Project, Environment, Device, TestStep } from '../../types';

const { Title, Text } = Typography;
const { TextArea } = Input;
const { Step } = Steps;

const Recording: React.FC = () => {
  const [current, setCurrent] = useState(0);
  const [form] = Form.useForm();
  const [saveForm] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [isRecording, setIsRecording] = useState(false);
  const [sessionId, setSessionId] = useState<string>('');
  const [recordedSteps, setRecordedSteps] = useState<TestStep[]>([]);
  const [ws, setWs] = useState<WebSocket | null>(null);

  // Data states
  const [projects, setProjects] = useState<Project[]>([]);
  const [environments, setEnvironments] = useState<Environment[]>([]);
  const [devices, setDevices] = useState<Device[]>([]);
  const [recordingConfig, setRecordingConfig] = useState<any>(null);

  useEffect(() => {
    loadData();
    return () => {
      if (ws) {
        ws.close();
      }
    };
  }, []);

  const loadData = async () => {
    setLoading(true);
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
      console.error('Failed to load data:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleStartRecording = async (values: any) => {
    setLoading(true);
    try {
      // Save recording configuration for later use
      setRecordingConfig(values);
      
      const response = await api.startRecording({
        environment_id: values.environment_id,
        device_id: values.device_id,
      });

      setSessionId(response.session_id);
      setIsRecording(true);
      setCurrent(1);

      // Establish WebSocket connection for real-time updates
      const wsUrl = `ws://localhost:8080/api/v1/ws/recording?session_id=${response.session_id}`;
      const websocket = new WebSocket(wsUrl);
      
      websocket.onmessage = (event) => {
        try {
          const step = JSON.parse(event.data);
          if (step && typeof step === 'object') {
            setRecordedSteps(prev => (prev || []).concat([step]));
          }
        } catch (error) {
          console.error('Failed to parse WebSocket message:', error);
        }
      };

      websocket.onerror = (error) => {
        console.error('WebSocket error:', error);
        message.error('WebSocket连接失败');
      };

      setWs(websocket);
      message.success('录制已开始，请在浏览器中执行操作');
    } catch (error) {
      console.error('Failed to start recording:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleStopRecording = async () => {
    setLoading(true);
    try {
      await api.stopRecording(sessionId);
      setIsRecording(false);
      setCurrent(2);

      if (ws) {
        ws.close();
        setWs(null);
      }

      // Get final recording status
      const status = await api.getRecordingStatus(sessionId);
      setRecordedSteps(status.steps || []);

      message.success('录制已停止');
    } catch (error) {
      console.error('Failed to stop recording:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleSaveRecording = async (values: any) => {
    setLoading(true);
    try {
      if (!recordingConfig) {
        message.error('录制配置丢失，请重新录制');
        return;
      }
      
      const saveData = {
        session_id: sessionId,
        name: values.name,
        description: values.description,
        project_id: recordingConfig.project_id,
        environment_id: recordingConfig.environment_id,
        device_id: recordingConfig.device_id,
        expected_result: values.expected_result,
        tags: values.tags || '',
        priority: values.priority || 2,
      };

      await api.saveRecording(saveData);
      
      message.success('测试用例保存成功');
      
      // Reset form and state
      form.resetFields();
      saveForm.resetFields();
      setRecordedSteps([]);
      setSessionId('');
      setRecordingConfig(null);
      setCurrent(0);
      
    } catch (error) {
      console.error('Failed to save recording:', error);
    } finally {
      setLoading(false);
    }
  };

  const getStepTypeColor = (type: string) => {
    const colors: Record<string, string> = {
      click: 'blue',
      input: 'green',
      scroll: 'orange',
      keydown: 'purple',
      touchstart: 'cyan',
      change: 'magenta',
      submit: 'red',
    };
    return colors[type] || 'default';
  };

  const steps = [
    {
      title: '配置录制',
      description: '设置录制参数',
    },
    {
      title: '执行录制',
      description: '在浏览器中执行操作',
    },
    {
      title: '保存用例',
      description: '保存为测试用例',
    },
  ];

  return (
    <div>
      <Title level={2}>录制测试用例</Title>
      
      <Steps current={current} style={{ marginBottom: 24 }}>
        {steps.map((item, index) => (
          <Step key={index} title={item.title} description={item.description} />
        ))}
      </Steps>

      <Spin spinning={loading}>
        {current === 0 && (
          <Card title="录制配置">
            <Form
              form={form}
              layout="vertical"
              onFinish={handleStartRecording}
            >
              <Form.Item
                name="project_id"
                label="选择项目"
                rules={[{ required: true, message: '请选择项目' }]}
              >
                <Select placeholder="请选择项目">
                  {projects.map(project => (
                    <Select.Option key={project.id} value={project.id}>
                      {project.name}
                    </Select.Option>
                  ))}
                </Select>
              </Form.Item>

              <Form.Item
                name="environment_id"
                label="测试环境"
                rules={[{ required: true, message: '请选择测试环境' }]}
              >
                <Select placeholder="请选择测试环境">
                  {environments.map(env => (
                    <Select.Option key={env.id} value={env.id}>
                      {env.name} - {env.base_url}
                    </Select.Option>
                  ))}
                </Select>
              </Form.Item>

              <Form.Item
                name="device_id"
                label="选择设备"
                rules={[{ required: true, message: '请选择设备' }]}
              >
                <Select placeholder="请选择设备">
                  {devices.map(device => (
                    <Select.Option key={device.id} value={device.id}>
                      {device.name} ({device.width}x{device.height})
                    </Select.Option>
                  ))}
                </Select>
              </Form.Item>


              <Form.Item>
                <Button 
                  type="primary" 
                  htmlType="submit"
                  icon={<PlayCircleOutlined />}
                  size="large"
                >
                  开始录制
                </Button>
              </Form.Item>
            </Form>
          </Card>
        )}

        {current === 1 && (
          <Card title="录制中...">
            <Alert
              message="录制已启动"
              description="请在打开的浏览器窗口中执行您的操作。所有操作都会被自动记录。"
              type="info"
              showIcon
              style={{ marginBottom: 16 }}
            />

            <div style={{ marginBottom: 16 }}>
              <Text strong>已录制步骤: {recordedSteps?.length || 0}</Text>
            </div>

            {recordedSteps && recordedSteps.length > 0 && (
              <>
                <Divider>录制的操作步骤</Divider>
                <List
                  dataSource={recordedSteps}
                  renderItem={(step, index) => (
                    <List.Item>
                      <Space>
                        <Tag color={getStepTypeColor(step.type)}>
                          {step.type}
                        </Tag>
                        <Text code>{step.selector}</Text>
                        {step.value && <Text>值: {step.value}</Text>}
                        <Text type="secondary">
                          {new Date(step.timestamp).toLocaleTimeString()}
                        </Text>
                      </Space>
                    </List.Item>
                  )}
                  style={{ maxHeight: 300, overflow: 'auto' }}
                />
              </>
            )}

            <div style={{ marginTop: 16 }}>
              <Button 
                type="primary" 
                danger
                icon={<StopOutlined />}
                onClick={handleStopRecording}
                disabled={!isRecording}
                size="large"
              >
                停止录制
              </Button>
            </div>
          </Card>
        )}

        {current === 2 && (
          <Card title="保存测试用例">
            <Alert
              message={`录制完成，共记录了 ${recordedSteps?.length || 0} 个操作步骤`}
              type="success"
              showIcon
              style={{ marginBottom: 16 }}
            />

            <Form
              form={saveForm}
              layout="vertical"
              onFinish={handleSaveRecording}
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
                label="测试用例描述"
                rules={[{ max: 1000, message: '描述不能超过1000个字符' }]}
              >
                <TextArea 
                  rows={4} 
                  placeholder="请输入测试用例描述" 
                />
              </Form.Item>

              <Form.Item
                name="expected_result"
                label="预期结果"
                rules={[{ max: 1000, message: '预期结果不能超过1000个字符' }]}
              >
                <TextArea 
                  rows={3} 
                  placeholder="请输入预期结果" 
                />
              </Form.Item>

              <Form.Item
                name="tags"
                label="标签"
              >
                <Input placeholder="请输入标签，多个标签用逗号分隔" />
              </Form.Item>

              <Form.Item
                name="priority"
                label="优先级"
                initialValue={2}
              >
                <Select>
                  <Select.Option value={1}>低</Select.Option>
                  <Select.Option value={2}>中</Select.Option>
                  <Select.Option value={3}>高</Select.Option>
                </Select>
              </Form.Item>

              <Form.Item>
                <Space>
                  <Button 
                    type="primary"
                    htmlType="submit"
                    icon={<SaveOutlined />}
                    size="large"
                  >
                    保存测试用例
                  </Button>
                  <Button 
                    onClick={() => {
                      setCurrent(0);
                      setRecordedSteps([]);
                      setSessionId('');
                      setRecordingConfig(null);
                      form.resetFields();
                      saveForm.resetFields();
                    }}
                  >
                    重新录制
                  </Button>
                </Space>
              </Form.Item>
            </Form>
          </Card>
        )}
      </Spin>
    </div>
  );
};

export default Recording;