<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { bindKeyboard, PlayerCore, qualityLabel } from '@dlidli/player'
import { ElMessage, ElMessageBox } from 'element-plus'
import { formatCount, formatDuration, formatPubdate } from '@dlidli/shared'
import { ApiError, type CollectionItem, type DanmakuItem, type PartItem, type StreamItem, type VideoCard, type VideoDetail } from '@dlidli/api-client'
import { api } from '@/api'
import { useUserStore } from '@/stores/user'
import DanmakuLayer from '@/components/DanmakuLayer.vue'
import CommentSection from '@/components/CommentSection.vue'
import ReportDialog from '@/components/ReportDialog.vue'
import { useDanmakuSettings, type DanmakuSettings } from '@/composables/useDanmakuSettings'
import defaultCover from '@/assets/default-cover.svg'
import defaultAvatar from '@/assets/default-avatar.png'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()

const detail = ref<VideoDetail | null>(null)
const related = ref<VideoCard[]>([])
const loading = ref(true)
const notFound = ref(false)
const descExpanded = ref(false)

// 举报视频
const reportDialog = ref<InstanceType<typeof ReportDialog> | null>(null)
function openReport() {
  if (!userStore.token) {
    router.push('/login')
    return
  }
  reportDialog.value?.open()
}

// 播放器与清晰度
const videoEl = ref<HTMLVideoElement>()
const playerBox = ref<HTMLElement>()
const currentStream = ref<StreamItem | null>(null)
let player: PlayerCore | null = null
let unbindKeys: (() => void) | null = null

// 多P投稿（PRD VID-05）：分P列表与当前 P
const partList = ref<PartItem[]>([])
const currentPart = ref(0)

function switchPart(i: number) {
  if (i === currentPart.value || !partList.value[i]) return
  const part = partList.value[i]
  if (!part.streams.length) {
    ElMessage.warning('该分P暂无可用播放流')
    return
  }
  currentPart.value = i
  // 切换播放源（保留播放器实例，跳过续播：跨 P 进度独立由服务端按 bvid 记忆）
  player?.setSources(part.streams)
}

// 倍速
const playbackRate = ref(1)
const rateOptions = [0.5, 0.75, 1, 1.25, 1.5, 2]
function setRate(rate: number) {
  playbackRate.value = rate
  player?.setRate(rate)
}

// 弹幕
const dmEnabled = ref(true)
const dmInput = ref('')
const dmSending = ref(false)
const dmLayer = ref<InstanceType<typeof DanmakuLayer>>()
const dmSettings = useDanmakuSettings().settings

// 发送工具条（M2-DM-01）：模式 + 颜色（非白/顶底需 Lv3）
const dmMode = ref<1 | 2 | 3>(1)
const dmColor = ref(0xffffff)
const DM_COLORS: Array<{ name: string; value: number }> = [
  { name: '白', value: 0xffffff },
  { name: '红', value: 0xff0000 },
  { name: '橙', value: 0xff7f00 },
  { name: '黄', value: 0xffff00 },
  { name: '绿', value: 0x00ff00 },
  { name: '蓝', value: 0x00bfff },
  { name: '紫', value: 0xff00ff },
  { name: '粉', value: 0xff69b4 },
]
const isDmLevel3 = () => (userStore.profile?.level ?? 0) >= 3

// 弹幕列表面板（右侧内嵌，M3-DM-01 列表）
const dmList = ref<DanmakuItem[]>([])
const dmListTotal = ref(0)
const dmListPage = ref(1)
const dmListLoading = ref(false)

async function loadDmList(reset = false) {
  if (reset) {
    dmListPage.value = 1
    dmList.value = []
  }
  if (!detail.value) return
  dmListLoading.value = true
  try {
    const res = await api.danmaku.listAll(detail.value.bvid, dmListPage.value, 50)
    dmList.value = reset ? res.list : [...dmList.value, ...res.list]
    dmListTotal.value = res.total
  } catch {
    ElMessage.error('弹幕列表加载失败')
  } finally {
    dmListLoading.value = false
  }
}

function dmSeekTo(timeMs: number) {
  const video = videoEl.value
  if (!video) return
  video.currentTime = timeMs / 1000
}

// 弹幕设置面板
const dmSettingsVisible = ref(false)
const AREA_OPTIONS: Array<{ label: string; value: DanmakuSettings['area'] }> = [
  { label: '1/4 屏', value: 'quarter' },
  { label: '半屏', value: 'half' },
  { label: '全屏', value: 'full' },
]
const SPEED_OPTIONS: Array<{ label: string; value: DanmakuSettings['speed'] }> = [
  { label: '慢', value: 'slow' },
  { label: '标准', value: 'normal' },
  { label: '快', value: 'fast' },
]
const DENSITY_OPTIONS: Array<{ label: string; value: DanmakuSettings['density'] }> = [
  { label: '低', value: 'low' },
  { label: '标准', value: 'normal' },
  { label: '高', value: 'high' },
]

// 弹幕举报（复用 ReportDialog，target_type=3）
const dmReportDialog = ref<InstanceType<typeof ReportDialog> | null>(null)
const dmReportItem = ref<DanmakuItem | null>(null)
function onDmReport(item: DanmakuItem) {
  dmReportItem.value = item
  dmReportDialog.value?.open()
}

// 三连：赞 / 币 / 藏
const liked = ref(false)
const coined = ref(0) // 已投币数
const faved = ref(false)
const acting = ref(false)
const coinPopVisible = ref(false)

