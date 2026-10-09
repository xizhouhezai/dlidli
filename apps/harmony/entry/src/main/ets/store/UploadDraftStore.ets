import { preferences } from '@kit.ArkData';
import { ApiError } from '../model/Api';
import { ErrCode } from '../common/constants/ErrorMessages';
import { PartDraft, SubmitDraft, emptyDraft, extraPartLabel } from '../model/Upload';
import { UploadApi } from '../service/UploadApi';
import { Logger } from '../common/utils/Logger';
import { PreferenceStore } from './PreferenceStore';

const TAG: string = 'UploadDraftStore';
/** key 见 plan §3 的 upload.draft（与令牌、弹幕设置、搜索历史共用 dlidli_prefs） */
const KEY_DRAFT: string = 'upload.draft';

/** 单个上传会话的校验结论 */
export interface PartSessionCheck {
  /** 稳定标识：`main` 或 `part:<下标>` */
  key: string;
  /** 可展示的编号，如 `分P1` / `分P3` */
  label: string;
  /** valid 可续传 / expired 会话已失效（须重选文件） */
  state: 'valid' | 'expired';
  /** 后端已收到的分片数（valid 时有意义） */
  uploaded: number;
  /** 总分片数 */
  total: number;
}

/** 一次草稿恢复的会话校验汇总 */
export interface SessionReport {
  /** 逐会话结论 */
  checks: PartSessionCheck[];
  /** 需要重选文件的编号（已失效） */
  expired: string[];
  /** 可断点续传的编号 */
  resumable: string[];
  /** 校验途中网络不可用：**不许据此判定会话失效**（否则用户会因一次断网被要求重选全部文件） */
  networkFailed: boolean;
}

/**
 * 投稿草稿的本地持久化（HMY-52：投稿中退出可恢复）。
 *
 * 只存**端侧自己产出的东西**：稿件表单字段、各分P 的 `file_id` 与上传会话 ID、封面沙箱路径。
 * 分片账本仍在后端 Redis（`up:sess:*`，TTL 24h），本类不复制它——它只在 `checkSessions()`
 * 里被**读一次**用于判断会话是否还有效。
 */
export class UploadDraftStore {
  private static draft: SubmitDraft | null = null;
  private static loaded: boolean = false;

  /** 读草稿；从未存过返回 null（与「存过但为空」区分，后者不落盘故不会出现） */
  static async load(): Promise<SubmitDraft | null> {
    await UploadDraftStore.ensureLoaded();
    return UploadDraftStore.draft;
  }

  /** 写草稿（覆盖式）。存储不可用时降级为仅内存生效并记日志 */
  static async save(draft: SubmitDraft): Promise<void> {
    UploadDraftStore.draft = UploadDraftStore.sanitize(draft);
    UploadDraftStore.loaded = true;
    const store: preferences.Preferences | null = await PreferenceStore.open();
    if (store === null) {
      return;
    }
    try {
      await store.put(KEY_DRAFT, JSON.stringify(UploadDraftStore.draft));
      await store.flush();
      Logger.info(TAG, `草稿已保存：${UploadDraftStore.describe(UploadDraftStore.draft)}`);
    } catch (err) {
      Logger.error(TAG, `草稿持久化失败（仅内存生效）：${JSON.stringify(err)}`);
    }
  }

  /** 丢弃草稿（用户选择「丢弃」或投稿成功后调用） */
  static async clear(): Promise<void> {
    UploadDraftStore.draft = null;
    UploadDraftStore.loaded = true;
    const store: preferences.Preferences | null = await PreferenceStore.open();
    if (store === null) {
      return;
    }
    try {
      await store.delete(KEY_DRAFT);
      await store.flush();
      Logger.info(TAG, '草稿已清除');
    } catch (err) {
      Logger.error(TAG, `草稿清除失败（内存已清）：${JSON.stringify(err)}`);
    }
  }

  /**
   * 重进投稿流程时校验各分P 的上传会话（HMY-52 的硬性要求：**不要让用户提交后才失败**）。
   *
   * 三个必须成立的口径：
   * ① **只校验「有 uploadId 但没有 fileId」的条目**。上传一旦收口，后端 `Complete` 会主动
   *    `Del` 掉该会话——拿收口后的 uploadId 去查必然 30001，那不是「失效」而是「已完成」。
   *    秒传（fast）同样没有会话，也在跳过之列。
   * ② `GET /upload/{id}` 返回 **30001（上传任务不存在或已过期）** 才是真失效（Redis TTL 24h 到期）；
   *    其余错误码一律不判失效。
   * ③ **网络类失败（负数合成码 / 5xx）终止整轮校验**并置 `networkFailed`：断网不等于会话过期，
   *    据此让用户重选文件是错的。
   */
  static async checkSessions(draft: SubmitDraft): Promise<SessionReport> {
    const checks: PartSessionCheck[] = [];
    const expired: string[] = [];
    const resumable: string[] = [];

    const mainLabel: string = '分P1';
    if (draft.uploadId.length > 0 && draft.fileId.length === 0) {
      const one: PartSessionCheck | null = await UploadDraftStore.probe('main', mainLabel, draft.uploadId);
      if (one === null) {
        return { checks: [], expired: [], resumable: [], networkFailed: true };
      }
      checks.push(one);
    }
    for (let i = 0; i < draft.parts.length; i++) {
      const part: PartDraft = draft.parts[i];
      if (part.uploadId.length === 0 || part.fileId.length > 0) {
        continue;
      }
      const label: string = extraPartLabel(i);
      const one: PartSessionCheck | null = await UploadDraftStore.probe(`part:${i}`, label, part.uploadId);
      if (one === null) {
        return { checks: checks, expired: [], resumable: [], networkFailed: true };
      }
      checks.push(one);
    }

    for (const item of checks) {
      if (item.state === 'expired') {
        expired.push(item.label);
      } else {
        resumable.push(item.label);
      }
    }
    Logger.info(TAG, `会话校验完成：可续传=[${resumable.join(',')}] 已失效=[${expired.join(',')}]`);
    return { checks: checks, expired: expired, resumable: resumable, networkFailed: false };
  }

