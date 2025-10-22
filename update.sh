#!/bin/bash

# 88code_reset 更新脚本
# 使用方法: ./update.sh

set -e  # 遇到错误立即退出

echo "=========================================="
echo "开始更新 88code_reset"
echo "=========================================="

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 项目目录（根据实际情况修改）
PROJECT_DIR=$(cd "$(dirname "$0")" && pwd)
BINARY_NAME="88code-reset"
LOG_FILE="app.log"

cd "$PROJECT_DIR"

# 1. 检查是否有未提交的更改
echo -e "${YELLOW}[1/8] 检查本地更改...${NC}"
if [[ -n $(git status -s) ]]; then
    echo -e "${YELLOW}警告: 发现未提交的更改${NC}"
    git status -s
    read -p "是否继续？(y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "更新已取消"
        exit 1
    fi
fi

# 2. 停止正在运行的程序
echo -e "${YELLOW}[2/8] 停止正在运行的程序...${NC}"
if pgrep -f "$BINARY_NAME" > /dev/null; then
    echo "发现运行中的程序，正在停止..."
    pkill -f "$BINARY_NAME" || true
    sleep 2

    # 强制杀死（如果还在运行）
    if pgrep -f "$BINARY_NAME" > /dev/null; then
        echo "强制停止..."
        pkill -9 -f "$BINARY_NAME" || true
        sleep 1
    fi
    echo -e "${GREEN}程序已停止${NC}"
else
    echo "没有运行中的程序"
fi

# 3. 备份当前版本
echo -e "${YELLOW}[3/8] 备份当前版本...${NC}"
if [ -f "$BINARY_NAME" ]; then
    BACKUP_NAME="${BINARY_NAME}.backup.$(date +%Y%m%d_%H%M%S)"
    cp "$BINARY_NAME" "$BACKUP_NAME"
    echo -e "${GREEN}备份完成: $BACKUP_NAME${NC}"
fi

# 4. 拉取最新代码
echo -e "${YELLOW}[4/8] 拉取最新代码...${NC}"
git fetch origin
BEFORE_COMMIT=$(git rev-parse HEAD)
git pull origin main
AFTER_COMMIT=$(git rev-parse HEAD)

if [ "$BEFORE_COMMIT" = "$AFTER_COMMIT" ]; then
    echo -e "${GREEN}已是最新版本${NC}"
else
    echo -e "${GREEN}代码已更新${NC}"
    echo "变更日志:"
    git log --oneline "$BEFORE_COMMIT".."$AFTER_COMMIT"
fi

# 5. 检查 Go 环境
echo -e "${YELLOW}[5/8] 检查 Go 环境...${NC}"
if ! command -v go &> /dev/null; then
    echo -e "${RED}错误: 未找到 Go 环境${NC}"
    exit 1
fi
echo "Go 版本: $(go version)"

# 6. 安装依赖
echo -e "${YELLOW}[6/8] 安装依赖...${NC}"
go mod download

# 7. 重新编译
echo -e "${YELLOW}[7/8] 重新编译...${NC}"
go build -o "$BINARY_NAME" cmd/reset/main.go
if [ $? -eq 0 ]; then
    echo -e "${GREEN}编译成功${NC}"
    ls -lh "$BINARY_NAME"
else
    echo -e "${RED}编译失败${NC}"
    exit 1
fi

# 8. 启动程序
echo -e "${YELLOW}[8/8] 启动程序...${NC}"
nohup ./"$BINARY_NAME" -mode=run > "$LOG_FILE" 2>&1 &
NEW_PID=$!
sleep 2

# 验证程序是否启动成功
if ps -p $NEW_PID > /dev/null; then
    echo -e "${GREEN}程序启动成功 (PID: $NEW_PID)${NC}"
else
    echo -e "${RED}程序启动失败，请检查日志${NC}"
    tail -20 "$LOG_FILE"
    exit 1
fi

echo "=========================================="
echo -e "${GREEN}更新完成！${NC}"
echo "=========================================="
echo "程序状态:"
ps aux | grep "$BINARY_NAME" | grep -v grep
echo ""
echo "查看日志: tail -f $LOG_FILE"
echo "停止程序: pkill -f $BINARY_NAME"
echo "重启程序: ./update.sh"
