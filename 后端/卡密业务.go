package main

import (
	"bufio"
	"crypto/md5"
	cryptorand "crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 日志锁保护文本运行日志；本文件其余内容按“卡端基础接口、卡密生成、
// 管理端查询与维护”的顺序排列，集中保存卡密主体相关业务。
var 日志锁 sync.Mutex

// 写入文件_追加是系统运行日志的最小封装。日志文件只记录服务端生成内容，
// 外部字段在调用前会经过清理，避免并发写入和换行伪造。
func 写入文件_追加(filePath string, content string) {
	日志锁.Lock()
	defer 日志锁.Unlock()
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		fmt.Println("文件打开失败", err)
		return
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	if _, err := writer.WriteString(content); err != nil {
		fmt.Println("日志写入失败", err)
		return
	}
	if err := writer.Flush(); err != nil {
		fmt.Println("日志刷新失败", err)
	}
}

func 日志(filePath string, content string) {
	写入文件_追加(filePath, "\n"+time.Now().Format("2006-01-02 15:04:05")+":"+content)
}

// 清理拒绝日志字段防止外部输入伪造日志行，并限制异常请求的日志体积。
func 清理拒绝日志字段(value string, maxRunes int) string {
	value = strings.NewReplacer("\r", " ", "\n", " ", "\t", " ").Replace(value)
	runes := []rune(value)
	if maxRunes > 0 && len(runes) > maxRunes {
		return string(runes[:maxRunes]) + "..."
	}
	return value
}

func 拒绝日志(ctx *gin.Context) {
	日志("log/拒绝"+time.Now().Format("200601"), fmt.Sprintf("ip:%s;method:%s;card:%s;path:%s", ctx.ClientIP(), ctx.Request.Method, 清理拒绝日志字段(input(ctx, "card"), 80), 清理拒绝日志字段(ctx.Request.URL.Path, 300)))
}

// GetRandomString 使用系统安全随机源生成卡密和会话令牌。
func GetRandomString(length int, lower string) string {
	if length <= 0 {
		return ""
	}
	alphabet := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	if strings.EqualFold(lower, "a") {
		alphabet = "abcdefghijklmnopqrstuvwxyz"
	}
	result := make([]byte, 0, length)
	buffer := make([]byte, 32)
	for len(result) < length {
		if _, err := cryptorand.Read(buffer); err != nil {
			panic(fmt.Errorf("读取系统安全随机源失败: %w", err))
		}
		for _, value := range buffer {
			if value >= 234 { // 26 的最大整除边界，避免取模偏差。
				continue
			}
			result = append(result, alphabet[int(value)%len(alphabet)])
			if len(result) == length {
				break
			}
		}
	}
	return string(result)
}

// card_id获取用户设置校验持卡接口的管理员范围，并把卡密上下文写入 Gin。
// center_id 是面向访客页面的公开入口，name 仅用于管理员生成的内部链接。
func card_id获取用户设置(ctx *gin.Context) {
	card := strings.ToLower(strings.TrimSpace(input(ctx, "card")))
	if !卡密格式规则.MatchString(card) {
		失败提示(ctx, "card错误")
		拒绝日志(ctx)
		ctx.Abort()
		return
	}
	name := strings.TrimSpace(input(ctx, "name"))
	if name == "" {
		centerID, _ := strconv.Atoi(input(ctx, "center_id"))
		name = 全局_运行状态.读取用户名(centerID)
	}
	userinfo, ok := 全局_运行状态.读取用户设置(name)
	if !ok {
		失败提示(ctx, "center_id或name错误")
		拒绝日志(ctx)
		ctx.Abort()
		return
	}
	if !请求防火墙(name) {
		失败提示(ctx, "api次数超限")
		ctx.Abort()
		return
	}
	ctx.Set("card", 卡密请求上下文{Card: card, user_info: userinfo})
	ctx.Next()
}

// 计算卡密接口签名直接对“安全密码 + 实际请求或响应字节”计算 MD5。
// JSON 模式使用原始正文；查询参数模式使用去除 sign 后的原始查询字符串。
func 计算卡密接口签名(apiPassword string, rawContent []byte) string {
	hash := md5.New()
	_, _ = hash.Write([]byte(apiPassword))
	_, _ = hash.Write(rawContent)
	return fmt.Sprintf("%x", hash.Sum(nil))
}

// 写入卡密响应先把顶层 sign 置空并序列化，再对这份原始 JSON 计算签名。
// 客户端收到响应后，只需在原始响应文本中把顶层 sign 还原为空即可校验，
// 不依赖响应头，也不能先解析后重新序列化，以免字段顺序或转义方式变化。
func 写入卡密响应(ctx *gin.Context, data gin.H) {
	data["timestamp"] = time.Now().Unix()
	if nonce := input(ctx, "nonce"); nonce != "" {
		data["nonce"] = nonce
	}
	data["sign"] = ""
	unsignedJSON, err := json.Marshal(data)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"state": false, "code": 0, "msg": "响应编码失败", "sign": ""})
		return
	}
	if value, exists := ctx.Get("card"); exists {
		// 响应签名只取决于是否设置接口口令；安全开关仅决定请求是否必须验签。
		if cardContext, ok := value.(卡密请求上下文); ok && cardContext.Api_password != "" {
			data["sign"] = 计算卡密接口签名(cardContext.Api_password, unsignedJSON)
		}
	}
	signedJSON, err := json.Marshal(data)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"state": false, "code": 0, "msg": "响应编码失败", "sign": ""})
		return
	}
	ctx.Data(http.StatusOK, "application/json; charset=utf-8", signedJSON)
}

func 失败提示(ctx *gin.Context, message interface{}) {
	data, ok := message.(gin.H)
	if !ok {
		data = gin.H{"msg": message}
	}
	data["state"], data["code"] = false, 0
	写入卡密响应(ctx, data)
}

