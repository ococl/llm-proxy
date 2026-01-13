#!/usr/bin/env pwsh
$ErrorActionPreference = "Stop"

$OUTPUT_DIR = "dist"
$BINARY_NAME = "llm-proxy"

Write-Host "========================================"
Write-Host "LLM Proxy Build Script"
Write-Host "========================================"

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

function Build-Current {
    Write-Host ""
    Write-Host "[Building for current platform]"
    Push-Location src
    $env:CGO_ENABLED = "0"
    if ($IsWindows -or $env:OS -eq "Windows_NT") {
        Write-Host "Windows amd64"
        $env:GOOS = "windows"
        $env:GOARCH = "amd64"
        go build -trimpath -ldflags "-s -w" -o "..\$OUTPUT_DIR\$BINARY_NAME-windows-amd64.exe" .
    } elseif ($IsMacOS) {
        Write-Host "macOS arm64"
        $env:GOOS = "darwin"
        $env:GOARCH = "arm64"
        go build -trimpath -ldflags "-s -w" -o "..\$OUTPUT_DIR\$BINARY_NAME-darwin-arm64" .
    } else {
        Write-Host "Linux amd64"
        $env:GOOS = "linux"
        $env:GOARCH = "amd64"
        go build -trimpath -ldflags "-s -w" -o "..\$OUTPUT_DIR\$BINARY_NAME-linux-amd64" .
    }
    Pop-Location
}

function Clean-Build {
    Write-Host "Cleaning build artifacts..."
    Remove-Item -Path "$OUTPUT_DIR\$BINARY_NAME-*" -Force -ErrorAction SilentlyContinue
}

switch ($args[0]) {
    "all" {
        Build-Windows; Build-Linux; Build-Darwin
        Write-Host ""
        Write-Host "Build complete! Output: $OUTPUT_DIR"
    }
    "windows" { Build-Windows }
    "linux" { Build-Linux }
    "darwin" { Build-Darwin }
    "clean" { Clean-Build }
    default {
        Build-Current
        Write-Host ""
        Write-Host "Build complete! Output: $OUTPUT_DIR"
    }
}
