// 管理后台 API 客户端：管理员令牌失效时跳回登录页
import { createApiClient } from '@dlidli/api-client'
import { clearAdminToken, readAdminToken } from '@/utils/token'
import router from '@/router'

export const adminApi = createApiClient({
  baseURL: '',
  getToken: readAdminToken,
  onUnauthorized: () => {
    clearAdminToken()
    router.push({ name: 'login' })
  },
})
