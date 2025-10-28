@echo off
chcp 65001 >nul
echo ========================================
echo 🎨 88code UI 预览启动脚本
echo ========================================
echo.

echo [1/3] 检查 Go 环境...
where go >nul 2>&1
if %errorlevel% neq 0 (
    echo ❌ 未找到 Go 环境，请先安装 Go 1.21+
    pause
    exit /b 1
)
echo ✅ Go 环境已就绪
echo.

echo [2/3] 编译程序...
go build -o 88code-reset.exe cmd/reset/main.go
if %errorlevel% neq 0 (
    echo ❌ 编译失败
    pause
    exit /b 1
)
echo ✅ 编译成功
echo.

echo [3/3] 启动 Web 服务器...
echo.
echo ========================================
echo 🚀 服务器启动中...
echo ========================================
echo.
echo 📍 访问地址: http://localhost:8966
echo 🔑 默认密码: admin123
echo.
echo 按 Ctrl+C 停止服务器
echo ========================================
echo.

88code-reset.exe -mode=web -webport=8966

pause

