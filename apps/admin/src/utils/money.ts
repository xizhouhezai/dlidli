// 金额换算（分 ↔ 元）。
//
// **后端所有金额字段单位都是分**（整数，spec §6.1 明确禁用浮点）。页面展示与输入
// 用的是元，两边的换算只允许走本文件 —— 散落在各页面里手写 `v / 100` 与
// `Number(x) * 100` 是本项目最容易出错的点，典型失效有三类：
//
//  1. `19.99 * 100` 得到 `1998.9999999999998`，直接取整会少 1 分；
//  2. 元输入没做小数位校验，`1.005` 这种三分位价格会被静默截断或进位；
//  3. 展示用 `toFixed` 之后又参与计算，字符串拼接出 `"15.000.00"`。
//
// 故：展示走整数运算（Math.round + 补零），输入走字符串解析（正则 + 整数拼接），
// 全链路不出现 `Number(元字符串) * 100` 这种写法。

/** 元输入的小数位上限：人民币最小单位是分，故最多两位。 */
const MAX_FRACTION_DIGITS = 2

/**
 * 分 → 元字符串（固定两位小数，展示用）。
 *
 * 例：`1500 → "15.00"`、`5 → "0.05"`、`0 → "0.00"`、`21000 → "210.00"`。
 *
 * 用整数运算而非 `(v / 100).toFixed(2)`：后者先做一次浮点除法，
 * 对超大值（分超过 2^53）与边界值会引入误差；这里全程整数，结果可预期。
 * 负数（退款/冲正场景）单独取绝对值处理符号，避免 `-5/100` 的取整方向歧义。
 */
export function fenToYuan(fen: number | null | undefined): string {
  if (fen === null || fen === undefined || Number.isNaN(fen)) return '0.00'
  const neg = fen < 0
  const abs = Math.abs(Math.trunc(fen))
  const yuan = Math.floor(abs / 100)
  const cents = abs % 100
  const text = `${yuan}.${String(cents).padStart(2, '0')}`
  return neg ? `-${text}` : text
}

/**
 * 分 → 元数字（需要参与表单输入或数值比较时用；展示一律用 fenToYuan）。
 *
 * 注意返回的是 JS number，超过 2^53 分（约 9e13 元）会丢精度 —— 本项目单笔上限
 * 100 万元（1e8 分），远低于该阈值，故不引入 BigInt。
 */
export function fenToYuanNumber(fen: number | null | undefined): number {
  if (fen === null || fen === undefined || Number.isNaN(fen)) return 0
  const neg = fen < 0
  const abs = Math.abs(Math.trunc(fen))
  const value = Math.floor(abs / 100) + (abs % 100) / 100
  return neg ? -value : value
}

/** 元输入解析结果。 */
export type YuanParseResult =
  { ok: true; fen: number } | { ok: false; reason: 'empty' | 'format' | 'fraction' | 'negative' }

/**
 * 元字符串 → 分（表单提交用）。
 *
 * 全整数实现：把整数部分与补齐到两位的小数部分拼成字符串再转整数，
 * 因此 `"19.99"` 得到 `1999`，而不是 `Number("19.99") * 100 = 1998.999...`。
 *
 * 拒绝三类输入：非数字格式、超过两位小数（`1.005` 元无法用分精确表示）、负数
 * （金额字段不接受负值；冲正走独立接口）。调用方需按 `reason` 给出针对性提示，
 * 静默取整会让运营以为「填了就是按填的算」。
 */
export function yuanToFen(input: string | number | null | undefined): YuanParseResult {
  if (input === null || input === undefined) return { ok: false, reason: 'empty' }
  const text = String(input).trim()
  if (text === '') return { ok: false, reason: 'empty' }
  if (text.startsWith('-')) return { ok: false, reason: 'negative' }

  const m = /^(\d+)(?:\.(\d+))?$/.exec(text)
  if (!m) return { ok: false, reason: 'format' }
  const intPart = m[1]
  const fracPart = m[2] ?? ''
  if (fracPart.length > MAX_FRACTION_DIGITS) return { ok: false, reason: 'fraction' }

  const cents = fracPart.padEnd(MAX_FRACTION_DIGITS, '0')
  const fen = Number(intPart) * 100 + Number(cents)
  if (!Number.isSafeInteger(fen)) return { ok: false, reason: 'format' }
  return { ok: true, fen }
}

/** 把 yuanToFen 的失败原因转成可直接展示的提示文案。 */
export function yuanParseErrorText(reason: 'empty' | 'format' | 'fraction' | 'negative'): string {
  switch (reason) {
    case 'empty':
      return '请输入金额'
    case 'fraction':
      return '金额最多保留两位小数'
    case 'negative':
      return '金额不能为负数'
    default:
      return '金额格式不正确'
  }
}
