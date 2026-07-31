<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
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

onMounted(loadStats)

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
