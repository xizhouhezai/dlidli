<script setup lang="ts">
// 提示词实验室 · 夜骑的鹈鹕（/pelican-night）
// 与日间水彩版（/pelican）互为对照：同一提示词、不同视觉方向——夜色霓虹。
// 内联手绘 SVG，无外部依赖；车轮旋转 / 公路虚线流动 / 星星闪烁 / 车灯呼吸等动效由 CSS 关键帧驱动。
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
  <div class="night-page">
    <header class="night-page__head">
      <p class="night-page__eyebrow">
        <span class="i-mingcute-moon-line" />
        提示词实验室 · Night Ride
      </p>
      <h1 class="night-page__title">夜骑的鹈鹕</h1>
      <p class="night-page__lede">
        同一句提示词，换一种视觉方向：白昼水彩之外，还有夜色霓虹。下面是提示词原文，以及它对应的产物——一幅内联手绘的
        SVG。
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
        class="pelican-night"
        viewBox="0 0 860 580"
        xmlns="http://www.w3.org/2000/svg"
        role="img"
        aria-labelledby="pn-title pn-desc"
        preserveAspectRatio="xMidYMid meet"
      >
        <title id="pn-title">一只在夜色中骑自行车的鹈鹕</title>
        <desc id="pn-desc">
          矢量插画：深蓝紫色夜空中挂着一轮明月与点点繁星，远处是亮着窗火的城市天际线；一只白色鹈鹕骑着蔚蓝色霓虹自行车
          沿公路前行，翅膀前伸搭在车把上，橙黄色长喙下垂着喉囊，橙色蹼足踩着脚踏板；车头灯打出暖黄光束，车后拖曳着
          青蓝与品红的速度光轨。
        </desc>

        <defs>
          <linearGradient id="pn-sky" x1="0" y1="0" x2="0.12" y2="1">
            <stop offset="0" stop-color="#04060e" />
            <stop offset="0.3" stop-color="#0b1128" />
            <stop offset="0.55" stop-color="#191a42" />
            <stop offset="0.76" stop-color="#3a2054" />
            <stop offset="0.88" stop-color="#632c5e" />
            <stop offset="1" stop-color="#8a3a5e" />
          </linearGradient>

          <radialGradient id="pn-moon-glow" cx="0.5" cy="0.5" r="0.5">
            <stop offset="0" stop-color="#d6e8ff" stop-opacity="0.55" />
            <stop offset="0.35" stop-color="#a8c8ff" stop-opacity="0.18" />
            <stop offset="1" stop-color="#a8c8ff" stop-opacity="0" />
          </radialGradient>
          <radialGradient id="pn-moon" cx="0.38" cy="0.32" r="0.75">
            <stop offset="0" stop-color="#fffdf4" />
            <stop offset="0.55" stop-color="#f2ead0" />
            <stop offset="1" stop-color="#d3c9a2" />
          </radialGradient>

          <linearGradient id="pn-bldg" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0" stop-color="#182046" />
            <stop offset="0.55" stop-color="#0d1330" />
            <stop offset="1" stop-color="#070a1a" />
          </linearGradient>
          <pattern id="pn-windows" width="20" height="26" patternUnits="userSpaceOnUse">
            <rect x="4" y="5" width="7" height="10" rx="1" fill="#ffd98a" opacity="0.5" />
          </pattern>

          <linearGradient id="pn-road" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0" stop-color="#3a2c52" />
            <stop offset="0.1" stop-color="#1b1d33" />
            <stop offset="0.5" stop-color="#121422" />
            <stop offset="1" stop-color="#080a14" />
          </linearGradient>

          <linearGradient id="pn-beam" x1="0" y1="0" x2="1" y2="0.15">
            <stop offset="0" stop-color="#fff6cf" stop-opacity="0.5" />
            <stop offset="0.45" stop-color="#ffe9a8" stop-opacity="0.16" />
            <stop offset="1" stop-color="#ffe9a8" stop-opacity="0" />
          </linearGradient>
          <radialGradient id="pn-pool" cx="0.5" cy="0.5" r="0.5">
            <stop offset="0" stop-color="#ffdf9b" stop-opacity="0.42" />
            <stop offset="0.55" stop-color="#ffcf7a" stop-opacity="0.14" />
            <stop offset="1" stop-color="#ffcf7a" stop-opacity="0" />
          </radialGradient>
          <radialGradient id="pn-lamp" cx="0.5" cy="0.5" r="0.5">
            <stop offset="0" stop-color="#ffd98a" stop-opacity="0.5" />
            <stop offset="1" stop-color="#ffb347" stop-opacity="0" />
          </radialGradient>

          <linearGradient id="pn-body" x1="0.12" y1="0" x2="0.78" y2="1">
            <stop offset="0" stop-color="#ffffff" />
            <stop offset="0.42" stop-color="#eef4fb" />
            <stop offset="1" stop-color="#a9bcd4" />
          </linearGradient>
          <linearGradient id="pn-wing" x1="0.05" y1="0" x2="0.85" y2="1">
            <stop offset="0" stop-color="#ffffff" />
            <stop offset="0.5" stop-color="#dfe9f5" />
            <stop offset="1" stop-color="#9db3cd" />
          </linearGradient>
          <linearGradient id="pn-beak" x1="0" y1="0" x2="1" y2="0.4">
            <stop offset="0" stop-color="#ffdb7d" />
            <stop offset="0.5" stop-color="#ffab1f" />
            <stop offset="1" stop-color="#e67c12" />
          </linearGradient>
          <linearGradient id="pn-pouch" x1="0.1" y1="0" x2="0.3" y2="1">
            <stop offset="0" stop-color="#ffdd85" />
            <stop offset="0.5" stop-color="#f5a52a" />
            <stop offset="1" stop-color="#cf6f0e" />
          </linearGradient>
          <linearGradient id="pn-frame" x1="0" y1="0" x2="0.9" y2="1">
            <stop offset="0" stop-color="#5ceaff" />
            <stop offset="0.45" stop-color="#1e93c8" />
            <stop offset="1" stop-color="#0f3352" />
          </linearGradient>

          <radialGradient id="pn-shadow" cx="0.5" cy="0.5" r="0.5">
            <stop offset="0" stop-color="#000000" stop-opacity="0.5" />
            <stop offset="1" stop-color="#000000" stop-opacity="0" />
          </radialGradient>

          <filter id="pn-soft" x="-80%" y="-80%" width="260%" height="260%">
            <feGaussianBlur stdDeviation="16" />
          </filter>

          <clipPath id="pn-frame-clip">
            <rect x="0" y="0" width="860" height="580" rx="24" />
          </clipPath>
          <clipPath id="pn-sky-clip">
            <path
              d="M -10,470 L -10,396 L 40,396 L 40,352 L 96,352 L 96,410 L 150,410 L 150,316
                 L 214,316 L 214,372 L 258,372 L 258,288 L 322,288 L 322,344 L 368,344 L 368,400
                 L 430,400 L 430,330 L 488,330 L 488,386 L 540,386 L 540,300 L 604,300 L 604,358
                 L 660,358 L 660,404 L 716,404 L 716,340 L 776,340 L 776,392 L 830,392 L 830,366
                 L 880,366 L 880,470 Z"
            />
          </clipPath>

          <g id="pn-wheel">
            <circle r="84" fill="none" stroke="#0d1524" stroke-width="13" />
            <circle r="84" fill="none" stroke="#38e4ff" stroke-width="3.5" opacity="0.95" />
            <circle r="73" fill="none" stroke="#2b6480" stroke-width="2.6" />
            <g stroke="#45dcf5" stroke-width="1.6" opacity="0.5">
              <line x1="-72" y1="0" x2="72" y2="0" />
              <line x1="0" y1="-72" x2="0" y2="72" />
              <line x1="-58.25" y1="-42.32" x2="58.25" y2="42.32" />
              <line x1="-58.25" y1="42.32" x2="58.25" y2="-42.32" />
              <line x1="-22.25" y1="-68.48" x2="22.25" y2="68.48" />
              <line x1="-22.25" y1="68.48" x2="22.25" y2="-68.48" />
            </g>
            <circle r="9" fill="#16303f" />
            <circle r="3.5" fill="#9df3ff" />
          </g>

          <g id="pn-tree">
            <rect x="-2.5" y="-6" width="5" height="26" rx="2" fill="#2b2338" />
            <circle cx="0" cy="-18" r="17" fill="#16203a" />
            <circle cx="-11" cy="-7" r="11" fill="#111a31" />
            <circle cx="11" cy="-9" r="12" fill="#1a2543" />
          </g>
        </defs>

        <g clip-path="url(#pn-frame-clip)">
          <!-- 夜空 -->
          <rect x="0" y="0" width="860" height="580" fill="url(#pn-sky)" />

          <!-- 星星（分三组错开闪烁节奏） -->
          <g class="pn-tw-a" fill="#ffffff">
            <circle cx="46" cy="58" r="1.6" opacity="0.9" />
            <circle cx="140" cy="46" r="1.9" opacity="0.95" />
            <circle cx="214" cy="88" r="1.4" opacity="0.75" />
            <circle cx="300" cy="150" r="1.7" opacity="0.85" />
            <circle cx="386" cy="128" r="1.5" opacity="0.8" />
            <circle cx="508" cy="66" r="1.8" opacity="0.9" />
            <circle cx="600" cy="52" r="1.4" opacity="0.75" />
            <circle cx="806" cy="60" r="1.6" opacity="0.85" />
          </g>
          <g class="pn-tw-b" fill="#ffffff">
            <circle cx="92" cy="132" r="1.1" opacity="0.6" />
            <circle cx="176" cy="176" r="1.2" opacity="0.55" />
            <circle cx="262" cy="40" r="1.1" opacity="0.6" />
            <circle cx="338" cy="72" r="1.2" opacity="0.6" />
            <circle cx="424" cy="38" r="1.1" opacity="0.55" />
            <circle cx="556" cy="146" r="1.1" opacity="0.55" />
            <circle cx="838" cy="164" r="1.2" opacity="0.6" />
            <circle cx="782" cy="222" r="1.3" opacity="0.55" />
          </g>
          <g class="pn-tw-c" fill="#ffffff">
            <circle cx="470" cy="182" r="1.3" opacity="0.65" />
            <circle cx="636" cy="200" r="1.2" opacity="0.5" />
            <circle cx="114" cy="242" r="1.1" opacity="0.45" />
            <circle cx="70" cy="318" r="1.3" opacity="0.5" />
            <circle cx="452" cy="252" r="1.1" opacity="0.4" />
            <circle cx="368" cy="216" r="1.2" opacity="0.45" />
          </g>
          <!-- 闪星 -->
          <g fill="#ffffff" opacity="0.85">
            <path d="M628,96 l2.6,7.4 7.4,2.6 -7.4,2.6 -2.6,7.4 -2.6,-7.4 -7.4,-2.6 7.4,-2.6 Z" />
            <path d="M188,120 l2.2,6.2 6.2,2.2 -6.2,2.2 -2.2,6.2 -2.2,-6.2 -6.2,-2.2 6.2,-2.2 Z" />
            <path
              d="M760,150 l1.9,5.4 5.4,1.9 -5.4,1.9 -1.9,5.4 -1.9,-5.4 -5.4,-1.9 5.4,-1.9 Z"
              opacity="0.7"
            />
          </g>

          <!-- 月亮 -->
          <circle cx="704" cy="104" r="148" fill="url(#pn-moon-glow)" />
          <circle cx="704" cy="104" r="52" fill="url(#pn-moon)" />
          <g fill="#c9bf98" opacity="0.32">
            <circle cx="688" cy="92" r="8" />
            <circle cx="716" cy="120" r="11" />
            <circle cx="722" cy="84" r="5.5" />
            <circle cx="692" cy="128" r="4.5" />
          </g>

          <!-- 城市天际线 -->
          <path
            d="M -10,470 L -10,396 L 40,396 L 40,352 L 96,352 L 96,410 L 150,410 L 150,316
               L 214,316 L 214,372 L 258,372 L 258,288 L 322,288 L 322,344 L 368,344 L 368,400
               L 430,400 L 430,330 L 488,330 L 488,386 L 540,386 L 540,300 L 604,300 L 604,358
               L 660,358 L 660,404 L 716,404 L 716,340 L 776,340 L 776,392 L 830,392 L 830,366
               L 880,366 L 880,470 Z"
            fill="url(#pn-bldg)"
          />
          <g clip-path="url(#pn-sky-clip)">
            <rect x="-10" y="280" width="900" height="200" fill="url(#pn-windows)" opacity="0.75" />
            <rect
              x="-10"
              y="280"
              width="900"
              height="200"
              fill="url(#pn-windows)"
              opacity="0.4"
              transform="translate(9,13)"
            />
            <g fill="#7ff0ff" opacity="0.5">
              <rect x="272" y="304" width="7" height="10" rx="1" />
              <rect x="552" y="318" width="7" height="10" rx="1" />
              <rect x="560" y="344" width="7" height="10" rx="1" />
              <rect x="176" y="336" width="7" height="10" rx="1" />
            </g>
          </g>
          <!-- 楼顶信号灯与天线 -->
          <g stroke="#2c3763" stroke-width="2" fill="none">
            <line x1="286" y1="288" x2="286" y2="252" />
            <line x1="568" y1="300" x2="568" y2="266" />
          </g>
          <circle cx="286" cy="250" r="3.4" fill="#ff5f7a" opacity="0.95" />
          <circle cx="568" cy="264" r="3" fill="#ff5f7a" opacity="0.8" />

          <!-- 公路 -->
          <path d="M -10,470 L 880,470 L 880,590 L -10,590 Z" fill="url(#pn-road)" />
          <path
            d="M -10,470 L 880,470"
            fill="none"
            stroke="#4b3a6b"
            stroke-width="2.4"
            opacity="0.8"
          />
          <g opacity="0.4">
            <ellipse cx="300" cy="486" rx="180" ry="7" fill="#5a3f7d" />
            <ellipse cx="640" cy="482" rx="150" ry="6" fill="#3f5f86" />
          </g>
          <!-- 中央虚线（流动） -->
          <line
            class="pn-road-line"
            x1="-20"
            y1="530"
            x2="880"
            y2="530"
            stroke="#c9d4ea"
            stroke-width="5"
            stroke-dasharray="40 52"
            stroke-linecap="round"
            opacity="0.5"
          />
          <!-- 霓虹倒影 -->
          <g opacity="0.3">
            <rect x="150" y="496" width="120" height="3" rx="1.5" fill="#38e4ff" />
            <rect x="470" y="504" width="90" height="3" rx="1.5" fill="#ff4fa3" />
            <rect x="600" y="558" width="150" height="3" rx="1.5" fill="#38e4ff" />
          </g>

          <!-- 路灯 -->
          <g>
            <ellipse cx="66" cy="486" rx="76" ry="20" fill="url(#pn-lamp)" />
            <path
              d="M 66,486 L 66,300 C 66,282 78,272 96,270"
              fill="none"
              stroke="#1a2138"
              stroke-width="7"
              stroke-linecap="round"
            />
            <ellipse cx="103" cy="270" rx="30" ry="24" fill="url(#pn-lamp)" />
            <path d="M 86,272 C 88,264 96,258 104,258 C 114,258 120,264 120,272 Z" fill="#2a3350" />
            <circle cx="103" cy="274" r="6" fill="#ffe6a8" />
          </g>
          <g opacity="0.72">
            <ellipse cx="806" cy="486" rx="66" ry="18" fill="url(#pn-lamp)" />
            <path
              d="M 806,486 L 806,330 C 806,314 796,306 780,304"
              fill="none"
              stroke="#1a2138"
              stroke-width="6"
              stroke-linecap="round"
            />
            <ellipse cx="774" cy="304" rx="26" ry="21" fill="url(#pn-lamp)" />
            <path
              d="M 760,306 C 762,299 768,294 775,294 C 783,294 788,299 788,306 Z"
              fill="#2a3350"
            />
            <circle cx="774" cy="308" r="5" fill="#ffe6a8" />
          </g>

          <!-- 远处树木 -->
          <g opacity="0.85">
            <g transform="translate(34,470) scale(0.95)"><use href="#pn-tree" /></g>
            <g transform="translate(150,470) scale(0.8)"><use href="#pn-tree" /></g>
            <g transform="translate(842,470) scale(0.9)"><use href="#pn-tree" /></g>
          </g>

          <!-- 速度光轨（车后） -->
          <g class="pn-streaks" stroke-linecap="round" opacity="0.55">
            <line x1="18" y1="292" x2="150" y2="292" stroke="#38e4ff" stroke-width="5" />
            <line x1="6" y1="330" x2="112" y2="330" stroke="#2fb9e0" stroke-width="4" />
            <line x1="30" y1="368" x2="146" y2="368" stroke="#ff4fa3" stroke-width="3.4" />
            <line x1="52" y1="404" x2="128" y2="404" stroke="#38e4ff" stroke-width="3" />
            <line x1="66" y1="446" x2="150" y2="446" stroke="#2fb9e0" stroke-width="2.6" />
          </g>

          <!-- 地面影子 -->
          <ellipse cx="418" cy="512" rx="300" ry="20" fill="url(#pn-shadow)" />

          <!-- ===== 骑手 + 自行车（整体轻微起伏）===== -->
          <g class="pn-bob">
            <!-- 远侧腿 / 脚踏 -->
            <path
              d="M 356,262 C 366,300 372,352 368,398"
              fill="none"
              stroke="#b4761f"
              stroke-width="13"
              stroke-linecap="round"
            />
            <path
              d="M 344,392 C 362,388 380,394 386,404 C 372,412 352,412 340,404 Z"
              fill="#c98a2c"
              stroke="#9c6516"
              stroke-width="1.6"
              stroke-linejoin="round"
            />
            <path
              d="M 400,432 L 366,398"
              stroke="#212b3d"
              stroke-width="8"
              stroke-linecap="round"
            />
            <rect x="350" y="392" width="30" height="9" rx="4" fill="#1a2233" />

            <!-- 链轮 / 链条 -->
            <g stroke="#3d4a63" stroke-width="2.6" fill="none">
              <path d="M 400,409 L 208,424" />
              <path d="M 400,455 L 208,440" />
            </g>
            <circle cx="208" cy="432" r="11" fill="#2b3549" />

            <!-- 车轮（旋转） -->
            <g transform="translate(208,432)">
              <g class="pn-spin"><use href="#pn-wheel" /></g>
            </g>
            <g transform="translate(618,432)">
              <g class="pn-spin"><use href="#pn-wheel" /></g>
            </g>

            <!-- 车架 -->
            <g stroke="url(#pn-frame)" stroke-width="11" stroke-linecap="round" fill="none">
              <path d="M 208,432 L 400,432" />
              <path d="M 208,432 L 330,264" />
              <path d="M 400,432 L 330,264" />
              <path d="M 400,432 L 592,252" />
              <path d="M 330,264 L 588,250" />
              <path d="M 618,432 L 594,254" />
            </g>
            <path
              d="M 330,264 L 326,250"
              stroke="#1e93c8"
              stroke-width="8"
              stroke-linecap="round"
            />

            <!-- 牙盘 -->
            <circle cx="400" cy="432" r="24" fill="none" stroke="#33415c" stroke-width="5" />
            <circle
              cx="400"
              cy="432"
              r="28"
              fill="none"
              stroke="#4a5b7d"
              stroke-width="3.4"
              stroke-dasharray="3 6"
            />
            <circle cx="400" cy="432" r="6.5" fill="#3a4863" />

            <!-- 车座 -->
            <path
              d="M 292,258 C 312,242 356,240 380,249 C 384,262 366,270 342,270 C 318,270 296,268 292,258 Z"
              fill="#1d2536"
            />

            <!-- 车把 -->
            <path
              d="M 552,238 C 572,229 610,227 636,236"
              fill="none"
              stroke="#212b3d"
              stroke-width="9"
              stroke-linecap="round"
            />
            <rect x="542" y="232" width="16" height="12" rx="5" fill="#151c2b" />
            <rect x="632" y="230" width="16" height="12" rx="5" fill="#151c2b" />

            <!-- 尾羽 -->
            <path
              d="M 262,206 C 222,184 184,168 148,166 C 170,190 176,214 164,246 C 200,238 234,232 268,236 Z"
              fill="url(#pn-body)"
              stroke="#93a9c2"
              stroke-width="2.6"
              stroke-linejoin="round"
            />
            <g stroke="#93a9c2" stroke-width="1.8" fill="none" opacity="0.85">
              <path d="M 254,208 C 222,192 196,178 166,174" />
              <path d="M 258,224 C 230,216 202,212 174,212" />
            </g>

            <!-- 身体 -->
            <path
              d="M 402,186 C 356,172 296,182 256,210 C 232,226 228,252 244,270
                 C 266,294 320,306 372,300 C 418,294 450,266 454,226 C 457,202 434,190 402,186 Z"
              fill="url(#pn-body)"
              stroke="#93a9c2"
              stroke-width="2.6"
            />
            <!-- 月光冷边 -->
            <path
              d="M 256,210 C 296,182 356,172 402,186 C 434,190 457,202 454,226"
              fill="none"
              stroke="#7fe6ff"
              stroke-width="2.6"
              opacity="0.5"
              stroke-linecap="round"
            />
            <!-- 腹部暖反光 -->
            <path
              d="M 258,272 C 300,300 350,306 392,298"
              fill="none"
              stroke="#ffc978"
              stroke-width="2.4"
              opacity="0.32"
              stroke-linecap="round"
            />

            <!-- 脖子 -->
            <path
              d="M 406,192 C 420,150 452,122 490,116 C 506,113 518,120 520,134
                 C 522,148 512,158 498,158 C 470,158 448,182 440,216 Z"
              fill="url(#pn-body)"
              stroke="#93a9c2"
              stroke-width="2.6"
            />

            <!-- 头 -->
            <circle
              cx="520"
              cy="124"
              r="32"
              fill="url(#pn-body)"
              stroke="#93a9c2"
              stroke-width="2.6"
            />
            <path
              d="M 494,102 C 508,94 526,94 538,101"
              fill="none"
              stroke="#7fe6ff"
              stroke-width="2.4"
              opacity="0.45"
              stroke-linecap="round"
            />

            <!-- 喉囊 -->
            <path
              d="M 548,126 C 578,130 638,150 702,166 C 704,200 678,230 636,236
                 C 590,242 552,196 548,126 Z"
              fill="url(#pn-pouch)"
              stroke="#b8640c"
              stroke-width="2"
              stroke-linejoin="round"
            />
            <path
              d="M 566,146 C 588,178 610,204 636,220"
              fill="none"
              stroke="#fff0c2"
              stroke-width="2.6"
              opacity="0.45"
            />
            <path
              d="M 660,164 C 668,186 666,206 654,222"
              fill="none"
              stroke="#c9720f"
              stroke-width="1.8"
              opacity="0.5"
            />
            <!-- 喉囊里露出的小鱼尾 -->
            <path
              d="M 604,214 L 596,234 L 612,228 L 626,244 L 624,212 Z"
              fill="#7fe0f5"
              stroke="#3fb6d8"
              stroke-width="1.8"
              stroke-linejoin="round"
            />

            <!-- 上喙 -->
            <path
              d="M 540,104 C 600,110 668,126 716,142 C 722,144 721,152 713,151 C 664,142 600,132 542,124 Z"
              fill="url(#pn-beak)"
              stroke="#c4760c"
              stroke-width="1.8"
              stroke-linejoin="round"
            />
            <path
              d="M 556,111 C 612,119 668,132 706,144"
              fill="none"
              stroke="#ffeec0"
              stroke-width="2.6"
              opacity="0.7"
            />
            <path d="M 592,120 L 590,134" stroke="#c4760c" stroke-width="1.4" opacity="0.55" />
            <path d="M 646,131 L 645,144" stroke="#c4760c" stroke-width="1.4" opacity="0.45" />

            <!-- 眼睛 -->
            <circle cx="530" cy="116" r="8.4" fill="#ffffff" stroke="#93a9c2" stroke-width="1.4" />
            <circle cx="532" cy="116" r="4.2" fill="#141b28" />
            <circle cx="530" cy="114" r="1.7" fill="#ffffff" />
            <circle cx="535" cy="118" r="0.9" fill="#7fe6ff" />

            <!-- 翅膀（前缘搭向车把） -->
            <path
              d="M 398,188 C 442,192 500,206 566,226 C 588,233 598,240 600,246
                 C 588,252 572,252 556,248 C 496,234 438,226 398,224 C 378,222 380,190 398,188 Z"
              fill="url(#pn-wing)"
              stroke="#93a9c2"
              stroke-width="2.6"
            />
            <g stroke="#93a9c2" stroke-width="1.8" fill="none" opacity="0.85">
              <path d="M 436,204 C 480,214 526,228 566,242" />
              <path d="M 412,216 C 456,224 502,236 546,246" />
            </g>
            <path
              d="M 398,188 C 442,192 500,206 566,226"
              fill="none"
              stroke="#7fe6ff"
              stroke-width="2.4"
              opacity="0.42"
              stroke-linecap="round"
            />

            <!-- 近侧腿 / 脚踏 -->
            <path
              d="M 372,268 C 388,318 400,382 430,452"
              fill="none"
              stroke="#f59a33"
              stroke-width="14"
              stroke-linecap="round"
            />
            <path
              d="M 414,450 C 438,450 458,458 464,470 C 450,478 426,478 412,470 Z"
              fill="#f59a33"
              stroke="#d17d18"
              stroke-width="1.8"
              stroke-linejoin="round"
            />
            <path
              d="M 400,432 L 434,466"
              stroke="#212b3d"
              stroke-width="8"
              stroke-linecap="round"
            />
            <rect x="420" y="462" width="30" height="10" rx="4" fill="#1a2233" />
          </g>

          <!-- 车头灯光束（呼吸） -->
          <g class="pn-beam">
            <path d="M 650,250 L 880,266 L 880,404 Z" fill="url(#pn-beam)" />
            <ellipse cx="812" cy="500" rx="130" ry="30" fill="url(#pn-pool)" />
          </g>
          <ellipse
            cx="806"
            cy="470"
            rx="34"
            ry="14"
            fill="#fff3c8"
            opacity="0.16"
            filter="url(#pn-soft)"
          />

          <!-- 尾灯（脉动） -->
          <g class="pn-tail">
            <circle cx="198" cy="392" r="26" fill="#ff3b6b" opacity="0.16" filter="url(#pn-soft)" />
            <circle cx="198" cy="392" r="4.4" fill="#ff6b8f" />
          </g>

          <!-- 萤火 / 尘埃 -->
          <g fill="#ffe6a8">
            <circle cx="250" cy="360" r="2.2" opacity="0.7" />
            <circle cx="300" cy="330" r="1.6" opacity="0.5" />
            <circle cx="196" cy="300" r="1.8" opacity="0.45" />
            <circle cx="662" cy="352" r="2" opacity="0.6" />
            <circle cx="712" cy="300" r="1.5" opacity="0.4" />
            <circle cx="404" cy="122" r="1.7" opacity="0.45" />
          </g>

          <!-- 暗角 -->
          <rect
            x="0"
            y="0"
            width="860"
            height="580"
            fill="none"
            stroke="#000000"
            stroke-width="60"
            opacity="0.22"
          />
        </g>
      </svg>
    </section>

    <p class="night-page__note">
      说明：本页 SVG
      为手工绘制的内联矢量图（非位图、无外部依赖），任意缩放不失真；车轮旋转、公路虚线流动、星光闪烁、
      车灯呼吸与车身起伏由 CSS 关键帧驱动，并在系统开启「减少动态效果」时自动静止。
    </p>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/variables' as v;

