<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import * as echarts from 'echarts'
import { formatCount } from '@dlidli/shared'
import type { CreatorOverview, CreatorVideoStat, SettlementItem, TrendPoint } from '@dlidli/api-client'
import { api } from '@/api'
import defaultCover from '@/assets/default-cover.png'

const router = useRouter()

const overview = ref<CreatorOverview | null>(null)
const trend = ref<TrendPoint[]>([])
const loading = ref(true)

// 稿件数据
const videos = ref<CreatorVideoStat[]>([])
const videosTotal = ref(0)
const videosPage = ref(1)
const videosLoading = ref(false)
const videosLoaded = ref(false)

// 收益明细
const settles = ref<SettlementItem[]>([])
const settlesTotal = ref(0)
const settlesPage = ref(1)
const settlesLoading = ref(false)
const settlesLoaded = ref(false)

// 趋势图（echarts）：指标 + 天数切换
const trendChartEl = ref<HTMLDivElement>()
let trendChart: echarts.ECharts | null = null
const trendMetric = ref<'play' | 'like' | 'coin' | 'fav' | 'fans' | 'earning' | 'interact' | 'click' | 'expose'>('play')
const trendDays = ref<7 | 30>(7)
const METRIC_OPTIONS: Array<{ value: 'play' | 'like' | 'coin' | 'fav' | 'fans' | 'earning' | 'interact' | 'click' | 'expose'; label: string }> = [
  { value: 'play', label: '有效播放' },
  { value: 'like', label: '点赞' },
  { value: 'coin', label: '投币' },
  { value: 'fav', label: '收藏' },
  { value: 'fans', label: '涨粉' },
  { value: 'earning', label: '收益' },
  { value: 'interact', label: '互动' },
  { value: 'click', label: '点击' },
  { value: 'expose', label: '曝光' },
]
const metricLabel = computed(
  () => METRIC_OPTIONS.find((m) => m.value === trendMetric.value)?.label ?? '',
)

// 统计卡点击联动趋势指标：总播放→有效播放（近似）、总点赞→点赞、总投币→投币、粉丝→涨粉、累计收益→收益
function onStatCardClick(metric: 'play' | 'like' | 'coin' | 'fans' | 'earning') {
  trendMetric.value = metric
  trendDays.value = 7
}

async function loadTrend() {
  const tr = await api.creator.trend(trendDays.value, trendMetric.value)
  trend.value = tr.list
  await nextTick()
  renderTrend()
}

function renderTrend() {
  const el = trendChartEl.value
  if (!el) return
  if (!trendChart) trendChart = echarts.init(el)
  trendChart.setOption({
    tooltip: {
      trigger: 'axis',
      formatter: (params: unknown) => {
        const p = (params as Array<{ name?: string; value?: number }>)[0]
        return `${p?.name ?? ''}<br/>${metricLabel.value}：${p?.value ?? 0}`
      },
    },
    grid: { left: 40, right: 16, top: 28, bottom: 28 },
    xAxis: {
      type: 'category',
      data: trend.value.map((t) => t.date),
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
      {
        name: metricLabel.value,
        type: 'bar',
        data: trend.value.map((t) => t.views),
        barMaxWidth: 32,
        itemStyle: {
          borderRadius: [4, 4, 0, 0],
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: '#fb7299' },
            { offset: 1, color: '#ffb3c9' },
          ]),
        },
        emphasis: { itemStyle: { color: '#fb7299' } },
      },
    ],
  })
}

function onResize() {
  trendChart?.resize()
}

const STATUS_MAP: Record<number, { text: string; type: 'info' | 'warning' | 'success' | 'danger' }> = {
  0: { text: '草稿', type: 'info' },
  2: { text: '转码中', type: 'info' },
  3: { text: '审核中', type: 'warning' },
  4: { text: '已发布', type: 'success' },
  5: { text: '已驳回', type: 'danger' },
  6: { text: '已锁定', type: 'danger' },
}

// 收益展示：分 → 元
const earningsYuan = computed(() => ((overview.value?.earnings ?? 0) / 100).toFixed(2))

async function load() {
  loading.value = true
  try {
    const [ov] = await Promise.all([api.creator.overview()])
    overview.value = ov
  } finally {
    loading.value = false
  }
}

async function loadVideos(reset = false) {
  if (reset) {
    videosPage.value = 1
    videos.value = []
  }
  videosLoading.value = true
  try {
    const res = await api.creator.videos(videosPage.value, 10)
    videos.value = reset ? (res.list ?? []) : [...videos.value, ...(res.list ?? [])]
    videosTotal.value = res.total
  } finally {
    videosLoading.value = false
    if (reset) videosLoaded.value = true
  }
}

async function loadSettles(reset = false) {
  if (reset) {
    settlesPage.value = 1
    settles.value = []
  }
  settlesLoading.value = true
  try {
    const res = await api.creator.settlements(settlesPage.value, 10)
    settles.value = reset ? (res.list ?? []) : [...settles.value, ...(res.list ?? [])]
    settlesTotal.value = res.total
  } finally {
    settlesLoading.value = false
    if (reset) settlesLoaded.value = true
  }
}

