<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import * as echarts from 'echarts'
import type { DashboardStats } from '@dlidli/api-client'
import { readAdminInfo } from '@/utils/token'
import { adminApi } from '@/api'

const router = useRouter()
const adminInfo = readAdminInfo()

const stats = ref([
  { key: 'review', label: '待审稿件', value: '—', icon: 'i-mingcute-task-2-line', color: '#fb7299', to: '/review' },
  { key: 'users', label: '注册用户', value: '—', icon: 'i-mingcute-user-3-line', color: '#3b82f6', to: '/users' },
  { key: 'banned', label: '封禁用户', value: '—', icon: 'i-mingcute-forbid-circle-line', color: '#f59e0b', to: '/users' },
  { key: 'words', label: '敏感词', value: '—', icon: 'i-mingcute-shield-line', color: '#10b981', to: '/sensitive-words' },
])

// 数据大盘（M3-OPS-02）
const dash = ref<DashboardStats | null>(null)
const trendChartEl = ref<HTMLDivElement>()
let trendChart: echarts.ECharts | null = null

const shortcuts = [
  { title: '审核工作台', desc: '处理待审稿件', icon: 'i-mingcute-task-2-line', to: '/review' },
  { title: '用户管理', desc: '查询与处罚用户', icon: 'i-mingcute-user-3-line', to: '/users' },
  { title: '敏感词库', desc: '维护屏蔽词', icon: 'i-mingcute-shield-line', to: '/sensitive-words' },
]

async function loadStats() {
  try {
    const [review, users, banned, words] = await Promise.all([
      adminApi.admin.reviewList(1, 1),
      adminApi.admin.users({ page: 1, page_size: 1 }),
      adminApi.admin.users({ status: 2, page: 1, page_size: 1 }),
      adminApi.admin.sensitiveWords(),
    ])
    stats.value[0].value = String(review.total)
    stats.value[1].value = String(users.total)
    stats.value[2].value = String(banned.total)
    stats.value[3].value = String(words.list.length)
  } catch {
    // 统计失败不阻塞页面
  }
}

async function loadDashboard() {
  try {
    dash.value = await adminApi.admin.dashboardStats()
    await nextTick()
    renderTrend()
  } catch {
    // 大盘失败不阻塞页面
  }
}

function renderTrend() {
  const el = trendChartEl.value
  if (!el || !dash.value) return
  if (!trendChart) trendChart = echarts.init(el)
  const t = dash.value.trend
  trendChart.setOption({
    tooltip: { trigger: 'axis' },
    legend: { data: ['活跃用户', '新增用户', '投稿', '播放'], bottom: 0, textStyle: { color: '#9499a0', fontSize: 11 } },
    grid: { left: 44, right: 16, top: 24, bottom: 36 },
    xAxis: {
      type: 'category',
      data: t.map((p) => p.date),
      axisLine: { lineStyle: { color: '#e3e5e7' } },
      axisTick: { show: false },
      axisLabel: { color: '#9499a0', fontSize: 11 },
    },
    yAxis: {
      type: 'value',
      minInterval: 1,
      splitLine: { lineStyle: { color: '#f1f2f3' } },
      axisLabel: { color: '#9499a0', fontSize: 11 },
    },
    series: [
      { name: '活跃用户', type: 'line', smooth: true, data: t.map((p) => p.dau), itemStyle: { color: '#fb7299' } },
      { name: '新增用户', type: 'line', smooth: true, data: t.map((p) => p.new_users), itemStyle: { color: '#3b82f6' } },
      { name: '投稿', type: 'line', smooth: true, data: t.map((p) => p.uploads), itemStyle: { color: '#10b981' } },
      { name: '播放', type: 'line', smooth: true, data: t.map((p) => p.views), itemStyle: { color: '#f59e0b' } },
    ],
  })
}

function onResize() {
  trendChart?.resize()
}

