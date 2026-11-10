#!/usr/bin/env bash

# Penetration-testing-AI 一键部署与启动脚本：检查依赖 → 准备 venv → 编译 → 启动。
# 用法：./run.sh [--http|--https] [其他传递给二进制的参数]

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT_DIR"

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

info() { echo -e "${BLUE}[*]${NC} $1"; }
success() { echo -e "${GREEN}[+]${NC} $1"; }
warn() { echo -e "${YELLOW}[!]${NC} $1"; }
error() { echo -e "${RED}[-]${NC} $1"; }

BINARY_NAME="penetration-testing-ai"
CONFIG_FILE="$ROOT_DIR/config.yaml"
EXAMPLE_CONFIG_FILE="$ROOT_DIR/config.example.yaml"
VENV_DIR="$ROOT_DIR/venv"
REQUIREMENTS_FILE="$ROOT_DIR/requirements.txt"
GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"
PIP_INDEX_URL="${PIP_INDEX_URL:-https://pypi.tuna.tsinghua.edu.cn/simple}"

check_go() {
    if ! command -v go >/dev/null 2>&1; then
        error "未找到 go，请先安装 Go 1.25+"
        exit 1
    fi
    success "Go: $(go version | awk '{print $3}')"
}

check_python() {
    if command -v python3 >/dev/null 2>&1; then
        PYTHON_BIN=python3
    elif command -v python >/dev/null 2>&1; then
        PYTHON_BIN=python
    else
        warn "未找到 python，部分 Python 工具配方将不可用"
        return 0
    fi
    success "Python: $($PYTHON_BIN --version 2>&1 | awk '{print $2}')"
}

prepare_config() {
    if [ ! -f "$CONFIG_FILE" ]; then
        if [ -f "$EXAMPLE_CONFIG_FILE" ]; then
            cp "$EXAMPLE_CONFIG_FILE" "$CONFIG_FILE"
            success "已根据 config.example.yaml 生成 config.yaml，请先填写 ai 通道后再启动"
        else
            warn "未找到配置模板，程序将自行生成默认配置"
        fi
    fi
}

prepare_venv() {
    if [ -z "${PYTHON_BIN:-}" ] || [ ! -f "$REQUIREMENTS_FILE" ]; then
        return 0
    fi
    if [ ! -x "$VENV_DIR/bin/python" ]; then
        info "创建 Python 虚拟环境 venv/"
        "$PYTHON_BIN" -m venv "$VENV_DIR"
    fi
    info "安装 Python 依赖（首次较慢）"
    "$VENV_DIR/bin/python" -m pip install -q --upgrade pip -i "$PIP_INDEX_URL" >/dev/null 2>&1 || true
    "$VENV_DIR/bin/python" -m pip install -q -r "$REQUIREMENTS_FILE" -i "$PIP_INDEX_URL" \
        && success "Python 依赖就绪" \
        || warn "Python 依赖安装失败，可稍后手动执行：venv/bin/pip install -r requirements.txt"
}

build() {
    info "下载 Go 依赖"
    GOPROXY="$GOPROXY" go mod download
    info "编译二进制 $BINARY_NAME"
    GOPROXY="$GOPROXY" go build -o "$BINARY_NAME" ./cmd/server
    success "编译完成"
}

main() {
    echo ""
    echo "=========================================="
    echo "  Penetration-testing-AI 部署与启动"
    echo "  默认 HTTP；需要 HTTPS 时: ./run.sh --https"
    echo "=========================================="
    echo ""

    check_go
    check_python
    prepare_config
    prepare_venv
    build

    echo ""
    info "启动服务……（Ctrl+C 停止）"
    exec "./$BINARY_NAME" -config "$CONFIG_FILE" "$@"
}

main "$@"
