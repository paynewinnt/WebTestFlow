import React, { useState, useEffect } from 'react';
import {
  Card,
  Table,
  Button,
  Modal,
  Form,
  Input,
  InputNumber,
  Select,
  message,
  Popconfirm,
  Space,
  Tag,
  Switch,
} from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  MobileOutlined,
  DesktopOutlined,
  TabletOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import { api } from '../../services/api';
import type { Device } from '../../types';

const { TextArea } = Input;

const DeviceManagement: React.FC = () => {
  const [devices, setDevices] = useState<Device[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [editingDevice, setEditingDevice] = useState<Device | null>(null);
  const [form] = Form.useForm();

  useEffect(() => {
    fetchDevices();
  }, []);

  const fetchDevices = async () => {
    setLoading(true);
    try {
      const data = await api.getDevices();
      setDevices(data);
    } catch (error) {
      message.error('获取设备列表失败');
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = () => {
    setEditingDevice(null);
    form.resetFields();
    setModalVisible(true);
  };

  const handleEdit = (record: Device) => {
    setEditingDevice(record);
    form.setFieldsValue(record);
    setModalVisible(true);
  };

  const handleDelete = async (id: number) => {
    try {
      await api.deleteDevice(id);
      message.success('删除成功');
      fetchDevices();
    } catch (error) {
      message.error('删除失败');
    }
  };

  const handleSubmit = async (values: any) => {
    try {
      if (editingDevice) {
        await api.updateDevice(editingDevice.id, values);
        message.success('更新成功');
      } else {
        await api.createDevice(values);
        message.success('创建成功');
      }
      
      setModalVisible(false);
      fetchDevices();
    } catch (error) {
      message.error(editingDevice ? '更新失败' : '创建失败');
    }
  };

  const getDeviceIcon = (device: Device) => {
    if (device.width <= 480) {
      return <MobileOutlined style={{ color: '#1890ff' }} />;
    } else if (device.width <= 1024) {
      return <TabletOutlined style={{ color: '#52c41a' }} />;
    } else {
      return <DesktopOutlined style={{ color: '#722ed1' }} />;
    }
  };

  const getDeviceType = (device: Device) => {
    if (device.width <= 480) {
      return { text: '手机', color: 'blue' };
    } else if (device.width <= 1024) {
      return { text: '平板', color: 'green' };
    } else {
      return { text: '桌面', color: 'purple' };
    }
  };

  const presetDevices = [
    {
      name: 'iPhone 12 Pro',
      width: 390,
      height: 844,
      user_agent: 'Mozilla/5.0 (iPhone; CPU iPhone OS 14_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0 Mobile/15E148 Safari/604.1',
    },
    {
      name: 'iPhone 14 Pro Max',
      width: 430,
      height: 932,
      user_agent: 'Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Mobile/15E148 Safari/604.1',
    },
    {
      name: 'Samsung Galaxy S21',
      width: 360,
      height: 800,
      user_agent: 'Mozilla/5.0 (Linux; Android 11; SM-G991B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/87.0.4280.141 Mobile Safari/537.36',
    },
    {
      name: 'iPad Pro',
      width: 1024,
      height: 1366,
      user_agent: 'Mozilla/5.0 (iPad; CPU OS 14_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0 Mobile/15E148 Safari/604.1',
    },
    {
      name: 'Desktop 1920x1080',
      width: 1920,
      height: 1080,
      user_agent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36',
    },
    {
      name: 'Desktop 1366x768',
      width: 1366,
      height: 768,
      user_agent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36',
    },
  ];

  const handlePresetSelect = (preset: any) => {
    form.setFieldsValue(preset);
  };

  const columns: ColumnsType<Device> = [
    {
      title: 'ID',
      dataIndex: 'id',
      key: 'id',
      width: 60,
    },
    {
      title: '设备信息',
      key: 'deviceInfo',
      render: (_, record: Device) => {
        const deviceType = getDeviceType(record);
        return (
          <Space>
            {getDeviceIcon(record)}
            <div>
              <div><strong>{record.name}</strong></div>
              <div style={{ color: '#666', fontSize: '12px' }}>
                {record.width} × {record.height}
              </div>
            </div>
            <Tag color={deviceType.color}>{deviceType.text}</Tag>
          </Space>
        );
      },
    },
    {
      title: '设备名称',
      dataIndex: 'name',
      key: 'name',
      render: (text: string) => <strong>{text}</strong>,
    },
    {
      title: '分辨率',
      key: 'resolution',
      render: (_, record: Device) => (
        <span>{record.width} × {record.height}</span>
      ),
    },
    {
      title: 'User Agent',
      dataIndex: 'user_agent',
      key: 'user_agent',
      ellipsis: true,
      width: 200,
    },
    {
      title: '默认设备',
      dataIndex: 'is_default',
      key: 'is_default',
      render: (isDefault: boolean) => (
        <Tag color={isDefault ? 'gold' : 'default'}>
          {isDefault ? '默认' : '普通'}
        </Tag>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status: number) => (
        <Tag color={status === 1 ? 'green' : 'red'}>
          {status === 1 ? '启用' : '禁用'}
        </Tag>
      ),
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
      render: (text: string) => new Date(text).toLocaleString(),
    },
    {
      title: '操作',
      key: 'action',
      fixed: 'right',
      width: 150,
      render: (_, record: Device) => (
        <Space>
          <Button
            type="text"
            icon={<EditOutlined />}
            onClick={() => handleEdit(record)}
          >
            编辑
          </Button>
          <Popconfirm
            title="确定要删除这个设备吗？"
            onConfirm={() => handleDelete(record.id)}
            okText="确定"
            cancelText="取消"
          >
            <Button
              type="text"
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
      <Card
        title={
          <Space>
            <MobileOutlined />
            设备管理
          </Space>
        }
        extra={
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={handleCreate}
          >
            新建设备
          </Button>
        }
      >
        <Table
          columns={columns}
          dataSource={devices}
          rowKey="id"
          loading={loading}
          pagination={{
            showSizeChanger: true,
            showQuickJumper: true,
            showTotal: (total) => `共 ${total} 条记录`,
          }}
          scroll={{ x: 1200 }}
        />
      </Card>

      <Modal
        title={editingDevice ? '编辑设备' : '新建设备'}
        open={modalVisible}
        onCancel={() => setModalVisible(false)}
        footer={null}
        width={700}
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={handleSubmit}
          initialValues={{
            status: 1,
            is_default: false,
          }}
        >
          {!editingDevice && (
            <Form.Item label="常用设备预设">
              <Space wrap>
                {presetDevices.map((preset, index) => (
                  <Button
                    key={index}
                    size="small"
                    onClick={() => handlePresetSelect(preset)}
                  >
                    {preset.name}
                  </Button>
                ))}
              </Space>
            </Form.Item>
          )}

          <Form.Item
            name="name"
            label="设备名称"
            rules={[
              { required: true, message: '请输入设备名称' },
              { max: 100, message: '设备名称不能超过100个字符' },
            ]}
          >
            <Input placeholder="请输入设备名称" />
          </Form.Item>

          <Space.Compact style={{ width: '100%' }}>
            <Form.Item
              name="width"
              label="宽度"
              style={{ width: '50%' }}
              rules={[
                { required: true, message: '请输入宽度' },
                { type: 'number', min: 1, max: 4000, message: '宽度必须在1-4000之间' },
              ]}
            >
              <InputNumber
                placeholder="宽度"
                style={{ width: '100%' }}
                addonAfter="px"
              />
            </Form.Item>

            <Form.Item
              name="height"
              label="高度"
              style={{ width: '50%' }}
              rules={[
                { required: true, message: '请输入高度' },
                { type: 'number', min: 1, max: 4000, message: '高度必须在1-4000之间' },
              ]}
            >
              <InputNumber
                placeholder="高度"
                style={{ width: '100%' }}
                addonAfter="px"
              />
            </Form.Item>
          </Space.Compact>

          <Form.Item
            name="user_agent"
            label="User Agent"
            rules={[
              { required: true, message: '请输入User Agent' },
              { max: 500, message: 'User Agent不能超过500个字符' },
            ]}
          >
            <TextArea
              rows={4}
              placeholder="Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36..."
            />
          </Form.Item>

          <Form.Item
            name="is_default"
            label="设为默认设备"
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>

          <Form.Item
            name="status"
            label="状态"
            rules={[{ required: true, message: '请选择状态' }]}
          >
            <Select>
              <Select.Option value={1}>启用</Select.Option>
              <Select.Option value={0}>禁用</Select.Option>
            </Select>
          </Form.Item>

          <Form.Item>
            <Space>
              <Button type="primary" htmlType="submit">
                {editingDevice ? '更新' : '创建'}
              </Button>
              <Button onClick={() => setModalVisible(false)}>
                取消
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default DeviceManagement;