function requireLogin(): boolean {
  if (!userStore.token) {
    router.push('/login')
    return false
  }
  return true
}

async function toggleLike() {
  if (!requireLogin() || !detail.value || acting.value) return
  acting.value = true
  try {
    const res = await api.interaction.likeVideo(detail.value.bvid)
    liked.value = res.liked
    detail.value.stat.like += res.liked ? 1 : -1
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : '操作失败')
  } finally {
    acting.value = false
  }
}

// 关注（UP 主卡片）
const following = ref(false)
const followerCnt = ref(0)
const followPending = ref(false)

function loadRelation(ownerID: string) {
  api.relation
    .stat(ownerID)
    .then((st) => {
      following.value = st.following
      followerCnt.value = st.follower_cnt
    })
    .catch(() => {})
}

async function toggleFollow() {
  if (!requireLogin() || !detail.value || followPending.value) return
  followPending.value = true
  try {
    const res = await api.relation.follow(detail.value.owner.id)
    following.value = res.following
    followerCnt.value += res.following ? 1 : -1
    ElMessage.success(res.following ? '关注成功，可以召唤 TA 的更多更新啦' : '已取消关注')
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : '操作失败')
  } finally {
    followPending.value = false
  }
}

// 长按点赞 0.8s 触发一键三连（B 站同款交互）
let pressTimer: ReturnType<typeof setTimeout> | null = null
let longPressed = false

function startPress() {
  if (!userStore.token || !detail.value) return
  longPressed = false
  pressTimer = setTimeout(() => {
    longPressed = true
    doTriple()
  }, 800)
}

function endPress(isClick: boolean) {
  if (pressTimer) {
    clearTimeout(pressTimer)
    pressTimer = null
  }
  if (isClick && !longPressed) toggleLike()
}

async function doTriple() {
  if (!requireLogin() || !detail.value || acting.value) return
  acting.value = true
  try {
    const res = await api.interaction.triple(detail.value.bvid)
    liked.value = res.liked
    coined.value = res.coin_count
    faved.value = res.faved
    detail.value.stat.like += res.like_delta
    detail.value.stat.coin += res.coin_delta
    detail.value.stat.fav += res.fav_delta
    ElMessage.success(res.coin_delta > 0 ? '三连成功，感谢支持！' : '三连成功！')
    userStore.refreshProfile()
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : '操作失败')
  } finally {
    acting.value = false
  }
}

function openCoinPop() {
  if (!requireLogin() || !detail.value) return
  if (coined.value > 0) {
    ElMessage.info('已经投过币啦')
    return
  }
  coinPopVisible.value = !coinPopVisible.value
}

async function doCoin(count: 1 | 2) {
  coinPopVisible.value = false
  if (!detail.value || acting.value) return
  acting.value = true
  try {
    await api.interaction.coinVideo(detail.value.bvid, count)
    coined.value = count
    detail.value.stat.coin += count
    ElMessage.success(`投了 ${count} 枚硬币～`)
    userStore.refreshProfile()
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : '投币失败')
  } finally {
    acting.value = false
  }
}

// 转发到动态
async function openShare() {
  if (!requireLogin() || !detail.value) return
  let content = ''
  try {
    const res = await ElMessageBox.prompt('说点什么（可留空）', '转发到动态', {
      confirmButtonText: '转发',
      cancelButtonText: '取消',
      inputPlaceholder: '分享给关注你的人~',
    })
    content = res.value?.trim() ?? ''
  } catch {
    return
  }
  try {
    await api.dynamic.shareVideo(detail.value.bvid, content)
    detail.value.stat.share += 1
    ElMessage.success('已转发到动态')
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : '转发失败')
  }
}

async function toggleFav() {
  if (!requireLogin() || !detail.value || acting.value) return
  if (faved.value) {
    // 已收藏 → 直接取消
    doFav('0')
    return
  }
  // 未收藏 → 弹层选收藏夹
  favPopVisible.value = !favPopVisible.value
  if (favPopVisible.value) loadCollections()
}

// 收藏夹弹层
const favPopVisible = ref(false)
const collections = ref<CollectionItem[]>([])
const newColName = ref('')

async function loadCollections() {
  try {
    collections.value = await api.interaction.listCollections()
  } catch {
    collections.value = []
  }
}

async function createCollection() {
  const name = newColName.value.trim()
  if (!name) return
  try {
    const col = await api.interaction.createCollection(name)
    collections.value = [...collections.value, col]
    newColName.value = ''
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : '创建失败')
  }
}

async function doFav(collectionId: string) {
  favPopVisible.value = false
  if (!detail.value || acting.value) return
  acting.value = true
  try {
    const res = await api.interaction.toggleFavorite(detail.value.bvid, collectionId)
    faved.value = res.faved
    detail.value.stat.fav += res.faved ? 1 : -1
    ElMessage.success(res.faved ? '已收藏' : '已取消收藏')
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : '操作失败')
  } finally {
    acting.value = false
  }
}

async function sendDanmaku() {
  if (!userStore.token) {
    router.push('/login')
    return
  }
  const content = dmInput.value.trim()
  if (!content || !detail.value) return
  dmSending.value = true
  try {
    const item = await api.danmaku.send(detail.value.bvid, {
      content,
      time_ms: Math.floor((videoEl.value?.currentTime ?? 0) * 1000),
      mode: dmMode.value,
      color: dmColor.value,
    })
    dmLayer.value?.inject(item) // 乐观上屏
    dmInput.value = ''
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : '发送失败，请重试')
  } finally {
    dmSending.value = false
  }
}

