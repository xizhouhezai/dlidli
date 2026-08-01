<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useUserStore } from '@/stores/user'
import type { AssetLogItem, GrowthSummary } from '@dlidli/api-client'
import { api } from '@/api'
import { formatPubdate } from '@dlidli/shared'

const userStore = useUserStore()

/** 等级规则（与后端 growth.LevelRules 对齐：阈值/称号/权益） */
const LEVEL_RULES = [
  { level: 0, minExp: 0, title: '', privilege: '' },
  { level: 1, minExp: 0, title: '注册会员', privilege: '解锁弹幕发送' },
  { level: 2, minExp: 100, title: '初级会员', privilege: '' },
  { level: 3, minExp: 300, title: '中级会员', privilege: '解锁彩色弹幕、顶部/底部弹幕' },
  { level: 4, minExp: 800, title: '高级会员', privilege: '' },
  { level: 5, minExp: 1800, title: '资深会员', privilege: '' },
  { level: 6, minExp: 3600, title: '元老会员', privilege: '' },
]

const summary = ref<GrowthSummary | null>(null)
const loading = ref(true)

const levelRule = computed(() => LEVEL_RULES.find((r) => r.level === summary.value?.level) ?? LEVEL_RULES[1])
const nextRule = computed(() => LEVEL_RULES.find((r) => r.level === summary.value?.next_level) ?? null)
const maxLevel = computed(() => !summary.value || summary.value.next_level === 0)

