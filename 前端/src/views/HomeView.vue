<template>
  <section class="导航页">
    <div class="欢迎区">
      <div class="欢迎内容">
        <div class="产品标识">
          <span class="产品图标"><el-icon><Postcard /></el-icon></span>
          <span>卡密管理工作台</span>
        </div>
        <h1>{{ 是代理账号 ? '小伙伴工作台' : '卡密管理工作台' }}</h1>
        <p>
          {{
            是代理账号
              ? '从这里管理名下点卡、查看接入方式和运行记录。'
              : '从这里快速进入点卡、时长卡、软件计费和系统设置。'
          }}
        </p>
      </div>

      <div class="账号摘要">
        <el-avatar class="账号头像" :size="46">
          <el-icon><UserFilled /></el-icon>
        </el-avatar>
        <div>
          <span>{{ 是代理账号 ? '小伙伴账号' : '管理员账号' }}</span>
          <strong>{{ 账号 || '未命名账号' }}</strong>
        </div>
      </div>
    </div>

    <div class="概览区">
      <div class="概览项">
        <span class="概览标签">当前身份</span>
        <strong>{{ 是代理账号 ? '小伙伴' : '管理员' }}</strong>
        <small>{{ 是代理账号 ? '管理名下点卡' : '管理全部业务' }}</small>
      </div>
      <div class="概览项">
        <span class="概览标签">{{ 是代理账号 ? '可用余额' : '本小时请求' }}</span>
        <strong>{{ 是代理账号 ? `${账号信息.balance ?? 0} 点` : `${api次数} 次` }}</strong>
        <small>{{ 是代理账号 ? '用于生成点卡' : '接口请求统计' }}</small>
      </div>
      <div class="概览项 概览提示">
        <span class="概览标签">使用提示</span>
        <strong>{{ 是代理账号 ? '先查看软件设置' : '先配置软件' }}</strong>
        <small>{{ 是代理账号 ? '确认价格与点卡代扣开关' : '配置完成后再批量生成卡密' }}</small>
      </div>
    </div>

    <div class="功能标题行">
      <div>
        <h2>常用功能</h2>
        <p>选择一个入口开始操作</p>
      </div>
      <el-tag effect="plain" type="info">{{ 功能菜单.length }} 个入口</el-tag>
    </div>

    <div class="功能网格">
      <button v-for="item in 功能菜单" :key="item.path" type="button" class="功能卡片" @click="前往(item.path)">
        <span class="功能图标" :class="item.color">
          <el-icon><component :is="item.icon" /></el-icon>
        </span>
        <span class="功能文字">
          <strong>{{ item.label }}</strong>
          <span>{{ item.description }}</span>
        </span>
        <el-icon class="功能箭头"><ArrowRight /></el-icon>
      </button>
    </div>

    <div class="底部提示">
      <el-icon><InfoFilled /></el-icon>
      <span>点卡余额、设备授权和时长卡到期时间均以服务端数据为准。</span>
    </div>
  </section>
</template>

<script setup>
import { computed } from 'vue'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'
import { use登录状态Store } from '../stores/登录状态.js'

const router = useRouter()
const stores = use登录状态Store()
const { 账号, api次数, 是代理账号, 账号信息 } = storeToRefs(stores)

const 功能菜单 = computed(() => {
  const 通用入口 = [
    { path: '/card', label: '点卡管理', description: '生成、查询、冻结或删除卡密', icon: 'Postcard', color: '蓝色' },
    { path: '/duration-card', label: '时长卡管理', description: '生成、查询和续费固定时长卡', icon: 'Timer', color: '紫色' },
    { path: '/duration-recharge-card', label: '时长充值卡', description: '生成和管理可重复使用的充值卡', icon: 'CreditCard', color: '橙色' },
    { path: '/help', label: '接入帮助', description: '查看登录、心跳和签名参数', icon: 'QuestionFilled', color: '紫色' },
    { path: '/log', label: '运行日志', description: '查看当前账号的操作记录', icon: 'List', color: '橙色' },
    { path: '/about', label: '关于系统', description: '查看版本和支持信息', icon: 'InfoFilled', color: '灰色' }
  ]
  if (是代理账号.value) {
    return [
      { path: '/software', label: '软件设置', description: '查看软件价格和点卡代扣设置', icon: 'Iphone', color: '绿色' },
      ...通用入口
    ]
  }
  return [
    ...通用入口.slice(0, 3),
    { path: '/software', label: '软件管理', description: '配置软件和点卡计费方案', icon: 'Iphone', color: '绿色' },
    { path: '/point-ledger', label: '点数流水', description: '查看所有余额变动记录', icon: 'Tickets', color: '青色' },
    { path: '/setting', label: '账号设置', description: '管理公告、联系方式和安全选项', icon: 'Setting', color: '粉色' },
    ...通用入口.slice(3)
  ]
})

const 前往 = function (path) {
  router.push(path)
}
</script>

<style scoped>
.导航页 {
  min-height: 100%;
  padding: clamp(20px, 4vw, 42px);
  color: #e6eaf2;
  background:
    radial-gradient(circle at 88% 0%, rgba(64, 123, 209, 0.16), transparent 34%),
    #20242d;
}

.欢迎区 {
  position: relative;
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 24px;
  max-width: 1180px;
  margin: 0 auto 14px;
  padding: clamp(20px, 3vw, 30px);
  overflow: hidden;
  border: 1px solid rgba(130, 156, 201, 0.25);
  border-radius: 20px;
  background: linear-gradient(135deg, rgba(46, 62, 88, 0.96), rgba(31, 37, 49, 0.96));
  box-shadow: 0 18px 50px rgba(0, 0, 0, 0.16);
}

