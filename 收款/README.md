# 收款核对服务

Go + Gin + Viper 实现的单实例 HTTP 服务。使用支付宝官方 `alipay.data.bill.accountlog.query` 查询账务明细。调用方付款后提交金额和订单尾号，服务在本地缓存或本次查询结果中核对。

## 构建与启动

需要 Go 1.25 或以上；已开启自动工具链下载的旧版 Go 也可使用。

```powershell
# 默认使用本地 `.credentials/config.json` 和其中引用的密钥文件。
.\build.ps1

# 只检查配置和内置密钥，不访问支付宝。
.\.build\receipt-service-amd64.exe -check-config

# 启动服务；启动本身不会请求支付宝。
.\.build\receipt-service-amd64.exe
```

首次构建会从 `config.example.yaml` 生成 `config.yaml`，自动写入随机共享密钥。之后构建不会覆盖配置。修改中文配置后重启生效。

如果 PowerShell 阻止执行本地脚本，可仅对这次运行使用：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\build.ps1
```

指定其他凭据配置或 Go 可执行文件：

```powershell
.\build.ps1 -CredentialConfig 'D:\private\config.json' -GoExecutable 'D:\Program Files\Go\bin\go.exe'
```

凭据配置的格式沿用验证脚本：

```json
{
  "app_id": "你的APPID",
  "app_private_key_file": "app_private_key.pem",
  "alipay_public_key_file": "alipay_public_key.pem"
}
```

密钥相对路径以凭据配置文件目录为基准。支持 PEM 和纯 Base64；请求固定使用 RSA2。APPID、应用私钥和支付宝公钥在编译时嵌入程序，运行时只需要可执行文件及 `config.yaml`。程序只能部署在自己的服务器，不应交给调用方。

真实凭据、运行配置、构建产物和验证辅助文件都在 `.gitignore` 中；`internal/credentials/assets/placeholder.txt` 只用于让无凭据源码仍能编译。没有嵌入凭据的程序启动会明确报错。

Linux 构建：

```powershell
.\build.ps1 -TargetOS linux -TargetArch amd64
```

把 `.build/receipt-service-linux-amd64` 与 `config.yaml` 放到服务器，执行：

```sh
chmod +x receipt-service-linux-amd64
./receipt-service-linux-amd64 -config ./config.yaml
```

## HTTP 接口

默认 `POST http://127.0.0.1:8080/verify`，`Content-Type: application/json`。

```json
{"a":"12","b":100,"c":"123456","d":1791950400}
```

示例时间戳需要替换为发送时的 Unix 秒。

| 字段 | 要求 |
| --- | --- |
| `a` | 纯数字字符串，数值 0～99；`"01"` 和 `"1"` 共用额度，允许 `"0"`。 |
| `b` | JSON 正整数，单位分，例如 `100` 表示 1 元。拒绝 `"100"`、`1.00`、`1e2`、负数、0 和超出 int64 的值。 |
| `c` | 6 位数字字符串，可填写支付宝订单号或商户订单号的尾号；不是账务流水号尾号，保留前导零。 |
| `d` | JSON 正整数，Unix 秒。默认只允许服务器时间前后 60 秒。 |

只接受这四个字段；重复字段、额外字段、尾随 JSON 内容、超过 4 KiB 的请求体都会被拒绝。用户 ID 由调用项目后端取得。同一服务内，相同数字 ID 共享额度，不按项目隔离。

### 签名

使用 `config.yaml` 的 `鉴权.共享密钥` 对**最终发送的原始 JSON 字节**计算 HMAC-MD5，结果转小写十六进制，写入 `X-Signature` 请求头（名称可配置）。

```go
mac := hmac.New(md5.New, []byte(secret))
mac.Write(body) // body 是最终发送的 []byte，签名后不要再次序列化或格式化。
signature := hex.EncodeToString(mac.Sum(nil))
```

HMAC-MD5 是带密钥的认证摘要，不是 JSON 加密，也不是 `MD5(body)`。有效期内重复的已受理签名会被拒绝，不消耗查询额度。客户端修改时间戳后需要重新签名。HTTP 不提供传输内容保密性。

可运行调用示例位于 `cmd/verify-client/main.go`：

