// 服务端按“本月、上月”拼接正序文本；按时间排序才能让跨月日志也保持最新在前。
export const 日志倒序 = function (content) {
  const entries = []
  const unparsed = []
  for (const line of String(content || '').split(/\r?\n/)) {
    if (!line.trim() || line.trim() === '没有其他内容') continue
    const match = line.match(/^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}):/)
    if (match) {
      entries.push({ text: line, time: match[1], order: entries.length })
    } else if (entries.length) {
      entries[entries.length - 1].text += `\n${line}`
    } else {
      unparsed.push(line)
    }
  }
  entries.sort((a, b) => b.time.localeCompare(a.time) || b.order - a.order)
  return [...entries.map((item) => item.text), ...unparsed].join('\n')
}
