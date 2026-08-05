<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { formatCount } from '@dlidli/shared'
import { ApiError, type CollectionCard, type UserBrief, type VideoCard as VideoCardData } from '@dlidli/api-client'
import { api } from '@/api'
import { useUserStore } from '@/stores/user'
import defaultAvatar from '@/assets/default-avatar.png'
import defaultCover from '@/assets/default-cover.png'
import VideoCard from '@/components/VideoCard.vue'
import AccountStatusAlert from '@/components/AccountStatusAlert.vue'
import ReportDialog from '@/components/ReportDialog.vue'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()

const uid = computed(() => String(route.params.uid ?? ''))
const isSelf = computed(() => userStore.profile?.id === uid.value)

const profile = ref<UserBrief | null>(null)
const notFound = ref(false)
const following = ref(false)
const followerCnt = ref(0)
const followingCnt = ref(0)
const followPending = ref(false)

// 举报用户
const reportDialog = ref<InstanceType<typeof ReportDialog> | null>(null)
function openReport() {
  if (!userStore.token) {
    router.push('/login')
    return
  }
  reportDialog.value?.open()
}

type TabKey = 'videos' | 'collections' | 'followings' | 'followers' | 'favorites'
const activeTab = ref<TabKey>('videos')

const videos = ref<VideoCardData[]>([])
const users = ref<UserBrief[]>([])
const usersTotal = ref(0)
const tabLoading = ref(false)

// 合集（M3-CRT-05）：本人可创建，展示合集卡片
const collections = ref<CollectionCard[]>([])
const collectionsLoaded = ref(false)
const createColVisible = ref(false)
const createColForm = ref({ title: '', description: '' })
const createColSaving = ref(false)

async function loadCollections() {
  try {
    collections.value = (await api.collection.list(uid.value)).list ?? []
    collectionsLoaded.value = true
  } catch {
    collections.value = []
  }
}

async function createCollection() {
  if (!createColForm.value.title.trim()) {
    ElMessage.warning('请填写合集标题')
    return
  }
  createColSaving.value = true
  try {
    await api.collection.create({
      title: createColForm.value.title.trim(),
      description: createColForm.value.description.trim(),
    })
    createColVisible.value = false
    createColForm.value = { title: '', description: '' }
    ElMessage.success('合集已创建')
    loadCollections()
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : '创建失败')
  } finally {
    createColSaving.value = false
  }
}

async function loadHead() {
  notFound.value = false
  profile.value = null
  try {
    profile.value = await api.relation.profile(uid.value)
    document.title = `${profile.value.nickname} 的个人空间 - DliDli`
  } catch {
    notFound.value = true
    return
  }
  api.relation
    .stat(uid.value)
    .then((st) => {
      following.value = st.following
      followerCnt.value = st.follower_cnt
      followingCnt.value = st.following_cnt
    })
    .catch(() => {})
}

async function loadTab() {
  tabLoading.value = true
  videos.value = []
  users.value = []
  usersTotal.value = 0
  try {
    switch (activeTab.value) {
      case 'videos': {
        const res = await api.video.list({ uid: uid.value, sort: 'new', page_size: 24 })
        videos.value = res.list
        break
      }
      case 'collections': {
        await loadCollections()
        break
      }
      case 'followings': {
        const res = await api.relation.followings(uid.value, 1, 50)
        users.value = res.list
        usersTotal.value = res.total
        break
      }
      case 'followers': {
        const res = await api.relation.followers(uid.value, 1, 50)
        users.value = res.list
        usersTotal.value = res.total
        break
      }
      case 'favorites': {
        const res = await api.interaction.favorites(1, 24)
        videos.value = res.list
        break
      }
    }
  } finally {
    tabLoading.value = false
  }
}

async function toggleFollow() {
  if (!userStore.token) {
    router.push('/login')
    return
  }
  if (followPending.value) return
  followPending.value = true
  try {
    const res = await api.relation.follow(uid.value)
    following.value = res.following
    followerCnt.value += res.following ? 1 : -1
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : '操作失败')
  } finally {
    followPending.value = false
  }
}

function reload() {
  activeTab.value = 'videos'
  loadHead()
  loadTab()
}

onMounted(reload)
watch(uid, reload)
watch(activeTab, loadTab)
</script>

