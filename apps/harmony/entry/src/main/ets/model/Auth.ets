/**
 * 账号域端侧模型（手写并与后端 DTO 对齐，字段名以 server/internal/module/account/model.go 为准）。
 * 本期只落网络层所需的令牌对；资料等模型随 M4-HMY-04 登录与会话补齐。
 */
export interface TokenPair {
  access_token: string;
  refresh_token: string;
  expires_in: number;
}
