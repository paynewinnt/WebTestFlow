import React, { useState, useEffect } from 'react';
import {
  Card,
  Table,
  Button,
  Modal,
  Form,
  Input,
  Select,
  message,
  Popconfirm,
  Space,
  Tag,
} from 'antd';
import dayjs from 'dayjs';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  SettingOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import { api } from '../../services/api';
import type { Environment } from '../../types';

const { TextArea } = Input;

const EnvironmentManagement: React.FC = () => {
  const [environments, setEnvironments] = useState<Environment[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [editingEnvironment, setEditingEnvironment] = useState<Environment | null>(null);
  const [form] = Form.useForm();

  useEffect(() => {
    fetchEnvironments();
  }, []);

  const fetchEnvironments = async () => {
    setLoading(true);
    try {
      const data = await api.getEnvironments();
      setEnvironments(data);
    } catch (error) {
      message.error('获取环境列表失败');
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = () => {
    setEditingEnvironment(null);
    form.resetFields();
    setModalVisible(true);
  };

  const handleEdit = (record: Environment) => {
    setEditingEnvironment(record);
    form.setFieldsValue({
      ...record,
      headers: record.headers || '{}',
      variables: record.variables || '{}',
    });
    setModalVisible(true);
  };

  const handleDelete = async (id: number) => {
    try {
      await api.deleteEnvironment(id);
      message.success('删除成功');
      fetchEnvironments();
    } catch (error) {
      message.error('删除失败');
    }
  };

  const handleSubmit = async (values: any) => {
    try {
      // Validate JSON format
      try {
        JSON.parse(values.headers || '{}');
        JSON.parse(values.variables || '{}');
      } catch (e) {
        message.error('Headers 或 Variables 格式不正确，请输入有效的 JSON');
        return;
      }

      if (editingEnvironment) {
        await api.updateEnvironment(editingEnvironment.id, values);
        message.success('更新成功');
      } else {
        await api.createEnvironment(values);
        message.success('创建成功');
      }
      
      setModalVisible(false);
      fetchEnvironments();
    } catch (error) {
      message.error(editingEnvironment ? '更新失败' : '创建失败');
    }
  };

  const columns: ColumnsType<Environment> = [
    {
      title: 'ID',
      dataIndex: 'id',
      key: 'id',
      width: 60,
    },
    {
      title: '环境名称',
      dataIndex: 'name',
      key: 'name',
      render: (text: string, record: Environment) => (
        <Space>
          <strong>{text}</strong>
          {record.type === 'test' && <Tag color="blue">测试</Tag>}
          {record.type === 'product' && <Tag color="red">生产</Tag>}
        </Space>
      ),
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
    },
    {
      title: '基础URL',
      dataIndex: 'base_url',
      key: 'base_url',
      ellipsis: true,
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
      render: (text: string) => dayjs(text).format('YYYY/M/D HH:mm:ss'),
    },
    {
      title: '操作',
      key: 'action',
      render: (_, record: Environment) => (
        <Space size="small">
          <Button
            type="link"
            size="small"
            icon={<EditOutlined />}
            onClick={() => handleEdit(record)}
          >
            编辑
          </Button>
          <Popconfirm
            title="确定要删除这个环境吗？"
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
      <Card
        title={
          <Space>
            <SettingOutlined />
            环境管理
          </Space>
        }
      >
        <div style={{ marginBottom: 16 }}>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={handleCreate}
          >
            新建环境
          </Button>
        </div>
        <Table
          columns={columns}
          dataSource={environments}
          rowKey="id"
          loading={loading}
          pagination={{
            showSizeChanger: true,
            showQuickJumper: true,
            showTotal: (total) => `共 ${total} 条记录`,
          }}
          scroll={{ x: 1000 }}
        />
      </Card>

      <Modal
        title={editingEnvironment ? '编辑环境' : '新建环境'}
        open={modalVisible}
        onCancel={() => setModalVisible(false)}
        footer={null}
        width={600}
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={handleSubmit}
          initialValues={{
            type: 'test',
            status: 1,
            headers: '{"Content-Type": "application/json"}',
            variables: '{"timeout": 30000}',
          }}
        >
          <Form.Item
            name="name"
            label="环境名称"
            rules={[
              { required: true, message: '请输入环境名称' },
              { max: 100, message: '环境名称不能超过100个字符' },
            ]}
          >
            <Input placeholder="请输入环境名称" />
          </Form.Item>

          <Form.Item
            name="description"
            label="描述"
            rules={[{ max: 500, message: '描述不能超过500个字符' }]}
          >
            <TextArea rows={3} placeholder="请输入环境描述" />
          </Form.Item>

          <Form.Item
            name="base_url"
            label="基础URL"
            rules={[
              { required: true, message: '请输入基础URL' },
              { type: 'url', message: '请输入有效的URL' },
              { max: 500, message: 'URL不能超过500个字符' },
            ]}
          >
            <Input placeholder="https://example.com" />
          </Form.Item>

          <Form.Item
            name="type"
            label="环境类型"
            rules={[{ required: true, message: '请选择环境类型' }]}
          >
            <Select>
              <Select.Option value="test">测试环境</Select.Option>
              <Select.Option value="product">生产环境</Select.Option>
            </Select>
          </Form.Item>

          <Form.Item
            name="headers"
            label="请求头 (JSON格式)"
            rules={[
              {
                validator: (_, value) => {
                  if (!value) return Promise.resolve();
                  try {
                    JSON.parse(value);
                    return Promise.resolve();
                  } catch (e) {
                    return Promise.reject(new Error('请输入有效的JSON格式'));
                  }
                },
              },
            ]}
          >
            <TextArea
              rows={4}
              placeholder='{"Content-Type": "application/json", "Authorization": "Bearer token"}'
            />
          </Form.Item>

          <Form.Item
            name="variables"
            label="环境变量 (JSON格式)"
            rules={[
              {
                validator: (_, value) => {
                  if (!value) return Promise.resolve();
                  try {
                    JSON.parse(value);
                    return Promise.resolve();
                  } catch (e) {
                    return Promise.reject(new Error('请输入有效的JSON格式'));
                  }
                },
              },
            ]}
          >
            <TextArea
              rows={4}
              placeholder='{"timeout": 30000, "api_key": "your-api-key"}'
            />
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
                {editingEnvironment ? '更新' : '创建'}
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

export default EnvironmentManagement;