func 成功提示(ctx *gin.Context, data interface{}) {
	result, ok := data.(gin.H)
	if !ok {
		result = gin.H{"data": data}
	}
	result["state"], result["code"] = true, 1
	写入卡密响应(ctx, result)
}

var 查询签名参数规则 = regexp.MustCompile(`(^|&)sign=[^&]*`)

// 提取查询签名内容直接删除 sign=xxx 片段，保留其他查询字符串的顺序、编码和
// 分隔符原样不变。sign 固定从查询参数取得，重复 sign 直接视为无效请求。
func 提取查询签名内容(rawQuery string) (string, string, error) {
	matches := 查询签名参数规则.FindAllStringIndex(rawQuery, -1)
	if len(matches) != 1 {
		return "", "", fmt.Errorf("sign参数错误")
	}
	match := matches[0]
	matched := rawQuery[match[0]:match[1]]
	if strings.HasPrefix(matched, "&") {
		matched = matched[1:]
	}
	rawSign := matched[len("sign="):]
	sign, err := url.QueryUnescape(rawSign)
	if err != nil || sign == "" {
		return "", "", fmt.Errorf("sign参数错误")
	}
	removeEnd := match[1]
	if match[0] == 0 && removeEnd < len(rawQuery) && rawQuery[removeEnd] == '&' {
		removeEnd++
	}
	return rawQuery[:match[0]] + rawQuery[removeEnd:], sign, nil
}

// 获取卡密请求签名内容按实际参数来源选择验签内容。POST 只要带有 JSON 正文，
// 就签原始 JSON；没有 JSON 正文的 POST 与 GET 完全兼容，签原始查询字符串。
func 获取卡密请求签名内容(ctx *gin.Context) ([]byte, string, error) {
	if ctx.Request.Method != http.MethodGet && ctx.Request.Method != http.MethodPost {
		return nil, "", fmt.Errorf("接口安全模式只接受GET或POST请求")
	}
	isJSON := strings.Contains(strings.ToLower(ctx.GetHeader("Content-Type")), "application/json")
	if ctx.Request.Method == http.MethodPost && isJSON {
		rawValue, exists := ctx.Get(gin.BodyBytesKey)
		rawJSON, valid := rawValue.([]byte)
		if exists && valid && len(rawJSON) > 0 {
			if !json.Valid(rawJSON) {
				return nil, "", fmt.Errorf("请求JSON格式错误")
			}
			_, sign, err := 提取查询签名内容(ctx.Request.URL.RawQuery)
			if err != nil {
				return nil, "", err
			}
			return rawJSON, sign, nil
		}
	}
	unsignedQuery, sign, err := 提取查询签名内容(ctx.Request.URL.RawQuery)
	if err != nil {
		return nil, "", err
	}
	return []byte(unsignedQuery), sign, nil
}

// 卡密md5验证在安全模式下校验实际传输字节。JSON POST 签正文；GET 和无 JSON
// 正文的 POST 签去除 sign 后的原始查询字符串。服务端不保存 nonce，因此同一
// 原始请求在时间窗口内仍可被重复发送。
func 卡密md5验证(ctx *gin.Context) {
	value, _ := ctx.Get("card")
	cardContext, ok := value.(卡密请求上下文)
	if !ok || !cardContext.Api_safe {
		return
	}
	rawContent, sign, contentErr := 获取卡密请求签名内容(ctx)
	if contentErr != nil {
		失败提示(ctx, contentErr.Error())
		ctx.Abort()
		return
	}
	timestampText := input(ctx, "timestamp")
	timestamp, err := strconv.ParseInt(timestampText, 10, 64)
	if err != nil || timestamp < time.Now().Unix()-300 || timestamp > time.Now().Unix()+300 {
		失败提示(ctx, "时间不正确")
		拒绝日志(ctx)
		ctx.Abort()
		return
	}
	nonce := input(ctx, "nonce")
	if nonce == "" || len([]byte(nonce)) > 64 || strings.IndexFunc(nonce, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
		失败提示(ctx, "nonce格式不正确")
		拒绝日志(ctx)
		ctx.Abort()
		return
	}
	expected := 计算卡密接口签名(cardContext.Api_password, rawContent)
	if subtle.ConstantTimeCompare([]byte(expected), []byte(sign)) != 1 {
		失败提示(ctx, "sign错误")
		拒绝日志(ctx)
		ctx.Abort()
	}
}

// 读取卡密记录直接按主键查询管理员独立卡密表。卡密余额和状态会频繁变化，
// 不在进程内保存无上限缓存，避免大量不同卡密查询造成内存持续增长或读到旧值。
func 读取卡密记录(admin string, card string) (卡密表样式, bool, error) {
	admin = strings.TrimSpace(admin)
	card = strings.ToLower(strings.TrimSpace(card))
	tableName, err := 卡密数据表名(admin)
	if err != nil {
		return 卡密表样式{}, false, err
	}
	if db == nil {
		return 卡密表样式{}, false, fmt.Errorf("数据库尚未初始化")
	}
	var cardRow 卡密表样式
	query := db.Table(tableName).Where("card = ?", card).First(&cardRow)
	if errors.Is(query.Error, gorm.ErrRecordNotFound) {
		return 卡密表样式{}, false, nil
	}
	if query.Error != nil {
		return 卡密表样式{}, false, fmt.Errorf("读取卡密失败")
	}
	return cardRow, true, nil
}

type 卡密设备详情 struct {
	DeviceID        string    `json:"device_id"`
	DeviceAlias     string    `json:"device_alias"`
	Needle          string    `json:"needle"`
	AuthorizedUntil time.Time `json:"authorized_until"`
	Authorized      bool      `json:"authorized"`
	Online          bool      `json:"online"`
}

