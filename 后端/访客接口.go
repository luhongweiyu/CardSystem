package main

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 访客接口只允许通过管理员公开的 center_id 定位租户，不接受客户端直接
// 指定管理员名称。center_id 来自主账号 ID，查询结果不会泄露账号设置。
func visitor_验证对应id(ctx *gin.Context) {
	centerID, err := strconv.Atoi(input(ctx, "center_id"))
	if err != nil || centerID <= 0 {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "code": 0, "msg": "center_id错误"})
		ctx.Abort()
		return
	}
	name := 全局_运行状态.读取用户名(centerID)
	if name == "" {
		var account user
		if query := db_user.Where("id = ?", centerID).First(&account); query.Error != nil {
			ctx.JSON(http.StatusOK, gin.H{"state": false, "code": 0, "msg": "center_id错误"})
			ctx.Abort()
			return
		}
		name = account.Name
		_ = user_刷新用户设置(name)
	}
	settings, ok := 全局_运行状态.读取用户设置(name)
	if !ok {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "code": 0, "msg": "管理员设置不存在"})
		ctx.Abort()
		return
	}
	if !请求防火墙(name) {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "code": 0, "msg": "api次数超限"})
		ctx.Abort()
		return
	}
	ctx.Set("访客管理员", name)
	ctx.Set("访客设置", settings)
	ctx.Next()
}

func 访客管理员(ctx *gin.Context) (string, bool) {
	name, ok := ctx.Get("访客管理员")
	if !ok {
		return "", false
	}
	admin, ok := name.(string)
	return admin, ok && 验证管理员名称(admin)
}

func 访客读取点卡卡密(ctx *gin.Context) (string, string, 点卡表样式, error) {
	admin, ok := 访客管理员(ctx)
	if !ok {
		return "", "", 点卡表样式{}, fmt.Errorf("访客上下文错误")
	}
	card := strings.ToLower(strings.TrimSpace(input(ctx, "card")))
	if !卡密格式规则.MatchString(card) {
		return "", "", 点卡表样式{}, fmt.Errorf("卡密格式不正确")
	}
	tableName, err := 点卡数据表名(admin)
	if err != nil {
		return "", "", 点卡表样式{}, err
	}
	var row 点卡表样式
	query := db.Table(tableName).Where("card = ?", card).First(&row)
	if errors.Is(query.Error, gorm.ErrRecordNotFound) {
		return "", "", row, fmt.Errorf("卡密不存在")
	}
	if query.Error != nil {
		return "", "", row, fmt.Errorf("查询卡密失败")
	}
	return admin, card, row, nil
}

const (
	访客查卡每分钟上限  = 10
	访客查卡默认每页   = 20
	访客查卡每页上限   = 100
	访客批量精确查卡上限 = 20
)

var 访客查卡前缀规则 = regexp.MustCompile(`^[a-z0-9_-]{4,60}\*\*\*$`)

type 访客查卡频率记录 struct {
	开始 time.Time
	次数 int
}

var 访客查卡频率 = struct {
	sync.Mutex
	记录   map[string]访客查卡频率记录
	上次清理 time.Time
}{记录: make(map[string]访客查卡频率记录)}

// 公开查卡可能被用于枚举卡密，按租户和来源 IP 限制查询频率；定期清理
// 过期记录，避免进程长期运行时限流表无限增长。独立详情接口不受此限制。
func 允许访客查找卡密(admin, ip string) bool {
	now := time.Now()
	key := admin + "\x00" + ip
	访客查卡频率.Lock()
	defer 访客查卡频率.Unlock()
	if now.Sub(访客查卡频率.上次清理) >= 5*time.Minute {
		for key, value := range 访客查卡频率.记录 {
			if now.Sub(value.开始) >= 5*time.Minute {
				delete(访客查卡频率.记录, key)
			}
		}
		访客查卡频率.上次清理 = now
	}
	value := 访客查卡频率.记录[key]
	if now.Sub(value.开始) >= time.Minute {
		value.开始, value.次数 = now, 0
	}
	if value.次数 >= 访客查卡每分钟上限 {
		return false
	}
	value.次数++
	访客查卡频率.记录[key] = value
	return true
}

type 访客查卡方式 uint8

const (
	访客查卡精确 访客查卡方式 = iota + 1
	访客查卡前缀
	访客查卡多张精确
)

type 访客查卡条件 struct {
	方式   访客查卡方式
	卡密   string
	前缀   string
	卡密列表 []string
}