type TabKey = 'videos' | 'settlements'
const activeTab = ref<TabKey>('videos')

function switchTab(t: TabKey) {
  activeTab.value = t
  if (t === 'videos' && !videosLoaded.value) loadVideos(true)
  if (t === 'settlements' && !settlesLoaded.value) loadSettles(true)
}

// 趋势图：指标/天数切换时重新加载
watch([trendMetric, trendDays], loadTrend)

onMounted(async () => {
  await load()
  loadTrend() // overview 就绪后图表容器已挂载，避免初始渲染竞态
  loadVideos(true)
  window.addEventListener('resize', onResize)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', onResize)
  trendChart?.dispose()
  trendChart = null
})
</script>

<template>
  <div class="mx-auto max-w-1100px">
    <el-skeleton
      v-if="loading"
      :rows="8"
      animated
    />

    <template v-else-if="overview">
      <!-- 概览统计卡（点击联动下方趋势图指标） -->
      <div class="cr-grid">
        <div
          class="cr-card"
          :class="{ 'is-active': trendMetric === 'play' }"
          title="查看有效播放趋势"
          @click="onStatCardClick('play')"
        >
          <p class="cr-card__num">
            {{ formatCount(overview.total_view) }}
          </p>
          <p class="cr-card__label">
            总有效播放
          </p>
        </div>
        <div
          class="cr-card"
          :class="{ 'is-active': trendMetric === 'like' }"
          title="查看点赞趋势"
          @click="onStatCardClick('like')"
        >
          <p class="cr-card__num">
            {{ formatCount(overview.total_like) }}
          </p>
          <p class="cr-card__label">
            总点赞
          </p>
        </div>
        <div
          class="cr-card"
          :class="{ 'is-active': trendMetric === 'coin' }"
          title="查看投币趋势"
          @click="onStatCardClick('coin')"
        >
          <p class="cr-card__num">
            {{ formatCount(overview.total_coin) }}
          </p>
          <p class="cr-card__label">
            总投币
          </p>
        </div>
        <div
          class="cr-card"
          :class="{ 'is-active': trendMetric === 'fans' }"
          title="查看涨粉趋势"
          @click="onStatCardClick('fans')"
        >
          <p class="cr-card__num">
            {{ formatCount(overview.fans) }}
          </p>
          <p class="cr-card__label">
            粉丝
          </p>
        </div>
        <div
          class="cr-card"
          :class="{ 'is-active': trendMetric === 'earning' }"
          title="查看收益趋势"
          @click="onStatCardClick('earning')"
        >
          <p class="cr-card__num cr-card__num--money">
            ¥{{ earningsYuan }}
          </p>
          <p class="cr-card__label">
            累计收益
          </p>
        </div>
      </div>

      <div class="cr-main">
        <!-- 数据趋势（echarts，指标/天数可切换） -->
        <div class="cr-panel">
          <div class="flex items-center justify-between flex-wrap gap-2 mb-3">
            <h3 class="cr-panel__title m-0">
              数据趋势
              <span class="cr-panel__sub">基于行为日志</span>
            </h3>
            <div class="flex items-center gap-2">
              <div class="flex gap-1">
                <span
                  v-for="m in METRIC_OPTIONS"
                  :key="m.value"
                  class="cr-chip"
                  :class="{ 'is-active': trendMetric === m.value }"
                  @click="trendMetric = m.value"
                >{{ m.label }}</span>
              </div>
              <div class="flex gap-1">
                <span
                  v-for="d in [7, 30]"
                  :key="d"
                  class="cr-chip"
                  :class="{ 'is-active': trendDays === d }"
                  @click="trendDays = d as 7 | 30"
                >近{{ d }}日</span>
              </div>
            </div>
          </div>
          <div
            ref="trendChartEl"
            class="cr-trend-chart"
          />
        </div>

        <!-- 数据明细 -->
        <div class="cr-panel">
          <div class="flex items-center gap-1.5 mb-3">
            <span
              class="cr-tab"
              :class="{ 'is-active': activeTab === 'videos' }"
              @click="switchTab('videos')"
            >稿件数据</span>
            <span
              class="cr-tab"
              :class="{ 'is-active': activeTab === 'settlements' }"
              @click="switchTab('settlements')"
            >收益明细</span>
          </div>

          <!-- 稿件数据 -->
          <div v-if="activeTab === 'videos'">
            <el-table
              v-loading="videosLoading"
              :data="videos"
              stripe
            >
              <el-table-column
                label="稿件"
                min-width="180"
              >
                <template #default="{ row }">
                  <div class="flex items-center gap-2">
                    <img
                      :src="row.cover || defaultCover"
                      alt=""
                      class="w-16 aspect-video object-cover rounded-4px bg-#f1f2f3"
                    >
                    <div class="min-w-0">
                      <p
                        class="m-0 text-3.5 font-600 truncate cursor-pointer hover:text-primary"
                        @click="router.push(`/video/${row.bvid}`)"
                      >
                        {{ row.title }}
                      </p>
                      <el-tag
                        size="small"
                        effect="plain"
                        :type="STATUS_MAP[row.status]?.type ?? 'info'"
                        class="mt-1"
                      >
                        {{ STATUS_MAP[row.status]?.text ?? '未知' }}
                      </el-tag>
                    </div>
                  </div>
                </template>
              </el-table-column>
              <el-table-column
                label="播放"
                width="80"
              >
                <template #default="{ row }">
                  {{ formatCount(row.view) }}
                </template>
              </el-table-column>
              <el-table-column
                label="赞/币/藏"
                width="110"
              >
                <template #default="{ row }">
                  {{ row.like }} / {{ row.coin }} / {{ row.fav }}
                </template>
              </el-table-column>
              <el-table-column
                label="有效播放"
                width="90"
              >
                <template #default="{ row }">
                  {{ row.valid_views }}
                </template>
              </el-table-column>
              <el-table-column
                label="收益"
                width="80"
              >
                <template #default="{ row }">
                  ¥{{ (row.earnings / 100).toFixed(2) }}
                </template>
              </el-table-column>
            </el-table>
            <div
              v-if="videos.length < videosTotal"
              class="text-center py-2"
            >
              <el-button
                link
                :loading="videosLoading"
                @click="videosPage++; loadVideos()"
              >
                加载更多（{{ videos.length }}/{{ videosTotal }}）
              </el-button>
            </div>
          </div>

          <!-- 收益明细 -->
          <div v-else>
            <el-table
              v-loading="settlesLoading"
              :data="settles"
              stripe
            >
              <el-table-column
                prop="date"
                label="日期"
                width="110"
              />
              <el-table-column
                label="稿件"
                min-width="180"
              >
                <template #default="{ row }">
                  <span
                    class="truncate block cursor-pointer hover:text-primary"
                    @click="router.push(`/video/${row.bvid}`)"
                  >{{ row.title }}</span>
                </template>
              </el-table-column>
              <el-table-column
                label="有效播放"
                width="100"
              >
                <template #default="{ row }">
                  {{ row.valid_views }}
                </template>
              </el-table-column>
              <el-table-column
                label="收益"
                width="90"
              >
                <template #default="{ row }">
                  ¥{{ (row.amount / 100).toFixed(2) }}
                </template>
              </el-table-column>
            </el-table>
            <div
              v-if="settles.length < settlesTotal"
              class="text-center py-2"
            >
              <el-button
                link
                :loading="settlesLoading"
                @click="settlesPage++; loadSettles()"
              >
                加载更多（{{ settles.length }}/{{ settlesTotal }}）
              </el-button>
            </div>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/variables' as v;

