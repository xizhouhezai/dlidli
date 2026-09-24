<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

/**
 * 鹈鹕骑行（原创 SVG 2D 动画）。
 *
 * 只有本页，不依赖任何既有页面/组件/样式变量，颜色与几何全部内联自持。
 *
 * 动画由两层驱动：
 *  ① 物理相关部分走 rAF（车轮转动、曲柄、双腿踩踏、车身起伏）——腿部用**双骨骼 IK**
 *     反解：髋关节与脚踏位置已知，反求膝关节。这样脚是真踩在圆周上的，比晃动更像骑行；
 *  ② 环境氛围走 CSS 关键帧（云飘、海面微光、栈道木纹滚动、远景飞鸟、前景芦苇）。
 */

interface Pt {
  x: number
  y: number
}

// ---------------- 几何常量（viewBox 坐标系） ----------------

const VIEW_W = 960
const VIEW_H = 560

/** 后轮心 / 前轮心 / 轮半径 */
const RW: Pt = { x: 296, y: 404 }
const FW: Pt = { x: 616, y: 404 }
const WHEEL_R = 70

/** 中轴（曲柄圆心）与曲柄长度 */
const BB: Pt = { x: 462, y: 404 }
const CRANK_R = 27

/** 髋关节与腿长（大腿 / 小腿）。取值保证脚踏走满圆周时都够得着（见 solveLeg 的钳制） */
const HIP_NEAR: Pt = { x: 404, y: 316 }
const HIP_FAR: Pt = { x: 390, y: 312 }
const L_THIGH = 74
const L_SHIN = 82

/** 传动比：车轮转角 = 曲柄转角 × 该值 */
const GEAR = 2.6

/**
 * 车轮辐条（按轮心生成，预计算避免每帧重算）。
 * 必须每个轮子各生成一组——前后轮心相距 320px，复用同一组会把辐条画到轮圈外
 * （表现为天上多出一团放射状星芒，且随车轮旋转）。
 */
function makeSpokes(hub: Pt) {
  const r = WHEEL_R - 12
  return Array.from({ length: 12 }, (_, i) => {
    const a = (i * Math.PI) / 6
    const dx = Math.cos(a) * r
    const dy = Math.sin(a) * r
    return { x1: hub.x + dx, y1: hub.y + dy, x2: hub.x - dx, y2: hub.y - dy }
  })
}

const SPOKES_REAR = makeSpokes(RW)
const SPOKES_FRONT = makeSpokes(FW)

/** 栈道木纹：一条缝隙每 46px，整组循环平移一格即无缝 */
const PLANK_GAP = 46
const PLANKS = Array.from({ length: 24 }, (_, i) => -PLANK_GAP + i * PLANK_GAP)

// ---------------- 状态 ----------------

const rpm = ref(56)
const paused = ref(false)

const wheelDeg = ref(0)
const crankDeg = ref(0)
const bobY = ref(0)
const tilt = ref(0)
const legNear = ref({ knee: { x: 0, y: 0 }, foot: { x: 0, y: 0 } })
const legFar = ref({ knee: { x: 0, y: 0 }, foot: { x: 0, y: 0 } })

// ---------------- 双骨骼 IK ----------------

/**
 * 已知髋关节与脚踏（脚）位置，反解膝关节。
 * 标准两段式 IK：先算髋→脚连线方向，再用余弦定理求髋角偏移，膝关节朝行进方向弯。
 * 超出可达范围时把脚钳到可达圆上（避免肢体被拉断变形）。
 */
function solveLeg(hip: Pt, foot: Pt, l1: number, l2: number) {
  const dx = foot.x - hip.x
  const dy = foot.y - hip.y
  const dist = Math.hypot(dx, dy) || 0.0001
  const reachMax = (l1 + l2) * 0.995
  const reachMin = Math.abs(l1 - l2) + 0.5
  const d = Math.min(reachMax, Math.max(reachMin, dist))

  const ux = dx / dist
  const uy = dy / dist
  const fx = hip.x + ux * d
  const fy = hip.y + uy * d

  const base = Math.atan2(fy - hip.y, fx - hip.x)
  const cosA = (l1 * l1 + d * d - l2 * l2) / (2 * l1 * d)
  const a = Math.acos(Math.min(1, Math.max(-1, cosA)))
  // 负号 = 膝关节前顶（骑行姿态：膝朝车头方向弯）
  const kneeAngle = base - a

  return {
    knee: { x: hip.x + l1 * Math.cos(kneeAngle), y: hip.y + l1 * Math.sin(kneeAngle) },
    foot: { x: fx, y: fy },
  }
}

