import { ref } from 'vue'
import { defineStore } from 'pinia'
import type { User } from '@dlidli/shared'
import type { TokenPair } from '@dlidli/api-client'
import { api } from '@/api'
import { clearTokens, readRefreshToken, readToken, saveTokens } from '@/utils/token'

/** 用户会话状态 */
export const useUserStore = defineStore('user', () => {
  const token = ref<string | null>(readToken())
  const profile = ref<User | null>(null)

  function applyTokenPair(pair: TokenPair) {
    token.value = pair.access_token
    saveTokens(pair.access_token, pair.refresh_token)
    profile.value = pair.user
  }

  async function loginBySms(phone: string, code: string) {
    applyTokenPair(await api.auth.loginBySms(phone, code))
  }

  async function loginByPassword(
    account: string,
    password: string,
    captchaId: string,
    captchaCode: string,
  ) {
    applyTokenPair(await api.auth.loginByPassword(account, password, captchaId, captchaCode))
  }

  /** 页面刷新后拉取用户资料（access 过期时由 api-client 自动续期重试） */
  async function fetchProfile() {
    if (!token.value || profile.value) return
    try {
      profile.value = await api.auth.me()
      // 静默续期后 localStorage 里是新 token，同步到 store
      token.value = readToken()
    } catch {
      // 续期也失败时由 onUnauthorized 统一清理
    }
  }

  /** 强制重拉资料（硬币余额等变更后同步） */
  async function refreshProfile() {
    if (!token.value) return
    try {
      profile.value = await api.auth.me()
    } catch {
      // 失败保留旧资料
    }
  }

  async function logout() {
    const refresh = readRefreshToken()
    if (refresh) {
      try {
        await api.auth.logout(refresh)
      } catch {
        // 服务端会话清理失败不阻塞本地登出
      }
    }
    forceLogout()
  }

  /** 清空全部登录信息（本地缓存 + store 状态）；token 失效时由 api 层调用 */
  function forceLogout() {
    token.value = null
    profile.value = null
    clearTokens()
  }

  return {
    token,
    profile,
    loginBySms,
    loginByPassword,
    fetchProfile,
    refreshProfile,
    logout,
    forceLogout,
  }
})
