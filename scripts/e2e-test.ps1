#!/usr/bin/env pwsh
# ============================================================================
# LLM-Proxy 端到端测试脚本
# ============================================================================
# 使用方法:
#   .\scripts\e2e-test.ps1                    # 运行所有测试
#   .\scripts\e2e-test.ps1 -HealthCheck       # 仅健康检查
#   .\scripts\e2e-test.ps1 -NormalRequest     # 仅正常请求测试
#   .\scripts\e2e-test.ps1 -StreamingRequest  # 仅流式请求测试
#   .\scripts\e2e-test.ps1 -Verbose           # 详细输出
# ============================================================================

[CmdletBinding(DefaultParameterSetName = "All")]
param(
    [Parameter(ParameterSetName = "All")]
    [switch]$All = $true,

    [Parameter(ParameterSetName = "HealthCheck")]
    [switch]$HealthCheck,

    [Parameter(ParameterSetName = "NormalRequest")]
    [switch]$NormalRequest,

    [Parameter(ParameterSetName = "StreamingRequest")]
    [switch]$StreamingRequest,

    [switch]$Verbose
)

# ============================================================================
# 配置
# ============================================================================
$ScriptDir = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$ConfigPath = "$ScriptDir/dist/config.yaml"
$BinaryPath = "$ScriptDir/dist/llm-proxy-latest.exe"
$LogDir = "$ScriptDir/logs"
$BaseURL = "http://localhost:8765"
$APIKey = "sk-aNbDRYsSMcbdVUptFyy9yWpeN6agx"

# 测试超时（毫秒）
$HealthTimeout = 5000
$RequestTimeout = 30000
$StreamTimeout = 60000

# 颜色定义
$Green = "#2ECC71"
$Red = "#E74C3C"
$Yellow = "#F1C40F"
$Blue = "#3498DB"
$Reset = "#FFFFFF"

# ============================================================================
# 辅助函数
# ============================================================================

function Write-Header {
    param([string]$Title)
    Write-Host "`n========================================" -ForegroundColor Cyan
    Write-Host " $Title" -ForegroundColor Cyan
    Write-Host "========================================" -ForegroundColor Cyan
}

function Write-Success {
    param([string]$Message)
    Write-Host "[✓] " -ForegroundColor Green -NoNewline
    Write-Host $Message -ForegroundColor White
}

function Write-Failure {
    param([string]$Message)
    Write-Host "[✗] " -ForegroundColor Red -NoNewline
    Write-Host $Message -ForegroundColor White
}

function Write-Info {
    param([string]$Message)
    Write-Host "[i] " -ForegroundColor Blue -NoNewline
    Write-Host $Message -ForegroundColor White
}

function Write-Warning {
    param([string]$Message)
    Write-Host "[!] " -ForegroundColor Yellow -NoNewline
    Write-Host $Message -ForegroundColor White
}

function Test-Port {
    param([int]$Port)
    $tcp = New-Object Net.Sockets.TcpClient
    try {
        $tcp.Connect("localhost", $Port)
        return $true
    } catch {
        return $false
    } finally {
        $tcp.Close()
    }
}

function Wait-ForService {
    param(
        [int]$Port = 8765,
        [int]$Timeout = 10000,
        [int]$Interval = 500
    )
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    while ($sw.ElapsedMilliseconds -lt $Timeout) {
        if (Test-Port -Port $Port) {
            return $true
        }
        Start-Sleep -Milliseconds $Interval
    }
    return $false
}

function Invoke-APIRequest {
    param(
        [string]$Endpoint,
        [string]$Method = "GET",
        [string]$Body = $null,
        [int]$Timeout = 30000,
        [switch]$Stream = $false
    )

    $headers = @{
        "Authorization" = "Bearer $APIKey"
        "Content-Type" = "application/json"
    }

    $params = @{
        Uri = "$BaseURL$Endpoint"
        Method = $Method
        Headers = $headers
        TimeoutSec = [int]($Timeout / 1000)
    }

    if ($Body) {
        $params.Body = $Body
    }

    if ($Stream) {
        $params.AllowAutoRedirect = $true
    }

    try {
        if ($Stream) {
            $response = Invoke-WebRequest @params -UseBasicParsing
        } else {
            $response = Invoke-RestMethod @params
        }
        return @{
            Success = $true
            Data = $response
            StatusCode = $response.StatusCode
        }
    } catch {
        $errorObj = $_.Exception.Response
        $statusCode = if ($errorObj) { [int]$errorObj.StatusCode } else { 0 }
        $responseBody = if ($_.ErrorDetails) { $_.ErrorDetails.Message } else { $_.Exception.Message }

        return @{
            Success = $false
            Error = $responseBody
            StatusCode = $statusCode
        }
    }
}

# ============================================================================
# 测试用例
# ============================================================================

