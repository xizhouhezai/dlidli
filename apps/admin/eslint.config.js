// 共享配置见 @dlidli/eslint-config（应用特定的规则覆盖在此追加）
import { fileURLToPath, URL } from 'node:url'
import { createVueConfig } from '@dlidli/eslint-config'

export default createVueConfig(fileURLToPath(new URL('.', import.meta.url)))
