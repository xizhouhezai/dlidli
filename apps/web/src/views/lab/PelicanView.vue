<script setup lang="ts">
// 提示词实验室 · 骑自行车的鹈鹕
// 独立演示页：展示一条提示词及与其对应的矢量产物（内联手绘 SVG，无外部依赖）。
import { onBeforeUnmount, ref } from 'vue'

const PROMPT = 'Create code for an SVG of a pelican riding a bicycle as nicely as you can.'

const copied = ref(false)
let timer: number | undefined

async function copyPrompt() {
  try {
    await navigator.clipboard.writeText(PROMPT)
    copied.value = true
    window.clearTimeout(timer)
    timer = window.setTimeout(() => {
      copied.value = false
    }, 1600)
  } catch {
    // 剪贴板不可用（非安全上下文等）时静默忽略
  }
}

onBeforeUnmount(() => window.clearTimeout(timer))
</script>

<template>
  <div class="prompt-page">
    <header class="prompt-page__head">
      <p class="prompt-page__eyebrow">
        <span class="i-mingcute-flask-line" />
        提示词实验室 · Prompt Lab
      </p>
      <h1 class="prompt-page__title">骑自行车的鹈鹕</h1>
      <p class="prompt-page__lede">
        用一句提示词生成矢量插图。下面是提示词原文，以及它对应的产物——一幅内联手绘的 SVG。
      </p>
    </header>

    <section class="prompt-card">
      <div class="prompt-card__bar">
        <span class="prompt-card__label">PROMPT</span>
        <button class="prompt-card__copy" type="button" @click="copyPrompt">
          <span :class="copied ? 'i-mingcute-check-line' : 'i-mingcute-copy-2-line'" />
          {{ copied ? '已复制' : '复制' }}
        </button>
      </div>
      <p class="prompt-card__text">{{ PROMPT }}</p>
    </section>

    <section class="canvas">
      <svg
        class="pelican"
        viewBox="0 0 820 560"
        xmlns="http://www.w3.org/2000/svg"
        role="img"
        aria-labelledby="pel-title pel-desc"
        preserveAspectRatio="xMidYMid meet"
      >
        <title id="pel-title">一只骑自行车的鹈鹕</title>
        <desc id="pel-desc">
          矢量插画：一只白色鹈鹕坐在蔚蓝色自行车的车座上，翅膀前伸搭在车把上，橙黄色的长喙下垂着大大的喉囊，
          橙色的蹼足踩着脚踏板，左侧是速度线。
        </desc>

        <defs>
          <linearGradient id="pel-sky" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0" stop-color="#e9f4ff" />
            <stop offset="1" stop-color="#fdfeff" />
          </linearGradient>
          <radialGradient id="pel-shadow" cx="0.5" cy="0.5" r="0.5">
            <stop offset="0" stop-color="#0b2233" stop-opacity="0.2" />
            <stop offset="1" stop-color="#0b2233" stop-opacity="0" />
          </radialGradient>
          <linearGradient id="pel-body" x1="0.18" y1="0" x2="0.7" y2="1">
            <stop offset="0" stop-color="#ffffff" />
            <stop offset="0.6" stop-color="#f5f8fa" />
            <stop offset="1" stop-color="#dae3ec" />
          </linearGradient>
          <linearGradient id="pel-wing" x1="0.1" y1="0" x2="0.55" y2="1">
            <stop offset="0" stop-color="#ffffff" />
            <stop offset="1" stop-color="#cbd7e1" />
          </linearGradient>
          <linearGradient id="pel-beak" x1="0" y1="0" x2="1" y2="0.5">
            <stop offset="0" stop-color="#ffd461" />
            <stop offset="0.55" stop-color="#ffb01f" />
            <stop offset="1" stop-color="#ef8c0c" />
          </linearGradient>
          <linearGradient id="pel-pouch" x1="0" y1="0" x2="0.3" y2="1">
            <stop offset="0" stop-color="#ffdc7d" />
            <stop offset="1" stop-color="#f6a327" />
          </linearGradient>
          <linearGradient id="pel-frame" x1="0" y1="0" x2="1" y2="1">
            <stop offset="0" stop-color="#4bb2d8" />
            <stop offset="1" stop-color="#1d6d94" />
          </linearGradient>

          <g id="pel-wheel">
            <circle r="86" fill="none" stroke="#2b3038" stroke-width="13" />
            <circle r="70" fill="none" stroke="#cfd6de" stroke-width="5" />
            <g stroke="#b9c2cc" stroke-width="2.4">
              <line x1="0" y1="-68" x2="0" y2="68" />
              <line x1="-68" y1="0" x2="68" y2="0" />
              <line x1="-48" y1="-48" x2="48" y2="48" />
              <line x1="-48" y1="48" x2="48" y2="-48" />
              <line x1="-59" y1="-34" x2="59" y2="34" />
              <line x1="-59" y1="34" x2="59" y2="-34" />
            </g>
            <circle r="10" fill="#5b6572" />
            <circle r="3.5" fill="#dfe5ea" />
          </g>
        </defs>

        <!-- 背景与云 -->
        <rect x="0" y="0" width="820" height="560" rx="26" fill="url(#pel-sky)" />
        <g fill="#ffffff" opacity="0.92">
          <ellipse cx="150" cy="108" rx="52" ry="25" />
          <ellipse cx="192" cy="94" rx="36" ry="21" />
          <ellipse cx="118" cy="98" rx="30" ry="17" />
        </g>
        <g fill="#ffffff" opacity="0.75">
          <ellipse cx="676" cy="84" rx="42" ry="21" />
          <ellipse cx="710" cy="74" rx="28" ry="16" />
        </g>

        <!-- 地面投影 -->
        <ellipse cx="413" cy="502" rx="300" ry="26" fill="url(#pel-shadow)" />

        <!-- 速度线 -->
        <g stroke="#cdd9e4" stroke-width="9" stroke-linecap="round" opacity="0.8">
          <line x1="76" y1="290" x2="148" y2="290" />
          <line x1="54" y1="334" x2="138" y2="334" />
          <line x1="82" y1="378" x2="152" y2="378" />
        </g>

        <!-- 远侧腿 + 远侧脚踏（在车架后方；颜色更暗以拉开层次） -->
        <path
          d="M348,260 C352,302 354,338 358,362"
          fill="none"
          stroke="#c98f3e"
          stroke-width="11"
          stroke-linecap="round"
        />
        <path
          d="M338,358 L374,360 L370,376 L342,376 Z"
          fill="#c98f3e"
          stroke="#a9712a"
          stroke-width="2"
          stroke-linejoin="round"
        />
        <path d="M392,404 L358,370" stroke="#333b47" stroke-width="9" stroke-linecap="round" />

        <!-- 链条 + 飞轮 -->
        <g stroke="#4a5260" stroke-width="3" fill="none">
          <path d="M392,378 L214,390" />
          <path d="M392,430 L214,414" />
        </g>
        <circle cx="214" cy="402" r="13" fill="#39414f" />

        <!-- 车轮 -->
        <use href="#pel-wheel" x="214" y="402" />
        <use href="#pel-wheel" x="612" y="402" />

        <!-- 车架 -->
        <g stroke="url(#pel-frame)" stroke-width="12" stroke-linecap="round" fill="none">
          <path d="M214,402 L392,404" />
          <path d="M368,254 L392,404" />
          <path d="M392,404 L584,292" />
          <path d="M376,262 L582,252" />
          <path d="M214,402 L368,254" />
          <path d="M580,246 L612,402" />
          <path d="M580,246 L574,228" />
        </g>

        <!-- 牙盘 -->
        <circle cx="392" cy="404" r="26" fill="none" stroke="#39414f" stroke-width="6" />
        <circle
          cx="392"
          cy="404"
          r="29"
          fill="none"
          stroke="#39414f"
          stroke-width="4"
          stroke-dasharray="3 7"
        />
        <circle cx="392" cy="404" r="7" fill="#39414f" />

        <!-- 车座 -->
        <path
          d="M320,252 C336,236 386,232 408,242 C412,254 398,262 374,262 C348,262 326,260 320,252 Z"
          fill="#39414f"
        />

        <!-- 车把 -->
        <path
          d="M552,244 C566,222 598,216 632,228"
          fill="none"
          stroke="#39414f"
          stroke-width="10"
          stroke-linecap="round"
        />
        <circle cx="550" cy="245" r="8" fill="#2f3742" />
        <circle cx="634" cy="227" r="8" fill="#2f3742" />

        <!-- 鹈鹕：尾羽 -->
        <path
          d="M266,204 C226,178 188,160 156,158 C176,184 180,210 168,246 C202,240 236,236 270,238 Z"
          fill="url(#pel-body)"
          stroke="#c3ced8"
          stroke-width="3"
          stroke-linejoin="round"
        />
        <g stroke="#c3ced8" stroke-width="2" fill="none">
          <path d="M258,206 C226,190 198,176 170,172" />
          <path d="M262,222 C232,214 204,210 176,210" />
        </g>

        <!-- 鹈鹕：身体 -->
        <path
          d="M256,214 C252,160 298,138 352,140 C406,142 448,172 448,216 C448,266 404,288 352,286 C300,284 260,268 256,214 Z"
          fill="url(#pel-body)"
          stroke="#c3ced8"
          stroke-width="3"
        />

        <!-- 鹈鹕：翅膀（自肩部向前收窄，前缘搭向车把） -->
        <path
          d="M380,190 C426,196 486,212 540,230 C560,237 570,244 578,249 C566,255 552,256 540,253 C486,242 432,234 388,232 C366,230 362,192 380,190 Z"
          fill="url(#pel-wing)"
          stroke="#c3ced8"
          stroke-width="3"
        />
        <g stroke="#c3ced8" stroke-width="2" fill="none" opacity="0.9">
          <path d="M418,206 C462,216 508,230 548,246" />
          <path d="M398,220 C442,228 490,238 532,250" />
        </g>

        <!-- 鹈鹕：脖子（粗实、平滑连接身体与头） -->
        <path
          d="M398,198 C406,150 440,118 480,110 C498,106 514,112 518,126 C522,140 512,150 498,152 C466,156 444,182 434,214 Z"
          fill="url(#pel-body)"
          stroke="#c3ced8"
          stroke-width="3"
        />

        <!-- 鹈鹕：头 -->
        <circle cx="500" cy="116" r="33" fill="url(#pel-body)" stroke="#c3ced8" stroke-width="3" />

        <!-- 鹈鹕：喉囊（自喙基沿喙下缘下垂的袋状，先画在底层） -->
        <path
          d="M528,120 C556,126 610,136 700,150 C696,190 654,216 604,210 C558,204 532,164 528,120 Z"
          fill="url(#pel-pouch)"
          stroke="#d0870f"
          stroke-width="2"
          stroke-linejoin="round"
        />
        <path
          d="M558,138 C580,168 602,192 628,202"
          fill="none"
          stroke="#fff0c8"
          stroke-width="3"
          opacity="0.6"
        />
        <!-- 喉囊里露出的小鱼尾 -->
        <path
          d="M596,206 L586,224 L603,219 L618,234 L615,208 Z"
          fill="#8fd6ea"
          stroke="#4bb4d4"
          stroke-width="2"
          stroke-linejoin="round"
        />

        <!-- 鹈鹕：上喙（细长、近水平微下垂，覆盖喉囊上缘） -->
        <path
          d="M522,104 C594,110 664,124 722,140 C726,142 723,149 715,148 C660,142 594,131 522,118 Z"
          fill="url(#pel-beak)"
          stroke="#c97e0e"
          stroke-width="2"
          stroke-linejoin="round"
        />
        <path
          d="M540,110 C600,118 660,131 704,143"
          fill="none"
          stroke="#ffe9b0"
          stroke-width="3"
          opacity="0.8"
        />

        <!-- 鹈鹕：眼睛 -->
        <circle cx="506" cy="108" r="9" fill="#ffffff" stroke="#c3ced8" stroke-width="1.5" />
        <circle cx="508" cy="108" r="4.5" fill="#263238" />
        <circle cx="506" cy="106" r="1.8" fill="#ffffff" />

        <!-- 近侧腿 + 近侧脚（在车架前方） -->
        <path
          d="M374,266 C386,326 404,388 424,430"
          fill="none"
          stroke="#f0932b"
          stroke-width="14"
          stroke-linecap="round"
        />
        <path
          d="M406,430 L448,430 L440,451 L414,451 Z"
          fill="#f0932b"
          stroke="#d9801a"
          stroke-width="2"
          stroke-linejoin="round"
        />
        <path d="M392,404 L426,436" stroke="#333b47" stroke-width="9" stroke-linecap="round" />
        <rect x="410" y="431" width="32" height="11" rx="5" fill="#2f3742" />
      </svg>
    </section>

    <p class="prompt-page__note">
      说明：本页 SVG
      为手工绘制的内联矢量图（非位图、无外部依赖），任意缩放不失真；页面直接渲染该提示词的
      理想产物，可用于对照不同模型在同一提示词下的输出质量。
    </p>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/variables' as v;