type 卡密设备统计 struct {
	AuthorizedCount int64    `json:"authorized_device_count"`
	OnlineCount     int64    `json:"online_device_count"`
	DeviceTotal     int64    `json:"device_total"`
	DevicePage      int      `json:"device_page"`
	DevicePageSize  int      `json:"device_page_size"`
	Devices         []卡密设备详情 `json:"devices"`
}

// 读取卡密设备分页参数只允许较小的明细页，避免单个卡密异常累积大量设备时
// 直接返回超大响应；设备总数仍由服务端单独统计。
func 读取卡密设备分页参数(ctx *gin.Context) (int, int) {
	page, _ := strconv.Atoi(input(ctx, "page"))
	if page == 0 {
		page, _ = strconv.Atoi(input(ctx, "当前页"))
	}
	pageSize, _ := strconv.Atoi(input(ctx, "page_size"))
	if pageSize == 0 {
		pageSize, _ = strconv.Atoi(input(ctx, "每页"))
	}
	if page < 1 {
		page = 1
	} else if page > 1000000 {
		page = 1000000
	}
	if pageSize < 1 {
		pageSize = 默认设备详情每页数量
	}
	if pageSize > 最大设备详情每页数量 {
		pageSize = 最大设备详情每页数量
	}
	return page, pageSize
}

