# echo-admin 管理端本地一键启动：后端(9322) + 前端(8000)。
# 数据库/缓存连接走 configs/config.toml（当前指向线上实例）。
#
# 用法（在仓库任意目录的 PowerShell 终端执行）：
#   powershell -ExecutionPolicy Bypass -File scripts\dev-up.ps1            # 用已编译二进制启动
#   powershell -ExecutionPolicy Bypass -File scripts\dev-up.ps1 -Build     # 先重新编译再启动（改过 Go 代码时）
#   powershell -ExecutionPolicy Bypass -File scripts\dev-up.ps1 -Stop      # 只停止前后端
param(
  [switch]$Build,
  [switch]$Stop
)

$ErrorActionPreference = 'Continue'
$root   = Split-Path -Parent $PSScriptRoot
$binDir = Join-Path $root '.build'
$logDir = Join-Path $binDir 'logs'
$webDir = Join-Path $root 'web'
New-Item -ItemType Directory -Force -Path $binDir, $logDir | Out-Null

# ── 停止旧进程 ──
Write-Output '== 清理旧进程 =='
$exe = Join-Path $binDir 'admin.exe'
if (Test-Path $exe) { Get-Process | Where-Object { $_.Path -eq $exe } | Stop-Process -Force -ErrorAction SilentlyContinue }
Get-Process -Name admin -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Get-CimInstance Win32_Process | Where-Object {
  $_.Name -eq 'node.exe' -and $_.CommandLine -like '*dev.mjs*'
} | ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
Get-Process -Name esbuild -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Start-Sleep 2

if ($Stop) { Write-Output '已全部停止。'; return }

# ── 编译：-Build 强制编译；二进制缺失时自动编译（首次/清理后）──
if ($Build -or -not (Test-Path $exe)) {
  Write-Output '== 编译 admin =='
  Push-Location $root
  go build -o $exe .
  if ($LASTEXITCODE -ne 0) { Write-Warning 'admin 编译失败'; Pop-Location; return }
  Pop-Location
}

if (-not (Test-Path $exe)) { Write-Warning "缺少 $exe，请加 -Build 先编译"; return }

# ── 启动后端(9322)：连线上库较慢，通常 20~30 秒后才监听 ──
Write-Output '== 启动管理端后端(9322) =='
Start-Process -FilePath $exe -ArgumentList '--config', 'configs/config.toml' -WorkingDirectory $root -WindowStyle Hidden `
  -RedirectStandardOutput (Join-Path $logDir 'admin.log') -RedirectStandardError (Join-Path $logDir 'admin.log.err')

# ── 启动前端(8000) ──
Write-Output '== 启动管理端前端(8000) =='
if (Test-Path (Join-Path $webDir 'scripts\dev.mjs')) {
  Start-Process -FilePath 'cmd.exe' -ArgumentList '/c', 'npm run dev' -WorkingDirectory $webDir -WindowStyle Hidden `
    -RedirectStandardOutput (Join-Path $logDir 'admin-web.log') -RedirectStandardError (Join-Path $logDir 'admin-web.log.err')
} else { Write-Warning "  未找到 $webDir\scripts\dev.mjs" }

# ── 轮询等待后端监听（最长 60 秒）──
Write-Output ''
Write-Output '== 等待服务就绪 =='
$ready = $false
for ($i = 0; $i -lt 20; $i++) {
  Start-Sleep 3
  try {
    $null = Invoke-WebRequest -Uri 'http://127.0.0.1:9322/api/setup/state' -UseBasicParsing -TimeoutSec 3
    $ready = $true
    break
  } catch { Write-Output ("  ... 等待后端 ({0}/60s)" -f (($i + 1) * 3)) }
}
if ($ready) { Write-Output '  后端 9322 就绪（连的是线上库）' } else { Write-Warning '  后端 60 秒内未就绪，看 .build/logs/admin.log.err' }

try {
  $r = Invoke-WebRequest -Uri 'http://localhost:8000/' -UseBasicParsing -TimeoutSec 20
  Write-Output ("  前端 8000 HTTP {0}" -f $r.StatusCode)
} catch { Write-Output '  前端 8000 首次编译中（1~2 分钟），稍后刷新即可' }

Write-Output ''
Write-Output '访问地址：'
Write-Output '  管理端   http://localhost:8000/    (admin / admin12345，ops1 / ops12345)'
Write-Output '日志目录：.build/logs/'
