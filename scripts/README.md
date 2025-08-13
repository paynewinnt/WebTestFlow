# WebTestFlow 启动脚本说明

本目录包含了用于启动和管理WebTestFlow开发环境的脚本。

## 📋 脚本列表

本目录包含以下脚本（已清理重复脚本，保持简洁）：

### 🚀 主要脚本

#### `one-click-start.sh` - 智能一键启动脚本（推荐）
完整的环境启动脚本，包含自动依赖安装功能。

**特性：**
- 🔍 自动检测操作系统和已安装的依赖
- 📦 自动安装缺失的依赖（Go、Node.js、Docker等）
- 🗄️ 自动启动MySQL数据库容器
- 🔧 自动启动后端服务
- 🎨 自动启动前端服务
- 📊 实时状态检查和日志输出

**使用方法：**
```bash
# 交互式安装（默认）
./scripts/one-click-start.sh

# 自动安装模式（无需手动确认）
./scripts/one-click-start.sh -y

# 跳过依赖检查（适用于已安装依赖的情况）
./scripts/one-click-start.sh --skip-deps

# 显示详细调试信息
./scripts/one-click-start.sh --verbose

# 查看帮助
./scripts/one-click-start.sh -h
```

**环境变量：**
```bash
# 启用自动安装模式
AUTO_INSTALL=1 ./scripts/one-click-start.sh

# 跳过依赖安装
SKIP_DEPS=1 ./scripts/one-click-start.sh

# 启用详细输出
VERBOSE=1 ./scripts/one-click-start.sh
```

#### `one-click-stop.sh` - 一键停止脚本
停止所有服务，包括数据库。

```bash
./scripts/one-click-stop.sh
```

### 🛠️ 工具脚本

#### `dev-setup.sh` - 开发环境初始化
初始化开发环境，安装依赖。

```bash
./scripts/dev-setup.sh
```

#### `install-chrome.sh` - Chrome安装
安装Chrome浏览器（用于自动化测试）。

```bash
./scripts/install-chrome.sh
```

#### `deploy.sh` - 部署脚本
生产环境部署脚本。

```bash
./scripts/deploy.sh
```

#### `setup-registry-mirrors.sh` - 镜像源配置
配置Docker/Podman镜像源，支持国内外镜像源选择。

```bash
# 交互式选择镜像源
./scripts/setup-registry-mirrors.sh

# 使用国内镜像源（适合中国用户）
./scripts/setup-registry-mirrors.sh --china

# 使用国际镜像源（适合海外用户）
./scripts/setup-registry-mirrors.sh --international

# 自动检测最佳镜像源
./scripts/setup-registry-mirrors.sh --auto
```

#### `fix-permissions.sh` - 权限修复
快速修复Docker/Podman相关权限问题。

```bash
./scripts/fix-permissions.sh
```

### 🗄️ 数据库管理脚本

#### `clean-database.sh` - 数据库纯净模式恢复（Bash版）
将数据库恢复到纯净状态，清除所有业务数据。

**使用方法：**
```bash
./scripts/clean-database.sh
```

**功能：**
- 清空所有业务数据（项目、测试用例、测试套件、执行记录、报告等）
- 保留admin用户
- 保留所有设备配置（4个预设设备）
- 保留环境配置（ID=2）
- 交互式确认，防止误操作
- 显示清理前后的数据统计

#### `clean-database.sql` - 数据库纯净模式恢复（SQL版）
纯SQL脚本，可直接在MySQL中执行。

**使用方法：**
```bash
# 命令行执行
mysql -u root -p webtestflow < scripts/clean-database.sql

# MySQL控制台执行
mysql> source scripts/clean-database.sql
```

**⚠️ 重要提示：**
执行数据清理前，强烈建议先备份数据库：
```bash
# 备份数据库
mysqldump -u root -p webtestflow > backup_$(date +%Y%m%d_%H%M%S).sql

# 恢复备份
mysql -u root -p webtestflow < backup_20241213_150000.sql
```

## 🎯 推荐使用流程

### 首次使用
```bash
# 1. 克隆项目后，直接使用一键启动（会自动安装依赖）
./scripts/one-click-start.sh -y

# 2. 等待启动完成，然后访问应用
# 前端：http://localhost:3000
# 后端：http://localhost:8080/api/v1
```

