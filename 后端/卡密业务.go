package main

import (
	"bufio"
	"crypto/md5"
	cryptorand "crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 日志锁保护文本运行日志；本文件其余内容按“卡端基础接口、点卡生成、
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

func 加入时间戳(ctx *gin.Context, data gin.H) gin.H {
	value, _ := ctx.Get("card")
	cardContext, _ := value.(卡密请求上下文)
	timestamp, err := strconv.ParseInt(input(ctx, "timestamp"), 10, 64)
	if err != nil {
		timestamp = time.Now().Unix()
	}
	timestamp += 10
	code, _ := data["code"].(int)
	hash := md5.Sum([]byte(strconv.FormatInt(timestamp, 10) + cardContext.Api_password + strconv.Itoa(code)))
	data["sign"] = fmt.Sprintf("%x", hash)
	data["timestamp"] = timestamp
	return data
}

func 失败提示(ctx *gin.Context, message interface{}) {
	data, ok := message.(gin.H)
	if !ok {
		data = gin.H{"msg": message}
	}
	data["state"], data["code"] = false, 0
	ctx.JSON(http.StatusOK, 加入时间戳(ctx, data))
}

func 成功提示(ctx *gin.Context, data interface{}) {
	result, ok := data.(gin.H)
	if !ok {
		result = gin.H{"data": data}
	}
	result["state"], result["code"] = true, 1
	ctx.JSON(http.StatusOK, 加入时间戳(ctx, result))
}

// 卡密md5验证保留原有可选的接口口令校验机制；系统不要求 HTTPS。该校验可阻止
// 不知道安全密码的请求，但不提供传输加密，HTTP 部署方仍需自行评估链路风险。
func 卡密md5验证(ctx *gin.Context) {
	value, _ := ctx.Get("card")
	cardContext, ok := value.(卡密请求上下文)
	if !ok || !cardContext.Api_safe {
		return
	}
	timestampText, sign := input(ctx, "timestamp"), input(ctx, "sign")
	timestamp, err := strconv.ParseInt(timestampText, 10, 64)
	if err != nil || timestamp < time.Now().Unix()-600 || timestamp > time.Now().Unix()+600 {
		失败提示(ctx, "时间不正确")
		拒绝日志(ctx)
		ctx.Abort()
		return
	}
	hash := md5.Sum([]byte(timestampText + cardContext.Api_password))
	if subtle.ConstantTimeCompare([]byte(fmt.Sprintf("%x", hash)), []byte(sign)) != 1 {
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

func 卡密_查询心跳(ctx *gin.Context) {
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
	var sessions int64
	if err := db_point_device_session.Where("admin = ? AND card = ? AND authorized_until > ?", cardContext.Name, cardRow.Card, time.Now()).Count(&sessions).Error; err != nil {
		失败提示(ctx, "查询授权设备失败")
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
	text := fmt.Sprintf("卡密:%s\n软件:%d\n点数余额:%d\n最近扣点:%s\n有效授权设备:%d\n状态:%s", cardRow.Card, cardRow.Software, cardRow.Point_balance, lastUse, sessions, status)
	成功提示(ctx, gin.H{"data": text, "software": cardRow.Software, "point_balance": cardRow.Point_balance, "card_state": cardRow.Card_state, "authorized_device_count": sessions})
}

func 解析整数参数(ctx *gin.Context, key string) (int64, bool) {
	text := strings.TrimSpace(input(ctx, key))
	if text == "" {
		return 0, true
	}
	value, err := strconv.ParseInt(text, 10, 64)
	return value, err == nil
}

// card_login 是点卡客户端的登录入口。period_seconds 可由客户端选择，
// 服务器会验证该授权时长对应的计费方案是否存在且启用；不传时使用软件默认授权时长。
func card_login(ctx *gin.Context) {
	softwareID, err := strconv.Atoi(input(ctx, "software"))
	if err != nil || softwareID <= 0 {
		失败提示(ctx, "software参数错误")
		return
	}
	period, valid := 解析整数参数(ctx, "period_seconds")
	if !valid || period < 0 {
		失败提示(ctx, "period_seconds参数错误")
		return
	}
	value, _ := ctx.Get("card")
	cardContext, ok := value.(卡密请求上下文)
	if !ok {
		失败提示(ctx, "卡密上下文错误")
		return
	}
	deviceID := input(ctx, "device_id")
	if strings.TrimSpace(deviceID) == "" {
		失败提示(ctx, "点卡登录必须提供device_id")
		return
	}
	result, err := 点卡登录并扣费(cardContext.Name, cardContext.Card, softwareID, deviceID, input(ctx, "device_alias"), period)
	if err != nil {
		失败提示(ctx, err.Error())
		return
	}
	charge := result.Charge
	成功提示(ctx, gin.H{
		"needle":                     result.Session.Needle,
		"software":                   result.Session.Software,
		"device_id":                  result.Session.DeviceID,
		"device_alias":               result.Session.DeviceAlias,
		"renewal_period_seconds":     result.Session.RenewalPeriodSeconds,
		"authorized_until":           result.Session.AuthorizedUntil,
		"heartbeat_interval_seconds": result.HeartbeatSeconds,
		"point_balance":              result.Card.Point_balance,
		"charged":                    charge.Charged,
		"cost":                       charge.Cost,
		"ledger_id":                  charge.LedgerID,
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
	result, err := 点卡设备心跳(cardContext.Name, cardContext.Card, needle, input(ctx, "device_id"), input(ctx, "device_alias"))
	if err != nil {
		失败提示(ctx, err.Error())
		return
	}
	成功提示(ctx, gin.H{
		"needle":                     result.Session.Needle,
		"device_id":                  result.Session.DeviceID,
		"device_alias":               result.Session.DeviceAlias,
		"authorized_until":           result.Session.AuthorizedUntil,
		"renewal_period_seconds":     result.Session.RenewalPeriodSeconds,
		"heartbeat_interval_seconds": result.HeartbeatSeconds,
		"charged":                    result.Charge.Charged,
		"cost":                       result.Charge.Cost,
		"point_balance":              result.Charge.Balance,
		"ledger_id":                  result.Charge.LedgerID,
	})
}

func card_logout(ctx *gin.Context) {
	value, _ := ctx.Get("card")
	cardContext, ok := value.(卡密请求上下文)
	if !ok {
		失败提示(ctx, "卡密上下文错误")
		return
	}
	softwareID, err := strconv.Atoi(input(ctx, "software"))
	if err != nil || softwareID <= 0 {
		失败提示(ctx, "software参数错误")
		return
	}
	if err := 退出点卡设备会话(cardContext.Name, cardContext.Card, softwareID, input(ctx, "device_id"), input(ctx, "needle")); err != nil {
		失败提示(ctx, err.Error())
		return
	}
	成功提示(ctx, gin.H{"msg": "退出成功"})
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
		if len([]byte(content)) > 1024*1024 {
			失败提示(ctx, "配置内容不能超过1MB")
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

func 生成随机点卡卡密(softwareID int) string {
	return strconv.FormatInt(int64(softwareID), 36) + GetRandomString(16, "a")
}

// 校验并准备点卡生成参数。所有调用方都在同一个事务连接上执行，
// 这样软件存在性、卡密重复检查和后续写入使用的是同一套数据库视图。
// 删除后重新使用原卡密是允许的；只要旧记录已经删除，本次即可重新创建。
func 准备点卡生成(tx *gorm.DB, admin string, softwareID int, points int64, count int, customCards string, random bool, notes string, config string) (string, []string, error) {
	admin = strings.TrimSpace(admin)
	if !验证管理员名称(admin) || softwareID <= 0 {
		return "", nil, fmt.Errorf("管理员或软件参数错误")
	}
	// 锁定所属软件，和删除软件、修改计费配置使用相同的同步点。
	// 这样并发删除软件时，不会在软件已经删除后又写入一批孤立卡密。
	var softwareRow 软件
	softwareQuery := tx.Table("software").Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id").Where("name = ? AND id = ?", admin, softwareID).First(&softwareRow)
	if errors.Is(softwareQuery.Error, gorm.ErrRecordNotFound) {
		return "", nil, fmt.Errorf("软件不存在")
	}
	if softwareQuery.Error != nil {
		return "", nil, fmt.Errorf("检查软件失败")
	}
	if points <= 0 || points > 最大单次点数 || count <= 0 || count > 1000 {
		return "", nil, fmt.Errorf("点数必须在1至%d之间，数量必须在1至1000之间", 最大单次点数)
	}
	var notesValid bool
	if notes, notesValid = 规范化可显示文本(notes, 500); !notesValid {
		return "", nil, fmt.Errorf("卡密备注不能包含控制字符且不能超过500个字符")
	}
	if len([]byte(config)) > 1024*1024 {
		return "", nil, fmt.Errorf("卡密配置不能超过1MB")
	}
	// 配置会随每张卡独立保存。限制一批任务实际写入的配置总量，避免用一个
	// 接近 1MB 的模板一次生成 1000 张卡，形成超大事务并占满数据库连接。
	if int64(len([]byte(config)))*int64(count) > 16*1024*1024 {
		return "", nil, fmt.Errorf("本批卡密配置总量不能超过16MB，请减少数量或配置内容")
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
	tableName, err := 卡密数据表名(admin)
	if err != nil {
		return "", nil, err
	}
	for attempt := 0; attempt < 5; attempt++ {
		cards := make([]string, 0, count)
		if random {
			seen := make(map[string]struct{}, count)
			for len(cards) < count {
				card := strings.ToLower(生成随机点卡卡密(softwareID))
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
		if err := tx.Table(tableName).Where("card IN ?", cards).Count(&existing).Error; err != nil {
			return "", nil, fmt.Errorf("检查卡密是否重复失败")
		}
		if existing == 0 {
			return tableName, cards, nil
		}
		if !random {
			return "", nil, fmt.Errorf("卡密已存在，不能重复生成")
		}
	}
	return "", nil, fmt.Errorf("随机卡密生成失败，请重试")
}

// 写入点卡记录只负责持久化已经校验过的卡密。调用方应把它放在自己的
// 事务中；卡密行和同名旧会话一起处理，保证删除后重用时不会继承旧设备授权。
func 写入点卡记录(tx *gorm.DB, tableName string, admin string, agentID int, softwareID int, points int64, cards []string, notes string, config string, now time.Time) error {
	// 准备阶段已经完成校验；这里再次去除首尾空格，确保管理员入口和
	// 代理入口无论调用路径如何，落库内容保持一致。
	var valid bool
	if notes, valid = 规范化可显示文本(notes, 500); !valid {
		return fmt.Errorf("卡密备注格式不正确")
	}
	rows := make([]卡密表样式, 0, len(cards))
	ledgers := make([]点数流水, 0, len(cards))
	remark := "生成点卡初始点数"
	if agentID > 0 {
		remark += fmt.Sprintf("；代理账号ID=%d", agentID)
	}
	for _, card := range cards {
		rows = append(rows, 卡密表样式{Card: card, Create_time: now, Software: softwareID, Card_state: 卡密状态_正常, Point_balance: points, Notes: notes, Config_content: config, AgentID: agentID})
		// 初始余额也作为一条补点流水保存。这样新卡的余额来源可审计，
		// 同名卡删除后重新生成时，旧流水和新一代初始流水仍能明确区分。
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
	// 同名卡删除后可以重新利用，重建时清理上一代遗留会话；流水保留。
	if err := tx.Table("point_device_session").Where("admin = ? AND card IN ?", admin, cards).Delete(&点卡设备会话{}).Error; err != nil {
		return fmt.Errorf("清理旧设备会话失败")
	}
	return nil
}

// 生成点卡记录采用全量事务：任意卡密重复或数据库错误都会全部回滚，
// 避免管理员得到数量不确定的半成功结果。代理账号生成点卡时会复用同一
// 套准备/写入函数，并把代理余额扣减放进同一事务。
func 生成点卡记录(admin string, agentID int, softwareID int, points int64, count int, customCards string, random bool, notes string, config string) ([]string, error) {
	var cards []string
	err := db.Transaction(func(tx *gorm.DB) error {
		tableName, prepared, err := 准备点卡生成(tx, admin, softwareID, points, count, customCards, random, notes, config)
		if err != nil {
			return err
		}
		cards = prepared
		if err := 写入点卡记录(tx, tableName, strings.TrimSpace(admin), agentID, softwareID, points, cards, notes, config, time.Now()); err != nil {
			return fmt.Errorf("生成点卡失败: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return cards, nil
}

type 点卡生成请求 struct {
	Software      int    `json:"software"`
	Points        int64  `json:"points"`
	Num           int    `json:"num"`
	Cards         string `json:"cards"`
	Random        bool   `json:"random"`
	Notes         string `json:"notes"`
	ConfigContent string `json:"config_content"`
}

func 管理员_add_new_card(ctx *gin.Context) {
	var request 点卡生成请求
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil {
		失败提示管理端(ctx, "数据错误")
		return
	}
	account, ok := 管理员_取账号信息(ctx)
	if !ok {
		失败提示管理端(ctx, "登录状态错误")
		return
	}
	cards, err := 生成点卡记录(account.Name, 0, request.Software, request.Points, request.Num, request.Cards, request.Random, request.Notes, request.ConfigContent)
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	日志("log/"+account.Name+time.Now().Format("200601"), fmt.Sprintf("新增点卡;软件:%d;数量:%d;点数:%d", request.Software, len(cards), request.Points))
	成功提示管理端(ctx, gin.H{"msg": fmt.Sprintf("成功生成%d张点卡", len(cards)), "data": strings.Join(cards, "\n")})
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

// 查询卡密列表是管理员和代理账号共用的查询实现；agentID 非零时限制到该代理
// 创建的卡密。点卡类型和时长筛选已删除，所有记录天然都是点卡。
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
	if err := query.Count(&total).Error; err != nil {
		失败提示管理端(ctx, "查询卡密数量失败")
		return
	}
	page, pageSize := 读取分页参数(ctx)
	var rows []卡密表样式
	if err := query.Order("create_time DESC, card ASC").Limit(pageSize).Offset((page - 1) * pageSize).Find(&rows).Error; err != nil {
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
		if err := db_point_device_session.Where("admin = ? AND card IN ? AND authorized_until > ?", admin, cards, time.Now()).Select("card, COUNT(*) AS count").Group("card").Scan(&grouped).Error; err != nil {
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
	for _, raw := range cards {
		card := strings.ToLower(strings.TrimSpace(raw))
		if !卡密格式规则.MatchString(card) {
			失败 = append(失败, raw)
			continue
		}
		err := db.Transaction(func(tx *gorm.DB) error {
			query := tx.Table(tableName).Where("card = ?", card)
			if agentID > 0 {
				query = query.Where("agent_id = ?", agentID)
			}
			result := query.Delete(&卡密表样式{})
			if result.Error != nil || result.RowsAffected != 1 {
				return fmt.Errorf("卡密不存在或无权删除")
			}
			return tx.Table("point_device_session").Where("admin = ? AND card = ?", admin, card).Delete(&点卡设备会话{}).Error
		})
		if err != nil {
			失败 = append(失败, raw)
			continue
		}
		成功 = append(成功, raw)
	}
	return 成功, 失败, nil
}

func 管理员_delete_card(ctx *gin.Context) {
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
	deleteLog := fmt.Sprintf("删除点卡;成功:%v;失败:%v", success, failed)
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
	if config != nil && len([]byte(*config)) > 1024*1024 {
		return fmt.Errorf("卡密配置不能超过1MB")
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