.night-page {
  max-width: 900px;
  margin: 0 auto;
}

.night-page__eyebrow {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin: 0 0 10px;
  padding: 4px 10px;
  border-radius: 999px;
  background: #1b2138;
  color: #8fd8ff;
  font-size: 12.5px;
  font-weight: 600;
}

.night-page__title {
  margin: 0 0 8px;
  font-size: 28px;
  line-height: 1.2;
  color: v.$text-1;
}

.night-page__lede {
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
  background: #05070f;
  box-shadow: v.$shadow-card;
  line-height: 0;
}

.pelican-night {
  display: block;
  width: 100%;
  height: auto;
}

.night-page__note {
  margin: 14px 0 0;
  color: v.$text-3;
  font-size: 12.5px;
  line-height: 1.7;
}

// ---- 骑行动画 ----
// 车轮旋转：以自身包围盒中心为轴（fill-box 避免被 viewBox 中心带偏）
.pn-spin {
  transform-box: fill-box;
  transform-origin: center;
  animation: pn-spin 0.75s linear infinite;
}

// 车身轻微起伏（模拟路面颠簸）
.pn-bob {
  animation: pn-bob 1.9s ease-in-out infinite;
}

// 公路中央虚线流动
.pn-road-line {
  animation: pn-road 0.85s linear infinite;
}

