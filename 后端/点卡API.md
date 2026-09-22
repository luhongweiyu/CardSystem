# 点卡与时长卡 API

所有接口同时支持 JSON POST；标注“兼容 GET”的接口也接受查询参数。成功响应通常包含 `state: true, code: 1`，失败响应包含 `state: false, code: 0, msg`。

JSON 请求的业务参数只从正文读取，缺字段时不回退到 URL 或表单。

新卡端接口通过 `center_id` 或 `name` 定位管理员，再提交 `card`。新接口始终检查 `timestamp` 和 `nonce`，请求需带 URL 参数 `sign`：JSON POST 计算 `MD5(api_password + 原始JSON)`；GET 或无 JSON 的 POST 计算 `MD5(api_password + 去掉 sign 后的原始查询字符串)`。开启 API 安全模式时比较请求 `sign`；关闭安全模式时仍检查时间戳和 nonce，仅跳过请求 `sign` 比较。响应把 `sign` 放在 JSON 内，客户端将其临时替换为空字符串后按同一规则校验；响应始终返回签名，安全密码为空时也按空字符串参与计算。

旧客户端只使用 `/card/card_login`、`/card/card_ping`。这两个接口按旧协议处理：始终检查时间戳，开启 API 安全模式时请求为 `MD5(timestamp + api_password)`，返回时间戳为请求时间戳加 `10`，返回签名为 `MD5(timestamp + api_password + code)`，不要求 `nonce`。旧签名只保护时间戳，不保护卡密、设备等业务参数，仅用于兼容旧客户端。接口安全模式不提供传输加密。

新客户端使用 `/point_card/*` 和 `/duration_card/*`；除上述两个旧时长卡兼容接口外，其他旧 `/card/*`、`/duration/*` 不再注册。新旧服务不要同时写入同一个数据库。

## 1. 查询可用点卡计费方案

`GET/POST /point_card/period_prices`（只读）

参数：`center_id/name`、`card`，可选 `software`。返回当前卡密所属软件的启用且时长有效的点卡计费方案、默认授权时长、心跳间隔和自动离线时间。计费周期有效范围为 15 分钟至 30 天，接口统一按分钟表示：

```json
{
  "state": true,
  "data": [{"period_minutes": 60, "cost": 5, "is_default": true}],
  "software": 1,
  "default_period_minutes": 60,
  "heartbeat_interval_seconds": 300,
  "online_grace_minutes": 60
}
```

## 2. 登录并按需扣点

`GET/POST /point_card/card_login`

```json
{
  "center_id": 1,
  "card": "1abcdefghijklmnop",
  "device_id": "7f6a0a38-2b1c-4f21-9b9c-3c5d5c7a1e22",
  "device_alias": "办公室电脑",
  "period_minutes": 60
}
```

字段：

- `software` 无需提交，服务端始终使用卡密记录中绑定的软件编号。
- `device_id` 可选；省略时统一按空字符串处理。使用非空设备 ID 时应由客户端生成并持久化，不能使用 IP。
- 每张卡最多保留 1000 台设备会话；达到上限后不能增加新会话，已有设备和不增加数量的授权复用不受影响。
- `device_alias` 仅登录时可选，最长 64 个字符，不参与唯一性，也不要求不重复。
- `period_minutes` 可选，含义是授权时长分钟数。有效值为 0 或 15 至 43200 分钟（30 天按固定 720 小时计算）；省略或为 0 时，新会话使用软件默认授权时长；已有会话沿用上次续费时长。显式提交的授权时长必须已经配置对应的点卡计费方案且处于启用状态。授权一次性预扣，提前退出不退点。
- `prefer_reuse` 可选，默认 `false`，接受 `true/false` 或 `1/0`。只有软件的 `point_card_reuse_enabled` 也开启时，才尝试跨设备复用；任一未开启就跳过，不因此报错。

成功响应业务字段：`needle`、`authorized_until`、`heartbeat_interval_seconds`；顶层还会返回本次请求的 `nonce`。