// 查询卡密设备统计同时返回数量和设备明细。查询不会扣点、续费或删除会话；
// 在线状态严格按计费规则使用最后心跳推断窗口，needle 按业务要求随设备明细返回。
// 设备明细按页查询，避免异常设备数量导致响应过大。
func 查询卡密设备统计(admin string, card string, softwareID int, now time.Time, page int, pageSize int) (卡密设备统计, error) {
	var result 卡密设备统计
	settings, err := 读取软件设置(db, admin, softwareID)
	if err != nil {
		return result, err
	}
	baseQuery := db_point_device_session.Where("admin = ? AND card = ? AND software = ?", admin, card, softwareID)
	if err := baseQuery.Count(&result.DeviceTotal).Error; err != nil {
		return result, fmt.Errorf("统计设备会话数量失败")
	}
	result.DevicePage = page
	result.DevicePageSize = pageSize
	countRows, err := baseQuery.Select("admin", "card", "software", "device_id", "needle", "authorized_until", "last_heartbeat_at").Rows()
	if err != nil {
		return result, fmt.Errorf("统计设备会话状态失败")
	}
	defer countRows.Close()
	在线截止 := now.Add(-time.Duration(settings.OnlineGraceMinutes) * time.Minute)
	for countRows.Next() {
		var row 点卡设备会话
		if err := baseQuery.ScanRows(countRows, &row); err != nil {
			return result, fmt.Errorf("统计设备会话状态失败")
		}
		// 计数也合并尚未到同步期限的心跳缓存，避免分页后授权设备和在线设备
		// 只统计当前明细页而不是整张卡密的真实数量。
		合并点卡心跳缓存(&row)
		if row.AuthorizedUntil.After(now) {
			result.AuthorizedCount++
		}
		if row.LastHeartbeatAt.After(在线截止) {
			result.OnlineCount++
		}
	}
	if err := countRows.Err(); err != nil {
		return result, fmt.Errorf("统计设备会话状态失败")
	}
	var rows []点卡设备会话
	query := baseQuery.Select("id", "admin", "card", "software", "device_id", "device_alias", "needle", "authorized_until", "last_heartbeat_at").
		Order("authorized_until DESC, device_id ASC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows)
	if query.Error != nil {
		return result, fmt.Errorf("查询设备会话失败")
	}
	result.Devices = make([]卡密设备详情, 0, len(rows))
	for _, row := range rows {
		// 普通心跳可能尚未达到数据库同步期限；详情查询只合并内存快照，
		// 不强制落库也能返回当前真实在线状态。
		合并点卡心跳缓存(&row)
		authorized := row.AuthorizedUntil.After(now)
		online := row.LastHeartbeatAt.After(在线截止)
		result.Devices = append(result.Devices, 卡密设备详情{
			DeviceID: row.DeviceID, DeviceAlias: row.DeviceAlias, Needle: row.Needle,
			AuthorizedUntil: row.AuthorizedUntil, Authorized: authorized, Online: online,
		})
	}
	return result, nil
}

func 卡密_查询详情(ctx *gin.Context) {
	value, _ := ctx.Get("card")
	cardContext, ok := value.(卡密请求上下文)
	if !ok {
		失败提示(ctx, "卡密上下文错误")
		return
	}
	cardRow, found, err := 读取卡密记录(cardContext.Name, cardContext.Card)
	if err != nil {
		失败提示(ctx, err.Error())
		return
	}
	if !found {
		失败提示(ctx, "卡密不存在")
		return
	}
	设备页, 设备每页 := 读取卡密设备分页参数(ctx)
	设备统计, err := 查询卡密设备统计(cardContext.Name, cardRow.Card, cardRow.Software, time.Now(), 设备页, 设备每页)
	if err != nil {
		失败提示(ctx, err.Error())
		return
	}
	lastUse := ""
	if cardRow.Use_time != nil {
		lastUse = cardRow.Use_time.Format(timeLayout)
	}
	status := "正常"
	if cardRow.Card_state == 卡密状态_冻结 {
		status = "冻结"
	}
	text := fmt.Sprintf("卡密:%s\n软件:%d\n点数余额:%d\n最近扣点:%s\n授权设备:%d\n在线设备:%d\n状态:%s", cardRow.Card, cardRow.Software, cardRow.Point_balance, lastUse, 设备统计.AuthorizedCount, 设备统计.OnlineCount, status)
	成功提示(ctx, gin.H{"data": text, "software": cardRow.Software, "point_balance": cardRow.Point_balance, "card_state": cardRow.Card_state, "authorized_device_count": 设备统计.AuthorizedCount, "online_device_count": 设备统计.OnlineCount, "device_total": 设备统计.DeviceTotal, "device_page": 设备统计.DevicePage, "device_page_size": 设备统计.DevicePageSize, "devices": 设备统计.Devices})
}

func 解析整数参数(ctx *gin.Context, key string) (int64, bool) {
	text := strings.TrimSpace(input(ctx, key))
	if text == "" {
		return 0, true
	}
	value, err := strconv.ParseInt(text, 10, 64)
	return value, err == nil
}

// card_login 是点卡客户端的登录入口。软件编号由卡密记录决定，客户端不需要
// 重复提交；period_minutes 不传时使用该软件的默认授权时长。
func card_login(ctx *gin.Context) {
	periodMinutes, valid := 解析整数参数(ctx, "period_minutes")
	if !valid || periodMinutes < 0 || !授权时长分钟有效(periodMinutes, true) {
		失败提示(ctx, "period_minutes参数错误")
		return
	}
	period := 分钟转秒(periodMinutes)
	value, _ := ctx.Get("card")
	cardContext, ok := value.(卡密请求上下文)
	if !ok {
		失败提示(ctx, "卡密上下文错误")
		return
	}
	result, err := 点卡登录并扣费(cardContext.Name, cardContext.Card, input(ctx, "device_id"), input(ctx, "device_alias"), period)
	if err != nil {
		失败提示(ctx, err.Error())
		return
	}
	成功提示(ctx, gin.H{
		"needle":                     result.Session.Needle,
		"authorized_until":           result.Session.AuthorizedUntil,
		"heartbeat_interval_seconds": result.HeartbeatSeconds,
	})
}

func card_ping(ctx *gin.Context) {
	value, _ := ctx.Get("card")
	cardContext, ok := value.(卡密请求上下文)
	if !ok {
		失败提示(ctx, "卡密上下文错误")
		return
	}
	needle := input(ctx, "needle")
	if needle == "" {
		失败提示(ctx, "needle不能为空")
		return
	}
	result, err := 点卡设备心跳(cardContext.Name, cardContext.Card, needle, input(ctx, "device_id"))
	if err != nil {
		失败提示(ctx, err.Error())
		return
	}
	成功提示(ctx, gin.H{
		"needle":                     result.Session.Needle,
		"authorized_until":           result.Session.AuthorizedUntil,
		"heartbeat_interval_seconds": result.HeartbeatSeconds,
	})
}

func card_logout(ctx *gin.Context) {
	value, _ := ctx.Get("card")
	cardContext, ok := value.(卡密请求上下文)
	if !ok {
		失败提示(ctx, "卡密上下文错误")
		return
	}
	if err := 退出点卡设备会话(cardContext.Name, cardContext.Card, input(ctx, "device_id"), input(ctx, "needle")); err != nil {
		失败提示(ctx, err.Error())
		return
	}
	成功提示(ctx, gin.H{"msg": "退出成功"})
}

// 校验卡密配置内容统一管理生成、编辑和卡端写配置的长度边界。
// 按 Unicode 字符计数，避免一个中文字符被 UTF-8 字节数重复计算。
func 校验卡密配置内容(content string) error {
	if len([]rune(content)) > 最大卡密配置字符数 {
		return fmt.Errorf("卡密配置不能超过%d个字符", 最大卡密配置字符数)
	}
	return nil
}

func modify_card_configContent(ctx *gin.Context) {
	value, _ := ctx.Get("card")
	cardContext, ok := value.(卡密请求上下文)
	if !ok {
		失败提示(ctx, "卡密上下文错误")
		return
	}
	tableName, err := 卡密数据表名(cardContext.Name)
	if err != nil {
		失败提示(ctx, err.Error())
		return
	}
	operation := input(ctx, "type")
	if operation == "write" {
		content := input(ctx, "value")
		if err := 校验卡密配置内容(content); err != nil {
			失败提示(ctx, err.Error())
			return
		}
		// 写入与原内容相同时，部分 MySQL 配置会返回 RowsAffected=0；后面的
		// 精确查询会负责判断卡密是否存在，因此这里不能把无变化更新当成失败。
		if result := db.Table(tableName).Where("card = ?", cardContext.Card).Update("config_content", content); result.Error != nil {
			失败提示(ctx, "保存配置失败")
			return
		}
	}
	if operation != "" && operation != "read" && operation != "write" {
		失败提示(ctx, "type参数不正确")
		return
	}
	cardRow, found, err := 读取卡密记录(cardContext.Name, cardContext.Card)
	if err != nil {
		失败提示(ctx, err.Error())
		return
	}
	if !found {
		失败提示(ctx, "卡密不存在")
		return
	}
	成功提示(ctx, cardRow.Config_content)
}

func 解析卡密列表(raw string) ([]string, error) {
	items := strings.FieldsFunc(raw, func(r rune) bool { return r == '\n' || r == '\r' || r == ',' || r == ';' || r == ' ' || r == '\t' })
	if len(items) == 0 {
		return nil, fmt.Errorf("请输入卡密")
	}
	result := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		item = strings.ToLower(strings.TrimSpace(item))
		if !卡密格式规则.MatchString(item) {
			return nil, fmt.Errorf("卡密格式不正确:%s", item)
		}
		if _, exists := seen[item]; exists {
			return nil, fmt.Errorf("本次提交存在重复卡密:%s", item)
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result, nil
}

// 生成随机卡密只负责生成候选值；批量生成时由调用方在事务外统一预检查，
// 最终写入仍由数据库主键唯一约束兜底。
func 生成随机卡密(softwareID int) string {
	return strconv.FormatInt(int64(softwareID), 36) + GetRandomString(16, "a")
}

// 校验并准备生成卡密的参数。卡密文本解析、随机值生成和重复预检查都在事务外
// 完成，避免无效请求或大批量计算持有数据库锁。最终写入仍依赖卡密主键唯一约束，
// 防止预检查完成后被并发请求抢先创建。
func 准备生成卡密(admin string, softwareID int, points int64, count int, customCards string, random bool, notes string, config string) (string, []string, error) {
	admin = strings.TrimSpace(admin)
	if !验证管理员名称(admin) || softwareID <= 0 {
		return "", nil, fmt.Errorf("管理员或软件参数错误")
	}
	if points <= 0 || points > 最大单次点数 || count <= 0 || count > 最大单次生成卡密数量 {
		return "", nil, fmt.Errorf("点数必须在1至%d之间，数量必须在1至%d之间", 最大单次点数, 最大单次生成卡密数量)
	}
	if _, valid := 规范化可显示文本(notes, 500); !valid {
		return "", nil, fmt.Errorf("卡密备注不能包含控制字符且不能超过500个字符")
	}
	if err := 校验卡密配置内容(config); err != nil {
		return "", nil, err
	}
	var specified []string
	if !random {
		var err error
		specified, err = 解析卡密列表(customCards)
		if err != nil {
			return "", nil, err
		}
		if len(specified) != count {
			return "", nil, fmt.Errorf("指定卡密数量与生成数量不一致")
		}
	}
	var softwareRow 软件
	softwareQuery := db.Table("software").Select("id").Where("name = ? AND id = ?", admin, softwareID).First(&softwareRow)
	if errors.Is(softwareQuery.Error, gorm.ErrRecordNotFound) {
		return "", nil, fmt.Errorf("软件不存在")
	}
	if softwareQuery.Error != nil {
		return "", nil, fmt.Errorf("检查软件失败")
	}
	tableName, err := 卡密数据表名(admin)
	if err != nil {
		return "", nil, err
	}
	cards := make([]string, 0, count)
	if random {
		seen := make(map[string]struct{}, count)
		for len(cards) < count {
			card := strings.ToLower(生成随机卡密(softwareID))
			if _, exists := seen[card]; exists {
				continue
			}
			seen[card] = struct{}{}
			cards = append(cards, card)
		}
	} else {
		cards = append(cards, specified...)
	}
	var existing int64
	if err := db.Table(tableName).Where("card IN ?", cards).Count(&existing).Error; err != nil {
		return "", nil, fmt.Errorf("检查卡密是否重复失败")
	}
	if existing > 0 {
		return "", nil, fmt.Errorf("卡密已存在，不能重复生成")
	}
	return tableName, cards, nil
}

// 锁定生成卡密软件只在最终写入事务中短暂锁定软件行，和删除软件保持一致的
// 加锁顺序，避免软件删除与卡密写入并发时产生孤立卡密。
func 锁定生成卡密软件(tx *gorm.DB, admin string, softwareID int) error {
	var softwareRow 软件
	query := tx.Table("software").Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id").Where("name = ? AND id = ?", strings.TrimSpace(admin), softwareID).First(&softwareRow)
	if errors.Is(query.Error, gorm.ErrRecordNotFound) {
		return fmt.Errorf("软件不存在")
	}
	if query.Error != nil {
		return fmt.Errorf("检查软件失败")
	}
	return nil
}

// 创建卡密并记录初始流水只负责持久化已经校验过的卡密。调用方应把它放在自己的
// 事务中，保证卡密和对应的初始流水同时成功或失败。
func 创建卡密并记录初始流水(tx *gorm.DB, tableName string, admin string, agentID int, softwareID int, points int64, cards []string, notes string, config string, now time.Time) error {
	// 准备阶段已经完成校验；这里再次去除首尾空格，确保管理员入口和
	// 代理入口无论调用路径如何，落库内容保持一致。
	var valid bool
	if notes, valid = 规范化可显示文本(notes, 500); !valid {
		return fmt.Errorf("卡密备注格式不正确")
	}
	rows := make([]卡密表样式, 0, len(cards))
	ledgers := make([]点数流水, 0, len(cards))
	remark := "生成卡密初始点数"
	if agentID > 0 {
		remark += fmt.Sprintf("；渠道合伙人ID=%d", agentID)
	}
	for _, card := range cards {
		rows = append(rows, 卡密表样式{Card: card, Create_time: now, Software: softwareID, Card_state: 卡密状态_正常, Point_balance: points, Notes: notes, Config_content: config, AgentID: agentID})
		// 初始余额也作为一条补点流水保存，便于审计同名卡重新生成后的新旧记录。
		ledgers = append(ledgers, 点数流水{Admin: admin, Card: card, Software: softwareID, EventType: 点数事件_补点, Change: points, BalanceBefore: 0, BalanceAfter: points, Remark: remark, CreatedAt: now})
	}
	// 按估算行大小控制每条 INSERT，既减少上千次数据库往返，也避免大配置
	// 让单条 SQL 超过常见的 max_allowed_packet。事务仍保持整批全成或全退。
	estimatedRowBytes := len(config) + len(notes) + 256
	batchSize := (2 * 1024 * 1024) / estimatedRowBytes
	if batchSize < 1 {
		batchSize = 1
	} else if batchSize > 200 {
		batchSize = 200
	}
	if err := tx.Table(tableName).CreateInBatches(&rows, batchSize).Error; err != nil {
		return fmt.Errorf("写入卡密失败")
	}
	if err := tx.Table("point_ledger").CreateInBatches(&ledgers, 500).Error; err != nil {
		return fmt.Errorf("写入初始点数流水失败")
	}
	return nil
}

// 生成并保存卡密采用全量事务：任意卡密重复或数据库错误都会全部回滚，
// 避免管理员得到数量不确定的半成功结果。代理生成卡密时会复用同一
// 套准备/创建函数，并把渠道余额扣减放进同一事务。
func 生成并保存卡密(admin string, agentID int, softwareID int, points int64, count int, customCards string, random bool, notes string, config string) ([]string, error) {
	tableName, cards, err := 准备生成卡密(admin, softwareID, points, count, customCards, random, notes, config)
	if err != nil {
		return nil, err
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := 锁定生成卡密软件(tx, admin, softwareID); err != nil {
			return err
		}
		if err := 创建卡密并记录初始流水(tx, tableName, strings.TrimSpace(admin), agentID, softwareID, points, cards, notes, config, time.Now()); err != nil {
			return fmt.Errorf("生成卡密失败: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return cards, nil
}

type 卡密生成请求 struct {
	Software      int    `json:"software"`
	Points        int64  `json:"points"`
	Num           int    `json:"num"`
	Cards         string `json:"cards"`
	Random        bool   `json:"random"`
	Notes         string `json:"notes"`
	ConfigContent string `json:"config_content"`
}

func 管理员_添加卡密(ctx *gin.Context) {
	var request 卡密生成请求
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil {
		失败提示管理端(ctx, "数据错误")
		return
	}
	account, ok := 管理员_取账号信息(ctx)
	if !ok {
		失败提示管理端(ctx, "登录状态错误")
		return
	}
	cards, err := 生成并保存卡密(account.Name, 0, request.Software, request.Points, request.Num, request.Cards, request.Random, request.Notes, request.ConfigContent)
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	日志("log/"+account.Name+time.Now().Format("200601"), fmt.Sprintf("新增卡密;软件:%d;数量:%d;点数:%d", request.Software, len(cards), request.Points))
	成功提示管理端(ctx, gin.H{"msg": fmt.Sprintf("成功生成%d张卡密", len(cards)), "data": strings.Join(cards, "\n")})
}

type 卡密列表项 struct {
	Card                  string     `json:"card"`
	CreateTime            time.Time  `json:"create_time"`
	UseTime               *time.Time `json:"use_time"`
	Software              int        `json:"software"`
	CardState             int        `json:"card_state"`
	PointBalance          int64      `json:"point_balance"`
	Notes                 string     `json:"notes"`
	ConfigContent         string     `json:"config_content"`
	AgentID               int        `json:"agent_id"`
	AuthorizedDeviceCount int64      `json:"authorized_device_count"`
}

func 读取分页参数(ctx *gin.Context) (int, int) {
	page, _ := strconv.Atoi(input(ctx, "page"))
	if page == 0 {
		page, _ = strconv.Atoi(input(ctx, "当前页"))
	}
	pageSize, _ := strconv.Atoi(input(ctx, "page_size"))
	if pageSize == 0 {
		pageSize, _ = strconv.Atoi(input(ctx, "每页"))
	}
	if page < 1 {
		page = 1
	} else if page > 1000000 {
		page = 1000000
	}
	if pageSize < 1 || pageSize > 500 {
		pageSize = 50
	}
	return page, pageSize
}

// 卡密列表排序只允许固定白名单字段。排序由数据库完成，避免前端只对当前页
// 排序造成分页结果错误；card_order 只作为其他字段相等时的第二排序键。
type 卡密列表排序参数 struct {
	字段   string
	方向   string
	卡密方向 string
}

func 读取卡密列表排序(ctx *gin.Context) 卡密列表排序参数 {
	field := strings.TrimSpace(input(ctx, "sort_by"))
	order := strings.ToLower(strings.TrimSpace(input(ctx, "sort_order")))
	cardOrder := strings.ToLower(strings.TrimSpace(input(ctx, "card_order")))
	allowed := map[string]bool{
		"card": true, "software": true, "point_balance": true, "card_state": true,
		"create_time": true, "use_time": true,
	}
	if !allowed[field] {
		field = "create_time"
		order = "desc"
	}
	if order != "asc" && order != "desc" {
		order = "desc"
	}
	if cardOrder != "asc" && cardOrder != "desc" {
		cardOrder = "asc"
	}
	return 卡密列表排序参数{字段: field, 方向: order, 卡密方向: cardOrder}
}

// 应用卡密列表排序生成固定 SQL 片段。字段和方向均来自白名单，不能直接
// 使用客户端传入的原始字符串；卡密本身是唯一键，作为第二排序键时可保证
// 相同余额、状态或时间的记录翻页顺序稳定。
func 应用卡密列表排序(query *gorm.DB, tableName string, sorting 卡密列表排序参数, admin string, now time.Time) *gorm.DB {
	if sorting.字段 == "card" {
		return query.Order("`card` " + sorting.方向)
	}
	return query.Order("`" + sorting.字段 + "` " + sorting.方向 + ", `card` " + sorting.卡密方向)
}

// 查询卡密列表是管理员和代理账号共用的查询实现；agentID 非零时限制到该代理
// 创建的卡密。卡密类型和时长筛选已删除，所有记录都是纯点卡卡密。
func 查询卡密列表(ctx *gin.Context, admin string, agentID int) {
	tableName, err := 卡密数据表名(admin)
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	softwareID, _ := strconv.Atoi(input(ctx, "software"))
	state, _ := strconv.Atoi(input(ctx, "card_state"))
	cardKeyword := strings.ToLower(strings.TrimSpace(input(ctx, "card")))
	notesKeyword := strings.TrimSpace(input(ctx, "notes"))
	if len([]rune(cardKeyword)) > 63 || len([]rune(notesKeyword)) > 500 {
		失败提示管理端(ctx, "卡密或备注筛选条件过长")
		return
	}
	sorting := 读取卡密列表排序(ctx)
	now := time.Now()
	query := db.Table(tableName)
	if softwareID > 0 {
		query = query.Where("software = ?", softwareID)
	}
	if state == 卡密状态_正常 || state == 卡密状态_冻结 {
		query = query.Where("card_state = ?", state)
	}
	if agentID > 0 {
		query = query.Where("agent_id = ?", agentID)
	}
	if cardKeyword != "" {
		query = query.Where("card LIKE ?", "%"+转义Like文本(cardKeyword)+"%")
	}
	if notesKeyword != "" {
		query = query.Where("notes LIKE ?", "%"+转义Like文本(notesKeyword)+"%")
	}
	var total int64
	// Count 会临时修改 SELECT 子句；使用独立会话避免后续 Find 继承 count(*)。
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		失败提示管理端(ctx, "查询卡密数量失败")
		return
	}
	page, pageSize := 读取分页参数(ctx)
	var rows []卡密表样式
	query = 应用卡密列表排序(query, tableName, sorting, admin, now)
	if err := query.Limit(pageSize).Offset((page - 1) * pageSize).Find(&rows).Error; err != nil {
		失败提示管理端(ctx, "查询卡密失败")
		return
	}
	counts := make(map[string]int64, len(rows))
	if len(rows) > 0 {
		type countRow struct {
			Card  string
			Count int64
		}
		var grouped []countRow
		cards := make([]string, 0, len(rows))
		for _, row := range rows {
			cards = append(cards, row.Card)
		}
		if err := db_point_device_session.Where("admin = ? AND card IN ? AND authorized_until > ?", admin, cards, now).Select("card, COUNT(*) AS count").Group("card").Scan(&grouped).Error; err != nil {
			失败提示管理端(ctx, "查询授权设备数量失败")
			return
		}
		for _, item := range grouped {
			counts[item.Card] = item.Count
		}
	}
	data := make([]卡密列表项, 0, len(rows))
	for _, row := range rows {
		data = append(data, 卡密列表项{Card: row.Card, CreateTime: row.Create_time, UseTime: row.Use_time, Software: row.Software, CardState: row.Card_state, PointBalance: row.Point_balance, Notes: row.Notes, ConfigContent: row.Config_content, AgentID: row.AgentID, AuthorizedDeviceCount: counts[row.Card]})
	}
	成功提示管理端(ctx, gin.H{"data": data, "num": total, "page": page, "page_size": pageSize})
}

func 管理员_查询所有卡密(ctx *gin.Context) {
	account, ok := 管理员_取账号信息(ctx)
	if !ok {
		失败提示管理端(ctx, "登录状态错误")
		return
	}
	查询卡密列表(ctx, account.Name, 0)
}

func 查询操作日志(ctx *gin.Context) {
	account, ok := 管理员_取账号信息(ctx)
	if !ok {
		失败提示管理端(ctx, "登录状态错误")
		return
	}
	read := func(month time.Time) string {
		content, err := os.ReadFile("log/" + account.Name + month.Format("200601"))
		if err != nil {
			return "没有其他内容"
		}
		return string(content)
	}
	ctx.String(http.StatusOK, read(time.Now())+"\n"+read(time.Now().AddDate(0, -1, 0)))
}

// 校验代理卡密归属在失效心跳缓存前确认代理确实拥有该卡，避免无权操作请求
// 仅凭已知卡密名称干扰其他渠道的设备心跳。管理员操作无需额外查询。
func 校验代理卡密归属(tableName string, agentID int, card string) error {
	if agentID <= 0 {
		return nil
	}
	var count int64
	if err := db.Table(tableName).Where("card = ? AND agent_id = ?", card, agentID).Count(&count).Error; err != nil {
		return fmt.Errorf("检查卡密权限失败")
	}
	if count != 1 {
		return fmt.Errorf("卡密不存在或无权操作")
	}
	return nil
}

func 删除卡密记录(admin string, agentID int, cards []string) ([]string, []string, error) {
	if len(cards) == 0 || len(cards) > 1000 {
		return nil, cards, fmt.Errorf("单次操作卡密数量必须在1至1000之间")
	}
	tableName, err := 卡密数据表名(admin)
	if err != nil {
		return nil, cards, err
	}
	成功 := make([]string, 0, len(cards))
	失败 := make([]string, 0)
	type 待删除卡密 struct {
		原始值 string
		卡密  string
	}
	待删除 := make([]待删除卡密, 0, len(cards))
	for _, raw := range cards {
		card := strings.ToLower(strings.TrimSpace(raw))
		if !卡密格式规则.MatchString(card) {
			失败 = append(失败, raw)
			continue
		}
		if err := 校验代理卡密归属(tableName, agentID, card); err != nil {
			失败 = append(失败, raw)
			continue
		}
		待删除 = append(待删除, 待删除卡密{原始值: raw, 卡密: card})
	}

	// 删除前把整批卡密的心跳缓存同步并失效，只扫描一次缓存。单张卡同步失败
	// 时仍保持原有的部分成功语义，仅跳过对应卡密，不影响其他卡密删除。
	待删除卡密值 := make([]string, 0, len(待删除))
	for _, item := range 待删除 {
		待删除卡密值 = append(待删除卡密值, item.卡密)
	}
	同步错误 := 同步并删除卡密心跳缓存_按卡返回错误(admin, 待删除卡密值)
	成功卡密值 := make([]string, 0, len(待删除))
	for _, item := range 待删除 {
		if _, exists := 同步错误[item.卡密]; exists {
			失败 = append(失败, item.原始值)
			continue
		}
		err := db.Transaction(func(tx *gorm.DB) error {
			query := tx.Table(tableName).Where("card = ?", item.卡密)
			if agentID > 0 {
				query = query.Where("agent_id = ?", agentID)
			}
			result := query.Delete(&卡密表样式{})
			if result.Error != nil || result.RowsAffected != 1 {
				return fmt.Errorf("卡密不存在或无权删除")
			}
			return tx.Table("point_device_session").Where("admin = ? AND card = ?", admin, item.卡密).Delete(&点卡设备会话{}).Error
		})
		if err != nil {
			失败 = append(失败, item.原始值)
			continue
		}
		成功 = append(成功, item.原始值)
		成功卡密值 = append(成功卡密值, item.卡密)
	}
	// 删除事务期间可能出现新的并发心跳，提交后对实际删除成功的卡密再统一
	// 扫描并失效一次；此时业务已经成功，缓存错误只写日志，不能改写结果。
	if cacheErr := 同步并删除卡密心跳缓存(admin, 成功卡密值); cacheErr != nil {
		日志("log/启动记录.txt", "批量删除卡密后同步心跳缓存失败:"+cacheErr.Error())
	}
	return 成功, 失败, nil
}

func 管理员_删除卡密(ctx *gin.Context) {
	var request struct {
		Cards []string `json:"cards"`
	}
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil {
		失败提示管理端(ctx, "数据错误")
		return
	}
	account, ok := 管理员_取账号信息(ctx)
	if !ok {
		失败提示管理端(ctx, "登录状态错误")
		return
	}
	success, failed, err := 删除卡密记录(account.Name, 0, request.Cards)
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	deleteLog := fmt.Sprintf("删除卡密;成功:%v;失败:%v", success, failed)
	日志("log/"+account.Name+time.Now().Format("200601"), 清理拒绝日志字段(deleteLog, 4000))
	成功提示管理端(ctx, gin.H{"msg": fmt.Sprintf("成功%d张，失败%d张", len(success), len(failed)), "success": success, "failed": failed})
}

func 修改卡密记录(admin string, agentID int, cardValue string, notes *string, config *string, state int) error {
	admin = strings.TrimSpace(admin)
	if !验证管理员名称(admin) {
		return fmt.Errorf("管理员名称格式不正确")
	}
	card := strings.ToLower(strings.TrimSpace(cardValue))
	if !卡密格式规则.MatchString(card) {
		return fmt.Errorf("卡密格式不正确")
	}
	if state != 0 && state != 卡密状态_正常 && state != 卡密状态_冻结 {
		return fmt.Errorf("卡密状态不正确")
	}
	if notes != nil {
		normalized, valid := 规范化可显示文本(*notes, 500)
		if !valid {
			return fmt.Errorf("卡密备注不能包含控制字符且不能超过500个字符")
		}
		*notes = normalized
	}
	if config != nil {
		if err := 校验卡密配置内容(*config); err != nil {
			return err
		}
	}
	tableName, err := 卡密数据表名(admin)
	if err != nil {
		return err
	}
	updates := map[string]interface{}{}
	if notes != nil {
		updates["notes"] = *notes
	}
	if config != nil {
		updates["config_content"] = *config
	}
	if state != 0 {
		updates["card_state"] = state
	}
	if len(updates) == 0 {
		return fmt.Errorf("没有需要修改的内容")
	}
	if err := 校验代理卡密归属(tableName, agentID, card); err != nil {
		return err
	}
	// 卡密状态或属性发生变化时统一失效设备心跳快照。即使只修改备注，下一次
	// 心跳重新加载数据库也能保证所有管理操作采用同一条缓存处理路径。
	if err := 同步并删除卡密心跳缓存(admin, []string{card}); err != nil {
		return err
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		query := tx.Table(tableName).Clauses(clause.Locking{Strength: "UPDATE"}).Where("card = ?", card)
		if agentID > 0 {
			query = query.Where("agent_id = ?", agentID)
		}
		var current 卡密表样式
		if result := query.First(&current); result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				return fmt.Errorf("卡密不存在或无权修改")
			}
			return fmt.Errorf("读取卡密失败")
		}
		if result := tx.Table(tableName).Where("card = ?", current.Card).Updates(updates); result.Error != nil {
			return fmt.Errorf("保存卡密失败")
		}
		if state == 卡密状态_冻结 {
			if result := tx.Table("point_device_session").Where("admin = ? AND card = ?", admin, card).Delete(&点卡设备会话{}); result.Error != nil {
				return fmt.Errorf("清理冻结设备会话失败")
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if cacheErr := 同步并删除卡密心跳缓存(admin, []string{card}); cacheErr != nil {
		// 卡密修改已经提交，第二次失效只用于清除事务期间并发建立的快照。
		日志("log/启动记录.txt", "修改卡密后同步心跳缓存失败:"+cacheErr.Error())
	}
	return nil
}

func modify_card(ctx *gin.Context) {
	var request struct {
		Card          string  `json:"card"`
		Notes         *string `json:"notes"`
		ConfigContent *string `json:"config_content"`
		CardState     int     `json:"card_state"`
	}
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil {
		失败提示管理端(ctx, "数据错误")
		return
	}
	account, ok := 管理员_取账号信息(ctx)
	if !ok {
		失败提示管理端(ctx, "登录状态错误")
		return
	}
	if err := 修改卡密记录(account.Name, 0, request.Card, request.Notes, request.ConfigContent, request.CardState); err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	成功提示管理端(ctx, gin.H{"msg": "修改成功"})
}

func 修改卡密_批量(admin string, agentID int, cards []string, state int) ([]string, []string, error) {
	if state != 卡密状态_正常 && state != 卡密状态_冻结 {
		return nil, cards, fmt.Errorf("卡密状态不正确")
	}
	if len(cards) == 0 || len(cards) > 1000 {
		return nil, cards, fmt.Errorf("单次操作卡密数量必须在1至1000之间")
	}
	success, failed := []string{}, []string{}
	for _, card := range cards {
		if err := 修改卡密记录(admin, agentID, card, nil, nil, state); err != nil {
			failed = append(failed, card)
		} else {
			success = append(success, card)
		}
	}
	return success, failed, nil
}

func 管理员_冻卡s(ctx *gin.Context) {
	var request struct {
		Cards     []string `json:"cards"`
		CardState int      `json:"card_state"`
	}
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil {
		失败提示管理端(ctx, "数据错误")
		return
	}
	account, ok := 管理员_取账号信息(ctx)
	if !ok {
		失败提示管理端(ctx, "登录状态错误")
		return
	}
	success, failed, err := 修改卡密_批量(account.Name, 0, request.Cards, request.CardState)
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	成功提示管理端(ctx, gin.H{"msg": fmt.Sprintf("成功%d张，失败%d张", len(success), len(failed)), "success": success, "failed": failed})
}