.prompt-page {
  max-width: 900px;
  margin: 0 auto;
}

.prompt-page__eyebrow {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin: 0 0 10px;
  padding: 4px 10px;
  border-radius: 999px;
  background: v.$primary-light;
  color: v.$primary;
  font-size: 12.5px;
  font-weight: 600;
}

.prompt-page__title {
  margin: 0 0 8px;
  font-size: 28px;
  line-height: 1.2;
}

.prompt-page__lede {
  margin: 0 0 20px;
  color: v.$text-2;
  font-size: 14px;
  line-height: 1.7;
}

.prompt-card {
  margin-bottom: 20px;
  border: 1px solid v.$border;
  border-radius: 12px;
  background: #fff;
  overflow: hidden;
}

.prompt-card__bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 14px;
  background: #fafbfc;
  border-bottom: 1px solid v.$border;
}

.prompt-card__label {
  font-size: 11.5px;
  font-weight: 700;
  letter-spacing: 0.08em;
  color: v.$text-3;
}

.prompt-card__copy {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 4px 10px;
  border: 1px solid v.$border;
  border-radius: 8px;
  background: #fff;
  color: v.$text-2;
  font-size: 12.5px;
  cursor: pointer;
  transition:
    color 0.15s,
    border-color 0.15s;

  &:hover {
    color: v.$primary;
    border-color: v.$primary;
  }
}

.prompt-card__text {
  margin: 0;
  padding: 16px 14px;
  font-family: ui-monospace, 'SFMono-Regular', Menlo, Consolas, monospace;
  font-size: 14.5px;
  line-height: 1.7;
  color: v.$text-1;
  word-break: break-word;
}

.canvas {
  border-radius: 16px;
  overflow: hidden;
  background: #fff;
  box-shadow: v.$shadow-card;
  line-height: 0;
}

.pelican {
  display: block;
  width: 100%;
  height: auto;
}

.prompt-page__note {
  margin: 14px 0 0;
  color: v.$text-3;
  font-size: 12.5px;
  line-height: 1.7;
}

@media (max-width: 640px) {
  .prompt-page__title {
    font-size: 22px;
  }
}
</style>
