# 88code 订阅重置工具 - 重构版本

## 🎉 重构完成

本项目已完成从环境变量配置到 Web 界面管理的完全重构。

## ✨ 新功能特性

### 核心改进

1. **Web 管理界面** - 无需重启 Docker 即可管理 Token 和配置
2. **零轮询设计** - 完全移除 5 分钟订阅检查，只在重置时请求 API
3. **自动获取订阅** - 添加 Token 时自动获取并展示订阅详情
4. **独立阈值配置** - 第一次重置 70%，第二次重置 100%，均可调整
5. **双重启用开关** - 第一次和第二次重置都可独立启用/禁用
6. **配置热重载** - 更新配置立即生效，无需重启
7. **手动重置功能** - 支持通过 Web 界面手动触发重置

## 📦 安装和编译

### 1. 下载依赖

```bash
cd D:\Cursor-test\88code_reset
go mod tidy
```

### 2. 本地编译

```bash
go build -o reset.exe ./cmd/reset
```

### 3. Docker 构建

```bash
docker-compose build
```

## 🚀 快速开始

### 方式一：Docker Compose（推荐）

1. **设置管理员密码**（可选，默认为 `admin123`）

创建或编辑 `.env` 文件：

```env
WEB_ADMIN_TOKEN=your_secure_password_here
```

2. **启动服务**

```bash
# 启动 Web 管理模式（新版）
docker-compose up -d reset-web

# 查看日志
docker-compose logs -f reset-web
```

3. **访问管理界面**

打开浏览器访问: `http://localhost:8966`

输入管理员 Token: `your_secure_password_here`（或默认的 `admin123`）

### 方式二：直接运行

```bash
# Windows
set WEB_ADMIN_TOKEN=your_password
.\reset.exe -mode=web

# Linux/Mac
export WEB_ADMIN_TOKEN=your_password
./reset -mode=web
```

## 🎯 使用流程

### 1. 添加 Token

在 Web 界面中：
1. 点击 "添加新 Token"
2. 输入 Token 名称（例如：账号A）
3. 输入 API Key（sk-ant-xxxxx）
4. 点击"添加"

系统会自动：
- 验证 API Key 有效性
- 获取关联的订阅信息
- 显示用户名、邮箱、积分等详情

### 2. 配置重置规则

在"重置配置"区域：
- **第一次重置**：设置是否启用、阈值（默认 70%）
- **第二次重置**：设置是否启用、阈值（默认 100%）
- 点击"保存配置"立即生效

### 3. 自动重置

系统会根据配置在指定时间自动重置：
- 第一次：18:50（如果启用且积分 < 70%）
- 第二次：23:55（如果启用且积分 < 100%）

### 4. 手动重置

随时可以：
- 点击单个 Token 的"重置"按钮
- 点击全局"手动触发重置"按钮

## 📋 API 接口文档

### 认证

所有 API 请求需要在 Header 中添加：

```
Authorization: Bearer <WEB_ADMIN_TOKEN>
```

### 主要端点

#### 获取系统状态

```bash
GET /api/status
```

返回：当前时间、下次重置时间、Token 数量等

#### 获取 Token 列表

```bash
GET /api/tokens
```

#### 添加 Token

```bash
POST /api/tokens
Content-Type: application/json

{
  "api_key": "sk-ant-xxxxx",
  "name": "账号A"
}
```

#### 刷新订阅信息

```bash
PUT /api/tokens/{token_id}/refresh
```

#### 手动重置

```bash
PUT /api/tokens/{token_id}/reset
Content-Type: application/json

{
  "reset_type": "second"
}
```

#### 批量重置

```bash
POST /api/reset/trigger
Content-Type: application/json

{
  "reset_type": "first"
}
```

## 🔧 配置说明

### 环境变量

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `WEB_ADMIN_TOKEN` | Web 管理员密码 | `admin123` |
| `WEB_PORT` | Web 服务器端口 | `8966` |

### 数据文件

所有数据保存在 `./data` 目录：

- `tokens.json` - Token 列表和订阅信息
- `config.json` - 动态配置（阈值、启用开关等）
- `status.json` - 执行状态记录
- `account.json` - 账号信息（传统模式）

### 日志文件

日志保存在 `./logs` 目录：

