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
    (cd src && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o "../$OUTPUT_DIR/${BINARY_NAME}-windows-amd64.exe" .)
    (cd src && CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o "../$OUTPUT_DIR/${BINARY_NAME}-windows-arm64.exe" .)
}

build_linux() {
    echo ""
    echo "[构建 Linux 版本]"
    (cd src && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o "../$OUTPUT_DIR/${BINARY_NAME}-linux-amd64" .)
    (cd src && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o "../$OUTPUT_DIR/${BINARY_NAME}-linux-arm64" .)
}

build_darwin() {
    echo ""
    echo "[构建 macOS 版本]"
    (cd src && CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o "../$OUTPUT_DIR/${BINARY_NAME}-darwin-amd64" .)
    (cd src && CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o "../$OUTPUT_DIR/${BINARY_NAME}-darwin-arm64" .)
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
    windows) build_windows ;;
    linux)   build_linux ;;
    darwin)  build_darwin ;;
    clean)   clean ;;
    "")
            build_all
            ;;
    *)
            echo "用法: ./build.sh [windows|linux|darwin|clean]"
            echo "  windows - 仅构建 Windows"
            echo "  linux   - 仅构建 Linux"
            echo "  darwin  - 仅构建 macOS"
            echo "  clean   - 清理构建产物"
            echo "  (无参数) - 构建所有平台"
            ;;
esac