同一管理员、卡密和设备 ID 只有一条会话。多个客户端都省略 `device_id` 时会共用空设备 ID 对应的同一条会话。当前授权未到期时重复登录不扣点；如果本次明确选择了另一个有效授权时长，只更新会话的下一次续费时长，不改变当前截止时间。

跨设备复用优先接手同卡已离线、未到期且最后心跳最早的授权；在线判断包含尚未落库的心跳。保留原到期时间，不扣任何余额、不写余额流水，旧设备凭证失效。新设备的下一次续费时长仍按本次登录规则决定，不继承被接手设备的设置；没有候选则正常扣费。复用不增加会话数量，达到设备上限时仍可接手。

复用候选按管理员和卡密缓存 1 分钟，期间只逐个检查、剔除候选，空列表或耗尽也不提前补查；新离线设备可能要等下一次刷新才能参与复用，期间没有可用候选就正常扣费。

## 3. 心跳续费

`GET/POST /point_card/card_ping`

```json
{
  "center_id": 1,
  "card": "1abcdefghijklmnop",
  "needle": "登录响应中的随机令牌",
  "device_id": "7f6a0a38-2b1c-4f21-9b9c-3c5d5c7a1e22"
}
```

服务端使用管理员、卡密和 `device_id` 查找设备会话，再使用 `needle` 校验该会话。`device_id` 可选，但必须与登录时保持一致：登录时省略则心跳也省略，登录时提交则心跳必须提交相同值。心跳不接收或更新 `device_alias`。响应返回新的 `authorized_until`、`heartbeat_interval_seconds`、`needle` 和本次请求 `nonce`。

授权尚未到期的会话首次从数据库加载后，普通心跳只更新内存中的 `last_heartbeat_at`，不会扣点；脏心跳达到配置的 15 至 60 分钟间隔后，由下一次心跳写入数据库。授权到期后，服务端按软件自动离线时间计算：

`推断截止 = last_heartbeat_at + online_grace_minutes × 1分钟`

如果推断截止晚于旧 `authorized_until` 且当前时间尚未超过推断截止，视为设备可能仍在线，从旧截止时间续一个授权时长并扣一次点。超过推断窗口后，实际收到登录或心跳就从当前时间开始新的授权时长；后台清理在没有新请求时会删除该离线会话。心跳或后台清理续费时若余额不足或计费方案不可用，会删除已过期会话，客户端需重新登录。三条路径采用同一续费起点规则。

授权时长按软件已启用的计费方案执行；心跳间隔最大为 86400 秒。自动离线时间设置为 0 时按 60 分钟处理。

心跳间隔、自动离线时间和计费周期独立配置；管理端会提示心跳间隔不小于自动离线时间、或自动离线时间长于计费周期的风险，不强制绑定。旧的低于 15 分钟的方案不会自动改价，客户端不再返回；如果软件默认授权时长也低于 15 分钟，管理员须先配置新方案并切换默认时长。

## 4. 退出

`GET/POST /point_card/card_logout`

提交 `center_id/name`、`card`；`device_id` 可省略或传空值，统一按空字符串查找会话。软件编号从卡密记录和设备会话读取，不需要提交 `software`。可选同时提交 `needle` 作为当前心跳会话的附加校验。接口按管理员、卡密和设备 ID 幂等地删除对应会话；退出不会产生点数流水，`needle` 不是设备唯一键。

## 5. 查询卡密和流水

