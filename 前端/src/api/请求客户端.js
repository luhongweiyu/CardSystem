import axios from 'axios'

// 默认使用当前站点作为 API 地址，生产环境可通过 VITE_API_BASE_URL 覆盖。
// 使用相对地址后，HTTPS、反向代理和自定义端口均不再需要修改业务组件。
const configuredBaseURL = (import.meta.env.VITE_API_BASE_URL || '').trim().replace(/\/$/, '')

const apiClient = axios.create({
  baseURL: configuredBaseURL || window.location.origin,
  timeout: 15000,
  headers: {
    'Content-Type': 'application/json'
  }
})

// 获取接口错误提示，避免各页面重复判断 Axios 错误结构。
export function 获取接口错误提示(error) {
  return error?.response?.data?.msg || error?.message || '网络请求失败，请稍后重试'
}

// 与后端使用同一套卡密白名单。返回空字符串表示输入不是一张完整卡密。
export function 规范化卡密(value) {
  const card = String(value ?? '')
    .trim()
    .toLowerCase()
  return /^[a-z0-9_-]{7,63}$/.test(card) ? card : ''
}

export default apiClient