// 有效播放上报（>5s，一次）
let viewReported = false
let watchedSeconds = 0
let lastTime = 0
// 观看进度上报节流
let lastProgressSave = 0

/** 登录用户续播：定位到上次观看位置（>2s 才续播，接近看完从头播） */
function tryResume(bvid: string) {
  if (!userStore.token) return
  api.video
    .getProgress(bvid)
    .then(({ position }) => {
      const video = videoEl.value
      if (!video || position <= 2) return
      const apply = () => {
        const dur = detail.value?.duration || 0
        if (dur > 0 && position >= dur - 3) return // 接近看完不续播
        video.currentTime = position
        lastTime = position
        ElMessage.info(`已定位到上次观看位置 ${formatDuration(position)}`)
      }
      if (video.readyState >= 1) apply()
      else video.addEventListener('loadedmetadata', apply, { once: true })
    })
    .catch(() => {})
}

/** 进页自动播放；被浏览器拦截时降级为静音自动播放 */
async function tryAutoplay() {
  const video = videoEl.value
  if (!video) return
  try {
    await video.play()
  } catch {
    video.muted = true
    try {
      await video.play()
      ElMessage.info('已静音自动播放，点击声音图标开启声音')
    } catch {
      // 仍被拦截则保持暂停，由用户手动播放
    }
  }
}

/** 确保 PlayerCore 已创建（绑定到当前 <video>）。 */
function ensurePlayer(): PlayerCore | null {
  const video = videoEl.value
  if (!video) return null
  if (!player) {
    player = new PlayerCore(video, {
      onSourceChange: (s) => {
        currentStream.value = s as StreamItem
      },
    })
  }
  return player
}

function switchQuality(stream: StreamItem) {
  player?.switchTo(stream)
  lastTime = videoEl.value?.currentTime ?? lastTime
}

async function load(bvid: string) {
  loading.value = true
  notFound.value = false
  detail.value = null
  related.value = []
  viewReported = false
  watchedSeconds = 0
  lastTime = 0
  lastProgressSave = 0
  player?.destroy()
  player = null

  try {
    const [d, partsRes] = await Promise.all([
      api.video.detail(bvid),
      api.video.parts(bvid).catch(() => ({ list: [] as PartItem[] })),
    ])
    detail.value = d
    document.title = `${d.title} - DliDli`

    // 先结束骨架屏让 <video> 渲染，再挂播放源（否则 videoEl 尚未存在，挂流静默失败）
    loading.value = false
    await nextTick()
    playbackRate.value = 1
    partList.value = partsRes.list
    currentPart.value = 0
    const p = ensurePlayer()
    // 多P：默认播第一 P 的流；单P：详情 streams
    const sources = partList.value[0]?.streams?.length ? partList.value[0].streams : detail.value.streams
    p?.setSources(sources) // 默认最高画质（streams 按 quality 降序，HLS 优先）
    // 右侧弹幕面板：进入即加载最近弹幕（失败不影响播放）
    loadDmList(true).catch(() => {})
    // 绑定快捷键（首次）
    if (!unbindKeys && videoEl.value) {
      unbindKeys = bindKeyboard(videoEl.value, { container: playerBox.value ?? null })
    }
    tryResume(bvid)
    void tryAutoplay()

    // 互动状态：赞/币/藏（仅登录用户）
    liked.value = false
    coined.value = 0
    faved.value = false
    coinPopVisible.value = false
    following.value = false
    followerCnt.value = 0
    loadRelation(detail.value.owner.id)
    if (userStore.token) {
      api.interaction
        .interactionState(bvid)
        .then((st) => {
          liked.value = st.liked
          coined.value = st.coin_count
          faved.value = st.faved
        })
        .catch(() => {})
    }

    // 相关推荐：同分区最热，剔除当前稿件（失败不影响播放）
    try {
      const res = await api.video.list({
        category_id: detail.value.category_id,
        sort: 'hot',
        page_size: 12,
      })
      related.value = res.list.filter((v) => v.bvid !== bvid).slice(0, 10)
    } catch {
      related.value = []
    }
  } catch {
    notFound.value = true
  } finally {
    loading.value = false
  }
}

onMounted(() => load(route.params.bvid as string))
watch(
  () => route.params.bvid,
  (bv) => {
    if (bv && route.name === 'video') load(bv as string)
  },
)

function onTimeUpdate(e: Event) {
  const video = e.target as HTMLVideoElement
  // 累计真实观看时长（跳转进度不计入）
  const delta = video.currentTime - lastTime
  if (delta > 0 && delta < 2) watchedSeconds += delta
  lastTime = video.currentTime

  if (!viewReported && watchedSeconds >= 5 && detail.value) {
    viewReported = true
    api.video.addView(detail.value.bvid).catch(() => {
      viewReported = false // 上报失败允许重试
    })
  }

  // 每 10s 上报观看进度（登录用户，跨端续播）
  if (userStore.token && detail.value && video.currentTime > 2) {
    const now = Date.now()
    if (now - lastProgressSave > 10_000) {
      lastProgressSave = now
      api.video.saveProgress(detail.value.bvid, Math.floor(video.currentTime)).catch(() => {})
    }
  }
}

