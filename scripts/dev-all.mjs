#!/usr/bin/env node
/**
 * DliDli 一键启动所有服务（开发环境）
 *
 * 用法：
 *   node scripts/dev-all.mjs                    # 默认：依赖 + 迁移 + api + web + admin + h5
 *   node scripts/dev-all.mjs --docs             # 额外启动文档站
 *   node scripts/dev-all.mjs --only api,web     # 只启动指定服务（逗号分隔）
 *   node scripts/dev-all.mjs --no-h5            # 不启动 h5
 *   node scripts/dev-all.mjs --no-deps          # 不自动拉起 Docker 依赖
 *   node scripts/dev-all.mjs --skip-migrate     # 跳过数据库迁移
 *   node scripts/dev-all.mjs --skip-build       # 跳过后端构建（复用已有二进制）
 *   node scripts/dev-all.mjs --strict           # 依赖未就绪即退出（默认降级继续）
 *   node scripts/dev-all.mjs --check-only       # 只做环境检查并退出
 *   node scripts/dev-all.mjs --list             # 列出可用服务名
 *
 * 行为：
 *   1. 解析 server/configs/dev.yaml 作为**唯一配置来源**（MySQL/Redis/API 端口都从它读，
 *      不再硬编码端口——历史上曾出现「脚本探 3307、配置却是 3306」的错位）
 *   2. 依赖探测：MySQL/Redis 未就绪时**自动拉起** docker compose 对应服务（--no-deps 关闭）
 *   3. 数据库迁移：显式把配置里的 DSN 以 DLIDLI_MIGRATE_DSN 传给迁移工具
 *      （该工具自身默认 DSN 指向 3307/root:root，不传必失败）
 *   4. 构建后端二进制再启动 API（避免 go run 子进程树难清理）
 *   5. 预检前端端口占用：vite 未设 strictPort，端口被占会静默改用下一个端口，
 *      故先探测并在冲突时报出来，避免「打印 5173 实际跑在 5174」
 *   6. 并发启动各前端，并从输出里解析**真实监听地址**
 *   7. 就绪判定区分语义：/livez 恒 200 表示进程活着，/health 200 才表示依赖就绪
 *   8. 各服务日志同时打印到控制台与 .dev-logs/<name>.log（便于回溯）
 *   9. Ctrl+C（或任一进程异常退出）统一清理全部子进程
 *
 * 注：鸿蒙端（apps/harmony）不纳入一键启动——它需 DevEco/hvigor 工具链，
 *     由 DevEco Studio 或 `hvigorw assembleHap` 单独构建运行。
 */
