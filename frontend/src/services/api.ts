import axios, { AxiosInstance, AxiosResponse } from 'axios';
import { message } from 'antd';
import type {
  ApiResponse,
  LoginRequest,
  LoginResponse,
  RegisterRequest,
  User,
  Project,
  Environment,
  Device,
  TestCase,
  TestSuite,
  TestExecution,
  TestReport,
  TestStep,
  PageData,
} from '../types';

class ApiService {
  private instance: AxiosInstance;

  constructor() {
    this.instance = axios.create({
      baseURL: '/api/v1',
      timeout: 30000,
      headers: {
        'Content-Type': 'application/json',
      },
    });

    this.setupInterceptors();
  }

  private setupInterceptors() {
    // Request interceptor
    this.instance.interceptors.request.use(
      (config) => {
        const token = localStorage.getItem('token');
        if (token) {
          config.headers.Authorization = `Bearer ${token}`;
        }
        return config;
      },
      (error) => {
        return Promise.reject(error);
      }
    );

    // Response interceptor
    this.instance.interceptors.response.use(
      (response: AxiosResponse<ApiResponse>) => {
        const { data } = response;
        if (data.code !== 200) {
          message.error(data.message || 'Request failed');
          return Promise.reject(new Error(data.message || 'Request failed'));
        }
        return response;
      },
      (error) => {
        if (error.response?.status === 401) {
          localStorage.removeItem('token');
          localStorage.removeItem('user');
          window.location.href = '/login';
        } else {
          message.error(error.response?.data?.message || 'Network error');
        }
        return Promise.reject(error);
      }
    );
  }

  // Auth APIs
  async login(data: LoginRequest): Promise<LoginResponse> {
    const response = await this.instance.post<ApiResponse<LoginResponse>>('/auth/login', data);
    return response.data.data!;
  }

  async register(data: RegisterRequest): Promise<User> {
    const response = await this.instance.post<ApiResponse<User>>('/auth/register', data);
    return response.data.data!;
  }

  // User APIs
  async getProfile(): Promise<User> {
    const response = await this.instance.get<ApiResponse<User>>('/users/profile');
    return response.data.data!;
  }

  async updateProfile(data: Partial<User>): Promise<User> {
    const response = await this.instance.put<ApiResponse<User>>('/users/profile', data);
    return response.data.data!;
  }

  async getUsers(params?: { page?: number; page_size?: number }): Promise<PageData<User>> {
    const response = await this.instance.get<ApiResponse<PageData<User>>>('/users', { params });
    return response.data.data!;
  }

  async changeUserPassword(userId: number, password: string): Promise<void> {
    await this.instance.put(`/users/${userId}/password`, { password });
  }

  // Environment APIs
  async getEnvironments(): Promise<Environment[]> {
    const response = await this.instance.get<ApiResponse<Environment[]>>('/environments');
    return response.data.data!;
  }

  async createEnvironment(data: Partial<Environment>): Promise<Environment> {
    const response = await this.instance.post<ApiResponse<Environment>>('/environments', data);
    return response.data.data!;
  }

  async updateEnvironment(id: number, data: Partial<Environment>): Promise<Environment> {
    const response = await this.instance.put<ApiResponse<Environment>>(`/environments/${id}`, data);
    return response.data.data!;
  }

  async deleteEnvironment(id: number): Promise<void> {
    await this.instance.delete(`/environments/${id}`);
  }

  // Project APIs
  async getProjects(params?: { page?: number; page_size?: number }): Promise<PageData<Project>> {
    const response = await this.instance.get<ApiResponse<PageData<Project>>>('/projects', { params });
    return response.data.data!;
  }

  async createProject(data: Partial<Project>): Promise<Project> {
    const response = await this.instance.post<ApiResponse<Project>>('/projects', data);
    return response.data.data!;
  }

  async getProject(id: number): Promise<Project> {
    const response = await this.instance.get<ApiResponse<Project>>(`/projects/${id}`);
    return response.data.data!;
  }

  async updateProject(id: number, data: Partial<Project>): Promise<Project> {
    const response = await this.instance.put<ApiResponse<Project>>(`/projects/${id}`, data);
    return response.data.data!;
  }

  async deleteProject(id: number): Promise<void> {
    await this.instance.delete(`/projects/${id}`);
  }

  // Device APIs
  async getDevices(): Promise<Device[]> {
    const response = await this.instance.get<ApiResponse<Device[]>>('/devices');
    return response.data.data!;
  }

