#!/bin/bash

OUTPUT_DIR="dist"
BINARY_NAME="llm-proxy"

echo "========================================"
echo "LLM Proxy 多平台构建脚本"
echo "========================================"

# 检测操作系统
detect_os() {
    case "$(uname -s)" in
        Linux*)     echo "linux" ;;
        Darwin*)    echo "darwin" ;;
        CYGWIN*|MINGW*|MSYS*) echo "windows" ;;
        *)          echo "unknown" ;;
    esac
}

CURRENT_OS=$(detect_os)

build_windows() {
    echo ""
    echo "[构建 Windows 版本]"
    GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o "$OUTPUT_DIR/${BINARY_NAME}-windows-amd64.exe" ./src
    GOOS=windows GOARCH=arm64 go build -ldflags "-s -w" -o "$OUTPUT_DIR/${BINARY_NAME}-windows-arm64.exe" ./src
}

build_linux() {
    echo ""
    echo "[构建 Linux 版本]"
    GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o "$OUTPUT_DIR/${BINARY_NAME}-linux-amd64" ./src
    GOOS=linux GOARCH=arm64 go build -ldflags "-s -w" -o "$OUTPUT_DIR/${BINARY_NAME}-linux-arm64" ./src
}

build_darwin() {
    echo ""
    echo "[构建 macOS 版本]"
    GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w" -o "$OUTPUT_DIR/${BINARY_NAME}-darwin-amd64" ./src
    GOOS=darwin GOARCH=arm64 go build -ldflags "-s -w" -o "$OUTPUT_DIR/${BINARY_NAME}-darwin-arm64" ./src
}

build_current() {
    echo ""
    echo "[构建当前系统版本]"
    case "$CURRENT_OS" in
        linux)
            echo "Linux amd64"
            GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o "$OUTPUT_DIR/${BINARY_NAME}-linux-amd64" ./src
            ;;
        darwin)
            echo "macOS arm64"
            GOOS=darwin GOARCH=arm64 go build -ldflags "-s -w" -o "$OUTPUT_DIR/${BINARY_NAME}-darwin-arm64" ./src
            ;;
        windows)
            echo "Windows amd64"
            GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o "$OUTPUT_DIR/${BINARY_NAME}-windows-amd64.exe" ./src
            ;;
        *)
            echo "不支持的操作系统: $CURRENT_OS"
            exit 1
            ;;
    esac
}

build_all() {
    build_windows
    build_linux
    build_darwin
    echo ""
    echo "构建完成！输出目录: $OUTPUT_DIR"
}

clean() {
    echo "清理构建产物..."
    rm -f "$OUTPUT_DIR/${BINARY_NAME}"-*
}

case "${1:-}" in
    all)     build_all ;;
    windows) build_windows ;;
    linux)   build_linux ;;
    darwin)  build_darwin ;;
    clean)   clean ;;
    "")
            build_current
            echo ""
            echo "构建完成！输出目录: $OUTPUT_DIR"
            ;;
    *)
            echo "用法: ./build.sh [all|windows|linux|darwin|clean]"
            echo "  all     - 构建所有平台"
            echo "  windows - 仅构建 Windows"
            echo "  linux   - 仅构建 Linux"
            echo "  darwin  - 仅构建 macOS"
            echo "  clean   - 清理构建产物"
            echo "  (无参数) - 仅构建当前系统"
            ;;
esac