function Test-HealthCheck {
    Write-Header "健康检查测试"

    $result = Invoke-APIRequest -Endpoint "/health" -Timeout $HealthTimeout

    if ($result.Success) {
        Write-Success "服务健康检查通过"
        Write-Info "状态: $($result.Data.status)"
        Write-Info "后端数量: $($result.Data.backends)"
        Write-Info "模型数量: $($result.Data.models)"

        if ($result.Data.status -eq "healthy") {
            Write-Success "服务运行正常"
            return $true
        } else {
            Write-Failure "服务状态异常"
            return $false
        }
    } else {
        Write-Failure "健康检查失败: $($result.Error)"
        return $false
    }
}

function Test-NormalRequest {
    Write-Header "正常请求测试"

    $testModels = @(
        @{ Model = "deepseek/deepseek-v3.2"; Description = "DeepSeek V3" },
        @{ Model = "z-ai/glm-4.7"; Description = "GLM-4.7" }
    )

    $passed = 0
    $failed = 0

    foreach ($testModel in $testModels) {
        Write-Info "测试模型: $($testModel.Description) ($($testModel.Model))"

        $body = @{
            model = $testModel.Model
            messages = @(
                @{ role = "user"; content = "你好，请用一句话回答。" }
            )
            max_tokens = 50
        } | ConvertTo-Json -Depth 10

        $result = Invoke-APIRequest `
            -Endpoint "/v1/chat/completions" `
            -Method "POST" `
            -Body $body `
            -Timeout $RequestTimeout

        if ($result.Success -and $result.Data.choices) {
            Write-Success "请求成功"
            Write-Info "  回复长度: $($result.Data.choices[0].message.content.Length) 字符"
            Write-Info "  Token使用: $($result.Data.usage.total_tokens)"
            $passed++
        } else {
            Write-Failure "请求失败: $($result.Error)"
            $failed++
        }
    }

    if ($passed -gt 0 -and $failed -eq 0) {
        Write-Success "正常请求测试全部通过 ($passed/$($testModels.Count))"
        return $true
    } else {
        Write-Failure "正常请求测试部分失败 ($passed/$($testModels.Count))"
        return $false
    }
}

function Test-StreamingRequest {
    Write-Header "流式请求测试"

    $testModels = @(
        @{ Model = "deepseek/deepseek-v3.2"; Description = "DeepSeek V3 (流式)" }
    )

    $passed = 0
    $failed = 0

    foreach ($testModel in $testModels) {
        Write-Info "测试流式模型: $($testModel.Description)"

        $body = @{
            model = $testModel.Model
            messages = @(
                @{ role = "user"; content = "从1数到3" }
            )
            max_tokens = 100
            stream = $true
        } | ConvertTo-Json -Depth 10

        try {
            $response = Invoke-WebRequest `
                -Uri "$BaseURL/v1/chat/completions" `
                -Method "POST" `
                -Headers @{
                    "Authorization" = "Bearer $APIKey"
                    "Content-Type" = "application/json"
                } `
                -Body $body `
                -TimeoutSec ($StreamTimeout / 1000) `
                -UseBasicParsing

            if ($response.StatusCode -eq 200) {
                $content = $response.Content
                # 计算流式块数量
                $chunks = ($content -split "`n" | Where-Object { $_ -match "^data:" }).Count
                Write-Success "流式请求成功"
                Write-Info "  收到 $chunks 个数据块"
                $passed++
            } else {
                Write-Failure "流式请求失败: HTTP $($response.StatusCode)"
                $failed++
            }
        } catch {
            Write-Failure "流式请求异常: $($_.Exception.Message)"
            $failed++
        }
    }

    if ($passed -gt 0 -and $failed -eq 0) {
        Write-Success "流式请求测试全部通过 ($passed/$($testModels.Count))"
        return $true
    } else {
        Write-Failure "流式请求测试部分失败 ($passed/$($testModels.Count))"
        return $false
    }
}

function Test-ErrorHandling {
    Write-Header "错误处理测试"

    # 测试无效模型
    $body = @{
        model = "invalid/model-not-exist"
        messages = @(
            @{ role = "user"; content = "Test" }
        )
    } | ConvertTo-Json -Depth 5

    $result = Invoke-APIRequest `
        -Endpoint "/v1/chat/completions" `
        -Method "POST" `
        -Body $body `
        -Timeout $RequestTimeout

    if (-not $result.Success) {
        Write-Success "无效模型请求正确返回错误 (HTTP $($result.StatusCode))"
        return $true
    } else {
        Write-Failure "无效模型请求应该返回错误"
        return $false
    }
}

