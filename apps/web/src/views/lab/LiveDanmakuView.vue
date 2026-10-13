<template>
  <div class="liv03-panel">
    <h2>直播弹幕测试面板（M4-LIV-03，dev 验证用）</h2>
    <p class="hint">
      非正式直播间页面（LIV-06 另做）。仅验证：WS 进房 / 进场消息 / 弹幕广播 / 频控。
    </p>
    <div class="row">
      <input v-model="roomId" placeholder="房间 ID" />
      <button :disabled="connected" @click="connect">连接</button>
      <button :disabled="!connected" @click="disconnect">断开</button>
      <span :class="connected ? 'on' : 'off'">{{ connected ? '已连接' : '未连接' }}</span>
    </div>
    <div class="row" v-if="connected">
      <input v-model="draft" placeholder="弹幕内容（2s 频控）" @keyup.enter="send" />
      <button @click="send">发送</button>
    </div>
    <ul class="frames">
      <li v-for="(f, i) in frames" :key="i" :class="f.type">[{{ f.type }}] {{ f.summary }}</li>
    </ul>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { readToken } from '@/utils/token'

const roomId = ref('')
const draft = ref('')
const connected = ref(false)
const frames = ref<{ type: string; summary: string }[]>([])
let ws: WebSocket | null = null
let hbTimer: number | null = null

function push(type: string, summary: string) {
  frames.value.unshift({ type, summary })
  if (frames.value.length > 100) frames.value.pop()
}

function connect() {
  const token = readToken()
  if (!token || !roomId.value) return
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  ws = new WebSocket(
    `${proto}://${location.host}/api/v1/live/rooms/${roomId.value}/ws?token=${token}`,
  )
  ws.onopen = () => {
    connected.value = true
    // 心跳 ≤15s（spec live §5B）
    hbTimer = window.setInterval(() => ws?.send(JSON.stringify({ type: 'heartbeat' })), 10000)
  }
  ws.onmessage = (ev) => {
    const m = JSON.parse(ev.data)
    if (m.type === 'enter')
      push('enter', `${m.data.nickname} 进入直播间（徽章 ${m.data.fan_badge}）`)
    else if (m.type === 'danmaku') push('danmaku', `${m.data.nickname}: ${m.data.content}`)
    else if (m.type === 'pong') push('pong', '心跳应答')
  }
  ws.onclose = () => {
    connected.value = false
    if (hbTimer) window.clearInterval(hbTimer)
  }
}

function disconnect() {
  ws?.close(1000)
  ws = null
  connected.value = false
  if (hbTimer) window.clearInterval(hbTimer)
}

function send() {
  if (!ws || !draft.value) return
  ws.send(JSON.stringify({ type: 'danmaku', content: draft.value }))
  draft.value = ''
}
</script>

<style scoped>
.liv03-panel {
  max-width: 640px;
  margin: 24px auto;
  padding: 16px;
}
.hint {
  color: #888;
  font-size: 13px;
}
.row {
  display: flex;
  gap: 8px;
  margin: 12px 0;
  align-items: center;
}
.row input {
  flex: 1;
  padding: 6px 10px;
}
.on {
  color: #2a9d52;
}
.off {
  color: #999;
}
.frames {
  list-style: none;
  padding: 0;
  max-height: 400px;
  overflow-y: auto;
}
.frames li {
  padding: 4px 8px;
  border-bottom: 1px solid #eee;
  font-size: 13px;
}
.frames li.enter {
  color: #1a73c7;
}
.frames li.danmaku {
  color: #333;
}
.frames li.pong {
  color: #aaa;
}
</style>
