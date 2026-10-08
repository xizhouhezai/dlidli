import { fileIo } from '@kit.CoreFileKit';
import { image } from '@kit.ImageKit';
import { media } from '@kit.MediaKit';
import { BusinessError } from '@kit.BasicServicesKit';
import { common } from '@kit.AbilityKit';
import { Logger } from './Logger';

const TAG: string = 'VideoFrame';

/** 截帧请求宽度：≤ 源视频宽度时系统按比例缩放，故 4K 源也只产出 1280 宽的位图 */
const FRAME_WIDTH: number = 1280;
/**
 * 截帧时间点取 1s 而非 0s：片头常为淡入黑帧，0s 的封面会是一整块黑。
 * 与后端转码抽帧取「10% 进度、最多 3s」的意图一致，但端侧不引入时长解析（元数据抽取是另一套 API）。
 */
const FRAME_TIME_US: number = 1000000;
/** 与 ImageCropper 共用封面目录：临时帧裁完即删 */
const TEMP_DIR: string = 'upload_cover';

/**
 * 视频截帧（HMY-51 端侧差异的第二种封面来源）。
 *
 * 只负责「取一帧并落成临时 JPEG」，**不做裁切**：裁切统一交给 `ImageCropper` 的图片源路径，
 * 避免 `PixelMap.crop` 与图片源 `desiredRegion` 两套几何各说各话。
 *
 * 容器/编码不支持时返回空串（mkv/flv 等），由调用方静默回退到「相册选图」或后端自动抽帧——
 * 与 Web 端 `captureVideoPoster` 的静默回退同策略，不让截帧失败挡住投稿。
 */
export class VideoFrame {
  /** 截第 1s 处的关键帧到 `${cacheDir}/upload_cover/<outName>.jpg`，返回沙箱路径；失败返回空串 */
  static async captureToCache(context: common.Context, videoPath: string, outName: string): Promise<string> {
    const size: number = fileIo.statSync(videoPath).size;
    const file: fileIo.File = fileIo.openSync(videoPath, fileIo.OpenMode.READ_ONLY);
    let generator: media.AVImageGenerator | null = null;
    try {
      generator = await media.createAVImageGenerator();
      const descriptor: media.AVFileDescriptor = { fd: file.fd, offset: 0, length: size };
      generator.fdSrc = descriptor;
      const param: media.PixelMapParams = { width: FRAME_WIDTH };
      const pixelMap: image.PixelMap =
        await generator.fetchFrameByTime(FRAME_TIME_US, media.AVImageQueryOptions.AV_IMAGE_QUERY_CLOSEST_SYNC, param);
      try {
        return await VideoFrame.pack(context, pixelMap, outName);
      } finally {
        await pixelMap.release();
      }
    } catch (err) {
      Logger.warn(TAG, `截帧不可用，回退相册选图：${(err as BusinessError).message}`);
      return '';
    } finally {
      if (generator !== null) {
        await generator.release();
      }
      fileIo.closeSync(file);
    }
  }

  private static async pack(context: common.Context, pixelMap: image.PixelMap, outName: string): Promise<string> {
    const dir: string = `${context.cacheDir}/${TEMP_DIR}`;
    if (!fileIo.accessSync(dir)) {
      fileIo.mkdirSync(dir, true);
    }
    const path: string = `${dir}/${outName}.jpg`;
    const out: fileIo.File = fileIo.openSync(path,
      fileIo.OpenMode.READ_WRITE | fileIo.OpenMode.CREATE | fileIo.OpenMode.TRUNC);
    const packer: image.ImagePacker = image.createImagePacker();
    try {
      // 中间帧不参与最终产物（还要被 ImageCropper 再编一次），故取 100 少丢一代
      const option: image.PackingOption = { format: 'image/jpeg', quality: 100 };
      await packer.packToFile(pixelMap, out.fd, option);
    } finally {
      fileIo.closeSync(out);
      await packer.release();
    }
    return path;
  }
}
