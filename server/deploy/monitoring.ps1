# DliDli local monitoring script (M1-REL-02)
# Downloads portable Prometheus + Grafana (no Docker required), starts/stops/stats the stack.
# Flow: ensure tools -> start prometheus(:9090) -> start grafana(:3000) -> verify targets up
# Usage:
#   powershell -ExecutionPolicy Bypass -File deploy/monitoring.ps1                # start
#   powershell -ExecutionPolicy Bypass -File deploy/monitoring.ps1 -Action start  -ScrapeTarget localhost:8100
#   powershell -ExecutionPolicy Bypass -File deploy/monitoring.ps1 -Action stop
#   powershell -ExecutionPolicy Bypass -File deploy/monitoring.ps1 -Action status
param(
  [ValidateSet('start', 'stop', 'status')]
  [string]$Action = 'start',
  [switch]$NoWait,
  [switch]$VerboseLog,
  [string]$PromVersion = '2.54.1',
  [string]$GrafanaVersion = '11.2.0'
)
$ErrorActionPreference = "Stop"

$MonitoringDir = Join-Path $PSScriptRoot 'monitoring'                      # server/deploy/monitoring
$ToolsRoot = Join-Path $env:LOCALAPPDATA 'dlidli-monitoring'                  # 便携版二进制与数据目录（仓库外）
$PromDir = Join-Path $ToolsRoot "prometheus-$PromVersion.windows-amd64"
$GrafanaDir = Join-Path $ToolsRoot "grafana-v$GrafanaVersion"
$PromUrl = "http://localhost:9090"
$GrafanaUrl = "http://localhost:3000"
$PromExe = Join-Path $PromDir 'prometheus.exe'
$GrafanaExe = Join-Path $GrafanaDir 'bin\grafana.exe'

function Get-ProxyArg {
  # 环境变量已有代理则直接走；否则探测系统代理（如 Clash 127.0.0.1:7890）
  if ($env:HTTP_PROXY -or $env:HTTPS_PROXY -or $env:ALL_PROXY) { return @() }
  $reg = Get-ItemProperty -Path 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings' -ErrorAction SilentlyContinue
  if ($reg.ProxyEnable -eq 1 -and $reg.ProxyServer) {
    $proxy = $reg.ProxyServer -split ';' | ForEach-Object { ($_ -split '=')[-1] } | Select-Object -First 1
    if ($proxy) { return @('-x', "http://$proxy") }
  }
  return @()
}

function Download-Zip {
  param([string[]]$Urls, [string]$DestZip)
  $proxy = Get-ProxyArg
  foreach ($u in $Urls) {
    Write-Output "  downloading $u ..."
    try {
      curl.exe -sL -m 600 @proxy -o $DestZip $u
      if ($LASTEXITCODE -eq 0 -and (Test-Path $DestZip) -and (Get-Item $DestZip).Length -gt 10MB) { return $true }
    } catch { }
    Write-Output "  failed, try next mirror ..."
  }
  return $false
}

function Ensure-Tools {
  if ((Test-Path $PromExe) -and (Test-Path $GrafanaExe)) { return }
  New-Item -ItemType Directory -Path $ToolsRoot -Force | Out-Null

  if (-not (Test-Path $PromExe)) {
    Write-Output "[1/5] Downloading Prometheus $PromVersion (portable) ..."
    $zip = Join-Path $ToolsRoot 'prometheus.zip'
    $ok = Download-Zip -Urls @(
      "https://github.com/prometheus/prometheus/releases/download/v$PromVersion/prometheus-$PromVersion.windows-amd64.zip",
      "https://mirrors.huaweicloud.com/prometheus/$PromVersion/prometheus-$PromVersion.windows-amd64.zip"
    ) -DestZip $zip
    if (-not $ok) { throw "prometheus download failed (github & huaweicloud mirror)" }
    Expand-Archive -Path $zip -DestinationPath $ToolsRoot -Force
    Remove-Item $zip -Force
    Write-Output "  prometheus -> $PromDir"
  }

  if (-not (Test-Path $GrafanaExe)) {
    Write-Output "[2/5] Downloading Grafana $GrafanaVersion (portable) ..."
    $zip = Join-Path $ToolsRoot 'grafana.zip'
    $ok = Download-Zip -Urls @("https://dl.grafana.com/oss/release/grafana-$GrafanaVersion.windows-amd64.zip") -DestZip $zip
    if (-not $ok) { throw "grafana download failed" }
    Expand-Archive -Path $zip -DestinationPath $ToolsRoot -Force
    Remove-Item $zip -Force
    Write-Output "  grafana -> $GrafanaDir"
  }
}

