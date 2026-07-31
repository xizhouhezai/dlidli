import { defineConfig } from 'vite'
import uni from '@dcloudio/vite-plugin-uni'

// UnoCSS 是 ESM-only，而 uni 的配置加载器以 require 方式读取本文件，
// 直接顶层 import 'unocss/vite' 会报 "ESM file cannot be loaded by require"。
// 因此改为异步 defineConfig + 动态 import 惰性加载 UnoCSS 插件。
export default defineConfig(async () => {
  const UnoCSS = (await import('unocss/vite')).default
  return {
    plugins: [uni(), UnoCSS()],
    server: {
      port: 5176,
      proxy: {
        // 本地开发直连 Go 后端（dlidli-api :8000）
        '/api': 'http://localhost:8000',
        '/static': 'http://localhost:8000',
        '/health': 'http://localhost:8000',
      },
    },
  }
})
