import { preferences } from '@kit.ArkData';
import { PreferenceStore } from '../store/PreferenceStore';
import { Logger } from '../common/utils/Logger';

const TAG: string = 'DmSettings';
/** 偏好键名见 plan §3 存储设计 */
const KEY_SETTINGS: string = 'danmaku.settings';

/** 显示区域：1/4 屏 / 半屏 / 全屏（对齐 Web 端 useDanmakuSettings） */
export type DmArea = 'quarter' | 'half' | 'full';
/** 滚动速度档：对应单条滚动弹幕跨屏耗时 12s / 9s / 6s */
export type DmSpeed = 'slow' | 'normal' | 'fast';
/** 同屏密度档：作为轨道数的乘子 0.7 / 1 / 1.3 */
export type DmDensity = 'low' | 'normal' | 'high';

const AREAS: DmArea[] = ['quarter', 'half', 'full'];
const SPEEDS: DmSpeed[] = ['slow', 'normal', 'fast'];
const DENSITIES: DmDensity[] = ['low', 'normal', 'high'];

/** 落盘形态（与 Web 端 localStorage 结构一致，便于后期跨端统一） */
interface DanmakuSettingsData {
  enabled: boolean;
  opacity: number;
  fontScale: number;
  area: DmArea;
  speed: DmSpeed;
  density: DmDensity;
}

/**
 * 弹幕展示设置（DM-11 开关记忆 + DM-12 展示项）。
 *
 * 与 Web 端的差异：Web 只持久化展示项、**不记忆开关**；本端按 DM-11 的「并记忆状态」
 * 把 `enabled` 一并落盘——关弹幕后退出再进仍是关的，符合用户对「一键关闭」的预期。
 *
 * 为 @ObservedV2 单例：设置面板改档位后，播放页的开关图标/面板高亮需即时跟随。
 */
@ObservedV2
export class DanmakuSettings {
  /** 弹幕总开关（DM-11） */
  @Trace enabled: boolean = true;
  /** 不透明度 0.2~1 */
  @Trace opacity: number = 0.8;
  /** 字号缩放 0.8~1.5 */
  @Trace fontScale: number = 1;
  @Trace area: DmArea = 'full';
  @Trace speed: DmSpeed = 'normal';
  @Trace density: DmDensity = 'normal';

  private loaded: boolean = false;

  /** 读取偏好；首次进入或读取失败一律用默认值，不阻塞播放 */
  async load(): Promise<void> {
    if (this.loaded) {
      return;
    }
    this.loaded = true;
    const store: preferences.Preferences | null = await PreferenceStore.open();
    if (store === null) {
      return;
    }
    try {
      const raw: string = await store.get(KEY_SETTINGS, '') as string;
      if (raw.length === 0) {
        return;
      }
      this.apply(JSON.parse(raw) as DanmakuSettingsData);
    } catch (err) {
      Logger.error(TAG, `读取弹幕设置失败，按默认值处理：${JSON.stringify(err)}`);
    }
  }

  /** 任一档位变更后调用（设置面板在每次改动后调一次，避免 watch 深比较） */
  persist(): void {
    const data: DanmakuSettingsData = {
      enabled: this.enabled,
      opacity: this.opacity,
      fontScale: this.fontScale,
      area: this.area,
      speed: this.speed,
      density: this.density
    };
    PreferenceStore.open().then((store: preferences.Preferences | null) => {
      if (store === null) {
        return;
      }
      store.put(KEY_SETTINGS, JSON.stringify(data))
        .then(() => store.flush())
        .catch((err: Object) => {
          Logger.error(TAG, `弹幕设置持久化失败（仅内存生效）：${JSON.stringify(err)}`);
        });
    }).catch((err: Object) => {
      Logger.error(TAG, `打开偏好存储失败：${JSON.stringify(err)}`);
    });
  }

  /** 字号基数：与 Web 端同口径（服务端默认 25 归入标准档） */
  baseFontSize(fontSize: number): number {
    return fontSize <= 18 ? DM_FONT_SMALL : DM_FONT_NORMAL;
  }

  /** 显示区域占画面高度的比例 */
  areaRatio(): number {
    if (this.area === 'quarter') {
      return 0.25;
    }
    return this.area === 'half' ? 0.5 : 0.85;
  }

  densityRatio(): number {
    if (this.density === 'low') {
      return 0.7;
    }
    return this.density === 'high' ? 1.3 : 1;
  }

  /** 滚动弹幕跨屏耗时（毫秒） */
  scrollDurationMs(): number {
    if (this.speed === 'slow') {
      return 12000;
    }
    return this.speed === 'fast' ? 6000 : 9000;
  }

  /** 落盘数据回填：逐项夹取，避免手改偏好文件把界面搞崩 */
  private apply(data: DanmakuSettingsData): void {
    this.enabled = data.enabled !== false;
    this.opacity = clamp(data.opacity, 0.2, 1, 0.8);
    this.fontScale = clamp(data.fontScale, 0.8, 1.5, 1);
    this.area = AREAS.includes(data.area) ? data.area : 'full';
    this.speed = SPEEDS.includes(data.speed) ? data.speed : 'normal';
    this.density = DENSITIES.includes(data.density) ? data.density : 'normal';
  }
}

/** 弹幕字号（vp）：小档/标准档，最终字号 = 基数 × 设置面板缩放 */
export const DM_FONT_SMALL: number = 15;
export const DM_FONT_NORMAL: number = 20;

function clamp(value: number, min: number, max: number, fallback: number): number {
  if (typeof value !== 'number' || Number.isNaN(value)) {
    return fallback;
  }
  return Math.max(min, Math.min(max, value));
}

/** 模块级单例：@ObservedV2 实例不可在 build 内创建 */
export const dmSettings: DanmakuSettings = new DanmakuSettings();
