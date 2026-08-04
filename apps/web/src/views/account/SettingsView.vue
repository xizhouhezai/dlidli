<script setup lang="ts">
import { onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ApiError, type DanmakuBlockItem } from '@dlidli/api-client'
import { api } from '@/api'
import { useUserStore } from '@/stores/user'
import AccountStatusAlert from '@/components/AccountStatusAlert.vue'
import defaultAvatar from '@/assets/default-avatar.png'

const userStore = useUserStore()

const form = reactive({
  nickname: '',
  signature: '',
  gender: 0 as 0 | 1 | 2,
})
const saving = ref(false)
const uploading = ref(false)
const fileInput = ref<HTMLInputElement>()

// 资料就绪后回填表单（页面刷新时 profile 异步加载）
watch(
  () => userStore.profile,
  (p) => {
    if (!p) return
    form.nickname = p.nickname
    form.signature = p.signature
    form.gender = p.gender
  },
  { immediate: true },
)

function pickAvatar() {
  fileInput.value?.click()
}

async function onAvatarChange(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  if (file.size > 2 * 1024 * 1024) {
    ElMessage.warning('头像大小须在 2MB 以内')
    return
  }
  uploading.value = true
  try {
    const { avatar } = await api.auth.uploadAvatar(file)
    if (userStore.profile) userStore.profile.avatar = avatar
    ElMessage.success('头像已更新')
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : '上传失败，请稍后再试')
  } finally {
    uploading.value = false
    if (fileInput.value) fileInput.value.value = ''
  }
}

async function onSave() {
  saving.value = true
  try {
    const updated = await api.auth.updateProfile({
      nickname: form.nickname,
      signature: form.signature,
      gender: form.gender,
    })
    userStore.profile = updated
    ElMessage.success('资料已保存')
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : '保存失败，请稍后再试')
  } finally {
    saving.value = false
  }
}

// 修改密码
const pwdForm = reactive({ old: '', next: '', confirm: '' })
const pwdSaving = ref(false)

async function onChangePassword() {
  if (pwdForm.next.length < 8 || pwdForm.next.length > 32) {
    ElMessage.warning('新密码长度需 8~32 位')
    return
  }
  if (pwdForm.next !== pwdForm.confirm) {
    ElMessage.warning('两次输入的新密码不一致')
    return
  }
  pwdSaving.value = true
  try {
    await api.auth.changePassword(pwdForm.old, pwdForm.next)
    pwdForm.old = ''
    pwdForm.next = ''
    pwdForm.confirm = ''
    ElMessage.success('密码已更新，下次登录请使用新密码')
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : '修改失败，请稍后再试')
  } finally {
    pwdSaving.value = false
  }
}

// 青少年模式（M2-AUD-04）：开关 + 每日 40 分钟使用提醒（本地计时）
const YOUTH_LIMIT_MIN = 40
const youthMode = ref(false)
const youthSaving = ref(false)
let youthTimer: ReturnType<typeof setInterval> | null = null

function youthUsageKey() {
  return `youth_usage_${new Date().toISOString().slice(0, 10)}`
}

async function loadYouthMode() {
  try {
    youthMode.value = (await api.auth.youthMode()).enabled
    if (youthMode.value) startYouthTimer()
  } catch {
    // 静默失败，保持关闭态
  }
}

async function toggleYouthMode() {
  youthSaving.value = true
  try {
    await api.auth.setYouthMode(youthMode.value)
    if (youthMode.value) {
      localStorage.setItem(youthUsageKey(), '0') // 开启当日重新计时
      startYouthTimer()
    } else {
      stopYouthTimer()
    }
    ElMessage.success(youthMode.value ? '已开启青少年模式' : '已关闭青少年模式')
  } catch (err) {
    youthMode.value = !youthMode.value
    ElMessage.error(err instanceof ApiError ? err.message : '操作失败')
  } finally {
    youthSaving.value = false
  }
}

function startYouthTimer() {
  stopYouthTimer()
  youthTimer = setInterval(() => {
    const key = youthUsageKey()
    const used = Number(localStorage.getItem(key) ?? '0')
    const next = used + 1
    localStorage.setItem(key, String(next))
    if (next >= YOUTH_LIMIT_MIN) {
      stopYouthTimer()
      ElMessage.warning('今日青少年模式使用时长已达 40 分钟，请注意休息')
    }
  }, 60_000)
}

function stopYouthTimer() {
  if (youthTimer) {
    clearInterval(youthTimer)
    youthTimer = null
  }
}

