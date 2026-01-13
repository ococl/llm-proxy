#!/bin/bash

OUTPUT_DIR="dist"
BINARY_NAME="llm-proxy"

echo "========================================"
echo "LLM Proxy 多平台构建脚本"
echo "========================================"

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

case "${1:-all}" in
    all)     build_all ;;
    windows) build_windows ;;
    linux)   build_linux ;;
    darwin)  build_darwin ;;
    clean)   clean ;;
    *)
        echo "用法: ./build.sh [all|windows|linux|darwin|clean]"
        echo "  all     - 构建所有平台 (默认)"
        echo "  windows - 仅构建 Windows"
        echo "  linux   - 仅构建 Linux"
        echo "  darwin  - 仅构建 macOS"
        echo "  clean   - 清理构建产物"
        ;;
esac
