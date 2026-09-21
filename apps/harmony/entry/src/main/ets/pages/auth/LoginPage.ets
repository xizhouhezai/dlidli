import { ApiError } from '../../model/Api';
import { CaptchaResult, TokenPair } from '../../model/Auth';
import { AuthApi } from '../../service/AuthApi';
import { CaptchaImage } from '../../components/CaptchaImage';
import { AppRouter } from '../../common/utils/AppRouter';
import { session } from '../../store/Session';
import { DliFont, DliRadius, DliSpace } from '../../common/constants/Theme';
import { ErrCode, ErrorMessages } from '../../common/constants/ErrorMessages';
import { Logger } from '../../common/utils/Logger';

const TAG: string = 'LoginPage';
const PHONE_RE = /^1\d{10}$/;
const SMS_COUNTDOWN_SEC: number = 60;

enum LoginMode {
  SMS = 0,
  PASSWORD = 1
}

/**
 * 登录页（NavDestination）：验证码登录 / 密码登录，承 ACC-01、ACC-03。
 * 登录成功后凭证交 Session 落 TokenStore，随即返回上一页；不在此页做跳转决策。
 */
@ComponentV2
export struct LoginPage {
  @Local mode: number = LoginMode.SMS;
  @Local phone: string = '';
  @Local smsCode: string = '';
  @Local account: string = '';
  @Local password: string = '';
  @Local captchaId: string = '';
  @Local captchaSvg: string = '';
  @Local captchaCode: string = '';
  @Local countdown: number = 0;
  @Local submitting: boolean = false;
  @Local errorText: ResourceStr = '';
  @Local hintText: ResourceStr = '';
  private timerId: number = -1;

  aboutToAppear(): void {
    this.loadCaptcha();
  }

  aboutToDisappear(): void {
    this.stopCountdown();
  }

  private stopCountdown(): void {
    if (this.timerId !== -1) {
      clearInterval(this.timerId);
      this.timerId = -1;
    }
  }

  private startCountdown(): void {
    this.countdown = SMS_COUNTDOWN_SEC;
    this.stopCountdown();
    this.timerId = setInterval(() => {
      this.countdown--;
      if (this.countdown <= 0) {
        this.stopCountdown();
      }
    }, 1000);
  }

  private async loadCaptcha(): Promise<void> {
    try {
      const result: CaptchaResult = await AuthApi.getCaptcha();
      this.captchaId = result.id;
      this.captchaSvg = result.svg;
    } catch (err) {
      const apiErr: ApiError = err as ApiError;
      Logger.warn(TAG, `图形验证码加载失败：${apiErr.describe()}`);
      this.captchaSvg = '';
    }
  }

  private async sendCode(): Promise<void> {
    if (!PHONE_RE.test(this.phone)) {
      this.errorText = $r('app.string.login_need_phone');
      return;
    }
    this.errorText = '';
    this.hintText = '';
    this.submitting = true;
    try {
      const debugCode: string = await AuthApi.sendSmsCode(this.phone);
      this.startCountdown();
      if (debugCode.length > 0) {
        // dev 环境后端 mock 短信并回显验证码，直接填入省去手输
        this.smsCode = debugCode;
        this.hintText = $r('app.string.login_hint_dev_filled');
      } else {
        this.hintText = $r('app.string.login_hint_sms_sent');
      }
    } catch (err) {
      const apiErr: ApiError = err as ApiError;
      this.errorText = apiErr.message;
      Logger.warn(TAG, `发送验证码失败：${apiErr.describe()}`);
    }
    this.submitting = false;
  }

