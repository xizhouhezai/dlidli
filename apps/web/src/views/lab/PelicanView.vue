<script setup lang="ts">
// 提示词实验室 · 骑自行车的鹈鹕
// 独立演示页：展示一条提示词及与其对应的矢量产物（内联手绘 SVG，无外部依赖）。
// SVG 含骑行动画（车轮旋转 / 云朵漂移 / 公路虚线流动 / 速度线脉动 / 车身起伏）与周围风景。
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
          矢量插画：一只白色鹈鹕坐在蔚蓝色自行车上沿公路前行，翅膀前伸搭在车把上，橙黄色长喙下垂着大大的喉囊，
          橙色蹼足踩着脚踏板；周围有太阳、远山、树木与草地，云朵、公路虚线与速度线构成骑行中的动感。
        </desc>

        <defs>
          <linearGradient id="pel-sky" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0" stop-color="#dceeff" />
            <stop offset="0.62" stop-color="#eef7ff" />
            <stop offset="1" stop-color="#fdfeff" />
          </linearGradient>
          <radialGradient id="pel-sun-glow" cx="0.5" cy="0.5" r="0.5">
            <stop offset="0" stop-color="#ffe9a8" stop-opacity="0.9" />
            <stop offset="0.45" stop-color="#ffe9a8" stop-opacity="0.35" />
            <stop offset="1" stop-color="#ffe9a8" stop-opacity="0" />
          </radialGradient>
          <radialGradient id="pel-sun" cx="0.4" cy="0.36" r="0.7">
            <stop offset="0" stop-color="#fff6d0" />
            <stop offset="1" stop-color="#ffcf5a" />
          </radialGradient>
          <linearGradient id="pel-hill-far" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0" stop-color="#cfe8d8" />
            <stop offset="1" stop-color="#b3d9c1" />
          </linearGradient>
          <linearGradient id="pel-hill-near" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0" stop-color="#b8e0c2" />
            <stop offset="1" stop-color="#93cf9f" />
          </linearGradient>
          <linearGradient id="pel-road" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0" stop-color="#e4eaf1" />
            <stop offset="1" stop-color="#cfd8e2" />
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

          <!-- 一棵树（树干 + 三层树冠） -->
          <g id="pel-tree">
            <rect x="-3" y="-6" width="6" height="30" rx="2.5" fill="#a9744a" />
            <circle cx="0" cy="-20" r="21" fill="#7cc47f" />
            <circle cx="-13" cy="-8" r="14" fill="#6cb771" />
            <circle cx="13" cy="-10" r="15" fill="#8bcd8d" />
          </g>
        </defs>

        <!-- 天空 -->
        <rect x="0" y="0" width="820" height="560" rx="26" fill="url(#pel-sky)" />

        <!-- 太阳 -->
        <circle cx="112" cy="102" r="66" fill="url(#pel-sun-glow)" />
        <circle cx="112" cy="102" r="30" fill="url(#pel-sun)" />

        <!-- 云朵（双份横向平铺，向左无缝漂移） -->
        <g class="pel-clouds" fill="#ffffff">
          <g opacity="0.92">
            <ellipse cx="300" cy="92" rx="52" ry="24" />
            <ellipse cx="342" cy="78" rx="34" ry="20" />
            <ellipse cx="268" cy="80" rx="28" ry="16" />
          </g>
          <g opacity="0.8">
            <ellipse cx="620" cy="74" rx="44" ry="21" />
            <ellipse cx="656" cy="62" rx="30" ry="17" />
            <ellipse cx="592" cy="64" rx="24" ry="14" />
          </g>
          <g transform="translate(820,0)" opacity="0.92">
            <ellipse cx="300" cy="92" rx="52" ry="24" />
            <ellipse cx="342" cy="78" rx="34" ry="20" />
            <ellipse cx="268" cy="80" rx="28" ry="16" />
          </g>
          <g transform="translate(820,0)" opacity="0.8">
            <ellipse cx="620" cy="74" rx="44" ry="21" />
            <ellipse cx="656" cy="62" rx="30" ry="17" />
            <ellipse cx="592" cy="64" rx="24" ry="14" />
          </g>
        </g>

        <!-- 远处飞鸟 -->
        <g fill="none" stroke="#9fb4c6" stroke-width="2.6" stroke-linecap="round" opacity="0.7">
          <path d="M248,166 C254,160 260,160 266,166" />
          <path d="M284,154 C290,148 296,148 302,154" />
          <path d="M314,170 C320,164 326,164 332,170" />
        </g>

        <!-- 远山 / 近山 -->
        <path
          d="M-20,468 C 90,356 220,348 350,406 C 470,456 590,352 720,392 C 782,412 822,438 840,462 L840,486 L-20,486 Z"
          fill="url(#pel-hill-far)"
        />
        <path
          d="M-20,484 C 130,436 300,452 430,472 C 560,492 690,444 840,468 L840,486 L-20,486 Z"
          fill="url(#pel-hill-near)"
        />

        <!-- 树木 -->
        <g transform="translate(96,442) scale(1.15)">
          <use href="#pel-tree" />
        </g>
        <g transform="translate(160,452) scale(0.95)">
          <use href="#pel-tree" />
        </g>
        <g transform="translate(694,446) scale(1.2)">
          <use href="#pel-tree" />
        </g>
        <g transform="translate(760,456) scale(0.9)">
          <use href="#pel-tree" />
        </g>

        <!-- 公路 -->
        <path d="M-20,486 L840,486 L840,560 L-20,560 Z" fill="url(#pel-road)" />
        <path d="M-20,486 L840,486" fill="none" stroke="#c1cbd7" stroke-width="3" />
        <!-- 公路中央虚线（流动） -->
        <line
          class="pel-road-line"
          x1="-20"
          y1="524"
          x2="840"
          y2="524"
          stroke="#ffffff"
          stroke-width="7"
          stroke-dasharray="46 54"
          stroke-linecap="round"
        />

        <!-- 地面投影 -->
        <ellipse cx="413" cy="490" rx="290" ry="16" fill="url(#pel-shadow)" />

        <!-- 速度线（脉动） -->
        <g
          class="pel-streaks"
          stroke="#c7d6e4"
          stroke-width="9"
          stroke-linecap="round"
          opacity="0.8"
        >
          <line x1="60" y1="286" x2="134" y2="286" />
          <line x1="40" y1="330" x2="124" y2="330" />
          <line x1="66" y1="374" x2="138" y2="374" />
        </g>

        <!-- ===== 骑行主体（整体轻微起伏）===== -->
        <g class="pel-bob">
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

          <!-- 车轮（旋转） -->
          <g transform="translate(214,402)">
            <g class="pel-spin"><use href="#pel-wheel" /></g>
          </g>
          <g transform="translate(612,402)">
            <g class="pel-spin"><use href="#pel-wheel" /></g>
          </g>

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
          <circle
            cx="500"
            cy="116"
            r="33"
            fill="url(#pel-body)"
            stroke="#c3ced8"
            stroke-width="3"
          />

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
        </g>
      </svg>
    </section>

    <p class="prompt-page__note">
      说明：本页 SVG
      为手工绘制的内联矢量图（非位图、无外部依赖），任意缩放不失真；车轮旋转、云朵漂移、公路虚线流动等动画由
      CSS
      关键帧驱动，并在系统开启「减少动态效果」时自动静止。可用于对照不同模型在同一提示词下的输出质量。
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