<template>
  <div class="mx-auto max-w-1100px">
    <el-result
      v-if="notFound"
      icon="warning"
      title="用户不存在"
    >
      <template #extra>
        <el-button @click="router.push('/')">
          回首页
        </el-button>
      </template>
    </el-result>

    <template v-else-if="profile">
      <!-- 本人空间才展示账号状态（禁言/封禁），不对外公开 -->
      <AccountStatusAlert
        v-if="isSelf"
        :user="userStore.profile"
      />

      <!-- 空间头部 -->
      <div class="space-head">
        <div class="space-head__banner" />
        <div class="space-head__main">
          <el-avatar
            :size="72"
            :src="profile.avatar || defaultAvatar"
            class="space-head__avatar"
          >
            {{ profile.nickname?.slice(0, 1) ?? 'U' }}
          </el-avatar>
          <div class="space-head__info">
            <p class="space-head__name">
              {{ profile.nickname }}
              <el-tag
                size="small"
                effect="plain"
              >
                Lv{{ profile.level }}
              </el-tag>
            </p>
            <p class="space-head__sign">
              {{ profile.signature || '这个人很神秘，什么都没有写' }}
            </p>
            <p class="space-head__stats">
              <span
                class="stat-item"
                @click="activeTab = 'followings'"
              >关注 <b>{{ formatCount(followingCnt) }}</b></span>
              <span
                class="stat-item"
                @click="activeTab = 'followers'"
              >粉丝 <b>{{ formatCount(followerCnt) }}</b></span>
            </p>
          </div>
          <el-button
            v-if="!isSelf"
            round
            class="space-head__follow"
            :class="{ 'is-following': following }"
            :loading="followPending"
            @click="toggleFollow"
          >
            {{ following ? '已关注' : '+ 关注' }}
          </el-button>
          <el-button
            v-else
            round
            @click="router.push('/settings')"
          >
            编辑资料
          </el-button>
          <el-button
            v-if="!isSelf"
            text
            class="space-head__report"
            @click="openReport"
          >
            举报
          </el-button>
        </div>
      </div>

      <!-- Tab 导航 -->
      <div class="flex gap-1.5 mb-3.5">
        <span
          class="space-tab"
          :class="{ 'is-active': activeTab === 'videos' }"
          @click="activeTab = 'videos'"
        >投稿</span>
        <span
          class="space-tab"
          :class="{ 'is-active': activeTab === 'collections' }"
          @click="activeTab = 'collections'"
        >合集</span>
        <span
          class="space-tab"
          :class="{ 'is-active': activeTab === 'followings' }"
          @click="activeTab = 'followings'"
        >关注</span>
        <span
          class="space-tab"
          :class="{ 'is-active': activeTab === 'followers' }"
          @click="activeTab = 'followers'"
        >粉丝</span>
        <span
          v-if="isSelf"
          class="space-tab"
          :class="{ 'is-active': activeTab === 'favorites' }"
          @click="activeTab = 'favorites'"
        >收藏</span>
      </div>

      <el-skeleton
        v-if="tabLoading"
        :rows="4"
        animated
      />

      <!-- 合集（M3-CRT-05） -->
      <template v-else-if="activeTab === 'collections'">
        <div
          v-if="isSelf"
          class="flex justify-end mb-2"
        >
          <el-button
            size="small"
            type="primary"
            round
            @click="createColVisible = true"
          >
            + 新建合集
          </el-button>
        </div>
        <el-empty
          v-if="collections.length === 0"
          description="还没有合集"
        />
        <div
          v-else
          class="grid grid-cols-2 sm:grid-cols-3 gap-3"
        >
          <div
            v-for="c in collections"
            :key="c.id"
            class="col-card"
            @click="router.push(`/collections/${c.id}`)"
          >
            <img
              :src="c.cover || defaultCover"
              alt=""
              class="col-card__cover"
            >
            <p class="col-card__title">
              {{ c.title }}
            </p>
            <p class="col-card__meta">
              {{ c.video_count }} 个视频
            </p>
          </div>
        </div>
      </template>

      <!-- 视频网格（投稿 / 收藏） -->
      <template v-else-if="activeTab === 'videos' || activeTab === 'favorites'">
        <el-empty
          v-if="videos.length === 0"
          :description="activeTab === 'videos' ? 'TA 还没有投稿' : '还没有收藏任何视频'"
        />
        <div
          v-else
          class="space-grid"
        >
          <VideoCard
            v-for="v in videos"
            :key="v.bvid"
            :video="v"
            show-date
          />
        </div>
      </template>

      <!-- 用户列表（关注 / 粉丝） -->
      <template v-else>
        <el-empty
          v-if="users.length === 0"
          :description="activeTab === 'followings' ? '还没有关注任何人' : '还没有粉丝，快去投稿吧'"
        />
        <div
          v-else
          class="space-users"
        >
          <div
            v-for="u in users"
            :key="u.id"
            class="space-user flex items-center gap-3 p-4 rounded-10px bg-white cursor-pointer"
            @click="router.push(`/space/${u.id}`)"
          >
            <el-avatar
              :size="48"
              :src="u.avatar || defaultAvatar"
              class="shrink-0 bg-primary text-white font-600"
            >
              {{ u.nickname?.slice(0, 1) ?? 'U' }}
            </el-avatar>
            <div class="flex-1 min-w-0">
              <p class="space-user__name m-0 text-3.5 font-600">
                {{ u.nickname }}
              </p>
              <p class="mt-0.75 mb-0 text-3 text-text-2 truncate">
                {{ u.signature || 'TA 还没有签名' }}
              </p>
            </div>
            <el-tag
              size="small"
              effect="plain"
            >
              Lv{{ u.level }}
            </el-tag>
          </div>
        </div>
      </template>
    </template>
  </div>

  <!-- 举报弹层 -->
  <ReportDialog
    ref="reportDialog"
    :target-type="5"
    :target-id="profile?.id ?? ''"
    :title="profile ? `用户：${profile.nickname}` : ''"
  />

  <!-- 新建合集 -->
  <el-dialog
    v-model="createColVisible"
    title="新建合集"
    width="420px"
  >
    <el-form
      label-width="60px"
      class="max-w-380px"
    >
      <el-form-item label="标题">
        <el-input
          v-model="createColForm.title"
          placeholder="合集标题（必填）"
          maxlength="64"
        />
      </el-form-item>
      <el-form-item label="简介">
        <el-input
          v-model="createColForm.description"
          type="textarea"
          :rows="2"
          placeholder="合集简介（选填）"
          maxlength="200"
        />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="createColVisible = false">
        取消
      </el-button>
      <el-button
        type="primary"
        :loading="createColSaving"
        @click="createCollection"
      >
        创建
      </el-button>
    </template>
  </el-dialog>