  /** 查单个会话；返回 null 表示网络类失败（调用方终止整轮）。30001 判失效，其余错误按有效处理 */
  private static async probe(key: string, label: string, uploadId: string): Promise<PartSessionCheck | null> {
    try {
      const res = await UploadApi.progress(uploadId);
      return {
        key: key, label: label, state: 'valid',
        uploaded: (res.uploaded ?? []).length, total: res.chunk_count
      };
    } catch (err) {
      const apiErr: ApiError = err as ApiError;
      if (apiErr.code === UploadDraftStore.SESSION_GONE) {
        Logger.warn(TAG, `${label} 上传会话已失效（${uploadId}）`);
        return { key: key, label: label, state: 'expired', uploaded: 0, total: 0 };
      }
      if (apiErr.code === ErrCode.UNAUTHORIZED) {
        // 未登录时无法判定会话，交给页面按「请先登录」处理；此处不制造失效结论
        return null;
      }
      Logger.warn(TAG, `${label} 会话校验异常，按有效处理：${apiErr.describe()}`);
      if (apiErr.retryable) {
        return null;
      }
      return { key: key, label: label, state: 'valid', uploaded: 0, total: 0 };
    }
  }

  /** 后端 30001：上传任务不存在或已过期 */
  private static readonly SESSION_GONE: number = 30001;

  private static async ensureLoaded(): Promise<void> {
    if (UploadDraftStore.loaded) {
      return;
    }
    UploadDraftStore.loaded = true;
    const store: preferences.Preferences | null = await PreferenceStore.open();
    if (store === null) {
      return;
    }
    try {
      const raw: string = await store.get(KEY_DRAFT, '') as string;
      if (raw.length === 0) {
        return;
      }
      UploadDraftStore.draft = UploadDraftStore.sanitize(JSON.parse(raw) as SubmitDraft);
      Logger.info(TAG, `草稿已恢复：${UploadDraftStore.describe(UploadDraftStore.draft)}`);
    } catch (err) {
      Logger.error(TAG, `读取草稿失败，按无草稿处理：${JSON.stringify(err)}`);
      UploadDraftStore.draft = null;
    }
  }

  /** 读盘内容不可信：逐字段补默认值，缺失字段按空处理（旧版本草稿、手改文件都不该让页面崩） */
  private static sanitize(raw: SubmitDraft): SubmitDraft {
    const base: SubmitDraft = emptyDraft();
    if (raw === null || raw === undefined) {
      return base;
    }
    base.title = typeof raw.title === 'string' ? raw.title : '';
    base.description = typeof raw.description === 'string' ? raw.description : '';
    base.categoryId = typeof raw.categoryId === 'number' ? raw.categoryId : 0;
    base.tags = Array.isArray(raw.tags) ? raw.tags : [];
    base.copyright = raw.copyright === 2 ? 2 : 1;
    base.fileId = typeof raw.fileId === 'string' ? raw.fileId : '';
    base.uploadId = typeof raw.uploadId === 'string' ? raw.uploadId : '';
    base.coverPath = typeof raw.coverPath === 'string' ? raw.coverPath : '';
    base.coverCacheUri = typeof raw.coverCacheUri === 'string' ? raw.coverCacheUri : '';
    base.coverOrigin = typeof raw.coverOrigin === 'string' ? raw.coverOrigin : '';
    base.coverWidth = typeof raw.coverWidth === 'number' ? raw.coverWidth : 0;
    base.coverHeight = typeof raw.coverHeight === 'number' ? raw.coverHeight : 0;
    base.coverUrl = typeof raw.coverUrl === 'string' ? raw.coverUrl : '';
    const parts: PartDraft[] = [];
    if (Array.isArray(raw.parts)) {
      for (const item of raw.parts) {
        if (item === null || item === undefined) {
          continue;
        }
        parts.push({
          title: typeof item.title === 'string' ? item.title : '',
          fileId: typeof item.fileId === 'string' ? item.fileId : '',
          fileName: typeof item.fileName === 'string' ? item.fileName : '',
          uploadId: typeof item.uploadId === 'string' ? item.uploadId : ''
        });
      }
    }
    base.parts = parts;
    return base;
  }

  /** 日志用摘要：只报可数与关键标识，不打印全文 */
  private static describe(draft: SubmitDraft): string {
    return `title=${draft.title.length}字 tags=${draft.tags.length} 主视频=${draft.fileId.length > 0 ? '已传' : ('会话' + (draft.uploadId.length > 0 ? '在' : '无'))}` +
      ` 额外分P=${draft.parts.length} 封面=${draft.coverPath.length > 0 ? draft.coverOrigin : '无'}`;
  }
}
