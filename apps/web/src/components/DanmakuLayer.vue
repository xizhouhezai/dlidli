<script setup lang="ts">
// 弹幕渲染层（M2-DM 进阶版）：DOM 轨道渲染 + 展示设置 + 本地屏蔽过滤 + WS 实时 + 悬停操作。
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { ApiError, type DanmakuItem } from '@dlidli/api-client'
import { api } from '@/api'
import type { DanmakuSettings } from '@/composables/useDanmakuSettings'

const props = defineProps<{
  bvid: string
  video: HTMLVideoElement | null
  enabled: boolean
  settings: DanmakuSettings
}>()

const emit = defineEmits<{ report: [item: DanmakuItem] }>()

const layerEl = ref<HTMLDivElement>()

const LINE_HEIGHT = 30
const FIXED_DURATION = 4000

const SPEED_MS: Record<DanmakuSettings['speed'], number> = {
  slow: 12000,
  normal: 9000,
  fast: 6000,
}

// 已加载分段与弹幕池
let segmentMs = 6 * 60 * 1000
const loadedSegs = new Set<number>()
const pool = new Map<number, DanmakuItem[]>()
const shown = new Set<string>()
let lastMs = 0

// 轨道占用（滚动/顶部/底部各自独立）
const trackBusy: Record<string, number[]> = { scroll: [], top: [], bottom: [] }

// 本地屏蔽（关键词 + 用户哈希；WS 实时消息走本地过滤，HTTP 拉取服务端已过滤）
const blockWords = ref<string[]>([])
const blockHashes = ref(new Set<string>())

function trackCount() {
  const h = layerEl.value?.clientHeight ?? 300
  const areaRatio = props.settings.area === 'quarter' ? 0.25 : props.settings.area === 'half' ? 0.5 : 0.85
  const densityRatio = props.settings.density === 'low' ? 0.7 : props.settings.density === 'high' ? 1.3 : 1
  return Math.max(3, Math.floor((h * areaRatio * densityRatio) / LINE_HEIGHT))
}

function allocTrack(kind: 'scroll' | 'top' | 'bottom'): number {
  const busy = trackBusy[kind]
  const n = trackCount()
  const now = Date.now()
  const dur = kind === 'scroll' ? SPEED_MS[props.settings.speed] / 3 : FIXED_DURATION
  for (let i = 0; i < n; i++) {
    if (!busy[i] || busy[i] <= now) {
      busy[i] = now + dur
      return i
    }
  }
  return Math.floor(Math.random() * n) // 满轨时随机叠放（密度已控）
}

function colorHex(c: number) {
  return `#${(c || 0xffffff).toString(16).padStart(6, '0')}`
}

// ---- 屏蔽 ----

async function loadBlocks() {
  try {
    const res = await api.danmaku.blocks()
    blockWords.value = res.list.filter((b) => b.block_type === 1).map((b) => b.keyword ?? '')
    blockHashes.value = new Set(res.list.filter((b) => b.block_type === 2).map((b) => b.block_hash ?? ''))
  } catch {
    // 游客/失败：保持空列表，不阻塞渲染
  }
}

function isBlocked(item: DanmakuItem): boolean {
  if (item.is_self) return false // 自己的弹幕始终显示
  if (item.sender_hash && blockHashes.value.has(item.sender_hash)) return true
  return blockWords.value.some((w) => w && item.content.includes(w))
}

async function blockSender(item: DanmakuItem) {
  if (!item.sender_hash) return
  try {
    await api.danmaku.addBlock({ block_type: 2, block_hash: item.sender_hash })
    blockHashes.value.add(item.sender_hash)
    removeById(item.id)
    ElMessage.success('已屏蔽该用户的弹幕')
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : '操作失败')
  }
}

// ---- 渲染 ----

