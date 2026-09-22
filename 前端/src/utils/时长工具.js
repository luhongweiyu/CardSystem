// 时长页面共用的纯展示函数和固定选项。接口请求、代理列表和软件列表
// 仍由各页面维护，避免工具函数绑定具体页面状态。
export const 最小计费周期分钟 = 5
export const 最大计费周期分钟 = 3 * 24 * 60
export const 最小时长分钟 = 5
export const 永久时长分钟 = 36500 * 24 * 60
export const 最大时长分钟 = 永久时长分钟

export const 时长预设 = Object.freeze([
  { label: '半日卡', value: 720 },
  { label: '日卡', value: 1440 },
  { label: '半周卡', value: 5040 },
  { label: '周卡', value: 10080 },
  { label: '半月卡', value: 21600 },
  { label: '月卡', value: 43200 },
  { label: '季卡', value: 131040 },
  { label: '半年卡', value: 262080 },
  { label: '年卡', value: 525600 },
  { label: '永久卡', value: 永久时长分钟 }
])

// 时长优先按天、小时、分钟组合显示，避免 1500 分钟只显示成“1500 分钟”。
export const 格式化时长 = (minutes) => {
  const value = Number(minutes)
  if (!Number.isFinite(value) || value <= 0) return '-'
  if (value === 永久时长分钟) return '永久卡'

  let remaining = Math.trunc(value)
  const days = Math.floor(remaining / 1440)
  remaining %= 1440
  const hours = Math.floor(remaining / 60)
  const minutesPart = remaining % 60
  const parts = []
  if (days) parts.push(`${days}天`)
  if (hours) parts.push(`${hours}小时`)
  if (minutesPart) parts.push(`${minutesPart}分`)
  return parts.join('') || '-'
}

export const 格式化时间 = (value) => {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime()) || date.getFullYear() <= 1) return ''
  return date.toLocaleString('zh-CN')
}

export const 格式化代理归属 = (id, agents = []) => {
  const agentID = Number(id || 0)
  if (!agentID) return '管理员'
  const agent = agents.find((item) => Number(item.id) === agentID)
  return agent ? `${agent.name}(${agentID})` : `代理#${agentID}(${agentID})`
}

export const 查找软件名称 = (id, softwares = []) => {
  return softwares.find((item) => Number(item.ID) === Number(id))?.Software || `软件#${id}`
}
