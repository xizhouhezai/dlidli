<script setup lang="ts">
import { onMounted, reactive, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ApiError } from '@dlidli/api-client'
import { api } from '@/api'
import { useUserStore } from '@/stores/user'
import { useCountdown } from '@/composables/useCountdown'

const router = useRouter()
const route = useRoute()
const userStore = useUserStore()

const tab = ref<'password' | 'sms' | 'register'>('password')
const loading = ref(false)

// 邮箱注册（ACC-02）表单与激活流程
const regForm = reactive({ email: '', password: '', confirm: '', inviteCode: '' })
const activateDialog = ref(false)
const activateToken = ref('')
const activateLoading = ref(false)
const pendingEmail = ref('')
const debugActivateUrl = ref('')
const regSubmitted = ref(false)

const pwdForm = reactive({ account: '', password: '' })
const smsForm = reactive({ phone: '', code: '' })

// 图形验证码
const captchaId = ref('')
const captchaSvg = ref('')
const captchaCode = ref('')

async function loadCaptcha() {
  try {
    const res = await api.auth.captcha()
    captchaId.value = res.id
    captchaSvg.value = res.svg
    captchaCode.value = ''
  } catch {
    // 静默失败，用户可点刷新
  }
}

onMounted(loadCaptcha)
watch(tab, (t) => {
  if (t === 'password') loadCaptcha()
})

// 验证码倒计时
const { count: countdown, start: startCountdown } = useCountdown(60)

// 背景漂浮弹幕（负延迟让首屏即有弹幕在途中）
const danmakus = [
  { text: '前方高能预警！', top: '6%', duration: 22, delay: -18 },
  { text: '一键三连支持一下～', top: '14%', duration: 26, delay: -6 },
  { text: 'AWSL 这也太可爱了', top: '22%', duration: 20, delay: -12 },
  { text: '泪目 QAQ', top: '30%', duration: 24, delay: -2 },
  { text: '爷青回！！！', top: '38%', duration: 19, delay: -15 },
  { text: '多谢款待 (＾▽＾)', top: '52%', duration: 27, delay: -9 },
  { text: '此生无悔入 DliDli', top: '62%', duration: 21, delay: -20 },
  { text: '弹幕护体！', top: '72%', duration: 25, delay: -4 },
  { text: '收藏从未停止，学习从未开始', top: '82%', duration: 23, delay: -14 },
  { text: '下次一定', top: '90%', duration: 18, delay: -8 },
]

async function sendCode() {
  if (!/^1\d{10}$/.test(smsForm.phone)) {
    ElMessage.warning('请输入正确的手机号')
    return
  }
  try {
    const res = await api.auth.sendSmsCode(smsForm.phone)
    ElMessage.success('验证码已发送')
    // dev 环境后端返回 debug_code，自动填充便于联调
    if (res.debug_code) smsForm.code = res.debug_code
    startCountdown()
  } catch (e) {
    ElMessage.error(e instanceof ApiError ? e.message : '发送失败，请稍后再试')
  }
}

async function onRegister() {
  const email = regForm.email.trim()
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
    ElMessage.warning('请输入正确的邮箱')
    return
  }
  if (regForm.password.length < 6 || regForm.password !== regForm.confirm) {
    ElMessage.warning('密码至少 6 位且两次输入需一致')
    return
  }
  loading.value = true
  try {
    const res = await api.auth.registerEmail(email, regForm.password, regForm.inviteCode.trim())
    pendingEmail.value = email
    regSubmitted.value = true
    // dev 环境返回 debug 激活链接（mock 邮件）；生产环境提示查收邮箱
    debugActivateUrl.value = res.debug_activate_url ?? ''
    activateToken.value = ''
    ElMessage.success('注册成功，请激活邮箱账号')
    activateDialog.value = true
  } catch (e) {
    ElMessage.error(e instanceof ApiError ? e.message : '注册失败，请稍后再试')
  } finally {
    loading.value = false
  }
}