onBeforeUnmount(() => {
  // 离开时落盘最终进度
  const video = videoEl.value
  if (userStore.token && detail.value && video && video.currentTime > 2) {
    api.video.saveProgress(detail.value.bvid, Math.floor(video.currentTime)).catch(() => {})
  }
  player?.destroy()
  player = null
  unbindKeys?.()
  unbindKeys = null
  document.title = 'DliDli - 视频社区'
})
</script>

<template>
  <el-skeleton
    v-if="loading"
    :rows="8"
    animated
  />

  <el-result
    v-else-if="notFound || !detail"
    icon="warning"
    title="稿件不存在或未发布"
  >
    <template #extra>
      <el-button
        type="primary"
        @click="router.push('/')"
      >
        回首页
      </el-button>
    </template>
  </el-result>

  <div
    v-else
    class="play-layout"
  >
    <!-- 左：播放器与信息 -->
    <div class="play-main">
      <h1 class="play-title">
        {{ detail.title }}
      </h1>
      <p class="play-meta">
        <span class="i-mingcute-play-circle-line mr-1" />{{ formatCount(detail.stat.view) }}
        <span class="gap" />
        <span class="i-mingcute-danmaku-line mr-1" />{{ formatCount(detail.stat.danmaku) }}
        <span class="gap" />
        {{ detail.published_at ? formatPubdate(detail.published_at) : '' }}
        <el-tag
          v-if="detail.copyright === 1"
          size="small"
          class="copyright-tag"
        >
          自制
        </el-tag>
      </p>

      <!-- 播放器：HLS 多清晰度（hls.js）+ 弹幕层 -->
      <div
        ref="playerBox"
        class="player-box"
      >
        <video
          ref="videoEl"
          class="play-video"
          :poster="detail.cover || defaultCover"
          controls
          preload="metadata"
          @timeupdate="onTimeUpdate"
        />
        <DanmakuLayer
          :key="detail.bvid"
          ref="dmLayer"
          :bvid="detail.bvid"
          :video="videoEl ?? null"
          :enabled="dmEnabled"
          :settings="dmSettings"
          @report="onDmReport"
        />
      </div>

      <!-- 控制行：弹幕开关（图标 toggle）+ 清晰度 -->
      <div class="play-controls">
        <span
          class="dm-toggle"
          :class="{ 'is-off': !dmEnabled }"
          :title="dmEnabled ? '弹幕开' : '弹幕关'"
          @click="dmEnabled = !dmEnabled"
        >
          <span :class="dmEnabled ? 'i-mingcute-danmaku-on-line' : 'i-mingcute-danmaku-off-line'" />
          <span class="dm-toggle__text">弹幕</span>
        </span>
        <span
          class="dm-tool"
          title="弹幕设置"
          @click="dmSettingsVisible = true"
        >
          <span class="i-mingcute-settings-3-line" />
        </span>
        <span class="play-controls__spacer" />
        <!-- 倍速 -->
        <el-dropdown
          trigger="click"
          @command="setRate"
        >
          <span class="play-toolbar__rate">
            {{ playbackRate === 1 ? '倍速' : playbackRate + 'x' }}
            <span class="i-mingcute-down-line" />
          </span>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item
                v-for="r in rateOptions"
                :key="r"
                :command="r"
                :class="{ 'is-active': playbackRate === r }"
              >
                {{ r === 1 ? '正常' : r + 'x' }}
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
        <span class="play-toolbar__label">清晰度</span>
        <el-button-group>
          <el-button
            v-for="s in detail.streams"
            :key="s.quality"
            size="small"
            :type="currentStream?.url === s.url ? 'primary' : 'default'"
            @click="switchQuality(s)"
          >
            {{ qualityLabel(s.quality) }}
          </el-button>
        </el-button-group>
      </div>
      <!-- 弹幕输入行（占满宽度） -->
      <div class="dm-bar">
        <el-input
          v-model="dmInput"
          class="dm-bar__input"
          maxlength="100"
          :placeholder="userStore.token ? '发个友善的弹幕见证当前进度' : '登录后发弹幕'"
          :disabled="!userStore.token"
          @keyup.enter="sendDanmaku"
        />
        <el-button
          type="primary"
          class="dm-send-btn"
          :loading="dmSending"
          @click="sendDanmaku"
        >
          发送
        </el-button>
      </div>

      <!-- 发送工具条（模式 + 色板） -->
      <div
        v-if="userStore.token"
        class="dm-toolbar"
      >
        <div class="flex items-center gap-1">
          <span
            v-for="m in [{ v: 1, t: '滚动' }, { v: 2, t: '顶部' }, { v: 3, t: '底部' }]"
            :key="m.v"
            class="dm-toolbar__mode"
            :class="{ 'is-active': dmMode === m.v, 'is-locked': m.v !== 1 && !isDmLevel3() }"
            :title="m.v !== 1 && !isDmLevel3() ? 'Lv3 解锁顶部/底部弹幕' : ''"
            @click="dmMode = isDmLevel3() || m.v === 1 ? (m.v as 1 | 2 | 3) : dmMode"
          >{{ m.t }}</span>
        </div>
        <div class="flex items-center gap-1">
          <span
            v-for="c in DM_COLORS"
            :key="c.value"
            class="dm-toolbar__color"
            :class="{ 'is-active': dmColor === c.value, 'is-locked': c.value !== 0xffffff && !isDmLevel3() }"
            :style="{ background: '#' + c.value.toString(16).padStart(6, '0') }"
            :title="c.value !== 0xffffff && !isDmLevel3() ? 'Lv3 解锁彩色弹幕' : c.name"
            @click="dmColor = isDmLevel3() || c.value === 0xffffff ? c.value : dmColor"
          />
        </div>
      </div>
      <p
        v-if="userStore.token && (userStore.profile?.level ?? 0) < 3"
        class="dm-privilege-tip"
      >
        <span class="i-mingcute-lock-line" />Lv3 解锁彩色弹幕与顶部/底部弹幕
      </p>

      <!-- 互动栏（播放器下方独立一行，对标 B 站） -->
      <div class="action-bar">
        <span
          class="act-btn"
          :class="{ 'is-active': liked }"
          title="长按一键三连"
          @mousedown="startPress"
          @mouseup="endPress(true)"
          @mouseleave="endPress(false)"
        >
          <span
            class="act-btn__icon"
            :class="liked ? 'i-mingcute-thumb-up-2-fill' : 'i-mingcute-thumb-up-2-line'"
          />
          <span class="act-btn__num">{{ formatCount(detail.stat.like) }}</span>
        </span>
        <span class="coin-wrap">
          <span
            class="act-btn"
            :class="{ 'is-active': coined > 0 }"
            @click="openCoinPop"
          >
            <span
              class="act-btn__icon"
              :class="coined > 0 ? 'i-mingcute-coin-2-fill' : 'i-mingcute-coin-2-line'"
            />
            <span class="act-btn__num">{{ formatCount(detail.stat.coin) }}</span>
          </span>
          <span
            v-if="coinPopVisible"
            class="coin-pop"
          >
            <span class="coin-pop__title">投给 UP 主（余额 {{ userStore.profile?.coin ?? 0 }}）</span>
            <span class="coin-pop__btns">
              <el-button
                size="small"
                type="primary"
                @click="doCoin(1)"
              >
                投 1 枚
              </el-button>
              <el-button
                v-if="detail.copyright === 1"
                size="small"
                type="primary"
                @click="doCoin(2)"
              >
                投 2 枚
              </el-button>
            </span>
          </span>
        </span>
        <span class="fav-wrap">
          <span
            class="act-btn"
            :class="{ 'is-active': faved }"
            @click="toggleFav"
          >
            <span
              class="act-btn__icon"
              :class="faved ? 'i-mingcute-star-2-fill' : 'i-mingcute-star-2-line'"
            />
            <span class="act-btn__num">{{ formatCount(detail.stat.fav) }}</span>
          </span>
          <span
            v-if="favPopVisible"
            class="fav-pop"
          >
            <span class="fav-pop__title">收藏到</span>
            <el-button
              v-for="col in collections"
              :key="col.id"
              size="small"
              @click="doFav(col.id)"
            >
              {{ col.name }}
            </el-button>
            <el-button
              v-if="collections.length === 0"
              size="small"
              type="primary"
              @click="doFav('0')"
            >
              默认收藏夹
            </el-button>
            <span class="fav-pop__new">
              <el-input
                v-model="newColName"
                size="small"
                maxlength="50"
                placeholder="新建收藏夹"
                @keyup.enter="createCollection"
              />
              <el-button
                size="small"
                @click="createCollection"
              >
                +
              </el-button>
            </span>
          </span>
        </span>
        <span
          class="act-btn"
          @click="openShare"
        >
          <span class="act-btn__icon i-mingcute-share-forward-line" />
          <span class="act-btn__num">{{ formatCount(detail.stat.share) }}</span>
        </span>
      </div>

      <!-- 简介与标签 -->
      <div
        v-if="detail.description"
        class="play-desc"
        :class="{ 'is-expanded': descExpanded }"
      >
        {{ detail.description }}
      </div>
      <el-button
        v-if="detail.description && detail.description.length > 60"
        link
        size="small"
        @click="descExpanded = !descExpanded"
      >
        {{ descExpanded ? '收起' : '展开更多' }}
      </el-button>
      <div class="play-tags">
        <el-tag
          v-for="t in detail.tags"
          :key="t"
          size="small"
          effect="plain"
        >
          {{ t }}
        </el-tag>
      </div>
      <span
        class="play-report"
        @click="openReport"
      >举报</span>

      <el-divider />
      <CommentSection
        :key="detail.bvid"
        :bvid="detail.bvid"
      />
    </div>

    <!-- 右：UP 主信息 / 分P / 弹幕列表 / 相关推荐（B 站风格侧栏） -->
    <aside class="play-side">
      <!-- UP 主卡片 -->
      <div class="up-card">
        <el-avatar
          :size="44"
          :src="detail.owner.avatar || defaultAvatar"
          class="up-card__avatar is-clickable"
          @click="$router.push(`/space/${detail.owner.id}`)"
        >
          {{ detail.owner.nickname?.slice(0, 1) ?? 'U' }}
        </el-avatar>
        <div class="up-card__info">
          <p
            class="up-card__name is-clickable"
            @click="$router.push(`/space/${detail.owner.id}`)"
          >
            {{ detail.owner.nickname }}
          </p>
          <p class="up-card__sign">
            {{ formatCount(followerCnt) }} 粉丝
          </p>
        </div>
        <el-button
          v-if="userStore.profile?.id !== detail.owner.id"
          round
          class="up-card__follow"
          :class="{ 'is-following': following }"
          :loading="followPending"
          @click="toggleFollow"
        >
          {{ following ? '已关注' : '+ 关注' }}
        </el-button>
      </div>

      <!-- 分P列表（竖排，多P投稿 PRD VID-05） -->
      <div
        v-if="partList.length > 0"
        class="side-block"
      >
        <p class="side-block__title">
          分P列表
        </p>
        <div
          v-for="(p, i) in partList"
          :key="p.index"
          class="part-list__item"
          :class="{ 'is-active': currentPart === i }"
          @click="switchPart(i)"
        >
          <span class="part-list__idx">P{{ i + 1 }}</span>
          <span class="part-list__title">{{ p.title || `分P${i + 1}` }}</span>
          <span class="part-list__dur">{{ p.duration ? formatDuration(p.duration) : '' }}</span>
        </div>
      </div>

      <!-- 弹幕列表面板（内嵌，点击跳转进度） -->
      <div class="side-block">
        <p class="side-block__title">
          弹幕列表
          <span class="side-block__count">{{ dmListTotal }}</span>
        </p>
        <div
          v-if="dmList.length === 0"
          class="dm-panel__empty"
        >
          还没有弹幕，来发第一条吧
        </div>
        <div
          v-else
          class="dm-panel"
        >
          <div
            v-for="d in dmList"
            :key="d.id"
            class="dm-panel__item"
            @click="dmSeekTo(d.time_ms)"
          >
            <span class="dm-panel__time">{{ formatDuration(d.time_ms / 1000) }}</span>
            <span
              class="dm-panel__text"
              :style="{ color: '#' + (d.color || 0xffffff).toString(16).padStart(6, '0') }"
            >{{ d.content }}</span>
          </div>
          <div
            v-if="dmList.length < dmListTotal"
            class="text-center py-1"
          >
            <el-button
              link
              size="small"
              :loading="dmListLoading"
              @click="dmListPage++; loadDmList()"
            >
              加载更多（{{ dmList.length }}/{{ dmListTotal }}）
            </el-button>
          </div>
        </div>
      </div>

      <!-- 相关推荐 -->
      <p class="side-title">
        相关推荐
      </p>
      <el-empty
        v-if="related.length === 0"
        description="暂无相关视频"
        :image-size="64"
      />
      <div
        v-for="v in related"
        :key="v.bvid"
        class="side-card"
        @click="router.push(`/video/${v.bvid}`)"
      >
        <div class="side-card__cover">
          <img
            :src="v.cover || defaultCover"
            :alt="v.title"
            loading="lazy"
          >
          <span
            v-if="v.duration > 0"
            class="side-card__duration"
          >{{ formatDuration(v.duration) }}</span>
        </div>
        <div class="side-card__info">
          <p class="side-card__title">
            {{ v.title }}
          </p>
          <p class="side-card__meta">
            {{ v.owner.nickname }}
          </p>
          <p class="side-card__meta flex items-center">
            <span class="i-mingcute-play-circle-line mr-1" />{{ formatCount(v.stat.view) }}
          </p>
        </div>
      </div>
    </aside>
  </div>

  <!-- 举报弹层 -->
  <ReportDialog
    ref="reportDialog"
    :target-type="1"
    :target-id="detail?.bvid ?? ''"
    :title="detail ? `视频：${detail.title}` : ''"
  />

  <!-- 弹幕举报弹层（target_type=3） -->
  <ReportDialog
    ref="dmReportDialog"
    :target-type="3"
    :target-id="dmReportItem?.id ?? ''"
    :title="dmReportItem ? `弹幕：${dmReportItem.content}` : ''"
  />

  <!-- 弹幕设置面板 -->
  <el-dialog
    v-model="dmSettingsVisible"
    title="弹幕设置"
    width="420px"
    top="18vh"
  >
    <div class="dm-settings">
      <div class="dm-settings__row">
        <span class="dm-settings__label">不透明度</span>
        <el-slider
          v-model="dmSettings.opacity"
          :min="0.2"
          :max="1"
          :step="0.05"
          class="flex-1"
        />
      </div>
      <div class="dm-settings__row">
        <span class="dm-settings__label">字号</span>
        <el-slider
          v-model="dmSettings.fontScale"
          :min="0.8"
          :max="1.5"
          :step="0.05"
          class="flex-1"
        />
      </div>
      <div class="dm-settings__row">
        <span class="dm-settings__label">显示区域</span>
        <el-radio-group v-model="dmSettings.area">
          <el-radio-button
            v-for="o in AREA_OPTIONS"
            :key="o.value"
            :value="o.value"
          >
            {{ o.label }}
          </el-radio-button>
        </el-radio-group>
      </div>
      <div class="dm-settings__row">
        <span class="dm-settings__label">滚动速度</span>
        <el-radio-group v-model="dmSettings.speed">
          <el-radio-button
            v-for="o in SPEED_OPTIONS"
            :key="o.value"
            :value="o.value"
          >
            {{ o.label }}
          </el-radio-button>
        </el-radio-group>
      </div>
      <div class="dm-settings__row">
        <span class="dm-settings__label">同屏密度</span>
        <el-radio-group v-model="dmSettings.density">
          <el-radio-button
            v-for="o in DENSITY_OPTIONS"
            :key="o.value"
            :value="o.value"
          >
            {{ o.label }}
          </el-radio-button>
        </el-radio-group>
      </div>
    </div>
  </el-dialog>
