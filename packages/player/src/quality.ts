import type { PlayerSource } from './types'

/**
 * 清晰度展示文案。
 *
 * 约定（M4-VIP-02）：`quality` 是**档位编号**，不是分辨率——0 为原画，其余为
 * 360/720/1080/2160（2160 即 4K）。4K 单独给标签而不是拼成「2160P」，
 * 因为 HLS 的 2160 行按行数命名，业内与用户认的都是「4K / 超清」。
 */
export function qualityLabel(quality: number): string {
  if (quality === 0) return '原画'
  if (quality >= 2160) return '4K'
  return `${quality}P`
}

/** 默认清晰度：优先最高档 HLS，无 HLS 时回退首个源。 */
export function pickDefaultSource(sources: PlayerSource[]): PlayerSource | null {
  return sources.find((s) => s.format === 'hls') ?? sources[0] ?? null
}
