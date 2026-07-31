// SVG → PNG 资产生成脚本（设计源文件用 SVG 维护，运行时用 PNG 保证兼容性）
// 用法：node scripts/svg2png.mjs
import { readFile } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
import path from 'node:path'
import sharp from 'sharp'

const root = path.dirname(path.dirname(fileURLToPath(import.meta.url)))

/** 待转换清单：[svg 源, png 目标, 尺寸] */
const tasks = [
  ['apps/web/src/assets/default-avatar.svg', 'apps/web/src/assets/default-avatar.png', 256],
]

for (const [src, dst, size] of tasks) {
  const svg = await readFile(path.join(root, src))
  await sharp(svg, { density: 300 })
    .resize(size, size)
    .png({ compressionLevel: 9 })
    .toFile(path.join(root, dst))
  console.log(`ok: ${src} -> ${dst} (${size}x${size})`)
}
