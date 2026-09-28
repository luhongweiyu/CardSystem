const 最大时长分钟 = 36500 * 24 * 60
const 最大计价数量 = 500000
const 最大整数 = 9223372036854775807n
const 每点单位 = 100n

const 安全乘法 = (left, right) => {
  const result = left * right
  return result <= 最大整数 ? result : null
}

const 向上整除 = (numerator, denominator) =>
  numerator / denominator + (numerator % denominator === 0n ? 0n : 1n)

// 只使用代理已缓存的启用价格，在浏览器本地按服务端整数分规则预估；
// 最终扣款仍由服务端在事务中重新计价。
export const 计算代理时长报价 = (rows, requestedMinutes, count) => {
  if (!Number.isSafeInteger(requestedMinutes) || requestedMinutes < 5 || requestedMinutes > 最大时长分钟) {
    return { error: '生成时长不在允许范围内' }
  }
  if (!Number.isSafeInteger(count) || count <= 0 || count > 最大计价数量) {
    return { error: `计价数量必须在1至${最大计价数量}之间` }
  }

  const active = []
  const seen = new Set()
  for (const row of rows || []) {
    if (row.enabled === false) continue
    const duration = Number(row.duration_minutes)
    const price = Number(row.price)
    const scaledPrice = price * 100
    const priceCents = Math.round(scaledPrice)
    if (
      !Number.isSafeInteger(duration) || duration < 5 || duration > 最大时长分钟 ||
      !Number.isFinite(price) || price <= 0 || price > 1000000000 ||
      Math.abs(scaledPrice - priceCents) > 1e-8
    ) {
      return { error: '时长卡价格配置不正确' }
    }
    if (seen.has(duration)) return { error: '时长卡价格锚点重复' }
    seen.add(duration)
    active.push({ duration, priceCents: BigInt(priceCents) })
  }
  if (!active.length) return { error: '该软件暂无可用的时长卡价格' }
  active.sort((left, right) => left.duration - right.duration)

  const exact = active.find((row) => row.duration === requestedMinutes)
  if (exact) {
    const numerator = 安全乘法(exact.priceCents, BigInt(count))
    if (numerator === null) return { error: '预计消费金额超出允许范围' }
    return {
      charge: 向上整除(numerator, 每点单位),
      pricePerCard: Number(exact.priceCents) / 100
    }
  }

  if (requestedMinutes < active[0].duration || requestedMinutes > active[active.length - 1].duration) {
    return { error: `时长需在${active[0].duration}至${active[active.length - 1].duration}分钟之间` }
  }
  let lower
  let upper
  for (const row of active) {
    if (row.duration < requestedMinutes) lower = row
    if (row.duration > requestedMinutes) {
      upper = row
      break
    }
  }
  if (!lower || !upper) return { error: '没有可用的相邻价格锚点' }

  const leftProduct = 安全乘法(lower.priceCents, BigInt(upper.duration))
  const rightProduct = 安全乘法(upper.priceCents, BigInt(lower.duration))
  if (leftProduct === null || rightProduct === null) return { error: '预计消费金额超出允许范围' }
  const source = rightProduct > leftProduct ? upper : lower
  const targetNumerator = 安全乘法(BigInt(requestedMinutes), source.priceCents)
  if (targetNumerator === null) return { error: '预计消费金额超出允许范围' }
  const batchNumerator = 安全乘法(targetNumerator, BigInt(count))
  const denominator = 安全乘法(BigInt(source.duration), 每点单位)
  if (batchNumerator === null || denominator === null) return { error: '预计消费金额超出允许范围' }

  const unitDenominator = BigInt(source.duration)
  const pricePerCardCents = (targetNumerator * 2n + unitDenominator) / (unitDenominator * 2n)
  return {
    charge: 向上整除(batchNumerator, denominator),
    pricePerCard: Number(pricePerCardCents) / 100
  }
}

export const 格式化代理点数 = (amount) => amount.toLocaleString('zh-CN')
