import { fileIo, hash, picker, ReadOptions } from '@kit.CoreFileKit';
import { BusinessError } from '@kit.BasicServicesKit';
import { common } from '@kit.AbilityKit';
import { PickedVideo } from '../../model/Upload';
import { Logger } from './Logger';

const TAG: string = 'MediaLib';

/** request.uploadFile 的 File.uri 只接受 `internal://cache/<相对 cacheDir 路径>`（SDK 明确限定），故分片副本必须落在 cache 目录 */
const CACHE_SCHEME: string = 'internal://cache/';
/** 分片副本的落盘目录名（相对 cacheDir） */
const STAGING_DIR: string = 'upload_staging';

/**
 * 端侧媒体文件工具：选片、沙箱副本、SHA-256。
 *
 * 关于「为什么必须拷贝一份」（HMY-50 遗留待验项，本卡定论）：
 * SDK 声明 `File.uri` **仅支持 `internal://cache/`**，而选择器返回的是媒体库/文档 URI
 * （`file://media/Photo/…` / `file://docs/…`），二者不同源；`uploadFile` 会把该串当**应用沙箱路径**解析，
 * 直接投递媒体库 URI 必然失败（HMY-50 实测绝对 `file://` 报 `401 GetInternalPath failed`）。
 * 故统一 `fileIo.copy` 到 cache 再做分片，代价是整文件多一次拷贝——换取该路径已被 HMY-50 实测验证。
 */
export class MediaLib {
  /**
   * 从系统选择器选一个视频。
   * 相册与文档各试一次：相册覆盖「刚拍的/相册里的视频」，文档覆盖「下载目录里的 mkv/flv」，
   * 二者都失败才认为是真错误（相册非法参数/能力缺失时抛 401/23800151，不应让用户完全无法选片）。
   * 用户取消是正常路径（返回 null），不抛错。
   */
  static async pickVideo(context: common.Context): Promise<PickedVideo | null> {
    const fromPhoto: PickedVideo | null = await MediaLib.tryPickFromPhoto(context);
    if (fromPhoto !== null) {
      return fromPhoto;
    }
    return await MediaLib.tryPickFromDocument(context);
  }

  private static async tryPickFromPhoto(context: common.Context): Promise<PickedVideo | null> {
    try {
      const options: picker.PhotoSelectOptions = new picker.PhotoSelectOptions();
      options.MIMEType = picker.PhotoViewMIMETypes.VIDEO_TYPE;
      options.maxSelectNumber = 1;
      const result: picker.PhotoSelectResult = await new picker.PhotoViewPicker(context).select(options);
      if (result.photoUris.length === 0) {
        return null;
      }
      return await MediaLib.describeUri(context, result.photoUris[0]);
    } catch (err) {
      const be: BusinessError = err as BusinessError;
      if (MediaLib.isUserCancel(be)) {
        Logger.info(TAG, '用户取消相册选片');
        return null;
      }
      Logger.warn(TAG, `相册选片不可用，回退文档选择器：code=${be.code} ${be.message}`);
      return null;
    }
  }

  private static async tryPickFromDocument(context: common.Context): Promise<PickedVideo | null> {
    const options: picker.DocumentSelectOptions = new picker.DocumentSelectOptions();
    // 后缀过滤与后端白名单一致；文档选择器的过滤项是「只能少选不能多选」的提示，不能当校验用
    options.fileSuffixFilters = MediaLib.suffixFilters();
    options.maxSelectNumber = 1;
    let uris: string[] = [];
    try {
      uris = await new picker.DocumentViewPicker(context).select(options);
    } catch (err) {
      const be: BusinessError = err as BusinessError;
      if (MediaLib.isUserCancel(be)) {
        Logger.info(TAG, '用户取消文档选片');
        return null;
      }
      Logger.error(TAG, `文档选片失败：code=${be.code} ${be.message}`);
      throw new Error(`选片失败（${be.code}）`);
    }
    if (uris.length === 0) {
      return null;
    }
    return await MediaLib.describeUri(context, uris[0]);
  }

  /** 文档选择器的过滤项：`描述|.mp4,.mov`，须与后端 allowedExts 一致 */
  private static suffixFilters(): string[] {
    return ['视频 (.mp4, .mov, .mkv, .flv, .avi)|.mp4,.mov,.mkv,.flv,.avi'];
  }

  /** 用户取消选择器：各选择器取消码不统一，文档侧为 13900020/16000011，统一按 13900020 与「无 uri」兜底 */
  private static isUserCancel(be: BusinessError): boolean {
    return be.code === 13900020 || be.code === 16000011;
  }

  /** 由 uri 补出文件名/扩展名/大小（选择器只给 uri，展示与校验都要靠这三个字段） */
  private static async describeUri(context: common.Context, uri: string): Promise<PickedVideo> {
    const fileName: string = MediaLib.fileNameOf(uri);
    const stat: fileIo.Stat = await fileIo.stat(uri);
    return {
      uri: uri,
      fileName: fileName,
      ext: MediaLib.extOf(fileName),
      size: stat.size
    };
  }

  /** 媒体库 uri 形如 `file://media/Photo/12/IMG_x/VID_x.mp4`，末段即文件名（含转义时先解码） */
  private static fileNameOf(uri: string): string {
    const cut: number = uri.lastIndexOf('/');
    const raw: string = cut >= 0 ? uri.substring(cut + 1) : uri;
    try {
      return decodeURIComponent(raw);
    } catch (err) {
      return raw;
    }
  }

