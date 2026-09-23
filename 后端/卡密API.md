# 点卡与时长卡 API

本文记录客户端、管理端和代理端需要遵守的接口契约。成功响应通常包含 `state: true, code: 1`；失败响应包含 `state: false, code: 0, msg`。JSON 请求参数只从正文读取。

## 通用协议

新客户端使用 `/point_card/*`、`/duration_card/*`。登录、心跳、退出、配置等经过请求验签的接口须提交 `timestamp`、`nonce` 和 URL 参数 `sign`。JSON POST 的签名为 `MD5(api_password + 原始JSON)`；GET 或非 JSON POST 的签名为 `MD5(api_password + 去掉 sign 后的原始查询字符串)`。安全模式开启时校验请求签名；关闭时仍校验时间戳和 nonce。点卡查询、计费方案、公告和流水等只读接口不校验请求签名，但响应仍包含 `sign`；客户端将响应中的 `sign` 置空后按相同规则计算。

旧签名协议用于时长卡 `/card/card_login`、`/card/card_ping`：始终校验时间戳；开启安全模式时，请求签名为 `MD5(timestamp + api_password)`，响应签名为 `MD5(timestamp + api_password + code)`，不要求 nonce。旧签名不覆盖业务参数。新旧服务不要同时写入同一个数据库。

## 点卡客户端

### 计费方案

`GET/POST /point_card/period_prices`：提交 `center_id/name`、`card`，可选 `software`。返回卡密所属软件启用的计费方案、默认授权时长、心跳间隔和自动离线时间。计费时长按分钟表示，范围为 15 分钟至 30 天。

### 登录

`GET/POST /point_card/card_login`：提交 `center_id/name`、`card`；可选 `device_id`、`device_alias`、`period_minutes`、`prefer_reuse`。

- `software` 从卡密记录读取，无需提交。`device_id` 省略时按空字符串处理；非空值应由客户端生成并持久化。`device_alias` 仅登录时使用，最长 64 个字符。
- `period_minutes` 省略或为 `0` 时，新会话使用软件默认时长；已有会话沿用上次选择的时长。显式时长必须对应启用的计费方案。
- `prefer_reuse` 默认 `false`。只有请求和软件设置都允许时，才复用同卡已离线、未到期的授权；复用保留原到期时间，不扣点，旧设备凭证失效。没有可复用授权时正常计费。
- 每张卡最多保留 1000 个设备会话。同一管理员、卡密和设备 ID 只有一条会话；当前授权未到期时重复登录不重复扣点。

成功响应包含 `needle`、`authorized_until`、`heartbeat_interval_seconds` 和本次请求的 `nonce`。

### 心跳

`GET/POST /point_card/card_ping`：提交 `center_id/name`、`card`、`needle`，以及与登录一致的 `device_id`。心跳不接收 `device_alias`。

授权未到期时不扣点。到期后，服务端以 `last_heartbeat_at + online_grace_minutes` 判断设备是否仍可能在线：仍在窗口内时从原授权截止时间续期，否则从当前时间开始。余额不足或计费方案不可用时，过期会话失效；后台清理与客户端心跳使用相同的续期规则。自动离线时间为 `0` 时按 60 分钟处理。

响应包含新的 `authorized_until`、`heartbeat_interval_seconds`、`needle` 和本次请求的 `nonce`。心跳间隔最大为 86400 秒。

### 退出、查询和流水

- `GET/POST /point_card/card_logout`：提交 `center_id/name`、`card`，可选 `device_id`、`needle`。删除对应设备会话，不退款；省略 `device_id` 按空字符串处理。
- `GET/POST /point_card/query`：查询余额、状态和设备会话；支持 `page`、`page_size`，默认每页 50 条，最多 100 条。设备项包含设备标识、授权截止时间、在线状态和心跳令牌。
- `GET/POST /point_card/point_ledger/query`：查询当前卡密流水，支持分页；流水保留 30 天。
- `GET/POST /point_card/bulletin`：读取软件公告。
- `GET/POST /point_card/config`：读写卡密配置，最多 200 个字符。

点卡流水只记录余额变化，字段为 `created_at`、`event_type`、`change`、`balance_before`、`balance_after`、`remark`。生成卡密会写初始流水，包括 0 点卡；删除并重用同名卡密时，30 天保留期内的新旧流水可能同时出现。

## 点卡管理与代理

管理员接口使用 `/admin` 前缀，代理接口使用 `/agent` 前缀。

- 点卡方案：`/point_card/price/list|save|delete`。
- 点卡管理：`/point_card/create|list|save|delete|state`；管理员可用 `/point_card/adjust` 调整余额、`/point_card/ledger` 查询流水。
- 设备下线：`POST /point_card/device/offline`，提交 `card`、`device_id`、详情中的 `needle`。下线停止后台续费并使旧凭证失效；未到期授权仍保留，设备可重新登录。
- 代理账号：`/创建代理账号`、`/设置代理账号`、`/查询代理账号`、`/删除代理账号`、`/代理账号充值`。