function Start-Prometheus {
  Write-Output "[3/5] Starting Prometheus (port 9090) ..."
  $cfg = Join-Path $MonitoringDir 'prometheus.yml'
  $args = @("--config.file=$cfg", "--storage.tsdb.path=$ToolsRoot\prometheus-data", "--web.listen-address=:9090")
  if ($VerboseLog) { $args += "--log.level=debug" }
  $p = Start-Process -FilePath $PromExe -ArgumentList $args -PassThru -WindowStyle Hidden `
    -RedirectStandardOutput (Join-Path $ToolsRoot 'prometheus.out.log') `
    -RedirectStandardError (Join-Path $ToolsRoot 'prometheus.err.log')
  if (-not $p) { throw "prometheus failed to start" }
  Set-Content -Path (Join-Path $ToolsRoot 'prometheus.pid') -Value $p.Id
  Write-Output "  started PID=$($p.Id)"

  for ($i = 0; $i -lt 40; $i++) {
    Start-Sleep -Seconds 1
    try { if ((Invoke-RestMethod -Uri "$PromUrl/-/ready" -TimeoutSec 3).ToString().Trim() -eq 'Prometheus Server is Ready.') { Write-Output "  ready at $PromUrl"; return } } catch { }
  }
  Write-Output "  WARNING: Prometheus 未在预期时间内就绪，请查看 $ToolsRoot\prometheus.err.log"
}

function Start-Grafana {
  Write-Output "[4/5] Starting Grafana (port 3000) ..."
  $env:GF_PATHS_DATA = Join-Path $ToolsRoot 'grafana-data'
  $env:GF_PATHS_LOGS = Join-Path $ToolsRoot 'grafana-logs'
  New-Item -ItemType Directory -Path $env:GF_PATHS_DATA, $env:GF_PATHS_LOGS -Force | Out-Null
  $env:GF_PATHS_PROVISIONING = Join-Path $MonitoringDir 'grafana\provisioning'
  $env:DLIDLI_DASHBOARDS_PATH = Join-Path $MonitoringDir 'grafana\dashboards'
  $env:GRAFANA_PROM_URL = "http://localhost:9090"
  $env:GF_USERS_DEFAULT_LANGUAGE = 'zh-Hans'   # 界面汉化（Grafana 内置 zh-Hans 语言包）
  if ($VerboseLog) { $env:GF_LOG_LEVEL = 'debug' }
  $p = Start-Process -FilePath $GrafanaExe -ArgumentList @('server', "--homepath=$GrafanaDir") -PassThru -WindowStyle Hidden `
    -RedirectStandardOutput (Join-Path $ToolsRoot 'grafana.out.log') `
    -RedirectStandardError (Join-Path $ToolsRoot 'grafana.err.log')
  Remove-Item Env:\GF_PATHS_DATA, Env:\GF_PATHS_LOGS, Env:\GF_PATHS_PROVISIONING, Env:\DLIDLI_DASHBOARDS_PATH, Env:\GRAFANA_PROM_URL, Env:\GF_USERS_DEFAULT_LANGUAGE
  if ($VerboseLog) { Remove-Item Env:\GF_LOG_LEVEL }
  if (-not $p) { throw "grafana failed to start" }
  Set-Content -Path (Join-Path $ToolsRoot 'grafana.pid') -Value $p.Id
  Write-Output "  started PID=$($p.Id)"

  for ($i = 0; $i -lt 30; $i++) {
    Start-Sleep -Seconds 2
    try { $h = Invoke-RestMethod -Uri "$GrafanaUrl/api/health" -TimeoutSec 3; if ($h.database -eq 'ok') { Write-Output "  ready at $GrafanaUrl (admin/admin)"; return } } catch { }
  }
  Write-Output "  WARNING: Grafana 未在预期时间内就绪，请查看 $ToolsRoot\grafana.err.log"
}

