<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { qualityLabel } from '@dlidli/player'
import { formatCount, formatPubdate } from '@dlidli/shared'
import type { PartItem, StreamItem, VideoCard, VideoDetail } from '@dlidli/api-client'
import { api } from '@/api'
import { useUserStore } from '@/stores/user'
import DanmakuLayer from '@/components/DanmakuLayer.vue'
import CommentSection from '@/components/CommentSection.vue'
import ReportDialog from '@/components/ReportDialog.vue'
import VideoSidebar from '@/components/video/VideoSidebar.vue'
import DanmakuSettingsDialog from '@/components/video/DanmakuSettingsDialog.vue'
import { useVideoPlayer } from '@/composables/video/useVideoPlayer'
import { useDanmakuController } from '@/composables/video/useDanmakuController'
import { useVideoActions } from '@/composables/video/useVideoActions'
import { usePlaybackReport } from '@/composables/video/usePlaybackReport'
import defaultCover from '@/assets/default-cover.svg'

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

// —— 拆分出的子模块（M3-ENG-06）：播放器 / 弹幕 / 互动 / 播放统计 ——
const player = useVideoPlayer(detail)
const dm = useDanmakuController(detail, player.videoEl)
const acts = useVideoActions(detail)
const report = usePlaybackReport(detail, player.videoEl)

const {
  videoEl,
  playerBox,
  currentStream,
  partList,
  currentPart,
  playbackRate,
  rateOptions,
  setRate,
  ensurePlayer,
  switchTo,
  destroy,
  bindKeys,
  unbindKeyListeners,
  tryResume,
  tryAutoplay,
} = player
const {
  dmEnabled,
  dmInput,
  dmSending,
  dmLayer,
  dmSettings,
  dmSettingsVisible,
  dmMode,
  dmColor,
  DM_COLORS,
  isDmLevel3,
  loadDmList,
  dmReportDialog,
  dmReportItem,
  onDmReport,
  sendDanmaku,
} = dm
const {
  liked,
  coined,
  faved,
  coinPopVisible,
  favPopVisible,
  collections,
  newColName,
  openCoinPop,
  doCoin,
  doFav,
  createCollection,
  openShare,
  toggleFav,
  startPress,
  endPress,
} = acts

// 模板 ref 绑定：vue-tsc 对解构变量不识别模板引用，此处显式登记避免误报未使用
void playerBox
void dmLayer
void dmReportDialog

function switchQuality(stream: StreamItem) {
  switchTo(stream)
  report.notePosition()
}

function onTimeUpdate(e: Event) {
  report.onTimeUpdate(e)
}

