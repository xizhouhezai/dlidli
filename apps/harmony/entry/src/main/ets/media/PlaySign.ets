/** 播放地址里的过期参数（后端 playsign.Query 生成 `?e=<exp>&s=<sig>`） */
const EXP_PARAM: RegExp = /[?&]e=(\d+)/;

/**
 * 取签名播放地址的过期时刻（毫秒）；解析不出返回 0。
 * 签名只覆盖资源相对路径与过期时间、不含 host，故地址 host 重写（loopback → ApiConfig.BASE_URL，
 * 见 common/utils/MediaUrl）不影响签名有效性。
 */
export function signExpiryMs(url: string): number {
  const matched: RegExpMatchArray | null = url.match(EXP_PARAM);
  if (matched === null) {
    return 0;
  }
  return Number.parseInt(matched[1], 10) * 1000;
}