function Test-Logging {
    Write-Header "日志验证"

    $logFiles = @(
        "$LogDir/general.log"
        "$LogDir/requests/request.log"
    )

    $foundLogs = $false
    foreach ($logFile in $logFiles) {
        if (Test-Path $logFile) {
            $size = (Get-Item $logFile).Length
            Write-Info "日志文件存在: $logFile ($size bytes)"
            $foundLogs = $true

            # 显示最后几行
            if ($size -gt 0) {
                Write-Info "日志预览 (最后5行):"
                Get-Content $logFile -Tail 5 | ForEach-Object {
                    Write-Host "  $_" -ForegroundColor Gray
                }
            }
        }
    }

    if ($foundLogs) {
        Write-Success "日志系统正常"
        return $true
    } else {
        Write-Warning "未找到日志文件"
        return $false
    }
}

# ============================================================================
# 主流程
# ============================================================================

function Start-E2ETest {
    Write-Host "`n" -NoNewline
    Write-Host "  ╔══════════════════════════════════════════════════╗" -ForegroundColor Cyan
    Write-Host "  ║          LLM-Proxy 端到端测试套件                 ║" -ForegroundColor Cyan
    Write-Host "  ╚══════════════════════════════════════════════════╝" -ForegroundColor Cyan

    # 检查前置条件
    Write-Header "环境检查"

    if (-not (Test-Path $BinaryPath)) {
        Write-Failure "未找到二进制文件: $BinaryPath"
        Write-Info "请先运行: cd src && go build -o ../dist/llm-proxy-latest.exe ."
        exit 1
    }
    Write-Success "二进制文件存在: $BinaryPath"

    if (-not (Test-Path $ConfigPath)) {
        Write-Failure "未找到配置文件: $ConfigPath"
        exit 1
    }
    Write-Success "配置文件存在: $ConfigPath"

    # 检查服务是否已运行
    if (Test-Port -Port 8765) {
        Write-Warning "检测到端口 8765 已被占用，假设服务已运行"
        $serviceRunning = $true
    } else {
        $serviceRunning = $false
    }

    # 启动服务（如果未运行）
    $process = $null
    if (-not $serviceRunning) {
        Write-Info "启动服务..."
        $process = Start-Process -FilePath $BinaryPath `
            -ArgumentList "-config", $ConfigPath `
            -NoNewWindow `
            -PassThru

        if (-not (Wait-ForService -Port 8765 -Timeout 10000)) {
            Write-Failure "服务启动失败"
            exit 1
        }
        Write-Success "服务已启动 (PID: $($process.Id))"
    }

    # 创建日志目录
    if (-not (Test-Path $LogDir)) {
        New-Item -ItemType Directory -Force -Path $LogDir | Out-Null
        Write-Info "创建日志目录: $LogDir"
    }

    # 执行测试
    $results = @{}

    if ($HealthCheck -or $All) {
        $results["HealthCheck"] = Test-HealthCheck
    }

    if ($NormalRequest -or $All) {
        $results["NormalRequest"] = Test-NormalRequest
    }

    if ($StreamingRequest -or $All) {
        $results["StreamingRequest"] = Test-StreamingRequest
    }

    $results["ErrorHandling"] = Test-ErrorHandling
    $results["Logging"] = Test-Logging

    # 停止服务（如果由我们启动）
    if ($process -and -not $serviceRunning) {
        Write-Header "停止服务"
        $process.Kill()
        $process.WaitForExit()
        Write-Success "服务已停止"
    }

    # 输出测试报告
    Write-Header "测试报告"

    $total = $results.Count
    $passed = ($results.Values | Where-Object { $_ }).Count
    $failed = $total - $passed

    Write-Host "  总测试数: $total" -ForegroundColor White
    Write-Host "  通过: " -ForegroundColor Green -NoNewline
    Write-Host $passed -ForegroundColor White
    Write-Host "  失败: " -ForegroundColor Red -NoNewline
    Write-Host $failed -ForegroundColor White
    Write-Host "  通过率: " -ForegroundColor Blue -NoNewline
    Write-Host ("{0:P0}" -f ($passed / $total)) -ForegroundColor White

    if ($failed -gt 0) {
        Write-Host "`n失败的测试:" -ForegroundColor Red
        foreach ($test in $results.GetEnumerator()) {
            if (-not $test.Value) {
                Write-Host "  - $($test.Key)" -ForegroundColor Red
            }
        }
    }

    Write-Host "`n" -NoNewline
    if ($failed -eq 0) {
        Write-Host "  ╔════════════════════════════════════════╗" -ForegroundColor Green
        Write-Host "  ║          所有测试通过! ✓               ║" -ForegroundColor Green
        Write-Host "  ╚════════════════════════════════════════╝" -ForegroundColor Green
        exit 0
    } else {
        Write-Host "  ╔════════════════════════════════════════╗" -ForegroundColor Red
        Write-Host "  ║          部分测试失败! ✗               ║" -ForegroundColor Red
        Write-Host "  ╚════════════════════════════════════════╝" -ForegroundColor Red
        exit 1
    }
}

# 运行测试
Start-E2ETest