- `reset.log` - 主日志文件
- 按天分割，自动清理

## 🔄 从旧版本迁移

### 兼容性

新版本完全向后兼容，保留了原有的 `run`、`test`、`list` 模式。

### 迁移步骤

1. **继续使用旧模式**（可选）

```bash
# 使用传统的环境变量方式
docker-compose --profile legacy up -d reset-scheduler
```

2. **迁移到新模式**（推荐）

```bash
# 停止旧服务
docker-compose down

# 启动新服务
docker-compose up -d reset-web

# 在 Web 界面添加原来环境变量中的 API Key
```

## 🛠️ 运行模式

### Web 模式（新版，推荐）

```bash
./reset -mode=web
```

- 启动 Web 管理界面
- 使用 Token 管理器
- 配置热重载
- 端口：8966

### Run 模式（传统）

```bash
./reset -mode=run -apikey=sk-ant-xxxxx
```

- 使用环境变量配置
- 传统调度器
- 与旧版本完全兼容

### Test 模式

```bash
./reset -mode=test -apikey=sk-ant-xxxxx
```

- 测试 API 连接
- 显示订阅信息
- 不执行重置

### List 模式

```bash
./reset -mode=list
```

- 列出历史账号信息

## 📊 重构统计

### 代码变更

- **新增文件**: 9 个
  - `internal/config/dynamic.go` (200 行)
  - `internal/token/storage.go` (220 行)
  - `internal/token/manager.go` (350 行)
  - `internal/web/server.go` (200 行)
  - `internal/web/handlers.go` (270 行)
  - `internal/web/middleware.go` (80 行)
  - `internal/web/static/index.html` (650 行)

- **修改文件**: 6 个
  - `internal/models/models.go` (+100 行)
  - `internal/reset/reset.go` (修改 30 行)
  - `internal/scheduler/*.go` (删除 100+ 行)
  - `cmd/reset/main.go` (完全重构，+250 行)
  - `go.mod` (+1 依赖)
  - `Dockerfile` (+2 行)
  - `docker-compose.yml` (重构)

- **总计**: 新增 ~2000 行，删除 ~100 行

### 功能对比

| 功能 | 旧版本 | 新版本 |
|------|--------|--------|
| API 轮询频率 | 每 5 分钟 | 0（仅重置时） |
| Token 管理方式 | 环境变量 | Web 界面 |
| 配置更新 | 需要重启 | 实时生效 |
| 订阅信息获取 | 定时轮询 | 添加时 + 手动刷新 |
| 第一次重置 | 可启用/禁用 | ✓ |
| 第二次重置 | 强制启用 | 可启用/禁用 |
| 第一次阈值 | 83% | 70%（可调） |
| 第二次阈值 | 100%（固定） | 100%（可调） |
| 手动重置 | ✗ | ✓ |
| 管理界面 | ✗ | ✓ |

## 🔒 安全建议

1. **修改默认密码**

```env
WEB_ADMIN_TOKEN=use_a_strong_password_here
```

2. **仅本地访问**

如果只在本地使用，将 docker-compose.yml 中的端口改为：

```yaml
ports:
  - "127.0.0.1:8966:8966"
```

3. **使用反向代理**

生产环境建议通过 Nginx 等反向代理访问，并启用 HTTPS。

## 📝 注意事项

1. **数据持久化**: 确保 `./data` 目录已挂载到 Docker 容器
2. **时区设置**: 默认使用 `Asia/Shanghai`，可在配置中修改
3. **日志管理**: 日志文件会自动按天分割
4. **并发控制**: 使用文件锁防止重复执行
5. **PAYGO 保护**: 多层检查防止误重置按量付费订阅

## 🐛 故障排查

### Web 界面无法访问

1. 检查端口是否被占用
2. 查看日志：`docker-compose logs reset-web`
3. 确认防火墙设置

### Token 添加失败

1. 检查 API Key 格式是否正确
2. 确认网络可以访问 `www.88code.org`
3. 查看详细错误信息

### 重置未执行

1. 检查 Token 是否启用
2. 确认重置配置已保存
3. 查看系统状态中的"下次重置时间"
4. 检查日志文件

## 📄 许可证

MIT License

## 👨‍💻 作者

88code Reset Tool Team

## 🙏 致谢

感谢所有贡献者和用户的支持！
