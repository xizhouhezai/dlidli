import { computed, nextTick, ref, watch } from 'vue'
import * as echarts from 'echarts'
import type { TrendPoint } from '@dlidli/api-client'
import { api } from '@/api'

export type CreatorTrendMetric =
  'play' | 'like' | 'coin' | 'fav' | 'fans' | 'earning' | 'interact' | 'click' | 'expose'

const METRIC_OPTIONS: Array<{ value: CreatorTrendMetric; label: string }> = [
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

/**
 * 数据趋势图（CreatorView 拆分，M3-ENG-10）：
 * echarts 柱状趋势，指标/天数切换自动重载；resize 与 dispose 由 start/stop 编排。
 */
export function useCreatorTrend() {
  const trendChartEl = ref<HTMLDivElement>()
  let trendChart: echarts.ECharts | null = null
  const trend = ref<TrendPoint[]>([])
  const trendMetric = ref<CreatorTrendMetric>('play')
  const trendDays = ref<7 | 30>(7)

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
          const v = p?.value ?? 0
          // 收益为分，tooltip 统一显示为 ¥ 元（与累计收益卡片同单位，避免歧义）
          const text = trendMetric.value === 'earning' ? `¥${(v / 100).toFixed(2)}` : String(v)
          return `${p?.name ?? ''}<br/>${metricLabel.value}：${text}`
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

  // 趋势图：指标/天数切换时重新加载
  watch([trendMetric, trendDays], () => void loadTrend())

  /** 进入页面：挂 resize 监听并首次加载。 */
  function start() {
    window.addEventListener('resize', onResize)
    return loadTrend()
  }
  function stop() {
    window.removeEventListener('resize', onResize)
    trendChart?.dispose()
    trendChart = null
  }

  return {
    trendChartEl,
    trendMetric,
    trendDays,
    METRIC_OPTIONS,
    onStatCardClick,
    start,
    stop,
  }
}