</template>

<style scoped lang="scss">
@use '@/styles/variables' as v;

.play-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 360px;
  gap: 20px;
  align-items: start;
}

@media (max-width: 1000px) {
  .play-layout {
    grid-template-columns: 1fr;
  }
}

.play-report {
  display: inline-block;
  margin: 10px 0 0;
  font-size: 12px;
  color: v.$text-2;
  cursor: pointer;
  transition: color 0.15s;

  &:hover {
    color: v.$primary;
  }
}

.play-title {
  margin: 0 0 6px;
  font-size: 20px;
  line-height: 1.4;
}

.play-meta {
  display: flex;
  align-items: center;
  margin: 0 0 12px;
  font-size: 13px;
  color: var(--dli-text-2);
}

.gap {
  display: inline-block;
  width: 14px;
}

.copyright-tag {
  margin-left: 12px;
}

/* 互动栏（播放器下方独立一行） */
.action-bar {
  display: flex;
  align-items: center;
  gap: 28px;
  margin: 14px 0 18px;
  padding-bottom: 16px;
  border-bottom: 1px solid var(--dli-border);
}

.act-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 15px;
  color: var(--dli-text-2);
  cursor: pointer;
  transition: color 0.15s;
  user-select: none;
}

.act-btn__icon {
  font-size: 24px;
}

