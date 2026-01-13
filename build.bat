@echo off
setlocal enabledelayedexpansion

set OUTPUT_DIR=dist
set BINARY_NAME=llm-proxy

echo ========================================
echo LLM Proxy 多平台构建脚本
echo ========================================

:: 检测操作系统
if "%OS%"=="Windows_NT" (
    set IS_WINDOWS=1
) else (
    set IS_WINDOWS=0
)

:: 默认构建当前系统
if "%1"=="" goto detect_os
if "%1"=="all" goto all
if "%1"=="windows" goto windows
if "%1"=="linux" goto linux
if "%1"=="darwin" goto darwin
if "%1"=="clean" goto clean
goto usage

:detect_os
if %IS_WINDOWS%==1 (
    call :windows
) else (
    call :build_current
)
echo.
echo 构建完成！输出目录: %OUTPUT_DIR%
goto end

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

:build_current
echo.
echo [构建当前系统版本]
if %IS_WINDOWS%==1 (
    echo Windows amd64
    set GOOS=windows
    set GOARCH=amd64
    go build -ldflags "-s -w" -o %OUTPUT_DIR%\%BINARY_NAME%-windows-amd64.exe .\src
) else (
    echo Linux amd64
    set GOOS=linux
    set GOARCH=amd64
    go build -ldflags "-s -w" -o %OUTPUT_DIR%\%BINARY_NAME%-linux-amd64 .\src
)
goto :eof

:clean
echo 清理构建产物...
del /q %OUTPUT_DIR%\%BINARY_NAME%-* 2>nul
goto end

:usage
echo 用法: build.bat [all^|windows^|linux^|darwin^|clean]
echo   all     - 构建所有平台
echo   windows - 仅构建 Windows
echo   linux   - 仅构建 Linux
echo   darwin  - 仅构建 macOS
echo   clean   - 清理构建产物
echo   (无参数) - 仅构建当前系统

:end
endlocal