func 解析访客查卡条件(keyword string) (访客查卡条件, error) {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if strings.ContainsAny(keyword, ",，") {
		items := strings.FieldsFunc(keyword, func(r rune) bool { return r == ',' || r == '，' })
		if len(items) == 0 || len(items) > 访客批量精确查卡上限 {
			return 访客查卡条件{}, fmt.Errorf("批量查询请提供1至%d张卡密", 访客批量精确查卡上限)
		}
		cards := make([]string, 0, len(items))
		seen := make(map[string]struct{}, len(items))
		for _, item := range items {
			card := strings.TrimSpace(item)
			if !卡密格式规则.MatchString(card) {
				return 访客查卡条件{}, fmt.Errorf("批量查询只支持完整卡密")
			}
			if _, exists := seen[card]; exists {
				continue
			}
			seen[card] = struct{}{}
			cards = append(cards, card)
		}
		return 访客查卡条件{方式: 访客查卡多张精确, 卡密列表: cards}, nil
	}
	if strings.Contains(keyword, "*") {
		if !访客查卡前缀规则.MatchString(keyword) {
			return 访客查卡条件{}, fmt.Errorf("前缀查询格式为至少4位前缀加***")
		}
		return 访客查卡条件{方式: 访客查卡前缀, 前缀: strings.TrimSuffix(keyword, "***")}, nil
	}
	if !卡密格式规则.MatchString(keyword) {
		return 访客查卡条件{}, fmt.Errorf("请输入完整卡密，或输入至少4位前缀并加上***")
	}
	return 访客查卡条件{方式: 访客查卡精确, 卡密: keyword}, nil
}

func (条件 访客查卡条件) 套用(query *gorm.DB) *gorm.DB {
	switch 条件.方式 {
	case 访客查卡多张精确:
		return query.Where("card IN ?", 条件.卡密列表)
	case 访客查卡前缀:
		return query.Where("card LIKE ?", 转义Like文本(条件.前缀)+"___")
	default:
		return query.Where("card = ?", 条件.卡密)
	}
}

func 执行访客查卡查询(tableName, selectFields string, 条件 访客查卡条件, page, pageSize int, rows interface{}) (int64, int64, error) {
	var total int64
	if 条件.方式 == 访客查卡前缀 && page == 1 {
		query := 条件.套用(db.Table(tableName))
		if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
			return 0, 0, err
		}
	}

	query := 条件.套用(db.Table(tableName).Select(selectFields))
	switch 条件.方式 {
	case 访客查卡多张精确:
		query = query.Order("card ASC")
	case 访客查卡前缀:
		query = query.Order("card ASC").Limit(pageSize).Offset((page - 1) * pageSize)
	default:
		query = query.Limit(1)
	}
	result := query.Find(rows)
	return total, result.RowsAffected, result.Error
}

func 访客查卡响应(data interface{}, 条件 访客查卡条件, page, pageSize int, total, matched int64) gin.H {
	response := gin.H{"data": data}
	switch 条件.方式 {
	case 访客查卡多张精确:
		response["num"], response["page"], response["page_size"] = matched, 1, 访客查卡默认每页
	case 访客查卡精确:
		response["num"], response["page"], response["page_size"] = matched, 1, pageSize
	default:
		response["page"], response["page_size"] = page, pageSize
		if page == 1 {
			response["num"] = total
		}
	}
	return response
}

// 同一路径兼容旧点卡查询，并用 mode 明确选择点卡或时长卡；前缀查询必须显式
// 以三个星号表示末尾三位。逗号批量输入只做完整卡密精确查询。
func 访客_查找卡密(ctx *gin.Context) {
	admin, ok := 访客管理员(ctx)
	if !ok {
		失败提示访客(ctx, "访客上下文错误")
		return
	}
	mode := strings.TrimSpace(input(ctx, "mode"))
	if mode == "" {
		mode = "point"
	}
	if mode != "point" && mode != "duration" {
		失败提示访客(ctx, "卡密模式不正确")
		return
	}
	条件, err := 解析访客查卡条件(input(ctx, "card"))
	if err != nil {
		失败提示访客(ctx, err.Error())
		return
	}
	if !允许访客查找卡密(admin, ctx.ClientIP()) {
		失败提示访客(ctx, "查询太频繁，请稍后再试")
		return
	}
	var tableName string
	if mode == "point" {
		tableName, err = 点卡数据表名(admin)
	} else {
		tableName, err = 时长卡数据表名(admin)
	}
	if err != nil {
		失败提示访客(ctx, err.Error())
		return
	}
	page, pageSize := 读取通用分页参数(ctx)
	if input(ctx, "page_size") == "" && input(ctx, "每页") == "" {
		pageSize = 访客查卡默认每页
	}
	if pageSize > 访客查卡每页上限 {
		pageSize = 访客查卡每页上限
	}
	if mode == "point" {
		rows := make([]点卡访客项, 0)
		total, matched, err := 执行访客查卡查询(tableName, "card, create_time, use_time, software, card_state, point_balance", 条件, page, pageSize, &rows)
		if err != nil {
			失败提示访客(ctx, "查询卡密失败")
			return
		}
		成功提示访客(ctx, 访客查卡响应(rows, 条件, page, pageSize, total, matched))
		return
	}
	rows := make([]时长卡表样式, 0)
	total, matched, err := 执行访客查卡查询(tableName, "card, software, card_state, duration_minutes, use_time, end_time, paused_remaining_minutes, needle, last_heartbeat_at", 条件, page, pageSize, &rows)
	if err != nil {
		失败提示访客(ctx, "查询卡密失败")
		return
	}
	result := make([]gin.H, 0, len(rows))
	now := time.Now()
	for _, row := range rows {
		result = append(result, 访客时长卡摘要(admin, row, now))
	}
	成功提示访客(ctx, 访客查卡响应(result, 条件, page, pageSize, total, matched))
}

