<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { formatCount } from '@dlidli/shared'
import type { CreatorOverview, CreatorVideoStat, SettlementItem, TrendPoint } from '@dlidli/api-client'
import { api } from '@/api'

const router = useRouter()

const overview = ref<CreatorOverview | null>(null)
const trend = ref<TrendPoint[]>([])
const loading = ref(true)

// 稿件数据
const videos = ref<CreatorVideoStat[]>([])
const videosTotal = ref(0)
const videosPage = ref(1)
const videosLoading = ref(false)

// 收益明细
const settles = ref<SettlementItem[]>([])
const settlesTotal = ref(0)
const settlesPage = ref(1)
const settlesLoading = ref(false)

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

// 趋势柱状图（纯 CSS 柱）
const maxTrend = computed(() => Math.max(...trend.value.map((t) => t.views), 1))

async function load() {
  loading.value = true
  try {
    const [ov, tr] = await Promise.all([api.creator.overview(), api.creator.trend(7)])
    overview.value = ov
    trend.value = tr.list
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
    videos.value = reset ? res.list : [...videos.value, ...res.list]
    videosTotal.value = res.total
  } finally {
    videosLoading.value = false
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
    settles.value = reset ? res.list : [...settles.value, ...res.list]
    settlesTotal.value = res.total
  } finally {
    settlesLoading.value = false
  }
}

type TabKey = 'videos' | 'settlements'
const activeTab = ref<TabKey>('videos')

function switchTab(t: TabKey) {
  activeTab.value = t
  if (t === 'videos' && videos.value.length === 0) loadVideos(true)
  if (t === 'settlements' && settles.value.length === 0) loadSettles(true)
}

onMounted(() => {
  load()
  loadVideos(true)
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
      <!-- 概览统计卡 -->
      <div class="cr-grid">
        <div class="cr-card cr-card--primary">
          <p class="cr-card__num">
            {{ formatCount(overview.total_view) }}
          </p>
          <p class="cr-card__label">
            总播放
          </p>
        </div>
        <div class="cr-card">
          <p class="cr-card__num">
            {{ formatCount(overview.total_like) }}
          </p>
          <p class="cr-card__label">
            总点赞
          </p>
        </div>
        <div class="cr-card">
          <p class="cr-card__num">
            {{ formatCount(overview.total_coin) }}
          </p>
          <p class="cr-card__label">
            总投币
          </p>
        </div>
        <div class="cr-card">
          <p class="cr-card__num">
            {{ formatCount(overview.fans) }}
          </p>
          <p class="cr-card__label">
            粉丝
          </p>
        </div>
        <div class="cr-card">
          <p class="cr-card__num cr-card__num--money">
            ¥{{ earningsYuan }}
          </p>
          <p class="cr-card__label">
            累计收益
          </p>
        </div>
      </div>

      <div class="cr-main">
        <!-- 近 7 日播放趋势 -->
        <div class="cr-panel">
          <h3 class="cr-panel__title">
            近 7 日有效播放
            <span class="cr-panel__sub">共 {{ overview.week_view }} 次</span>
          </h3>
          <div class="cr-trend">
            <div
              v-for="t in trend"
              :key="t.date"
              class="cr-trend__col"
            >
              <div class="cr-trend__bar-wrap">
                <div
                  class="cr-trend__bar"
                  :style="{ height: `${(t.views / maxTrend) * 100}%` }"
                  :class="{ 'is-zero': t.views === 0 }"
                />
              </div>
              <span class="cr-trend__val">{{ t.views }}</span>
              <span class="cr-trend__date">{{ t.date }}</span>
            </div>
          </div>
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
                      :src="row.cover"
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

  &--primary {
    background: linear-gradient(135deg, v.$primary, #ff9eb8);
    color: #fff;

    .cr-card__label {
      color: rgba(255, 255, 255, 0.85);
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

/* 趋势柱状 */
.cr-trend {
  display: flex;
  align-items: flex-end;
  gap: 10px;
  height: 160px;
}

.cr-trend__col {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  height: 100%;
}

.cr-trend__bar-wrap {
  flex: 1;
  width: 100%;
  display: flex;
  align-items: flex-end;
  justify-content: center;
}

.cr-trend__bar {
  width: 60%;
  min-height: 4px;
  border-radius: 4px 4px 0 0;
  background: linear-gradient(180deg, v.$primary, #ffb3c9);
  transition: height 0.3s;

  &.is-zero {
    background: v.$border;
  }
}

.cr-trend__val {
  font-size: 11px;
  color: v.$text-2;
}

.cr-trend__date {
  font-size: 11px;
  color: v.$text-3;
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