### 日常开发
```bash
# 启动（跳过依赖检查，更快）
./scripts/one-click-start.sh --skip-deps

# 停止所有服务
./scripts/one-click-stop.sh
```

### CI/CD环境
```bash
# 完全自动化
AUTO_INSTALL=1 SKIP_DEPS=0 ./scripts/one-click-start.sh
```

## 📊 支持的操作系统

### Linux发行版
- ✅ Ubuntu/Debian (apt-get)
- ✅ CentOS (yum)
- ✅ Fedora (dnf)
- ✅ Arch Linux (pacman)
- ✅ 其他Linux发行版（通用安装方式）

### macOS
- ✅ macOS (通过Homebrew)

### Windows
- ⚠️ 需要WSL或Git Bash环境

## 🔧 自动安装的依赖

### 核心依赖
- **Go** (1.21+) - 后端运行环境
- **Node.js** (18+) - 前端运行环境
- **npm** - Node.js包管理器
- **Docker** - 容器化环境
- **Docker Compose** - 容器编排工具

### 可选依赖
- **Chrome** - 自动化测试浏览器

## 📝 访问地址

启动成功后，可以通过以下地址访问服务：

| 服务 | 地址 | 说明 |
|------|------|------|
| 前端应用 | http://localhost:3000 | React前端界面 |
| 后端API | http://localhost:8080/api/v1 | REST API接口 |
| 健康检查 | http://localhost:8080/api/v1/health | 服务状态检查 |
| MySQL数据库 | localhost:3306 | 数据库连接（用户名：webtestflow，密码：123456） |

## 📋 日志查看

```bash
# 查看后端日志
tail -f logs/backend.log

# 查看前端日志
tail -f logs/frontend.log

# 同时查看所有日志
tail -f logs/*.log

# 查看Docker容器日志
docker logs webtestflow-mysql
```

## 🐛 故障排除

### 常见问题

1. **端口被占用**
   ```bash
   # 检查端口占用
   lsof -i :3000  # 前端端口
   lsof -i :8080  # 后端端口
   lsof -i :3306  # 数据库端口
   ```

2. **Docker权限问题**
   ```bash
   # 将当前用户添加到docker组
   sudo usermod -aG docker $USER
   # 重新登录或重启系统
   ```

3. **依赖安装失败**
   ```bash
   # 手动安装Go
   wget https://go.dev/dl/go1.21.5.linux-amd64.tar.gz
   sudo tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz
   echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
   
   # 手动安装Node.js
   curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash -
   sudo apt-get install -y nodejs
   ```

4. **MySQL连接失败**
   ```bash
   # 检查MySQL容器状态
   docker ps | grep mysql
   
   # 重启MySQL容器
   docker restart webtestflow-mysql
   ```

5. **容器镜像拉取失败（网络问题）**
   ```bash
   # 使用镜像源配置脚本（推荐）
   ./scripts/setup-registry-mirrors.sh
   
   # 或直接使用国内镜像源
   ./scripts/setup-registry-mirrors.sh --china
   
   # 或直接使用国际镜像源
   ./scripts/setup-registry-mirrors.sh --international
   
   # 或自动检测最佳镜像源
   ./scripts/setup-registry-mirrors.sh --auto
   
   # 手动拉取MySQL镜像（国内用户）
   docker pull registry.cn-hangzhou.aliyuncs.com/library/mysql:8.0
   docker tag registry.cn-hangzhou.aliyuncs.com/library/mysql:8.0 mysql:8.0
   
   # 手动拉取MySQL镜像（国际用户）
   docker pull mysql:8.0
   ```

## 🔄 更新和维护

### 更新依赖
```bash
# 更新后端依赖
cd backend && go mod tidy && cd ..

# 更新前端依赖
cd frontend && npm update && cd ..
```

### 清理环境
```bash
# 停止所有服务
./scripts/one-click-stop.sh

# 清理Docker资源
docker system prune -a

# 删除数据库数据（注意：会丢失所有数据）
docker-compose down -v
```

## 📞 支持

如果遇到问题，请：
1. 检查日志文件 `logs/` 目录
2. 使用 `--verbose` 参数运行脚本获取详细信息
3. 确保所有依赖都已正确安装
4. 检查防火墙和端口设置