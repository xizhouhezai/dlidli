/**
 * 展示格式化：与 Web 端 packages/shared/src/format.ts 同源移植。
 * 端侧不消费 TS 包（见 plan §1），故按 ArkTS 重写，取值口径保持一致。
 */

function pad2(n: number): string {
  return n < 10 ? `0${n}` : `${n}`;
}

/** 去掉 toFixed 产生的小数尾零：1.0万 → 1万 */
function trimZero(text: string): string {
  return text.endsWith('.0') ? text.substring(0, text.length - 2) : text;
}

/** 计数展示：12345 -> "1.2万" */
export function formatCount(n: number): string {
  if (n < 10000) {
    return `${n}`;
  }
  if (n < 100000000) {
    return `${trimZero((n / 10000).toFixed(1))}万`;
  }
  return `${trimZero((n / 100000000).toFixed(1))}亿`;
}

/** 时长展示：秒 -> "mm:ss" 或 "hh:mm:ss" */
export function formatDuration(seconds: number): string {
  const s: number = Math.max(0, Math.floor(seconds));
  const h: number = Math.floor(s / 3600);
  const m: number = Math.floor((s % 3600) / 60);
  const mm: string = pad2(m);
  const ss: string = pad2(s % 60);
  return h > 0 ? `${h}:${mm}:${ss}` : `${mm}:${ss}`;
}

/**
 * 发布时间展示：3 天内相对时间，更早显示日期。
 * 服务端为 Go 的 RFC3339Nano，毫秒位可能出现 6/9 位小数，而 ECMAScript 的 ISO 解析只认 3 位，
 * 故先截断到 3 位再解析，避免直接 NaN 导致时间整行丢失。
 */
export function formatPubdate(iso: string): string {
  if (iso.length === 0) {
    return '';
  }
  const t: number = Date.parse(iso.replace(/\.(\d{3})\d+/, '.$1'));
  if (Number.isNaN(t)) {
    return '';
  }
  const minute: number = 60000;
  const diff: number = Date.now() - t;
  if (diff < minute) {
    return '刚刚';
  }
  if (diff < 60 * minute) {
    return `${Math.floor(diff / minute)}分钟前`;
  }
  if (diff < 24 * 60 * minute) {
    return `${Math.floor(diff / (60 * minute))}小时前`;
  }
  if (diff < 3 * 24 * 60 * minute) {
    return `${Math.floor(diff / (24 * 60 * minute))}天前`;
  }
  const d: Date = new Date(t);
  return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`;
}
