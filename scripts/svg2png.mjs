// DliDli 品牌资产生成流水线
//
// 单一来源：assets/brand/{logo-bg,logo-rim,cover-default}.svg + {logo-mark,logo-mark-small}.png
// 其它所有产物（favicon.svg / .ico / apple-touch-icon / 鸿蒙分层图标与启动图标 /
// 三端默认封面 / h5·admin 成品图标）均由本脚本派生，改设计只改 assets/brand/
// 下的源文件，然后重跑：
//
//   node scripts/svg2png.mjs
//
// 说明：
// - 标记层是位图：由生图模型产出的鲸（`logo-mark.png` 1024×1024，纯白 + 透明底）
//   取代了此前手写的矢量标记。清理与归一化（阈值去水印/灰阶、较长边 67% 居中、
//   小尺寸变体填小洞）在临时脚本里做完后落盘，本脚本只消费成品。
// - 除 favicon.svg 外一律出位图：鸿蒙、小程序、iOS 图标都不接受 SVG，统一格式避免
//   同一张图在不同端两种形态、各自漂移。
// - favicon.svg 是唯一保留的 SVG 产物，内容为「玻璃底板 + 圆角裁切 + 内嵌标记位图 +
//   玻璃倒角」合成的自包含文件，由本脚本生成，不要手改（改了会被下次运行覆盖）。
// - 圆角与玻璃倒角不是画在源文件里的：logo-bg / 标记都不带到边元素，
//   到边的圆角与倒角只在「一块成品图标」（favicon / apple-touch-icon / startIcon / logo.png）
//   的合成阶段叠加——鸿蒙分层图标的遮罩形状由系统决定，硬边画进底板会与遮罩错位。
import { copyFile, mkdir, readFile, writeFile } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
import path from 'node:path'
import sharp from 'sharp'

const root = path.dirname(path.dirname(fileURLToPath(import.meta.url)))
const abs = (p) => path.join(root, p)

/** 品牌源文件目录 */
const SRC = 'assets/brand'
const read = (name) => readFile(abs(`${SRC}/${name}`), 'utf8')
const readBin = (name) => readFile(abs(`${SRC}/${name}`))

/** 圆角半径：1024 画布下 240，约等于 iOS squircle 观感 */
const SQUIRCLE_RX = 240

/** 取出 SVG 外层 <svg> 标签以内的内容，便于把多层拼成一张图 */
function inner(svg) {
  const start = svg.indexOf('>', svg.indexOf('<svg')) + 1
  return svg.slice(start, svg.lastIndexOf('</svg>')).trim()
}

/** 位图标记 → 可嵌进 SVG 的 <image>（data URI，自包含，librsvg 能解析） */
function imageTag(png) {
  return `<image x="0" y="0" width="1024" height="1024" href="data:image/png;base64,${png.toString('base64')}"/>`
}

/** 位图标记 → 独立 SVG 文档（只出标记本身时用，如鸿蒙 foreground） */
const markSvg = (png) =>
  `<svg xmlns="http://www.w3.org/2000/svg" width="1024" height="1024" viewBox="0 0 1024 1024">${imageTag(png)}</svg>`

/** 合成「玻璃底板 + 标记 + 玻璃倒角」的自包含 SVG（成品图标用，自带圆角） */
function composeSquircle(markTag, { header } = {}) {
  const layers = [inner(bg), markTag, inner(rim)]
  return `<svg xmlns="http://www.w3.org/2000/svg" width="1024" height="1024" viewBox="0 0 1024 1024">
${header ? `  <!-- ${header} -->\n` : ''}  <defs>
    <clipPath id="dliSquircle">
      <rect width="1024" height="1024" rx="${SQUIRCLE_RX}" ry="${SQUIRCLE_RX}"/>
    </clipPath>
  </defs>
  <g clip-path="url(#dliSquircle)">
${layers.map((l) => indent(l, 4)).join('\n')}
  </g>
</svg>
`
}

function indent(text, n) {
  const pad = ' '.repeat(n)
  const lines = text.split('\n')
  const widths = lines.filter((l) => l.trim()).map((l) => l.match(/^ */)[0].length)
  const min = widths.length ? Math.min(...widths) : 0
  return lines.map((l) => (l.trim() ? pad + l.slice(min) : '')).join('\n')
}

/** SVG 字符串 → 指定边长的 PNG buffer。density 拉高后降采样，保证小尺寸下的抗锯齿质量 */
async function raster(svg, size, density = 288) {
  return sharp(Buffer.from(svg), { density })
    .resize(size, size, { fit: 'contain', background: { r: 0, g: 0, b: 0, alpha: 0 } })
    .png({ compressionLevel: 9 })
    .toBuffer()
}

