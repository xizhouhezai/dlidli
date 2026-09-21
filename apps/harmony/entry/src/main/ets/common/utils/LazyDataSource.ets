/**
 * LazyForEach 的数据源：首页信息流与搜索结果共用。
 * 只暴露「整表替换」与「增量追加」两种变更形态——分页列表的变更不外乎这两类，
 * 变更通知按官方 BasicDataSource 的写法逐项下发（同一帧内的多次通知会被合并成一次布局）。
 */
export class LazyDataSource<T> implements IDataSource {
  private listeners: DataChangeListener[] = [];
  private items: T[] = [];

  totalCount(): number {
    return this.items.length;
  }

  getData(index: number): T {
    return this.items[index];
  }

  registerDataChangeListener(listener: DataChangeListener): void {
    if (this.listeners.indexOf(listener) < 0) {
      this.listeners.push(listener);
    }
  }

  unregisterDataChangeListener(listener: DataChangeListener): void {
    const pos: number = this.listeners.indexOf(listener);
    if (pos >= 0) {
      this.listeners.splice(pos, 1);
    }
  }

  /** 切换分区/排序、下拉刷新、重新搜索时使用 */
  reset(items: T[]): void {
    this.items = items;
    this.listeners.forEach((listener: DataChangeListener) => {
      listener.onDataReloaded();
    });
  }

  /** 触底加载下一页时使用 */
  append(items: T[]): void {
    items.forEach((item: T) => {
      this.items.push(item);
      const index: number = this.items.length - 1;
      this.listeners.forEach((listener: DataChangeListener) => {
        listener.onDataAdd(index);
      });
    });
  }

  count(): number {
    return this.items.length;
  }
}