  private async submit(): Promise<void> {
    if (this.submitting) {
      return;
    }
    const validation: ResourceStr = this.validate();
    if (validation !== '') {
      this.errorText = validation;
      return;
    }
    this.errorText = '';
    this.submitting = true;
    try {
      const pair: TokenPair = this.mode === LoginMode.SMS
        ? await AuthApi.loginBySms(this.phone, this.smsCode)
        : await AuthApi.loginByPassword(this.account, this.password, this.captchaId, this.captchaCode);
      await session.onLoggedIn(pair);
      AppRouter.pop();
    } catch (err) {
      const apiErr: ApiError = err as ApiError;
      this.errorText = apiErr.message.length > 0 ? apiErr.message : ErrorMessages.text(ErrCode.INTERNAL, '');
      Logger.warn(TAG, `登录失败：${apiErr.describe()}`);
      if (this.mode === LoginMode.PASSWORD) {
        // 图形验证码一次性，失败后必须换一张
        this.captchaCode = '';
        this.loadCaptcha();
      }
    }
    this.submitting = false;
  }

  private validate(): ResourceStr {
    if (this.mode === LoginMode.SMS) {
      if (!PHONE_RE.test(this.phone)) {
        return $r('app.string.login_need_phone');
      }
      return this.smsCode.length === 6 ? '' : $r('app.string.login_need_code');
    }
    if (this.account.length === 0) {
      return $r('app.string.login_need_account');
    }
    if (this.password.length < 6) {
      return $r('app.string.login_need_password');
    }
    return this.captchaCode.length > 0 ? '' : $r('app.string.login_need_captcha');
  }

  @Builder
  modeTab(label: Resource, index: number) {
    Column({ space: DliSpace.SM }) {
      Text(label)
        .fontSize(DliFont.BODY)
        .fontWeight(this.mode === index ? FontWeight.Medium : FontWeight.Normal)
        .fontColor(this.mode === index ? $r('app.color.text_primary') : $r('app.color.text_tertiary'))
      Divider()
        .width(24)
        .strokeWidth(2)
        .color(this.mode === index ? $r('app.color.brand_primary') : Color.Transparent)
    }
    .padding({ bottom: DliSpace.SM })
    .onClick(() => {
      this.mode = index;
      this.errorText = '';
      this.hintText = '';
      if (index === LoginMode.PASSWORD && this.captchaSvg.length === 0) {
        this.loadCaptcha();
      }
    })
  }

  @Builder
  smsForm() {
    Column({ space: DliSpace.MD }) {
      TextInput({ text: $$this.phone, placeholder: $r('app.string.login_phone_hint') })
        .type(InputType.PhoneNumber)
        .maxLength(11)
        .height(48)
        .fontSize(DliFont.BODY)
        .fontColor($r('app.color.text_primary'))
        .placeholderColor($r('app.color.text_tertiary'))
        .backgroundColor($r('app.color.bg_surface'))
        .borderRadius(DliRadius.MD)
        .padding({ left: DliSpace.LG, right: DliSpace.LG })

      Row({ space: DliSpace.SM }) {
        TextInput({ text: $$this.smsCode, placeholder: $r('app.string.login_code_hint') })
          .type(InputType.Number)
          .maxLength(6)
          .height(48)
          .fontSize(DliFont.BODY)
          .fontColor($r('app.color.text_primary'))
          .placeholderColor($r('app.color.text_tertiary'))
          .backgroundColor($r('app.color.bg_surface'))
          .borderRadius(DliRadius.MD)
          .padding({ left: DliSpace.LG, right: DliSpace.LG })
          .layoutWeight(1)

        Button(this.countdown > 0 ? `${this.countdown}s` : $r('app.string.login_send_code'))
          .fontSize(DliFont.BODY_SM)
          .fontColor(this.countdown > 0 ? $r('app.color.text_tertiary') : $r('app.color.brand_primary'))
          .backgroundColor($r('app.color.brand_primary_light'))
          .borderRadius(DliRadius.MD)
          .height(48)
          .enabled(this.countdown === 0 && !this.submitting)
          .onClick(() => {
            this.sendCode();
          })
      }
      .width('100%')
    }
    .width('100%')
  }

