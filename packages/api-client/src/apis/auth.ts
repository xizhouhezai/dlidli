import type { User } from '@dlidli/shared'
import type { HttpClient } from '../http'

export interface TokenPair {
  access_token: string
  refresh_token: string
  expires_in: number
  user: User
}

/** 资料更新请求；未传字段表示不修改 */
export interface UpdateProfileReq {
  nickname?: string
  signature?: string
  gender?: 0 | 1 | 2
}

/** 账号认证接口（对应后端 /api/v1/auth）。 */
export function createAuthApi(http: HttpClient) {
  return {
    /** 发送短信验证码；dev 环境后端返回 debug_code 便于联调 */
    sendSmsCode: (phone: string) =>
      http.post<{ debug_code?: string }>('/api/v1/auth/sms-code', { phone }),

    loginBySms: (phone: string, code: string, inviteCode = '') =>
      http.post<TokenPair>('/api/v1/auth/login/sms', { phone, code, invite_code: inviteCode }),

    loginByPassword: (account: string, password: string, captchaId: string, captchaCode: string) =>
      http.post<TokenPair>('/api/v1/auth/login/password', {
        account,
        password,
        captcha_id: captchaId,
        captcha_code: captchaCode,
      }),

    /** 邮箱注册（ACC-02）：创建待激活账号；dev 环境后端返回 debug_activate_url 便于联调 */
    registerEmail: (email: string, password: string, inviteCode = '') =>
      http.post<{ activated: boolean; debug_activate_url?: string }>(
        '/api/v1/auth/register/email',
        {
          email,
          password,
          invite_code: inviteCode,
        },
      ),

    /** 激活邮箱账号（ACC-02）：一次性 token */
    activate: (token: string) =>
      http.post<{ activated: boolean }>('/api/v1/auth/activate', { token }),

    /** 获取图形验证码（返回 id + 内联 SVG） */
    captcha: () => http.get<{ id: string; svg: string }>('/api/v1/auth/captcha'),

    refresh: (refreshToken: string) =>
      http.post<TokenPair>('/api/v1/auth/refresh', { refresh_token: refreshToken }),

    logout: (refreshToken: string) =>
      http.post<null>('/api/v1/auth/logout', { refresh_token: refreshToken }),

    me: () => http.get<User>('/api/v1/users/me'),

    /** 修改密码；从未设过密码时 oldPassword 传空串 */
    changePassword: (oldPassword: string, newPassword: string) =>
      http.put<null>('/api/v1/users/me/password', {
        old_password: oldPassword,
        new_password: newPassword,
      }),

    /** 忘记密码：短信验证码重置 */
    resetPassword: (phone: string, code: string, newPassword: string) =>
      http.post<null>('/api/v1/auth/reset-password', {
        phone,
        code,
        new_password: newPassword,
      }),

    updateProfile: (req: UpdateProfileReq) => http.patch<User>('/api/v1/users/me', req),

    uploadAvatar: (file: File) => {
      const form = new FormData()
      form.append('file', file)
      return http.postForm<{ avatar: string }>('/api/v1/users/me/avatar', form)
    },

    /** 青少年模式状态 */
    youthMode: () => http.get<{ enabled: boolean }>('/api/v1/users/me/youth-mode'),

    /** 开关青少年模式 */
    setYouthMode: (enabled: boolean) => http.put<null>('/api/v1/users/me/youth-mode', { enabled }),
  }
}
