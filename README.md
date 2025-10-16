# 88code 订阅自动重置工具

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Deploy on Zeabur](https://zeabur.com/button.svg)](https://dash.zeabur.com/templates/SC0K36)

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
- ✅ **Zeabur 一键部署**: 无需服务器，一键部署到云端
- ✅ **Systemd 服务**: Linux 系统服务支持，开机自启
- ✅ **灵活配置**: 环境变量和 .env 文件配置

### CI/CD
- ✅ **自动构建**: GitHub Actions 自动构建和发布
- ✅ **多平台发布**: 自动编译并发布到 GitHub Releases
- ✅ **Docker 发布**: 自动构建并发布到 GitHub Container Registry
- ✅ **校验和验证**: 自动生成 SHA256 校验和


## 快速开始

### 前置要求

- 有效的 88code.org API Key
- （可选）Go 1.21+ 用于从源码编译
- （可选）Docker 用于容器化部署

### 方式 1: Zeabur 一键部署（最简单）

[![Deploy on Zeabur](https://zeabur.com/button.svg)](https://dash.zeabur.com/templates/SC0K36)

**部署步骤：**

1. 点击上方按钮，前往 Zeabur
2. 登录或注册 Zeabur 账号（支持 GitHub 登录）
3. 选择部署区域（推荐：Asia - Hong Kong）
4. 配置环境变量：
   - `API_KEY`: 你的 88code.org API Key **（必填）**
   - `TARGET_PLANS`: 目标套餐类型（默认：`FREE`）
   - `TZ`: 时区设置（默认：`Asia/Shanghai`）
5. 点击 "Deploy" 部署，等待 1-2 分钟完成

**或者手动从 GitHub 导入：**

1. 前往 [Zeabur Dashboard](https://dash.zeabur.com)
2. 创建新项目 → 选择 "GitHub" → 选择本仓库
3. 添加环境变量（同上）
4. Zeabur 会自动识别 Go 项目并构建部署

**优势：**
- ✅ 无需服务器，完全托管
- ✅ 自动重启和健康检查
- ✅ 内置日志查看和监控
- ✅ 免费额度足够使用
- ✅ 支持持久化存储（自动挂载 `/app/data` 和 `/app/logs`）

### 方式 2: 使用预编译二进制文件

从 [Releases 页面](https://github.com/Vulpecula-Studio/88code_reset/releases) 下载适合你系统的版本。

**支持平台**：Linux (amd64/arm64/armv7)、macOS (Intel/Apple Silicon)、Windows (amd64/arm64)

```bash
# 下载示例（以 Linux amd64 为例）
wget https://github.com/Vulpecula-Studio/88code_reset/releases/latest/download/88code_reset-linux-amd64.tar.gz
tar xzf 88code_reset-linux-amd64.tar.gz
chmod +x 88code_reset-linux-amd64

# 创建配置文件
cat > .env << EOF
API_KEY=your_api_key_here
TARGET_PLANS=FREE
TZ=Asia/Shanghai
EOF

# 测试运行
./88code_reset-linux-amd64 -mode=test

# 启动调度器
./88code_reset-linux-amd64 -mode=run
```

### 方式 3: 从源码运行

需要 Go 1.21+

```bash
# 配置 API Key
cp .env.example .env
# 编辑 .env 文件，填入你的 API Key

# 测试运行
go run cmd/reset/main.go -mode=test

# 启动调度器
go run cmd/reset/main.go -mode=run
```

### 方式 4: Docker 部署

**Docker Compose**（最简单）
```bash
# 配置 API Key
cp .env.example .env
# 编辑 .env 文件

# 启动服务
docker-compose up -d reset-scheduler

# 查看日志
docker-compose logs -f reset-scheduler
```

**Docker 命令**
```bash
docker pull ghcr.io/vulpecula-studio/88code_reset:latest

# 测试模式
docker run --rm -e API_KEY=your_key ghcr.io/vulpecula-studio/88code_reset:latest -mode=test

# 后台运行
docker run -d --name 88code-reset \
  -e API_KEY=your_key \
  -v $(pwd)/data:/app/data \
  -v $(pwd)/logs:/app/logs \
  --restart unless-stopped \
  ghcr.io/vulpecula-studio/88code_reset:latest -mode=run
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

## 核心机制

### 重置逻辑
- **触发时间**: 每天 18:50 和 23:55（可配置时区）
- **触发条件**: `resetTimes >= 2`，且该时段未执行
- **间隔保护**: 两次重置至少间隔 5 小时

### 安全机制
- **PAYGO 保护**: 双重检查，永不重置 PAYGO 类型订阅
- **锁文件**: 防止并发执行
- **状态持久化**: 防止重复重置，记录执行历史
- **僵尸锁清理**: 超过 10 分钟自动失效

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

## Docker 常用命令

```bash
# 构建镜像
docker-compose build

# 查看状态
docker-compose ps

# 查看日志
docker-compose logs -f reset-scheduler

# 重启服务
docker-compose restart reset-scheduler
```

## Systemd 服务配置（Linux）

创建服务文件 `/etc/systemd/system/88code-reset.service`：

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
Environment="API_KEY=your_api_key_here"
Environment="TZ=Asia/Shanghai"

[Install]
WantedBy=multi-user.target
```

启用服务：
```bash
sudo systemctl daemon-reload
sudo systemctl enable 88code-reset
sudo systemctl start 88code-reset
sudo systemctl status 88code-reset
```

### 本地编译

```bash
# 当前平台
go build -o reset cmd/reset/main.go

# 交叉编译示例
GOOS=linux GOARCH=amd64 go build -o reset-linux-amd64 cmd/reset/main.go
GOOS=darwin GOARCH=arm64 go build -o reset-darwin-arm64 cmd/reset/main.go
GOOS=windows GOARCH=amd64 go build -o reset-windows-amd64.exe cmd/reset/main.go

# 优化体积
CGO_ENABLED=0 go build -a -ldflags="-s -w" -o reset cmd/reset/main.go

# 测试和格式化
go test ./...
go fmt ./...
```

## 高级配置

### 时区配置

程序内置时区设置，无需依赖系统或容器时区。配置优先级：命令行参数 > 环境变量 > .env 文件 > 默认值(Asia/Shanghai)

```bash
# 命令行方式
./88code_reset-linux-amd64 -mode=run -timezone=Asia/Tokyo

# 环境变量方式
export TZ=Asia/Hong_Kong

# Docker 方式
docker run -d -e TZ=Asia/Tokyo ...
```

### 修改重置时间

编辑 [internal/scheduler/scheduler.go](internal/scheduler/scheduler.go) 中的常量：
```go
FirstResetHour   = 18
FirstResetMinute = 50
SecondResetHour   = 23
SecondResetMinute = 55
```

## 开发者信息

### CI/CD

本项目使用 GitHub Actions 自动化构建：

**触发发布**
```bash
git tag v1.0.0
git push origin v1.0.0
```

**Workflows**
- [docker.yml](.github/workflows/docker.yml) - Docker 镜像（linux/amd64, linux/arm64）
- [release.yml](.github/workflows/release.yml) - 二进制文件（Linux/macOS/Windows，多架构）

## 许可证

MIT License

## 参与贡献

欢迎提交 [Issue](https://github.com/Vulpecula-Studio/88code_reset/issues) 和 Pull Request！

## 免责声明

本工具仅供学习和研究使用，使用者需自行承担使用风险。
