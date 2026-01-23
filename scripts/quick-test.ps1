param(
    [switch]$SkipBuild
)

$ErrorActionPreference = "Stop"

$ProjectRoot = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$SrcDir = Join-Path $ProjectRoot "src"
$DistDir = Join-Path $ProjectRoot "dist"
$BinaryPath = Join-Path $DistDir "llm-proxy-latest.exe"
$ConfigPath = Join-Path $DistDir "config.yaml"

Write-Host "=== LLM-Proxy Quick Test ===" -ForegroundColor Cyan

if (-not $SkipBuild) {
    Write-Host "`n[1/4] Building binary..." -ForegroundColor Yellow
    Push-Location $SrcDir
    go build -o $BinaryPath .
    Pop-Location
    Write-Host "[✓] Build completed" -ForegroundColor Green
} else {
    Write-Host "`n[1/4] Skipping build" -ForegroundColor Gray
}

Write-Host "`n[2/4] Starting service..." -ForegroundColor Yellow
$process = Start-Process -FilePath $BinaryPath -ArgumentList "-config", $ConfigPath -NoNewWindow -PassThru

Start-Sleep -Seconds 3

$port = 8765
$tcp = New-Object Net.Sockets.TcpClient
try {
    $tcp.Connect("localhost", $port)
    $tcp.Close()
    Write-Host "[✓] Service started on port $port" -ForegroundColor Green
} catch {
    Write-Host "[✗] Service failed to start" -ForegroundColor Red
    $process.Kill()
    exit 1
}

Write-Host "`n[3/4] Running health check..." -ForegroundColor Yellow
try {
    $response = Invoke-RestMethod -Uri "http://localhost:$port/health" -Method GET
    Write-Host "[✓] Health check passed: $($response.status)" -ForegroundColor Green
    Write-Host "    Backends: $($response.backends), Models: $($response.models)" -ForegroundColor Gray
} catch {
    Write-Host "[✗] Health check failed: $($_.Exception.Message)" -ForegroundColor Red
    $process.Kill()
    exit 1
}

Write-Host "`n[4/4] Testing simple request..." -ForegroundColor Yellow
$body = @{
    model = "deepseek/deepseek-v3.2"
    messages = @(
        @{ role = "user"; content = "Say 'test ok' in English" }
    )
    max_tokens = 10
} | ConvertTo-Json -Depth 5

try {
    $response = Invoke-RestMethod -Uri "http://localhost:$port/v1/chat/completions" `
        -Method POST `
        -Headers @{ "Authorization" = "Bearer sk-aNbDRYsSMcbdVUptFyy9yWpeN6agx"; "Content-Type" = "application/json" } `
        -Body $body `
        -TimeoutSec 30

    if ($response.choices) {
        Write-Host "[✓] Request successful" -ForegroundColor Green
        Write-Host "    Response: $($response.choices[0].message.content)" -ForegroundColor Gray
    } else {
        Write-Host "[✗] Invalid response format" -ForegroundColor Red
    }
} catch {
    Write-Host "[✗] Request failed: $($_.Exception.Message)" -ForegroundColor Red
}

Write-Host "`n=== Stopping service ===" -ForegroundColor Cyan
$process.Kill()
$process.WaitForExit()
Write-Host "[✓] Service stopped" -ForegroundColor Green

Write-Host "`n=== All tests completed ===" -ForegroundColor Green
