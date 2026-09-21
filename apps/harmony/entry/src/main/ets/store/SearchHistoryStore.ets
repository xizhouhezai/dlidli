import { preferences } from '@kit.ArkData';
import { PreferenceStore } from './PreferenceStore';
import { Logger } from '../common/utils/Logger';

const TAG: string = 'SearchHistory';
const KEY_HISTORY: string = 'search.local_history';
const MAX_ITEMS: number = 10;

/**
 * 搜索历史（端侧本地，key 见 plan §3 的 search.local_history）。
 *
 * SRH-04 要求「本地 + 云端同步」，但后端全仓未提供搜索历史接口（M2-SRH-04 亦标注「历史待补」），
 * 本期零后端改动，故只落本地副本；云端同步待后端接口就绪后在此补一对拉取/上报。
 */
export class SearchHistoryStore {
  private static items: string[] = [];
  private static loaded: boolean = false;

  /** 最近搜索在前，最多 MAX_ITEMS 条 */
  static async list(): Promise<string[]> {
    await SearchHistoryStore.load();
    return SearchHistoryStore.items;
  }

  /** 命中已有词时提到最前，不产生重复条目 */
  static async add(keyword: string): Promise<void> {
    const kw: string = keyword.trim();
    if (kw.length === 0) {
      return;
    }
    await SearchHistoryStore.load();
    const next: string[] = SearchHistoryStore.items.filter((item: string) => item !== kw);
    next.unshift(kw);
    SearchHistoryStore.items = next.slice(0, MAX_ITEMS);
    await SearchHistoryStore.persist();
  }

  static async remove(keyword: string): Promise<void> {
    await SearchHistoryStore.load();
    SearchHistoryStore.items = SearchHistoryStore.items.filter((item: string) => item !== keyword);
    await SearchHistoryStore.persist();
  }

  static async clear(): Promise<void> {
    await SearchHistoryStore.load();
    SearchHistoryStore.items = [];
    await SearchHistoryStore.persist();
  }

  private static async load(): Promise<void> {
    if (SearchHistoryStore.loaded) {
      return;
    }
    SearchHistoryStore.loaded = true;
    const store: preferences.Preferences | null = await PreferenceStore.open();
    if (store === null) {
      return;
    }
    try {
      const raw: string = await store.get(KEY_HISTORY, '') as string;
      SearchHistoryStore.items = raw.length > 0 ? JSON.parse(raw) as string[] : [];
    } catch (err) {
      Logger.error(TAG, `读取搜索历史失败，按空历史处理：${JSON.stringify(err)}`);
      SearchHistoryStore.items = [];
    }
  }

  private static async persist(): Promise<void> {
    const store: preferences.Preferences | null = await PreferenceStore.open();
    if (store === null) {
      return;
    }
    try {
      await store.put(KEY_HISTORY, JSON.stringify(SearchHistoryStore.items));
      await store.flush();
    } catch (err) {
      Logger.error(TAG, `搜索历史持久化失败（仅内存生效）：${JSON.stringify(err)}`);
    }
  }
}
