<script setup lang="ts">
// 开播管理页（M4-LIV-06，LIV-02 前端）：申请开播 → 房间设置 → 推流密钥（一次性展示）
// → 开播/下播 → 房管与禁言管理。PC 创作者侧。
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  ApiError,
  type LiveAdminItem,
  type LiveMuteItem,
  type LiveRoomInfo,
} from '@dlidli/api-client'
import { formatCount } from '@dlidli/shared'
import { api } from '@/api'
import defaultCover from '@/assets/default-cover.png'

type Stage = 'loading' | 'apply' | 'manage'

const stage = ref<Stage>('loading')
const room = ref<LiveRoomInfo | null>(null)
const pushUrl = ref('')

// ---- 房间设置 ----
const categories = ref<{ id: number; name: string }[]>([])
const form = ref({ title: '', cover: '', categoryId: undefined as number | undefined })
const savingInfo = ref(false)
const infoDirty = ref(false)

// ---- 推流密钥 ----
const streamKey = ref('') // 明文仅申请/重置接口返回，只保存在内存
const keyVisible = ref(false)

// ---- 房管/禁言 ----
const admins = ref<LiveAdminItem[]>([])
const mutes = ref<LiveMuteItem[]>([])
const adminUidDraft = ref('')
const muteUidDraft = ref('')
const muteMinutes = ref<number | undefined>(60)
const muteReason = ref('')

let onlineTimer: number | null = null

const isLive = computed(() => room.value?.status === 1)
const statusText = computed(
  () => ({ 0: '空闲（未开播）', 1: '直播中', 2: '本场已结束' })[room.value?.status ?? 0] ?? '',
)

async function loadMine() {
  stage.value = 'loading'
  try {
    const res = await api.live.myRoom()
    room.value = res.room
    pushUrl.value = res.push_url
    if (res.room) {
      form.value = {
        title: res.room.title,
        cover: res.room.cover,
        categoryId: res.room.category_id || undefined,
      }
      stage.value = 'manage'
      await loadCategories()
      await refreshModList()
      startPolling()
    } else {
      stage.value = 'apply'
    }
  } catch (err) {
    if (err instanceof ApiError && err.code === 91007) {
      stage.value = 'apply'
      await loadCategories()
    } else {
      ElMessage.error('加载失败，请重试')
      stage.value = 'apply'
    }
  }
}

async function loadCategories() {
  try {
    const list = await api.video.categories()
    categories.value = list.filter((c) => c.parent_id === 0)
  } catch {
    /* 分区可选，保留空态 */
  }
}

// ---- 申请开播（资格校验拦截在此显式呈现） ----
const applying = ref(false)
async function apply() {
  applying.value = true
  try {
    const res = await api.live.apply()
    room.value = res.room
    pushUrl.value = res.push_url
    if (res.stream_key) {
      streamKey.value = res.stream_key
      keyVisible.value = true
      ElMessage.success('开播申请成功，请保存推流密钥（仅显示一次）')
    }
    stage.value = 'manage'
    await loadCategories()
    await refreshModList()
    startPolling()
  } catch (err) {
    // 资格三闸（91001 白名单 / 91002 实名 / 91003 粉丝）/ 91004 已有房间，展示具体原因
    const reason = err instanceof ApiError ? err.message : '申请失败，请重试'
    ElMessage.error(reason)
  } finally {
    applying.value = false
  }
}

// ---- 房间设置 ----
async function saveInfo() {
  if (!room.value) return
  if (!form.value.title.trim() || !form.value.categoryId) {
    ElMessage.warning('请填写标题（1-80 字）并选择分区')
    return
  }
  savingInfo.value = true
  try {
    const info = await api.live.updateRoom({
      title: form.value.title.trim(),
      cover: form.value.cover,
      category_id: form.value.categoryId,
    })
    room.value = info
    infoDirty.value = false
    ElMessage.success('房间信息已保存')
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : '保存失败')
  } finally {
    savingInfo.value = false
  }
}

// ---- 开播/下播 ----
const toggling = ref(false)
async function startLive() {
  if (!form.value.title.trim() || !form.value.categoryId) {
    ElMessage.warning('开播前请先完善标题与分区')
    return
  }
  toggling.value = true
  try {
    if (infoDirty.value) await saveInfo()
    room.value = await api.live.start()
    ElMessage.success('已开播，房间已出现在直播首页')
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : '开播失败')
  } finally {
    toggling.value = false
  }
}

