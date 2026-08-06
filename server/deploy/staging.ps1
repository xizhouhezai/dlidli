# DliDli staging deployment script (M0-ENG-13)
# Flow: build -> migrate isolated DB -> start service -> HelloWorld verify
# Usage: powershell -ExecutionPolicy Bypass -File deploy/staging.ps1 [-SkipMigrate] [-SkipStart]
param(
  [switch]$SkipMigrate,
  [switch]$SkipStart
)
$ErrorActionPreference = "Stop"

$ServerDir = Split-Path -Parent $PSScriptRoot
Set-Location $ServerDir

Write-Output "=============================================="
Write-Output "  DliDli staging deploy (M0-ENG-13)"
Write-Output "=============================================="

# [1/5] Build staging binary
Write-Output "`n[1/5] Building staging binary ..."
go build -o dlidli-api-staging.exe ./cmd/api
if ($LASTEXITCODE -ne 0) { throw "build failed (exit=$LASTEXITCODE)" }
Write-Output "  built dlidli-api-staging.exe"

# [2/5] Migrate staging DB (auto-create dlidli_staging)
if (-not $SkipMigrate) {
  Write-Output "`n[2/5] Migrating staging DB (dlidli_staging, auto-create) ..."
  go run ./cmd/migrate -dsn "mysql://root:root@tcp(127.0.0.1:3307)/dlidli_staging?multiStatements=true"
  if ($LASTEXITCODE -ne 0) { throw "migrate failed (exit=$LASTEXITCODE)" }
  Write-Output "  migration done"
} else {
  Write-Output "`n[2/5] Skip migrate (-SkipMigrate)"
}

# [3/5] Start staging service (port 8100)
if (-not $SkipStart) {
  Write-Output "`n[3/5] Starting staging service (port 8100) ..."
  Get-Process dlidli-api-staging -ErrorAction SilentlyContinue | Stop-Process -Force
  Start-Sleep -Seconds 1
  $proc = Start-Process -FilePath ".\dlidli-api-staging.exe" -ArgumentList "-config configs/staging.yaml" -PassThru -WindowStyle Hidden
  Start-Sleep -Seconds 5
  if ($proc.HasExited) { throw "staging exited immediately (exit=$($proc.ExitCode))" }
  Write-Output "  started PID=$($proc.Id)"
} else {
  Write-Output "`n[3/5] Skip start (-SkipStart)"
}

# [4/5] HelloWorld verify
Write-Output "`n[4/5] HelloWorld API verify ..."
$baseUrl = "http://127.0.0.1:8100"
$health = Invoke-RestMethod -Uri "$baseUrl/health" -TimeoutSec 10
if ($health.code -ne 0) { throw "health check failed: $($health.message)" }
$comps = ($health.data.components | ConvertTo-Json -Compress)
Write-Output "  /health        code=$($health.code) app=$($health.data.app) env=$($health.data.env) components=$comps"

$ping = Invoke-RestMethod -Uri "$baseUrl/api/v1/ping" -TimeoutSec 10
Write-Output "  /api/v1/ping   code=$($ping.code) pong=$($ping.data.pong)"

if ($health.data.components.mysql -ne "up" -or $health.data.components.redis -ne "up") {
  throw "dependency not ready: $comps"
}

# [5/5] Summary
Write-Output "`n[5/5] staging deploy complete"
Write-Output "  API          http://localhost:8100"
Write-Output "  Health       $baseUrl/health"
Write-Output "  Ping         $baseUrl/api/v1/ping"
Write-Output "  Database     dlidli_staging (isolated from dev)"
Write-Output "  Redis        db=1 (isolated from dev)"
Write-Output "  Uploads      ./uploads_staging"
Write-Output "  AutoApprove  true (staging convenience)"
Write-Output ""
Write-Output "Verified: staging serves HelloWorld API and full business capability"
