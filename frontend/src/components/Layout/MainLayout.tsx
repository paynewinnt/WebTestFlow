// @ts-ignore
import React, { useState } from 'react';
import { Layout, Menu, Avatar, Dropdown, Button, Space } from 'antd';
import {
  UserOutlined,
  ProjectOutlined,
  ExperimentOutlined,
  BarChartOutlined,
  SettingOutlined,
  LogoutOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  PlayCircleOutlined,
  DatabaseOutlined,
} from '@ant-design/icons';
import { useNavigate, useLocation } from 'react-router-dom';
import { getUser, logout } from '../../utils/auth';
import type { MenuProps } from 'antd';

const { Header, Sider, Content } = Layout;

interface MainLayoutProps {
  children: React.ReactNode;
}

const MainLayout: React.FC<MainLayoutProps> = ({ children }) => {
  const [collapsed, setCollapsed] = useState(false);
  const navigate = useNavigate();
  const location = useLocation();
  const user = getUser();

  const menuItems: MenuProps['items'] = [
    {
      key: '/dashboard',
      icon: <BarChartOutlined />,
      label: '仪表板',
    },
    {
      key: '/projects',
      icon: <ProjectOutlined />,
      label: '项目管理',
    },
    {
      key: '/test-cases',
      icon: <ExperimentOutlined />,
      label: '测试用例',
    },
    {
      key: '/test-suites',
      icon: <DatabaseOutlined />,
      label: '测试套件',
    },
    {
      key: '/recording',
      icon: <PlayCircleOutlined />,
      label: '录制用例',
    },
    {
      key: '/executions',
      icon: <PlayCircleOutlined />,
      label: '执行记录',
    },
    {
      key: '/reports',
      icon: <BarChartOutlined />,
      label: '测试报告',
    },
    {
      key: '/settings',
      icon: <SettingOutlined />,
      label: '系统设置',
      children: [
        {
          key: '/settings/environments',
          label: '环境管理',
        },
        {
          key: '/settings/devices',
          label: '设备管理',
        },
        {
          key: '/settings/users',
          label: '用户管理',
        },
      ],
    },
  ];

  const userMenuItems: MenuProps['items'] = [
    {
      key: 'profile',
      icon: <UserOutlined />,
      label: '个人信息',
    },
    {
      type: 'divider',
    },
    {
      key: 'logout',
      icon: <LogoutOutlined />,
      label: '退出登录',
      onClick: logout,
    },
  ];

  const handleMenuClick = ({ key }: { key: string }) => {
    navigate(key);
  };

  const getSelectedKeys = () => {
    const pathname = location.pathname;
    // Handle nested routes
    if (pathname.startsWith('/settings/')) {
      return [pathname];
    }
    return [pathname];
  };

  const getOpenKeys = () => {
    const pathname = location.pathname;
    if (pathname.startsWith('/settings/')) {
      return ['/settings'];
    }
    return [];
  };

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider 
        trigger={null} 
        collapsible 
        collapsed={collapsed}
        style={{
          overflow: 'auto',
          height: '100vh',
          position: 'fixed',
          left: 0,
          top: 0,
          bottom: 0,
        }}
      >
        <div style={{
          height: 64,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          background: 'linear-gradient(135deg, rgba(102, 126, 234, 0.2), rgba(118, 75, 162, 0.2))',
          margin: '16px',
          borderRadius: '8px',
          border: '1px solid rgba(255, 255, 255, 0.1)',
          backdropFilter: 'blur(10px)',
        }}>
          {!collapsed ? (
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <div style={{
                width: '32px',
                height: '32px',
                background: 'linear-gradient(135deg, #667eea, #764ba2)',
                borderRadius: '6px',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                boxShadow: '0 2px 8px rgba(0,0,0,0.2)'
              }}>
                <svg width="20" height="20" viewBox="0 0 32 32" fill="none">
                  <path d="M 6 10 L 9 22 L 12 14 L 16 22 L 20 14 L 23 22 L 26 10" 
                        stroke="#ffffff" strokeWidth="2.5" fill="none" strokeLinecap="round" strokeLinejoin="round"/>
                  <circle cx="6" cy="10" r="1.5" fill="#ffffff"/>
                  <circle cx="26" cy="10" r="1.5" fill="#ffffff"/>
                </svg>
              </div>
              <h1 style={{ 
                color: 'white', 
                margin: 0, 
                fontSize: '18px',
                fontWeight: '600',
                background: 'linear-gradient(135deg, #ffffff, #e2e8f0)',
                WebkitBackgroundClip: 'text',
                WebkitTextFillColor: 'transparent',
                backgroundClip: 'text',
                letterSpacing: '0.5px'
              }}>
                WebTestFlow
              </h1>
            </div>
          ) : (
            <div style={{
              width: '28px',
              height: '28px',
              background: 'linear-gradient(135deg, #667eea, #764ba2)',
              borderRadius: '6px',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              boxShadow: '0 2px 8px rgba(0,0,0,0.2)'
            }}>
              <svg width="16" height="16" viewBox="0 0 32 32" fill="none">
                <path d="M 8 12 L 12 20 L 16 16 L 20 20 L 24 12" 
                      stroke="#ffffff" strokeWidth="3" fill="none" strokeLinecap="round"/>
              </svg>
            </div>
          )}
        </div>
        
        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={getSelectedKeys()}
          defaultOpenKeys={getOpenKeys()}
          items={menuItems}
          onClick={handleMenuClick}
        />
      </Sider>
      
      <Layout style={{ marginLeft: collapsed ? 80 : 200, transition: 'margin-left 0.2s' }}>
        <Header style={{
          padding: '0 24px',
          background: '#fff',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          boxShadow: '0 1px 4px rgba(0,21,41,.08)',
          position: 'sticky',
          top: 0,
          zIndex: 1,
        }}>
          <Button
            type="text"
            icon={collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
            onClick={() => setCollapsed(!collapsed)}
            style={{
              fontSize: '16px',
              width: 64,
              height: 64,
            }}
          />
          
          <Space>
            <Dropdown menu={{ items: userMenuItems }} placement="bottomRight">
              <Space style={{ cursor: 'pointer' }}>
                <Avatar 
                  size="default" 
                  icon={<UserOutlined />} 
                  src={user?.avatar}
                />
                <span>{user?.username}</span>
              </Space>
            </Dropdown>
          </Space>
        </Header>
        
        <Content style={{
          margin: '24px',
          padding: '24px',
          background: '#fff',
          borderRadius: '6px',
          minHeight: 280,
        }}>
          {children}
        </Content>
      </Layout>
    </Layout>
  );
};

export default MainLayout;