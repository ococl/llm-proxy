@echo off
setlocal enabledelayedexpansion

set OUTPUT_DIR=dist
set BINARY_NAME=llm-proxy

echo ========================================
echo LLM Proxy 多平台构建脚本
echo ========================================

if "%1"=="" goto all
if "%1"=="all" goto all
if "%1"=="windows" goto windows
if "%1"=="linux" goto linux
if "%1"=="darwin" goto darwin
if "%1"=="clean" goto clean
goto usage

:all
call :windows
call :linux
call :darwin
echo.
echo 构建完成！输出目录: %OUTPUT_DIR%
goto end

:windows
echo.
echo [构建 Windows 版本]
set GOOS=windows
set GOARCH=amd64
go build -ldflags "-s -w" -o %OUTPUT_DIR%\%BINARY_NAME%-windows-amd64.exe .\src
set GOARCH=arm64
go build -ldflags "-s -w" -o %OUTPUT_DIR%\%BINARY_NAME%-windows-arm64.exe .\src
goto :eof

:linux
echo.
echo [构建 Linux 版本]
set GOOS=linux
set GOARCH=amd64
go build -ldflags "-s -w" -o %OUTPUT_DIR%\%BINARY_NAME%-linux-amd64 .\src
set GOARCH=arm64
go build -ldflags "-s -w" -o %OUTPUT_DIR%\%BINARY_NAME%-linux-arm64 .\src
goto :eof

:darwin
echo.
echo [构建 macOS 版本]
set GOOS=darwin
set GOARCH=amd64
go build -ldflags "-s -w" -o %OUTPUT_DIR%\%BINARY_NAME%-darwin-amd64 .\src
set GOARCH=arm64
go build -ldflags "-s -w" -o %OUTPUT_DIR%\%BINARY_NAME%-darwin-arm64 .\src
goto :eof

:clean
echo 清理构建产物...
del /q %OUTPUT_DIR%\%BINARY_NAME%-* 2>nul
goto end

:usage
echo 用法: build.bat [all^|windows^|linux^|darwin^|clean]
echo   all     - 构建所有平台 (默认)
echo   windows - 仅构建 Windows
echo   linux   - 仅构建 Linux
echo   darwin  - 仅构建 macOS
echo   clean   - 清理构建产物

:end
endlocal