  /** 小写扩展名，不含点；无扩展名返回空串（由调用方按白名单拒绝） */
  static extOf(fileName: string): string {
    const dot: number = fileName.lastIndexOf('.');
    if (dot < 0 || dot === fileName.length - 1) {
      return '';
    }
    return fileName.substring(dot + 1).toLowerCase();
  }

  /**
   * 把选片 uri 拷进应用 cache，返回 `request.uploadFile` 可用的 `internal://cache/…` 字符串。
   * 返回值的相对路径部分同时是该副本的沙箱路径，供 HashStream 算哈希（hash 接口只接受沙箱路径）。
   */
  static async stageToCache(context: common.Context, picked: PickedVideo): Promise<string> {
    const dir: string = `${context.cacheDir}/${STAGING_DIR}`;
    if (!fileIo.accessSync(dir)) {
      fileIo.mkdirSync(dir, true);
    }
    // 以文件名命名（不含目录层级），避免 uri 里的转义字符带进沙箱路径；
    // 重名会被 copy 覆盖，语义上正确——同一文件重复上传本就该复用同一副本。
    const relative: string = `${STAGING_DIR}/${MediaLib.safeName(picked.fileName)}`;
    const destPath: string = `${context.cacheDir}/${relative}`;
    const destUri: string = `${CACHE_SCHEME}${relative}`;

    // 同尺寸且在位的副本直接复用，省掉一次整文件拷贝（重进投稿页续传时会命中）
    if (MediaLib.isStaged(dir, picked)) {
      Logger.info(TAG, `复用已暂存的副本：${relative}`);
      return destUri;
    }

    // fileIo.copy 的目的地必须是**沙箱真实路径**（`internal://cache/…` 只是 uploadFile 认的形态，
    // 文件系统 API 不认它——实测传该形态报 401「The input parameter is invalid」）。
    await MediaLib.copyInto(picked.uri, destPath);
    Logger.info(TAG, `已暂存分片源：${relative} (${picked.size} 字节)`);
    return destUri;
  }

  /** 把选择器 URI 拷到沙箱路径；copy 不接受时退回「打开源 + copyFile(fd, dest)」 */
  private static async copyInto(srcUri: string, destPath: string): Promise<void> {
    try {
      await fileIo.copy(srcUri, destPath);
      return;
    } catch (err) {
      Logger.warn(TAG, `fileIo.copy 失败，改用 copyFile(fd)：${(err as BusinessError).message}`);
    }
    const src: fileIo.File = fileIo.openSync(srcUri, fileIo.OpenMode.READ_ONLY);
    try {
      await fileIo.copyFile(src.fd, destPath);
    } finally {
      fileIo.closeSync(src);
    }
  }

  /** 暂存副本的沙箱路径（算哈希用）；与 stageToCache 的 destPath 同源 */
  static stagedPath(context: common.Context, picked: PickedVideo): string {
    return `${context.cacheDir}/${STAGING_DIR}/${MediaLib.safeName(picked.fileName)}`;
  }

  private static isStaged(dir: string, picked: PickedVideo): boolean {
    const path: string = `${dir}/${MediaLib.safeName(picked.fileName)}`;
    if (!fileIo.accessSync(path)) {
      return false;
    }
    try {
      return fileIo.statSync(path).size === picked.size;
    } catch (err) {
      return false;
    }
  }

  /** 清掉暂存副本（上传收口或用户取消后调用，避免大文件长期占 cache） */
  static clearStaged(context: common.Context, picked: PickedVideo): void {
    const path: string = MediaLib.stagedPath(context, picked);
    try {
      if (fileIo.accessSync(path)) {
        fileIo.unlinkSync(path);
      }
    } catch (err) {
      Logger.warn(TAG, `清理暂存副本失败：${(err as BusinessError).message}`);
    }
  }

  /** 文件名里的路径分隔与非安全字符换成下划线，保证落在同一层目录内 */
  private static safeName(fileName: string): string {
    return fileName.replace(/[\\/:*?"<>|]/g, '_');
  }

  /**
   * 端侧 SHA-256，**返回小写 hex**。
   * 后端 `file_hash` 是 `required,len=64` 且按小写比对（Init 里 `strings.ToLower`），
   * 而 `hash.hash()` 返回的是**大写** hex（见 SDK 声明），故必须转小写——否则秒传永不命中、
   * 且 complete 的合并校验会以「文件校验失败」收场。
   *
   * 用 HashStream 分块喂数据（4MB/次）而不是整文件一次 `hash.hash(path, 'sha256')`：
   * 后者在 8GB 上限的文件上会受端侧单次调用约束，分块则内存恒定。
   */
  static async sha256Chunked(sandboxPath: string, onProgress?: (read: number, total: number) => void): Promise<string> {
    const CHUNK: number = 4 * 1024 * 1024;
    const total: number = fileIo.statSync(sandboxPath).size;
    const stream: hash.HashStream = hash.createHash('sha256');
    const file = fileIo.openSync(sandboxPath, fileIo.OpenMode.READ_ONLY);
    try {
      let read: number = 0;
      while (read < total) {
        const want: number = Math.min(CHUNK, total - read);
        const buffer: ArrayBuffer = new ArrayBuffer(want);
        const options: ReadOptions = { offset: read, length: want };
        const got: number = fileIo.readSync(file.fd, buffer, options);
        if (got <= 0) {
          break;
        }
        stream.update(got === want ? buffer : buffer.slice(0, got));
        read += got;
        if (onProgress !== undefined) {
          onProgress(read, total);
        }
      }
      return stream.digest().toLowerCase();
    } finally {
      fileIo.closeSync(file);
    }
  }
}
