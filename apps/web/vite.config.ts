import { fileURLToPath, URL } from 'node:url'
import { defineConfig, type PluginOption } from 'vite'
import vue from '@vitejs/plugin-vue'
import UnoCSS from 'unocss/vite'

export default defineConfig({
  // monorepo 中 h5(vite5)/web(vite7) 共存，UnoCSS 插件类型可能解析到旧版 vite，
  // 仅类型不兼容（运行时正常），断言为 PluginOption[] 消解 vue-tsc 报错
  plugins: [vue(), UnoCSS()] as PluginOption[],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 5173,
    proxy: {
      // 本地开发直连 Go 后端（dlidli-api :8000）；ws:true 转发弹幕实时 WS（M2-DM-03）
      '/api': { target: 'http://localhost:8000', ws: true },
      '/health': 'http://localhost:8000',
    },
  },
})