// 大盘刷新（5 分钟轮询保持实时）
let timer: ReturnType<typeof setInterval> | null = null
onMounted(() => {
  loadStats()
  loadDashboard()
  timer = setInterval(loadDashboard, 5 * 60 * 1000)
  window.addEventListener('resize', onResize)
})

onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
  window.removeEventListener('resize', onResize)
  trendChart?.dispose()
  trendChart = null
})

// 大盘数据变化时重绘
watch(dash, () => nextTick(renderTrend))

function go(to: string) {
  router.push(to)
}
</script>

<template>
  <div>
    <!-- 欢迎条 -->
    <div class="welcome">
      <div>
        <h2 class="welcome__title">
          你好，{{ adminInfo?.username ?? '管理员' }} 👋
        </h2>
        <p class="welcome__sub">
          欢迎回到 DliDli 管理后台，祝你今天工作愉快。
        </p>
      </div>
      <span class="i-mingcute-tv-2-line welcome__icon" />
    </div>

    <!-- 统计卡 -->
    <div class="stat-grid">
      <div
        v-for="s in stats"
        :key="s.key"
        class="stat-card"
        @click="go(s.to)"
      >
        <div
          class="stat-card__icon"
          :style="{ background: s.color + '1a', color: s.color }"
        >
          <span :class="s.icon" />
        </div>
        <div>
          <div class="stat-card__value">
            {{ s.value }}
          </div>
          <div class="stat-card__label">
            {{ s.label }}
          </div>
        </div>
      </div>
    </div>

    <!-- 数据大盘（M3-OPS-02） -->
    <div class="panel dashboard-panel">
      <div class="panel__title">
        数据大盘
        <span class="panel__sub">今日实时 + 近 7 日趋势（5 分钟自动刷新）</span>
      </div>

      <!-- 今日实时指标 -->
      <div class="dash-grid">
        <div class="dash-card dash-card--pink">
          <div class="dash-card__value">
            {{ dash?.today.dau ?? '—' }}
          </div>
          <div class="dash-card__label">
            今日活跃用户
          </div>
        </div>
        <div class="dash-card dash-card--blue">
          <div class="dash-card__value">
            {{ dash?.today.new_users ?? '—' }}
          </div>
          <div class="dash-card__label">
            今日新增用户
          </div>
        </div>
        <div class="dash-card dash-card--green">
          <div class="dash-card__value">
            {{ dash?.today.uploads ?? '—' }}
          </div>
          <div class="dash-card__label">
            今日投稿
          </div>
        </div>
        <div class="dash-card dash-card--orange">
          <div class="dash-card__value">
            {{ dash?.today.views ?? '—' }}
          </div>
          <div class="dash-card__label">
            今日有效播放
          </div>
        </div>
      </div>

      <!-- 近 7 日趋势 -->
      <div class="dash-chart">
        <div class="dash-chart__title">
          近 7 日趋势
        </div>
        <div
          ref="trendChartEl"
          class="dash-chart__body"
        />
      </div>

      <!-- 审核时效 -->
      <div class="dash-extra">
        <div class="dash-extra__item">
          <span class="dash-extra__value">
            {{ dash?.review_hours ? dash.review_hours.toFixed(1) : '—' }}h
          </span>
          <span class="dash-extra__label">近 7 日平均审核时长</span>
        </div>
        <div class="dash-extra__item">
          <span class="dash-extra__value">
            {{ dash?.pending_review ?? '—' }}
          </span>
          <span class="dash-extra__label">当前待审稿件</span>
        </div>
      </div>
    </div>

    <!-- 快捷入口 -->
    <div class="panel">
      <div class="panel__title">
        快捷入口
      </div>
      <div class="shortcut-grid">
        <div
          v-for="sc in shortcuts"
          :key="sc.title"
          class="shortcut-card"
          @click="go(sc.to)"
        >
          <span
            class="shortcut-card__icon"
            :class="sc.icon"
          />
          <div>
            <div class="shortcut-card__title">
              {{ sc.title }}
            </div>
            <div class="shortcut-card__desc">
              {{ sc.desc }}
            </div>
          </div>
          <span class="i-mingcute-right-line shortcut-card__arrow" />
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/variables' as v;