  async createDevice(data: Partial<Device>): Promise<Device> {
    const response = await this.instance.post<ApiResponse<Device>>('/devices', data);
    return response.data.data!;
  }

  async updateDevice(id: number, data: Partial<Device>): Promise<Device> {
    const response = await this.instance.put<ApiResponse<Device>>(`/devices/${id}`, data);
    return response.data.data!;
  }

  async deleteDevice(id: number): Promise<void> {
    await this.instance.delete(`/devices/${id}`);
  }

  // Test Case APIs
  async getTestCases(params?: {
    page?: number;
    page_size?: number;
    project_id?: number;
  }): Promise<PageData<TestCase>> {
    const response = await this.instance.get<ApiResponse<PageData<TestCase>>>('/test-cases', { params });
    return response.data.data!;
  }

  async createTestCase(data: Partial<TestCase>): Promise<TestCase> {
    const response = await this.instance.post<ApiResponse<TestCase>>('/test-cases', data);
    return response.data.data!;
  }

  async getTestCase(id: number): Promise<TestCase> {
    const response = await this.instance.get<ApiResponse<TestCase>>(`/test-cases/${id}`);
    return response.data.data!;
  }

  async updateTestCase(id: number, data: Partial<TestCase>): Promise<TestCase> {
    const response = await this.instance.put<ApiResponse<TestCase>>(`/test-cases/${id}`, data);
    return response.data.data!;
  }

  async deleteTestCase(id: number): Promise<void> {
    await this.instance.delete(`/test-cases/${id}`);
  }

  async executeTestCase(id: number, options?: { is_visual?: boolean }): Promise<TestExecution> {
    const response = await this.instance.post<ApiResponse<TestExecution>>(`/test-cases/${id}/execute`, options);
    return response.data.data!;
  }

  // Test Suite APIs
  async getTestSuites(params?: {
    page?: number;
    page_size?: number;
    project_id?: number;
  }): Promise<PageData<TestSuite>> {
    const response = await this.instance.get<ApiResponse<PageData<TestSuite>>>('/test-suites', { params });
    return response.data.data!;
  }

  async createTestSuite(data: Partial<TestSuite>): Promise<TestSuite> {
    const response = await this.instance.post<ApiResponse<TestSuite>>('/test-suites', data);
    return response.data.data!;
  }

  async getTestSuite(id: number): Promise<TestSuite> {
    const response = await this.instance.get<ApiResponse<TestSuite>>(`/test-suites/${id}`);
    return response.data.data!;
  }

  async updateTestSuite(id: number, data: Partial<TestSuite>): Promise<TestSuite> {
    const response = await this.instance.put<ApiResponse<TestSuite>>(`/test-suites/${id}`, data);
    return response.data.data!;
  }

  async deleteTestSuite(id: number): Promise<void> {
    await this.instance.delete(`/test-suites/${id}`);
  }

  async executeTestSuite(id: number, options?: { is_visual?: boolean; continue_failed_only?: boolean; parent_execution_id?: number }): Promise<TestExecution[]> {
    const response = await this.instance.post<ApiResponse<TestExecution[]>>(`/test-suites/${id}/execute`, options);
    return response.data.data!;
  }

  async stopTestSuite(id: number): Promise<{ stopped_count: number }> {
    const response = await this.instance.post<ApiResponse<{ stopped_count: number }>>(`/test-suites/${id}/stop`);
    return response.data.data!;
  }

  // Execution APIs
  async getExecutions(params?: {
    name?: string;
    page?: number;
    page_size?: number;
    status?: string;
    test_case_id?: number;
    project_id?: number;
    environment_id?: number;
    execution_type?: string;
    start_date?: string;
    end_date?: string;
    include_internal?: boolean;
    parent_execution_id?: number;
  }): Promise<PageData<TestExecution>> {
    const response = await this.instance.get<ApiResponse<PageData<TestExecution>>>('/executions', { params });
    return response.data.data!;
  }

  async getExecutionStatistics(params?: {
    name?: string;
    project_id?: number;
    environment_id?: number;
    status?: string;
    execution_type?: string;
    start_date?: string;
    end_date?: string;
  }): Promise<{
    total_executions: number;
    passed_count: number;
    failed_count: number;
    running_count: number;
    pending_count: number;
    success_rate: number;
    avg_duration: number;
  }> {
    const response = await this.instance.get<ApiResponse<{
      total_executions: number;
      passed_count: number;
      failed_count: number;
      running_count: number;
      pending_count: number;
      success_rate: number;
      avg_duration: number;
    }>>('/executions/statistics', { params });
    return response.data.data!;
  }

