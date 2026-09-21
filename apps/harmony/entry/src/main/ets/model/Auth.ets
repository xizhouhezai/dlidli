/**
 * 账号域端侧模型（手写并与后端 DTO 对齐，字段名以 server/internal/module/account/model.go 为准）。
 * 登录、刷新、/users/me 均返回同一份 Profile，故 TokenPair 恒带 user。
 */
export interface Profile {
  id: string;
  nickname: string;
  avatar: string;
  signature: string;
  gender: number;
  level: number;
  coin: number;
  /** 0 正常 1 禁言 2 封禁 3 注销 */
  status: number;
}

export interface TokenPair {
  access_token: string;
  refresh_token: string;
  expires_in: number;
  user: Profile;
}

/** GET /auth/sms-code：dev 环境返回 debug_code（mock 短信），生产为空 */
export interface SmsCodeResult {
  debug_code?: string;
}

/**
 * GET /auth/captcha：后端返回内联 SVG（server/internal/pkg/captcha.Result），
 * 字段名为 id/svg；密码登录时以 captcha_id 回传 id。处理器上的 swagger 注释写的是
 * {captcha_id, image_base64}，与实现不符，以实现为准。
 */
export interface CaptchaResult {
  id: string;
  svg: string;
}