.act-btn__num {
  font-size: 13px;
}

.act-btn:hover {
  color: var(--dli-primary);
}

.act-btn.is-active {
  color: var(--dli-primary);
}

.coin-wrap {
  position: relative;
  display: inline-flex;
}

.coin-pop {
  position: absolute;
  top: calc(100% + 8px);
  right: 0;
  z-index: 20;
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px;
  background: #fff;
  border: 1px solid #e3e5e7;
  border-radius: 8px;
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.12);
  white-space: nowrap;
}

.coin-pop__title {
  font-size: 12px;
  color: var(--dli-text-2);
}

.coin-pop__btns {
  display: flex;
}

/* 收藏夹弹层 */
.fav-wrap {
  position: relative;
  display: inline-flex;
}

.fav-pop {
  position: absolute;
  top: calc(100% + 8px);
  right: 0;
  z-index: 20;
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px;
  background: #fff;
  border: 1px solid #e3e5e7;
  border-radius: 8px;
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.12);
  min-width: 180px;
}

.fav-pop__title {
  font-size: 12px;
  color: var(--dli-text-2);
}

.fav-pop .el-button + .el-button {
  margin-left: 0;
}

.fav-pop__new {
  display: flex;
  gap: 6px;
}

.player-box {
  position: relative;
}

