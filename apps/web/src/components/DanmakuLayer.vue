<script setup lang="ts">
// 轻量 DOM 弹幕渲染层（M1 版）：滚动/顶部/底部三种模式、轨道分配、乐观上屏。
// Canvas 高性能引擎与防遮挡将在 packages/player（M1-VID-09）中实现。
import { onBeforeUnmount, ref, watch } from 'vue'
import type { DanmakuItem } from '@dlidli/api-client'
import { api } from '@/api'

const props = defineProps<{
  bvid: string
  video: HTMLVideoElement | null
  enabled: boolean
}>()

const layerEl = ref<HTMLDivElement>()

const SCROLL_DURATION = 9000 // 滚动弹幕横穿时长
const FIXED_DURATION = 4000 // 顶/底弹幕停留时长
const LINE_HEIGHT = 30

// 已加载分段与弹幕池
let segmentMs = 6 * 60 * 1000
const loadedSegs = new Set<number>()
const pool = new Map<number, DanmakuItem[]>() // segment -> 按 time_ms 升序
const shown = new Set<string>()
let lastMs = 0

// 轨道占用（滚动/顶部/底部各自独立）
const trackBusy: Record<string, number[]> = { scroll: [], top: [], bottom: [] }

function trackCount() {
  const h = layerEl.value?.clientHeight ?? 300
  return Math.max(3, Math.floor((h * 0.85) / LINE_HEIGHT))
}

function allocTrack(kind: 'scroll' | 'top' | 'bottom'): number {
  const busy = trackBusy[kind]
  const n = trackCount()
  const now = Date.now()
  for (let i = 0; i < n; i++) {
    if (!busy[i] || busy[i] <= now) {
      busy[i] = now + (kind === 'scroll' ? SCROLL_DURATION / 3 : FIXED_DURATION)
      return i
    }
  }
  return Math.floor(Math.random() * n) // 满轨时随机叠放
}

function colorHex(c: number) {
  return `#${(c || 0xffffff).toString(16).padStart(6, '0')}`
}

/** 立即上屏一条弹幕 */
function spawn(item: DanmakuItem) {
  const layer = layerEl.value
  if (!layer || !props.enabled) return

  const el = document.createElement('div')
  el.textContent = item.content
  el.className = 'dli-dm'
  el.style.color = colorHex(item.color)
  el.style.fontSize = item.font_size <= 18 ? '15px' : '20px'
  if (item.is_self) el.style.border = '1px solid rgba(255,255,255,0.8)'

  const kind = item.mode === 2 ? 'top' : item.mode === 3 ? 'bottom' : 'scroll'
  const track = allocTrack(kind)

  if (kind === 'scroll') {
    el.style.top = `${track * LINE_HEIGHT}px`
    el.style.left = '100%'
    layer.appendChild(el)
    const distance = layer.clientWidth + el.offsetWidth
    el.style.transition = `transform ${SCROLL_DURATION}ms linear`
    // 强制回流后启动动画
    void el.offsetWidth
    el.style.transform = `translateX(${-distance}px)`
    window.setTimeout(() => el.remove(), SCROLL_DURATION + 100)
  } else {
    el.style.left = '50%'
    el.style.transform = 'translateX(-50%)'
    if (kind === 'top') el.style.top = `${track * LINE_HEIGHT}px`
    else el.style.bottom = `${track * LINE_HEIGHT}px`
    layer.appendChild(el)
    window.setTimeout(() => el.remove(), FIXED_DURATION)
  }
}

/** 乐观上屏（发送成功后由父组件调用） */
function inject(item: DanmakuItem) {
  shown.add(item.id)
  const seg = Math.floor(item.time_ms / segmentMs)
  pool.get(seg)?.push(item)
  spawn(item)
}

async function ensureSegment(seg: number) {
  if (seg < 0 || loadedSegs.has(seg)) return
  loadedSegs.add(seg)
  try {
    const res = await api.danmaku.list(props.bvid, seg)
    segmentMs = res.segment_ms
    pool.set(seg, res.list)
  } catch {
    loadedSegs.delete(seg) // 拉取失败允许重试
  }
}

function onTimeUpdate() {
  const video = props.video
  if (!video) return
  const nowMs = video.currentTime * 1000
  const seg = Math.floor(nowMs / segmentMs)
  void ensureSegment(seg)
  void ensureSegment(seg + 1) // 预取下一段

  if (props.enabled && !video.paused) {
    const items = pool.get(seg) ?? []
    for (const item of items) {
      if (item.time_ms > lastMs && item.time_ms <= nowMs && !shown.has(item.id)) {
        shown.add(item.id)
        spawn(item)
      }
    }
  }
  lastMs = nowMs
}

function onSeeked() {
  const video = props.video
  if (!video) return
  lastMs = video.currentTime * 1000
  // 回退时允许重新显示历史弹幕
  shown.clear()
  clearScreen()
}

function clearScreen() {
  layerEl.value?.querySelectorAll('.dli-dm').forEach((n) => n.remove())
}

// 绑定视频事件（video 元素异步就绪）
watch(
  () => props.video,
  (video, old) => {
    old?.removeEventListener('timeupdate', onTimeUpdate)
    old?.removeEventListener('seeked', onSeeked)
    if (video) {
      video.addEventListener('timeupdate', onTimeUpdate)
      video.addEventListener('seeked', onSeeked)
      void ensureSegment(0)
    }
  },
  { immediate: true },
)

watch(
  () => props.enabled,
  (on) => {
    if (!on) clearScreen()
  },
)

onBeforeUnmount(() => {
  props.video?.removeEventListener('timeupdate', onTimeUpdate)
  props.video?.removeEventListener('seeked', onSeeked)
})

defineExpose({ inject })
</script>

<template>
  <div
    v-show="enabled"
    ref="layerEl"
    class="dm-layer"
  />
</template>

<style scoped>
.dm-layer {
  position: absolute;
  inset: 0;
  overflow: hidden;
  pointer-events: none;
  z-index: 5;
}

.dm-layer :deep(.dli-dm) {
  position: absolute;
  white-space: nowrap;
  font-weight: 600;
  text-shadow:
    1px 1px 2px rgba(0, 0, 0, 0.8),
    -1px -1px 2px rgba(0, 0, 0, 0.4);
  will-change: transform;
  padding: 1px 4px;
  border-radius: 4px;
  line-height: 28px;
}
</style>
