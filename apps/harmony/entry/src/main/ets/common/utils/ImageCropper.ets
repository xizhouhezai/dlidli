import { fileIo } from '@kit.CoreFileKit';
import { image } from '@kit.ImageKit';
import { BusinessError } from '@kit.BasicServicesKit';
import { common } from '@kit.AbilityKit';
import { Logger } from './Logger';

const TAG: string = 'ImageCropper';

/** 封面缓存目录（相对 cacheDir）。与上传暂存目录分开，清理大文件时不会误删封面 */
const COVER_DIR: string = 'upload_cover';
const ASPECT_W: number = 16;
const ASPECT_H: number = 9;
/** 输出上限：1280×720 的 JPEG 约 150~300KB，远低于后端 5MB 上限，也够投稿封面清晰度 */
const MAX_WIDTH: number = 1280;
const MAX_HEIGHT: number = 720;
const JPEG_QUALITY: number = 90;

/** 裁切产物：沙箱路径 + `request.uploadFile` 认的 cache uri + 实际像素 */
export interface CroppedCover {
  sandboxPath: string;
  cacheUri: string;
  width: number;
  height: number;
}

/**
 * 封面 16:9 裁切（HMY-51 端侧差异）。
 *
 * 只用**图片源解码**一条路径完成裁切：`DesiredRegion` 是源图坐标系里的矩形，
 * 配合 `cropAndScaleStrategy = CROP_FIRST` 明确「先裁后缩」——
 * 不指定该策略时，不同图片格式的裁/缩顺序不一致（SDK 明确不保证），JPEG 会先降采样再裁，
 * 结果可能比预期区域大一圈（边界带进主体外的内容）。
 *
 * 视频截帧不在此处：`VideoFrame` 先把帧落成临时 JPEG，再回到本类的 URI 路径，
 * 避免用 `PixelMap.crop`（其 region 的 x/y 偏移语义与图片源不一致）造成两套裁切几何。
 */
export class ImageCropper {
  /**
   * 从任意可打开的图片 uri（相册 `file://media/…`、文档 `file://docs/…`、沙箱路径）裁出 16:9 JPEG。
   * `name` 为不含扩展名的文件主名，重名覆盖（同一次投稿反复换封面不需要留历史）。
   */
  static async fromUri(context: common.Context, srcUri: string, name: string): Promise<CroppedCover> {
    const file: fileIo.File = fileIo.openSync(srcUri, fileIo.OpenMode.READ_ONLY);
    let source: image.ImageSource | null = null;
    try {
      source = image.createImageSource(file.fd);
      const info: image.ImageInfo = await source.getImageInfo();
      const region: image.Region = ImageCropper.regionOf(info.size.width, info.size.height);
      const options: image.DecodingOptions = {
        desiredRegion: region,
        desiredSize: ImageCropper.outputSize(region.size.width, region.size.height),
        cropAndScaleStrategy: image.CropAndScaleStrategy.CROP_FIRST
      };
      const pixelMap: image.PixelMap = await source.createPixelMap(options);
      try {
        return await ImageCropper.pack(context, pixelMap, name);
      } finally {
        await pixelMap.release();
      }
    } finally {
      if (source !== null) {
        await source.release();
      }
      fileIo.closeSync(file);
    }
  }

  /** 裁切产物落盘位置（重进页面时按同名覆盖，调用方无需先清理） */
  static coverPath(context: common.Context, name: string): string {
    return `${context.cacheDir}/${COVER_DIR}/${name}.jpg`;
  }

  /** 删掉本地裁切产物（提交成功或换封面后调用） */
  static remove(sandboxPath: string): void {
    try {
      if (fileIo.accessSync(sandboxPath)) {
        fileIo.unlinkSync(sandboxPath);
      }
    } catch (err) {
      Logger.warn(TAG, `清理封面失败：${(err as BusinessError).message}`);
    }
  }

  /**
   * 源图中心的最大 16:9 矩形。
   * 中心裁切与消费端 `object-fit: cover` 的可视结果一致——端侧预览即用户最终看到的构图。
   */
  private static regionOf(width: number, height: number): image.Region {
    const byWidth: number = Math.floor(width * ASPECT_H / ASPECT_W);
    if (byWidth <= height) {
      return { x: 0, y: Math.floor((height - byWidth) / 2), size: { width: width, height: byWidth } };
    }
    const byHeight: number = Math.floor(height * ASPECT_W / ASPECT_H);
    return { x: Math.floor((width - byHeight) / 2), y: 0, size: { width: byHeight, height: height } };
  }

  /** 只缩不放：源图小于上限时按原尺寸输出，避免无谓的放大与体积膨胀 */
  private static outputSize(width: number, height: number): image.Size {
    if (width <= MAX_WIDTH && height <= MAX_HEIGHT) {
      return { width: width, height: height };
    }
    const scale: number = Math.min(MAX_WIDTH / width, MAX_HEIGHT / height);
    return { width: Math.max(2, Math.floor(width * scale)), height: Math.max(2, Math.floor(height * scale)) };
  }

  /** 统一 JPEG 落盘（后端 coverExts 含 .jpg，且照片/截帧都不需要透明通道） */
  private static async pack(context: common.Context, pixelMap: image.PixelMap, name: string): Promise<CroppedCover> {
    const dir: string = `${context.cacheDir}/${COVER_DIR}`;
    if (!fileIo.accessSync(dir)) {
      fileIo.mkdirSync(dir, true);
    }
    const relative: string = `${COVER_DIR}/${name}.jpg`;
    const sandboxPath: string = `${context.cacheDir}/${relative}`;
    const out: fileIo.File = fileIo.openSync(sandboxPath,
      fileIo.OpenMode.READ_WRITE | fileIo.OpenMode.CREATE | fileIo.OpenMode.TRUNC);
    const packer: image.ImagePacker = image.createImagePacker();
    try {
      const option: image.PackingOption = { format: 'image/jpeg', quality: JPEG_QUALITY };
      await packer.packToFile(pixelMap, out.fd, option);
    } finally {
      fileIo.closeSync(out);
      await packer.release();
    }
    const info: image.ImageInfo = await pixelMap.getImageInfo();
    Logger.info(TAG, `封面已裁切：${relative} ${info.size.width}×${info.size.height}`);
    return {
      sandboxPath: sandboxPath,
      cacheUri: `internal://cache/${relative}`,
      width: info.size.width,
      height: info.size.height
    };
  }
}