onMounted(() => {
  loadYouthMode()
  loadDmBlocks()
  loadRecommendSetting()
})
onUnmounted(stopYouthTimer)

// 弹幕屏蔽管理（M2-DM-02）：关键词 + 屏蔽用户
const dmBlocks = ref<DanmakuBlockItem[]>([])
const dmBlockLoading = ref(false)
const dmBlockInput = ref('')
const dmBlockAdding = ref(false)

async function loadDmBlocks() {
  dmBlockLoading.value = true
  try {
    dmBlocks.value = (await api.danmaku.blocks()).list
  } catch {
    // 静默失败
  } finally {
    dmBlockLoading.value = false
  }
}

async function addDmBlock() {
  const kw = dmBlockInput.value.trim()
  if (!kw) return
  dmBlockAdding.value = true
  try {
    await api.danmaku.addBlock({ block_type: 1, keyword: kw })
    dmBlockInput.value = ''
    ElMessage.success('已添加屏蔽词')
    await loadDmBlocks()
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : '操作失败')
  } finally {
    dmBlockAdding.value = false
  }
}

async function removeDmBlock(b: DanmakuBlockItem) {
  try {
    await api.danmaku.deleteBlock(b.id)
    dmBlocks.value = dmBlocks.value.filter((x) => x.id !== b.id)
    ElMessage.success('已删除')
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : '删除失败')
  }
}

async function clearDmBlockHash(hash: string) {
  await ElMessageBox.confirm('确定解除对该用户的弹幕屏蔽吗？', '解除屏蔽', { type: 'warning' })
  const b = dmBlocks.value.find((x) => x.block_type === 2 && x.block_hash === hash)
  if (b) await removeDmBlock(b)
}

// 个性化推荐开关（M3-REC-07 合规）
const recEnabled = ref(true)
const recSaving = ref(false)

async function loadRecommendSetting() {
  try {
    recEnabled.value = (await api.recommend.recommendSetting()).enabled
  } catch {
    // 静默失败
  }
}

async function toggleRecommend() {
  recSaving.value = true
  try {
    await api.recommend.setRecommendSetting(recEnabled.value)
    ElMessage.success(recEnabled.value ? '已开启个性化推荐' : '已关闭个性化推荐（首页推荐将展示热门内容）')
  } catch (err) {
    recEnabled.value = !recEnabled.value
    ElMessage.error(err instanceof ApiError ? err.message : '操作失败')
  } finally {
    recSaving.value = false
  }
}
</script>

