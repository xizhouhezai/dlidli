<script setup lang="ts">
// 礼物特效渲染层（M4-LIV-04）：普通礼物走弹幕区连击样式（浮层字幕），
// 高级礼物全屏 CSS 动画（轻量实现，不引动画库）。
import { onBeforeUnmount, ref, watch } from 'vue'

export interface GiftEffect {
  id: string
  name: string
  sender: string
  count: number
  level: 1 | 2
}

const props = defineProps<{ events: GiftEffect[] }>()

// 浮层连击条（普通礼物）：每笔一条 4s 自动退场
const banners = ref<GiftEffect[]>([])
// 全屏特效（高级礼物）：同一时间至多一个，4s 退场
const screenFx = ref<GiftEffect | null>(null)
let timer: number | null = null

watch(
  () => props.events,
  (list) => {
    const latest = list[list.length - 1]
    if (!latest) return
    if (latest.level === 2) {
      screenFx.value = latest
      if (timer) window.clearTimeout(timer)
      timer = window.setTimeout(() => (screenFx.value = null), 4000)
    } else {
      banners.value.push(latest)
      if (banners.value.length > 5) banners.value.shift()
      window.setTimeout(() => {
        banners.value = banners.value.filter((b) => b.id !== latest.id)
      }, 4000)
    }
  },
)

onBeforeUnmount(() => {
  if (timer) window.clearTimeout(timer)
})
</script>

<template>
  <!-- 普通礼物：连击浮层字幕（弹幕区上方） -->
  <transition-group name="gift-banner" tag="div" class="gift-fx__banners">
    <div v-for="b in banners" :key="b.id" class="gift-fx__banner">
      <span class="gift-fx__banner-icon" />
      <span class="gift-fx__banner-text">
        <b>{{ b.sender }}</b> 送出 <em>{{ b.name }}</em> ×{{ b.count }}
      </span>
    </div>
  </transition-group>

  <!-- 高级礼物：全屏 CSS 动画（粒子上浮 + 礼物名浮现） -->
  <div v-if="screenFx" class="gift-fx__screen">
    <span v-for="i in 12" :key="i" class="gift-fx__particle" :style="{ '--i': i }" />
    <div class="gift-fx__card">
      <p class="gift-fx__gift-name">{{ screenFx.name }} ×{{ screenFx.count }}</p>
      <p class="gift-fx__sender">{{ screenFx.sender }} 送给主播</p>
    </div>
  </div>
</template>

<style scoped lang="scss">
.gift-fx__banners {
  position: absolute;
  left: 10px;
  bottom: 52px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  pointer-events: none;
  z-index: 10;
}

.gift-fx__banner {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 12px 4px 6px;
  border-radius: 999px;
  background: linear-gradient(90deg, rgba(240, 109, 146, 0.92), rgba(249, 167, 185, 0.85));
  color: #fff;
  font-size: 13px;
  width: fit-content;
}

.gift-fx__banner-icon {
  width: 22px;
  height: 22px;
  border-radius: 50%;
  background: radial-gradient(circle at 35% 35%, #fff, #ffd66b 60%);
}

.gift-fx__banner-text {
  em {
    font-style: normal;
    font-weight: 700;
    color: #ffe9a8;
  }
}

.gift-banner-enter-active,
.gift-banner-leave-active {
  transition: all 0.3s ease;
}
.gift-banner-enter-from,
.gift-banner-leave-to {
  opacity: 0;
  transform: translateX(-16px);
}

/* ---- 全屏特效 ---- */
.gift-fx__screen {
  position: absolute;
  inset: 0;
  overflow: hidden;
  pointer-events: none;
  z-index: 20;
  background: radial-gradient(ellipse at center, rgba(255, 214, 107, 0.18), transparent 65%);
}

.gift-fx__card {
  position: absolute;
  top: 30%;
  left: 50%;
  transform: translateX(-50%);
  text-align: center;
  animation: fx-card 0.6s cubic-bezier(0.22, 1.4, 0.4, 1) both;
}

.gift-fx__gift-name {
  margin: 0;
  font-size: 34px;
  font-weight: 800;
  color: #ffd66b;
  text-shadow:
    0 2px 12px rgba(245, 158, 11, 0.75),
    0 0 4px rgba(255, 255, 255, 0.6);
}

.gift-fx__sender {
  margin: 6px 0 0;
  font-size: 14px;
  color: #fff;
  text-shadow: 0 1px 6px rgba(0, 0, 0, 0.7);
}

.gift-fx__particle {
  position: absolute;
  bottom: -20px;
  left: calc((var(--i) - 1) * 8.3% + 2%);
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: radial-gradient(circle at 35% 35%, #fff, #ffd66b 65%, #f59e0b);
  opacity: 0.9;
  animation: fx-rise calc(1.6s + var(--i) * 0.12s) ease-in infinite;
}

@keyframes fx-rise {
  0% {
    transform: translateY(0) scale(1);
    opacity: 0.9;
  }
  100% {
    transform: translateY(-105%) scale(0.4);
    opacity: 0;
  }
}

@keyframes fx-card {
  0% {
    transform: translateX(-50%) scale(0.4);
    opacity: 0;
  }
  100% {
    transform: translateX(-50%) scale(1);
    opacity: 1;
  }
}
</style>
