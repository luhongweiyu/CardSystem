# 点卡 API

所有接口同时支持 JSON POST；标注“兼容 GET”的接口也接受查询参数。成功响应通常包含 `state: true, code: 1`，失败响应包含 `state: false, code: 0, msg`。

卡端接口通过 `center_id` 或 `name` 定位管理员，再提交 `card`。开启 API 安全模式后，请求需带 `timestamp`、`nonce` 和 URL 参数 `sign`。POST JSON 签原始 JSON；GET 或无 JSON 的 POST 签去掉 `sign` 后的查询字符串。

响应会返回顶层 `sign` 和相同的 `nonce`。有接口口令时，客户端按原始响应内容校验 `sign`；接口安全模式不提供传输加密。

## 1. 查询可用点卡计费方案

`GET/POST /card/period_prices`（只读）

参数：`center_id/name`、`card`，可选 `software`。返回当前卡密所属软件的启用点卡计费方案、默认授权时长和心跳间隔：

```json
{
  "state": true,
  "data": [{"period_seconds": 3600, "cost": 5, "is_default": true}],
  "software": 1,
  "default_period_seconds": 3600,
  "heartbeat_interval_seconds": 300
}
```

## 2. 登录并按需扣点

`GET/POST /card/card_login`

```json
{
  "center_id": 1,
  "card": "1abcdefghijklmnop",
  "device_id": "7f6a0a38-2b1c-4f21-9b9c-3c5d5c7a1e22",
  "device_alias": "办公室电脑",
  "period_seconds": 3600
}
```

字段：

- `software` 无需提交，服务端始终使用卡密记录中绑定的软件编号。
- `device_id` 可选；省略时统一按空字符串处理。使用非空设备 ID 时应由客户端生成并持久化，不能使用 IP。
- `device_alias` 仅登录时可选，最长 64 个字符，不参与唯一性，也不要求不重复。
- `period_seconds` 可选，含义是授权时长秒数。省略或为 0 时，新会话使用软件默认授权时长；已有会话沿用上次续费时长。显式提交的授权时长必须已经配置对应的点卡计费方案且处于启用状态。

成功响应业务字段：`needle`、`authorized_until`、`heartbeat_interval_seconds`；顶层还会返回本次请求的 `nonce`。

同一管理员、卡密和设备 ID 只有一条会话。多个客户端都省略 `device_id` 时会共用空设备 ID 对应的同一条会话。当前授权未到期时重复登录不扣点；如果本次明确选择了另一个有效授权时长，只更新会话的下一次续费时长，不改变当前截止时间。

## 3. 心跳续费

`GET/POST /card/card_ping`

```json
{
  "center_id": 1,
  "card": "1abcdefghijklmnop",
  "needle": "登录响应中的随机令牌",
  "device_id": "7f6a0a38-2b1c-4f21-9b9c-3c5d5c7a1e22"
}
```

服务端使用管理员、卡密和 `device_id` 查找设备会话，再使用 `needle` 校验该会话。`device_id` 可选，但必须与登录时保持一致：登录时省略则心跳也省略，登录时提交则心跳必须提交相同值。心跳不接收或更新 `device_alias`。响应返回新的 `authorized_until`、`heartbeat_interval_seconds`、`needle` 和本次请求 `nonce`。

授权尚未到期时，心跳只更新 `last_heartbeat_at`，不会扣点。授权到期后，服务端按软件心跳间隔计算：

`推断截止 = last_heartbeat_at + heartbeat_interval_seconds × 2`

如果推断截止晚于旧 `authorized_until` 且当前时间尚未超过推断截止，视为设备可能仍在线，从旧截止时间续一个授权时长并扣一次点。超过推断窗口后，实际收到登录或心跳就从当前时间开始新的授权时长；后台清理在没有新请求时会删除该离线会话。三条路径采用同一续费起点规则。

授权时长按软件已启用的计费方案执行，不强制要求达到心跳间隔的 2 倍；心跳间隔最大为 86400 秒。在线判断仍使用两个心跳间隔的推测窗口。

## 4. 退出

`GET/POST /card/card_logout`

提交 `center_id/name`、`card`；`device_id` 可省略或传空值，统一按空字符串查找会话。软件编号从卡密记录和设备会话读取，不需要提交 `software`。可选同时提交 `needle` 作为当前心跳会话的附加校验。接口按管理员、卡密和设备 ID 幂等地删除对应会话；退出不会产生点数流水，`needle` 不是设备唯一键。

## 5. 查询卡密和流水

- `GET/POST /card/query`：查询当前卡密余额、状态、授权设备数 `authorized_device_count`、在线设备数 `online_device_count` 和设备列表 `devices`。查询只读，不扣点、不续费。
- `GET/POST /card/point_ledger/query`：只查询当前卡密自己的流水，支持 `page`、`page_size`；可选 `software` 仅校验当前卡密归属，不会截断同名卡密的历史流水。
- `GET/POST /card/bulletin`：读取卡密所属软件公告。
- `GET/POST /card/config`：读取或写入卡密配置，配置最多 200 个字符（写入受可选签名保护）。

流水字段如下：

| 字段 | 含义 |
| --- | --- |
| `created_at` | 变动时间 |
| `event_type` | `credit` 补点，`debit` 扣点 |
| `change` | 有符号变动，扣点为负、补点为正 |
| `balance_before` / `balance_after` | 变动前后余额 |
| `remark` | 原因和设备 ID/别名快照（如有） |

只有余额确实改变时才写流水；同一授权时长内的重复登录/心跳不会重复写入。生成卡密时也会写一条“生成卡密初始点数”补点流水，便于审计。删除后重用同名卡密不会删除旧流水，新旧记录允许混合保存。

## 6. 管理端和代理账号

管理员接口统一位于 `/admin`：

- `/user_add_soft`、`/user_modify_bulletin`、`/user_del_soft`：软件及默认授权时长/心跳间隔设置。
- `/point_period_price/list|save|delete`：点卡计费方案管理。
- `/add_new_card`、`/user_query_card`、`/modify_card`、`/delete_card`、`/冻卡s`：点卡管理。
- `/point_card/adjust`：管理员手工补点或扣回，金额为有符号整数。
- `/point_ledger/query`：管理员分页查看流水。
- `/创建代理账号`、`/设置代理账号`、`/查询代理账号`、`/删除代理账号`、`/代理账号充值`：代理账号管理；创建接口使用 `agent_name`、`agent_password`，避免与管理员认证字段混淆。

代理账号使用 `/agent` 前缀，只能管理所属管理员分配的卡密。代理生成卡密时，卡密写入和渠道余额扣减在同一事务中完成；管理员调整代理价格不会改变已生成卡密的余额。
代理端也可调用 `/agent/point_ledger/query` 查看自己生成的卡密流水；该接口不会返回其他代理的记录。

代理价格是扁平 JSON：键为软件 ID，值为每生成 1 点卡点数需要消耗的代理余额，例如 `{"1": 0.25}`。未出现在映射中的软件不授权代理发卡；价格必须大于 0 且最多两位小数，一批点卡的总费用最终按整数点向上取整。

管理端和代理端的 `/user_query_card` 支持服务端排序：`sort_by` 可用 `card`、`software`、`point_balance`、`card_state`、`authorized_device_count`、`create_time`、`use_time`，`sort_order` 和 `card_order` 使用 `asc` 或 `desc`。单独选择 `card` 时只按卡密排序；选择其他字段时该字段为第一排序、卡密为第二排序。备注不参与排序，分页和总数始终由数据库计算。
