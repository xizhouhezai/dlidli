import { preferences } from '@kit.ArkData';
import { PreferenceStore } from './PreferenceStore';
import { Logger } from '../common/utils/Logger';

const TAG: string = 'PlaybackProgress';
const KEY_PROGRESS: string = 'playback.local_progress';
/** 只留最近这些稿件：进度缓存是「服务端拿不到时的兜底」，不是历史记录 */
const MAX_ENTRIES: number = 50;

/** 单条本地进度 */
interface LocalProgress {
  position: number;
  updatedAt: number;
}

/**
 * 本地观看进度缓存（key 见 plan §3 的 playback.local_progress）。
 *
 * 服务端进度为准（PLY-04 跨端续播），本地这份用于：未登录、或读服务端进度失败的场景，
 * 让用户回到同一台设备时仍能接着看。未登录用户的进度只存在这里。
 */
export class PlaybackProgressStore {
  private static entries: Map<string, LocalProgress> = new Map<string, LocalProgress>();
  private static loaded: boolean = false;

  static async get(bvid: string): Promise<number> {
    await PlaybackProgressStore.load();
    const entry: LocalProgress | undefined = PlaybackProgressStore.entries.get(bvid);
    return entry === undefined ? 0 : entry.position;
  }

  static async save(bvid: string, position: number): Promise<void> {
    const pos: number = Math.max(0, Math.floor(position));
    await PlaybackProgressStore.load();
    PlaybackProgressStore.entries.set(bvid, { position: pos, updatedAt: Date.now() });
    PlaybackProgressStore.evict();
    await PlaybackProgressStore.persist();
  }

  /** 超出上限时按 updatedAt 淘汰最旧的（Map 保持插入序，重写已存在键不会前移，故需显式排序） */
  private static evict(): void {
    if (PlaybackProgressStore.entries.size <= MAX_ENTRIES) {
      return;
    }
    const sorted: string[] = Array.from(PlaybackProgressStore.entries.keys());
    sorted.sort((a: string, b: string) => {
      const ea: LocalProgress | undefined = PlaybackProgressStore.entries.get(a);
      const eb: LocalProgress | undefined = PlaybackProgressStore.entries.get(b);
      return (ea?.updatedAt ?? 0) - (eb?.updatedAt ?? 0);
    });
    const drop: number = sorted.length - MAX_ENTRIES;
    for (let i = 0; i < drop; i++) {
      PlaybackProgressStore.entries.delete(sorted[i]);
    }
  }

  private static async load(): Promise<void> {
    if (PlaybackProgressStore.loaded) {
      return;
    }
    PlaybackProgressStore.loaded = true;
    const store: preferences.Preferences | null = await PreferenceStore.open();
    if (store === null) {
      return;
    }
    try {
      const raw: string = await store.get(KEY_PROGRESS, '') as string;
      if (raw.length === 0) {
        return;
      }
      const parsed: Record<string, LocalProgress> = JSON.parse(raw) as Record<string, LocalProgress>;
      const map: Map<string, LocalProgress> = new Map<string, LocalProgress>();
      Object.keys(parsed).forEach((key: string) => {
        const item: LocalProgress = parsed[key];
        if (item !== undefined && typeof item.position === 'number') {
          map.set(key, item);
        }
      });
      PlaybackProgressStore.entries = map;
    } catch (err) {
      Logger.error(TAG, `读取本地进度失败，按空处理：${JSON.stringify(err)}`);
      PlaybackProgressStore.entries = new Map<string, LocalProgress>();
    }
  }

  private static async persist(): Promise<void> {
    const store: preferences.Preferences | null = await PreferenceStore.open();
    if (store === null) {
      return;
    }
    const plain: Record<string, LocalProgress> = {};
    PlaybackProgressStore.entries.forEach((value: LocalProgress, key: string) => {
      plain[key] = value;
    });
    try {
      await store.put(KEY_PROGRESS, JSON.stringify(plain));
      await store.flush();
    } catch (err) {
      Logger.error(TAG, `本地进度持久化失败（仅内存生效）：${JSON.stringify(err)}`);
    }
  }
}
