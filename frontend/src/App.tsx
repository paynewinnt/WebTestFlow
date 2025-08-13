import React from 'react';
import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom';
import { ConfigProvider } from 'antd';
import zhCN from 'antd/locale/zh_CN';
import 'antd/dist/reset.css';

import MainLayout from './components/Layout/MainLayout';
import Login from './pages/Login/Login';
import Register from './pages/Register/Register';
import Dashboard from './pages/Dashboard/Dashboard';
import Projects from './pages/Projects/Projects';
import Recording from './pages/Recording/Recording';
import TestCases from './pages/TestCases/TestCases';
import TestSuites from './pages/TestSuites/TestSuites';
import Reports from './pages/Reports/Reports';
import Executions from './pages/Executions/Executions';
import EnvironmentManagement from './pages/Settings/EnvironmentManagement';
import UserManagement from './pages/Settings/UserManagement';
import DeviceManagement from './pages/Settings/DeviceManagement';
import { isAuthenticated } from './utils/auth';

// Route guard component
const ProtectedRoute: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  return isAuthenticated() ? <>{children}</> : <Navigate to="/login" replace />;
};

const App: React.FC = () => {
  return (
    <ConfigProvider locale={zhCN}>
      <Router>
        <Routes>
          {/* Public routes */}
          <Route path="/login" element={<Login />} />
          <Route path="/register" element={<Register />} />
          
          {/* Protected routes */}
          <Route path="/" element={
            <ProtectedRoute>
              <MainLayout>
                <Navigate to="/dashboard" replace />
              </MainLayout>
            </ProtectedRoute>
          } />
          
          <Route path="/dashboard" element={
            <ProtectedRoute>
              <MainLayout>
                <Dashboard />
              </MainLayout>
            </ProtectedRoute>
          } />

          <Route path="/projects" element={
            <ProtectedRoute>
              <MainLayout>
                <Projects />
              </MainLayout>
            </ProtectedRoute>
          } />

          <Route path="/recording" element={
            <ProtectedRoute>
              <MainLayout>
                <Recording />
              </MainLayout>
            </ProtectedRoute>
          } />

          <Route path="/test-cases" element={
            <ProtectedRoute>
              <MainLayout>
                <TestCases />
              </MainLayout>
            </ProtectedRoute>
          } />

          <Route path="/test-suites" element={
            <ProtectedRoute>
              <MainLayout>
                <TestSuites />
              </MainLayout>
            </ProtectedRoute>
          } />

          <Route path="/executions" element={
            <ProtectedRoute>
              <MainLayout>
                <Executions />
              </MainLayout>
            </ProtectedRoute>
          } />

          <Route path="/reports" element={
            <ProtectedRoute>
              <MainLayout>
                <Reports />
              </MainLayout>
            </ProtectedRoute>
          } />

          {/* Settings routes */}
          <Route path="/settings/environments" element={
            <ProtectedRoute>
              <MainLayout>
                <EnvironmentManagement />
              </MainLayout>
            </ProtectedRoute>
          } />

          <Route path="/settings/users" element={
            <ProtectedRoute>
              <MainLayout>
                <UserManagement />
              </MainLayout>
            </ProtectedRoute>
          } />

          <Route path="/settings/devices" element={
            <ProtectedRoute>
              <MainLayout>
                <DeviceManagement />
              </MainLayout>
            </ProtectedRoute>
          } />
          
          {/* Fallback route */}
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </Router>
    </ConfigProvider>
  );
};

export default App;