/** 立即上屏一条弹幕（已过滤屏蔽项） */
function spawn(item: DanmakuItem) {
  const layer = layerEl.value
  if (!layer || !props.enabled || isBlocked(item)) return

  const el = document.createElement('div')
  el.textContent = item.content
  el.className = 'dli-dm'
  el.dataset.dmId = item.id
  el.style.color = colorHex(item.color)
  el.style.fontSize = `${item.font_size <= 18 ? 15 : 20}px`
  if (item.is_self) el.classList.add('is-self')

  const kind = item.mode === 2 ? 'top' : item.mode === 3 ? 'bottom' : 'scroll'
  const track = allocTrack(kind)

  if (kind === 'scroll') {
    el.style.top = `${track * LINE_HEIGHT}px`
    el.style.left = '0'
    layer.appendChild(el)
    // 挂载后按实际宽度计算滚动轨迹：右边缘外 → 左边缘外（--dm-from/--dm-to）
    void el.offsetWidth
    el.style.setProperty('--dm-from', `${layer.clientWidth}px`)
    el.style.setProperty('--dm-to', `${-el.offsetWidth}px`)
    el.style.animation = `dli-dm-scroll ${SPEED_MS[props.settings.speed]}ms linear forwards`
    window.setTimeout(() => el.remove(), SPEED_MS[props.settings.speed] + 200)
  } else {
    el.style.left = '50%'
    el.style.transform = 'translateX(-50%)'
    if (kind === 'top') el.style.top = `${track * LINE_HEIGHT}px`
    else el.style.bottom = `${track * LINE_HEIGHT}px`
    layer.appendChild(el)
    el.style.animation = `dli-dm-fade ${FIXED_DURATION}ms linear forwards`
    window.setTimeout(() => el.remove(), FIXED_DURATION + 100)
  }
}

function removeById(id: string) {
  layerEl.value?.querySelector(`[data-dm-id="${id}"]`)?.remove()
}

/** 乐观上屏（发送成功后由父组件调用） */
function inject(item: DanmakuItem) {
  shown.add(item.id)
  const seg = Math.floor(item.time_ms / segmentMs)
  pool.get(seg)?.push(item)
  spawn(item)
}

// ---- 分段拉取 ----

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

// ---- WS 实时（M2-DM-03）----

let ws: WebSocket | null = null
let wsRetry = 0
let wsTimer: ReturnType<typeof setTimeout> | null = null

function connectWs() {
  const proto = window.location.protocol === 'https:' ? 'wss' : 'ws'
  const url = `${proto}://${window.location.host}${api.danmaku.wsUrl(props.bvid)}`
  ws = new WebSocket(url)
  ws.onopen = () => {
    wsRetry = 0
  }
  ws.onmessage = (e) => {
    try {
      const msg = JSON.parse(e.data as string)
      if (msg.type === 'danmaku' && props.enabled) {
        spawn(msg.data as DanmakuItem)
      }
    } catch {
      // 非法帧忽略
    }
  }
  ws.onclose = () => {
    ws = null
    // 断线重连（最多 5 次指数退避），失败后回退 HTTP 拉取
    if (wsRetry < 5) {
      wsRetry += 1
      wsTimer = setTimeout(connectWs, Math.min(1000 * 2 ** wsRetry, 15000))
    }
  }
  ws.onerror = () => ws?.close()
}

function closeWs() {
  if (wsTimer) {
    clearTimeout(wsTimer)
    wsTimer = null
  }
  ws?.close()
  ws = null
}

// ---- 悬停操作（暂停 + 复制/举报/屏蔽）----

function onLayerMouseOver(e: MouseEvent) {
  const el = (e.target as HTMLElement).closest('.dli-dm') as HTMLElement | null
  if (!el) return
  el.style.animationPlayState = 'paused'
  showActions(el)
}

function onLayerMouseOut(e: MouseEvent) {
  const el = (e.target as HTMLElement).closest('.dli-dm') as HTMLElement | null
  if (el) el.style.animationPlayState = 'running'
  layerEl.value?.querySelector('.dm-actions')?.remove()
}