<template>
  <div class="mx-auto max-w-160">
    <!-- 账号状态（禁言/封禁）提示 -->
    <AccountStatusAlert :user="userStore.profile" />

    <el-card shadow="never">
      <template #header>
        <span>账号设置</span>
      </template>

      <!-- 头像 -->
      <div class="flex items-center gap-6">
        <el-avatar
          :size="80"
          :src="userStore.profile?.avatar || defaultAvatar"
          class="shrink-0 bg-primary text-white text-7 font-600"
        >
          {{ userStore.profile?.nickname?.slice(0, 1) ?? 'U' }}
        </el-avatar>
        <div>
          <el-button
            :loading="uploading"
            @click="pickAvatar"
          >
            更换头像
          </el-button>
          <p class="mt-2 mb-0 text-3 text-text-2">
            支持 jpg / png / webp，2MB 以内
          </p>
          <input
            ref="fileInput"
            type="file"
            accept="image/jpeg,image/png,image/webp"
            hidden
            @change="onAvatarChange"
          >
        </div>
      </div>

      <el-divider />

      <!-- 基本资料 -->
      <el-form
        label-width="72px"
        class="max-w-480px"
      >
        <el-form-item label="昵称">
          <el-input
            v-model="form.nickname"
            maxlength="24"
            show-word-limit
            placeholder="2-24 个字符"
          />
        </el-form-item>
        <el-form-item label="签名">
          <el-input
            v-model="form.signature"
            type="textarea"
            :rows="3"
            maxlength="200"
            show-word-limit
            placeholder="介绍一下自己吧"
          />
        </el-form-item>
        <el-form-item label="性别">
          <el-radio-group v-model="form.gender">
            <el-radio :value="0">
              保密
            </el-radio>
            <el-radio :value="1">
              男
            </el-radio>
            <el-radio :value="2">
              女
            </el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item>
          <el-button
            type="primary"
            class="save-btn"
            :loading="saving"
            @click="onSave"
          >
            保存修改
          </el-button>
        </el-form-item>
      </el-form>

      <el-divider />

      <!-- 修改密码 -->
      <el-form
        label-width="72px"
        class="max-w-480px"
      >
        <el-form-item label="旧密码">
          <el-input
            v-model="pwdForm.old"
            type="password"
            show-password
            placeholder="首次设置密码可留空"
            autocomplete="current-password"
          />
        </el-form-item>
        <el-form-item label="新密码">
          <el-input
            v-model="pwdForm.next"
            type="password"
            show-password
            placeholder="8~32 位"
            autocomplete="new-password"
          />
        </el-form-item>
        <el-form-item label="确认密码">
          <el-input
            v-model="pwdForm.confirm"
            type="password"
            show-password
            placeholder="再次输入新密码"
            autocomplete="new-password"
          />
        </el-form-item>
        <el-form-item>
          <el-button
            type="primary"
            class="save-btn"
            :loading="pwdSaving"
            :disabled="!pwdForm.next"
            @click="onChangePassword"
          >
            更新密码
          </el-button>
        </el-form-item>
      </el-form>

      <el-divider />

      <!-- 青少年模式 -->
      <div class="flex items-center justify-between max-w-480px">
        <div>
          <p class="m-0 font-600">
            青少年模式
          </p>
          <p class="mt-1 mb-0 text-3 text-text-2">
            开启后每日累计使用 40 分钟将收到提醒，请合理安排上网时间
          </p>
        </div>
        <el-switch
          v-model="youthMode"
          :loading="youthSaving"
          @change="toggleYouthMode"
        />
      </div>

      <el-divider />

      <!-- 弹幕屏蔽 -->
      <div class="max-w-480px">
        <p class="m-0 font-600">
          弹幕屏蔽
        </p>
        <p class="mt-1 mb-2 text-3 text-text-2">
          屏蔽词与屏蔽用户的弹幕将不再显示（跨设备生效）
        </p>
        <div class="flex gap-2 mb-3">
          <el-input
            v-model="dmBlockInput"
            class="max-w-280px"
            placeholder="输入要屏蔽的关键词"
            maxlength="64"
            @keyup.enter="addDmBlock"
          />
          <el-button
            type="primary"
            class="save-btn"
            :loading="dmBlockAdding"
            :disabled="!dmBlockInput.trim()"
            @click="addDmBlock"
          >
            添加
          </el-button>
        </div>
        <div
          v-loading="dmBlockLoading"
          class="flex flex-wrap gap-1.5 min-h-8"
        >
          <el-tag
            v-for="b in dmBlocks.filter((x) => x.block_type === 1)"
            :key="b.id"
            closable
            effect="plain"
            @close="removeDmBlock(b)"
          >
            {{ b.keyword }}
          </el-tag>
          <span
            v-if="!dmBlockLoading && dmBlocks.filter((x) => x.block_type === 1).length === 0"
            class="text-3 text-text-3 self-center"
          >暂无屏蔽词</span>
        </div>
        <template v-if="dmBlocks.some((x) => x.block_type === 2)">
          <p class="mt-3 mb-1.5 text-3 text-text-2">
            已屏蔽用户
          </p>
          <div class="flex flex-wrap gap-1.5">
            <el-tag
              v-for="b in dmBlocks.filter((x) => x.block_type === 2)"
              :key="b.id"
              closable
              type="info"
              effect="plain"
              @close="clearDmBlockHash(b.block_hash ?? '')"
            >
              {{ b.block_hash?.slice(0, 8) }}…
            </el-tag>
          </div>
        </template>
      </div>

      <el-divider />

      <!-- 个性化推荐 -->
      <div class="flex items-center justify-between max-w-480px">
        <div>
          <p class="m-0 font-600">
            个性化推荐
          </p>
          <p class="mt-1 mb-0 text-3 text-text-2">
            关闭后首页推荐将仅展示热门内容，不再根据你的观看行为个性化
          </p>
        </div>
        <el-switch
          v-model="recEnabled"
          :loading="recSaving"
          @change="toggleRecommend"
        />
      </div>
    </el-card>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/variables' as v;

// 主按钮品牌色（变量覆盖 Element Plus）
.save-btn {
  --el-button-bg-color: #{v.$primary};
  --el-button-border-color: #{v.$primary};
  --el-button-hover-bg-color: #{v.$primary-hover};
  --el-button-hover-border-color: #{v.$primary-hover};
}
</style>