/** SVG 字符串 → PNG 文件。effort 只在出大图时给（zlib 压缩更狠，封面 331KB → 162KB） */
async function writePng(svg, outPath, width, height = width, { density = 288, effort } = {}) {
  await mkdir(path.dirname(abs(outPath)), { recursive: true })
  await sharp(Buffer.from(svg), { density })
    .resize(width, height, { fit: 'contain', background: { r: 0, g: 0, b: 0, alpha: 0 } })
    .png({ compressionLevel: 9, ...(effort ? { effort } : {}) })
    .toFile(abs(outPath))
  console.log(`ok: ${outPath} (${width}x${height})`)
}

/**
 * 手写 ICO 容器：ICONDIR(6B) + ICONDIRENTRY(16B × n) + 各帧 PNG payload。
 * 现代 Windows / 浏览器均支持 PNG 压缩的 ICO 帧，无需引入额外依赖。
 */
async function writeIco(frames, outPath) {
  const pngs = await Promise.all(frames.map(({ svg, size }) => raster(svg, size)))
  const count = frames.length
  const header = Buffer.alloc(6)
  header.writeUInt16LE(0, 0) // reserved
  header.writeUInt16LE(1, 2) // type = icon
  header.writeUInt16LE(count, 4)

  let offset = 6 + 16 * count
  const entries = frames.map(({ size }, i) => {
    const png = pngs[i]
    const e = Buffer.alloc(16)
    e.writeUInt8(size >= 256 ? 0 : size, 0) // width（0 表示 256）
    e.writeUInt8(size >= 256 ? 0 : size, 1) // height
    e.writeUInt8(0, 2) // 调色板数
    e.writeUInt8(0, 3) // reserved
    e.writeUInt16LE(1, 4) // color planes
    e.writeUInt16LE(32, 6) // bits per pixel
    e.writeUInt32LE(png.length, 8)
    e.writeUInt32LE(offset, 12)
    offset += png.length
    return e
  })

  await mkdir(path.dirname(abs(outPath)), { recursive: true })
  await writeFile(abs(outPath), Buffer.concat([header, ...entries, ...pngs]))
  console.log(`ok: ${outPath} (${frames.map((f) => f.size).join(',')})`)
  return frames.map((f, i) => ({ size: f.size, png: pngs[i] }))
}

const bg = await read('logo-bg.svg')
const rim = await read('logo-rim.svg')
const cover = await read('cover-default.svg')
/** 位图标记源（纯白 + 透明底，较长边已归一化到画布 67%） */
const mark = await readBin('logo-mark.png')
const markSmall = await readBin('logo-mark-small.png')

const squircle = composeSquircle(imageTag(mark), {
  header: 'generated by scripts/svg2png.mjs — do not edit; 源文件见 assets/brand/',
})

// ── Web：favicon 三件套 ──────────────────────────────────────────────────────
// favicon.svg 是唯一保留的 SVG 产物（自包含，内嵌标记位图），其余产物一律位图
await writeFile(abs('apps/web/public/favicon.svg'), squircle)
console.log('ok: apps/web/public/favicon.svg')
const icoFrames = await writeIco(
  [
    { svg: composeSquircle(imageTag(markSmall)), size: 16 },
    { svg: composeSquircle(imageTag(markSmall)), size: 32 },
    { svg: squircle, size: 48 },
  ],
  'apps/web/public/favicon.ico',
)
await writePng(squircle, 'apps/web/public/apple-touch-icon.png', 180)

// ── 鸿蒙：分层图标（background 满幅 / foreground 透明标记）＋ 启动图标 ─────────
// AppScope 与 entry 两份是镜像关系，必须同时更新。
// foreground 必须是「单色图形 + 透明底」（系统可能重着色），所以玻璃效果全在底板与成品图标上，
// 底板用满幅无倒角的 logo-bg，启动图标才是带玻璃倒角的成品。
// 标记源本身就是 1024×1024 的白形 + 透明底，直接用 density 72 过一遍统一输出格式。
for (const base of ['apps/harmony/AppScope', 'apps/harmony/entry/src/main']) {
  await writePng(bg, `${base}/resources/base/media/background.png`, 1024)
  await writePng(markSvg(mark), `${base}/resources/base/media/foreground.png`, 1024, 1024, {
    density: 72,
  })
}
await writePng(squircle, 'apps/harmony/entry/src/main/resources/base/media/startIcon.png', 152)

