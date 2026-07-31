<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ApiError } from '@dlidli/api-client'
import { api } from '@/api'
import { useCountdown } from '@/composables/useCountdown'

const router = useRouter()

const form = reactive({ phone: '', code: '', password: '', confirm: '' })
const sending = ref(false)
const submitting = ref(false)
const { count: countdown, start: startCountdown } = useCountdown(60)

async function sendCode() {
  if (!/^1\d{10}$/.test(form.phone)) {
    ElMessage.warning('请输入正确的手机号')
    return
  }
  sending.value = true
  try {
    const res = await api.auth.sendSmsCode(form.phone)
    ElMessage.success('验证码已发送')
    if (res.debug_code) form.code = res.debug_code // dev 联调便利
    startCountdown()
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : '发送失败')
  } finally {
    sending.value = false
  }
}


async function onSubmit() {
  if (!/^1\d{10}$/.test(form.phone) || !form.code) {
    ElMessage.warning('请填写手机号和验证码')
    return
  }
  if (form.password.length < 8 || form.password.length > 32) {
    ElMessage.warning('新密码长度需 8~32 位')
    return
  }
  if (form.password !== form.confirm) {
    ElMessage.warning('两次输入的密码不一致')
    return
  }
  submitting.value = true
  try {
    await api.auth.resetPassword(form.phone, form.code, form.password)
    ElMessage.success('密码已重置，请使用新密码登录')
    router.push('/login')
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : '重置失败')
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="reset flex items-start justify-center pt-15">
    <el-card class="w-420px rounded-12px">
      <h2 class="m-0 text-primary">
        找回密码
      </h2>
      <p class="mt-1.5 mb-4.5 text-3.25 text-text-2">
        通过注册手机号验证身份后重置密码
      </p>
      <el-form
        label-position="top"
        size="large"
        @submit.prevent="onSubmit"
      >
        <el-form-item label="手机号">
          <el-input
            v-model="form.phone"
            maxlength="11"
            placeholder="注册时使用的手机号"
          />
        </el-form-item>
        <el-form-item label="验证码">
          <div class="flex gap-2.5 w-full">
            <el-input
              v-model="form.code"
              maxlength="6"
              placeholder="6 位验证码"
            />
            <el-button
              :disabled="countdown > 0"
              :loading="sending"
              @click="sendCode"
            >
              {{ countdown > 0 ? `${countdown}s` : '获取验证码' }}
            </el-button>
          </div>
        </el-form-item>
        <el-form-item label="新密码">
          <el-input
            v-model="form.password"
            type="password"
            show-password
            placeholder="8~32 位"
            autocomplete="new-password"
          />
        </el-form-item>
        <el-form-item label="确认密码">
          <el-input
            v-model="form.confirm"
            type="password"
            show-password
            placeholder="再次输入新密码"
            autocomplete="new-password"
          />
        </el-form-item>
        <el-button
          type="primary"
          size="large"
          class="reset__submit"
          :loading="submitting"
          native-type="submit"
        >
          重置密码
        </el-button>
        <el-button
          link
          class="reset__back"
          @click="router.push('/login')"
        >
          返回登录
        </el-button>
      </el-form>
    </el-card>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/variables' as v;

.reset {
  min-height: calc(100vh - 64px);
}

// 主按钮品牌色（变量覆盖 Element Plus）
.reset__submit {
  width: 100%;
  --el-button-bg-color: #{v.$primary};
  --el-button-border-color: #{v.$primary};
  --el-button-hover-bg-color: #{v.$primary-hover};
  --el-button-hover-border-color: #{v.$primary-hover};
}

.reset__back {
  width: 100%;
  margin: 10px 0 0 !important;
}
</style>