每次最多生成 500 张点卡，允许 0 点。代理价格以软件 ID 为键，表示生成 1 点卡点数所需的余额；未配置价格的软件不能发卡。价格最多两位小数，批次总额向上取整。

代理点卡代扣设置：

- `POST /agent/point_card/settings` 的 `point_card_auto_deduct` 控制代理总开关，默认关闭；设置也随 `/agent/user_query_soft_list` 返回。
- `/agent/point_card/save` 可设置单卡 `point_card_auto_deduct_mode`：`inherit` 跟随总开关，`allow` 允许，`deny` 禁止。`/agent/point_card/auto_deduct` 可批量设置。
- 管理员通过 `/admin/设置代理账号` 设置 `min_balance`，范围为 -10 亿至 10 亿点。负数允许欠费至该下限，正数要求扣款后至少保留该余额；旧数据不自动转换，由管理员按需调整。
- 点卡余额不足时，按代理软件单价计算代扣额并向上取整。未允许代扣、无有效价格或扣后低于余额下限时，整笔续费失败，不发生部分扣款。代理扣款、卡内扣点和续费共同成功或回滚。
- 同一余额下限也用于代理生成点卡、时长卡、时长充值卡及续费时长卡；管理员手动调整代理余额不受限制。

代理只能查询自己生成的卡及流水：`POST /agent/point_card/ledger`。点卡列表支持按 `card`、`software`、`point_balance`、`card_state`、`create_time`、`use_time` 排序。

## 时长卡客户端与管理

时长卡独立保存固定时长和到期时间，不使用点卡余额或点卡流水。

- `GET/POST /duration_card/card_login`：提交 `center_id/name`、`card`。首次登录激活；重复登录不延长有效期，只刷新心跳令牌。受最晚激活时间限制的卡，超时后不能激活。
- `GET/POST /duration_card/card_ping`：提交 `card`、`needle`，更新心跳并返回到期时间；令牌无效、卡被冻结或已到期时失败。
- `GET/POST /duration_card/card_logout|query|bulletin|config`：退出、查询卡密、读取公告、读写最多 200 字符的卡密配置。
- `GET/POST /duration_card/recharge`：当前卡为目标，提交 `source_card`（兼容 `card2`）。来源须为同软件、未激活且正常的时长卡；成功后来源卡不能再登录或充值。目标卡可已激活或暂停。

管理员和代理分别通过 `/admin/duration_card/*`、`/agent/duration_card/*` 管理时长卡。常用接口：`list|detail|create|save|state|renew|delete`。时长范围为 5 分钟至 36500 天；`latest_activation_minutes` 为 `-1` 表示不限、`0` 表示立即激活。续费已激活卡延长到期时间，暂停卡增加暂停剩余时长。

时长充值卡使用 `/admin/duration_recharge_card/*`、`/agent/duration_recharge_card/*`；支持 `list|detail|create|save|delete`。代理只能管理自己生成的充值卡。

时长卡代理价格锚点由管理员通过 `/admin/duration_card/agent_price/list|save|delete` 管理。代理可调用：

- `POST /agent/duration_card/price/list` 查询可用价格。
- `POST /agent/duration_card/price_preview` 提交 `software`、`duration_minutes` 和可选 `num` 预览价格。
- `POST /agent/duration_card/create` 发卡；`POST /agent/duration_card/renew` 续费。

锚点时长按总价计费；锚点之间按较高的每分钟价格折算。目标时长须处于启用锚点范围内，永久卡须有专用锚点。价格修改不影响已生成的卡。

## 访客与充值

- `POST /visitor/查询所有卡密`：提交 `center_id`、`mode`（`point` 或 `duration`，默认 `point`）和 `card`。完整卡密精确查询；前缀查询格式为至少 4 位前缀加 `***`，固定匹配末尾 3 位；逗号分隔最多 20 张完整卡密时批量精确查询。结果默认每页 20 张，最多 100 张；每个管理员和来源 IP 每分钟最多查询 10 次。
- `POST /visitor/查询时长卡`：提交完整卡密，只返回状态和到期信息，不返回心跳令牌。
- `POST /visitor/duration_recharge_card/query`：查询充值卡状态，兼容 `/visitor/查询充值卡`。
- `POST /visitor/duration_recharge_card/redeem`：提交 `recharge_card` 和目标 `cards`。可给同软件、已激活（含已到期）的正常时长卡续费，也可给仍有剩余时长的暂停卡增加暂停余额；兼容 `/visitor/续费卡密` 和旧字段 `Rechargeable_card`。
- `POST /visitor/duration_card/pause|resume`：暂停或恢复时长卡；由软件的 `pause_deduct_minutes` 控制，`0` 表示关闭。兼容 `/visitor/暂停时长`、`/visitor/恢复时长`。
