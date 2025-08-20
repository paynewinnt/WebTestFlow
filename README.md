# WebTestFlow

基于Web的UI自动化测试平台，支持移动端网页自动化测试。

## 功能特性

- 🎯 **浏览器录制**: 通过Chrome DevTools Protocol实现网页操作录制
- 📱 **移动端模拟**: 支持Chrome Device Mode和自定义设备参数
- 🏗️ **项目管理**: 多项目、多环境支持
- 📝 **测试用例管理**: JSON格式存储，支持复杂交互录制
- 📊 **测试报告**: 包含执行日志、性能指标、截图等
- 📄 **报告导出**: 支持HTML和PDF格式导出，无页眉页脚的专业报告
- ⚡ **并发执行**: 支持最多10个测试用例同时运行
- ⏰ **定时任务**: 支持cron表达式配置自动执行
- 🔐 **用户管理**: 完整的用户认证和权限控制
- 🌐 **跨域支持**: 完整的跨域页面录制和执行能力

## 技术栈

**后端**:
- Go 1.21+ + Gin框架
- MySQL 8.0数据库
- Chrome DevTools Protocol
- ChromeDP (PDF生成)
- Cron定时任务
- JWT认证
- WebSocket实时通信

**前端**:
- React 18 + TypeScript
- Ant Design UI组件库
- Axios HTTP客户端

## 系统架构

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   前端 React    │    │   后端 Go       │    │   数据库 MySQL  │
│                 │◄──►│                 │◄──►│                 │
│ - 用户界面      │    │ - REST API      │    │ - 用户数据      │
│ - 测试用例管理  │    │ - 录制引擎      │    │ - 测试用例      │
│ - 实时录制      │    │ - 执行引擎      │    │ - 执行记录      │
│ - 报告展示      │    │ - 定时任务      │    │ - 测试报告      │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                │
                       ┌─────────────────┐
                       │  Chrome浏览器   │
                       │                 │
                       │ - 网页录制      │
                       │ - 自动化执行    │
                       │ - 移动端模拟    │
                       └─────────────────┘
```

## 快速开始

### 环境要求
- Go 1.21+
- Node.js 18+
- MySQL 8.0+
- Chrome浏览器
- Docker & Docker Compose (可选)

### 开发环境搭建

1. **克隆项目**
```bash
git clone <repository-url>
cd WebTestFlow
```

2. **运行开发环境设置脚本**
```bash
chmod +x scripts/dev-setup.sh
./scripts/dev-setup.sh
```

3. **配置数据库**
```bash
# 创建数据库
mysql -u root -p
CREATE DATABASE webtestflow CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

# 更新 .env 文件中的数据库配置
```

4. **启动后端服务**
```bash
cd backend
go run cmd/main.go
```

5. **启动前端服务**
```bash
cd frontend
npm start
```

### Docker部署 (推荐)

1. **一键部署**
```bash
chmod +x scripts/deploy.sh
./scripts/deploy.sh
```

2. **手动部署**
```bash
# 复制环境配置
cp .env.example .env

# 编辑环境变量
vim .env

# 启动服务
docker-compose up -d
```

## 使用指南

### 访问应用
- 前端界面: http://localhost (Docker) 或 http://localhost:3000 (开发环境)
- 后端API: http://localhost/api/v1 (Docker) 或 http://localhost:8080/api/v1 (开发环境)
- 健康检查: http://localhost/health

### 默认登录
首次启动后需要注册账户，或通过API创建管理员账户。

### 录制测试用例

1. **创建项目**: 在项目管理中创建新项目
2. **配置环境**: 设置测试环境URL和参数
3. **选择设备**: 选择移动设备模拟参数
4. **开始录制**: 
   - 进入录制页面
   - 输入目标网页URL
   - 点击开始录制
   - 在打开的浏览器中执行操作
   - 停止录制并保存

### 执行测试

1. **单个测试用例**: 在测试用例列表点击执行
2. **测试套件**: 创建测试套件，批量执行多个用例
3. **定时任务**: 配置cron表达式，自动执行测试套件

### 查看和导出报告

1. **执行记录**: 查看每次执行的详细日志和截图
2. **测试报告**: 生成汇总报告，包含统计信息和趋势
3. **性能指标**: 查看页面加载时间、内存使用等指标
4. **报告导出**: 
   - **HTML格式**: 完整的网页报告，包含交互式截图查看
   - **PDF格式**: 专业打印格式，无页眉页脚，适合归档和分享
   - 支持IP地址共享，团队成员可直接访问报告中的截图

## API文档

### 认证接口
```
POST /api/v1/auth/login      # 用户登录
POST /api/v1/auth/register   # 用户注册
```

### 项目管理
```
GET    /api/v1/projects      # 获取项目列表
POST   /api/v1/projects      # 创建项目
PUT    /api/v1/projects/:id  # 更新项目
DELETE /api/v1/projects/:id  # 删除项目
```

### 测试用例
```
GET    /api/v1/test-cases           # 获取测试用例列表
POST   /api/v1/test-cases           # 创建测试用例
PUT    /api/v1/test-cases/:id       # 更新测试用例
DELETE /api/v1/test-cases/:id       # 删除测试用例
POST   /api/v1/test-cases/:id/execute # 执行测试用例
```

### 录制功能
```
POST /api/v1/recording/start        # 开始录制
POST /api/v1/recording/stop         # 停止录制
GET  /api/v1/recording/status       # 获取录制状态
POST /api/v1/recording/save         # 保存录制
GET  /api/v1/ws/recording           # WebSocket录制连接
```

### 报告导出
```
GET /api/v1/executions/:id/report/html  # 导出HTML格式报告  
GET /api/v1/executions/:id/report/pdf   # 导出PDF格式报告
```

## 配置说明

### 环境变量

```bash
# 数据库配置
DB_HOST=localhost
DB_PORT=3306
DB_USERNAME=autoui
DB_PASSWORD=123456
DB_NAME=autoui_platform

