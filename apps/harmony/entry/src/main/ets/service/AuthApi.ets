import { HttpClient } from './HttpClient';
import { CaptchaResult, Profile, SmsCodeResult, TokenPair } from '../model/Auth';
import { ApiConfig } from '../common/constants/ApiConfig';

const PATH_SMS_CODE: string = '/api/v1/auth/sms-code';
const PATH_LOGIN_SMS: string = '/api/v1/auth/login/sms';
const PATH_LOGIN_PASSWORD: string = '/api/v1/auth/login/password';
const PATH_CAPTCHA: string = '/api/v1/auth/captcha';
const PATH_ME: string = '/api/v1/users/me';

/**
 * 认证域接口。登录/验证码/续期/登出均为「自证接口」，走 postPublic/getPublic——
 * 既不带 Authorization，也不参与 401 续期重放（凭证本身无效时没有可重放的前提）。
 */
export class AuthApi {
  /** 发送短信验证码，返回 dev 环境回显的 debug_code（生产为空串） */
  static async sendSmsCode(phone: string): Promise<string> {
    const payload: Record<string, string> = { 'phone': phone };
    const data: SmsCodeResult = await HttpClient.postPublic<SmsCodeResult>(PATH_SMS_CODE, payload);
    return data.debug_code ?? '';
  }

  /** 验证码登录（未注册手机号由服务端自动注册） */
  static loginBySms(phone: string, code: string): Promise<TokenPair> {
    const payload: Record<string, string> = { 'phone': phone, 'code': code };
    return HttpClient.postPublic<TokenPair>(PATH_LOGIN_SMS, payload);
  }

  /** 账号密码登录（account 为手机号或邮箱，需图形验证码） */
  static loginByPassword(
    account: string,
    password: string,
    captchaId: string,
    captchaCode: string
  ): Promise<TokenPair> {
    const payload: Record<string, string> = {
      'account': account,
      'password': password,
      'captcha_id': captchaId,
      'captcha_code': captchaCode
    };
    return HttpClient.postPublic<TokenPair>(PATH_LOGIN_PASSWORD, payload);
  }

  static getCaptcha(): Promise<CaptchaResult> {
    return HttpClient.getPublic<CaptchaResult>(PATH_CAPTCHA);
  }

  /** 吊销刷新令牌；失败由调用方兜底（本地凭证仍须清干净） */
  static logout(refreshToken: string): Promise<void> {
    const payload: Record<string, string> = { 'refresh_token': refreshToken };
    return HttpClient.postPublic<void>(ApiConfig.LOGOUT_PATH, payload);
  }

  /** 当前登录用户资料（需鉴权，走静默续期链路） */
  static me(): Promise<Profile> {
    return HttpClient.get<Profile>(PATH_ME);
  }
}
