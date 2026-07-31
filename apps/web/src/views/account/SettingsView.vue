<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { ApiError } from '@dlidli/api-client'
import { api } from '@/api'
import { useUserStore } from '@/stores/user'
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
</script>

<template>
  <div class="mx-auto max-w-160">
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
