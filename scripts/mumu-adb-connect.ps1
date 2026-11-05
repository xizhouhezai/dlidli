# MuMu Player 12 ↔ adb 稳定连接脚本
#
# 用法（仓库根）：
#   powershell -ExecutionPolicy Bypass -File scripts/mumu-adb.ps1              # 连接并等 device ready
#   powershell -ExecutionPolicy Bypass -File scripts/mumu-adb.ps1 -Status      # 只看现状
#   powershell -ExecutionPolicy Bypass -File scripts/mumu-adb.ps1 -Restart     # 卡住时重启 adb server 再连
#   powershell -ExecutionPolicy Bypass -File scripts/mumu-adb.ps1 -Index 2     # 指定多开实例
#
# ── 为什么需要这个脚本（实测得出的三个坑）────────────────────────────────
# 1) **adb 端口是动态的，且不要从配置文件读**。
#    MuMu 的 adb 走 NAT 端口映射（本次是 16416），但 `vm_config.json` 会滞后于实际状态；
#    权威来源是 MuMuManager 的 RPC：
#        MuMuManager.exe info -v all  →  adb_host_ip / adb_port / player_state / is_android_started
#    写死端口或读到过期配置，就是「有时连得上、有时连不上」的头号原因。
#    **实例下标也不是 0**：多开时从 1 开始，`-v 0` 会返回 `player index not found`。
# 2) **两套 adb 会互踢**：本机 SDK 的 adb 是 35.0.2，MuMu 自带的是 36.0.0，两者都抢 5037 端口的
#    adb server，版本不一致时会互相 kill —— 表现是「刚才 devices 有、突然空了」。
#    对策：**全程只用一套 adb**（本脚本默认 SDK 的，可用 -AdbPath 指定），卡住时 -Restart。
# 3) **端口处于 Established 而不是 Listen 是正常现象**：adbd 通道建立后，监听套接字被接受的
#    连接占用，`Get-NetTCPConnection -State Listen` 会查不到。所以**不要用 Listen 判断实例是否存活**，
#    要用 `player_state == start_finished` 或直接 adb connect（早期版本脚本就栽在这一点上）。

param(
  [switch]$Status,
  [switch]$Restart,
  [string]$AdbPath = "",
  [int]$TimeoutSec = 45,
  [string]$Index = ""
)

# 注意：**不能**用 "Stop"。PS 5.1 在 Stop 模式下会把原生命令（adb/MuMuManager）写到 stderr 的
# 任何提示都升级为终止性异常，而 adb 的 "daemon not running" / "cannot connect to daemon"、
# MuMuManager 的 RPC 提示都属于**预期内**输出 —— 会表现为「脚本莫名中断」。
# 故统一用 Continue，靠 $LASTEXITCODE 与显式判断做失败处理。
$ErrorActionPreference = "Continue"

$mumuRoot    = "D:\Program Files\Netease\MuMu Player 12"
$mumuManager = Join-Path $mumuRoot "nx_main\MuMuManager.exe"

if (-not (Test-Path $mumuManager)) { throw "找不到 MuMuManager：$mumuManager" }

# ---- 1. 用官方 RPC 取实例真实状态（权威来源，不猜端口）----
Write-Host "查询 MuMu 实例状态（RPC）..." -ForegroundColor DarkGray
$raw = & $mumuManager info -v all 2>&1 | Out-String
try { $all = $raw | ConvertFrom-Json } catch { throw "MuMuManager 返回无法解析的 JSON：`n$raw" }

$instances = @()
foreach ($prop in $all.PSObject.Properties) {
  $v = $prop.Value
  if (-not ($v.PSObject.Properties.Name -contains "adb_port")) { continue }
  $instances += [pscustomobject]@{
    Index       = [string]$v.index
    Name        = [string]$v.name
    AdbPort     = [int]$v.adb_port
    AdbHost     = if ($v.adb_host_ip) { [string]$v.adb_host_ip } else { "127.0.0.1" }
    PlayerState = [string]$v.player_state
    AndroidUp   = [bool]$v.is_android_started
  }
}

if ($instances.Count -eq 0) { throw "MuMuManager 没有返回任何实例（实例可能从未创建）" }
if (-not $Index) { $Index = $instances[0].Index }
$inst = $instances | Where-Object { $_.Index -eq $Index } | Select-Object -First 1
if (-not $inst) {
  Write-Host "[X] 没有下标为 $Index 的实例。现有：" -ForegroundColor Red
  $instances | ForEach-Object { Write-Host ("    index={0}  name={1}  adb={2}:{3}  state={4}" -f $_.Index, $_.Name, $_.AdbHost, $_.AdbPort, $_.PlayerState) }
  exit 2
}

Write-Host ("实例     : index={0}  {1}" -f $inst.Index, $inst.Name)
Write-Host ("adb 目标 : {0}:{1}" -f $inst.AdbHost, $inst.AdbPort)
Write-Host ("运行状态 : {0}  (android_started={1})" -f $inst.PlayerState, $inst.AndroidUp)

