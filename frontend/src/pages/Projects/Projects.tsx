import React, { useEffect, useState } from 'react';
import { 
  Table, 
  Button, 
  Space, 
  Modal, 
  Form, 
  Input, 
  message, 
  Popconfirm,
  Tag,
  Typography 
} from 'antd';
import { 
  PlusOutlined, 
  EditOutlined, 
  DeleteOutlined, 
  EyeOutlined 
} from '@ant-design/icons';
import { api } from '../../services/api';
import type { Project } from '../../types';

const { Title } = Typography;
const { TextArea } = Input;

const Projects: React.FC = () => {
  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(false);
  const [isModalVisible, setIsModalVisible] = useState(false);
  const [editingProject, setEditingProject] = useState<Project | null>(null);
  const [form] = Form.useForm();
  const [pagination, setPagination] = useState({
    current: 1,
    pageSize: 10,
    total: 0,
  });

  useEffect(() => {
    loadProjects();
  }, [pagination.current, pagination.pageSize]);

  const loadProjects = async () => {
    setLoading(true);
    try {
      const data = await api.getProjects({
        page: pagination.current,
        page_size: pagination.pageSize,
      });
      setProjects(data.list);
      setPagination({
        ...pagination,
        total: data.total,
      });
    } catch (error) {
      console.error('Failed to load projects:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = () => {
    setEditingProject(null);
    setIsModalVisible(true);
    form.resetFields();
  };

  const handleEdit = (project: Project) => {
    setEditingProject(project);
    setIsModalVisible(true);
    form.setFieldsValue({
      name: project.name,
      description: project.description,
    });
  };

  const handleDelete = async (project: Project) => {
    try {
      await api.deleteProject(project.id);
      message.success('删除成功');
      loadProjects();
    } catch (error) {
      console.error('Failed to delete project:', error);
    }
  };

  const handleSubmit = async (values: any) => {
    try {
      if (editingProject) {
        await api.updateProject(editingProject.id, values);
        message.success('更新成功');
      } else {
        await api.createProject(values);
        message.success('创建成功');
      }
      setIsModalVisible(false);
      loadProjects();
    } catch (error) {
      console.error('Failed to save project:', error);
    }
  };

  const columns = [
    {
      title: 'ID',
      dataIndex: 'id',
      key: 'id',
      width: 80,
    },
    {
      title: '项目名称',
      dataIndex: 'name',
      key: 'name',
      render: (text: string, record: Project) => (
        <Space>
          <span>{text}</span>
          <Tag color={record.status === 1 ? 'success' : 'default'}>
            {record.status === 1 ? '活跃' : '禁用'}
          </Tag>
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
      title: '创建者',
      dataIndex: ['user', 'username'],
      key: 'creator',
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
      render: (time: string) => new Date(time).toLocaleString(),
    },
    {
      title: '操作',
      key: 'actions',
      render: (_: any, record: Project) => (
        <Space>
          <Button 
            icon={<EyeOutlined />} 
            size="small"
            onClick={() => {/* TODO: View details */}}
          >
            查看
          </Button>
          <Button 
            icon={<EditOutlined />} 
            size="small"
            onClick={() => handleEdit(record)}
          >
            编辑
          </Button>
          <Popconfirm
            title="确定要删除这个项目吗？"
            onConfirm={() => handleDelete(record)}
            okText="确定"
            cancelText="取消"
          >
            <Button 
              icon={<DeleteOutlined />} 
              size="small" 
              danger
            >
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  const handleTableChange = (newPagination: any) => {
    setPagination({
      ...pagination,
      current: newPagination.current,
      pageSize: newPagination.pageSize,
    });
  };

  return (
    <div>
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <Title level={2}>项目管理</Title>
        <Button 
          type="primary" 
          icon={<PlusOutlined />}
          onClick={handleCreate}
        >
          新建项目
        </Button>
      </div>

      <Table
        columns={columns}
        dataSource={projects}
        rowKey="id"
        loading={loading}
        pagination={{
          ...pagination,
          showSizeChanger: true,
          showQuickJumper: true,
          showTotal: (total, range) =>
            `第 ${range[0]}-${range[1]} 条，共 ${total} 条`,
        }}
        onChange={handleTableChange}
      />

      <Modal
        title={editingProject ? '编辑项目' : '新建项目'}
        open={isModalVisible}
        onCancel={() => setIsModalVisible(false)}
        onOk={form.submit}
        okText="确定"
        cancelText="取消"
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={handleSubmit}
        >
          <Form.Item
            name="name"
            label="项目名称"
            rules={[
              { required: true, message: '请输入项目名称' },
              { min: 1, max: 100, message: '项目名称长度为1-100个字符' },
            ]}
          >
            <Input placeholder="请输入项目名称" />
          </Form.Item>

          <Form.Item
            name="description"
            label="项目描述"
            rules={[
              { max: 500, message: '项目描述不能超过500个字符' },
            ]}
          >
            <TextArea 
              rows={4} 
              placeholder="请输入项目描述" 
            />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default Projects;