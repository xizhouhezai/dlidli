<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ApiError } from '@dlidli/api-client'
import { adminApi } from '@/api'
import { saveAdminToken, saveAdminInfo } from '@/utils/token'

const router = useRouter()
const form = reactive({ username: '', password: '' })
const loading = ref(false)

async function onLogin() {
  if (!form.username || !form.password) {
    ElMessage.warning('请输入账号和密码')
    return
  }
  loading.value = true
  try {
    const res = await adminApi.admin.login(form.username, form.password)
    saveAdminToken(res.token)
    saveAdminInfo({ username: res.username, role: res.role })
    ElMessage.success(`欢迎，${res.username}（${res.role}）`)
    router.push({ name: 'dashboard' })
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : '登录失败')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login">
    <!-- 背景装饰：光晕 / 网格 / 浮动光斑 -->
    <div class="login__glow login__glow--pink" />
    <div class="login__glow login__glow--blue" />
    <div class="login__grid" />
    <div
      v-for="i in 5"
      :key="i"
      class="login__orb"
      :style="{ '--i': i }"
    />

    <div class="panel">
      <div class="panel__brand">
        <span class="panel__logo">
          <svg
            viewBox="0 0 48 48"
            width="44"
            height="44"
            aria-hidden="true"
          >
            <rect
              x="4"
              y="12"
              width="40"
              height="28"
              rx="8"
              fill="none"
              stroke="url(#lg)"
              stroke-width="3.5"
            />
            <path
              d="M15 5l6 7M33 5l-6 7"
              stroke="url(#lg)"
              stroke-width="3.5"
              stroke-linecap="round"
              fill="none"
            />
            <circle
              cx="17"
              cy="26"
              r="2.6"
              fill="#fb7299"
            />
            <circle
              cx="31"
              cy="26"
              r="2.6"
              fill="#23ade5"
            />
            <path
              d="M19 33q5 3.5 10 0"
              stroke="#fb7299"
              stroke-width="2.6"
              stroke-linecap="round"
              fill="none"
            />
            <defs>
              <linearGradient
                id="lg"
                x1="0"
                y1="0"
                x2="1"
                y2="1"
              >
                <stop
                  offset="0"
                  stop-color="#fb7299"
                />
                <stop
                  offset="1"
                  stop-color="#23ade5"
                />
              </linearGradient>
            </defs>
          </svg>
        </span>
        <h1 class="panel__title">
          DliDli <span class="panel__title-sub">管理后台</span>
        </h1>
        <p class="panel__desc">
          内容审核 · 用户治理 · 运营配置
        </p>
      </div>

      <el-form
        class="panel__form"
        label-position="top"
        size="large"
        @submit.prevent="onLogin"
      >
        <el-form-item>
          <el-input
            v-model="form.username"
            placeholder="管理员账号"
            autocomplete="username"
          >
            <template #prefix>
              <span class="input-icon">👤</span>
            </template>
          </el-input>
        </el-form-item>
        <el-form-item>
          <el-input
            v-model="form.password"
            type="password"
            show-password
            placeholder="密码"
            autocomplete="current-password"
          >
            <template #prefix>
              <span class="input-icon">🔒</span>
            </template>
          </el-input>
        </el-form-item>
        <el-button
          class="panel__submit"
          :loading="loading"
          native-type="submit"
        >
          登 录
        </el-button>
      </el-form>

      <p class="panel__foot">
        内部系统 · 仅限授权人员访问 · 操作全程审计留痕
      </p>
    </div>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/variables' as v;

.login {
  position: relative;
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  background: v.$dark-deep;
}

/* ---- 背景装饰 ---- */
.login__glow {
  position: absolute;
  width: 560px;
  height: 560px;
  border-radius: 50%;
  filter: blur(120px);
  opacity: 0.32;
  animation: glow-drift 14s ease-in-out infinite alternate;
}

.login__glow--pink {
  background: v.$primary;
  top: -180px;
  left: -120px;
}

.login__glow--blue {
  background: #23ade5;
  bottom: -200px;
  right: -140px;
  animation-delay: -7s;
}

@keyframes glow-drift {
  from {
    transform: translate(0, 0) scale(1);
  }

  to {
    transform: translate(60px, 40px) scale(1.15);
  }
}

.login__grid {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(rgba(255, 255, 255, 0.035) 1px, transparent 1px),
    linear-gradient(90deg, rgba(255, 255, 255, 0.035) 1px, transparent 1px);
  background-size: 44px 44px;
  mask-image: radial-gradient(ellipse 70% 60% at 50% 45%, #000 30%, transparent 100%);
}

.login__orb {
  position: absolute;
  left: calc(12% + var(--i) * 16%);
  bottom: -12px;
  width: calc(5px + var(--i) * 2px);
  height: calc(5px + var(--i) * 2px);
  border-radius: 50%;
  background: rgba(251, 114, 153, 0.5);
  animation: orb-rise calc(9s + var(--i) * 3s) linear infinite;
  animation-delay: calc(var(--i) * -2.4s);
}

.login__orb:nth-child(even) {
  background: rgba(35, 173, 229, 0.45);
}

@keyframes orb-rise {
  from {
    transform: translateY(0);
    opacity: 0;
  }

  12% {
    opacity: 1;
  }

  85% {
    opacity: 0.5;
  }

  to {
    transform: translateY(-105vh);
    opacity: 0;
  }
}

/* ---- 玻璃卡片 ---- */
.panel {
  position: relative;
  z-index: 1;
  width: 400px;
  padding: 40px 36px 28px;
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.055);
  border: 1px solid rgba(255, 255, 255, 0.1);
  backdrop-filter: blur(22px);
  box-shadow: 0 24px 60px rgba(0, 0, 0, 0.45);
  animation: panel-in 0.55s cubic-bezier(0.22, 0.85, 0.35, 1);
}

@keyframes panel-in {
  from {
    opacity: 0;
    transform: translateY(22px) scale(0.98);
  }

  to {
    opacity: 1;
    transform: none;
  }
}

.panel__brand {
  text-align: center;
  margin-bottom: 26px;
}

.panel__logo {
  display: inline-flex;
  padding: 12px;
  border-radius: 14px;
  background: rgba(251, 114, 153, 0.1);
  border: 1px solid rgba(251, 114, 153, 0.22);
}

.panel__title {
  margin: 14px 0 6px;
  font-size: 24px;
  letter-spacing: 1px;
  background: linear-gradient(120deg, v.$primary, #23ade5);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
}

.panel__title-sub {
  font-weight: 500;
}

.panel__desc {
  margin: 0;
  font-size: 12.5px;
  letter-spacing: 2px;
  color: rgba(255, 255, 255, 0.4);
}

/* ---- 表单（深色定制） ---- */
.panel__form :deep(.el-input__wrapper) {
  background: rgba(255, 255, 255, 0.06);
  box-shadow: 0 0 0 1px rgba(255, 255, 255, 0.12) inset;
  border-radius: 10px;
  padding: 4px 14px;
  transition: box-shadow 0.2s;
}

.panel__form :deep(.el-input__wrapper:hover) {
  box-shadow: 0 0 0 1px rgba(255, 255, 255, 0.24) inset;
}

.panel__form :deep(.el-input__wrapper.is-focus) {
  box-shadow:
    0 0 0 1.5px var(--dli-primary) inset,
    0 0 14px rgba(251, 114, 153, 0.25);
}

.panel__form :deep(.el-input__inner) {
  color: #f1f2f3;
  caret-color: var(--dli-primary);
}

.panel__form :deep(.el-input__inner::placeholder) {
  color: rgba(255, 255, 255, 0.35);
}

.panel__form :deep(.el-input__suffix) {
  color: rgba(255, 255, 255, 0.45);
}

.input-icon {
  font-size: 14px;
  opacity: 0.75;
}

.panel__submit {
  width: 100%;
  margin-top: 6px;
  height: 44px;
  border: none;
  border-radius: 10px;
  font-size: 15px;
  letter-spacing: 6px;
  color: #fff;
  background: linear-gradient(120deg, v.$primary 10%, #e8467c 90%);
  box-shadow: 0 8px 22px rgba(251, 114, 153, 0.32);
  transition:
    transform 0.18s,
    box-shadow 0.18s,
    filter 0.18s;
}

.panel__submit:hover {
  transform: translateY(-1px);
  filter: brightness(1.06);
  box-shadow: 0 12px 28px rgba(251, 114, 153, 0.42);
}

.panel__submit:active {
  transform: translateY(0);
}

.panel__foot {
  margin: 22px 0 0;
  text-align: center;
  font-size: 11.5px;
  letter-spacing: 0.5px;
  color: rgba(255, 255, 255, 0.28);
}

/* 无障碍：偏好减少动效时关闭动画 */
@media (prefers-reduced-motion: reduce) {
  .login__glow,
  .login__orb,
  .panel {
    animation: none;
  }
}
</style>
