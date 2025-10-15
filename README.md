# 88code 订阅自动重置工具

[![CI](https://github.com/yourusername/88code_reset/actions/workflows/ci.yml/badge.svg)](https://github.com/yourusername/88code_reset/actions/workflows/ci.yml)
[![Docker Build](https://github.com/yourusername/88code_reset/actions/workflows/docker-build.yml/badge.svg)](https://github.com/yourusername/88code_reset/actions/workflows/docker-build.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

这是一个基于 Go 语言开发的自动化工具，用于在每天的固定时间自动重置 88code.org 订阅积分。支持 FREE、PRO、PLUS 等多种套餐类型，并具有 PAYGO 保护机制。

## 功能特性

- ✅ **多套餐支持**: 支持 FREE、PRO、PLUS 或自定义套餐名称
- ✅ **PAYGO 保护**: 自动检测并拒绝重置 PAYGO 类型订阅，确保安全
- ✅ **定期监控**: 每小时检查订阅状态，及时发现问题
- ✅ **自动重置**: 每天 18:50 和 23:55（北京时间）自动执行重置
- ✅ **智能检查**: 仅在 `resetTimes >= 2` 时执行重置
- ✅ **间隔控制**: 两次重置至少间隔 5 小时
- ✅ **状态持久化**: 完整记录账号信息和执行状态
- ✅ **并发保护**: 防重复执行的锁机制
- ✅ **详细日志**: 完善的日志记录系统
- ✅ **Docker 支持**: 多架构镜像（amd64, arm64, arm/v7）
- ✅ **灵活配置**: 环境变量和 .env 文件配置
- ✅ **自动构建**: GitHub Actions 自动构建和发布

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

- Go 1.21+ 或 Docker
- 有效的 88code.org API Key

### 方式 1: 直接运行

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

### 方式 2: GitHub Container Registry 镜像（最简单）

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

### 方式 3: Docker Compose（推荐）

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

重置时间：
- **第一次重置**: 每天 18:50（北京时间）
- **第二次重置**: 每天 23:55（北京时间）

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
```

支持的格式：
- `API_KEY=xxx` 或 `api-key=xxx`
- `TARGET_PLANS=xxx` 或通过命令行参数 `-plans=xxx`

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

## 开发说明

### 编译
```bash
go build -o reset cmd/reset/main.go
```

### 运行测试
```bash
go test ./...
```

### 代码格式化
```bash
go fmt ./...
```

## 注意事项

1. **时区设置**: 确保系统时区或 Docker 容器时区设置为北京时间
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

- **自动测试**: 每次推送代码都会运行 CI 测试
- **Docker 构建**: 自动构建多架构 Docker 镜像（amd64, arm64, arm/v7）
- **自动发布**: 推送到 GitHub Container Registry

### 配置 GitHub Actions

详细配置步骤请查看 [docs/GITHUB_ACTIONS.md](docs/GITHUB_ACTIONS.md)

简要步骤：
1. Fork 本项目到你的 GitHub
2. 推送代码，GitHub Actions 会自动构建
3. 镜像会自动发布到 `ghcr.io/yourusername/88code_reset`

无需额外配置 Secrets，使用内置的 `GITHUB_TOKEN` 即可。

### 使用发布的镜像

```bash
# 使用最新版本
docker pull ghcr.io/yourusername/88code_reset:latest

# 使用特定版本
docker pull ghcr.io/yourusername/88code_reset:v1.0.0

# 指定架构
docker pull --platform linux/arm64 ghcr.io/yourusername/88code_reset:latest
```

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

## 相关文档

- [README.md](README.md) - 项目文档（本文件）
- [docs/QUICKSTART.md](docs/QUICKSTART.md) - 快速开始指南
- [docs/USAGE.md](docs/USAGE.md) - 详细使用说明
- [docs/PROJECT_SUMMARY.md](docs/PROJECT_SUMMARY.md) - 项目总结
- [docs/GITHUB_ACTIONS.md](docs/GITHUB_ACTIONS.md) - GitHub Actions 配置指南
- [docs/FILES.md](docs/FILES.md) - 文件清单
- [docs/README.md](docs/README.md) - 文档索引

## 免责声明

本工具仅供学习和研究使用，使用者需自行承担使用风险。