// ---------------- 动画循环 ----------------

let raf = 0
let lastTs = 0
let crank = 0

function tick(ts: number) {
  if (!lastTs) lastTs = ts
  const dt = Math.min(0.05, (ts - lastTs) / 1000)
  lastTs = ts

  if (!paused.value) {
    // 踏频 rpm → 度/秒
    crank = (crank + rpm.value * 6 * dt) % 360
  }
  crankDeg.value = crank
  wheelDeg.value = crank * GEAR

  const rad = (crank * Math.PI) / 180
  // 左右脚踏相位差 180°
  const footNear: Pt = { x: BB.x + CRANK_R * Math.cos(rad), y: BB.y + CRANK_R * Math.sin(rad) }
  const footFar: Pt = { x: BB.x - CRANK_R * Math.cos(rad), y: BB.y - CRANK_R * Math.sin(rad) }

  legNear.value = solveLeg(HIP_NEAR, footNear, L_THIGH, L_SHIN)
  legFar.value = solveLeg(HIP_FAR, footFar, L_THIGH, L_SHIN)

  // 车身起伏：每圈两次（左右各蹬一下），并带极轻微俯仰
  const phase = rad * 2
  bobY.value = Math.sin(phase) * 2.4
  tilt.value = Math.sin(phase) * 0.55

  raf = requestAnimationFrame(tick)
}

onMounted(() => {
  raf = requestAnimationFrame(tick)
})

onBeforeUnmount(() => {
  cancelAnimationFrame(raf)
})

// ---------------- 模板用的派生值 ----------------

const rigTransform = computed(
  () => `translate(0 ${bobY.value.toFixed(2)}) rotate(${tilt.value.toFixed(2)} 456 404)`,
)
const wheelTransform = computed(() => `rotate(${wheelDeg.value.toFixed(2)} ${RW.x} ${RW.y})`)
const wheelFrontTransform = computed(() => `rotate(${wheelDeg.value.toFixed(2)} ${FW.x} ${FW.y})`)
const crankTransform = computed(() => `rotate(${crankDeg.value.toFixed(2)} ${BB.x} ${BB.y})`)

const legNearD = computed(() => {
  const l = legNear.value
  return `M ${HIP_NEAR.x} ${HIP_NEAR.y} L ${l.knee.x.toFixed(2)} ${l.knee.y.toFixed(2)} L ${l.foot.x.toFixed(2)} ${l.foot.y.toFixed(2)}`
})
const legFarD = computed(() => {
  const l = legFar.value
  return `M ${HIP_FAR.x} ${HIP_FAR.y} L ${l.knee.x.toFixed(2)} ${l.knee.y.toFixed(2)} L ${l.foot.x.toFixed(2)} ${l.foot.y.toFixed(2)}`
})

/** 脚踏（含蹼足）跟着曲柄走 */
const pedalNear = computed(() => {
  const p = legNear.value.foot
  return `translate(${p.x.toFixed(2)} ${p.y.toFixed(2)})`
})
const pedalFar = computed(() => {
  const p = legFar.value.foot
  return `translate(${p.x.toFixed(2)} ${p.y.toFixed(2)})`
})
</script>

