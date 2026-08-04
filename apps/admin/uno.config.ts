import { defineConfig, presetUno, presetAttributify, presetIcons, transformerDirectives } from 'unocss'

// DliDli 管理后台 UnoCSS 配置。
// 品牌 token 与 C 端 web 保持一致（apps/web/uno.config.ts）；后台自身深色科技风色板另加。
export default defineConfig({
  presets: [
    presetUno(),
    presetAttributify(),
    presetIcons({
      scale: 1.2,
      warn: true,
      // 默认 inline-block + 基线居中，防 inline span 图标宽高塔陷
      extraProperties: {
        display: 'inline-block',
        'vertical-align': 'middle',
      },
    }),
  ],
  transformers: [transformerDirectives()],
  // 侧边栏菜单图标由后端 /me/permissions 动态下发（:class="item.icon"），
  // 非源码字面量，UnoCSS 静态扫描不到会漏生成，需 safelist 强制纳入。
  // 新增后台菜单（builtinPermissions 的 menu 图标）时同步补充此处。
  safelist: [
    'i-mingcute-dashboard-3-line',
    'i-mingcute-task-2-line',
    'i-mingcute-shield-line',
    'i-mingcute-user-3-line',
    'i-mingcute-safe-lock-line',
    'i-mingcute-group-line',
    'i-mingcute-classify-2-line',
    'i-mingcute-key-2-line',
    'i-mingcute-flag-3-line',
    'i-mingcute-document-2-line',
    'i-mingcute-settings-4-line',
    'i-mingcute-book-4-line',
  ],
  theme: {
    colors: {
      primary: '#fb7299',
      'primary-light': '#ffd9e4',
      'primary-hover': '#fc8bab',
      bg: '#f1f2f3',
      'text-1': '#18191c',
      'text-2': '#61666d',
      'text-3': '#9499a0',
      border: '#e3e5e7',
      // 后台深色调（登录页/顶栏）
      dark: '#1e2022',
      'dark-deep': '#131419',
    },
  },
  shortcuts: {
    'flex-center': 'flex items-center justify-center',
  },
})
