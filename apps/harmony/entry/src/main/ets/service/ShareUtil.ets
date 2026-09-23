import { systemShare } from '@kit.ShareKit';
import { common } from '@kit.AbilityKit';
import { uniformTypeDescriptor } from '@kit.ArkData';
import { pasteboard } from '@kit.BasicServicesKit';
import { ApiConfig } from '../common/constants/ApiConfig';
import { Logger } from '../common/utils/Logger';

const TAG: string = 'Share';

/**
 * 系统分享（HMY-32）：调起系统分享面板分享视频链接。
 *
 * 端侧差异（承 spec HMY-32）：分享不依赖微信小程序卡片——鸿蒙端没有小程序载体，
 * 直接走系统分享面板（ShareController），由系统分发给已安装的目标应用。
 *
 * 数据用通用超链接类型（utd = general.hyperlink）：目标应用据此决定最合适的接收方式。
 * 分享结果是异步的（show 的 Promise 只在面板关闭时 resolve，不代表用户真的发出去了），
 * 所以本工具不提供「分享成功」回调——SHR-02 只要求能调出面板，不统计分享计数。
 */
export class ShareUtil {
  /** 视频站外分享链接的构造口径：与 Web 端一致，指向 /video/{bvid} 页面 */
  static videoShareUrl(bvid: string): string {
    return `${ApiConfig.WEB_BASE_URL}/video/${encodeURIComponent(bvid)}`;
  }

  /**
   * 调起系统分享面板。context 为 UIAbilityContext（页面经 getUIContext().getHostContext() 取得）。
   * 返回的 Promise 在面板关闭时 resolve，不代表用户完成分享（SHR-02 不要求统计分享计数）。
   */
  static async shareVideo(context: common.UIAbilityContext, title: string, url: string): Promise<void> {
    try {
      const record: systemShare.SharedRecord = {
        utd: uniformTypeDescriptor.UniformDataType.HYPERLINK,
        content: url,
        title: title
      };
      const data: systemShare.SharedData = new systemShare.SharedData(record);
      const controller: systemShare.ShareController = new systemShare.ShareController(data);
      const options: systemShare.ShareControllerOptions = {
        previewMode: systemShare.SharePreviewMode.DEFAULT
      };
      await controller.show(context, options);
    } catch (err) {
      Logger.warn(TAG, `调起分享面板失败：${JSON.stringify(err)}`);
      throw (err as Error);
    }
  }

  /**
   * 复制链接到剪贴板（SHR-02 站外分享的最简形态，分享面板之外提供备用）。
   * @throws { Error } 剪贴板写入失败（调用方用 try-catch 兜底，对齐 showToast 的 @throws 口径）
   */
  static async copyLink(url: string): Promise<void> {
    const data: pasteboard.PasteData = pasteboard.createData(pasteboard.MIMETYPE_TEXT_PLAIN, url);
    await pasteboard.getSystemPasteboard().setData(data);
  }
}