- `GET/POST /point_card/query`：查询当前卡密余额、状态、授权设备数 `authorized_device_count`、在线设备数 `online_device_count` 和分页设备列表 `devices`。可提交 `page`、`page_size`（默认 50，最大 100），响应返回 `device_total`、`device_page`、`device_page_size`。查询只读，不扣点、不续费。
- `devices` 中每项包含 `device_id`、`device_alias`、`needle`、`authorized_until`、`authorized`、`online` 和 `forced_offline`（管理端主动下线）；已下线设备不计入在线数，未到期时仍计入授权数。`needle` 是服务端心跳令牌，设备 ID 仍由 `device_id` 标识。
- `GET/POST /point_card/point_ledger/query`：只查询当前卡密自己的流水，支持 `page`、`page_size`；可选 `software` 仅校验当前卡密归属。流水只保留最近 30 天，删除卡密后重用同名卡密时，保留期内的新旧流水可能混合显示。
- `GET/POST /point_card/bulletin`：读取卡密所属软件公告。
- `GET/POST /point_card/config`：读取或写入卡密配置，配置最多 200 个字符（写入受可选签名保护）。

流水字段如下：

| 字段 | 含义 |
| --- | --- |
| `created_at` | 变动时间 |
| `event_type` | `credit` 补点，`debit` 扣点 |
| `change` | 有符号变动，扣点为负、补点为正 |
| `balance_before` / `balance_after` | 变动前后余额 |
| `remark` | 原因和设备 ID/别名快照（如有） |

日常操作只有余额确实改变时才写流水；同一授权时长内的重复登录/心跳不会重复写入。生成卡密时始终写一条“生成卡密初始点数”补点流水，点数为 0 时该流水的变动为 0，便于审计。后台定时清理超过 30 天的流水；删除后重用同名卡密时，保留期内的新旧记录允许混合保存。

## 6. 管理端和代理账号

管理员接口统一位于 `/admin`，代理接口统一位于 `/agent`；下面未重复书写前缀的路径，按所在小节补上对应前缀：

- `/user_add_soft`、`/user_modify_bulletin`、`/user_del_soft`：软件及默认授权时长、心跳间隔、自动离线时间、时长卡暂停扣除分钟数设置。
- 软件设置中的 `point_card_reuse_enabled` 仅管理员可修改，默认关闭；编辑时省略则保留原值，软件列表返回该字段。
- `/point_card/price/list|save|delete`：点卡计费方案管理。
- `/point_card/create`、`/point_card/list`、`/point_card/save`、`/point_card/delete`、`/point_card/state`：点卡管理。
- `POST /point_card/device/offline`：管理员或代理提交 `card`、`device_id`（可为空）、详情中的 `needle`。只能操作自己有权限的卡；成功返回 `msg`。下线保留未到期授权并停止后台续费，原凭证失效，到期后清理；设备仍可重新登录。与客户端 `card_logout` 直接删除会话不同。
- 管理员和代理单次最多生成 500 张卡密；点数允许为 0，生成 0 点卡也会写入一条初始点数流水；卡密文本解析、随机生成和重复预检查在事务外完成，最终写入遇到重复或其他错误直接结束，不自动重试。
- `/point_card/adjust`：管理员手工补点或扣回，金额为有符号整数。
- `/point_card/ledger`：管理员分页查看流水。
- `/创建代理账号`、`/设置代理账号`、`/查询代理账号`、`/删除代理账号`、`/代理账号充值`：代理账号管理；创建接口使用 `agent_name`、`agent_password`，避免与管理员认证字段混淆。

代理账号使用 `/agent` 前缀，只能管理所属管理员分配的卡密。代理生成卡密时，卡密写入和渠道余额扣减在同一事务中完成；管理员调整代理价格不会改变已生成卡密的余额。
代理端也可调用 `/agent/point_card/ledger` 查看自己生成的卡密流水；该接口不会返回其他代理的记录。

代理价格是扁平 JSON：键为软件 ID，值为每生成 1 点卡点数需要消耗的代理余额，例如 `{"1": 0.25}`。未出现在映射中的软件不授权代理发卡；价格必须大于 0 且最多两位小数，配置最多 100 个软件且不超过 4096 字节，一批点卡的总费用最终按整数点向上取整。

点卡余额不足代扣：