// ── 默认封面：底板 1600x900 = 16:9，与 VideoCard 的 aspect-ratio 一致，不被裁切 ──
// 鲸标记由本脚本在合成阶段贴上：渲染宽度 620px（与旧封面等宽）、中心落在 (800,450)。
// 只出 PNG（H5 小程序与鸿蒙端本就只吃位图，统一格式避免同一张图两种形态）。
// 1280x720 覆盖 640px 宽卡片在 2x 屏下的清晰度。
const COVER_W = 1280
const COVER_H = 720
const MARK_W = Math.round((620 * COVER_W) / 1600)
const markOnCover = await sharp(mark)
  .resize(MARK_W, MARK_W, { fit: 'contain', background: { r: 0, g: 0, b: 0, alpha: 0 } })
  .png()
  .toBuffer({ resolveWithObject: true })
for (const t of [
  'apps/web/src/assets/default-cover.png',
  'apps/h5/src/static/default-cover.png',
  'apps/admin/src/assets/default-cover.png',
  // 鸿蒙端走资源目录（`$r('app.media.default_cover')`）；文件名只能是小写字母/数字/下划线
  'apps/harmony/entry/src/main/resources/base/media/default_cover.png',
]) {
  await mkdir(path.dirname(abs(t)), { recursive: true })
  const plate = await sharp(Buffer.from(cover), { density: 288 })
    .resize(COVER_W, COVER_H, { fit: 'contain' })
    .png()
    .toBuffer()
  const m = markOnCover.info
  await sharp(plate)
    .composite([
      {
        input: markOnCover.data,
        left: Math.round((800 * COVER_W) / 1600 - m.width / 2),
        top: Math.round((450 * COVER_H) / 900 - m.height / 2),
      },
    ])
    .png({ compressionLevel: 9, effort: 10 })
    .toFile(abs(t))
  console.log(`ok: ${t} (${COVER_W}x${COVER_H})`)
}

// ── 各端顶栏/登录页用的成品图标 ─────────────────────────────────────────────
// 顶栏是 28px 左右的小尺寸，必须是位图（矢量在这个尺寸下会被反走样糊掉笔画）。
await writePng(squircle, 'apps/web/public/logo.png', 176)
await writePng(squircle, 'apps/h5/src/static/logo.png', 144)
await writePng(squircle, 'apps/admin/public/logo.png', 176)

// ── 默认头像（沿用原有规则，仅 web 端；density 300 且不加 effort，与历史产物逐字节一致）──
await writePng(
  await readFile(abs('apps/web/src/assets/default-avatar.svg'), 'utf8'),
  'apps/web/src/assets/default-avatar.png',
  256,
  256,
  { density: 300 },
)

// ── 产物归档：按平台命名镜像一份到 assets/brand/dist/，供审阅与交付 ───────────
// 该目录不参与各端引用（各端吃的仍是上面写在工程里的那一份），也被 .gitignore 里
// 已有的 `dist/` 规则忽略、不进仓库，随时可重跑重建。同目录的 index.html 是看图页，
// 本脚本只覆盖镜像文件，不动它。
const DIST = 'assets/brand/dist'
const archive = [
  ['assets/brand/logo-mark.png', 'mark-1024.png'],
  ['assets/brand/logo-mark-small.png', 'mark-small-1024.png'],
  ['apps/web/public/favicon.svg', 'favicon.svg'],
  ['apps/web/public/favicon.ico', 'favicon.ico'],
  ['apps/web/public/apple-touch-icon.png', 'apple-touch-icon-180.png'],
  ['apps/web/public/logo.png', 'logo-web-176.png'],
  ['apps/admin/public/logo.png', 'logo-admin-176.png'],
  ['apps/h5/src/static/logo.png', 'logo-h5-144.png'],
  ['apps/harmony/AppScope/resources/base/media/background.png', 'harmony-background-1024.png'],
  ['apps/harmony/AppScope/resources/base/media/foreground.png', 'harmony-foreground-1024.png'],
  ['apps/harmony/entry/src/main/resources/base/media/startIcon.png', 'harmony-startIcon-152.png'],
  ['apps/web/src/assets/default-cover.png', 'cover-1280x720.png'],
  [
    'apps/harmony/entry/src/main/resources/base/media/default_cover.png',
    'harmony-default_cover-1280x720.png',
  ],
]
await mkdir(abs(DIST), { recursive: true })
for (const [from, to] of archive) await copyFile(abs(from), abs(`${DIST}/${to}`))
// ico 三帧单独落盘，便于逐帧看小尺寸实际成像
for (const { size, png } of icoFrames) await writeFile(abs(`${DIST}/ico-${size}.png`), png)
console.log(`ok: ${DIST}/ (${archive.length + icoFrames.length} 个归档产物)`)
