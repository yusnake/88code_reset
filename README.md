# 88code 订阅自动重置工具
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

这是一个基于 Go 语言开发的自动化工具，用于在每天的固定时间自动重置 88code.org 订阅积分。支持 FREE、PRO、PLUS 等多种套餐类型，并具有 PAYGO 保护机制。

## 功能特性

### 核心功能
- ✅ **多套餐支持**: 支持 FREE、PRO、PLUS 或自定义套餐名称
- ✅ **PAYGO 保护**: 自动检测并拒绝重置 PAYGO 类型订阅，确保安全
- ✅ **定期监控**: 每小时检查订阅状态，及时发现问题
- ✅ **自动重置**: 每天 18:50 和 23:55 自动执行重置
- ✅ **时区配置**: 内置时区设置，默认 UTC+8，支持自定义时区
- ✅ **智能检查**: 仅在 `resetTimes >= 2` 时执行重置
- ✅ **间隔控制**: 两次重置至少间隔 5 小时
- ✅ **状态持久化**: 完整记录账号信息和执行状态
- ✅ **并发保护**: 防重复执行的锁机制
- ✅ **详细日志**: 完善的日志记录系统

### 部署方式
- ✅ **预编译二进制**: 支持多平台（Linux、macOS、Windows）和多架构（amd64、arm64、armv7）
- ✅ **Docker 镜像**: 多架构镜像（amd64、arm64）
- ✅ **Systemd 服务**: Linux 系统服务支持，开机自启
- ✅ **灵活配置**: 环境变量和 .env 文件配置

### CI/CD
- ✅ **自动构建**: GitHub Actions 自动构建和发布
- ✅ **多平台发布**: 自动编译并发布到 GitHub Releases
- ✅ **Docker 发布**: 自动构建并发布到 GitHub Container Registry
- ✅ **校验和验证**: 自动生成 SHA256 校验和

## 项目结构

```
88code_reset/
├── cmd/
│   └── reset/
│       └── main.go          # 主程序入口
├── internal/
│   ├── api/
│   │   └── client.go        # API 客户端
│   ├── models/
│   │   └── models.go        # 数据模型
│   ├── scheduler/
│   │   └── scheduler.go     # 定时任务调度器
│   └── storage/
│       └── storage.go       # 存储管理
├── pkg/
│   └── logger/
│       └── logger.go        # 日志系统
├── data/                    # 数据目录（持久化）
│   ├── account.json         # 账号信息
│   ├── status.json          # 执行状态
│   └── reset.lock           # 锁文件
├── logs/                    # 日志目录
├── .env                     # 配置文件
├── Dockerfile               # Docker 镜像
├── docker-compose.yml       # Docker Compose 配置
└── README.md               # 本文档
```

## 快速开始

### 前置要求

- 有效的 88code.org API Key
- （可选）Go 1.21+ 用于从源码编译
- （可选）Docker 用于容器化部署

### 方式 1: 使用预编译二进制文件（推荐）

#### 下载二进制文件