// 速度光轨脉动
.pn-streaks {
  transform-origin: left center;
  animation: pn-streak 1.1s ease-in-out infinite alternate;
}

// 星光闪烁（三组错开）
.pn-tw-a {
  animation: pn-twinkle 3.2s ease-in-out infinite;
}

.pn-tw-b {
  animation: pn-twinkle 2.6s ease-in-out infinite 0.9s;
}

.pn-tw-c {
  animation: pn-twinkle 3.8s ease-in-out infinite 1.7s;
}

// 车头灯呼吸
.pn-beam {
  animation: pn-beam 3.4s ease-in-out infinite;
}

// 尾灯脉动
.pn-tail {
  animation: pn-tail 1.6s ease-in-out infinite;
}

@keyframes pn-spin {
  to {
    transform: rotate(360deg);
  }
}

@keyframes pn-bob {
  0%,
  100% {
    transform: translateY(0);
  }
  50% {
    transform: translateY(-2.6px);
  }
}

@keyframes pn-road {
  to {
    stroke-dashoffset: -92;
  }
}

@keyframes pn-streak {
  from {
    transform: translateX(0);
    opacity: 0.55;
  }
  to {
    transform: translateX(-18px);
    opacity: 0.28;
  }
}

@keyframes pn-twinkle {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.35;
  }
}

@keyframes pn-beam {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.62;
  }
}

@keyframes pn-tail {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.45;
  }
}

// 无障碍：系统开启「减少动态效果」时静止
@media (prefers-reduced-motion: reduce) {
  .pn-spin,
  .pn-bob,
  .pn-road-line,
  .pn-streaks,
  .pn-tw-a,
  .pn-tw-b,
  .pn-tw-c,
  .pn-beam,
  .pn-tail {
    animation: none;
  }
}

@media (max-width: 640px) {
  .night-page__title {
    font-size: 22px;
  }
}
</style>