.欢迎区::after {
  position: absolute;
  right: -90px;
  bottom: -135px;
  width: 300px;
  height: 300px;
  border: 1px solid rgba(116, 170, 255, 0.18);
  border-radius: 50%;
  box-shadow: 0 0 0 28px rgba(116, 170, 255, 0.04), 0 0 0 58px rgba(116, 170, 255, 0.025);
  content: '';
  pointer-events: none;
}

.欢迎内容,
.账号摘要 {
  position: relative;
  z-index: 1;
}

.产品标识 {
  display: flex;
  align-items: center;
  gap: 9px;
  color: #a9c8f5;
  font-size: 13px;
  letter-spacing: 0.08em;
}

.产品图标 {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 27px;
  height: 27px;
  border-radius: 8px;
  color: #fff;
  background: #4b8fe8;
}

h1,
h2,
p {
  margin: 0;
}

h1 {
  margin-top: 9px;
  color: #fff;
  font-size: clamp(26px, 4vw, 38px);
  line-height: 1.2;
}

.欢迎内容 p {
  max-width: 620px;
  margin-top: 8px;
  color: #bdc8d8;
  line-height: 1.8;
}

.账号摘要 {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 180px;
  padding: 10px 14px;
  border: 1px solid rgba(174, 198, 232, 0.18);
  border-radius: 13px;
  background: rgba(13, 18, 27, 0.28);
}

.账号头像 {
  flex: none;
  color: #dceaff;
  background: #386ba9;
}

.账号摘要 div {
  display: flex;
  flex-direction: column;
  min-width: 0;
  gap: 2px;
}

.账号摘要 span {
  color: #aab8cd;
  font-size: 12px;
}

.账号摘要 strong {
  overflow: hidden;
  color: #fff;
  font-size: 15px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.概览区 {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14px;
  max-width: 1180px;
  margin: 0 auto 26px;
}

.概览项 {
  display: flex;
  flex-direction: column;
  min-height: 98px;
  padding: 16px 20px;
  border: 1px solid #343d4d;
  border-radius: 14px;
  background: rgba(35, 41, 53, 0.82);
}

.概览标签 {
  color: #8f9caf;
  font-size: 13px;
}

.概览项 strong {
  margin-top: 6px;
  color: #f2f6fc;
  font-size: 21px;
  line-height: 1.3;
}

.概览项 small {
  margin-top: 3px;
  color: #7f8b9d;
  font-size: 12px;
}

.概览提示 {
  border-color: rgba(74, 144, 226, 0.28);
  background: linear-gradient(135deg, rgba(45, 75, 112, 0.62), rgba(35, 41, 53, 0.82));
}

.功能标题行 {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 12px;
  max-width: 1180px;
  margin: 0 auto 15px;
}

.功能标题行 h2 {
  color: #f2f5fa;
  font-size: 21px;
}

.功能标题行 p {
  margin-top: 4px;
  color: #8f9caf;
  font-size: 13px;
}

.功能网格 {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
  max-width: 1180px;
  margin: 0 auto;
}

.功能卡片 {
  display: flex;
  align-items: center;
  gap: 14px;
  min-height: 92px;
  padding: 16px;
  border: 1px solid #343d4d;
  border-radius: 14px;
  color: inherit;
  text-align: left;
  background: rgba(35, 41, 53, 0.82);
  cursor: pointer;
  transition: border-color 0.18s ease, background 0.18s ease, transform 0.18s ease;
}

.功能卡片:hover,
.功能卡片:focus-visible {
  border-color: #5d8dcc;
  outline: none;
  background: rgba(43, 53, 70, 0.96);
  transform: translateY(-2px);
}

.功能图标 {
  display: inline-flex;
  flex: none;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  border-radius: 12px;
  color: #fff;
  font-size: 21px;
}

.蓝色 { background: #397bc7; }
.绿色 { background: #318e78; }
.青色 { background: #2f8d9d; }
.紫色 { background: #7653ad; }
.粉色 { background: #ae547e; }
.橙色 { background: #b87538; }
.灰色 { background: #5d6a7b; }

.功能文字 {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-width: 0;
  gap: 4px;
}

.功能文字 strong {
  color: #f1f5fb;
  font-size: 16px;
}

.功能文字 span {
  overflow: hidden;
  color: #9eaabd;
  font-size: 12px;
  line-height: 1.5;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.功能箭头 {
  flex: none;
  color: #68778c;
  transition: transform 0.18s ease, color 0.18s ease;
}

.功能卡片:hover .功能箭头,
.功能卡片:focus-visible .功能箭头 {
  color: #a8c9f5;
  transform: translateX(3px);
}

.底部提示 {
  display: flex;
  align-items: center;
  gap: 8px;
  max-width: 1180px;
  margin: 22px auto 0;
  color: #78869a;
  font-size: 12px;
}

@media (max-width: 850px) {
  .欢迎区 {
    align-items: stretch;
    flex-direction: column;
  }

  .账号摘要 {
    align-self: flex-start;
  }

  .功能网格 {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 600px) {
  .导航页 {
    padding: 16px;
  }

  .欢迎区 {
    padding: 20px 18px;
    border-radius: 16px;
  }

  .概览区,
  .功能网格 {
    grid-template-columns: 1fr;
  }

  .概览区 {
    margin-bottom: 28px;
  }
}
</style>