- 代理使用 `POST /agent/point_card/settings` 提交 `point_card_auto_deduct: true/false`，只修改自己的总开关，默认关闭。当前设置随 `/agent/user_query_soft_list` 返回，保存接口也返回 `point_card_auto_deduct`、`allow_point_debt`、`point_debt_limit` 和 `balance`。
- 代理使用 `/agent/point_card/save` 提交 `card` 和 `point_card_auto_deduct_mode`：`inherit` 跟随总开关（默认）、`allow` 允许、`deny` 禁止。只允许修改自己的卡，卡密设置优先于总开关；卡密列表返回该字段。批量设置使用 `/agent/point_card/auto_deduct`，提交 `cards` 和同名字段，整批设置为同一种模式。
- 管理员在 `/admin/设置代理账号` 的 `data` 中提交代理 `id`、`allow_point_debt` 和 `point_debt_limit`（整数，0 至 10 亿点）；未提交 `prices` 时保留原价格。默认禁止欠费；允许且额度大于 0 时，点卡代扣后的代理余额最低可到负额度。代理不能修改这两个字段。

登录、心跳续费和后台续费均先扣卡内点数，不足部分按所属代理当前软件单价折算，每次代扣费用按整数点向上取整。代扣未允许、价格不可用或超出可扣额度时，本次不扣任何余额。代理扣款、卡内扣点和设备续费在同一事务中完成；管理员名下或没有有效所属代理的卡不会代扣。欠费额度只用于点卡代扣，不用于生成卡密或时长卡业务。

点数流水仍只记录卡内真实余额变化；代理代扣在事务提交后写入代理操作日志。修改开关不会撤销已付费授权，下一次需要扣费时读取最新设置。

时长卡代理价格与点卡价格分开保存。管理员使用 `/admin/duration_card/agent_price/list|save|delete` 按代理和软件维护时长价格锚点；每个锚点的 `duration_minutes` 为卡面时长，`price` 为该时长的代理总价。保存请求示例：

```json
{
  "agent_id": 3,
  "software": 1,
  "prices": [
    {"duration_minutes": 2880, "price": 20},
    {"duration_minutes": 7200, "price": 36}
  ]
}
```

`prices` 必须显式传入；传空数组表示清空该软件的全部锚点。代理可使用以下接口：

- `POST /agent/duration_card/price/list`：查询当前代理全部启用的时长价格锚点。
- `POST /agent/duration_card/price_preview`：提交 `software`、`duration_minutes` 和可选 `num`（默认1）预览单卡价格和本批扣款。
- `POST /agent/duration_card/create`：使用与管理员生成接口相同的时长卡字段发卡。价格在事务内重新计算，代理余额扣减与卡密写入要么全部成功，要么全部回滚。
- `POST /agent/duration_card/list|detail|save|state|delete`：只管理当前代理自己生成的时长卡。
- `POST /agent/duration_card/renew`：给自己已激活或暂停中的时长卡续费，按当前价格锚点计价并在同一事务扣除代理余额；暂停卡累加 `paused_remaining_minutes`。

精确命中启用锚点时使用该锚点原价；位于两个启用锚点之间时，比较左右锚点的总价/分钟数，取较高者折算目标时长。目标时长必须位于最短和最长启用锚点之间，永久卡必须配置专用锚点。价格最多保留两位小数，批量总额最后向上取整；价格修改不影响已经生成的时长卡。

管理端和代理端的点卡列表接口（分别为 `/admin/point_card/list`、`/agent/point_card/list`）支持服务端排序：`sort_by` 可用 `card`、`software`、`point_balance`、`card_state`、`create_time`、`use_time`，`sort_order` 和 `card_order` 使用 `asc` 或 `desc`。单独选择 `card` 时只按卡密排序；选择其他字段时该字段为第一排序、卡密为第二排序。备注不参与排序，分页和总数始终由数据库计算。

## 7. 时长卡客户端接口

时长卡与点卡使用不同表和接口。卡密本身保存固定时长，接口不读取 `point_balance`，也不会写入点数流水。

### 登录

