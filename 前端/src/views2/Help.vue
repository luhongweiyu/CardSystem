<template>
  <section class="页面">
    <h2>接入帮助</h2>
    <p class="说明">下面只列出客户接入和管理员自动化会用到的接口；展开接口即可查看参数和返回值。</p>

    <el-card shadow="never" class="说明卡片">
      <h3>通用返回与签名</h3>
      <p class="接口文字">成功响应包含 <code>state: true</code>、<code>code: 1</code> 和业务字段；失败响应包含 <code>state: false</code>、<code>code: 0</code>、<code>msg</code>。</p>
      <p class="接口文字">新客户端接口支持 GET 和 JSON POST。安全模式开启时，<code>sign</code> 放在 URL 参数中：JSON POST 使用 <code>MD5(api_password + 原始JSON)</code>；GET 或无 JSON 的 POST 使用去掉 <code>sign</code> 后的原始查询字符串计算。响应签名放在 JSON 的 <code>sign</code> 字段。</p>
    </el-card>

    <el-card shadow="never" class="说明卡片">
      <h3>点卡客户端接口</h3>
      <details class="接口项">
        <summary>登录 · GET/POST <code>/point_card/card_login</code></summary>
        <div class="接口内容">
          <p><strong>参数：</strong><code>center_id</code> 或 <code>name</code>、<code>card</code>；可选 <code>device_id</code>、<code>device_alias</code>、<code>period_minutes</code>、<code>prefer_reuse</code>。</p>
          <p><strong>说明：</strong><code>software</code> 由卡密记录自动确定；省略 <code>device_id</code> 按空字符串处理，后续心跳必须保持一致；<code>period_minutes</code> 为 0 或省略时，已有设备沿用上次周期，新设备使用软件默认周期。</p>
          <p><strong>成功返回：</strong><code>needle</code>、<code>authorized_until</code>、<code>heartbeat_interval_seconds</code>。</p>
          <p><strong>授权复用：</strong>只有软件允许且提交 <code>prefer_reuse=true</code>（也支持 1），才优先接手同卡离线最久的未到期授权，不扣点、不延长截止时间；没有可用授权则正常扣费。软件未开启或参数未传时保持原行为。</p>
        </div>
      </details>
      <details class="接口项">
        <summary>心跳 · GET/POST <code>/point_card/card_ping</code></summary>
        <div class="接口内容">
          <p><strong>参数：</strong><code>center_id</code> 或 <code>name</code>、<code>card</code>、<code>needle</code>、<code>device_id</code>。</p>
          <p><strong>说明：</strong><code>device_id</code> 必须与登录时一致，不提交 <code>device_alias</code>。</p>
          <p><strong>成功返回：</strong><code>needle</code>、<code>authorized_until</code>、<code>heartbeat_interval_seconds</code>。</p>
        </div>
      </details>
      <details class="接口项">
        <summary>退出 · GET/POST <code>/point_card/card_logout</code></summary>
        <div class="接口内容">
          <p><strong>参数：</strong><code>center_id</code> 或 <code>name</code>、<code>card</code>；可选 <code>device_id</code>、<code>needle</code>。</p>
          <p><strong>成功返回：</strong><code>msg</code>；退出后立即释放对应设备会话。</p>
        </div>
      </details>
      <details class="接口项">
        <summary>计费方案 · GET/POST <code>/point_card/period_prices</code></summary>
        <div class="接口内容">
          <p><strong>参数：</strong><code>center_id</code> 或 <code>name</code>、<code>card</code>；可选 <code>software</code>。</p>
          <p><strong>成功返回：</strong><code>data</code>（含 <code>period_minutes</code>、<code>cost</code>、<code>is_default</code>）、<code>software</code>、<code>default_period_minutes</code>、<code>heartbeat_interval_seconds</code>、<code>online_grace_minutes</code>。</p>
        </div>
      </details>
      <details class="接口项">
        <summary>流水查询 · GET/POST <code>/point_card/point_ledger/query</code></summary>
        <div class="接口内容">
          <p><strong>参数：</strong><code>center_id</code> 或 <code>name</code>、<code>card</code>；可选 <code>software</code>、<code>page</code>、<code>page_size</code>。</p>
          <p><strong>成功返回：</strong><code>data</code>、<code>num</code>、<code>page</code>、<code>page_size</code>、<code>balance</code>。</p>
        </div>
      </details>
    </el-card>

    <el-card shadow="never" class="说明卡片">
      <h3>时长卡客户端接口</h3>
      <details class="接口项">
        <summary>登录 · GET/POST <code>/duration_card/card_login</code></summary>
        <div class="接口内容">
          <p><strong>参数：</strong><code>center_id</code> 或 <code>name</code>、<code>card</code>。</p>
          <p><strong>说明：</strong>软件和固定时长由时长卡记录确定，首次登录会激活卡密。</p>
          <p><strong>成功返回：</strong><code>needle</code>、<code>authorized_until</code>、<code>software</code>、<code>heartbeat_interval_seconds</code>。</p>
        </div>
      </details>
      <details class="接口项">
        <summary>心跳 · GET/POST <code>/duration_card/card_ping</code></summary>
        <div class="接口内容">
          <p><strong>参数：</strong><code>center_id</code> 或 <code>name</code>、<code>card</code>、<code>needle</code>。</p>
          <p><strong>成功返回：</strong><code>needle</code>、<code>authorized_until</code>、<code>heartbeat_interval_seconds</code>。</p>
        </div>
      </details>
      <details class="接口项">
        <summary>退出 · GET/POST <code>/duration_card/card_logout</code></summary>
        <div class="接口内容">
          <p><strong>参数：</strong><code>center_id</code> 或 <code>name</code>、<code>card</code>；可选 <code>needle</code>。</p>
          <p><strong>成功返回：</strong><code>msg</code>；只清除在线校验，不改变剩余时长。</p>
        </div>
      </details>
      <details class="接口项">
        <summary>卡密互充 · GET/POST <code>/duration_card/recharge</code></summary>
        <div class="接口内容">
          <p><strong>参数：</strong><code>center_id</code> 或 <code>name</code>、当前目标 <code>card</code>、来源 <code>source_card</code>（旧参数名 <code>card2</code> 也可用）。</p>
          <p><strong>成功返回：</strong><code>msg</code>、<code>added_minutes</code>、<code>authorized_until</code>；暂停卡的 <code>authorized_until</code> 为 null。</p>
        </div>
      </details>
      <details class="接口项">
        <summary>独立充值卡 · POST <code>/visitor/duration_recharge_card/query</code> / <code>/visitor/duration_recharge_card/redeem</code></summary>
        <div class="接口内容">
          <p><strong>查询参数：</strong><code>center_id</code>、充值卡 <code>card</code>。成功返回充值卡软件、分钟数、剩余次数、有效期和状态。</p>
          <p><strong>使用参数：</strong><code>center_id</code>、<code>recharge_card</code>（旧字段 <code>Rechargeable_card</code> 也可用）、目标卡数组 <code>cards</code>。</p>
          <p><strong>使用成功返回：</strong><code>success</code>、<code>paused</code>、<code>failed</code>、<code>remaining_uses</code>。</p>
        </div>
      </details>
      <details class="接口项">
        <summary>暂停与恢复 · POST <code>/visitor/duration_card/pause</code> / <code>/visitor/duration_card/resume</code></summary>
        <div class="接口内容">
          <p><strong>参数：</strong><code>center_id</code>、<code>card</code>。</p>
          <p><strong>成功返回：</strong>暂停返回 <code>remaining_minutes</code>；恢复返回 <code>authorized_until</code>。</p>
        </div>
      </details>
      <details v-if="!是代理账号" class="接口项">
        <summary>旧客户端兼容 · GET/POST <code>/card/card_login</code> / <code>/card/card_ping</code></summary>
        <div class="接口内容">
          <p><strong>说明：</strong>仅供旧时长卡客户端使用。请求签名为 <code>MD5(timestamp + api_password)</code>，响应时间戳为请求时间戳加 10。</p>
          <p><strong>返回：</strong>沿用旧客户端响应字段和旧签名格式。</p>
        </div>
      </details>
    </el-card>

    <el-card v-if="!是代理账号" shadow="never" class="说明卡片">
      <h3>管理员自动化接口</h3>
      <details class="接口项">
        <summary>代理余额充值 · POST <code>/admin/代理账号充值</code></summary>
        <div class="接口内容">
          <p><strong>参数：</strong><code>id</code>（代理账号编号，必填）、<code>amount</code>（余额变更点数，整数；正数充值、负数扣款，0 表示不变更，必填）、<code>note</code>（可选备注，最多 200 个字符）。</p>
          <p><strong>成功返回：</strong><code>msg</code>、充值后的 <code>balance</code>。</p>
        </div>
      </details>
    </el-card>
  </section>
</template>

<script setup>
import { storeToRefs } from 'pinia'
import { use登录状态Store } from '../stores/登录状态.js'

const stores = use登录状态Store()
const { 是代理账号 } = storeToRefs(stores)

// 帮助页直接展示当前实现的接口，不依赖外部文档站点，避免链接失效。
</script>

<style scoped>
.页面 {
  padding: 16px;
  color: #e6eaf2;
}
h2 {
  margin-top: 0;
}
h3 {
  margin: 0 0 14px;
}
.说明 {
  color: #aeb6c3;
}
.说明卡片 {
  max-width: 980px;
  margin-bottom: 16px;
}
.接口文字,
.接口内容 {
  color: #aeb6c3;
  line-height: 1.8;
}
.接口文字 {
  margin: 8px 0;
}
.接口项 {
  border-top: 1px solid #364052;
  padding: 12px 0;
}
.接口项:last-child {
  padding-bottom: 0;
}
.接口项 summary {
  cursor: pointer;
  color: #e6eaf2;
  font-weight: 600;
}
.接口内容 {
  padding: 8px 4px 0;
}
.接口内容 p {
  margin: 4px 0;
}
code {
  color: #8ec5ff;
  font-family: Consolas, 'Courier New', monospace;
}
</style>