从 [Releases 页面](https://github.com/yourusername/88code_reset/releases) 下载适合你系统的二进制文件：

**Linux (amd64)**
```bash
wget https://github.com/yourusername/88code_reset/releases/latest/download/88code_reset-linux-amd64.tar.gz
tar xzf 88code_reset-linux-amd64.tar.gz
chmod +x 88code_reset-linux-amd64
```

**Linux (arm64)**
```bash
wget https://github.com/yourusername/88code_reset/releases/latest/download/88code_reset-linux-arm64.tar.gz
tar xzf 88code_reset-linux-arm64.tar.gz
chmod +x 88code_reset-linux-arm64
```

**Linux (armv7)** - 适用于树莓派等设备
```bash
wget https://github.com/yourusername/88code_reset/releases/latest/download/88code_reset-linux-armv7.tar.gz
tar xzf 88code_reset-linux-armv7.tar.gz
chmod +x 88code_reset-linux-armv7
```

**macOS (Intel)**
```bash
wget https://github.com/yourusername/88code_reset/releases/latest/download/88code_reset-darwin-amd64.tar.gz
tar xzf 88code_reset-darwin-amd64.tar.gz
chmod +x 88code_reset-darwin-amd64
```

**macOS (Apple Silicon)**
```bash
wget https://github.com/yourusername/88code_reset/releases/latest/download/88code_reset-darwin-arm64.tar.gz
tar xzf 88code_reset-darwin-arm64.tar.gz
chmod +x 88code_reset-darwin-arm64
```

**Windows (amd64)**
```powershell
# 从 Releases 页面下载 88code_reset-windows-amd64.zip
# 解压后运行 88code_reset-windows-amd64.exe
```

#### 验证下载（可选）

```bash
# 下载校验和文件
wget https://github.com/yourusername/88code_reset/releases/latest/download/SHA256SUMS

# 验证文件完整性
sha256sum -c SHA256SUMS 2>&1 | grep OK
```

#### 配置和运行

1. **创建配置文件**
```bash
# 创建 .env 文件
cat > .env << EOF
API_KEY=your_api_key_here
TARGET_PLANS=FREE
TZ=Asia/Shanghai
EOF
```

2. **测试连接**
```bash
# Linux/macOS
./88code_reset-linux-amd64 -mode=test

# Windows
88code_reset-windows-amd64.exe -mode=test
```

3. **启动调度器**
```bash
# Linux/macOS
./88code_reset-linux-amd64 -mode=run

# Windows
88code_reset-windows-amd64.exe -mode=run
```

4. **后台运行（Linux/macOS）**
```bash
# 使用 nohup
nohup ./88code_reset-linux-amd64 -mode=run > output.log 2>&1 &

# 使用 systemd（推荐）
# 参见下方的 "设置 Systemd 服务" 部分
```

### 方式 2: 从源码编译

1. **克隆项目**
```bash
cd /path/to/88code_reset
```

2. **配置 API Key**
```bash
cp .env.example .env
# 编辑 .env 文件，填入你的 API Key
```

3. **测试连接**
```bash
go run cmd/reset/main.go -mode=test
```

4. **启动调度器**
```bash
go run cmd/reset/main.go -mode=run
```

### 方式 3: Docker 镜像部署

#### 使用 GitHub Container Registry 镜像

使用预构建的 Docker 镜像：

```bash
# 拉取最新镜像
docker pull ghcr.io/yourusername/88code_reset:latest

# 运行测试模式
docker run --rm \
  -e API_KEY=your_api_key_here \
  ghcr.io/yourusername/88code_reset:latest -mode=test

# 运行调度器（后台）
docker run -d \
  --name 88code-reset \
  -e API_KEY=your_api_key_here \
  -v $(pwd)/data:/app/data \
  -v $(pwd)/logs:/app/logs \
  --restart unless-stopped \
  ghcr.io/yourusername/88code_reset:latest -mode=run
```

#### 使用 Docker Compose（推荐）

1. **配置 API Key**
```bash
cp .env.example .env
# 编辑 .env 文件，填入你的 API Key
```

2. **测试模式**
```bash
docker-compose --profile test up reset-test
```

3. **启动调度器**
```bash
docker-compose up -d reset-scheduler
```

4. **查看日志**
```bash
docker-compose logs -f reset-scheduler
```

5. **停止服务**
```bash
docker-compose down
```

## 运行模式

### 1. 测试模式 (test)

测试 API 连接并获取订阅信息，**不会执行重置操作**。

```bash
go run cmd/reset/main.go -mode=test
```

输出示例：
```
【测试 1/3】测试 API 连接...
✅ API 连接测试通过

【测试 2/3】获取订阅列表...
✅ 获取到 2 个订阅

【测试 3/3】查找 FREE 订阅...
✅ 找到 FREE 订阅

目标订阅详细信息:
  ID: xxxxx
  当前积分: xx.xxxx / 20.00
  resetTimes: 2
  ...
```

### 2. 调度器模式 (run)

启动定时任务调度器，在指定时间自动执行重置。

```bash
go run cmd/reset/main.go -mode=run
```

重置时间（基于配置的时区，默认为北京时间 UTC+8）：
- **第一次重置**: 每天 18:50
- **第二次重置**: 每天 23:55

### 3. 手动重置模式 (manual)

手动触发一次重置操作，**需要确认后才会执行**。

```bash
go run cmd/reset/main.go -mode=manual
```

或跳过确认提示：
```bash
go run cmd/reset/main.go -mode=manual -yes
```

## 命令行参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-mode` | `test` | 运行模式: test/run/manual |
| `-apikey` | - | API Key（可选） |
| `-baseurl` | `https://www.88code.org` | API Base URL |
| `-plans` | `FREE` | 目标套餐名称，多个用逗号分隔（如：FREE,PRO,PLUS） |
| `-timezone` | `Asia/Shanghai` | 时区设置（如：Asia/Shanghai, Asia/Hong_Kong, UTC） |
| `-datadir` | `./data` | 数据目录 |
| `-logdir` | `./logs` | 日志目录 |
| `-yes` | `false` | 跳过确认提示（仅用于 manual 模式） |

### 示例

```bash
# 仅重置 FREE 套餐
go run cmd/reset/main.go -mode=test -plans=FREE

# 重置 PRO 套餐
go run cmd/reset/main.go -mode=run -plans=PRO

# 重置多个套餐
go run cmd/reset/main.go -mode=run -plans=FREE,PRO,PLUS

# 使用自定义时区
go run cmd/reset/main.go -mode=run -timezone=Asia/Tokyo

# 使用 UTC 时区
go run cmd/reset/main.go -mode=run -timezone=UTC
```

## 配置文件

### .env 文件

```bash
# API Key（必填）
API_KEY=88_xxxxxxxxxxxx

# API Base URL（可选）
BASE_URL=https://www.88code.org

# 目标订阅计划（可选，默认为 FREE）
# 支持多个计划，用逗号分隔
TARGET_PLANS=FREE
# TARGET_PLANS=FREE,PRO
# TARGET_PLANS=PLUS

# 时区设置（可选，默认为 Asia/Shanghai UTC+8）
# 支持标准时区名称，用于控制重置时间
# 示例：
TZ=Asia/Shanghai          # 北京/上海时区 (UTC+8)
# TZ=Asia/Hong_Kong       # 香港时区 (UTC+8)
# TZ=Asia/Tokyo           # 东京时区 (UTC+9)
# TZ=America/New_York     # 纽约时区 (UTC-5/-4)
# TZ=Europe/London        # 伦敦时区 (UTC+0/+1)
# TZ=UTC                  # 世界协调时
#
# 或使用 TIMEZONE 变量
# TIMEZONE=Asia/Shanghai
#
# 默认重置时间为北京时间：
# - 第一次: 18:50
# - 第二次: 23:55
```

支持的格式：
- `API_KEY=xxx` 或 `api-key=xxx`
- `TARGET_PLANS=xxx` 或通过命令行参数 `-plans=xxx`
- `TZ=xxx` 或 `TIMEZONE=xxx` 或通过命令行参数 `-timezone=xxx`

## 数据文件

### account.json

存储账号信息：
```json
{
  "employee_id": 12345,
  "employee_name": "用户名称",
  "employee_email": "user_email",
  "free_subscription_id": 67890,
  "current_credits": 15.1772,
  "credit_limit": 20,
  "reset_times": 2,
  "last_updated": "2025-10-15T18:50:00+08:00"
}
```

### status.json

存储执行状态：
```json
{
  "last_check_time": "2025-10-15T18:50:00+08:00",
  "first_reset_today": true,
  "second_reset_today": false,
  "last_reset_success": true,
  "last_reset_message": "重置成功",
  "consecutive_failures": 0,
  "today_date": "2025-10-15",
  "reset_times_before_reset": 2,
  "reset_times_after_reset": 1
}
```

## 日志系统

日志文件位于 `logs/` 目录，按日期命名：
- `reset_2025-10-15.log`

日志级别：
- `[INFO]` - 一般信息
- `[WARN]` - 警告信息
- `[ERROR]` - 错误信息
- `[DEBUG]` - 调试信息

## 重置逻辑

### 触发条件

1. 当前时间为 18:50 或 23:55（北京时间）
2. `resetTimes >= 2`
3. 今天该时段尚未执行过重置
4. 两次重置间隔至少 5 小时

### 执行流程

1. 获取锁，防止并发执行
2. 检查执行状态，避免重复重置
3. 获取 FREE 订阅信息
4. 检查 `resetTimes` 是否满足条件
5. 调用重置接口
6. 验证重置结果
7. 更新账号信息和执行状态
8. 释放锁

### 安全机制

- **PAYGO 保护**: 双重检查机制，永不重置 PAYGO 类型订阅
  - 第一层：在 `GetTargetSubscription()` 时过滤
  - 第二层：在 `ResetCredits()` 前再次验证
- **锁文件**: 防止多个进程同时执行
- **状态持久化**: 记录每次执行结果，防止重复
- **时间间隔检查**: 确保两次重置间隔至少 5 小时
- **僵尸锁清理**: 超过 10 分钟的锁自动失效
- **定期监控**: 每小时检查订阅状态，及时发现异常

## 故障排查

### API 连接失败

检查：
1. API Key 是否正确
2. 网络连接是否正常
3. Base URL 是否可访问

### resetTimes 不足

原因：
- 今天的两次重置次数已用完
- 需等待次日或联系客服

### 锁文件问题

如果出现"操作正在进行中"错误：
```bash
# 手动清理锁文件
rm data/reset.lock
```

## Docker 管理

### 构建镜像
```bash
docker-compose build
```

### 查看容器状态
```bash
docker-compose ps
```

### 查看实时日志
```bash
docker-compose logs -f reset-scheduler
```

### 重启服务
```bash
docker-compose restart reset-scheduler
```

### 进入容器
```bash
docker-compose exec reset-scheduler sh
```

## 设置 Systemd 服务（Linux）

将程序设置为系统服务，实现开机自启和自动重启。

1. **创建服务文件**
```bash
sudo nano /etc/systemd/system/88code-reset.service
```

2. **编辑服务配置**
```ini
[Unit]
Description=88code Subscription Auto Reset Service
After=network.target

[Service]
Type=simple
User=your_username
WorkingDirectory=/path/to/88code_reset
ExecStart=/path/to/88code_reset/88code_reset-linux-amd64 -mode=run
Restart=always
RestartSec=10
StandardOutput=append:/path/to/88code_reset/logs/service.log
StandardError=append:/path/to/88code_reset/logs/service.log

# 环境变量（可选，也可以使用 .env 文件）
Environment="API_KEY=your_api_key_here"
Environment="TZ=Asia/Shanghai"

[Install]
WantedBy=multi-user.target
```

3. **启用并启动服务**
```bash
# 重载 systemd 配置
sudo systemctl daemon-reload

# 启用服务（开机自启）
sudo systemctl enable 88code-reset

# 启动服务
sudo systemctl start 88code-reset

# 查看服务状态
sudo systemctl status 88code-reset

# 查看服务日志
sudo journalctl -u 88code-reset -f
```

4. **管理服务**
```bash
# 停止服务
sudo systemctl stop 88code-reset

# 重启服务
sudo systemctl restart 88code-reset

# 禁用服务
sudo systemctl disable 88code-reset
```

## 开发说明

### 编译

**编译当前平台**
```bash
go build -o reset cmd/reset/main.go
```

**交叉编译**
```bash
# Linux amd64
GOOS=linux GOARCH=amd64 go build -o reset-linux-amd64 cmd/reset/main.go

# Linux arm64
GOOS=linux GOARCH=arm64 go build -o reset-linux-arm64 cmd/reset/main.go

# macOS arm64 (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o reset-darwin-arm64 cmd/reset/main.go

# Windows amd64
GOOS=windows GOARCH=amd64 go build -o reset-windows-amd64.exe cmd/reset/main.go
```

**优化编译（减小体积）**
```bash
CGO_ENABLED=0 go build -a -ldflags="-s -w" -o reset cmd/reset/main.go
```

### 运行测试
```bash
go test ./...
```

### 代码格式化
```bash
go fmt ./...
```

## 时区配置

程序内置时区设置，无需依赖系统或容器时区配置。

### 配置优先级

1. 命令行参数 `-timezone`
2. 环境变量 `TZ`
3. 环境变量 `TIMEZONE`
4. `.env` 文件中的 `TZ` 或 `TIMEZONE`
5. 默认值 `Asia/Shanghai` (UTC+8)

### 常用时区示例

```bash
# 通过命令行设置
go run cmd/reset/main.go -mode=run -timezone=Asia/Tokyo

# 通过环境变量设置
export TZ=Asia/Hong_Kong
go run cmd/reset/main.go -mode=run

# 通过 .env 文件设置
echo "TZ=America/New_York" >> .env
go run cmd/reset/main.go -mode=run
```

### Docker 时区配置

```bash
# docker run 方式
docker run -d \
  -e API_KEY=your_key \
  -e TZ=Asia/Tokyo \
  ghcr.io/yourusername/88code_reset:latest -mode=run

# docker-compose.yml 方式
environment:
  - API_KEY=88_xxxxxxxxxxxx
  - TZ=Asia/Hong_Kong
```

### 重置时间说明

默认重置时间基于配置的时区：
- **第一次重置**: 18:50
- **第二次重置**: 23:55

例如，如果设置为 `Asia/Tokyo` (UTC+9)，则重置时间为东京时间 18:50 和 23:55。

## 注意事项

1. **时区设置**: 使用内置时区配置，无需修改系统或容器时区
2. **API 限制**: 遵守 88code.org 的 API 使用规范
3. **数据备份**: 定期备份 `data/` 目录
4. **日志清理**: 定期清理旧日志文件

## 常见问题

### Q: 如何更改重置时间？

A: 修改 `internal/scheduler/scheduler.go` 中的常量：
```go
FirstResetHour   = 18
FirstResetMinute = 50

SecondResetHour   = 23
SecondResetMinute = 55
```

### Q: 如何禁用某一次重置？

A: 在调度器代码中注释掉相应的检查逻辑。

### Q: 可以手动修改 status.json 吗？

A: 可以，但要确保 JSON 格式正确。通常不建议手动修改。

## CI/CD 和自动构建

本项目使用 GitHub Actions 实现自动化构建和发布：

### 支持的平台和架构

**预编译二进制文件** (通过 GitHub Releases 发布)
- Linux: amd64, arm64, armv7
- macOS: amd64 (Intel), arm64 (Apple Silicon)
- Windows: amd64, arm64

**Docker 镜像** (通过 GitHub Container Registry 发布)
- linux/amd64
- linux/arm64

### 自动化流程

1. **推送标签触发发布**
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

2. **自动构建和发布**
   - 编译所有平台的二进制文件
   - 生成 SHA256 校验和
   - 创建 GitHub Release 并上传文件
   - 构建多架构 Docker 镜像
   - 推送到 GitHub Container Registry

3. **无需额外配置**
   - 使用内置的 `GITHUB_TOKEN`
   - 无需额外配置 Secrets

### 使用发布的资源

**二进制文件**
```bash
# 从 Releases 页面下载
wget https://github.com/yourusername/88code_reset/releases/latest/download/88code_reset-linux-amd64.tar.gz
```

**Docker 镜像**
```bash
# 使用最新版本
docker pull ghcr.io/yourusername/88code_reset:latest

# 使用特定版本
docker pull ghcr.io/yourusername/88code_reset:v1.0.0

# 指定架构
docker pull --platform linux/arm64 ghcr.io/yourusername/88code_reset:latest
```

### GitHub Actions Workflows

本项目包含两个主要的 workflow：

1. **[docker.yml](.github/workflows/docker.yml)** - Docker 镜像构建和发布
   - 触发条件: 推送到 main 分支、推送标签、手动触发
   - 输出: GitHub Container Registry 镜像

2. **[release.yml](.github/workflows/release.yml)** - 二进制文件编译和发布
   - 触发条件: 推送 v*.*.* 标签、手动触发
   - 输出: GitHub Releases 二进制文件和校验和

## 许可证

MIT License

## 贡献

欢迎提交 Issue 和 Pull Request！

开发流程：
1. Fork 本项目
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 创建 Pull Request

## 相关链接

- [GitHub Issues](https://github.com/Vulpecula-Studio/88code_reset/issues) - 问题反馈
- [GitHub Actions](https://github.com/Vulpecula-Studio/88code_reset/actions) - CI/CD 状态

## 免责声明

本工具仅供学习和研究使用，使用者需自行承担使用风险。