<template>
  <div class="pelican-ride">
    <div class="stage">
      <svg :viewBox="`0 0 ${VIEW_W} ${VIEW_H}`" role="img" aria-label="鹈鹕骑自行车的 2D 动画">
        <defs>
          <linearGradient id="pr-sky" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stop-color="#FFE3BC" />
            <stop offset="34%" stop-color="#FBD9C0" />
            <stop offset="62%" stop-color="#BFE0F0" />
            <stop offset="100%" stop-color="#8FCEE6" />
          </linearGradient>
          <radialGradient id="pr-sun-glow" cx="50%" cy="50%" r="50%">
            <stop offset="0%" stop-color="#FFD98E" stop-opacity="0.95" />
            <stop offset="45%" stop-color="#FFC98A" stop-opacity="0.4" />
            <stop offset="100%" stop-color="#FFC98A" stop-opacity="0" />
          </radialGradient>
          <linearGradient id="pr-sea" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stop-color="#7EC4DC" />
            <stop offset="100%" stop-color="#4E9CBE" />
          </linearGradient>
          <linearGradient id="pr-walk" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stop-color="#EFDCBB" />
            <stop offset="100%" stop-color="#D8BE94" />
          </linearGradient>
          <linearGradient id="pr-bill" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stop-color="#FFC24D" />
            <stop offset="100%" stop-color="#EE8A11" />
          </linearGradient>
          <linearGradient id="pr-pouch" x1="0" y1="0" x2="0.3" y2="1">
            <stop offset="0%" stop-color="#FFD37F" />
            <stop offset="100%" stop-color="#F2A02A" />
          </linearGradient>
          <linearGradient id="pr-body" x1="0.15" y1="0" x2="0.7" y2="1">
            <stop offset="0%" stop-color="#FFFFFF" />
            <stop offset="62%" stop-color="#F7F2E9" />
            <stop offset="100%" stop-color="#DFE6EC" />
          </linearGradient>
          <radialGradient id="pr-shadow" cx="50%" cy="50%" r="50%">
            <stop offset="0%" stop-color="#3A2B1A" stop-opacity="0.32" />
            <stop offset="100%" stop-color="#3A2B1A" stop-opacity="0" />
          </radialGradient>
        </defs>

        <!-- ============ 天空 ============ -->
        <rect x="0" y="0" :width="VIEW_W" :height="VIEW_H" fill="url(#pr-sky)" />

        <!-- 太阳与光晕 -->
        <circle cx="742" cy="126" r="128" fill="url(#pr-sun-glow)" />
        <circle cx="742" cy="126" r="46" fill="#FFE9AE" />
        <circle cx="742" cy="126" r="46" fill="#FFF6DC" opacity="0.55" />

        <!-- 云（不同速度漂移） -->
        <g class="pr-cloud pr-cloud--a" opacity="0.85">
          <path
            d="M 0 96 C 14 74, 44 70, 60 86 C 74 68, 108 66, 122 84 C 142 78, 162 90, 164 104 L 0 104 Z"
            fill="#FFFFFF"
          />
        </g>
        <g class="pr-cloud pr-cloud--b" opacity="0.7">
          <path
            d="M 0 168 C 16 146, 46 142, 64 158 C 82 138, 120 140, 134 160 C 152 154, 174 166, 176 180 L 0 180 Z"
            fill="#FFFFFF"
          />
        </g>
        <g class="pr-cloud pr-cloud--c" opacity="0.55">
          <path d="M 0 60 C 12 44, 34 42, 46 54 C 60 40, 88 42, 98 58 L 0 66 Z" fill="#FFFFFF" />
        </g>

        <!-- 远山 -->
        <path
          d="M 0 300 L 92 246 L 154 286 L 236 216 L 318 288 L 380 256 L 452 300 Z"
          fill="#A9C6BB"
          opacity="0.9"
        />
        <path
          d="M 400 300 L 486 232 L 552 276 L 640 208 L 726 268 L 806 226 L 900 300 Z"
          fill="#93B7AB"
          opacity="0.85"
        />

        <!-- ============ 海面 ============ -->
        <rect x="0" y="300" :width="VIEW_W" height="96" fill="url(#pr-sea)" />
        <g
          class="pr-shimmer"
          stroke="#FFFFFF"
          stroke-width="2.4"
          stroke-linecap="round"
          opacity="0.55"
        >
          <line x1="72" y1="322" x2="132" y2="322" />
          <line x1="212" y1="340" x2="292" y2="340" />
          <line x1="404" y1="318" x2="452" y2="318" />
          <line x1="522" y1="346" x2="606" y2="346" />
          <line x1="668" y1="326" x2="736" y2="326" />
          <line x1="828" y1="352" x2="902" y2="352" />
          <line x1="140" y1="366" x2="228" y2="366" />
          <line x1="452" y1="372" x2="520" y2="372" />
        </g>
        <!-- 海面反光带 -->
        <ellipse cx="742" cy="336" rx="58" ry="10" fill="#FFE9AE" opacity="0.4" />

        <!-- 远处飞鸟 -->
        <g class="pr-birds" stroke="#5C7C86" stroke-width="2.4" fill="none" stroke-linecap="round">
          <path d="M 176 152 q 9 -8 18 0 q 9 -8 18 0" />
          <path d="M 232 128 q 7 -6 14 0 q 7 -6 14 0" />
          <path d="M 300 166 q 6 -5 12 0 q 6 -5 12 0" />
        </g>

        <!-- ============ 栈道 ============ -->
        <rect x="0" y="392" :width="VIEW_W" height="72" fill="url(#pr-walk)" />
        <rect x="0" y="392" :width="VIEW_W" height="5" fill="#C7A87C" opacity="0.7" />
        <g class="pr-planks" stroke="#C3A578" stroke-width="2.6" opacity="0.85">
          <line v-for="x in PLANKS" :key="`plank-${x}`" :x1="x" y1="397" :x2="x" y2="462" />
        </g>
        <rect x="0" y="464" :width="VIEW_W" height="96" fill="#C9AE83" />
        <rect x="0" y="464" :width="VIEW_W" height="4" fill="#B09468" opacity="0.6" />

        <!-- 车影 -->
        <ellipse cx="456" cy="474" rx="186" ry="15" fill="url(#pr-shadow)" />

        <!-- ============ 车与鹈鹕（整体随踩踏起伏） ============ -->
        <g :transform="rigTransform">
          <!-- ---- 后轮 ---- -->
          <g>
            <circle
              :cx="RW.x"
              :cy="RW.y"
              :r="WHEEL_R"
              fill="none"
              stroke="#26343C"
              stroke-width="9"
            />
            <circle
              :cx="RW.x"
              :cy="RW.y"
              :r="WHEEL_R - 6"
              fill="none"
              stroke="#A9BECD"
              stroke-width="3.4"
            />
            <g :transform="wheelTransform" stroke="#C6D6E2" stroke-width="2" stroke-linecap="round">
              <line
                v-for="(s, i) in SPOKES_REAR"
                :key="`rs-${i}`"
                :x1="s.x1"
                :y1="s.y1"
                :x2="s.x2"
                :y2="s.y2"
              />
            </g>
            <circle :cx="RW.x" :cy="RW.y" r="7.5" fill="#5E7280" />
          </g>

          <!-- ---- 前轮 ---- -->
          <g>
            <circle
              :cx="FW.x"
              :cy="FW.y"
              :r="WHEEL_R"
              fill="none"
              stroke="#26343C"
              stroke-width="9"
            />
            <circle
              :cx="FW.x"
              :cy="FW.y"
              :r="WHEEL_R - 6"
              fill="none"
              stroke="#A9BECD"
              stroke-width="3.4"
            />
            <g
              :transform="wheelFrontTransform"
              stroke="#C6D6E2"
              stroke-width="2"
              stroke-linecap="round"
            >
              <line
                v-for="(s, i) in SPOKES_FRONT"
                :key="`fs-${i}`"
                :x1="s.x1"
                :y1="s.y1"
                :x2="s.x2"
                :y2="s.y2"
              />
            </g>
            <circle :cx="FW.x" :cy="FW.y" r="7.5" fill="#5E7280" />
          </g>

          <!-- ---- 车架 ---- -->
          <g stroke="#1F4E5F" stroke-width="8" stroke-linecap="round" fill="none">
            <!-- 下后叉：中轴 → 后轮心 -->
            <line :x1="BB.x" :y1="BB.y" :x2="RW.x" :y2="RW.y" />
            <!-- 上后叉：座管口 → 后轮心 -->
            <line x1="382" y1="298" :x2="RW.x" :y2="RW.y" />
            <!-- 座管：中轴 → 座垫 -->
            <line :x1="BB.x" :y1="BB.y" x2="382" y2="298" />
            <!-- 上管：座管口 → 头管 -->
            <line x1="382" y1="298" x2="596" y2="286" />
            <!-- 下管：中轴 → 头管 -->
            <line :x1="BB.x" :y1="BB.y" x2="596" y2="286" />
            <!-- 前叉：头管 → 前轮心 -->
            <line x1="596" y1="286" :x2="FW.x" :y2="FW.y" />
          </g>
          <!-- 头管与座管口点缀 -->
          <line
            x1="592"
            y1="276"
            x2="600"
            y2="296"
            stroke="#163B49"
            stroke-width="11"
            stroke-linecap="round"
          />
          <!-- 车把 -->
          <path
            d="M 596 288 C 608 280, 626 278, 636 284"
            stroke="#163B49"
            stroke-width="9"
            fill="none"
            stroke-linecap="round"
          />
          <circle cx="638" cy="285" r="6" fill="#E8503A" />
          <!-- 座垫 -->
          <path
            d="M 356 296 C 368 288, 396 288, 404 296 C 398 304, 366 306, 356 296 Z"
            fill="#2A3F4A"
          />

          <!-- ---- 链条（后轮心 ↔ 中轴） ---- -->
          <g stroke="#7E8C93" stroke-width="3" stroke-dasharray="7 5" class="pr-chain">
            <line :x1="RW.x" :y1="RW.y - 9" :x2="BB.x" :y2="BB.y - 17" />
            <line :x1="RW.x" :y1="RW.y + 9" :x2="BB.x" :y2="BB.y + 17" />
          </g>

          <!-- ---- 远侧腿（先画，被车身遮住一部分） ---- -->
          <g
            stroke="#C4CBD2"
            stroke-width="17"
            stroke-linecap="round"
            stroke-linejoin="round"
            fill="none"
          >
            <path :d="legFarD" />
          </g>
          <g :transform="pedalFar">
            <path d="M -11 0 C -4 -6, 8 -6, 15 0 C 8 7, -4 7, -11 0 Z" fill="#D9A05B" />
          </g>

          <!-- ---- 曲柄与牙盘 ---- -->
          <g :transform="crankTransform">
            <circle :cx="BB.x" :cy="BB.y" r="21" fill="none" stroke="#B8C4CC" stroke-width="4" />
            <circle :cx="BB.x" :cy="BB.y" r="12" fill="#8FA0AA" />
            <line
              :x1="BB.x"
              :y1="BB.y"
              :x2="BB.x + CRANK_R"
              :y2="BB.y"
              stroke="#9AA8B1"
              stroke-width="7"
              stroke-linecap="round"
            />
            <line
              :x1="BB.x"
              :y1="BB.y"
              :x2="BB.x - CRANK_R"
              :y2="BB.y"
              stroke="#7E8C93"
              stroke-width="7"
              stroke-linecap="round"
            />
          </g>

          <!-- ---- 鹈鹕身体 ---- -->
          <g>
            <!-- 尾羽 -->
            <path
              d="M 312 262 C 284 250, 260 240, 240 232 C 250 252, 252 262, 244 278 C 264 282, 288 294, 306 306 C 300 288, 302 274, 312 262 Z"
              fill="#E7EDF2"
              stroke="#2F4858"
              stroke-width="3"
              stroke-linejoin="round"
            />
            <!-- 躯干 -->
            <path
              d="M 292 266
                 C 320 234, 376 214, 432 220
                 C 448 222, 456 236, 448 250
                 C 462 280, 452 316, 416 330
                 C 384 342, 338 336, 312 314
                 C 294 298, 284 282, 292 266 Z"
              fill="url(#pr-body)"
              stroke="#2F4858"
              stroke-width="3.4"
              stroke-linejoin="round"
            />
            <!-- 腹部阴影 -->
            <path
              d="M 330 320 C 356 334, 392 334, 420 322 C 404 338, 356 340, 330 320 Z"
              fill="#CAD4DC"
              opacity="0.75"
            />

            <!-- 近侧翅膀（搭在车把上） -->
            <path
              d="M 428 238
                 C 474 248, 544 264, 614 282
                 L 636 290
                 C 642 298, 636 308, 626 306
                 L 598 298
                 C 528 286, 464 276, 420 270
                 C 410 258, 414 244, 428 238 Z"
              fill="#F1F5F8"
              stroke="#2F4858"
              stroke-width="3.2"
              stroke-linejoin="round"
            />
            <!-- 翼羽分缝 -->
            <g stroke="#C3CDD6" stroke-width="2" fill="none" stroke-linecap="round">
              <path d="M 470 252 C 500 260, 546 271, 590 283" />
              <path d="M 452 262 C 486 270, 532 280, 578 291" />
            </g>

            <!-- 颈 -->
            <path
              d="M 438 242 C 460 206, 484 188, 518 182"
              stroke="#FBF7F0"
              stroke-width="32"
              fill="none"
              stroke-linecap="round"
            />
            <path
              d="M 442 246 C 462 214, 486 198, 516 192"
              stroke="#E3EAF0"
              stroke-width="9"
              fill="none"
              stroke-linecap="round"
              opacity="0.8"
            />

            <!-- 头 -->
            <circle cx="524" cy="178" r="27" fill="#FDFBF7" stroke="#2F4858" stroke-width="3.2" />
            <!-- 头顶浅灰 -->
            <path
              d="M 500 166 C 510 154, 538 152, 548 164 C 534 158, 512 159, 500 166 Z"
              fill="#E3EAF0"
            />

            <!-- 喙：上颚 -->
            <path
              d="M 543 168 L 650 176 C 658 177, 660 182, 656 185 L 545 187 Z"
              fill="url(#pr-bill)"
              stroke="#B96C0B"
              stroke-width="2.4"
              stroke-linejoin="round"
            />
            <!-- 喙：下颚 + 喉囊 -->
            <path
              d="M 545 186
                 C 557 224, 592 244, 626 216
                 C 642 202, 652 192, 656 185
                 C 630 192, 600 196, 573 196
                 C 559 196, 549 192, 545 186 Z"
              fill="url(#pr-pouch)"
              stroke="#C1740C"
              stroke-width="2.4"
              stroke-linejoin="round"
            />
            <!-- 喉囊褶皱 -->
            <g stroke="#D98A18" stroke-width="1.8" fill="none" opacity="0.75">
              <path d="M 566 194 C 574 210, 586 220, 600 220" />
              <path d="M 592 196 C 600 206, 610 211, 620 209" />
            </g>
            <!-- 喙尖钩 -->
            <path
              d="M 650 176 C 658 178, 660 184, 655 187"
              stroke="#B96C0B"
              stroke-width="2.4"
              fill="none"
            />

            <!-- 眼 -->
            <circle cx="524" cy="170" r="4.6" fill="#22303A" />
            <circle cx="525.6" cy="168.4" r="1.5" fill="#FFFFFF" />
          </g>

          <!-- ---- 近侧腿（画在身体之上） ---- -->
          <g
            stroke="#E4E9EE"
            stroke-width="18"
            stroke-linecap="round"
            stroke-linejoin="round"
            fill="none"
          >
            <path :d="legNearD" />
          </g>
          <g stroke="#2F4858" stroke-width="1.6" fill="none" opacity="0.35">
            <path :d="legNearD" />
          </g>
          <g :transform="pedalNear">
            <path
              d="M -12 0 C -5 -7, 9 -7, 16 0 C 9 8, -5 8, -12 0 Z"
              fill="#E8A94E"
              stroke="#2F4858"
              stroke-width="1.8"
            />
          </g>
          <!-- 脚踏板 -->
          <g :transform="pedalNear">
            <rect x="-13" y="-3" width="30" height="6" rx="3" fill="#3A4A54" />
          </g>
          <g :transform="pedalFar">
            <rect x="-13" y="-3" width="30" height="6" rx="3" fill="#5A6A74" />
          </g>
        </g>

        <!-- ============ 前景芦苇（快速掠过） ============ -->
        <g class="pr-reeds" stroke="#5E8C61" stroke-width="4" fill="none" stroke-linecap="round">
          <path d="M 40 560 C 46 520, 34 496, 44 470" />
          <path d="M 300 560 C 308 522, 296 500, 306 476" />
          <path d="M 620 560 C 628 524, 616 502, 626 478" />
          <path d="M 880 560 C 888 526, 876 504, 886 480" />
        </g>
      </svg>

      <!-- 控制条 -->
      <div class="panel">
        <button class="panel__btn" type="button" @click="paused = !paused">
          {{ paused ? '继续' : '暂停' }}
        </button>
        <label class="panel__range">
          <span>踏频</span>
          <input v-model.number="rpm" type="range" min="20" max="110" step="1" />
          <b>{{ rpm }}</b>
        </label>
      </div>
    </div>
  </div>