/* 分P列表（竖排，侧栏内） */
.part-list__item {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 8px 10px;
  border-radius: 8px;
  border: 1px solid var(--dli-border);
  font-size: 13px;
  color: var(--dli-text-2);
  cursor: pointer;
  transition: all 0.15s;
  margin-bottom: 6px;

  &:hover {
    color: var(--dli-primary);
    border-color: var(--dli-primary);
  }

  &.is-active {
    background: var(--dli-primary);
    border-color: var(--dli-primary);
    color: #fff;
  }
}

.part-list__idx {
  font-weight: 700;
  flex-shrink: 0;
}

.part-list__title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.part-list__dur {
  color: var(--dli-text-3);
  font-size: 11.5px;
  flex-shrink: 0;
}

.part-list__item.is-active .part-list__dur {
  color: rgba(255, 255, 255, 0.8);
}

.play-video {
  width: 100%;
  aspect-ratio: 16 / 9;
  background: #000;
  border-radius: 8px;
  display: block;
}

/* 控制行（弹幕开关 + 清晰度） */
.play-controls {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 10px 0;
}

.play-controls__spacer {
  flex: 1;
}

.play-toolbar__label {
  font-size: 13px;
  color: var(--dli-text-2);
}

.play-toolbar__rate {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  padding: 3px 10px;
  border-radius: 6px;
  font-size: 13px;
  color: var(--dli-text-2);
  cursor: pointer;
  transition: color 0.15s, background 0.15s;

  &:hover {
    color: var(--dli-primary);
    background: var(--dli-fill-1, rgba(0, 0, 0, 0.04));
  }
}

/* 弹幕开关（图标 toggle） */
.dm-toggle {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 12px;
  border-radius: 6px;
  font-size: 20px;
  color: var(--dli-primary);
  background: var(--dli-primary-light);
  cursor: pointer;
  user-select: none;
  transition: all 0.15s;
}

.dm-toggle__text {
  font-size: 13px;
}

.dm-toggle.is-off {
  color: var(--dli-text-3);
  background: #f1f2f3;
}

/* 弹幕列表/设置入口（开关旁小图标） */
.dm-tool {
  display: inline-flex;
  align-items: center;
  padding: 4px 8px;
  border-radius: 6px;
  font-size: 17px;
  color: var(--dli-text-2);
  cursor: pointer;
  user-select: none;
  transition: all 0.15s;

  &:hover {
    color: var(--dli-primary);
    background: var(--dli-primary-light);
  }
}

/* 发送工具条（模式 + 色板） */
.dm-toolbar {
  display: flex;
  align-items: center;
  gap: 14px;
  margin: -10px 0 12px;
}

.dm-toolbar__mode {
  padding: 2px 10px;
  border-radius: 12px;
  font-size: 12px;
  color: v.$text-2;
  cursor: pointer;
  border: 1px solid v.$border;
  user-select: none;
  transition: all 0.15s;

  &.is-active {
    background: v.$primary;
    border-color: v.$primary;
    color: #fff;
  }

  &.is-locked {
    color: v.$text-3;
    cursor: not-allowed;
    opacity: 0.5;
  }
}

