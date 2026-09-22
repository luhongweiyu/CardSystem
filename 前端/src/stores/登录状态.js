import { ref, watch } from 'vue'
import { defineStore } from 'pinia'
import { ElMessage } from 'element-plus'
import apiClient from '../api/请求客户端.js'

export const use登录状态Store = defineStore('登录状态', () => {
  // 密码不落地；有效令牌只保存在当前标签页的 sessionStorage，刷新可继续使用。
  const 用户id = ref('')
  const 导航开关 = ref(false)
  const 账号 = ref('')
  const 密码 = ref('')
  const 登录状态 = ref(false)
  const api次数 = ref(0)
  const 是代理账号 = ref(false)
  const 账号信息 = ref({})
  const token = ref('')
  const 登录到期时间 = ref('')
  const 软件列表 = ref([])
  const 软件列表响应 = ref(null)
  const 软件列表已加载 = ref(false)
  const 代理列表 = ref([])
  const 代理列表已加载 = ref(false)
  const 代理时长价格列表 = ref([])
  const 代理时长价格已加载 = ref(false)
  let 软件列表请求 = null
  let 代理列表请求 = null
  let 代理时长价格请求 = null
  let 公共缓存版本 = 0
  let 请求序号 = 0
  const 账号字段版本 = {}

  // 每次请求复制业务参数，避免向页面中的响应式表单对象写入账号或令牌字段。
  const post = function (链接, 参数) {
    const 请求参数 = { ...(参数 || {}) }
    if (!Object.prototype.hasOwnProperty.call(请求参数, 'name') && 账号.value) {
      请求参数.name = 账号.value
    }
    if (token.value) {
      请求参数.token = token.value
    }
    // 登录接口由登录页直接调用；其他接口只发送令牌，不重复发送明文密码。
    const 前缀 = 是代理账号.value ? '/agent' : '/admin'
    const requestVersion = 公共缓存版本
    const sequence = ++请求序号
    const agent = 是代理账号.value
    return apiClient.post(前缀 + 链接, 请求参数).then((response) => {
      // 只清理发出此请求的会话，避免旧请求的失败响应踢掉刚登录的新账号。
      if (token.value === 请求参数.token && requestVersion === 公共缓存版本 &&
          response.data?.state === false &&
          ['登录已过期，请重新登录', '登录会话与账号不匹配'].includes(response.data.msg)) {
        清理登录状态()
      }
      // 余额和代扣设置只接收当前会话的真实响应，不能在切页时用软件缓存回填旧值。
      // 较早发出的慢请求也不能覆盖较新请求已经返回的同一字段。
      if (agent && requestVersion === 公共缓存版本 && response.data?.state) {
        for (const key of ['balance', 'prices', 'point_card_auto_deduct', 'allow_point_debt', 'point_debt_limit']) {
          if (response.data[key] !== undefined && sequence >= (账号字段版本[key] || 0)) {
            账号信息.value[key] = response.data[key]
            账号字段版本[key] = sequence
          }
        }
      }
      return response
    }).catch((error) => {
      if (error.response?.status === 401 && token.value === 请求参数.token && requestVersion === 公共缓存版本) {
        清理登录状态()
      }
      throw error
    })
  }

  // 软件和代理属于管理端多个页面共用的基础数据。首次请求后放入 Pinia，
  // 页面切换直接复用；force=true 供新增、修改或删除后主动刷新。
  const 查询软件列表 = function (force = false) {
    if (!force && 软件列表请求) return 软件列表请求
    if (!force && 软件列表已加载.value) return Promise.resolve(软件列表响应.value)
    const requestVersion = 公共缓存版本
    const request = post('/user_query_soft_list', {})
      .then((response) => {
        if (!response.data?.state) throw new Error(response.data?.msg || '查询软件失败')
        if (requestVersion !== 公共缓存版本 || 软件列表请求 !== request) return response.data
        软件列表.value = response.data.data || []
        软件列表响应.value = { state: true, data: 软件列表.value }
        软件列表已加载.value = true
        return response.data
      })
      .finally(() => {
        if (软件列表请求 === request) 软件列表请求 = null
      })
    软件列表请求 = request
    return request
  }

  const 查询代理列表 = function (force = false) {
    if (是代理账号.value) return Promise.resolve([])
    if (!force && 代理列表请求) return 代理列表请求
    if (!force && 代理列表已加载.value) return Promise.resolve(代理列表.value)
    const requestVersion = 公共缓存版本
    const request = post('/查询代理账号', {})
      .then((response) => {
        if (!response.data?.state) throw new Error(response.data?.msg || '查询代理账号失败')
        if (requestVersion !== 公共缓存版本 || 代理列表请求 !== request) return 代理列表.value
        代理列表.value = response.data.data || []
        代理列表已加载.value = true
        return 代理列表.value
      })
      .finally(() => {
        if (代理列表请求 === request) 代理列表请求 = null
      })
    代理列表请求 = request
    return request
  }

  // 小伙伴的时长卡价格是多个页面共用的只读数据。登录后首次读取并缓存，
  // 页面切换直接复用；手动刷新时传 force=true。
  const 查询代理时长价格列表 = function (force = false) {
    if (!是代理账号.value) return Promise.resolve([])
    if (代理时长价格请求) return 代理时长价格请求
    if (!force && 代理时长价格已加载.value) return Promise.resolve(代理时长价格列表.value)
    const requestVersion = 公共缓存版本
    const request = post('/duration_card/price/list', {})
      .then((response) => {
        if (!response.data?.state) throw new Error(response.data?.msg || '查询时长卡价格失败')
        if (requestVersion !== 公共缓存版本 || 代理时长价格请求 !== request) return response.data.data || []
        代理时长价格列表.value = response.data.data || []
        代理时长价格已加载.value = true
        return 代理时长价格列表.value
      })
      .finally(() => {
        if (代理时长价格请求 === request) 代理时长价格请求 = null
      })
    代理时长价格请求 = request
    return request
  }

  const 清理公共缓存 = function () {
    公共缓存版本++
    软件列表.value = []
    软件列表响应.value = null
    软件列表已加载.value = false
    代理列表.value = []
    代理列表已加载.value = false
    代理时长价格列表.value = []
    代理时长价格已加载.value = false
    软件列表请求 = null
    代理列表请求 = null
    代理时长价格请求 = null
  }

  const 清理登录状态 = function () {
    清理公共缓存()
    登录状态.value = false
    token.value = ''
    登录到期时间.value = ''
    密码.value = ''
    用户id.value = ''
    api次数.value = 0
    是代理账号.value = false
    账号信息.value = {}
    // 主动退出时立即删除，不等待 Vue 的批量更新。
    try { sessionStorage.removeItem('card_system_session') } catch { /* 浏览器禁用存储时仍允许登录。 */ }
  }

  const 检查登录到期 = function () {
    if (登录状态.value && Date.parse(登录到期时间.value) <= Date.now()) {
      清理登录状态()
      ElMessage.warning('登录已过期，请重新登录')
    }
  }

  try {
    const saved = JSON.parse(sessionStorage.getItem('card_system_session') || 'null')
    if (saved?.token && saved.name && Date.parse(saved.expires_at) > Date.now()) {
      账号.value = saved.name
      token.value = saved.token
      登录到期时间.value = saved.expires_at
      用户id.value = saved.id
      是代理账号.value = saved.agent === true
      账号信息.value = { center_id: saved.center_id }
      登录状态.value = true
    } else {
      sessionStorage.removeItem('card_system_session')
    }
  } catch { /* 损坏或禁用的存储不影响手动登录。 */ }

  let 到期定时器
  watch([登录状态, token, 登录到期时间], () => {
    clearTimeout(到期定时器)
    try {
      if (登录状态.value && token.value) {
        // 只保存恢复会话必需的字段，不持久化密码、业务余额和接口安全码。
        sessionStorage.setItem('card_system_session', JSON.stringify({
          name: 账号.value, id: 用户id.value, agent: 是代理账号.value,
          center_id: 账号信息.value.center_id, token: token.value, expires_at: 登录到期时间.value
        }))
      } else {
        sessionStorage.removeItem('card_system_session')
      }
    } catch { /* 禁用存储时退回内存会话。 */ }
    const remaining = Date.parse(登录到期时间.value) - Date.now()
    if (登录状态.value && Number.isFinite(remaining)) {
      到期定时器 = setTimeout(检查登录到期, Math.max(0, Math.min(remaining, 2147483647)))
    }
  }, { immediate: true })

  return {
    post,
    查询软件列表,
    查询代理列表,
    查询代理时长价格列表,
    清理公共缓存,
    清理登录状态,
    检查登录到期,
    登录到期时间,
    登录状态,
    账号,
    密码,
    token,
    用户id,
    导航开关,
    api次数,
    是代理账号,
    账号信息,
    软件列表,
    代理列表,
    代理时长价格列表
  }
})