.welcome {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: linear-gradient(135deg, #{v.$primary} 0%, #fc8bab 100%);
  border-radius: 12px;
  padding: 24px 28px;
  color: #fff;
  margin-bottom: 20px;
  overflow: hidden;
}

.welcome__title {
  margin: 0 0 6px;
  font-size: 22px;
}

.welcome__sub {
  margin: 0;
  font-size: 14px;
  opacity: 0.9;
}

.welcome__icon {
  font-size: 72px;
  opacity: 0.35;
}

.stat-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
  margin-bottom: 20px;
}

.stat-card {
  display: flex;
  align-items: center;
  gap: 16px;
  background: #fff;
  border-radius: 12px;
  padding: 20px;
  box-shadow: v.$shadow-card;
  cursor: pointer;
  transition: transform 0.15s, box-shadow 0.15s;

  &:hover {
    transform: translateY(-2px);
    box-shadow: v.$shadow-pop;
  }
}

.stat-card__icon {
  width: 52px;
  height: 52px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 26px;
  flex-shrink: 0;
}

.stat-card__value {
  font-size: 26px;
  font-weight: 700;
  color: v.$text-1;
  line-height: 1.2;
}

.stat-card__label {
  font-size: 13px;
  color: v.$text-2;
}

.panel {
  background: #fff;
  border-radius: 12px;
  padding: 20px;
  box-shadow: v.$shadow-card;
}

.panel__title {
  font-size: 16px;
  font-weight: 600;
  color: v.$text-1;
  margin-bottom: 16px;
}

/* 数据大盘 */
.dashboard-panel {
  margin-bottom: 20px;
}

.panel__sub {
  margin-left: 10px;
  font-size: 12px;
  font-weight: 400;
  color: v.$text-2;
}

.dash-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
  margin-bottom: 16px;
}

.dash-card {
  border-radius: 10px;
  padding: 16px 18px;
  color: #fff;

  &--pink {
    background: linear-gradient(135deg, #fb7299, #ff9eb8);
  }

  &--blue {
    background: linear-gradient(135deg, #3b82f6, #7ab0ff);
  }

  &--green {
    background: linear-gradient(135deg, #10b981, #5cd9b0);
  }

  &--orange {
    background: linear-gradient(135deg, #f59e0b, #ffc46b);
  }
}

.dash-card__value {
  font-size: 26px;
  font-weight: 700;
  line-height: 1.2;
}

.dash-card__label {
  margin-top: 4px;
  font-size: 12.5px;
  opacity: 0.9;
}

.dash-chart {
  margin-bottom: 16px;
}

.dash-chart__title {
  font-size: 13.5px;
  font-weight: 600;
  color: v.$text-1;
  margin-bottom: 8px;
}

.dash-chart__body {
  height: 260px;
  width: 100%;
}

.dash-extra {
  display: flex;
  gap: 12px;
}

.dash-extra__item {
  flex: 1;
  background: v.$surface;
  border: 1px solid v.$border;
  border-radius: 10px;
  padding: 14px 18px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.dash-extra__value {
  font-size: 22px;
  font-weight: 700;
  color: v.$primary;
}

.dash-extra__label {
  font-size: 12px;
  color: v.$text-2;
}

.shortcut-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
}

.shortcut-card {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 16px;
  border: 1px solid v.$border;
  border-radius: 10px;
  cursor: pointer;
  transition: all 0.15s;

  &:hover {
    border-color: v.$primary;
    background: #{v.$primary}0a;
  }
}

.shortcut-card__icon {
  font-size: 28px;
  color: v.$primary;
  flex-shrink: 0;
}

.shortcut-card__title {
  font-size: 15px;
  font-weight: 600;
  color: v.$text-1;
}

.shortcut-card__desc {
  font-size: 12px;
  color: v.$text-2;
}

.shortcut-card__arrow {
  margin-left: auto;
  color: v.$text-3;
}
</style>
