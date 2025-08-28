# WebTestFlow

🚀 **基于Web的UI自动化测试平台**，专注于移动端网页自动化测试，支持Chrome DevTools Protocol驱动的录制和执行。

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![React Version](https://img.shields.io/badge/React-18.2+-61DAFB?style=flat&logo=react)](https://reactjs.org/)
[![MySQL](https://img.shields.io/badge/MySQL-8.0+-4479A1?style=flat&logo=mysql&logoColor=white)](https://www.mysql.com/)
[![Chrome DevTools](https://img.shields.io/badge/Chrome_DevTools-Protocol-4285F4?style=flat&logo=googlechrome)](https://chromedevtools.github.io/devtools-protocol/)

## ✨ 核心特性

### 🎯 智能录制与执行
- **📱 移动端优先**: 完整支持移动设备模拟(iPhone、iPad、Android等)
- **🌐 跨域录制**: 业界领先的跨域页面录制技术，支持复杂单页应用
- **⚡ 实时录制**: Chrome DevTools Protocol驱动，WebSocket实时同步
- **🎬 可视化执行**: 实时查看执行过程，支持调试模式

### 🔐 验证码自动化 (v2.2.0)
- **📱 短信验证码**: 纯ADB方案，零配置获取Android设备短信
- **🖼️ 图形验证码**: OCR自动识别数字、字母验证码
- **🧩 滑块验证码**: AI识别滑块缺口位置，自动拖拽完成验证
- **🎯 智能标记**: 录制时实时标记，执行时自动处理

### 📊 企业级测试管理
- **🏗️ 项目管理**: 多项目、多环境、多设备配置支持
- **📝 测试套件**: 批量执行，支持失败重试和继续执行
- **⏰ 定时任务**: Cron表达式调度，自动化回归测试
- **👥 用户权限**: JWT认证，多用户协作支持

### 📈 专业测试报告
- **📄 PDF/HTML报告**: 专业格式，支持团队分享和归档
- **📊 性能指标**: 页面加载时间、内存使用、网络请求分析
- **📸 截图记录**: 每个步骤自动截图，可视化执行过程
- **📈 统计分析**: 通过率趋势、执行时长分析

## 🛠️ 技术架构

### 后端技术栈
- **Go 1.21+** + Gin框架 - 高性能REST API
- **MySQL 8.0** + GORM - 数据持久化和ORM
- **Chrome DevTools Protocol** + ChromeDP - 浏览器自动化
- **WebSocket** (gorilla/websocket) - 实时通信
- **Cron** (robfig/cron/v3) - 定时任务调度
- **JWT** - 用户认证授权

### 前端技术栈
- **React 18** + TypeScript - 现代化前端框架
- **Ant Design 5.x** - 企业级UI组件库
- **React Router 6.x** - 单页应用路由
- **Axios** - HTTP客户端
- **Hooks** - 响应式状态管理

### 项目结构
```
WebTestFlow/
├── backend/                 # Go后端服务
│   ├── cmd/main.go         # 应用入口
│   ├── internal/           # 核心业务逻辑
│   │   ├── api/            # RESTful API (handlers, routes, middleware)
│   │   ├── executor/       # 测试执行引擎
│   │   ├── recorder/       # 录制引擎
│   │   ├── models/         # 数据模型
│   │   └── services/       # 业务服务
│   ├── pkg/                # 公共组件
│   │   ├── chrome/         # Chrome实例管理
│   │   ├── sms/            # 短信验证码服务
│   │   ├── auth/           # JWT认证
│   │   └── database/       # 数据库连接
│   └── go.mod              # Go依赖管理
├── frontend/               # React前端应用
│   ├── src/
│   │   ├── pages/          # 页面组件
│   │   ├── components/     # 可复用组件
│   │   ├── services/       # API服务
│   │   └── types/          # TypeScript类型
│   └── package.json        # NPM依赖管理
├── scripts/                # 自动化脚本
│   ├── one-click-start.sh  # 一键启动
│   ├── one-click-stop.sh   # 一键停止
│   ├── dev-setup.sh        # 开发环境设置
│   └── deploy.sh           # 部署脚本
├── screenshots/            # 截图存储(按日期组织)
├── logs/                   # 日志文件
└── docker-compose.yml      # 容器编排
```

## 🚀 快速开始

### 系统要求
- **Go**: 1.21+ 
- **Node.js**: 18.2+
- **MySQL**: 8.0+
- **Chrome**: 最新版本
- **Android SDK**: Platform Tools (短信验证码功能需要)
- **Docker** (可选): 容器化部署

### 🎯 一键启动 (推荐)

```bash
# 克隆项目
git clone <repository-url>
cd WebTestFlow

# 一键启动所有服务
chmod +x scripts/one-click-start.sh
./scripts/one-click-start.sh
```

脚本会自动：
- ✅ 检查并安装依赖 (Go、Node.js、MySQL)
- ✅ 配置数据库连接
- ✅ 启动后端服务 (端口8080)
- ✅ 启动前端服务 (端口3000)
- ✅ 启动OCR服务 (端口8888)

**访问地址**:
- 🖥️ 前端应用: http://localhost:3000
- 🔌 后端API: http://localhost:8080/api/v1
- ❤️ 健康检查: http://localhost:8080/api/v1/health

### 🛑 停止服务

```bash
./scripts/one-click-stop.sh
```

### 🔧 开发环境手动搭建

<details>
<summary>点击展开详细步骤</summary>

1. **环境设置**
```bash
# 运行开发环境设置脚本
chmod +x scripts/dev-setup.sh
./scripts/dev-setup.sh
```

2. **数据库配置**
```bash
# 创建数据库
mysql -u root -p
CREATE DATABASE webtestflow CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

# 配置环境变量
cp .env.example .env
# 编辑 .env 文件中的数据库配置
```

3. **启动后端**
```bash
cd backend
go mod tidy
go run cmd/main.go
```

4. **启动前端**
```bash
cd frontend
npm install
npm start
```

</details>

## 📖 使用指南

### 1. 用户管理
首次启动需要注册账户，系统支持多用户协作。

### 2. 项目和环境配置
- **项目管理**: 创建项目，组织测试用例
- **环境配置**: 设置测试环境URL、请求头等
- **设备管理**: 配置移动设备模拟参数

### 3. 录制测试用例

#### 🎬 基础录制
1. 进入录制页面，选择项目、环境和设备
2. 输入目标网页URL，点击"开始录制"
3. 在打开的Chrome浏览器中操作
4. 实时查看录制的步骤
5. 完成后点击"停止录制"并保存

#### 🔐 验证码录制 (重要)

**短信验证码** (v2.2.0 推荐):
1. **设备准备**: Android设备USB连接，开启调试模式
2. **录制标记**: 录制过程中点击验证码相关步骤的"标记验证码"
3. **配置参数**:
   - 验证码类型: 短信验证码
   - 手机号码: 接收验证码的手机号
   - 发送按钮: CSS选择器
   - 输入框: CSS选择器
   - 等待超时: 60秒(默认)

**支持的短信格式**:
- `验证码：123456`
- `验证码123456` 
- `code:123456`
- `【平台名】123456`
- `(123456)`

**图形验证码**:
- 配置验证码图片和输入框选择器
- OCR自动识别填入

**滑块验证码**:
- 配置滑块背景图和拖拽元素选择器
- AI识别缺口位置自动拖拽

### 4. 执行测试

#### 📝 单用例执行
在测试用例列表点击"执行"按钮，支持可视化调试模式。

#### 📦 测试套件执行
1. 创建测试套件，添加多个测试用例
2. 支持批量执行、失败重试、继续执行
3. 实时查看执行进度和状态

#### ⏰ 定时任务
- 配置Cron表达式实现自动化回归测试
- 支持邮件通知和报告推送

### 5. 测试报告

#### 📊 执行报告
- **实时状态**: 执行进度、通过率统计
- **详细日志**: 每个步骤的执行日志和错误信息
- **性能指标**: 页面加载时间、内存使用情况
- **截图记录**: 每个操作步骤的自动截图

#### 📄 报告导出
- **PDF格式**: 专业打印版本，适合归档分享
- **HTML格式**: 交互式网页版本，支持在线查看
- **团队分享**: 使用实际IP地址，团队成员可直接访问

## 🔌 API文档

### 认证接口
```http
POST /api/v1/auth/login          # 用户登录
POST /api/v1/auth/register       # 用户注册
```

### 项目管理
```http
GET    /api/v1/projects          # 获取项目列表
POST   /api/v1/projects          # 创建项目
PUT    /api/v1/projects/:id      # 更新项目
DELETE /api/v1/projects/:id      # 删除项目
```

### 环境管理
```http
GET    /api/v1/environments      # 获取环境列表
POST   /api/v1/environments      # 创建环境
PUT    /api/v1/environments/:id  # 更新环境
DELETE /api/v1/environments/:id  # 删除环境
```

### 设备管理
```http
GET    /api/v1/devices           # 获取设备列表
POST   /api/v1/devices           # 创建设备
PUT    /api/v1/devices/:id       # 更新设备
DELETE /api/v1/devices/:id       # 删除设备
```

### 测试用例
```http
GET    /api/v1/test-cases                # 获取测试用例列表
POST   /api/v1/test-cases                # 创建测试用例
PUT    /api/v1/test-cases/:id            # 更新测试用例
DELETE /api/v1/test-cases/:id            # 删除测试用例
POST   /api/v1/test-cases/:id/execute    # 执行测试用例
```

### 测试套件
```http
GET    /api/v1/test-suites               # 获取测试套件列表
POST   /api/v1/test-suites               # 创建测试套件
PUT    /api/v1/test-suites/:id           # 更新测试套件
DELETE /api/v1/test-suites/:id           # 删除测试套件
POST   /api/v1/test-suites/:id/execute   # 执行测试套件
POST   /api/v1/test-suites/:id/stop      # 停止执行
```

### 执行记录
```http
GET    /api/v1/executions               # 获取执行记录列表
GET    /api/v1/executions/:id           # 获取执行详情
POST   /api/v1/executions/:id/stop      # 停止执行
GET    /api/v1/executions/:id/logs      # 获取执行日志
GET    /api/v1/executions/:id/screenshots # 获取执行截图
GET    /api/v1/executions/:id/status    # 获取执行状态
```

### 录制功能
```http
POST /api/v1/recording/start            # 开始录制
POST /api/v1/recording/stop             # 停止录制
GET  /api/v1/recording/status           # 获取录制状态
POST /api/v1/recording/save             # 保存录制结果
GET  /api/v1/ws/recording               # WebSocket连接
```

### 报告导出
```http
GET /api/v1/executions/:id/report/html          # 导出单次执行HTML报告
GET /api/v1/executions/:id/report/pdf           # 导出单次执行PDF报告
GET /api/v1/test-suite-latest-report-html/:id   # 导出测试套件最新HTML报告
GET /api/v1/test-suite-latest-report-pdf/:id    # 导出测试套件最新PDF报告
```

### 用户管理
```http
GET    /api/v1/users/profile            # 获取用户信息
PUT    /api/v1/users/profile            # 更新用户信息
GET    /api/v1/users                    # 获取用户列表(管理员)
PUT    /api/v1/users/:id/password       # 修改用户密码(管理员)
```

### 系统接口
```http
GET /api/v1/health                      # 系统健康检查
```

## ⚙️ 配置说明

### 环境变量
```bash
# 数据库配置
DB_HOST=localhost
DB_PORT=3306
DB_USERNAME=webtestflow
DB_PASSWORD=123456
DB_NAME=webtestflow

# 服务器配置
SERVER_PORT=8080
SERVER_HOST=0.0.0.0
SERVER_MODE=release

# JWT配置
JWT_SECRET=your-secret-key
JWT_EXPIRES_IN=24h

# Chrome配置
CHROME_PATH=/usr/bin/google-chrome
CHROME_MAX_INSTANCES=10
CHROME_TIMEOUT=30

# 短信验证码配置 (v2.2.0)
SMS_TIMEOUT=60s
SMS_MAX_ATTEMPTS=3

# OCR服务配置
OCR_SERVICE_URL=http://localhost:8888
OCR_TIMEOUT=30s

# 文件存储配置
UPLOAD_PATH=./uploads
SCREENSHOT_PATH=./screenshots
LOG_PATH=./logs
```

### Chrome设备配置
系统预设多种移动设备模拟配置：

```yaml
设备类型:
  iPhone 12 Pro: 390x844, iOS 14
  iPhone SE: 375x667, iOS 14  
  iPad Pro: 1024x1366, iPadOS 14
  Samsung Galaxy S21: 360x800, Android 11
  Pixel 5: 393x851, Android 11
  自定义设备: 可配置分辨率和User-Agent
```

## 🚧 故障排除

### 常见问题

#### Chrome相关
```bash
# Chrome启动失败
- 检查Chrome是否安装：google-chrome --version
- 检查端口占用：netstat -tlnp | grep 9222
- 清理Chrome进程：pkill -f chrome

# Chrome权限问题  
- 运行权限脚本：./scripts/fix-permissions.sh
- 手动设置权限：chmod +x /usr/bin/google-chrome
```

#### 数据库相关
```bash
# 连接失败
- 检查MySQL状态：systemctl status mysql
- 测试连接：mysql -u webtestflow -p123456 webtestflow
- 重置数据库：./scripts/clean-database.sh
```

#### 短信验证码相关 (v2.2.0)
```bash
# ADB设备连接
- 检查设备：adb devices
- 重启ADB：adb kill-server && adb start-server
- 授权设备：确保设备屏幕解锁并授权调试

# 验证码获取失败
- 检查权限：确保应用有短信读取权限
- 检查格式：查看短信内容是否包含数字验证码
- 检查时间：确保10秒等待时间足够
```

#### 录制和执行
```bash
# 录制步骤丢失
- 检查网络：确保目标页面可正常访问
- 检查跨域：使用Chrome开发者工具查看错误
- 重启录制：停止当前录制，重新开始

# 执行失败
- 查看日志：tail -f logs/backend.log
- 检查截图：查看screenshots目录下的执行截图
- 调试模式：使用可视化执行查看详细过程
```

### 性能优化

#### Chrome优化
```bash
# 限制Chrome实例数
export CHROME_MAX_INSTANCES=5

# 禁用GPU加速
--disable-gpu --disable-dev-shm-usage

# 清理临时文件
./scripts/cleanup-now.sh
```

#### 数据库优化
```sql
-- 索引优化
CREATE INDEX idx_executions_status ON test_executions(status);
CREATE INDEX idx_executions_start_time ON test_executions(start_time);

-- 清理历史数据
DELETE FROM test_executions WHERE start_time < DATE_SUB(NOW(), INTERVAL 30 DAY);
```

### 日志查看
```bash
# 实时查看所有日志
tail -f logs/*.log

# 查看特定服务日志
tail -f logs/backend.log    # 后端日志
tail -f logs/frontend.log   # 前端日志  
tail -f logs/ocr.log        # OCR服务日志

# 查看错误日志
grep -i error logs/backend.log
```

## 🔄 版本历史

### v2.2.0 - 短信验证码系统重构 (当前)
- ✅ **纯ADB短信验证码**: 移除SMS Helper APK，使用直接ADB查询
- ✅ **自动等待机制**: 10秒智能等待，确保短信到达
- ✅ **零配置部署**: 无需APK安装，开箱即用
- ✅ **架构简化**: 删除1000+行Android代码，保持核心功能
- ✅ **最新报告修复**: 修复测试套件PDF报告生成功能

### v2.1.0 - 可视化执行优化
- ✅ **统一可视化执行**: 移除后台执行，提供一致的调试体验
- ✅ **JavaScript环境友好**: 移除反调试保护，不干扰页面功能  
- ✅ **录制问题修复**: 解决跨域导航时步骤丢失问题
- ✅ **Chrome实例复用**: 优化资源管理，减少启动延迟

### v2.0.0 - 跨域录制系统
- ✅ **跨域录制支持**: 完整的跨域页面录制和执行
- ✅ **6层事件过滤**: 解决cookiePart等JSON解析错误
- ✅ **智能脚本重注入**: 页面导航后自动恢复录制
- ✅ **多种导航支持**: navigate/back/popstate/hashchange等

### v1.0.0 - 基础功能
- ✅ **基础UI录制**: Chrome DevTools Protocol录制
- ✅ **测试用例管理**: 完整的CRUD操作
- ✅ **执行引擎**: 并发执行和状态管理
- ✅ **用户系统**: JWT认证和权限控制

## 🤝 贡献指南

欢迎提交Issue和Pull Request！

### 开发流程
1. Fork项目
2. 创建功能分支: `git checkout -b feature/amazing-feature`
3. 提交更改: `git commit -m 'Add amazing feature'`
4. 推送到分支: `git push origin feature/amazing-feature`
5. 提交Pull Request

### 代码规范
- **Go**: 遵循Go标准规范，使用`gofmt`格式化
- **React**: 使用TypeScript，遵循ESLint规则
- **提交信息**: 使用语义化提交格式

## 📜 许可证

本项目采用 MIT 许可证。详见 [LICENSE](LICENSE) 文件。

## 🌟 Star History

如果这个项目对您有帮助，请给我们一个 ⭐️ Star！

---

<div align="center">

**🌐 WebTestFlow v2.2.0**

*让移动端UI自动化测试更简单、更智能、更可靠*

[📖 详细文档](./CLAUDE.md) | [🐛 反馈问题](../../issues) | [💬 讨论交流](../../discussions)

</div>