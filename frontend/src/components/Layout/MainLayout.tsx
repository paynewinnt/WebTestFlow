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
          background: 'rgba(255, 255, 255, 0.2)',
          margin: '16px',
          borderRadius: '6px',
        }}>
          <h1 style={{ 
            color: 'white', 
            margin: 0, 
            fontSize: collapsed ? '16px' : '18px',
            fontWeight: 'bold'
          }}>
            {collapsed ? 'AU' : 'WebTestFlow'}
          </h1>
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