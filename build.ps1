#!/usr/bin/env pwsh
$ErrorActionPreference = "Stop"

$OUTPUT_DIR = "dist"
$BINARY_NAME = "llm-proxy"

Write-Host "========================================"
Write-Host "LLM Proxy Build Script"
Write-Host "========================================"

#Write-Host "清理 Go 缓存..." -ForegroundColor Gray
#go clean -cache

function Build-Windows {
    Write-Host ""
    Write-Host "[Building Windows]"
    Push-Location src
    $env:CGO_ENABLED = "0"
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"
    go build -trimpath -ldflags "-s -w" -o "..\$OUTPUT_DIR\$BINARY_NAME-windows-amd64.exe" .
    $env:GOARCH = "arm64"
    go build -trimpath -ldflags "-s -w" -o "..\$OUTPUT_DIR\$BINARY_NAME-windows-arm64.exe" .
    Pop-Location
}

function Build-Linux {
    Write-Host ""
    Write-Host "[Building Linux]"
    Push-Location src
    $env:CGO_ENABLED = "0"
    $env:GOOS = "linux"
    $env:GOARCH = "amd64"
    go build -trimpath -ldflags "-s -w" -o "..\$OUTPUT_DIR\$BINARY_NAME-linux-amd64" .
    $env:GOARCH = "arm64"
    go build -trimpath -ldflags "-s -w" -o "..\$OUTPUT_DIR\$BINARY_NAME-linux-arm64" .
    Pop-Location
}

function Build-Darwin {
    Write-Host ""
    Write-Host "[Building macOS]"
    Push-Location src
    $env:CGO_ENABLED = "0"
    $env:GOOS = "darwin"
    $env:GOARCH = "amd64"
    go build -trimpath -ldflags "-s -w" -o "..\$OUTPUT_DIR\$BINARY_NAME-darwin-amd64" .
    $env:GOARCH = "arm64"
    go build -trimpath -ldflags "-s -w" -o "..\$OUTPUT_DIR\$BINARY_NAME-darwin-arm64" .
    Pop-Location
}

function Clean-Build {
    Write-Host "清理构建产物..."
    Remove-Item -Path "$OUTPUT_DIR\$BINARY_NAME-*" -Force -ErrorAction SilentlyContinue
}

switch ($args[0]) {
    "windows" { Build-Windows }
    "linux" { Build-Linux }
    "darwin" { Build-Darwin }
    "clean" { Clean-Build }
    default {
        # 默认构建所有平台产物
        Build-Windows; Build-Linux; Build-Darwin
        Write-Host ""
        Write-Host "构建完成！输出目录: $OUTPUT_DIR"
    }
}
