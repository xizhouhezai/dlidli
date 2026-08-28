<script setup lang="ts">
import { onMounted } from 'vue'
import { useUserStore } from '@/stores/user'
import AccountStatusAlert from '@/components/AccountStatusAlert.vue'
import defaultAvatar from '@/assets/default-avatar.png'
import { useProfileSettings } from '@/composables/settings/useProfileSettings'
import { usePasswordChange } from '@/composables/settings/usePasswordChange'
import { useYouthMode } from '@/composables/settings/useYouthMode'
import { useDmBlocks } from '@/composables/settings/useDmBlocks'
import { useRecommendSetting } from '@/composables/settings/useRecommendSetting'

const userStore = useUserStore()

// —— 拆分出的子模块（M3-ENG-12）：资料 / 密码 / 青少年模式 / 弹幕屏蔽 / 推荐开关 ——
const profileApi = useProfileSettings()
const pwdApi = usePasswordChange()
const youthApi = useYouthMode()
const dmApi = useDmBlocks()
const recApi = useRecommendSetting()

const { form, saving, uploading, fileInput, pickAvatar, onAvatarChange, onSave } = profileApi
const { pwdForm, pwdSaving, onChangePassword } = pwdApi
const { youthMode, youthSaving, loadYouthMode, toggleYouthMode } = youthApi
const {
  dmBlocks,
  dmBlockLoading,
  dmBlockInput,
  dmBlockAdding,
  loadDmBlocks,
  addDmBlock,
  removeDmBlock,
  clearDmBlockHash,
} = dmApi
const { recEnabled, recSaving, loadRecommendSetting, toggleRecommend } = recApi

// 模板 ref 绑定：vue-tsc 对解构变量不识别模板引用，此处显式登记避免误报未使用
void fileInput

onMounted(() => {
  void loadYouthMode()
  void loadDmBlocks()
  void loadRecommendSetting()
})
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
          <el-button :loading="uploading" @click="pickAvatar"> 更换头像 </el-button>
          <p class="mt-2 mb-0 text-3 text-text-2">支持 jpg / png / webp，2MB 以内</p>
          <input
            ref="fileInput"
            type="file"
            accept="image/jpeg,image/png,image/webp"
            hidden
            @change="onAvatarChange"
          />
        </div>
      </div>

      <el-divider />

      <!-- 基本资料 -->
      <el-form label-width="72px" class="max-w-480px">
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
            <el-radio :value="0"> 保密 </el-radio>
            <el-radio :value="1"> 男 </el-radio>
            <el-radio :value="2"> 女 </el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" class="save-btn" :loading="saving" @click="onSave">
            保存修改
          </el-button>
        </el-form-item>
      </el-form>

      <el-divider />

      <!-- 修改密码 -->
      <el-form label-width="72px" class="max-w-480px">
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
          <p class="m-0 font-600">青少年模式</p>
          <p class="mt-1 mb-0 text-3 text-text-2">
            开启后每日累计使用 40 分钟将收到提醒，请合理安排上网时间
          </p>
        </div>
        <el-switch v-model="youthMode" :loading="youthSaving" @change="toggleYouthMode" />
      </div>

      <el-divider />

      <!-- 弹幕屏蔽 -->
      <div class="max-w-480px">
        <p class="m-0 font-600">弹幕屏蔽</p>
        <p class="mt-1 mb-2 text-3 text-text-2">屏蔽词与屏蔽用户的弹幕将不再显示（跨设备生效）</p>
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
        <div v-loading="dmBlockLoading" class="flex flex-wrap gap-1.5 min-h-8">
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
            >暂无屏蔽词</span
          >
        </div>
        <template v-if="dmBlocks.some((x) => x.block_type === 2)">
          <p class="mt-3 mb-1.5 text-3 text-text-2">已屏蔽用户</p>
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
          <p class="m-0 font-600">个性化推荐</p>
          <p class="mt-1 mb-0 text-3 text-text-2">
            关闭后首页推荐将仅展示热门内容，不再根据你的观看行为个性化
          </p>
        </div>
        <el-switch v-model="recEnabled" :loading="recSaving" @change="toggleRecommend" />
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
