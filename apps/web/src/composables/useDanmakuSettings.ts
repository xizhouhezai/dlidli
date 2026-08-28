import { ref, watch } from 'vue'

/** 弹幕展示设置（播放页渲染层与设置面板共用） */
export interface DanmakuSettings {
  opacity: number // 0.2~1，默认 0.8
  fontScale: number // 0.8~1.5，默认 1
  area: 'quarter' | 'half' | 'full' // 显示区域，默认 full
  speed: 'slow' | 'normal' | 'fast' // 滚动速度，默认 normal
  density: 'low' | 'normal' | 'high' // 同屏密度，默认 normal
}

export const DEFAULT_DM_SETTINGS: DanmakuSettings = {
  opacity: 0.8,
  fontScale: 1,
  area: 'full',
  speed: 'normal',
  density: 'normal',
}

const STORAGE_KEY = 'dlidli_dm_settings'

function load(): DanmakuSettings {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw) return { ...DEFAULT_DM_SETTINGS, ...JSON.parse(raw) }
  } catch {
    // 解析失败回退默认
  }
  return { ...DEFAULT_DM_SETTINGS }
}

/** 弹幕展示设置：localStorage 持久化，播放页与设置面板共享同一实例 */
export function useDanmakuSettings() {
  const settings = ref<DanmakuSettings>(load())
  watch(settings, (v) => localStorage.setItem(STORAGE_KEY, JSON.stringify(v)), { deep: true })
  return { settings }
}
