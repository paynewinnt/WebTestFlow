import React, { useEffect, useState } from 'react';
import { Card, Row, Col, Statistic, Table, Tag, Progress, Space, Typography } from 'antd';
import dayjs from 'dayjs';
import {
  ProjectOutlined,
  ExperimentOutlined,
  CheckCircleOutlined,
  PlayCircleOutlined,
  BarChartOutlined,
} from '@ant-design/icons';
import { api } from '../../services/api';
import type { TestExecution, TestReport } from '../../types';

const { Title } = Typography;

const Dashboard: React.FC = () => {
  const [loading, setLoading] = useState(true);
  const [statistics, setStatistics] = useState({
    totalProjects: 0,
    totalTestCases: 0,
    totalExecutions: 0,
    successRate: 0,
  });
  const [recentExecutions, setRecentExecutions] = useState<TestExecution[]>([]);
  const [recentReports, setRecentReports] = useState<TestReport[]>([]);

  useEffect(() => {
    loadDashboardData();
  }, []);

  const loadDashboardData = async () => {
    setLoading(true);
    try {
      // Load statistics
      const [projectsData, testCasesData, executionsData, reportsData, statsData] = await Promise.all([
        api.getProjects({ page: 1, page_size: 1 }),
        api.getTestCases({ page: 1, page_size: 1 }),
        api.getExecutions({ page: 1, page_size: 10 }),
        api.getReports({ page: 1, page_size: 5 }),
        api.getExecutionStatistics(),
      ]);

      setStatistics({
        totalProjects: projectsData.total,
        totalTestCases: testCasesData.total,
        totalExecutions: executionsData.total,
        successRate: Math.round(statsData.success_rate),
      });

      setRecentExecutions(executionsData.list);
      setRecentReports(reportsData.list);
    } catch (error) {
      console.error('Failed to load dashboard data:', error);
    } finally {
      setLoading(false);
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'passed':
        return 'success';
      case 'failed':
        return 'error';
      case 'running':
        return 'processing';
      case 'pending':
        return 'default';
      default:
        return 'warning';
    }
  };

  const getStatusText = (status: string) => {
    switch (status) {
      case 'passed':
        return '通过';
      case 'failed':
        return '失败';
      case 'running':
        return '运行中';
      case 'pending':
        return '等待中';
      case 'error':
        return '错误';
      default:
        return status;
    }
  };

  const executionColumns = [
    {
      title: '测试用例',
      dataIndex: ['test_case', 'name'],
      key: 'test_case_name',
      render: (text: string) => text || '未知用例',
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => (
        <Tag color={getStatusColor(status)}>
          {getStatusText(status)}
        </Tag>
      ),
    },
    {
      title: '耗时',
      dataIndex: 'duration',
      key: 'duration',
      render: (duration: number) => `${(duration / 1000).toFixed(3)}s`,
    },
    {
      title: '开始时间',
      dataIndex: 'start_time',
      key: 'start_time',
      render: (time: string) => dayjs(time).format('YYYY/M/D HH:mm:ss'),
    },
  ];

  const reportColumns = [
    {
      title: '报告名称',
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: '总用例',
      dataIndex: 'total_cases',
      key: 'total_cases',
    },
    {
      title: '通过率',
      key: 'success_rate',
      render: (_: any, record: TestReport) => {
        const rate = record.total_cases > 0 
          ? Math.round((record.passed_cases / record.total_cases) * 100) 
          : 0;
        return <Progress percent={rate} size="small" />;
      },
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
      render: (time: string) => dayjs(time).format('YYYY/M/D HH:mm:ss'),
    },
  ];

  return (
    <div>
      <div style={{
        display: 'flex',
        alignItems: 'center',
        marginBottom: '24px',
        padding: '20px 0',
        borderBottom: '1px solid #f0f0f0'
      }}>
        <div style={{
          width: '48px',
          height: '48px',
          background: 'linear-gradient(135deg, #667eea, #764ba2)',
          borderRadius: '12px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          marginRight: '16px',
          boxShadow: '0 4px 12px rgba(102, 126, 234, 0.3)'
        }}>
          <svg width="28" height="28" viewBox="0 0 32 32" fill="none">
            <path d="M 6 10 L 9 22 L 12 14 L 16 22 L 20 14 L 23 22 L 26 10" 
                  stroke="#ffffff" strokeWidth="2.5" fill="none" strokeLinecap="round" strokeLinejoin="round"/>
            <circle cx="6" cy="10" r="1.5" fill="#ffffff"/>
            <circle cx="12" cy="14" r="1" fill="#ffffff" opacity="0.9"/>
            <circle cx="20" cy="14" r="1" fill="#ffffff" opacity="0.9"/>
            <circle cx="26" cy="10" r="1.5" fill="#ffffff"/>
          </svg>
        </div>
        <div>
          <Title level={2} style={{ 
            margin: 0, 
            background: 'linear-gradient(135deg, #667eea, #764ba2)',
            WebkitBackgroundClip: 'text',
            WebkitTextFillColor: 'transparent',
            backgroundClip: 'text',
            fontSize: '28px',
            fontWeight: '700'
          }}>
            WebTestFlow
          </Title>
          <div style={{ 
            color: '#8c8c8c', 
            fontSize: '14px', 
            marginTop: '4px',
            fontWeight: '500'
          }}>
            Web自动化测试平台仪表板
          </div>
        </div>
      </div>
      
      <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
        <Col xs={24} sm={12} md={6}>
          <Card>
            <Statistic
              title="项目总数"
              value={statistics.totalProjects}
              prefix={<ProjectOutlined />}
              valueStyle={{ color: '#1890ff' }}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} md={6}>
          <Card>
            <Statistic
              title="测试用例"
              value={statistics.totalTestCases}
              prefix={<ExperimentOutlined />}
              valueStyle={{ color: '#52c41a' }}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} md={6}>
          <Card>
            <Statistic
              title="执行次数"
              value={statistics.totalExecutions}
              prefix={<PlayCircleOutlined />}
              valueStyle={{ color: '#722ed1' }}
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} md={6}>
          <Card>
            <Statistic
              title="成功率"
              value={statistics.successRate}
              suffix="%"
              prefix={<CheckCircleOutlined />}
              valueStyle={{ 
                color: statistics.successRate >= 80 ? '#52c41a' : 
                       statistics.successRate >= 60 ? '#faad14' : '#f5222d' 
              }}
            />
          </Card>
        </Col>
      </Row>

      <Row gutter={[16, 16]}>
        <Col xs={24} lg={12}>
          <Card 
            title={
              <Space>
                <PlayCircleOutlined />
                最近执行记录
              </Space>
            }
            loading={loading}
          >
            <Table
              dataSource={recentExecutions}
              columns={executionColumns}
              pagination={false}
              size="small"
              rowKey="id"
              scroll={{ x: true }}
            />
          </Card>
        </Col>
        
        <Col xs={24} lg={12}>
          <Card 
            title={
              <Space>
                <BarChartOutlined />
                最近测试报告
              </Space>
            }
            loading={loading}
          >
            <Table
              dataSource={recentReports}
              columns={reportColumns}
              pagination={false}
              size="small"
              rowKey="id"
              scroll={{ x: true }}
            />
          </Card>
        </Col>
      </Row>
    </div>
  );
};

export default Dashboard;