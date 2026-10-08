import { fileIo } from '@kit.CoreFileKit';
import { util } from '@kit.ArkTS';
import { common } from '@kit.AbilityKit';
import { ApiError } from '../model/Api';
import { ErrCode } from '../common/constants/ErrorMessages';
import { CoverDraft, CoverOrigin, CoverUploadResult, PickedVideo } from '../model/Upload';
import { CroppedCover, ImageCropper } from '../common/utils/ImageCropper';
import { VideoFrame } from '../common/utils/VideoFrame';
import { MediaLib } from '../common/utils/MediaLib';
import { HttpClient } from './HttpClient';
import { Logger } from '../common/utils/Logger';

const TAG: string = 'CoverService';
const PATH_COVER: string = '/api/v1/videos/cover';
/** 临时中间帧的固定名（裁完即删，同一时刻只有一次截帧在飞） */
const FRAME_TEMP_NAME: string = 'frame_src';

/**
 * 封面域（HMY-51）：两种来源 → 统一 16:9 裁切 → multipart 上传。
 *
 * **协议不要与分片混用**：分片是裸二进制 body（`method:'PUT'` + `begins`/`ends` 字节区间，见 `UploadController`），
 * 封面是 `multipart/form-data`（后端 `c.FormFile("file")`）——两处虽然都发字节，信封格式完全不同。
 */
export class CoverService {
  /** 相册选图 → 16:9 裁切（用户取消返回 null） */
  static async fromPhoto(context: common.UIAbilityContext): Promise<CoverDraft | null> {
    const uri: string = await MediaLib.pickImage(context);
    if (uri.length === 0) {
      return null;
    }
    const cropped: CroppedCover = await ImageCropper.fromUri(context, uri, CoverService.fileName('photo'));
    return CoverService.toDraft('photo', cropped);
  }

  /**
   * 视频截帧 → 16:9 裁切。
   * 截帧不可用（mkv/flv 等）返回 null，由调用方提示「该视频无法截帧，请从相册选图」并继续投稿。
   */
  static async fromVideo(context: common.UIAbilityContext, picked: PickedVideo): Promise<CoverDraft | null> {
    const videoPath: string = await MediaLib.ensureStagedPath(context, picked);
    const framePath: string = await VideoFrame.captureToCache(context, videoPath, FRAME_TEMP_NAME);
    if (framePath.length === 0) {
      return null;
    }
    try {
      const cropped: CroppedCover = await ImageCropper.fromUri(context, framePath, CoverService.fileName('frame'));
      return CoverService.toDraft('frame', cropped);
    } finally {
      ImageCropper.remove(framePath);
    }
  }

  /** 删掉本地裁切产物（提交成功、换封面或页面销毁时调用） */
  static discard(draft: CoverDraft | null): void {
    if (draft !== null) {
      ImageCropper.remove(draft.sandboxPath);
    }
  }

  /**
   * multipart 上传封面，返回可直接写进 `SubmitReq.cover` 的 URL。
   *
   * **为什么自造 multipart 而不用 `request.uploadFile`（HMY-52 实测修正）**：`uploadFile` 的 `POST` 分支确实
   * 能发出合法 multipart（后端 `c.FormFile("file")` 解析成功、封面落盘 `covers/<uid>_<ms>.jpg`），
   * **但 `TaskState.message` 不带响应体**——实测为空串，拿不到 `data.cover`；`request.agent` 的
   * `HttpResponse` 也只有 `statusCode/reason/headers`，同无 body。故改走 B 路线（`extraData` 传 `ArrayBuffer`
   * 裸 body + 自带 `Content-Type: multipart/form-data; boundary=…`，plan §6.1 已实测该通道可用），
   * 换来可读的统一包裹与 `HttpClient` 的凭证/续期/错误定级。
   */
  static async upload(context: common.UIAbilityContext, draft: CoverDraft): Promise<string> {
    const bytes: ArrayBuffer = CoverService.readBytes(draft.sandboxPath);
    const boundary: string = `----dlidliCover${Date.now()}`;
    const body: ArrayBuffer = CoverService.multipart(boundary, `${draft.origin}.jpg`, bytes);
    const result: CoverUploadResult =
      await HttpClient.postRaw<CoverUploadResult>(PATH_COVER, body, `multipart/form-data; boundary=${boundary}`);
    const url: string = result.cover ?? '';
    if (url.length === 0) {
      throw new ApiError(ErrCode.BAD_RESPONSE, '封面地址为空，请重试', '', true);
    }
    Logger.info(TAG, `封面已上传：${url}`);
    return url;
  }

  /** 读裁切产物（≤5MB，一次读入内存；`readSync` 可能短读，按实读长度截断） */
  private static readBytes(path: string): ArrayBuffer {
    const file: fileIo.File = fileIo.openSync(path, fileIo.OpenMode.READ_ONLY);
    try {
      const size: number = fileIo.statSync(file.fd).size;
      const buffer: ArrayBuffer = new ArrayBuffer(size);
      const read: number = fileIo.readSync(file.fd, buffer);
      return read === size ? buffer : buffer.slice(0, read);
    } finally {
      fileIo.closeSync(file);
    }
  }

  /**
   * 拼 multipart/form-data 信封：前言 + 文件字节 + 结束边界。
   * 字段名固定 `file`（后端 `c.FormFile("file")`），文件名用 ASCII 的 `<来源>.jpg`——中文文件名在
   * `Content-Disposition` 里要额外编码，没必要给自己找麻烦。
   */
  private static multipart(boundary: string, fileName: string, bytes: ArrayBuffer): ArrayBuffer {
    const encoder: util.TextEncoder = new util.TextEncoder();
    const head: Uint8Array = encoder.encodeInto(
      `--${boundary}\r\nContent-Disposition: form-data; name="file"; filename="${fileName}"\r\n` +
      `Content-Type: image/jpeg\r\n\r\n`);
    const tail: Uint8Array = encoder.encodeInto(`\r\n--${boundary}--\r\n`);
    const payload: Uint8Array = new Uint8Array(bytes);
    const merged: Uint8Array = new Uint8Array(head.length + payload.length + tail.length);
    merged.set(head, 0);
    merged.set(payload, head.length);
    merged.set(tail, head.length + payload.length);
    return merged.buffer;
  }

  private static toDraft(origin: CoverOrigin, cropped: CroppedCover): CoverDraft {
    return {
      origin: origin,
      cacheUri: cropped.cacheUri,
      sandboxPath: cropped.sandboxPath,
      width: cropped.width,
      height: cropped.height,
      url: ''
    };
  }

  /** 文件名主名带来源与时间戳：换封面即换文件，不会读到上一次的裁切产物 */
  private static fileName(origin: CoverOrigin): string {
    return `cover_${origin}_${Date.now()}`;
  }
}