function showActions(el: HTMLElement) {
  layerEl.value?.querySelector('.dm-actions')?.remove()
  const bar = document.createElement('div')
  bar.className = 'dm-actions'
  const itemId = el.dataset.dmId ?? ''
  const actions: Array<[string, () => void]> = [
    ['复制', () => copyContent(el)],
    ['举报', () => reportItem(itemId)],
    ['屏蔽', () => blockById(itemId)],
  ]
  for (const [label, fn] of actions) {
    const btn = document.createElement('span')
    btn.textContent = label
    btn.className = 'dm-actions__btn'
    btn.addEventListener('click', (ev) => {
      ev.stopPropagation()
      fn()
      bar.remove()
    })
    bar.appendChild(btn)
  }
  el.appendChild(bar)
}

function copyContent(el: HTMLElement) {
  void navigator.clipboard?.writeText(el.textContent ?? '').then(
    () => ElMessage.success('已复制'),
    () => ElMessage.error('复制失败'),
  )
}

function reportItem(id: string) {
  const item = poolItems().find((i) => i.id === id)
  if (item) emit('report', item)
}

function blockById(id: string) {
  const item = poolItems().find((i) => i.id === id)
  if (item) void blockSender(item)
}

function poolItems(): DanmakuItem[] {
  const all: DanmakuItem[] = []
  for (const list of pool.values()) all.push(...list)
  return all
}

// ---- 生命周期 ----

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

onMounted(() => {
  void loadBlocks()
  connectWs()
})

onBeforeUnmount(() => {
  props.video?.removeEventListener('timeupdate', onTimeUpdate)
  props.video?.removeEventListener('seeked', onSeeked)
  closeWs()
})

defineExpose({ inject })
</script>

<template>
  <div
    v-show="enabled"
    ref="layerEl"
    class="dm-layer"
    :style="{
      '--dm-opacity': settings.opacity,
      '--dm-font-scale': settings.fontScale,
    }"
    @mouseover="onLayerMouseOver"
    @mouseout="onLayerMouseOut"
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
  will-change: transform, opacity;
  padding: 1px 4px;
  border-radius: 4px;
  line-height: 28px;
  opacity: var(--dm-opacity);
  font-size: calc(20px * var(--dm-font-scale));
  /* 悬停操作交互（仅弹幕本体响应，层其余区域不拦截播放器） */
  pointer-events: auto;
  cursor: default;
}

.dm-layer :deep(.dli-dm.is-self) {
  border: 1px solid rgba(255, 255, 255, 0.8);
}

.dm-layer :deep(.dm-actions) {
  position: absolute;
  top: -24px;
  left: 0;
  display: flex;
  gap: 2px;
  padding: 2px 4px;
  border-radius: 4px;
  background: rgba(0, 0, 0, 0.75);
  z-index: 6;
}

.dm-layer :deep(.dm-actions__btn) {
  padding: 0 6px;
  font-size: 11px;
  line-height: 18px;
  color: #fff;
  cursor: pointer;
  border-radius: 3px;
}

.dm-layer :deep(.dm-actions__btn:hover) {
  background: rgba(255, 255, 255, 0.2);
}
</style>

<style lang="scss">
/* 弹幕动画 keyframes：动态创建元素的 animation 名由 JS 内联指定，
 * 若放 scoped 样式会被 Vue 编译重命名（加哈希后缀）导致匹配不上、动画不执行，
 * 因此必须定义为全局样式。 */
@keyframes dli-dm-scroll {
  from {
    transform: translateX(var(--dm-from));
  }
  to {
    transform: translateX(var(--dm-to));
  }
}

@keyframes dli-dm-fade {
  0% {
    opacity: 0;
  }
  10% {
    opacity: var(--dm-opacity);
  }
  90% {
    opacity: var(--dm-opacity);
  }
  100% {
    opacity: 0;
  }
}
</style>