# 服务器配置
SERVER_PORT=8080
SERVER_HOST=0.0.0.0
SERVER_MODE=debug

# JWT配置
JWT_SECRET=your-secret-key
JWT_EXPIRE_TIME=86400

# Chrome配置
CHROME_HEADLESS=false
CHROME_MAX_INSTANCES=10
```

### 设备模拟配置

系统预置了常用移动设备配置：
- iPhone 12 Pro (390x844)
- iPad Pro (1024x1366) 
- Samsung Galaxy S21 (360x800)
- Desktop 1920x1080

也可以自定义设备参数：
- 屏幕宽度/高度
- User-Agent字符串
- 设备名称

### 定时任务配置

支持标准cron表达式：
```
# 每天上午9点执行
0 0 9 * * *

# 每小时执行
0 0 * * * *

# 每5分钟执行
0 */5 * * * *
```

## 开发说明

### 项目结构
```
WebTestFlow/
├── backend/                 # Go后端
│   ├── cmd/                # 应用入口
│   ├── internal/           # 内部包
│   │   ├── api/           # API路由和处理器
│   │   ├── config/        # 配置管理
│   │   ├── models/        # 数据模型
│   │   ├── services/      # 业务逻辑
│   │   ├── recorder/      # 录制引擎
│   │   └── executor/      # 执行引擎
│   └── pkg/               # 公共包
├── frontend/              # React前端
│   ├── src/
│   │   ├── components/    # 组件
│   │   ├── pages/        # 页面
│   │   ├── services/     # API服务
│   │   └── utils/        # 工具函数
│   └── public/
├── deployments/          # 部署配置
└── scripts/             # 脚本文件
```

### 录制原理

1. **浏览器启动**: 使用Chrome DevTools Protocol启动Chrome实例
2. **脚本注入**: 在目标页面注入JavaScript录制脚本
3. **事件捕获**: 监听用户操作事件(点击、输入、滚动等)
4. **数据传输**: 通过WebSocket实时传输录制数据
5. **格式转换**: 将操作转换为标准化的JSON格式

### 执行原理

1. **步骤解析**: 解析JSON格式的测试步骤
2. **浏览器控制**: 使用Chrome DevTools Protocol控制浏览器
3. **操作回放**: 按步骤执行点击、输入等操作
4. **结果收集**: 收集执行结果、日志、截图
5. **性能监控**: 收集页面性能指标

## 故障排除

### 常见问题

1. **Chrome启动失败**
   - 检查Chrome是否已安装
   - 确认Chrome版本兼容性
   - 检查系统权限

2. **录制无响应**
   - 检查目标网页是否可访问
   - 确认网页JavaScript未被禁用
   - 检查WebSocket连接

3. **执行失败**
   - 检查目标元素是否存在
   - 确认页面加载完成
   - 检查选择器是否正确

4. **数据库连接失败**
   - 检查MySQL服务状态
   - 确认数据库连接参数
   - 检查用户权限

5. **PDF导出失败**
   - 检查Chrome可执行文件路径
   - 确认Flatpak Chrome安装（Linux）
   - 检查Chrome包装脚本权限
   - 验证系统内存充足

### 日志查看

```bash
# Docker环境
docker-compose logs -f autoui-app

# 开发环境
tail -f logs/app.log
```

## PDF导出功能

### 特性说明

WebTestFlow支持将测试报告导出为PDF格式，具有以下特性：

- ✅ **Chrome原生打印**: 使用Chrome DevTools Protocol的原生打印功能
- ✅ **无页眉页脚**: 干净的PDF输出，只包含测试报告内容
- ✅ **A4标准格式**: 适合打印和归档的标准页面尺寸
- ✅ **完整内容**: 包含测试汇总、详细结果、截图等所有信息
- ✅ **高质量渲染**: 确保PDF与网页显示完全一致
- ✅ **中文支持**: 完整支持中文字符和格式

### 使用方法

1. **Web界面导出**:
   - 进入测试报告页面
   - 点击"导出报告"下拉菜单
   - 选择"PDF报告"
   - 浏览器将提示选择保存位置

2. **API导出**:
   ```bash
   curl -H "Authorization: Bearer YOUR_TOKEN" \
     "http://localhost:8080/api/v1/executions/{id}/report/pdf" \
     -o test_report.pdf
   ```

### 技术实现

- **后端**: Go + ChromeDP + Chrome DevTools Protocol
- **PDF生成**: Chrome无头模式原生打印
- **编码处理**: Base64编码避免字符编码问题
- **文件下载**: 浏览器Blob API + 原生下载

### 系统要求

- Chrome浏览器（支持Flatpak安装）
- 充足的内存空间（推荐4GB+）
- 稳定的网络连接（用于加载截图资源）

## 贡献指南

1. Fork 项目
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交改动 (`git commit -m 'Add some AmazingFeature'`)
4. 推送分支 (`git push origin feature/AmazingFeature`)
5. 提交Pull Request

## 许可证

本项目采用 MIT 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情。

## 联系方式

如有问题或建议，请提交Issue或联系开发团队。