// ---- 骑行动画 ----
// 车轮旋转：以自身包围盒中心为轴（fill-box 避免被 viewBox 中心带偏）
.pel-spin {
  transform-box: fill-box;
  transform-origin: center;
  animation: pel-spin 0.8s linear infinite;
}

// 车身轻微起伏（模拟路面颠簸）
.pel-bob {
  animation: pel-bob 1.7s ease-in-out infinite;
}

// 云朵向左无缝漂移（内容平铺两份，位移恰好一个画布宽）
.pel-clouds {
  animation: pel-clouds 22s linear infinite;
}

// 公路中央虚线流动
.pel-road-line {
  animation: pel-road 0.9s linear infinite;
}

// 速度线脉动
.pel-streaks {
  transform-origin: left center;
  animation: pel-streaks 1s ease-in-out infinite alternate;
}

@keyframes pel-spin {
  to {
    transform: rotate(360deg);
  }
}

@keyframes pel-bob {
  0%,
  100% {
    transform: translateY(0);
  }
  50% {
    transform: translateY(-2.6px);
  }
}

@keyframes pel-clouds {
  from {
    transform: translateX(0);
  }
  to {
    transform: translateX(-820px);
  }
}

@keyframes pel-road {
  to {
    stroke-dashoffset: -100;
  }
}

@keyframes pel-streaks {
  from {
    transform: translateX(0);
    opacity: 0.8;
  }
  to {
    transform: translateX(-16px);
    opacity: 0.4;
  }
}

// 无障碍：系统开启「减少动态效果」时静止
@media (prefers-reduced-motion: reduce) {
  .pel-spin,
  .pel-bob,
  .pel-clouds,
  .pel-road-line,
  .pel-streaks {
    animation: none;
  }
}

@media (max-width: 640px) {
  .prompt-page__title {
    font-size: 22px;
  }
}
</style>
