// Vue3 + TS 共享 ESLint FlatConfig（web / admin 共用，避免逐字节复制的双份配置漂移）。
// rootDir：调用方应用根目录（用于 tsconfigRootDir 解析）；不传则退化为本包目录。
import { fileURLToPath, URL } from 'node:url'
import pluginVue from 'eslint-plugin-vue'
import tseslint from 'typescript-eslint'

export function createVueConfig(rootDir) {
  return [
    { ignores: ['dist/**', 'node_modules/**'] },
    ...tseslint.configs.recommended,
    ...pluginVue.configs['flat/recommended'],
    {
      languageOptions: {
        parserOptions: {
          tsconfigRootDir: rootDir ?? fileURLToPath(new URL('.', import.meta.url)),
        },
      },
    },
    {
      files: ['**/*.vue'],
      languageOptions: {
        parserOptions: { parser: tseslint.parser },
      },
    },
    {
      rules: {
        'vue/multi-word-component-names': 'off',
      },
    },
  ]
}