async function load(bvid: string) {
  loading.value = true
  notFound.value = false
  detail.value = null
  related.value = []
  report.reset()
  destroy()

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
    const sources = partList.value[0]?.streams?.length
      ? partList.value[0].streams
      : detail.value.streams
    p?.setSources(sources) // 默认最高画质（streams 按 quality 降序，HLS 优先）
    // 右侧弹幕面板：进入即加载最近弹幕（失败不影响播放）
    loadDmList(true).catch(() => {})
    bindKeys()
    tryResume(bvid)
    void tryAutoplay()
    // 互动状态：赞/币/藏/关注（仅登录用户）
    acts.reset()
    acts.loadRelation(detail.value.owner.id)
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

onBeforeUnmount(() => {
  // 离开时落盘最终进度
  report.flushProgress()
  destroy()
  unbindKeyListeners()
  document.title = 'DliDli - 视频社区'
})
</script>

<template>
  <el-skeleton v-if="loading" :rows="8" animated />

  <el-result v-else-if="notFound || !detail" icon="warning" title="稿件不存在或未发布">
    <template #extra>
      <el-button type="primary" @click="router.push('/')"> 回首页 </el-button>
    </template>
  </el-result>

  <div v-else class="play-layout">
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
        <el-tag v-if="detail.copyright === 1" size="small" class="copyright-tag"> 自制 </el-tag>
      </p>

      <!-- 播放器：HLS 多清晰度（hls.js）+ 弹幕层 -->
      <div ref="playerBox" class="player-box">
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
        <span class="dm-tool" title="弹幕设置" @click="dmSettingsVisible = true">
          <span class="i-mingcute-settings-3-line" />
        </span>
        <span class="play-controls__spacer" />
        <!-- 倍速 -->
        <el-dropdown trigger="click" @command="setRate">
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
        <el-button type="primary" class="dm-send-btn" :loading="dmSending" @click="sendDanmaku">
          发送
        </el-button>
      </div>

      <!-- 发送工具条（模式 + 色板） -->
      <div v-if="userStore.token" class="dm-toolbar">
        <div class="flex items-center gap-1">
          <span
            v-for="m in [
              { v: 1, t: '滚动' },
              { v: 2, t: '顶部' },
              { v: 3, t: '底部' },
            ]"
            :key="m.v"
            class="dm-toolbar__mode"
            :class="{ 'is-active': dmMode === m.v, 'is-locked': m.v !== 1 && !isDmLevel3() }"
            :title="m.v !== 1 && !isDmLevel3() ? 'Lv3 解锁顶部/底部弹幕' : ''"
            @click="dmMode = isDmLevel3() || m.v === 1 ? (m.v as 1 | 2 | 3) : dmMode"
            >{{ m.t }}</span
          >
        </div>
        <div class="flex items-center gap-1">
          <span
            v-for="c in DM_COLORS"
            :key="c.value"
            class="dm-toolbar__color"
            :class="{
              'is-active': dmColor === c.value,
              'is-locked': c.value !== 0xffffff && !isDmLevel3(),
            }"
            :style="{ background: '#' + c.value.toString(16).padStart(6, '0') }"
            :title="c.value !== 0xffffff && !isDmLevel3() ? 'Lv3 解锁彩色弹幕' : c.name"
            @click="dmColor = isDmLevel3() || c.value === 0xffffff ? c.value : dmColor"
          />
        </div>
      </div>
      <p v-if="userStore.token && (userStore.profile?.level ?? 0) < 3" class="dm-privilege-tip">
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
          <span class="act-btn" :class="{ 'is-active': coined > 0 }" @click="openCoinPop">
            <span
              class="act-btn__icon"
              :class="coined > 0 ? 'i-mingcute-coin-2-fill' : 'i-mingcute-coin-2-line'"
            />
            <span class="act-btn__num">{{ formatCount(detail.stat.coin) }}</span>
          </span>
          <span v-if="coinPopVisible" class="coin-pop">
            <span class="coin-pop__title"
              >投给 UP 主（余额 {{ userStore.profile?.coin ?? 0 }}）</span
            >
            <span class="coin-pop__btns">
              <el-button size="small" type="primary" @click="doCoin(1)"> 投 1 枚 </el-button>
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
          <span class="act-btn" :class="{ 'is-active': faved }" @click="toggleFav">
            <span
              class="act-btn__icon"
              :class="faved ? 'i-mingcute-star-2-fill' : 'i-mingcute-star-2-line'"
            />
            <span class="act-btn__num">{{ formatCount(detail.stat.fav) }}</span>
          </span>
          <span v-if="favPopVisible" class="fav-pop">
            <span class="fav-pop__title">收藏到</span>
            <el-button v-for="col in collections" :key="col.id" size="small" @click="doFav(col.id)">
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
              <el-button size="small" @click="createCollection"> + </el-button>
            </span>
          </span>
        </span>
        <span class="act-btn" @click="openShare">
          <span class="act-btn__icon i-mingcute-share-forward-line" />
          <span class="act-btn__num">{{ formatCount(detail.stat.share) }}</span>
        </span>
      </div>

      <!-- 简介与标签 -->
      <div v-if="detail.description" class="play-desc" :class="{ 'is-expanded': descExpanded }">
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
        <el-tag v-for="t in detail.tags" :key="t" size="small" effect="plain">
          {{ t }}
        </el-tag>
      </div>
      <span class="play-report" @click="openReport">举报</span>

      <el-divider />
      <CommentSection :key="detail.bvid" :bvid="detail.bvid" />
    </div>

    <!-- 右：UP 主信息 / 分P / 弹幕列表 / 相关推荐（B 站风格侧栏） -->
    <VideoSidebar :detail="detail" :related="related" :player="player" :dm="dm" :acts="acts" />
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
  <DanmakuSettingsDialog :dm="dm" />
</template>

<style scoped lang="scss" src="./video-view.scss"></style>
