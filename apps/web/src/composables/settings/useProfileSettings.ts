import { reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { ApiError } from '@dlidli/api-client'
import { api } from '@/api'
import { useUserStore } from '@/stores/user'

/**
 * 基本资料设置（SettingsView 拆分，M3-ENG-12）：资料表单回填 / 头像上传 / 资料保存。
 */
export function useProfileSettings() {
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

  return { form, saving, uploading, fileInput, pickAvatar, onAvatarChange, onSave }
}