.cr-grid {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 12px;
  margin-bottom: 16px;

  @media (max-width: 900px) {
    grid-template-columns: repeat(2, 1fr);
  }
}

.cr-card {
  background: v.$surface;
  border-radius: v.$radius-lg;
  padding: 18px 16px;
  text-align: center;
  cursor: pointer;
  border: 2px solid transparent;
  transition: all 0.15s;

  &:hover {
    transform: translateY(-2px);
    box-shadow: v.$shadow-card;
  }

  /* 激活态统一：品牌粉渐变背景 + 白字 + 光晕（5 卡一致） */
  &.is-active {
    background: linear-gradient(135deg, v.$primary, #ff9eb8);
    color: #fff;
    border-color: transparent;
    box-shadow: 0 6px 16px rgba(251, 114, 153, 0.28);

    .cr-card__label {
      color: rgba(255, 255, 255, 0.85);
    }

    .cr-card__num--money {
      color: #fff;
    }
  }
}

.cr-card__num {
  margin: 0;
  font-size: 22px;
  font-weight: 800;

  &--money {
    color: v.$primary;
  }
}

.cr-card__label {
  margin: 6px 0 0;
  font-size: 12.5px;
  color: v.$text-2;
}

.cr-main {
  display: grid;
  gap: 16px;
}

.cr-panel {
  background: v.$surface;
  border-radius: v.$radius-lg;
  padding: 20px;
}

.cr-panel__title {
  margin: 0 0 16px;
  font-size: 15px;
  font-weight: 700;
  display: flex;
  align-items: baseline;
  gap: 8px;
}

.cr-panel__sub {
  font-size: 12px;
  font-weight: 400;
  color: v.$text-2;
}

/* 趋势图 */
.cr-trend-chart {
  height: 240px;
  width: 100%;
}

/* 指标/天数切换 chip */
.cr-chip {
  padding: 3px 12px;
  border-radius: 12px;
  font-size: 12px;
  color: v.$text-2;
  border: 1px solid v.$border;
  cursor: pointer;
  user-select: none;
  transition: all 0.15s;

  &:hover {
    color: v.$primary;
    border-color: v.$primary;
  }

  &.is-active {
    background: v.$primary;
    border-color: v.$primary;
    color: #fff;
  }
}

/* Tab */
.cr-tab {
  padding: 6px 16px;
  border-radius: v.$radius-md;
  font-size: 14px;
  cursor: pointer;
  color: v.$text-2;
  transition: all 0.15s;

  &:hover {
    color: v.$primary;
  }

  &.is-active {
    background: v.$primary;
    color: #fff;
    font-weight: 600;
  }
}
</style>