// ---- 明细 Tab：经验 / 硬币 ----
type TabKey = 'exp' | 'coin'
const activeTab = ref<TabKey>('exp')
const logs = ref<AssetLogItem[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 10
const tabLoading = ref(false)

const taskIcons: Record<string, string> = {
  daily_login: 'i-mingcute-calendar-line',
  daily_watch: 'i-mingcute-play-line',
  video_upload: 'i-mingcute-upload-2-line',
  danmaku_send: 'i-mingcute-danmaku-line',
  comment_send: 'i-mingcute-message-line',
}

async function loadSummary() {
  loading.value = true
  try {
    summary.value = await api.growth.summary()
  } catch {
    summary.value = null
  } finally {
    loading.value = false
  }
}

async function loadLogs() {
  tabLoading.value = true
  try {
    const res =
      activeTab.value === 'exp'
        ? await api.growth.expLogs(page.value, pageSize)
        : await api.growth.coinLogs(page.value, pageSize)
    logs.value = res.list
    total.value = res.total
  } catch {
    logs.value = []
    total.value = 0
  } finally {
    tabLoading.value = false
  }
}

function switchTab(tab: TabKey) {
  activeTab.value = tab
  page.value = 1
  loadLogs()
}

function onPageChange(p: number) {
  page.value = p
  loadLogs()
}

onMounted(() => {
  loadSummary()
  loadLogs()
})
</script>

<template>
  <div class="mx-auto max-w-1100px">
    <el-skeleton
      v-if="loading"
      :rows="6"
      animated
    />

    <template v-else-if="summary">
      <!-- 顶部：等级卡 + 今日任务 -->
      <div class="growth-grid">
        <!-- 等级卡 -->
        <div class="g-card g-level">
          <div class="g-level__badge">
            <span class="g-level__num">Lv{{ summary.level }}</span>
            <span class="g-level__title">{{ levelRule.title }}</span>
          </div>
          <p class="g-level__exp">
            经验 <b>{{ summary.exp }}</b>
            <template v-if="!maxLevel">
              / {{ summary.next_exp }}
            </template>
          </p>
          <el-progress
            :percentage="summary.progress"
            :show-text="false"
            :stroke-width="8"
            class="g-level__bar"
          />
          <p class="g-level__next">
            <template v-if="!maxLevel">
              距 Lv{{ summary.next_level }} {{ levelRule.title }} 还差
              <b>{{ Math.max(summary.next_exp - summary.exp, 0) }}</b> 经验
            </template>
            <template v-else>
              已达成最高等级，感谢一路陪伴
            </template>
          </p>
          <ul class="g-level__priv">
            <li
              v-if="levelRule.privilege"
              class="is-locked"
            >
              <span class="i-mingcute-check-circle-line" />{{ levelRule.privilege }}
            </li>
            <li
              v-if="nextRule?.privilege"
              class="is-locked"
            >
              <span class="i-mingcute-lock-line" />Lv{{ nextRule.level }} 解锁：{{ nextRule.privilege }}
            </li>
          </ul>
        </div>

        <!-- 今日任务 -->
        <div class="g-card g-tasks">
          <h3 class="g-card__title">
            今日任务
            <span class="g-card__sub">每日经验获取</span>
          </h3>
          <ul class="g-tasks__list">
            <li
              v-for="t in summary.tasks"
              :key="t.reason"
              class="g-task"
              :class="{ 'is-done': t.done }"
            >
              <span :class="['g-task__icon', taskIcons[t.reason] ?? 'i-mingcute-star-line']" />
              <div class="g-task__info">
                <p class="g-task__name">
                  {{ t.name }}
                </p>
                <p class="g-task__progress">
                  <template v-if="t.limit === 1">
                    {{ t.done ? '已完成' : '未完成' }}
                  </template>
                  <template v-else>
                    {{ t.current }} / {{ t.limit }}
                  </template>
                </p>
              </div>
              <span class="g-task__delta">+{{ t.delta }} 经验</span>
              <span
                v-if="t.done"
                class="g-task__done i-mingcute-check-circle-fill"
              />
            </li>
          </ul>
        </div>
      </div>

      <!-- 明细 -->
      <div class="g-card g-logs">
        <div class="flex items-center gap-1.5 mb-3">
          <span
            class="g-tab"
            :class="{ 'is-active': activeTab === 'exp' }"
            @click="switchTab('exp')"
          >经验明细</span>
          <span
            class="g-tab"
            :class="{ 'is-active': activeTab === 'coin' }"
            @click="switchTab('coin')"
          >硬币明细<span class="g-tab__coin">余额 {{ userStore.profile?.coin ?? 0 }}</span></span>
        </div>

        <el-skeleton
          v-if="tabLoading"
          :rows="3"
          animated
        />
        <el-empty
          v-else-if="logs.length === 0"
          description="暂无记录，去完成任务吧"
        />
        <template v-else>
          <ul class="g-logs__list">
            <li
              v-for="l in logs"
              :key="l.id"
              class="g-log"
            >
              <span
                class="g-log__delta"
                :class="l.delta > 0 ? 'is-plus' : 'is-minus'"
              >{{ l.delta > 0 ? '+' : '' }}{{ l.delta }}</span>
              <div class="g-log__info">
                <p class="g-log__name">
                  {{ l.reason_name }}
                </p>
                <p class="g-log__time">
                  {{ formatPubdate(l.created_at) }}
                </p>
              </div>
            </li>
          </ul>
          <el-pagination
            v-if="total > pageSize"
            class="mt-3 justify-center"
            layout="prev, pager, next"
            :total="total"
            :page-size="pageSize"
            :current-page="page"
            @current-change="onPageChange"
          />
        </template>
      </div>
    </template>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/variables' as v;

.growth-grid {
  display: grid;
  grid-template-columns: 3fr 2fr;
  gap: 16px;
  margin-bottom: 16px;

  @media (max-width: 900px) {
    grid-template-columns: 1fr;
  }
}

.g-card {
  background: v.$surface;
  border-radius: v.$radius-lg;
  padding: 20px;
}

.g-card__title {
  margin: 0 0 14px;
  font-size: 16px;
  font-weight: 700;
  display: flex;
  align-items: baseline;
  gap: 8px;
}

.g-card__sub {
  font-size: 12px;
  font-weight: 400;
  color: v.$text-2;
}

/* ---- 等级卡 ---- */
.g-level__badge {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
}

.g-level__num {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 64px;
  height: 64px;
  padding: 0 12px;
  border-radius: 18px;
  background: linear-gradient(135deg, v.$primary, #ff9eb8);
  color: #fff;
  font-size: 22px;
  font-weight: 800;
  box-shadow: 0 6px 16px rgba(251, 114, 153, 0.35);
}

.g-level__title {
  font-size: 16px;
  font-weight: 600;
  color: v.$text-1;
}

.g-level__exp {
  margin: 0 0 6px;
  font-size: 13px;
  color: v.$text-2;

  b {
    color: v.$primary;
    font-size: 15px;
  }
}

.g-level__bar :deep(.el-progress-bar__outer) {
  background: v.$border;
}

.g-level__next {
  margin: 8px 0 12px;
  font-size: 12px;
  color: v.$text-2;

  b {
    color: v.$text-1;
  }
}

.g-level__priv {
  list-style: none;
  margin: 0;
  padding: 10px 0 0;
  border-top: 1px dashed v.$border;
  display: grid;
  gap: 8px;

  li {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 13px;
    color: v.$text-2;

    span {
      font-size: 16px;
      color: v.$primary;
    }

    &.is-locked span:last-child {
      color: v.$text-2;
    }
  }
}

/* ---- 今日任务 ---- */
.g-tasks__list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 10px;
}

.g-task {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border-radius: v.$radius-md;
  background: #fafafa;
  transition: background 0.15s;

  &.is-done {
    background: rgba(251, 114, 153, 0.08);

    .g-task__name {
      color: v.$text-2;
      text-decoration: line-through;
    }
  }
}

.g-task__icon {
  font-size: 20px;
  color: v.$primary;
  flex-shrink: 0;
}

.g-task__info {
  flex: 1;
  min-width: 0;
}

.g-task__name {
  margin: 0;
  font-size: 13.5px;
  font-weight: 600;
  color: v.$text-1;
}

.g-task__progress {
  margin: 2px 0 0;
  font-size: 12px;
  color: v.$text-2;
}

.g-task__delta {
  font-size: 12px;
  color: v.$primary;
  flex-shrink: 0;
}

.g-task__done {
  font-size: 16px;
  color: v.$primary;
  flex-shrink: 0;
}

/* ---- 明细 ---- */
.g-tab {
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

.g-tab__coin {
  margin-left: 6px;
  font-size: 12px;
  opacity: 0.8;
}

.g-logs__list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 2px;
}

.g-log {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 10px 8px;
  border-radius: v.$radius-md;

  &:hover {
    background: #fafafa;
  }
}

.g-log__delta {
  min-width: 56px;
  font-size: 15px;
  font-weight: 700;
  text-align: center;

  &.is-plus {
    color: v.$primary;
  }

  &.is-minus {
    color: v.$text-2;
  }
}

.g-log__info {
  flex: 1;
}

.g-log__name {
  margin: 0;
  font-size: 13.5px;
  color: v.$text-1;
}

.g-log__time {
  margin: 2px 0 0;
  font-size: 12px;
  color: v.$text-2;
}
</style>
