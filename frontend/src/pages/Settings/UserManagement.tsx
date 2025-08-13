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
  Space,
  Tag,
  Avatar,
} from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  UserOutlined,
  TeamOutlined,
  LockOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import { api } from '../../services/api';
import type { User, PageData } from '../../types';
import { getUser } from '../../utils/auth';

const UserManagement: React.FC = () => {
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [editingUser, setEditingUser] = useState<User | null>(null);
  const [passwordModalVisible, setPasswordModalVisible] = useState(false);
  const [changingPasswordUser, setChangingPasswordUser] = useState<User | null>(null);
  const [pagination, setPagination] = useState({
    current: 1,
    pageSize: 10,
    total: 0,
  });
  const [form] = Form.useForm();
  const [passwordForm] = Form.useForm();
  
  // Check if current user is admin
  const currentUser = getUser();
  const isAdmin = currentUser?.username === 'admin';

  useEffect(() => {
    fetchUsers();
  }, [pagination.current, pagination.pageSize]);

  const fetchUsers = async () => {
    setLoading(true);
    try {
      const data: PageData<User> = await api.getUsers({
        page: pagination.current,
        page_size: pagination.pageSize,
      });
      setUsers(data.list);
      setPagination(prev => ({
        ...prev,
        total: data.total,
      }));
    } catch (error) {
      message.error('获取用户列表失败');
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = () => {
    setEditingUser(null);
    form.resetFields();
    setModalVisible(true);
  };

  const handleEdit = (record: User) => {
    setEditingUser(record);
    form.setFieldsValue({
      username: record.username,
      email: record.email,
      status: record.status,
    });
    setModalVisible(true);
  };

  const handleSubmit = async (values: any) => {
    try {
      if (editingUser) {
        // For editing, we use the profile update API
        // Note: This only updates the current user's profile
        // In a real admin interface, you'd need a separate admin API
        await api.updateProfile({
          username: values.username,
          email: values.email,
        });
        message.success('更新成功');
      } else {
        // For creating, we use the register API
        await api.register({
          username: values.username,
          email: values.email,
          password: values.password,
        });
        message.success('创建成功');
      }
      
      setModalVisible(false);
      fetchUsers();
    } catch (error) {
      message.error(editingUser ? '更新失败' : '创建失败');
    }
  };

  const handleChangePassword = (record: User) => {
    setChangingPasswordUser(record);
    passwordForm.resetFields();
    setPasswordModalVisible(true);
  };

  const handlePasswordSubmit = async (values: any) => {
    if (!changingPasswordUser) return;
    
    try {
      await api.changeUserPassword(changingPasswordUser.id, values.password);
      message.success(`用户 ${changingPasswordUser.username} 的密码已成功修改`);
      setPasswordModalVisible(false);
      setChangingPasswordUser(null);
      passwordForm.resetFields();
    } catch (error) {
      message.error('密码修改失败');
    }
  };

  const handleTableChange = (pagination: any) => {
    setPagination({
      current: pagination.current,
      pageSize: pagination.pageSize,
      total: pagination.total,
    });
  };

  const columns: ColumnsType<User> = [
    {
      title: 'ID',
      dataIndex: 'id',
      key: 'id',
      width: 60,
    },
    {
      title: '用户信息',
      key: 'userInfo',
      render: (_, record: User) => (
        <Space>
          <Avatar 
            size="default" 
            icon={<UserOutlined />} 
            src={record.avatar}
          />
          <div>
            <div><strong>{record.username}</strong></div>
            <div style={{ color: '#666', fontSize: '12px' }}>{record.email}</div>
          </div>
        </Space>
      ),
    },
    {
      title: '用户名',
      dataIndex: 'username',
      key: 'username',
      render: (text: string) => <strong>{text}</strong>,
    },
    {
      title: '邮箱',
      dataIndex: 'email',
      key: 'email',
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status: number) => (
        <Tag color={status === 1 ? 'green' : 'red'}>
          {status === 1 ? '活跃' : '禁用'}
        </Tag>
      ),
    },
    {
      title: '注册时间',
      dataIndex: 'created_at',
      key: 'created_at',
      render: (text: string) => new Date(text).toLocaleString(),
    },
    {
      title: '最后更新',
      dataIndex: 'updated_at',
      key: 'updated_at',
      render: (text: string) => new Date(text).toLocaleString(),
    },
    {
      title: '操作',
      key: 'action',
      fixed: 'right',
      width: isAdmin ? 160 : 100,
      render: (_, record: User) => (
        <Space>
          <Button
            type="text"
            icon={<EditOutlined />}
            onClick={() => handleEdit(record)}
          >
            编辑
          </Button>
          {isAdmin && (
            <Button
              type="text"
              icon={<LockOutlined />}
              onClick={() => handleChangePassword(record)}
            >
              改密码
            </Button>
          )}
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Card
        title={
          <Space>
            <TeamOutlined />
            用户管理
          </Space>
        }
        extra={
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={handleCreate}
          >
            新建用户
          </Button>
        }
      >
        <Table
          columns={columns}
          dataSource={users}
          rowKey="id"
          loading={loading}
          pagination={{
            ...pagination,
            showSizeChanger: true,
            showQuickJumper: true,
            showTotal: (total, range) => 
              `第 ${range?.[0]}-${range?.[1]} 条，共 ${total} 条记录`,
          }}
          onChange={handleTableChange}
          scroll={{ x: 1000 }}
        />
      </Card>

      {/* 编辑用户弹窗 */}
      <Modal
        title={editingUser ? '编辑用户' : '新建用户'}
        open={modalVisible}
        onCancel={() => setModalVisible(false)}
        footer={null}
        width={500}
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={handleSubmit}
          initialValues={{
            status: 1,
          }}
        >
          <Form.Item
            name="username"
            label="用户名"
            rules={[
              { required: true, message: '请输入用户名' },
              { min: 3, message: '用户名至少3个字符' },
              { max: 50, message: '用户名不能超过50个字符' },
              { 
                pattern: /^[a-zA-Z0-9_-]+$/, 
                message: '用户名只能包含字母、数字、下划线和连字符' 
              },
            ]}
          >
            <Input placeholder="请输入用户名" />
          </Form.Item>

          <Form.Item
            name="email"
            label="邮箱"
            rules={[
              { required: true, message: '请输入邮箱地址' },
              { type: 'email', message: '请输入有效的邮箱地址' },
              { max: 100, message: '邮箱地址不能超过100个字符' },
            ]}
          >
            <Input placeholder="请输入邮箱地址" />
          </Form.Item>

          {!editingUser && (
            <Form.Item
              name="password"
              label="密码"
              rules={[
                { required: true, message: '请输入密码' },
                { min: 6, message: '密码至少6个字符' },
                { max: 50, message: '密码不能超过50个字符' },
              ]}
            >
              <Input.Password placeholder="请输入密码" />
            </Form.Item>
          )}

          {!editingUser && (
            <Form.Item
              name="confirmPassword"
              label="确认密码"
              dependencies={['password']}
              rules={[
                { required: true, message: '请确认密码' },
                ({ getFieldValue }) => ({
                  validator(_, value) {
                    if (!value || getFieldValue('password') === value) {
                      return Promise.resolve();
                    }
                    return Promise.reject(new Error('两次输入的密码不一致'));
                  },
                }),
              ]}
            >
              <Input.Password placeholder="请再次输入密码" />
            </Form.Item>
          )}

          {editingUser && (
            <Form.Item
              name="status"
              label="状态"
              rules={[{ required: true, message: '请选择状态' }]}
            >
              <Select>
                <Select.Option value={1}>活跃</Select.Option>
                <Select.Option value={0}>禁用</Select.Option>
              </Select>
            </Form.Item>
          )}

          <Form.Item>
            <Space>
              <Button type="primary" htmlType="submit">
                {editingUser ? '更新' : '创建'}
              </Button>
              <Button onClick={() => setModalVisible(false)}>
                取消
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>

      {/* 修改密码弹窗 */}
      <Modal
        title={`修改用户密码 - ${changingPasswordUser?.username}`}
        open={passwordModalVisible}
        onCancel={() => {
          setPasswordModalVisible(false);
          setChangingPasswordUser(null);
          passwordForm.resetFields();
        }}
        footer={null}
        width={400}
      >
        <Form
          form={passwordForm}
          layout="vertical"
          onFinish={handlePasswordSubmit}
        >
          <div style={{ marginBottom: 16, padding: 12, backgroundColor: '#f6f6f6', borderRadius: 4 }}>
            <p style={{ margin: 0, color: '#666' }}>
              <strong>用户信息：</strong>
            </p>
            <p style={{ margin: 0 }}>
              用户名：{changingPasswordUser?.username}
            </p>
            <p style={{ margin: 0 }}>
              邮箱：{changingPasswordUser?.email}
            </p>
          </div>
          
          <Form.Item
            name="password"
            label="新密码"
            rules={[
              { required: true, message: '请输入新密码' },
              { min: 6, message: '密码至少6个字符' },
              { max: 50, message: '密码不能超过50个字符' },
            ]}
          >
            <Input.Password 
              placeholder="请输入新密码" 
              autoComplete="new-password"
            />
          </Form.Item>

          <Form.Item
            name="confirmPassword"
            label="确认新密码"
            dependencies={['password']}
            rules={[
              { required: true, message: '请确认新密码' },
              ({ getFieldValue }) => ({
                validator(_, value) {
                  if (!value || getFieldValue('password') === value) {
                    return Promise.resolve();
                  }
                  return Promise.reject(new Error('两次输入的密码不一致'));
                },
              }),
            ]}
          >
            <Input.Password 
              placeholder="请再次输入新密码" 
              autoComplete="new-password"
            />
          </Form.Item>

          <Form.Item>
            <Space>
              <Button type="primary" htmlType="submit">
                修改密码
              </Button>
              <Button onClick={() => {
                setPasswordModalVisible(false);
                setChangingPasswordUser(null);
                passwordForm.resetFields();
              }}>
                取消
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default UserManagement;