</template>

<style scoped>
.pelican-ride {
  display: flex;
  justify-content: center;
  padding: 20px 16px 32px;
}

.stage {
  position: relative;
  width: 100%;
  max-width: 960px;
}

.stage svg {
  display: block;
  width: 100%;
  height: auto;
  border-radius: 16px;
  box-shadow: 0 18px 44px rgba(31, 78, 95, 0.18);
}

/* ---------- 云 ---------- */
.pr-cloud {
  will-change: transform;
}
.pr-cloud--a {
  animation: pr-drift-a 46s linear infinite;
}
.pr-cloud--b {
  animation: pr-drift-b 68s linear infinite;
}
.pr-cloud--c {
  animation: pr-drift-c 34s linear infinite;
}
@keyframes pr-drift-a {
  from {
    transform: translateX(-220px);
  }
  to {
    transform: translateX(1000px);
  }
}
@keyframes pr-drift-b {
  from {
    transform: translateX(-280px);
  }
  to {
    transform: translateX(1040px);
  }
}
@keyframes pr-drift-c {
  from {
    transform: translateX(-160px);
  }
  to {
    transform: translateX(1020px);
  }
}

/* ---------- 海面微光 ---------- */
.pr-shimmer {
  animation: pr-shimmer 3.4s ease-in-out infinite;
}
@keyframes pr-shimmer {
  0%,
  100% {
    opacity: 0.3;
  }
  50% {
    opacity: 0.7;
  }
}

