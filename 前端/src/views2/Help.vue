<template>
  <section class="页面">
    <h2>接入帮助</h2>
    <p class="说明">点卡和时长卡都按“登录 → 按心跳间隔发送心跳 → 退出”接入，但使用不同接口前缀。</p>
    <el-card shadow="never" class="说明卡片">
      <el-descriptions :column="1" border>
        <el-descriptions-item label="登录">
          GET/POST /point_card/card_login，提交 card；可选 device_id、period_minutes 和 device_alias，软件由卡密自动确定。
        </el-descriptions-item>
        <el-descriptions-item label="心跳">
          GET/POST /point_card/card_ping，提交登录返回的 needle，device_id 必须与登录时保持一致；不提交 device_alias。
        </el-descriptions-item>
        <el-descriptions-item label="退出">
          GET/POST /point_card/card_logout，提交 card；device_id 可省略或传空值，软件由卡密和设备会话自动确定；可同时提交 needle
          做附加校验，退出后立即释放设备会话。
        </el-descriptions-item>
        <el-descriptions-item label="点卡计费方案">
          GET/POST /point_card/period_prices，可读取当前软件启用的授权时长和扣点数。
        </el-descriptions-item>
        <el-descriptions-item label="流水查询">
          GET/POST /point_card/point_ledger/query，只能查询当前卡密自己的点数流水。
        </el-descriptions-item>
        <el-descriptions-item label="时长卡登录">
          GET/POST /duration_card/card_login，提交 card；软件从时长卡记录读取，首次登录会激活固定时长。
        </el-descriptions-item>
        <el-descriptions-item label="时长卡心跳">
          GET/POST /duration_card/card_ping，提交登录返回的 needle；时长卡模式同一时间只保留最后一次登录的 needle。
        </el-descriptions-item>
        <el-descriptions-item label="时长卡退出">
          GET/POST /duration_card/card_logout，提交 card 和 needle；退出只清除当前在线校验，不改变剩余时长。
        </el-descriptions-item>
        <el-descriptions-item label="时长卡充值">
          GET/POST /duration_card/recharge，当前卡提交 source_card，使用另一张同软件未激活时长卡充值；暂停卡会累加暂停剩余分钟。
        </el-descriptions-item>
        <el-descriptions-item label="独立充值卡">
          POST /visitor/duration_recharge_card/query 查询充值卡；POST /visitor/duration_recharge_card/redeem 提交 recharge_card 和 cards，给已激活或暂停中的同软件时长卡充值。
        </el-descriptions-item>
        <el-descriptions-item label="暂停与恢复">
          POST /visitor/duration_card/pause 或 /visitor/duration_card/resume，提交 center_id 和 card；暂停费用由软件设置决定。
        </el-descriptions-item>
      </el-descriptions>
      <p class="提示">
        device_id 可省略，省略时按空字符串处理；使用非空值时应由客户端生成并持久化。device_alias
        只是登录时设置的展示名称，不参与设备唯一性判断。开启接口安全模式后，POST JSON 签原始 JSON；GET 或无 JSON 的 POST 签去除 sign 后的原始查询字符串，
        两种请求都通过地址传 sign，并提交 timestamp、nonce。响应 JSON 会返回相同 nonce 和顶层 sign。计费周期按分钟，心跳间隔仍按秒；签名不提供传输加密。
      </p>
    </el-card>
  </section>
</template>

<script setup>
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
.说明 {
  color: #aeb6c3;
}
.说明卡片 {
  max-width: 980px;
}
.提示 {
  margin: 18px 0 0;
  color: #aeb6c3;
  line-height: 1.8;
}
</style>