async function stopLive() {
  toggling.value = true
  try {
    room.value = await api.live.stop()
    ElMessage.success('已下播')
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : '下播失败')
  } finally {
    toggling.value = false
  }
}

// ---- 推流密钥 ----
async function resetKey() {
  try {
    await ElMessageBox.confirm('重置后旧密钥立即失效，OBS 需更新密钥。确认重置？', '重置推流密钥', {
      type: 'warning',
      confirmButtonText: '重置',
      cancelButtonText: '取消',
    })
  } catch {
    return
  }
  try {
    const res = await api.live.resetStreamKey()
    if (res.stream_key) {
      streamKey.value = res.stream_key
      keyVisible.value = true
      ElMessage.success('密钥已重置，旧密钥立即失效')
    }
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : '重置失败')
  }
}

async function copyText(text: string, label: string) {
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success(`${label}已复制`)
  } catch {
    ElMessage.error('复制失败，请手动选择复制')
  }
}

// ---- 房管 ----
async function refreshModList() {
  if (!room.value) return
  try {
    const [a, m] = await Promise.all([
      api.live.admins(room.value.id),
      api.live.mutes(room.value.id),
    ])
    admins.value = a.list
    mutes.value = m.list
  } catch {
    /* silent */
  }
}

async function addAdmin() {
  const uid = adminUidDraft.value.trim()
  if (!uid || !room.value) return
  try {
    await api.live.addAdmin(room.value.id, uid)
    ElMessage.success('已任命房管')
    adminUidDraft.value = ''
    await refreshModList()
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : '任命失败')
  }
}

async function removeAdmin(uid: string) {
  if (!room.value) return
  try {
    await api.live.removeAdmin(room.value.id, uid)
    ElMessage.success('已移除房管')
    await refreshModList()
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : '移除失败')
  }
}

// ---- 禁言 ----
async function addMute() {
  const uid = muteUidDraft.value.trim()
  const minutes = muteMinutes.value ?? 0
  if (!uid || !room.value) return
  if (minutes < 1 || minutes > 43200) {
    ElMessage.warning('禁言时长需在 1 分钟到 720 小时之间')
    return
  }
  try {
    await api.live.mute(room.value.id, uid, { minutes, reason: muteReason.value })
    ElMessage.success('已禁言')
    muteUidDraft.value = ''
    muteReason.value = ''
    await refreshModList()
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : '禁言失败')
  }
}

async function removeMute(uid: string) {
  if (!room.value) return
  try {
    await api.live.unmute(room.value.id, uid)
    ElMessage.success('已解除禁言')
    await refreshModList()
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : '解禁失败')
  }
}

function startPolling() {
  onlineTimer = window.setInterval(refreshModList, 20000)
}

onMounted(loadMine)
onBeforeUnmount(() => {
  if (onlineTimer) window.clearInterval(onlineTimer)
})
</script>

