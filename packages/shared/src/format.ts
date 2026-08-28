/** 计数展示：12345 -> "1.2万" */
export function formatCount(n: number): string {
  if (n < 10000) return String(n)
  if (n < 100000000) return `${(n / 10000).toFixed(1).replace(/\.0$/, '')}万`
  return `${(n / 100000000).toFixed(1).replace(/\.0$/, '')}亿`
}

/** 时长展示：秒 -> "mm:ss" 或 "hh:mm:ss" */
export function formatDuration(seconds: number): string {
  const s = Math.max(0, Math.floor(seconds))
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = s % 60
  const mm = String(m).padStart(2, '0')
  const ss = String(sec).padStart(2, '0')
  return h > 0 ? `${h}:${mm}:${ss}` : `${mm}:${ss}`
}

/** 发布时间展示：相对时间（3 天内）或日期 */
export function formatPubdate(iso: string): string {
  const t = new Date(iso).getTime()
  if (Number.isNaN(t)) return ''
  const diff = Date.now() - t
  const minute = 60_000
  if (diff < minute) return '刚刚'
  if (diff < 60 * minute) return `${Math.floor(diff / minute)}分钟前`
  if (diff < 24 * 60 * minute) return `${Math.floor(diff / (60 * minute))}小时前`
  if (diff < 3 * 24 * 60 * minute) return `${Math.floor(diff / (24 * 60 * minute))}天前`
  const d = new Date(t)
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

/** 后台时间展示：ISO -> "YYYY-MM-DD HH:mm:ss"（本地时区）；空值/非法值返回 fallback */
export function formatDateTime(iso: string | null | undefined, fallback = '—'): string {
  if (!iso) return fallback
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return fallback
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
}