</template>

<style scoped lang="scss">
@use '@/styles/variables' as v;

/* 合集卡片 */
.col-card {
  background: #fff;
  border-radius: 10px;
  overflow: hidden;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.06);
  cursor: pointer;
  transition: transform 0.15s, box-shadow 0.15s;

  &:hover {
    transform: translateY(-2px);
    box-shadow: 0 6px 18px rgba(0, 0, 0, 0.12);
  }
}

.col-card__cover {
  width: 100%;
  aspect-ratio: 16 / 9;
  object-fit: cover;
  background: #f1f2f3;
}

.col-card__title {
  margin: 8px 10px 0;
  font-size: 14px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.col-card__meta {
  margin: 2px 10px 10px;
  font-size: 12px;
  color: #9499a0;
}

/* 头部 */
.space-head {
  background: v.$surface;
  border-radius: v.$radius-lg;
  overflow: hidden;
  margin-bottom: 16px;
}

.space-head__banner {
  height: 110px;
  background:
    radial-gradient(ellipse at 80% 120%, rgba(35, 173, 229, 0.35), transparent 60%),
    linear-gradient(120deg, v.$primary, #fc9db8);
}

.space-head__main {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 0 24px 18px;
}

.space-head__avatar {
  margin-top: -28px;
  border: 3px solid #fff;
  background: v.$primary;
  color: #fff;
  font-size: 26px;
  font-weight: 600;
  flex-shrink: 0;
}

.space-head__info {
  flex: 1;
  min-width: 0;
  padding-top: 10px;
}

.space-head__name {
  margin: 0;
  font-size: 18px;
  font-weight: 700;
  display: flex;
  align-items: center;
  gap: 8px;
}

.space-head__sign {
  margin: 4px 0;
  font-size: 13px;
  color: v.$text-2;
  @include v.ellipsis(1);
}

.space-head__stats {
  margin: 0;
  font-size: 13px;
  color: v.$text-2;
  display: flex;
  gap: 18px;
}

.stat-item {
  cursor: pointer;

  &:hover {
    color: v.$primary;
  }

  b {
    color: v.$text-1;
    margin-left: 2px;
  }
}

.space-head__follow {
  --el-button-bg-color: #{v.$primary};
  --el-button-border-color: #{v.$primary};
  --el-button-text-color: #fff;
  --el-button-hover-bg-color: #{v.$primary-hover};
  --el-button-hover-border-color: #{v.$primary-hover};
  --el-button-hover-text-color: #fff;
  min-width: 96px;

  &.is-following {
    --el-button-bg-color: #f1f2f3;
    --el-button-border-color: #{v.$border};
    --el-button-text-color: #{v.$text-2};
    --el-button-hover-bg-color: #e9eaeb;
    --el-button-hover-border-color: #{v.$border};
    --el-button-hover-text-color: #{v.$text-2};
  }
}

.space-head__report {
  color: v.$text-2;

  &:hover {
    color: v.$primary;
  }
}

/* Tab（hover/active 交互态） */
.space-tab {
  padding: 6px 18px;
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

/* 自适应网格 */
.space-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 16px;
}

.space-users {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 12px;
}

.space-user {
  transition: box-shadow 0.15s;

  &:hover {
    box-shadow: v.$shadow-card;

    .space-user__name {
      color: v.$primary;
    }
  }
}
</style>
