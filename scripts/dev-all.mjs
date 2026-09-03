#!/usr/bin/env node
/**
 * DliDli 一键启动所有服务（开发环境）
 *
 * 用法：
 *   node scripts/dev-all.mjs                # 默认：迁移 + API + web + admin + h5
 *   node scripts/dev-all.mjs --docs         # 额外启动文档站（vitepress）
 *   node scripts/dev-all.mjs --skip-migrate # 跳过数据库迁移
 *   node scripts/dev-all.mjs --no-h5        # 不启动 h5（节省资源）
 *   node scripts/dev-all.mjs --check-only   # 只做依赖检查并退出
 *
 * 行为：
 *   - 检查 MySQL(3307)/Redis(6379) 是否就绪（可用 DLIDLI_* 覆盖关键端口/地址）
 *   - 先跑数据库迁移（go run ./cmd/migrate）
 *   - 构建后端二进制并启动 API（避免 go run 子进程树难清理）
 *   - 并发生起 web / admin / h5（可选 docs）
 *   - 轮询 /health 确认 API 就绪
 *   - Ctrl+C 统一终止所有子进程
 */
import { spawn, spawnSync } from 'node:child_process'
import { createConnection } from 'node:net'
import { existsSync } from 'node:fs'
import path from 'node:path'
import process from 'node:process'

const ROOT = process.cwd()
const SERVER = path.join(ROOT, 'server')
const isWin = process.platform === 'win32'

const args = new Set(process.argv.slice(2))
const withDocs = args.has('--docs')
const skipMigrate = args.has('--skip-migrate')
const noH5 = args.has('--no-h5')
const checkOnly = args.has('--check-only')

// —— 颜色标签 ——
const C = {
  reset: '\x1b[0m',
  dim: '\x1b[2m',
  red: '\x1b[31m',
  green: '\x1b[32m',
  yellow: '\x1b[33m',
  cyan: '\x1b[36m',
  magenta: '\x1b[35m',
  blue: '\x1b[34m',
}
const COLORS = [C.cyan, C.magenta, C.blue, C.green, C.yellow]
let colorIdx = 0
function tag(name) {
  const c = COLORS[colorIdx++ % COLORS.length]
  return `${c}[${name}]${C.reset}`
}
function log(name, msg) {
  console.log(`${tag(name)} ${msg}`)
}
function fail(msg) {
  console.error(`${C.red}[dev-all]${C.reset} ${msg}`)
  process.exit(1)
}

// —— 工具 ——
function pnpmCmd() {
  return isWin ? 'pnpm.cmd' : 'pnpm'
}
function runSync(cmd, args, opts = {}) {
  const res = spawnSync(cmd, args, { stdio: 'inherit', ...opts })
  return res.status
}
function probePort(port, host = '127.0.0.1', timeout = 800) {
  return new Promise((resolve) => {
    const sock = createConnection({ port, host })
    const done = (ok) => {
      sock.destroy()
      resolve(ok)
    }
    sock.setTimeout(timeout, () => done(false))
    sock.on('connect', () => done(true))
    sock.on('error', () => done(false))
    sock.on('timeout', () => done(false))
  })
}
async function sleep(ms) {
  return new Promise((r) => setTimeout(r, ms))
}

// —— 配置 ——
const MYSQL_PORT = Number(process.env.DLIDLI_MYSQL_PORT || 3307)
const REDIS_PORT = Number(process.env.DLIDLI_REDIS_PORT || 6379)
const API_PORT = Number(process.env.DLIDLI_APP_PORT || 8000)

const children = new Set()

/** 启动一个长期子进程，带标签日志 */
function start(name, cmd, argsList, opts = {}) {
  const child = spawn(cmd, argsList, {
    cwd: opts.cwd || ROOT,
    shell: isWin,
    env: { ...process.env, ...(opts.env || {}) },
    stdio: ['ignore', 'pipe', 'pipe'],
  })
  children.add(child)
  child.on('error', (err) => fail(`${name} 启动失败: ${err.message}`))
  child.on('exit', (code) => {
    children.delete(child)
    if (code !== null && code !== 0) {
      console.error(`${tag(name)} ${C.red}进程退出 code=${code}${C.reset}`)
    }
  })
  const pipe = (chunk) => {
    const text = chunk.toString()
    for (const line of text.split('\n')) {
      if (line.trim()) console.log(`${tag(name)} ${line}`)
    }
  }
  child.stdout.on('data', pipe)
  child.stderr.on('data', pipe)
  return child
}

/** 终止所有子进程（Windows 用 taskkill 杀整棵进程树） */
function stopAll() {
  for (const child of children) {
    try {
      if (isWin && child.pid) {
        spawnSync('taskkill', ['/pid', String(child.pid), '/T', '/F'], { stdio: 'ignore' })
      } else {
        child.kill('SIGTERM')
      }
    } catch {
      /* ignore */
    }
  }
  children.clear()
}

process.on('SIGINT', () => {
  console.log('\n\x1b[33m[dev-all] 收到 Ctrl+C，正在关闭所有服务…\x1b[0m')
  stopAll()
  process.exit(0)
})
process.on('SIGTERM', () => stopAll())