```powershell
# 共享密钥从自己的 config.yaml 获取；不要把真实值提交到源码。
$env:RECEIPT_SECRET = '你的共享密钥'
$env:GO111MODULE = 'on'
go run ./cmd/verify-client -user 12 -cents 100 -suffix 123456
```

### 返回值

正常业务结果 HTTP 200：

```json
{"msg":true,"冷却":45}
```

`msg` 只有首次成功认领才为 `true`。`false` 表示本次未确认，可能是未匹配、匹配多笔、已使用、冷却中、查询正在进行或支付宝查询失败。服务端日志记录原因，不返回账单内容。

`冷却` 是当前个人与全局剩余冷却的较大值，向上取整为秒，最小为 0。它表示再次调用支付宝的等待时间，不禁止读取缓存；其他用户的新查询也会更新全局冷却。

参数不合法统一 HTTP 400；签名无效 HTTP 401。都只返回通用错误，不向调用方披露具体校验规则：

```json
{"msg":false,"冷却":0,"error":"参数错误"}
```

## 已实现的业务规则

- 不使用查询游标，不定时请求支付宝。有需要时才查。
- 查询范围为 `max(启动边界, 当前时间 - 回查范围)` 到当前时间。因支付宝入账时间精确到秒，启动边界向上取整到秒，避免认领启动前同秒的旧记录。
- **只请求第一页：`page_no=1`、`page_size=1000`。不检查总条数、不翻页、不保证覆盖该时间段内超过第一页的记录。**
- 每次验签且业务查询成功后，第一页结果整批替换缓存，不合并；成功空结果清空缓存。失败保留仍在时间范围内的旧缓存。
- 全局每 1 分钟最多一轮实际查询；同一时间最多一轮请求进行中。
- 每个用户初始 2 次快速额度。实际请求支付宝才消耗额度并计时，即使失败也计入。额度耗尽后，与个人上次实际查询间隔至少 30 分钟，并同时遵守全局冷却。
- 缓存核对不消耗额度、不延长冷却。首次成功认领新支付，恢复 2 次快速额度、清除个人冷却；全局冷却保留。不按天恢复。
- 筛选收入且具有账务流水号、有效正金额和入账时间，并至少包含一种完整订单号的记录。金额按整数分比较；`alipay_order_no`（支付宝订单号）或 `merchant_order_no`（商户订单号）任一字段的后 6 位匹配即可。同一笔的两个编号都匹配只算一笔。不依赖可能变化的 `type` 文本来猜测收款业务。
- 必须唯一匹配。计算候选数量时包含已使用记录，避免排除它们后错误认领另一笔同金额、同尾号记录。
- 同时按账务流水号、非空的完整支付宝订单号和商户订单号记录已使用标记，三种编号分开存储。认领后换用另一种编号的尾号也不能再次成功；认领和恢复额度在同一锁内完成，并发只能成功一次。
- 已使用记录从成功认领时开始保留 60 分钟，过期清理。此设置不得短于回查范围。
- 用户状态最多 100 份。已消耗额度不会因为闲置而恢复。缓存、已使用标记和防重放记录在请求到来时清理，不需要后台同步任务。
- 纯内存、单实例；启动前的付款不参与核对。重复认领返回 false，不保证响应丢失后能恢复成功结果，按约定人工处理。

## 源码结构

- `main.go`：启动、离线配置检查和退出。
- `config.go`：中文配置读取与校验。
- `credentials.go`：编译内置凭据和 RSA 密钥解析。
- `alipay.go`：官方接口 POST、RSA2 签名/验签、第一页解析、元转分。
- `service.go`：共享缓存、冷却、额度、并发认领、内存过期清理。
- `http.go`：固定请求字段、HMAC-MD5、时间戳校验、防重放与统一响应。
- `cmd/verify-client/main.go`：供其他 Go 项目参考的调用示例。

官方接口文档：https://opendocs.alipay.com/open/dae3ac99_alipay.data.bill.accountlog.query

官方参数定义：https://github.com/alipay/alipay-sdk-java-all/blob/master/v2/src/main/java/com/alipay/api/domain/AlipayDataBillAccountlogQueryModel.java