import { spawn, spawnSync } from 'node:child_process'
import { createConnection } from 'node:net'
import { appendFileSync, existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import path from 'node:path'
import process from 'node:process'

const ROOT = process.cwd()
const SERVER = path.join(ROOT, 'server')
const CONFIG_FILE = path.join(SERVER, 'configs', 'dev.yaml')
const COMPOSE_FILE = path.join('server', 'deploy', 'docker-compose.yaml')
const LOG_DIR = path.join(ROOT, '.dev-logs')
const isWin = process.platform === 'win32'

// —— 参数解析 ——
const argv = process.argv.slice(2)
const args = new Set(argv)
/** 取 --key value 或 --key=value 的值 */
function argValue(key) {
  const withEq = argv.find((a) => a.startsWith(`${key}=`))
  if (withEq) return withEq.slice(key.length + 1)
  const idx = argv.indexOf(key)
  if (idx >= 0 && idx + 1 < argv.length && !argv[idx + 1].startsWith('--')) return argv[idx + 1]
  return undefined
}
const withDocs = args.has('--docs')
const skipMigrate = args.has('--skip-migrate')
const skipBuild = args.has('--skip-build')
const noDeps = args.has('--no-deps')
const strict = args.has('--strict')
const checkOnly = args.has('--check-only')
const listOnly = args.has('--list')
const onlyArg = argValue('--only')

// —— 颜色 ——
const C = {
  reset: '\x1b[0m',
  dim: '\x1b[2m',
  bold: '\x1b[1m',
  red: '\x1b[31m',
  green: '\x1b[32m',
  yellow: '\x1b[33m',
  cyan: '\x1b[36m',
  magenta: '\x1b[35m',
  blue: '\x1b[34m',
}
const PALETTE = [C.cyan, C.magenta, C.blue, C.green, C.yellow]
const tagColor = new Map()
function tag(name) {
  if (!tagColor.has(name)) tagColor.set(name, PALETTE[tagColor.size % PALETTE.length])
  return `${tagColor.get(name)}[${name}]${C.reset}`
}
function log(name, msg) {
  console.log(`${tag(name)} ${msg}`)
}
function note(msg) {
  console.log(`${C.dim}[dev-all]${C.reset} ${msg}`)
}
function ok(msg) {
  console.log(`${C.green}[✓]${C.reset} ${msg}`)
}
function warn(msg) {
  console.log(`${C.yellow}[!]${C.reset} ${msg}`)
}
function fail(msg) {
  console.error(`${C.red}[dev-all] ✗ ${msg}${C.reset}`)
  process.exit(1)
}

// —— 通用工具 ——
function pnpmCmd() {
  return isWin ? 'pnpm.cmd' : 'pnpm'
}
function runSync(cmd, cmdArgs, opts = {}) {
  return spawnSync(cmd, cmdArgs, { stdio: 'inherit', shell: isWin, ...opts }).status
}
function runCapture(cmd, cmdArgs, opts = {}) {
  const res = spawnSync(cmd, cmdArgs, { encoding: 'utf8', shell: isWin, ...opts })
  return { code: res.status, out: `${res.stdout || ''}${res.stderr || ''}`.trim() }
}
function probePort(port, host = '127.0.0.1', timeout = 900) {
  return new Promise((resolve) => {
    const sock = createConnection({ port, host })
    let settled = false
    const done = (val) => {
      if (settled) return
      settled = true
      sock.destroy()
      resolve(val)
    }
    sock.setTimeout(timeout, () => done(false))
    sock.on('connect', () => done(true))
    sock.on('error', () => done(false))
  })
}

/**
 * 探测端口是否被监听（IPv4 与 IPv6 都要试）。
 *
 * 必须双栈：vite 默认只绑 `::1`（纯 IPv6），只探 127.0.0.1 会得到「空闲」的假阴性，
 * 导致「端口冲突预检」与「前端真实端口回填」双双失灵。
 */
async function probePortAny(port) {
  if (await probePort(port, '127.0.0.1')) return true
  return probePort(port, '::1')
}
const sleep = (ms) => new Promise((r) => setTimeout(r, ms))

/**
 * 从 dev.yaml 读取配置。
 * 只做**针对性抽取**（不引第三方 YAML 依赖）：本文件结构简单，
 * 用锚点行 + 缩进匹配即可稳定取到所需字段；取不到时回落默认值并提示。
 */
function readDevConfig() {
  const fallback = { mysqlPort: 3306, redisPort: 6379, apiPort: 8000, mysqlDsn: '', redisAddr: '' }
  if (!existsSync(CONFIG_FILE)) {
    warn(`未找到 ${path.relative(ROOT, CONFIG_FILE)}，使用默认端口`)
    return fallback
  }
  const text = readFileSync(CONFIG_FILE, 'utf8')
  const cfg = { ...fallback, mysqlDsn: '', redisAddr: '' }

  // mysql.dsn: "user:pass@tcp(host:port)/db?..."
  const dsn = text.match(/^\s*dsn:\s*["']?([^"'\r\n]+)["']?/m)
  if (dsn) {
    cfg.mysqlDsn = dsn[1].trim()
    const m = cfg.mysqlDsn.match(/tcp\(([^:)]+):(\d+)\)/)
    if (m) cfg.mysqlPort = Number(m[2])
  }
  // redis.addr: "host:port"
  const addr = text.match(/^\s*addr:\s*["']?([^"'\r\n]+)["']?/m)
  if (addr) {
    cfg.redisAddr = addr[1].trim()
    const m = cfg.redisAddr.match(/:(\d+)\s*$/)
    if (m) cfg.redisPort = Number(m[1])
  }
  // app.port：注意文档站/前端配置里也可能有 port，故限定在 app: 段内查找
  const appSection = text.match(/^app:\s*$([\s\S]*?)^\w/m)
  if (appSection) {
    const p = appSection[1].match(/^\s*port:\s*(\d+)/m)
    if (p) cfg.apiPort = Number(p[1])
  }
  return cfg
}