func 访客时长卡摘要(admin string, row 时长卡表样式, now time.Time) gin.H {
	detail := 构建时长卡详情(admin, row, now)
	return gin.H{"card": detail["card"], "software": detail["software"], "card_state": detail["card_state"], "duration_minutes": detail["duration_minutes"], "use_time": detail["use_time"], "end_time": detail["end_time"], "paused_remaining_minutes": detail["paused_remaining_minutes"], "status": detail["status"], "online": detail["online"]}
}

type 点卡访客项 struct {
	Card         string     `json:"card"`
	CreateTime   time.Time  `json:"create_time"`
	UseTime      *time.Time `json:"use_time"`
	Software     int        `json:"software"`
	CardState    int        `json:"card_state"`
	PointBalance int64      `json:"point_balance"`
}

// 访客_查询点卡卡密详情是点卡公开查询的实现；旧中文路径继续调用同一逻辑。
func 访客_查询点卡卡密详情(ctx *gin.Context) {
	admin, card, row, err := 访客读取点卡卡密(ctx)
	if err != nil {
		失败提示访客(ctx, err.Error())
		return
	}
	设备页, 设备每页 := 读取点卡设备分页参数(ctx)
	设备统计, err := 查询点卡设备统计(admin, card, row.Software, time.Now(), 设备页, 设备每页)
	if err != nil {
		失败提示访客(ctx, err.Error())
		return
	}
	status := "正常"
	if row.Card_state == 卡密状态_冻结 {
		status = "冻结"
	}
	text := fmt.Sprintf("卡密:%s\n软件:%d\n点数余额:%d\n授权设备:%d\n在线设备:%d\n状态:%s", row.Card, row.Software, row.Point_balance, 设备统计.AuthorizedCount, 设备统计.OnlineCount, status)
	成功提示访客(ctx, gin.H{"data": text, "card": row.Card, "software": row.Software, "point_balance": row.Point_balance, "card_state": row.Card_state, "authorized_device_count": 设备统计.AuthorizedCount, "online_device_count": 设备统计.OnlineCount, "device_total": 设备统计.DeviceTotal, "device_page": 设备统计.DevicePage, "device_page_size": 设备统计.DevicePageSize, "devices": 设备统计.Devices})
}

func 访客_查询点卡流水(ctx *gin.Context) {
	admin, card, row, err := 访客读取点卡卡密(ctx)
	if err != nil {
		失败提示访客(ctx, err.Error())
		return
	}
	softwareID, _ := strconv.Atoi(input(ctx, "software"))
	if softwareID > 0 && softwareID != row.Software {
		失败提示访客(ctx, "软件与卡密不匹配")
		return
	}
	page, _ := strconv.Atoi(input(ctx, "page"))
	pageSize, _ := strconv.Atoi(input(ctx, "page_size"))
	page, pageSize = 规范化流水分页(page, pageSize)
	rows, total, err := 查询点卡流水记录(db_point_ledger.Where("admin = ? AND card = ?", admin, card), page, pageSize)
	if err != nil {
		失败提示访客(ctx, "查询点数流水失败")
		return
	}
	成功提示访客(ctx, gin.H{"data": 点卡流水展示列表(rows, false), "num": total, "page": page, "page_size": pageSize, "balance": row.Point_balance})
}

// 访客查询时长卡只按完整卡密精确读取独立时长卡表，不会回退到点卡表。
// 访客页需要展示到期时间和激活状态，因此复用同一套状态推断逻辑。
func 访客_查询时长卡(ctx *gin.Context) {
	admin, ok := 访客管理员(ctx)
	if !ok {
		失败提示访客(ctx, "访客上下文错误")
		return
	}
	card := strings.ToLower(strings.TrimSpace(input(ctx, "card")))
	if !卡密格式规则.MatchString(card) {
		失败提示访客(ctx, "请输入完整且格式正确的卡密")
		return
	}
	data, err := 时长卡详情(admin, card, time.Now())
	if err != nil {
		失败提示访客(ctx, err.Error())
		return
	}
	// 访客查询是公开页面，只返回状态所需字段，不返回服务端 needle。
	delete(data, "needle")
	delete(data, "last_heartbeat_at")
	delete(data, "notes")
	delete(data, "config_content")
	delete(data, "agent_id")
	成功提示访客(ctx, gin.H{"data": data})
}

func 失败提示访客(ctx *gin.Context, message string) {
	ctx.JSON(http.StatusOK, gin.H{"state": false, "code": 0, "msg": message})
}

func 成功提示访客(ctx *gin.Context, data interface{}) {
	if object, ok := data.(gin.H); ok {
		object["state"] = true
		object["code"] = 1
		ctx.JSON(http.StatusOK, object)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"state": true, "code": 1, "data": data})
}