// —— 主流程 ——
async function main() {
  console.log(`${C.cyan}════════════════════════════════════════════${C.reset}`)
  console.log(`${C.cyan}  DliDli 一键启动 dev-all${C.reset}`)
  console.log(`${C.cyan}════════════════════════════════════════════${C.reset}`)

  // 1. 依赖检查
  console.log(`\n${C.dim}── 依赖检查 ──${C.reset}`)
  const [mysqlOk, redisOk] = await Promise.all([probePort(MYSQL_PORT), probePort(REDIS_PORT)])
  if (!mysqlOk) {
    console.log(`${C.yellow}[!] MySQL 端口 ${MYSQL_PORT} 未就绪${C.reset}`)
    console.log(
      `    请先启动 MySQL（本机服务，或: docker compose -f server/deploy/docker-compose.yaml up -d mysql）`,
    )
  } else {
    console.log(`${C.green}[✓]${C.reset} MySQL  ${MYSQL_PORT} 就绪`)
  }
  if (!redisOk) {
    console.log(`${C.yellow}[!] Redis 端口 ${REDIS_PORT} 未就绪${C.reset}`)
    console.log(
      `    请先启动 Redis（本机服务，或: docker compose -f server/deploy/docker-compose.yaml up -d redis）`,
    )
  } else {
    console.log(`${C.green}[✓]${C.reset} Redis  ${REDIS_PORT} 就绪`)
  }

  if (!existsSync(path.join(SERVER, 'go.mod'))) {
    fail(`未找到 ${SERVER}/go.mod，请确认在项目根目录运行`)
  }
  if (checkOnly) {
    console.log(
      `\n${C.green}依赖检查完成，环境${mysqlOk && redisOk ? '就绪' : '未完全就绪'}。${C.reset}`,
    )
    process.exit(mysqlOk && redisOk ? 0 : 1)
  }
  if (!mysqlOk || !redisOk) {
    console.log(`\n${C.yellow}[!] 依赖未就绪，仍继续启动应用（后端可能降级/不可用）。${C.reset}`)
    if (args.has('--strict')) fail('依赖未就绪（--strict）')
  }

  // 2. 数据库迁移
  if (!skipMigrate) {
    console.log(`\n${C.dim}── 数据库迁移 ──${C.reset}`)
    const code = runSync('go', ['run', './cmd/migrate'], { cwd: SERVER })
    if (code !== 0) {
      console.log(
        `${C.yellow}[!] 迁移失败（code=${code}），将尝试继续启动（后端可能不可用）。${C.reset}`,
      )
    } else {
      console.log(`${C.green}[✓]${C.reset} 迁移完成`)
    }
  }

  // 3. 构建后端二进制
  console.log(`\n${C.dim}── 构建后端 ──${C.reset}`)
  const binPath = path.join(SERVER, 'bin', isWin ? 'api.exe' : 'api')
  const bcode = runSync('go', ['build', '-o', binPath, './cmd/api'], { cwd: SERVER })
  if (bcode !== 0) fail('后端构建失败，无法启动 API')
  console.log(`${C.green}[✓]${C.reset} 后端构建完成 → ${path.relative(ROOT, binPath)}`)

  // 4. 启动 API
  console.log(`\n${C.dim}── 启动服务 ──${C.reset}`)
  start('api', binPath, [], { cwd: SERVER })

  // 5. 等待 API 就绪
  console.log(`${C.dim}  等待 API 就绪 http://127.0.0.1:${API_PORT}/health …${C.reset}`)
  let apiReady = false
  for (let i = 0; i < 60; i++) {
    await sleep(500)
    try {
      const res = await fetch(`http://127.0.0.1:${API_PORT}/health`)
      if (res.ok) {
        apiReady = true
        break
      }
    } catch {
      /* not up yet */
    }
  }
  if (apiReady) {
    console.log(
      `${C.green}[✓]${C.reset} API 就绪 → http://127.0.0.1:${API_PORT}  （Swagger: /swagger/index.html）`,
    )
  } else {
    console.log(`${C.yellow}[!] API 未在预期时间内就绪，请检查日志。${C.reset}`)
  }

  // 6. 并发生起前端
  const fe = [
    ['web', pnpmCmd(), ['web:dev']],
    ['admin', pnpmCmd(), ['admin:dev']],
  ]
  if (!noH5) fe.push(['h5', pnpmCmd(), ['h5:dev']])
  if (withDocs) fe.push(['docs', pnpmCmd(), ['docs:dev']])
  for (const [name, cmd, argsList] of fe) start(name, cmd, argsList)

  // 7. 汇总
  console.log(`\n${C.green}════════════════════════════════════════════${C.reset}`)
  console.log(`${C.green}  全部服务启动中：${C.reset}`)
  console.log(`  API   → http://127.0.0.1:${API_PORT}`)
  console.log(`  Web   → http://localhost:5173`)
  console.log(`  Admin → http://localhost:5175`)
  if (!noH5) console.log(`  H5    → http://localhost:5176`)
  if (withDocs) console.log(`  Docs  → http://localhost:5177`)
  console.log(`\n  ${C.dim}Ctrl+C 停止全部服务。${C.reset}\n`)
}

main().catch((err) => fail(err.message))