/**
 * 把 dev.yaml 的 DSN 转成迁移工具认的 DSN。
 * dev.yaml 用的是 go-sql-driver 格式（user:pass@tcp(h:p)/db?x=y），
 * golang-migrate 需要 URL 格式（mysql://user:pass@tcp(h:p)/db?x=y）。
 */
function toMigrateDsn(dsn) {
  if (!dsn) return ''
  return dsn.startsWith('mysql://') ? dsn : `mysql://${dsn}`
}

// —— 服务清单 ——
function buildServices() {
  const list = [
    { name: 'api', kind: 'api' },
    { name: 'web', kind: 'pnpm', script: 'web:dev', port: 5173 },
    { name: 'admin', kind: 'pnpm', script: 'admin:dev', port: 5175 },
  ]
  if (!args.has('--no-h5')) list.push({ name: 'h5', kind: 'pnpm', script: 'h5:dev', port: 5176 })
  if (withDocs) list.push({ name: 'docs', kind: 'pnpm', script: 'docs:dev', port: 5177 })
  return list
}
const ALL_SERVICES = buildServices()
const selected = onlyArg
  ? ALL_SERVICES.filter((s) =>
      onlyArg
        .split(',')
        .map((x) => x.trim())
        .includes(s.name),
    )
  : ALL_SERVICES

if (listOnly) {
  console.log('可用服务：' + ALL_SERVICES.map((s) => s.name).join(', '))
  process.exit(0)
}
if (onlyArg && selected.length === 0) {
  fail(`--only ${onlyArg} 未匹配到任何服务；可用：${ALL_SERVICES.map((s) => s.name).join(', ')}`)
}

// —— 子进程管理 ——
const children = new Map()
let shuttingDown = false

/** 启动一个长期子进程：带标签日志 + 落盘 + 异常退出联动收摊 */
function start(name, cmd, cmdArgs, opts = {}) {
  const child = spawn(cmd, cmdArgs, {
    cwd: opts.cwd || ROOT,
    shell: isWin,
    env: { ...process.env, ...(opts.env || {}) },
    stdio: ['ignore', 'pipe', 'pipe'],
  })
  children.set(name, child)

  // 各服务日志落 .dev-logs/<name>.log，便于回溯被刷走的输出
  const logFile = path.join(LOG_DIR, `${name}.log`)
  try {
    mkdirSync(LOG_DIR, { recursive: true })
    writeFileSync(logFile, `# ${name} @ ${new Date().toISOString()}\n`)
  } catch {
    /* 日志目录不可用时忽略，不影响启动 */
  }

  const pipe = (chunk) => {
    for (const line of chunk.toString().split('\n')) {
      if (!line.trim()) continue
      console.log(`${tag(name)} ${line}`)
      try {
        appendFileSync(logFile, `${line}\n`)
      } catch {
        /* ignore */
      }
    }
  }
  child.stdout.on('data', pipe)
  child.stderr.on('data', pipe)

  child.on('error', (err) => {
    if (!shuttingDown) fail(`${name} 启动失败: ${err.message}`)
  })
  child.on('exit', (code, signal) => {
    children.delete(name)
    if (shuttingDown) return
    if (code !== 0 && code !== null) {
      console.error(`${tag(name)} ${C.red}进程退出 code=${code}${C.reset}`)
      // 前端/API 异常退出通常意味着端口或配置问题，直接收摊更省排查时间
      if (code !== 0) {
        warn(`${name} 异常退出，正在停止其余服务…`)
        stopAll()
        process.exit(1)
      }
    } else if (signal) {
      note(`${name} 被信号 ${signal} 终止`)
    }
  })
  return child
}