<template>
  <div class="live-studio">
    <div v-if="stage === 'loading'" class="live-studio__tip">加载中…</div>

    <!-- 未建房：申请开播 -->
    <div v-else-if="stage === 'apply'" class="live-studio__apply">
      <h2 class="live-studio__h2">
        <span class="i-mingcute-broadcast-line mr-2 text-primary" />申请开播
      </h2>
      <p class="live-studio__desc">
        开播需满足：<b>开播白名单</b> · <b>实名认证</b> · <b>粉丝 ≥ 100</b>。
        校验不通过会提示具体原因。
      </p>
      <el-button type="primary" size="large" round :loading="applying" @click="apply">
        立即申请开播
      </el-button>
    </div>

    <!-- 房间管理 -->
    <div v-else-if="room" class="live-studio__manage">
      <div class="live-studio__head">
        <h2 class="live-studio__h2">开播管理</h2>
        <span class="live-studio__status" :class="{ 'live-studio__status--live': isLive }">
          {{ statusText }}
        </span>
        <RouterLink v-if="isLive" :to="`/live/${room.id}`" class="live-studio__view-link">
          查看直播间 →
        </RouterLink>
      </div>

      <div class="live-studio__cols">
        <!-- 左列：房间设置 + 推流信息 + 开播控制 -->
        <div class="live-studio__col">
          <section class="live-studio__card">
            <h3 class="live-studio__h3">房间设置</h3>
            <el-form label-width="70px">
              <el-form-item label="标题" required>
                <el-input
                  v-model="form.title"
                  maxlength="80"
                  show-word-limit
                  placeholder="直播间标题（1-80 字）"
                  @input="infoDirty = true"
                />
              </el-form-item>
              <el-form-item label="封面">
                <el-input
                  v-model="form.cover"
                  placeholder="封面图 URL（可留空）"
                  @input="infoDirty = true"
                />
                <img
                  v-if="form.cover"
                  :src="form.cover"
                  class="live-studio__cover-preview"
                  alt="封面预览"
                  @error="($event.target as HTMLImageElement).src = defaultCover"
                />
              </el-form-item>
              <el-form-item label="分区" required>
                <el-select
                  v-model="form.categoryId"
                  placeholder="选择分区"
                  style="width: 240px"
                  @change="infoDirty = true"
                >
                  <el-option v-for="c in categories" :key="c.id" :label="c.name" :value="c.id" />
                </el-select>
              </el-form-item>
              <el-form-item>
                <el-button
                  type="primary"
                  :loading="savingInfo"
                  :disabled="!infoDirty"
                  @click="saveInfo"
                >
                  保存设置
                </el-button>
              </el-form-item>
            </el-form>
          </section>

          <section class="live-studio__card">
            <h3 class="live-studio__h3">推流信息（OBS）</h3>
            <div class="live-studio__kv">
              <span class="live-studio__kv-label">服务器</span>
              <span class="live-studio__kv-value">{{ pushUrl }}</span>
              <el-button size="small" @click="copyText(pushUrl, '推流地址')">复制</el-button>
            </div>
            <div class="live-studio__kv">
              <span class="live-studio__kv-label">串流密钥</span>
              <template v-if="keyVisible && streamKey">
                <span class="live-studio__kv-value live-studio__key">{{ streamKey }}</span>
                <el-button size="small" @click="copyText(streamKey, '密钥')">复制</el-button>
              </template>
              <span v-else class="live-studio__kv-value live-studio__key--hidden">
                ••••••••••••••••（已隐藏）
              </span>
            </div>
            <p class="live-studio__warn">
              ⚠️ 密钥仅申请/重置时显示一次，请立即保存。泄露可在「重置密钥」后立即失效旧密钥。
            </p>
            <el-button size="small" @click="resetKey">重置密钥</el-button>
          </section>

          <section class="live-studio__card">
            <h3 class="live-studio__h3">开播控制</h3>
            <div class="live-studio__live-ctrl">
              <el-button
                v-if="!isLive"
                type="primary"
                size="large"
                round
                :loading="toggling"
                @click="startLive"
              >
                开播
              </el-button>
              <el-button
                v-else
                type="danger"
                size="large"
                round
                :loading="toggling"
                @click="stopLive"
              >
                下播
              </el-button>
              <span v-if="isLive" class="live-studio__online-hint">
                当前在线 <b>{{ formatCount(room.online) }}</b> 人
              </span>
            </div>
          </section>
        </div>

        <!-- 右列：房管 + 禁言 -->
        <div class="live-studio__col">
          <section class="live-studio__card">
            <h3 class="live-studio__h3">房管（{{ admins.length }}）</h3>
            <div class="live-studio__add-row">
              <el-input v-model="adminUidDraft" placeholder="用户 UID" style="width: 160px" />
              <el-button type="primary" plain @click="addAdmin">任命</el-button>
            </div>
            <ul class="live-studio__list">
              <li v-for="a in admins" :key="a.user_id" class="live-studio__list-item">
                <span class="truncate">{{ a.nickname || `UID ${a.user_id}` }}</span>
                <el-button size="small" text type="danger" @click="removeAdmin(a.user_id)">
                  移除
                </el-button>
              </li>
              <li v-if="admins.length === 0" class="live-studio__list-empty">暂无房管</li>
            </ul>
          </section>

          <section class="live-studio__card">
            <h3 class="live-studio__h3">禁言列表（{{ mutes.length }}）</h3>
            <div class="live-studio__add-row">
              <el-input v-model="muteUidDraft" placeholder="用户 UID" style="width: 120px" />
              <el-input-number
                v-model="muteMinutes"
                :min="1"
                :max="43200"
                controls-position="right"
                style="width: 110px"
                placeholder="分钟"
              />
              <el-button type="primary" plain @click="addMute">禁言</el-button>
            </div>
            <el-input
              v-model="muteReason"
              placeholder="禁言原因（可选）"
              size="small"
              class="mb-2"
              maxlength="100"
            />
            <ul class="live-studio__list">
              <li v-for="m in mutes" :key="m.user_id" class="live-studio__list-item">
                <span class="truncate">
                  {{ m.nickname || `UID ${m.user_id}` }}
                  <small class="text-#c0c4cc">{{ m.reason || '—' }}</small>
                </span>
                <span class="live-studio__mute-until">
                  <el-button size="small" text type="primary" @click="removeMute(m.user_id)">
                    解禁
                  </el-button>
                </span>
              </li>
              <li v-if="mutes.length === 0" class="live-studio__list-empty">暂无禁言记录</li>
            </ul>
          </section>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/variables' as v;

