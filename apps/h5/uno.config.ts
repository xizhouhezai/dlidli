import { defineConfig, presetUno, presetIcons, transformerDirectives } from 'unocss'

// DliDli H5 端 UnoCSS 配置（uni-app / H5 目标）。
// 品牌 token 与 web 端 uno.config.ts、src/styles/variables.scss 同源。
// 图标与 web 统一用 MingCute（i-mingcute-*），extraProperties 防 inline 塔陷。
export default defineConfig({
  presets: [
    presetUno(),
    presetIcons({
      scale: 1.2,
      warn: true,
      extraProperties: {
        display: 'inline-block',
        'vertical-align': 'middle',
      },
    }),
  ],
  transformers: [transformerDirectives()],
  theme: {
    colors: {
      primary: '#fb7299',
      'primary-light': '#ffd9e4',
      'primary-hover': '#fc8bab',
      bg: '#f4f5f7',
      'text-1': '#18191c',
      'text-2': '#61666d',
      'text-3': '#9499a0',
      border: '#e3e5e7',
    },
  },
})
