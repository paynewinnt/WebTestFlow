export interface User {
  id: number;
  username: string;
  email: string;
  avatar?: string;
  status: number;
  created_at: string;
  updated_at: string;
}

export interface Environment {
  id: number;
  name: string;
  description: string;
  base_url: string;
  type: 'test' | 'product';
  headers: string;
  variables: string;
  status: number;
  created_at: string;
  updated_at: string;
}

export interface Project {
  id: number;
  name: string;
  description: string;
  user_id: number;
  user: User;
  status: number;
  created_at: string;
  updated_at: string;
}

export interface Device {
  id: number;
  name: string;
  width: number;
  height: number;
  user_agent: string;
  is_default: boolean;
  status: number;
  created_at: string;
  updated_at: string;
}

export interface TestStep {
  type: string;
  selector: string;
  value: string;
  coordinates: Record<string, any>;
  options: Record<string, any>;
  timestamp: number;
  screenshot?: string;
  description?: string;
  wait_before?: number;
}

export interface TestCase {
  id: number;
  name: string;
  description: string;
  project_id: number;
  project: Project;
  environment_id: number;
  environment: Environment;
  device_id: number;
  device: Device;
  steps: string;
  expected_result: string;
  tags: string;
  priority: number;
  status: number;
  user_id: number;
  user: User;
  created_at: string;
  updated_at: string;
}

export interface TestSuite {
  id: number;
  name: string;
  description: string;
  project_id: number;
  project: Project;
  environment_id: number;
  environment: Environment;
  test_cases: TestCase[];
  test_case_count: number;
  schedule: string;
  cron_expression: string;
  is_parallel: boolean;
  timeout_minutes: number;
  tags: string;
  priority: number;
  status: number;
  user_id: number;
  user: User;
  created_at: string;
  updated_at: string;
}

export interface TestExecution {
  id: number;
  test_case_id: number;
  test_case: TestCase;
  test_suite_id?: number;
  test_suite?: TestSuite;
  execution_type: 'test_case' | 'test_suite';
  status: 'pending' | 'running' | 'passed' | 'failed' | 'cancelled';
  start_time: string;
  end_time?: string;
  duration: number;
  total_count: number;
  passed_count: number;
  failed_count: number;
  error_message: string;
  execution_logs: string;
  screenshots: string;
  user_id: number;
  user: User;
  created_at: string;
  updated_at: string;
}

export interface TestReport {
  id: number;
  name: string;
  project_id: number;
  project: Project;
  test_suite_id?: number;
  test_suite?: TestSuite;
  executions: TestExecution[];
  total_cases: number;
  passed_cases: number;
  failed_cases: number;
  error_cases: number;
  start_time: string;
  end_time: string;
  duration: number;
  status: 'completed' | 'running' | 'error';
  user_id: number;
  user: User;
  created_at: string;
  updated_at: string;
}

export interface PerformanceMetric {
  id: number;
  execution_id: number;
  execution: TestExecution;
  page_load_time: number;
  dom_content_loaded: number;
  first_paint: number;
  first_contentful_paint: number;
  memory_usage: number;
  network_requests: number;
  network_time: number;
  js_heap_size: number;
  created_at: string;
  updated_at: string;
}

export interface Screenshot {
  id: number;
  execution_id: number;
  execution: TestExecution;
  step_index: number;
  type: 'before' | 'after' | 'error';
  file_path: string;
  file_name: string;
  file_size: number;
  created_at: string;
  updated_at: string;
}

export interface ApiResponse<T = any> {
  code: number;
  message: string;
  data?: T;
}

export interface PageData<T = any> {
  list: T[];
  total: number;
  page: number;
  page_size: number;
}

export interface LoginRequest {
  username: string;
  password: string;
}

export interface LoginResponse {
  token: string;
  user: User;
}

export interface RegisterRequest {
  username: string;
  email: string;
  password: string;
}

export interface RecordingSession {
  id: string;
  target_url: string;
  device: Device;
  is_recording: boolean;
  steps: TestStep[];
  start_time: string;
}

export interface ExecutionLog {
  timestamp: string;
  level: 'info' | 'warn' | 'error';
  message: string;
  step_index: number;
  step_type?: string;
  step_status?: 'success' | 'failed' | 'running';
  selector?: string;
  value?: string;
  screenshot?: string;
  duration?: number; // milliseconds
  error_detail?: string;
}