  @Builder
  passwordForm() {
    Column({ space: DliSpace.MD }) {
      TextInput({ text: $$this.account, placeholder: $r('app.string.login_account_hint') })
        .type(InputType.Normal)
        .maxLength(64)
        .height(48)
        .fontSize(DliFont.BODY)
        .fontColor($r('app.color.text_primary'))
        .placeholderColor($r('app.color.text_tertiary'))
        .backgroundColor($r('app.color.bg_surface'))
        .borderRadius(DliRadius.MD)
        .padding({ left: DliSpace.LG, right: DliSpace.LG })

      TextInput({ text: $$this.password, placeholder: $r('app.string.login_password_hint') })
        .type(InputType.Password)
        .maxLength(64)
        .height(48)
        .fontSize(DliFont.BODY)
        .fontColor($r('app.color.text_primary'))
        .placeholderColor($r('app.color.text_tertiary'))
        .backgroundColor($r('app.color.bg_surface'))
        .borderRadius(DliRadius.MD)
        .padding({ left: DliSpace.LG, right: DliSpace.LG })

      Row({ space: DliSpace.SM }) {
        TextInput({ text: $$this.captchaCode, placeholder: $r('app.string.login_captcha_hint') })
          .type(InputType.Normal)
          .maxLength(4)
          .height(48)
          .fontSize(DliFont.BODY)
          .fontColor($r('app.color.text_primary'))
          .placeholderColor($r('app.color.text_tertiary'))
          .backgroundColor($r('app.color.bg_surface'))
          .borderRadius(DliRadius.MD)
          .padding({ left: DliSpace.LG, right: DliSpace.LG })
          .layoutWeight(1)

        if (this.captchaSvg.length > 0) {
          CaptchaImage({
            svg: this.captchaSvg,
            onRefresh: () => {
              this.loadCaptcha();
            }
          })
        } else {
          Row() {
            Text($r('app.string.login_captcha_hint'))
              .fontSize(DliFont.CAPTION)
              .fontColor($r('app.color.text_tertiary'))
          }
          .width(150)
          .height(50)
          .justifyContent(FlexAlign.Center)
          .borderRadius(DliRadius.MD)
          .backgroundColor($r('app.color.bg_surface'))
          .onClick(() => {
            this.loadCaptcha();
          })
        }
      }
      .width('100%')
    }
    .width('100%')
  }

  build() {
    NavDestination() {
      Scroll() {
        Column({ space: DliSpace.LG }) {
          Row({ space: DliSpace.XL }) {
            this.modeTab($r('app.string.login_tab_sms'), LoginMode.SMS)
            this.modeTab($r('app.string.login_tab_password'), LoginMode.PASSWORD)
          }
          .width('100%')

          if (this.mode === LoginMode.SMS) {
            this.smsForm()
          } else {
            this.passwordForm()
          }

          if (this.hintText !== '') {
            Text(this.hintText)
              .fontSize(DliFont.BODY_SM)
              .fontColor($r('app.color.brand_primary'))
              .width('100%')
          }

          if (this.errorText !== '') {
            Text(this.errorText)
              .fontSize(DliFont.BODY_SM)
              .fontColor($r('app.color.state_danger'))
              .width('100%')
          }

          Button($r('app.string.login_submit'))
            .width('100%')
            .height(48)
            .fontSize(DliFont.BODY)
            .fontColor($r('app.color.text_on_brand'))
            .backgroundColor($r('app.color.brand_primary'))
            .borderRadius(DliRadius.FULL)
            .enabled(!this.submitting)
            .onClick(() => {
              this.submit();
            })
        }
        .width('100%')
        .padding({ left: DliSpace.PAGE, right: DliSpace.PAGE, top: DliSpace.LG })
        .alignItems(HorizontalAlign.Start)
      }
      .scrollBar(BarState.Off)
      .align(Alignment.Top)
      .width('100%')
      .height('100%')
    }
    .title($r('app.string.login_title'))
    .backgroundColor($r('app.color.bg_page'))
  }
}