if ($inst.PlayerState -ne "start_finished" -or -not $inst.AndroidUp) {
  Write-Host ""
  Write-Host "[X] 实例未启动完成（state=$($inst.PlayerState)）—— 此时任何 adb connect 都会失败。" -ForegroundColor Red
  Write-Host "    先启动实例并等模拟器桌面出现：" -ForegroundColor Yellow
  Write-Host ("    `"{0}`" launch -v {1}" -f $mumuManager, $inst.Index) -ForegroundColor DarkGray
  exit 2
}

# ---- 2. 定位 adb（只用一套，避免两套 server 互踢）----
if (-not $AdbPath) {
  $sdkAdb = Join-Path $env:LOCALAPPDATA "Android\Sdk\platform-tools\adb.exe"
  if (Test-Path $sdkAdb) { $AdbPath = $sdkAdb }
  else {
    $mumuAdb = Join-Path $mumuRoot "nx_main\adb.exe"
    if (Test-Path $mumuAdb) { $AdbPath = $mumuAdb } else { throw "找不到 adb.exe" }
  }
}
Write-Host "adb      : $AdbPath"

$serial = "$($inst.AdbHost):$($inst.AdbPort)"

if ($Status) {
  Write-Host ""
  & $AdbPath devices -l
  exit 0
}

function Test-AdbServer {
  return [bool](Get-NetTCPConnection -LocalPort 5037 -State Listen -ErrorAction SilentlyContinue)
}

if ($Restart) {
  # ⚠ 已知坑（实测）：`adb kill-server` 会派生/等待 daemon，**在 PowerShell 里前台或 -Wait 调用都可能挂住**
  # （daemon 继承管道句柄，PS 一直等；实测卡死 3 分钟以上）。所以这里**不自动 kill**，
  # 而是先看 server 是否健康，不健康就打印可复制的命令让用户自己执行（用户手动跑不会卡）。
  Write-Host "[i] -Restart：检查 adb server 健康状态（不在脚本内 kill-server，见脚本内注释）" -ForegroundColor DarkGray
  if (Test-AdbServer) {
    & $AdbPath devices 2>&1 | Out-Null
    Write-Host "    server 在跑；如需硬重启，请手动执行（脚本内调用会卡住）："
    Write-Host "      `"$AdbPath`" kill-server" -ForegroundColor DarkGray
    Write-Host "      # 然后重跑本脚本（会自动 start-server）" -ForegroundColor DarkGray
  } else {
    Write-Host "    server 未运行，将由本脚本自动拉起"
  }
}

# ---- 3. 确保 server 在跑（异步 + 轮询 5037，绝不在前台等 daemon）----
if (-not (Test-AdbServer)) {
  # 关键：必须 Start-Process 分离启动。前台 `& adb start-server` 会让 PS 等住 daemon 的句柄。
  Start-Process -FilePath $AdbPath -ArgumentList 'start-server' -NoNewWindow -ErrorAction SilentlyContinue | Out-Null
  $dl = (Get-Date).AddSeconds(25)
  while ((Get-Date) -lt $dl -and -not (Test-AdbServer)) { Start-Sleep -Milliseconds 300 }
}
if (-not (Test-AdbServer)) {
  Write-Host "[X] adb server 未能在 5037 端口就绪" -ForegroundColor Red
  Write-Host "    手动排查（按序）：" -ForegroundColor Yellow
  Write-Host "      1) `"$AdbPath`" kill-server        # 在独立终端里跑，不要放进脚本"
  Write-Host "      2) Get-NetTCPConnection -LocalPort 5037 -State Listen | Select OwningProcess"
  Write-Host "         # 若仍有进程占用，End-Process 掉它（通常是残留的旧 adb server）"
  Write-Host "      3) 重跑本脚本"
  exit 4
}
Write-Host "adb server: 5037 已就绪"

$conn = (& $AdbPath connect $serial 2>&1) -join ' '
Write-Host "connect  : $conn"

$deadline = (Get-Date).AddSeconds($TimeoutSec)
$state = ""
while ((Get-Date) -lt $deadline) {
  $lines = & $AdbPath devices 2>$null
  $hit = $lines | Where-Object { $_ -match [regex]::Escape($serial) }
  if ($hit) {
    $state = ($hit -split "\s+")[1]
    if ($state -eq "device") { break }
  }
  Start-Sleep -Seconds 1
}

if ($state -ne "device") {
  Write-Host ""
  Write-Host "[X] 等待超时（最后状态 '$state'）。按序试：" -ForegroundColor Red
  Write-Host "    1) 本脚本加 -Restart（两套 adb server 互踢的常见解法）"
  Write-Host ("    2) 关掉 MuMu 自带 adb server：`"{0}\nx_main\adb.exe`" kill-server，再用本脚本连" -f $mumuRoot)
  Write-Host "    3) MuMu 设置 -> 其他 -> ADB 调试：本地/远端连接切一次再切回（会重新分配端口），随后重跑"
  Write-Host "    4) MuMuManager info -v all 看 player_state 是否真的 start_finished"
  exit 3
}

Write-Host ""
Write-Host "[OK] 已连接 $serial" -ForegroundColor Green
$model = & $AdbPath -s $serial shell getprop ro.product.model 2>$null
$rel   = & $AdbPath -s $serial shell getprop ro.build.version.release 2>$null
$sdk   = & $AdbPath -s $serial shell getprop ro.build.version.sdk 2>$null
$abi   = & $AdbPath -s $serial shell getprop ro.product.cpu.abi 2>$null
$w     = & $AdbPath -s $serial shell wm size 2>$null
Write-Host ("     机型   : {0}" -f $model)
Write-Host ("     Android: {0} (API {1})" -f $rel, $sdk)
Write-Host ("     ABI    : {0}" -f $abi)
Write-Host ("     分辨率 : {0}" -f $w)
if ($abi -match "x86") {
  Write-Host "     [!] MuMu 是 x86_64：Flutter 3.27 之后官方已宣布移除 x86 支持，升级 Flutter 前先用真机/ARM AVD 兜底" -ForegroundColor Yellow
}
Write-Host ""
Write-Host "Flutter 侧：" -ForegroundColor Cyan
Write-Host "     flutter devices"
Write-Host "     flutter run -d $serial"