let stopping = false
function stopAll() {
  if (stopping) return
  stopping = true
  for (const [name, child] of children) {
    try {
      if (isWin && child.pid) {
        spawnSync('taskkill', ['/pid', String(child.pid), '/T', '/F'], { stdio: 'ignore' })
      } else {
        child.kill('SIGTERM')
      }
      note(`已停止 ${name}`)
    } catch {
      /* ignore */
    }
  }
  children.clear()
}

function shutdown(reason) {
  if (shuttingDown) return
  shuttingDown = true
  console.log(`\n${C.yellow}[dev-all] ${reason}，正在关闭所有服务…${C.reset}`)
  stopAll()
  process.exit(0)
}
process.on('SIGINT', () => shutdown('收到 Ctrl+C'))
process.on('SIGTERM', () => shutdown('收到 SIGTERM'))

// —— 依赖：未就绪时自动拉起 Docker ——
function dockerAvailable() {
  return runCapture('docker', ['--version']).code === 0
}
async function ensureDeps(cfg) {
  const [mysqlOk, redisOk] = await Promise.all([probePort(cfg.mysqlPort), probePort(cfg.redisPort)])
  const status = { mysqlOk, redisOk }

  const report = () => {
    console.log(
      `  MySQL ${cfg.mysqlPort}  ${status.mysqlOk ? `${C.green}就绪${C.reset}` : `${C.yellow}未就绪${C.reset}`}`,
    )
    console.log(
      `  Redis ${cfg.redisPort}  ${status.redisOk ? `${C.green}就绪${C.reset}` : `${C.yellow}未就绪${C.reset}`}`,
    )
  }

  if (status.mysqlOk && status.redisOk) {
    report()
    return status
  }

  // 需要拉起依赖
  const need = []
  if (!status.mysqlOk) need.push('mysql')
  if (!status.redisOk) need.push('redis')

  if (noDeps) {
    report()
    warn(`以下依赖未就绪：${need.join(', ')}（--no-deps 已禁用自动拉起）`)
    console.log(`    手动启动：docker compose -f ${COMPOSE_FILE} up -d ${need.join(' ')}`)
    return status
  }

  if (!dockerAvailable()) {
    report()
    warn(`依赖未就绪且未检测到 docker：${need.join(', ')}`)
    console.log(`    安装 Docker，或手动启动 MySQL/Redis 后重试。`)
    return status
  }

  note(`依赖未就绪（${need.join(', ')}），正在启动容器…`)
  const code = runSync('docker', ['compose', '-f', COMPOSE_FILE, 'up', '-d', ...need])
  if (code !== 0) {
    warn('docker compose 启动失败，继续尝试（后端可能降级）')
    return status
  }

  // 等端口起来（最多 60s，容器初始化需要时间）
  note('等待依赖端口就绪（最多 60s）…')
  for (let i = 0; i < 60; i++) {
    await sleep(1000)
    status.mysqlOk = status.mysqlOk || (await probePort(cfg.mysqlPort))
    status.redisOk = status.redisOk || (await probePort(cfg.redisPort))
    if (status.mysqlOk && status.redisOk) break
  }
  report()
  if (!status.mysqlOk || !status.redisOk) {
    warn('依赖在 60s 内未全部就绪，继续启动（后端可能降级）')
  }
  return status
}

// —— 前端真实端口解析 ——
/** 从 vite/uni 输出里抓 "Local: http://localhost:5173/" 拿到真实监听端口 */
function makePortSniffer(onFound) {
  const re = /https?:\/\/(?:localhost|127\.0\.0\.1):(\d+)/
  return (line) => {
    const m = line.match(re)
    if (m) onFound(Number(m[1]))
  }
}