.dm-toolbar__color {
  width: 16px;
  height: 16px;
  border-radius: 50%;
  border: 2px solid #fff;
  box-shadow: 0 0 0 1px v.$border;
  cursor: pointer;
  transition: transform 0.15s;

  &:hover {
    transform: scale(1.2);
  }

  &.is-active {
    box-shadow: 0 0 0 2px v.$primary;
  }

  &.is-locked {
    opacity: 0.35;
    cursor: not-allowed;
  }
}

/* 弹幕设置面板 */
.dm-settings {
  display: grid;
  gap: 16px;
}

.dm-settings__row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.dm-settings__label {
  flex-shrink: 0;
  width: 64px;
  font-size: 13.5px;
  color: v.$text-1;
}

/* 弹幕输入行（占满宽度） */
.dm-bar {
  display: flex;
  gap: 8px;
  margin: 0 0 20px;
}

.dm-bar__input {
  flex: 1;
  min-width: 0;
}

.dm-send-btn {
  --el-button-bg-color: #{v.$primary};
  --el-button-border-color: #{v.$primary};
  --el-button-hover-bg-color: #{v.$primary-hover};
  --el-button-hover-border-color: #{v.$primary-hover};
}

.dm-privilege-tip {
  margin: 8px 0 12px;
  font-size: 12px;
  color: v.$text-2;
  display: flex;
  align-items: center;
  gap: 4px;

  span {
    color: v.$primary;
  }
}

/* 侧栏信息块（分P / 弹幕列表） */
.side-block {
  margin-top: 16px;
}

.side-block__title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 14px;
  font-weight: 600;
  margin: 0 0 10px;
}

.side-block__count {
  font-size: 12px;
  font-weight: 400;
  color: var(--dli-text-3);
}

/* 弹幕列表面板（内嵌） */
.dm-panel {
  max-height: 280px;
  overflow-y: auto;
  border: 1px solid var(--dli-border);
  border-radius: 8px;
  padding: 4px 6px;
}

.dm-panel__item {
  display: flex;
  align-items: baseline;
  gap: 8px;
  padding: 6px 8px;
  border-radius: 6px;
  font-size: 12.5px;
  cursor: pointer;
  transition: background 0.15s;

  &:hover {
    background: #f6f7f8;
  }
}

.dm-panel__time {
  flex-shrink: 0;
  font-size: 11px;
  color: var(--dli-text-3);
  min-width: 34px;
}

.dm-panel__text {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dm-panel__empty {
  padding: 14px 0;
  text-align: center;
  font-size: 12.5px;
  color: var(--dli-text-3);
  border: 1px dashed var(--dli-border);
  border-radius: 8px;
}

/* UP 主卡片（侧栏置顶） */
.up-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  background: #fff;
  border: 1px solid var(--dli-border);
  border-radius: 10px;
}

.up-card__avatar {
  background: var(--dli-primary);
  color: #fff;
  font-weight: 600;
  flex-shrink: 0;
}

.up-card__info {
  flex: 1;
  min-width: 0;
}

.up-card__name {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
}

.up-card .is-clickable {
  cursor: pointer;
}

.up-card__name.is-clickable:hover {
  color: var(--dli-primary);
}

.up-card__sign {
  margin: 2px 0 0;
  font-size: 12px;
  color: var(--dli-text-2);
}

.up-card__follow {
  --el-button-bg-color: #{v.$primary};
  --el-button-border-color: #{v.$primary};
  --el-button-text-color: #fff;
  --el-button-hover-bg-color: #{v.$primary-hover};
  --el-button-hover-border-color: #{v.$primary-hover};
  --el-button-hover-text-color: #fff;
  min-width: 92px;
}

.up-card__follow.is-following {
  --el-button-bg-color: #f1f2f3;
  --el-button-border-color: #e3e5e7;
  --el-button-text-color: var(--dli-text-2);
  --el-button-hover-bg-color: #e9eaeb;
  --el-button-hover-border-color: #e3e5e7;
  --el-button-hover-text-color: var(--dli-text-2);
}

/* 简介与标签 */
.play-desc {
  font-size: 14px;
  line-height: 1.7;
  white-space: pre-wrap;
  display: -webkit-box;
  -webkit-line-clamp: 3;
  line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.play-desc.is-expanded {
  display: block;
  -webkit-line-clamp: unset;
  line-clamp: unset;
}

.play-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 12px;
}

/* 相关推荐 */
.side-title {
  margin: 0 0 12px;
  font-size: 15px;
  font-weight: 600;
}

.side-card {
  display: flex;
  gap: 10px;
  margin-bottom: 12px;
  cursor: pointer;
}

.side-card__cover {
  position: relative;
  width: 140px;
  aspect-ratio: 16 / 10;
  border-radius: 6px;
  overflow: hidden;
  flex-shrink: 0;
}

.side-card__cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.side-card__duration {
  position: absolute;
  right: 4px;
  bottom: 4px;
  padding: 0 5px;
  border-radius: 4px;
  background: rgba(0, 0, 0, 0.6);
  color: #fff;
  font-size: 11px;
}

.side-card__info {
  min-width: 0;
}

.side-card__title {
  margin: 0;
  font-size: 13px;
  line-height: 1.4;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.side-card:hover .side-card__title {
  color: var(--dli-primary);
}

.side-card__meta {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--dli-text-2);
}
</style>
