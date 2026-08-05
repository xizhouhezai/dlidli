<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useUserStore } from '@/stores/user'
import { api } from '@/api'
import defaultAvatar from '@/assets/default-avatar.png'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()

// 通知未读数（60s 轮询；进通知页后已读延时刷新）
const unread = ref(0)
let unreadTimer: ReturnType<typeof setInterval> | null = null

// 私信未读数（M3-IM：与通知同轮询）
const msgUnread = ref(0)

async function pollUnread() {
  if (!userStore.token) {
    unread.value = 0
    msgUnread.value = 0
    return
  }
  try {
    unread.value = (await api.notify.unreadCount()).count
  } catch {
    // 静默失败，下轮重试
  }
  try {
    msgUnread.value = (await api.message.unreadCount()).unread
  } catch {
    // 静默失败
  }
}

onMounted(() => {
  pollUnread()
  unreadTimer = setInterval(pollUnread, 60_000)
  // 私信实时收消息时刷新头部未读（MessagesView 派发）
  window.addEventListener('msg-unread-changed', pollUnread)
})

onUnmounted(() => {
  if (unreadTimer) clearInterval(unreadTimer)
  window.removeEventListener('msg-unread-changed', pollUnread)
})

watch(
  () => route.fullPath,
  () => {
    // 进入通知/私信页或私信会话切换后，未读已随读取清零，延时刷新头部红点
    if (route.name === 'notifications' || route.name === 'messages') {
      setTimeout(pollUnread, 800)
    }
  },
)

const searchDraft = ref('')

function onSearch() {
  const kw = searchDraft.value.trim()
  if (!kw) return
  router.push({ name: 'search', query: { kw } })
}

async function onLogout() {
  await userStore.logout()
  ElMessage.success('已退出登录')
  router.push('/')
}
</script>

<template>
  <header class="dli-header">
    <div class="mx-auto max-w-1440px h-15 flex items-center gap-6 px-6">
      <RouterLink
        to="/"
        class="dli-logo shrink-0 text-6 font-800"
      >
        DliDli
      </RouterLink>

      <nav class="dli-nav flex gap-5 text-3.5 shrink-0">
        <RouterLink to="/">
          首页
        </RouterLink>
        <RouterLink to="/feed">
          动态
        </RouterLink>
        <span
          class="text-#c0c4cc cursor-not-allowed"
          title="M1 开发中"
        >分区</span>
      </nav>

      <div class="flex-1 max-w-500px mx-auto">
        <el-input
          v-model="searchDraft"
          placeholder="搜索视频、UP 主"
          class="dli-search__input"
          @keyup.enter="onSearch"
        >
          <template #suffix>
            <span
              class="i-mingcute-search-2-line text-4.5 cursor-pointer"
              @click="onSearch"
            />
          </template>
        </el-input>
      </div>

      <div class="flex items-center gap-4 shrink-0 ml-auto">
        <template v-if="userStore.token">
          <RouterLink
            to="/notifications"
            class="dli-bell"
            title="消息通知"
          >
            <span class="i-mingcute-notification-line text-5.5" />
            <span
              v-if="unread > 0"
              class="dli-bell__badge"
            >{{ unread > 99 ? '99+' : unread }}</span>
          </RouterLink>
          <RouterLink
            to="/messages"
            class="dli-bell"
            title="私信"
          >
            <span class="i-mingcute-chat-3-line text-5.5" />
            <span
              v-if="msgUnread > 0"
              class="dli-bell__badge"
            >{{ msgUnread > 99 ? '99+' : msgUnread }}</span>
          </RouterLink>
          <el-dropdown>
            <div class="dli-user flex items-center gap-2 cursor-pointer outline-none">
              <el-avatar
                :size="36"
                :src="userStore.profile?.avatar || defaultAvatar"
                class="bg-primary text-white font-600"
              >
                {{ userStore.profile?.nickname?.slice(0, 1) ?? 'U' }}
              </el-avatar>
              <span class="text-3.5 max-w-120px truncate">{{ userStore.profile?.nickname }}</span>
            </div>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item @click="$router.push('/growth')">
                  Lv{{ userStore.profile?.level ?? 0 }} · 硬币 {{ userStore.profile?.coin ?? 0 }} · 成长中心
                </el-dropdown-item>
                <el-dropdown-item
                  divided
                  @click="$router.push(`/space/${userStore.profile?.id}`)"
                >
                  个人空间
                </el-dropdown-item>
                <el-dropdown-item
                  @click="$router.push('/mine/videos')"
                >
                  稿件管理
                </el-dropdown-item>
                <el-dropdown-item
                  @click="$router.push('/creator')"
                >
                  创作者中心
                </el-dropdown-item>
                <el-dropdown-item
                  @click="$router.push('/settings')"
                >
                  账号设置
                </el-dropdown-item>
                <el-dropdown-item
                  divided
                  @click="onLogout"
                >
                  退出登录
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
          <el-button
            type="primary"
            round
            class="dli-upload-btn"
            @click="router.push('/upload')"
          >
            投稿
          </el-button>
        </template>
        <el-button
          v-else
          type="primary"
          round
          @click="$router.push('/login')"
        >
          登录
        </el-button>
      </div>
    </div>
  </header>
</template>

<style scoped lang="scss">
@use '@/styles/variables' as v;

.dli-header {
  background: v.$surface;
  border-bottom: 1px solid v.$border;
  position: sticky;
  top: 0;
  z-index: v.$z-header;
}

.dli-logo {
  color: v.$primary;
}

.dli-nav {
  a.router-link-active {
    color: v.$primary;
    font-weight: 600;
  }
}

.dli-search__input :deep(.el-input__wrapper) {
  border-radius: v.$radius-md;
  background: #f1f2f3;
  box-shadow: none;

  &.is-focus {
    background: #fff;
    box-shadow: 0 0 0 1px v.$primary inset;
  }
}

/* 通知铃铛 */
.dli-bell {
  position: relative;
  font-size: 18px;
  text-decoration: none;
  cursor: pointer;
  transition: transform 0.15s;

  &:hover {
    transform: scale(1.15);
  }
}

.dli-bell__badge {
  position: absolute;
  top: -6px;
  right: -10px;
  min-width: 16px;
  height: 16px;
  padding: 0 4px;
  border-radius: 8px;
  background: v.$primary;
  color: #fff;
  font-size: 10px;
  line-height: 16px;
  text-align: center;
  font-weight: 600;
}

.dli-upload-btn {
  --el-button-bg-color: #{v.$primary};
  --el-button-border-color: #{v.$primary};
  --el-button-hover-bg-color: #{v.$primary-hover};
  --el-button-hover-border-color: #{v.$primary-hover};
}
</style>
