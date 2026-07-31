import type { PlayerSource } from './types'

/** 清晰度展示文案：0=原画，其余为 `${q}P`。 */
export function qualityLabel(quality: number): string {
  return quality === 0 ? '原画' : `${quality}P`
}

/** 默认清晰度：优先最高档 HLS，无 HLS 时回退首个源。 */
export function pickDefaultSource(sources: PlayerSource[]): PlayerSource | null {
  return sources.find((s) => s.format === 'hls') ?? sources[0] ?? null
}