.live-studio__tip {
  padding: 100px 0;
  text-align: center;
  color: v.$text-3;
}

.live-studio__apply {
  max-width: 560px;
  margin: 60px auto;
  padding: 40px;
  text-align: center;
  border: 1px solid v.$border;
  border-radius: 12px;
}

.live-studio__h2 {
  display: flex;
  align-items: center;
  justify-content: center;
  margin: 0 0 12px;
  font-size: 22px;
  font-weight: 700;
}

.live-studio__desc {
  margin-bottom: 24px;
  color: v.$text-2;
  font-size: 14px;
  line-height: 1.8;
}

.live-studio__head {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}

.live-studio__manage .live-studio__h2 {
  margin: 0;
}

.live-studio__status {
  padding: 2px 12px;
  border-radius: 12px;
  background: #f1f2f3;
  color: v.$text-2;
  font-size: 13px;

  &--live {
    background: rgba(v.$primary, 0.1);
    color: v.$primary;
    font-weight: 600;
  }
}

.live-studio__view-link {
  margin-left: auto;
  font-size: 13px;
  color: v.$primary;
  text-decoration: none;
}

.live-studio__cols {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
  align-items: start;

  @media (max-width: 960px) {
    grid-template-columns: 1fr;
  }
}

.live-studio__card {
  padding: 20px;
  margin-bottom: 16px;
  border: 1px solid v.$border;
  border-radius: 8px;
}

.live-studio__h3 {
  margin: 0 0 14px;
  font-size: 15px;
  font-weight: 600;
}

.live-studio__kv {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
  font-size: 13px;
}

.live-studio__kv-label {
  flex-shrink: 0;
  width: 62px;
  color: v.$text-3;
}

.live-studio__kv-value {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: v.$text-1;
  word-break: break-all;
}

.live-studio__key {
  font-family: monospace;
  letter-spacing: 1px;
  color: v.$primary;
  font-weight: 600;
}

.live-studio__key--hidden {
  color: v.$text-3;
}

.live-studio__warn {
  margin: 8px 0 12px;
  padding: 8px 12px;
  border-radius: 6px;
  background: #fdf6ec;
  color: #b88230;
  font-size: 12px;
  line-height: 1.6;
}

.live-studio__live-ctrl {
  display: flex;
  align-items: center;
  gap: 16px;
}

.live-studio__online-hint {
  font-size: 13px;
  color: v.$text-2;

  b {
    color: v.$primary;
  }
}

.live-studio__add-row {
  display: flex;
  gap: 8px;
  margin-bottom: 12px;
}

.live-studio__list {
  margin: 0;
  padding: 0;
  list-style: none;
  max-height: 220px;
  overflow-y: auto;
}

.live-studio__list-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 0;
  border-bottom: 1px solid #f5f5f5;
  font-size: 13px;

  &:last-child {
    border-bottom: none;
  }
}

.live-studio__list-empty {
  padding: 16px 0;
  text-align: center;
  color: v.$text-3;
  font-size: 13px;
}

.live-studio__cover-preview {
  margin-top: 8px;
  max-width: 240px;
  max-height: 135px;
  border-radius: 6px;
  object-fit: cover;
}
</style>