/* ---------- 飞鸟 ---------- */
.pr-birds {
  animation: pr-birds 18s linear infinite;
}
@keyframes pr-birds {
  from {
    transform: translate(140px, 26px);
    opacity: 0;
  }
  15% {
    opacity: 0.9;
  }
  85% {
    opacity: 0.9;
  }
  to {
    transform: translate(-240px, -14px);
    opacity: 0;
  }
}

/* ---------- 栈道木纹滚动（恰好一格，无缝） ---------- */
.pr-planks {
  animation: pr-planks 0.62s linear infinite;
  will-change: transform;
}
@keyframes pr-planks {
  from {
    transform: translateX(0);
  }
  to {
    transform: translateX(-46px);
  }
}

/* ---------- 链条 ---------- */
.pr-chain {
  animation: pr-chain 0.4s linear infinite;
}
@keyframes pr-chain {
  from {
    stroke-dashoffset: 0;
  }
  to {
    stroke-dashoffset: -24;
  }
}

/* ---------- 前景芦苇 ---------- */
.pr-reeds {
  animation: pr-reeds 1.1s linear infinite;
  opacity: 0.9;
}
@keyframes pr-reeds {
  from {
    transform: translateX(0);
  }
  to {
    transform: translateX(-220px);
  }
}

/* ---------- 控制条 ---------- */
.panel {
  position: absolute;
  left: 16px;
  bottom: 16px;
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 10px 14px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.82);
  border: 1px solid rgba(31, 78, 95, 0.14);
  backdrop-filter: blur(8px);
  box-shadow: 0 8px 22px rgba(31, 78, 95, 0.16);
}

.panel__btn {
  min-width: 58px;
  padding: 6px 14px;
  border: none;
  border-radius: 999px;
  background: #1f4e5f;
  color: #fff;
  font-size: 13px;
  cursor: pointer;
  transition: background 0.18s ease;
}
.panel__btn:hover {
  background: #2c6a80;
}

.panel__range {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 13px;
  color: #1f4e5f;
}
.panel__range input {
  width: 132px;
  accent-color: #1f4e5f;
  cursor: pointer;
}
.panel__range b {
  min-width: 26px;
  text-align: right;
  font-variant-numeric: tabular-nums;
}

/* 尊重「减少动态效果」偏好：关掉环境动画，仅保留可交互的踩踏 */
@media (prefers-reduced-motion: reduce) {
  .pr-cloud,
  .pr-shimmer,
  .pr-birds,
  .pr-planks,
  .pr-chain,
  .pr-reeds {
    animation: none;
  }
}
</style>
