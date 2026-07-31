import { defineConfig, presetUno, presetAttributify, presetIcons, transformerDirectives } from 'unocss'

// DliDli web 端 UnoCSS 配置。
// 品牌 token 单一真源：这里的 theme 与 styles/_variables.scss、styles/main.scss 的 CSS 变量保持一致。
export default defineConfig({
  presets: [
    presetUno(), // 原子类核心（等价 Tailwind 常用类）
    presetAttributify(), // 支持属性化写法 <div text="sm gray">
    presetIcons({
      scale: 1.2, // 图标相对字号的倍率
      warn: true,
      // 默认 inline-block + 基线居中，否则 span 图标会因 inline 宽高塔陷不显示
      extraProperties: {
        display: 'inline-block',
        'vertical-align': 'middle',
      },
    }),
  ],
  transformers: [
    transformerDirectives(), // 支持 @apply / --uno 指令
  ],
  theme: {
    colors: {
      primary: '#fb7299', // 品牌粉
      'primary-light': '#ffd9e4',
      'primary-hover': '#fc8bab',
      bg: '#f6f7f8',
      'text-1': '#18191c',
      'text-2': '#61666d',
      'text-3': '#9499a0',
      border: '#e3e5e7',
    },
  },
  shortcuts: {
    // 常用组合：品牌主按钮、卡片、居中
    'btn-primary': 'bg-primary text-white rounded-lg px-4 py-2 cursor-pointer transition-colors hover:bg-primary-hover',
    'card-surface': 'bg-white rounded-xl',
    'flex-center': 'flex items-center justify-center',
  },
})