`GET/POST /duration_card/card_login`

提交 `center_id/name` 和 `card`。软件编号从时长卡记录读取，不需要提交 `software`。未激活卡在首次登录时激活；如果生成时设置了最晚激活时间，超过该时间后不能激活。成功响应包含：

```json
{
  "needle": "服务端随机心跳令牌",
  "authorized_until": "2026-09-09T12:00:00+08:00",
  "software": 1,
  "heartbeat_interval_seconds": 300
}
```

重复登录只刷新 `needle`，不会再次延长 `authorized_until`；同一张时长卡同时只接受最后一次登录生成的 needle。

### 心跳和退出

- `GET/POST /duration_card/card_ping`：提交 `card`、`needle`，只更新最后心跳时间并返回当前到期时间；needle 不正确、卡被冻结或已到期时失败。
- 时长卡普通心跳命中进程内缓存时只更新内存中的最后心跳；达到 `心跳缓存.同步间隔分钟` 后由下一次心跳同步数据库。登录、退出、暂停、恢复、充值、续费、冻结、删除或软件变更会先同步并失效相关缓存。
- `GET/POST /duration_card/card_logout`：提交 `card`，可附带 `needle`；清除在线校验，不改变卡密剩余时长。
- `GET/POST /duration_card/query`：查询当前时长卡的固定时长、激活状态、到期时间和配置。
- `GET/POST /duration_card/bulletin`：读取时长卡所属软件的公告。
- `GET/POST /duration_card/config`：读取或写入卡密配置，仍受 200 字符限制。
- `GET/POST /duration_card/recharge`：当前卡作为目标，提交 `source_card`（旧参数名 `card2` 也可用）；来源必须是同软件、未激活且状态正常的时长卡。成功后来源状态变为 `6`，不能再次登录或充值。目标卡可以是已激活或已暂停状态。

### 时长卡管理

管理员接口位于 `/admin`：

- `/duration_card/list`：分页筛选和排序；状态值 `1` 未激活、`2` 已激活、`3` 已到期、`4` 冻结、`5` 已暂停、`6` 已用于充值。
- `/duration_card/create`：生成时长卡，`duration_minutes` 范围为 5 分钟至 36500 天，`latest_activation_minutes` 为 `-1` 表示不限、`0` 表示立即激活。
- `/duration_card/detail`、`/duration_card/save`、`/duration_card/state`、`/duration_card/renew`、`/duration_card/delete`：详情、编辑、冻结/解冻、续费和删除；续费已激活卡延长到期时间，暂停卡累加暂停剩余分钟。

访客页面可调用 `POST /visitor/查询时长卡`，提交 `center_id` 和完整 `card`，只返回状态和到期信息，不返回服务端 needle。

### 时长充值卡和暂停

管理员和代理分别使用 `/admin/duration_recharge_card/*`、`/agent/duration_recharge_card/*`：

- `list|detail|save|delete`：查询、修改备注/冻结状态和删除；代理只能操作自己生成的充值卡。
- `create`：提交 `software`、`duration_minutes`、`uses`、`num`、`cards/random`、可选 `expires_at` 和 `notes`。代理生成时按充值分钟数、次数和卡数计价并扣余额。

访客接口：

- `POST /visitor/duration_recharge_card/query`：提交 `center_id`、`card` 查询充值卡状态；兼容旧路径 `/visitor/查询充值卡`。
- `POST /visitor/duration_recharge_card/redeem`：提交 `center_id`、`recharge_card`、`cards`，对同软件且已激活的正常时长卡续费，或给暂停中的时长卡累加 `paused_remaining_minutes`；兼容旧路径 `/visitor/续费卡密` 和旧字段 `Rechargeable_card`。
- `POST /visitor/duration_card/pause|resume`：提交 `center_id`、`card`。暂停功能由软件的 `pause_deduct_minutes` 控制，`0` 表示关闭；兼容旧路径 `/visitor/暂停时长`、`/visitor/恢复时长`。