  async getExecution(id: number): Promise<TestExecution> {
    const response = await this.instance.get<ApiResponse<TestExecution>>(`/executions/${id}`);
    return response.data.data!;
  }

  async deleteExecution(id: number): Promise<void> {
    await this.instance.delete(`/executions/${id}`);
  }

  async stopExecution(id: number): Promise<void> {
    await this.instance.post(`/executions/${id}/stop`);
  }

  async getExecutionLogs(id: number): Promise<{ logs: any[] }> {
    const response = await this.instance.get<ApiResponse<{ logs: any[] }>>(`/executions/${id}/logs`);
    return response.data.data!;
  }

  async getExecutionScreenshots(id: number): Promise<{ 
    screenshots: any[];
    execution_screenshots: string[];
    debug_info?: {
      screenshot_count: number;
      execution_screenshot_count: number;
    };
  }> {
    const response = await this.instance.get<ApiResponse<{
      screenshots: any[];
      execution_screenshots: string[];
      debug_info?: {
        screenshot_count: number;
        execution_screenshot_count: number;
      };
    }>>(`/executions/${id}/screenshots`);
    return response.data.data!;
  }

  async getTestSuiteExecutions(suiteId: number): Promise<PageData<TestExecution>> {
    const response = await this.instance.get<ApiResponse<PageData<TestExecution>>>(`/test-suites/${suiteId}/executions`);
    return response.data.data!;
  }

  async getTestSuiteLatestReport(suiteId: number): Promise<any> {
    const response = await this.instance.get<ApiResponse<any>>(`/test-suites/${suiteId}/latest-report`);
    return response.data.data!;
  }

  async getCurrentBatchExecutions(executionId: number): Promise<PageData<TestExecution>> {
    const response = await this.instance.get<ApiResponse<PageData<TestExecution>>>(`/executions/${executionId}/batch`);
    return response.data.data!;
  }

  async getExecutionStatus(id: number): Promise<{
    id: number;
    database_status: string;
    executor_running: boolean;
    start_time: string;
    end_time?: string;
    consistent: boolean;
  }> {
    const response = await this.instance.get<ApiResponse<{
      id: number;
      database_status: string;
      executor_running: boolean; 
      start_time: string;
      end_time?: string;
      consistent: boolean;
    }>>(`/executions/${id}/status`);
    return response.data.data!;
  }

  // Report APIs
  async getReports(params?: {
    page?: number;
    page_size?: number;
    project_id?: number;
  }): Promise<PageData<TestReport>> {
    const response = await this.instance.get<ApiResponse<PageData<TestReport>>>('/reports', { params });
    return response.data.data!;
  }

  async getReport(id: number): Promise<TestReport> {
    const response = await this.instance.get<ApiResponse<TestReport>>(`/reports/${id}`);
    return response.data.data!;
  }

  async deleteReport(id: number): Promise<void> {
    await this.instance.delete(`/reports/${id}`);
  }

  async createReport(data: { 
    name: string; 
    project_id: number; 
    test_suite_id?: number | null; 
    execution_ids: number[] 
  }): Promise<TestReport> {
    const response = await this.instance.post<ApiResponse<TestReport>>('/reports', data);
    return response.data.data!;
  }


  // Recording APIs
  async startRecording(data: {
    environment_id: number;
    device_id: number;
  }): Promise<{ session_id: string }> {
    const response = await this.instance.post<ApiResponse<{ session_id: string }>>('/recording/start', data);
    return response.data.data!;
  }

  async stopRecording(sessionId: string): Promise<void> {
    await this.instance.post('/recording/stop', { session_id: sessionId });
  }

  async getRecordingStatus(sessionId: string): Promise<{
    is_recording: boolean;
    steps: any[];
  }> {
    const response = await this.instance.get<ApiResponse<any>>('/recording/status', {
      params: { session_id: sessionId },
    });
    return response.data.data!;
  }

  async saveRecording(data: {
    session_id: string;
    name: string;
    description: string;
    project_id: number;
    environment_id: number;
    device_id: number;
    expected_result: string;
    tags: string;
    priority?: number;
    steps?: TestStep[]; // 发送TestStep数组，符合后端期望
  }): Promise<TestCase> {
    const response = await this.instance.post<ApiResponse<TestCase>>('/recording/save', data);
    return response.data.data!;
  }

  // Health check
  async healthCheck(): Promise<{ status: string; timestamp: string }> {
    const response = await this.instance.get<ApiResponse<any>>('/health');
    return response.data.data!;
  }
}

export const api = new ApiService();
export default api;