function Test-TargetUp {
  # 等 Prometheus 完成首次抓取：dlidli-api target 状态为 up
  for ($i = 0; $i -lt 10; $i++) {
    Start-Sleep -Seconds 2
    try {
      $t = Invoke-RestMethod -Uri "$PromUrl/api/v1/targets?state=active" -TimeoutSec 5
      foreach ($tt in $t.data.activeTargets) {
        if ($tt.labels.job -eq 'dlidli-api') {
          $lastErr = if ($tt.lastError) { " lastError=$($tt.lastError)" } else { "" }
          if ($tt.health -eq 'up') { return "UP  ($($tt.labels.instance))" }
          return "DOWN$lastErr"
        }
      }
    } catch { }
  }
  return 'UNKNOWN (no active target yet)'
}

function Invoke-Start {
  Write-Output "=============================================="
  Write-Output "  DliDli monitoring start (M1-REL-02)"
  Write-Output "=============================================="
  Ensure-Tools
  Start-Prometheus
  Start-Grafana

  if ($NoWait) {
    Write-Output "  started detached (-NoWait): Prometheus $PromUrl | Grafana $GrafanaUrl (admin/admin)"
    return
  }

  Write-Output "[5/5] Verify scrape target ..."
  $targetState = Test-TargetUp
  Write-Output "  dlidli-api target: $targetState"
  Write-Output "  Prometheus $PromUrl  |  Grafana $GrafanaUrl (admin/admin, dashboard: DliDli 基础监控)"
  if ($targetState -notlike 'UP*') {
    Write-Output "  WARNING: backend localhost:8000 不可达或 /metrics 未暴露，请确认后端已启动（抓取目标见 deploy/monitoring/prometheus.yml）"
  }
}

function Invoke-Stop {
  foreach ($name in @('prometheus', 'grafana', 'grafana-server')) {
    $pidFile = Join-Path $ToolsRoot "$name.pid"
    if (Test-Path $pidFile) {
      $procId = [int](Get-Content $pidFile)
      Stop-Process -Id $procId -Force -ErrorAction SilentlyContinue
      Remove-Item $pidFile -Force
      Write-Output "stopped $name (PID=$procId)"
    } else {
      Get-Process -Name $name -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
      Write-Output "stopped $name (no pid file, by name)"
    }
  }
}

function Invoke-Status {
  $promUp = $false; $grafanaUp = $false
  try { if ((Invoke-RestMethod -Uri "$PromUrl/-/ready" -TimeoutSec 3).ToString().Trim() -eq 'Prometheus Server is Ready.') { $promUp = $true } } catch { }
  try { $h = Invoke-RestMethod -Uri "$GrafanaUrl/api/health" -TimeoutSec 3; if ($h.database -eq 'ok') { $grafanaUp = $true } } catch { }
  Write-Output "Prometheus :9090  $($(if ($promUp) { 'UP' } else { 'DOWN' }))"
  Write-Output "Grafana   :3000  $($(if ($grafanaUp) { 'UP' } else { 'DOWN' }))"
  if ($promUp) { Write-Output "target: $(Test-TargetUp)" }
}

switch ($Action) {
  'start'  { Invoke-Start }
  'stop'   { Invoke-Stop }
  'status' { Invoke-Status }
}