// —— 主流程 ——
async function main() {
  console.log(`${C.cyan}${C.bold}════════════════════════════════════════════${C.reset}`)
  console.log(`${C.cyan}${C.bold}  DliDli 一键启动 dev-all${C.reset}`)
  console.log(`${C.cyan}${C.bold}════════════════════════════════════════════${C.reset}`)
  console.log(
    `${C.dim}  服务：${selected.map((s) => s.name).join(', ')}${withDocs ? '' : ''}${C.reset}`,
  )
  if (onlyArg) note(`--only ${onlyArg} 生效`)

  if (!existsSync(path.join(SERVER, 'go.mod'))) {
    fail(`未找到 ${path.relative(ROOT, path.join(SERVER, 'go.mod'))}，请在项目根目录运行`)
  }

  // 1. 配置
  console.log(`\n${C.dim}── 配置（来源 ${path.relative(ROOT, CONFIG_FILE)}）──${C.reset}`)
  const cfg = readDevConfig()
  note(`MySQL 端口 ${cfg.mysqlPort} ｜ Redis 端口 ${cfg.redisPort} ｜ API 端口 ${cfg.apiPort}`)

  // 2. 依赖
  const needApi = selected.some((s) => s.kind === 'api')
  console.log(`\n${C.dim}── 依赖检查 ──${C.reset}`)
  const deps = needApi ? await ensureDeps(cfg) : { mysqlOk: true, redisOk: true }
  const depsReady = deps.mysqlOk && deps.redisOk

  if (checkOnly) {
    // 前端端口预检也纳入检查（只列本次会启动的服务）
    console.log(`\n${C.dim}── 端口占用预检 ──${C.reset}`)
    for (const s of selected) {
      if (s.port === undefined) continue
      const busy = await probePortAny(s.port)
      console.log(
        `  ${s.name.padEnd(6)} ${s.port}  ${busy ? `${C.yellow}已被占用${C.reset}` : `${C.green}空闲${C.reset}`}`,
      )
    }
    const healthy = depsReady
    console.log(
      `\n${healthy ? C.green : C.yellow}环境检查完成：依赖${depsReady ? '就绪' : '未完全就绪'}。${C.reset}`,
    )
    process.exit(healthy ? 0 : 1)
  }
  if (!depsReady && strict) fail('依赖未就绪（--strict）')
  if (!depsReady) warn('依赖未就绪，继续启动（后端将降级，业务接口可能不可用）')

  // 3. 迁移（显式传 DSN——迁移工具默认 3307/root:root，不传必失败）
  if (needApi && !skipMigrate) {
    console.log(`\n${C.dim}── 数据库迁移 ──${C.reset}`)
    const migrateDsn = toMigrateDsn(cfg.mysqlDsn)
    const env = migrateDsn ? { ...process.env, DLIDLI_MIGRATE_DSN: migrateDsn } : process.env
    if (!migrateDsn) warn('未解析到 mysql.dsn，迁移将使用工具默认值（很可能失败）')
    const code = runSync('go', ['run', './cmd/migrate'], { cwd: SERVER, env })
    if (code !== 0) {
      warn(`迁移失败（code=${code}），继续启动（后端可能不可用）`)
    } else {
      ok('迁移完成')
    }
  } else if (skipMigrate) {
    note('已跳过数据库迁移（--skip-migrate）')
  }

  // 4. 构建后端
  let binPath = path.join(SERVER, 'bin', isWin ? 'api.exe' : 'api')
  if (needApi) {
    console.log(`\n${C.dim}── 后端构建 ──${C.reset}`)
    if (skipBuild && existsSync(binPath)) {
      note(`已跳过构建（--skip-build），复用 ${path.relative(ROOT, binPath)}`)
    } else {
      const code = runSync('go', ['build', '-o', binPath, './cmd/api'], { cwd: SERVER })
      if (code !== 0) fail('后端构建失败，无法启动 API')
      ok(`后端构建完成 → ${path.relative(ROOT, binPath)}`)
    }
  }

  // 5. 前端端口预检（vite 未设 strictPort，被占会静默换端口）
  console.log(`\n${C.dim}── 启动服务 ──${C.reset}`)
  const realPorts = new Map()
  for (const s of selected) {
    if (s.port === undefined) continue
    if (await probePortAny(s.port)) {
      warn(`${s.name} 端口 ${s.port} 已被占用——vite 未设 strictPort，会自动改用下一个端口`)
      warn(`    如非预期，请先释放 ${s.port}（默认地址印刷在下方便于核对）`)
    }
  }

  // 6. 启动
  if (needApi) {
    start('api', binPath, [], { cwd: SERVER })
  }
  for (const s of selected) {
    if (s.kind !== 'pnpm') continue
    const sniff = makePortSniffer((p) => realPorts.set(s.name, p))
    const child = start(s.name, pnpmCmd(), [s.script])
    // 在既有日志 pipe 之外再挂一个解析器，读取同一份 stdout 拿到真实端口
    child.stdout.on('data', (chunk) => {
      for (const line of chunk.toString().split('\n')) if (line.trim()) sniff(line)
    })
  }

  // 7. 等 API 就绪（/livez=进程活着；/health=依赖就绪）
  if (needApi) {
    note(`等待 API 就绪 http://127.0.0.1:${cfg.apiPort}/health …`)
    let live = false
    let ready = false
    for (let i = 0; i < 60; i++) {
      await sleep(500)
      if (!live) {
        try {
          const r = await fetch(`http://127.0.0.1:${cfg.apiPort}/livez`)
          if (r.ok) live = true
        } catch {
          /* not up yet */
        }
      }
      try {
        const r = await fetch(`http://127.0.0.1:${cfg.apiPort}/health`)
        if (r.ok) {
          ready = true
          break
        }
      } catch {
        /* not up yet */
      }
    }
    if (ready) {
      ok(`API 就绪 → http://127.0.0.1:${cfg.apiPort}  （Swagger: /swagger/index.html）`)
    } else if (live) {
      warn(`API 进程已启动，但依赖未就绪（/health 非 200）——请检查 MySQL/Redis`)
    } else {
      warn('API 未在预期时间内响应，请检查上方日志')
    }
  }

  // 8. 等前端真正开始监听（vite 首次启动含依赖预构建，可能 2~40s；
  //    固定 sleep 会在冷启动时让汇总印出「(预期)」，与实际不符）
  const feSelected = selected.filter((s) => s.kind === 'pnpm')
  if (feSelected.length > 0) {
    note('等待前端 dev server 就绪…')
    const deadline = Date.now() + 60000
    while (Date.now() < deadline) {
      let pending = 0
      for (const s of feSelected) {
        // 已有解析到的端口且确实在监听，才算就绪
        const p = realPorts.get(s.name)
        if (p !== undefined && (await probePortAny(p))) continue
        if (p === undefined && s.port !== undefined && (await probePortAny(s.port))) {
          realPorts.set(s.name, s.port)
          continue
        }
        pending++
      }
      if (pending === 0) break
      await sleep(1000)
    }
  }

  // 9. 汇总
  console.log(`\n${C.green}${C.bold}════════════════════════════════════════════${C.reset}`)
  console.log(`${C.green}${C.bold}  服务地址${C.reset}`)
  if (needApi) console.log(`  API    → http://127.0.0.1:${cfg.apiPort}`)
  for (const s of selected) {
    if (s.kind === 'api') continue
    const p = realPorts.get(s.name) ?? s.port
    const mark = realPorts.has(s.name) ? '' : `${C.dim} (启动中)${C.reset}`
    console.log(`  ${s.name.padEnd(6)} → http://localhost:${p}${mark}`)
  }
  console.log(`\n  ${C.dim}日志：${path.relative(ROOT, LOG_DIR)}/<service>.log${C.reset}`)
  console.log(`  ${C.dim}Ctrl+C 停止全部服务。${C.reset}\n`)
}

main().catch((err) => fail(err.message))