async function onActivate() {
  const token = (activateToken.value || debugActivateUrl.value.split('token=')[1] || '').trim()
  if (!token) {
    ElMessage.warning('请输入激活链接中的 token')
    return
  }
  activateLoading.value = true
  try {
    await api.auth.activate(token)
    ElMessage.success('激活成功，请使用邮箱 + 密码登录')
    activateDialog.value = false
    // 切回密码登录并预填邮箱
    pwdForm.account = pendingEmail.value
    tab.value = 'password'
    loadCaptcha()
  } catch (e) {
    ElMessage.error(e instanceof ApiError ? e.message : '激活失败，请检查链接是否有效')
  } finally {
    activateLoading.value = false
  }
}

async function onSubmit() {
  loading.value = true
  try {
    if (tab.value === 'password') {
      if (!pwdForm.account || !pwdForm.password) {
        ElMessage.warning('请输入账号和密码')
        return
      }
      if (!captchaCode.value) {
        ElMessage.warning('请输入验证码')
        return
      }
      await userStore.loginByPassword(
        pwdForm.account,
        pwdForm.password,
        captchaId.value,
        captchaCode.value,
      )
    } else {
      if (!smsForm.phone || !smsForm.code) {
        ElMessage.warning('请输入手机号和验证码')
        return
      }
      await userStore.loginBySms(smsForm.phone, smsForm.code)
    }
    ElMessage.success(`欢迎回来，${userStore.profile?.nickname}`)
    // 登录后回跳被拦截前的页面（守卫带来的 redirect）
    const redirect = route.query.redirect
    router.push(typeof redirect === 'string' && redirect.startsWith('/') ? redirect : '/')
  } catch (e) {
    ElMessage.error(e instanceof ApiError ? e.message : '登录失败，请稍后再试')
    if (tab.value === 'password') loadCaptcha() // 失败刷新验证码
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-page">
    <!-- 动态背景层 -->
    <div class="bg-blob bg-blob--pink" />
    <div class="bg-blob bg-blob--blue" />
    <div class="bg-blob bg-blob--peach" />
    <div
      v-for="(d, i) in danmakus"
      :key="i"
      class="bg-danmaku"
      :style="{
        top: d.top,
        animationDuration: d.duration + 's',
        animationDelay: d.delay + 's',
      }"
    >
      {{ d.text }}
    </div>

    <!-- 返回首页 -->
    <RouterLink to="/" class="back-home"> ← 返回首页 </RouterLink>

    <!-- 登录卡片 -->
    <el-card class="login-card" shadow="always">
      <div class="login-brand">
        <span class="login-logo">DliDli</span>
        <p class="login-slogan">你感兴趣的视频都在 DliDli</p>
      </div>

      <el-tabs v-model="tab" class="login-tabs" stretch>
        <el-tab-pane label="密码登录" name="password">
          <el-form label-position="top" @submit.prevent="onSubmit">
            <el-form-item>
              <el-input v-model="pwdForm.account" placeholder="手机号 / 邮箱" size="large" />
            </el-form-item>
            <el-form-item>
              <el-input
                v-model="pwdForm.password"
                type="password"
                placeholder="密码"
                size="large"
                show-password
              />
            </el-form-item>
            <el-form-item>
              <div class="captcha-row">
                <el-input v-model="captchaCode" placeholder="验证码" size="large" maxlength="4" />
                <!-- eslint-disable vue/no-v-html -- 内联自家后端生成的验证码 SVG，可控 -->
                <span
                  class="captcha-img"
                  title="点击刷新"
                  @click="loadCaptcha"
                  v-html="captchaSvg"
                />
                <!-- eslint-enable vue/no-v-html -->
              </div>
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <el-tab-pane label="短信登录" name="sms">
          <el-form label-position="top" @submit.prevent="onSubmit">
            <el-form-item>
              <el-input
                v-model="smsForm.phone"
                placeholder="手机号（未注册将自动注册）"
                size="large"
                maxlength="11"
              />
            </el-form-item>
            <el-form-item>
              <div class="code-row">
                <el-input
                  v-model="smsForm.code"
                  placeholder="6 位验证码"
                  size="large"
                  maxlength="6"
                />
                <el-button size="large" :disabled="countdown > 0" @click="sendCode">
                  {{ countdown > 0 ? `${countdown}s` : '获取验证码' }}
                </el-button>
              </div>
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <el-tab-pane label="邮箱注册" name="register">
          <el-form label-position="top" @submit.prevent="onRegister">
            <el-form-item>
              <el-input v-model="regForm.email" placeholder="邮箱" size="large" type="email" />
            </el-form-item>
            <el-form-item>
              <el-input
                v-model="regForm.password"
                type="password"
                placeholder="设置密码（6-64 位）"
                size="large"
                show-password
              />
            </el-form-item>
            <el-form-item>
              <el-input
                v-model="regForm.confirm"
                type="password"
                placeholder="确认密码"
                size="large"
                show-password
              />
            </el-form-item>
            <el-form-item>
              <el-input v-model="regForm.inviteCode" placeholder="邀请码（选填，内测开启时必填）" size="large" maxlength="16" />
            </el-form-item>
          </el-form>
        </el-tab-pane>
      </el-tabs>

      <el-button
        type="primary"
        size="large"
        class="login-btn"
        :loading="loading"
        @click="tab === 'register' ? onRegister() : onSubmit()"
      >
        {{ tab === 'register' ? '注册' : '登录' }}
      </el-button>
      <p class="login-tip">
        首次使用推荐「短信登录」，未注册手机号将自动创建账号
        <RouterLink to="/reset-password" class="login-forgot"> 忘记密码？ </RouterLink>
      </p>
      <!-- 邮箱激活对话框（ACC-02） -->
      <el-dialog
        v-model="activateDialog"
        title="激活邮箱账号"
        width="440px"
        :close-on-click-modal="false"
      >
        <p v-if="debugActivateUrl" class="activate-tip">
          （dev 模式 mock 邮件）激活链接已生成，可直接复制 token 或点击下方按钮：
        </p>
        <p v-else class="activate-tip">激活邮件已发送至 {{ pendingEmail }}，请查收并输入链接中的 token。</p>
        <el-input v-model="activateToken" placeholder="激活 token" size="large" />
        <div class="activate-actions">
          <el-button size="large" @click="activateDialog = false">稍后激活</el-button>
          <el-button
            type="primary"
            size="large"
            :loading="activateLoading"
            @click="onActivate"
          >
            立即激活
          </el-button>
        </div>
      </el-dialog>
    </el-card>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/variables' as v;

.login-page {
  position: relative;
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  background: linear-gradient(135deg, #ffeef4 0%, #fff7fa 35%, #eef6ff 70%, #ffeef4 100%);
  background-size: 300% 300%;
  animation: gradient-shift 18s ease infinite;
}

@keyframes gradient-shift {
  0%,
  100% {
    background-position: 0% 50%;
  }

  50% {
    background-position: 100% 50%;
  }
}

/* 柔光光斑 */
.bg-blob {
  position: absolute;
  border-radius: 50%;
  filter: blur(80px);
  opacity: 0.55;
  animation: blob-float 14s ease-in-out infinite;
}

.bg-blob--pink {
  width: 420px;
  height: 420px;
  left: -120px;
  top: -80px;
  background: v.$primary;
}

.bg-blob--blue {
  width: 380px;
  height: 380px;
  right: -100px;
  bottom: -60px;
  background: #7ac9ff;
  animation-delay: -5s;
}

.bg-blob--peach {
  width: 260px;
  height: 260px;
  right: 18%;
  top: -100px;
  background: #ffc37a;
  opacity: 0.35;
  animation-delay: -9s;
}

@keyframes blob-float {
  0%,
  100% {
    transform: translate(0, 0) scale(1);
  }

  33% {
    transform: translate(40px, 30px) scale(1.08);
  }

  66% {
    transform: translate(-30px, 20px) scale(0.95);
  }
}

/* 漂浮弹幕 */
.bg-danmaku {
  position: absolute;
  left: 100%;
  padding: 6px 16px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.75);
  color: #9499a0;
  font-size: 14px;
  white-space: nowrap;
  box-shadow: 0 2px 8px rgba(251, 114, 153, 0.08);
  animation-name: danmaku-fly;
  animation-timing-function: linear;
  animation-iteration-count: infinite;
  pointer-events: none;
  user-select: none;
}

@keyframes danmaku-fly {
  from {
    transform: translateX(0);
  }

  to {
    transform: translateX(calc(-100vw - 100%));
  }
}

/* 返回首页 */
.back-home {
  position: absolute;
  left: 24px;
  top: 20px;
  z-index: 2;
  font-size: 14px;
  color: var(--dli-text-2);
  transition: color 0.15s;
}

.back-home:hover {
  color: var(--dli-primary);
}

.login-forgot {
  margin-left: 8px;
  color: var(--dli-primary);
  text-decoration: none;
}

.login-forgot:hover {
  text-decoration: underline;
}

/* 登录卡片 */
.login-card {
  position: relative;
  z-index: 1;
  width: 420px;
  border-radius: 16px;
  border: none;
  background: rgba(255, 255, 255, 0.88);
  backdrop-filter: blur(12px);
  animation: card-in 0.5s ease both;
}

@keyframes card-in {
  from {
    opacity: 0;
    transform: translateY(24px) scale(0.98);
  }

  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

.login-brand {
  text-align: center;
  margin-bottom: 8px;
}

.login-logo {
  font-size: 32px;
  font-weight: 800;
  color: var(--dli-primary);
  display: inline-block;
  animation: logo-bounce 2.4s ease-in-out infinite;
}

@keyframes logo-bounce {
  0%,
  100% {
    transform: translateY(0);
  }

  50% {
    transform: translateY(-4px);
  }
}

.login-slogan {
  margin: 4px 0 0;
  font-size: 13px;
  color: var(--dli-text-2);
}

.login-tabs :deep(.el-tabs__item.is-active) {
  color: var(--dli-primary);
}

.login-tabs :deep(.el-tabs__active-bar) {
  background-color: var(--dli-primary);
}

.code-row {
  display: flex;
  gap: 8px;
  width: 100%;
}

.code-row .el-input {
  flex: 1;
}

/* 验证码行 */
.captcha-row {
  display: flex;
  gap: 10px;
  width: 100%;
}

.captcha-img {
  flex-shrink: 0;
  width: 120px;
  height: 40px;
  cursor: pointer;
  border-radius: 6px;
  overflow: hidden;
  border: 1px solid #e3e5e7;
}

.captcha-img :deep(svg) {
  display: block;
  width: 100%;
  height: 100%;
}

.login-btn {
  width: 100%;
  margin-top: 4px;
  --el-button-bg-color: #{v.$primary};
  --el-button-border-color: #{v.$primary};
  --el-button-hover-bg-color: #{v.$primary-hover};
  --el-button-hover-border-color: #{v.$primary-hover};
}

.login-tip {
  margin: 12px 0 0;
  font-size: 12px;
  color: var(--dli-text-2);
  text-align: center;
}

/* 无障碍：用户偏好减少动效时关闭动画 */
@media (prefers-reduced-motion: reduce) {
  .login-page,
  .bg-blob,
  .bg-danmaku,
  .login-logo,
  .login-card {
    animation: none;
  }

  .bg-danmaku {
    display: none;
  }
}

.activate-tip {
  margin: 0 0 12px;
  font-size: 13px;
  color: var(--dli-text-2);
  word-break: break-all;
}

.activate-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 16px;
}
</style>
