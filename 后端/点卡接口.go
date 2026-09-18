package main

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 点卡计费方案请求是管理员配置软件授权时长时使用的输入模型。
type 点卡周期价格请求 struct {
	Software      int   `json:"software"`
	PeriodMinutes int64 `json:"period_minutes"`
	Cost          int64 `json:"cost"`
	IsDefault     *bool `json:"is_default"`
	Enabled       *bool `json:"enabled"`
}

// 点卡周期价格写入值显式组装新增记录的所有业务字段。点卡周期价格模型的
// Enabled 使用 default:true 兼容旧表默认值；如果直接用结构体 Create，GORM
// 会把零值 false 替换成模型默认值 true，导致“停用”方案实际被写成启用。
// 使用 map 将 false 作为明确的数据库值写入，避免把业务输入交给 GORM 的零值默认逻辑。
func 点卡周期价格写入值(price 点卡周期价格) map[string]interface{} {
	return map[string]interface{}{
		"admin":          price.Admin,
		"software":       price.Software,
		"period_seconds": price.PeriodSeconds,
		"cost":           price.Cost,
		"is_default":     price.IsDefault,
		"enabled":        price.Enabled,
		"created_at":     price.CreatedAt,
		"updated_at":     price.UpdatedAt,
	}
}

// 创建点卡周期价格只用于新增方案。map Create 会完整保留 Enabled=false，
// 随后按唯一业务键重新读取记录，使自增 ID、数据库时间和响应数据保持完整。
func 创建点卡周期价格(tx *gorm.DB, price *点卡周期价格) error {
	now := time.Now()
	if price.CreatedAt.IsZero() {
		price.CreatedAt = now
	}
	if price.UpdatedAt.IsZero() {
		price.UpdatedAt = price.CreatedAt
	}
	if err := tx.Model(&点卡周期价格{}).Table("point_period_price").Create(点卡周期价格写入值(*price)).Error; err != nil {
		return err
	}
	return tx.Table("point_period_price").Where(
		"admin = ? AND software = ? AND period_seconds = ?",
		price.Admin, price.Software, price.PeriodSeconds,
	).First(price).Error
}

func 管理员名称和点卡软件(ctx *gin.Context, softwareID int) (string, error) {
	account, ok := 管理员_取账号信息(ctx)
	if !ok || account.Name == "" {
		return "", fmt.Errorf("登录状态错误")
	}
	if softwareID <= 0 || !user_soft_验证(account.Name, softwareID) {
		return "", fmt.Errorf("软件不存在")
	}
	return account.Name, nil
}

// 管理员_保存点卡周期价格新增或更新一个软件授权时长方案。
// 设置为默认方案时，会在同一事务内清除该软件其他方案的默认标记。
func 管理员_保存点卡周期价格(ctx *gin.Context) {
	var request 点卡周期价格请求
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil {
		失败提示管理端(ctx, "数据错误")
		return
	}
	admin, err := 管理员名称和点卡软件(ctx, request.Software)
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	if !点卡授权时长分钟有效(request.PeriodMinutes, false) {
		失败提示管理端(ctx, fmt.Sprintf("授权时长必须在%d至%d分钟之间", 最小点卡计费周期分钟, 最大点卡计费周期分钟))
		return
	}
	periodSeconds := 分钟转秒(request.PeriodMinutes)
	if request.Cost <= 0 || request.Cost > 最大单次点数 {
		失败提示管理端(ctx, fmt.Sprintf("扣点价格必须在1至%d之间", 最大单次点数))
		return
	}
	enabled := true
	if request.Enabled != nil {
		enabled = *request.Enabled
	}
	isDefault := false
	if request.IsDefault != nil {
		isDefault = *request.IsDefault
	}
	var saved 点卡周期价格
	err = db.Transaction(func(tx *gorm.DB) error {
		var settings 软件
		if query := tx.Table("software").Clauses(clause.Locking{Strength: "UPDATE"}).Where("name = ? AND id = ?", admin, request.Software).First(&settings); query.Error != nil {
			return fmt.Errorf("软件不存在")
		}
		if isDefault && !enabled {
			return fmt.Errorf("默认方案必须保持启用")
		}
		// 软件可能尚未配置任何点卡计费方案。此时即使客户端没有显式勾选“默认”，
		// 也把本次保存的启用方案作为默认，避免软件进入“有方案但默认时长
		// 永远无法扣费”的不可用状态。
		var currentDefaultCount int64
		if err := tx.Table("point_period_price").Where("admin = ? AND software = ? AND period_seconds = ? AND enabled = ?", admin, request.Software, settings.DefaultPeriodSeconds, true).Count(&currentDefaultCount).Error; err != nil {
			return fmt.Errorf("检查默认授权时长失败")
		}
		if !isDefault && periodSeconds != settings.DefaultPeriodSeconds && currentDefaultCount == 0 {
			if !enabled {
				return fmt.Errorf("请先保存并启用一个默认授权时长方案")
			}
			isDefault = true
		}
		// 当前软件默认授权时长不能被直接停用或取消默认。需要切换默认时，
		// 把另一个启用方案设为默认即可，事务会同时更新软件设置。
		if periodSeconds == settings.DefaultPeriodSeconds && !isDefault {
			if !enabled {
				return fmt.Errorf("默认方案不能停用，请先设置另一个默认方案")
			}
			isDefault = true
		}
		if isDefault {
			if err := tx.Table("point_period_price").Where("admin = ? AND software = ?", admin, request.Software).Updates(map[string]interface{}{"is_default": false}).Error; err != nil {
				return fmt.Errorf("更新默认方案失败")
			}
			if err := tx.Table("software").Where("name = ? AND id = ?", admin, request.Software).Update("default_period_seconds", periodSeconds).Error; err != nil {
				return fmt.Errorf("更新软件默认授权时长失败")
			}
		}
		query := tx.Table("point_period_price").Where("admin = ? AND software = ? AND period_seconds = ?", admin, request.Software, periodSeconds).First(&saved)
		if errors.Is(query.Error, gorm.ErrRecordNotFound) {
			saved = 点卡周期价格{Admin: admin, Software: request.Software, PeriodSeconds: periodSeconds, Cost: request.Cost, IsDefault: isDefault, Enabled: enabled}
			return 创建点卡周期价格(tx, &saved)
		}
		if query.Error != nil {
			return fmt.Errorf("读取点卡计费方案失败")
		}
		if err := tx.Table("point_period_price").Where("id = ?", saved.ID).Updates(map[string]interface{}{"cost": request.Cost, "is_default": isDefault, "enabled": enabled}).Error; err != nil {
			return fmt.Errorf("保存点卡计费方案失败")
		}
		saved.Cost, saved.IsDefault, saved.Enabled = request.Cost, isDefault, enabled
		return nil
	})
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	清除点卡计费配置缓存(admin, request.Software)
	成功提示管理端(ctx, gin.H{"msg": "保存成功", "data": gin.H{"id": saved.ID, "software": saved.Software, "period_minutes": 秒转分钟(saved.PeriodSeconds), "cost": saved.Cost, "is_default": saved.IsDefault, "enabled": saved.Enabled}})
}

// 管理员_查询点卡周期价格只返回当前管理员自己的点卡计费方案配置。
func 管理员_查询点卡周期价格(ctx *gin.Context) {
	account, ok := 管理员_取账号信息(ctx)
	if !ok {
		失败提示管理端(ctx, "登录状态错误")
		return
	}
	softwareID, _ := strconv.Atoi(input(ctx, "software"))
	query := db_point_period_price.Where("admin = ?", account.Name)
	if softwareID > 0 {
		query = query.Where("software = ?", softwareID)
	}
	var rows []点卡周期价格
	if err := query.Order("software ASC, period_seconds ASC").Find(&rows).Error; err != nil {
		失败提示管理端(ctx, "查询点卡计费方案失败")
		return
	}
	data := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		data = append(data, gin.H{"id": row.ID, "admin": row.Admin, "software": row.Software, "period_minutes": 秒转分钟(row.PeriodSeconds), "cost": row.Cost, "is_default": row.IsDefault, "enabled": row.Enabled, "created_at": row.CreatedAt, "updated_at": row.UpdatedAt})
	}
	成功提示管理端(ctx, gin.H{"data": data})
}

// 点卡公开计费方案只返回客户端选择授权时长所需的字段。管理员名称、数据库
// 主键和停用记录不会暴露；客户端可先调用此接口再决定本次登录提交哪个授权时长。
type 点卡公开周期价格 struct {
	PeriodMinutes int64 `json:"period_minutes"`
	Cost          int64 `json:"cost"`
	IsDefault     bool  `json:"is_default"`
}

// 点卡端_查询周期价格使用点卡所属软件作为默认软件，也接受客户端显式传入
// software 做一致性校验。它是只读接口，不改变余额和设备会话。
func 点卡端_查询周期价格(ctx *gin.Context) {
	value, ok := ctx.Get("card")
	cardContext, valid := value.(卡密请求上下文)
	if !ok || !valid {
		失败提示(ctx, "卡密上下文错误")
		return
	}
	tableName, err := 点卡数据表名(cardContext.Name)
	if err != nil {
		失败提示(ctx, err.Error())
		return
	}
	var card 点卡表样式
	query := db.Table(tableName).Where("card = ?", cardContext.Card).First(&card)
	if errors.Is(query.Error, gorm.ErrRecordNotFound) {
		失败提示(ctx, "卡密不存在")
		return
	}
	if query.Error != nil {
		失败提示(ctx, "读取卡密失败")
		return
	}
	softwareText := strings.TrimSpace(input(ctx, "software"))
	softwareID := card.Software
	if softwareText != "" {
		var parseErr error
		softwareID, parseErr = strconv.Atoi(softwareText)
		if parseErr != nil {
			失败提示(ctx, "software参数错误")
			return
		}
	}
	if softwareID <= 0 || softwareID != card.Software {
		失败提示(ctx, "software与卡密不匹配")
		return
	}
	var settings 软件
	if err := db_software.Where("name = ? AND id = ?", cardContext.Name, softwareID).First(&settings).Error; err != nil {
		失败提示(ctx, "软件不存在")
		return
	}
	var prices []点卡周期价格
	if err := db_point_period_price.Where("admin = ? AND software = ? AND enabled = ?", cardContext.Name, softwareID, true).Order("period_seconds ASC").Find(&prices).Error; err != nil {
		失败提示(ctx, "查询点卡计费方案失败")
		return
	}
	result := make([]点卡公开周期价格, 0, len(prices))
	for _, price := range prices {
		result = append(result, 点卡公开周期价格{PeriodMinutes: 秒转分钟(price.PeriodSeconds), Cost: price.Cost, IsDefault: price.PeriodSeconds == settings.DefaultPeriodSeconds})
	}
	成功提示(ctx, gin.H{"data": result, "software": softwareID, "default_period_minutes": 秒转分钟(settings.DefaultPeriodSeconds), "heartbeat_interval_seconds": settings.HeartbeatIntervalSeconds, "online_grace_minutes": settings.OnlineGraceMinutes})
}

// 管理员_删除点卡周期价格按管理员条件删除，避免拿到其他租户 ID 后越权。
func 管理员_删除点卡周期价格(ctx *gin.Context) {
	account, ok := 管理员_取账号信息(ctx)
	if !ok {
		失败提示管理端(ctx, "登录状态错误")
		return
	}
	id, _ := strconv.ParseUint(input(ctx, "id"), 10, 64)
	if id == 0 {
		失败提示管理端(ctx, "点卡计费方案编号不正确")
		return
	}
	// 先读取所属软件编号，用于在事务中按“软件行 -> 价格行”的顺序加锁；
	// 该顺序与保存点卡计费方案保持一致，避免并发切换默认方案时互相等待。
	var hint 点卡周期价格
	if err := db_point_period_price.Select("id", "software").Where("id = ? AND admin = ?", id, account.Name).First(&hint).Error; err != nil {
		失败提示管理端(ctx, "点卡计费方案不存在")
		return
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		var settings 软件
		if result := tx.Table("software").Clauses(clause.Locking{Strength: "UPDATE"}).Where("name = ? AND id = ?", account.Name, hint.Software).First(&settings); result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				return fmt.Errorf("所属软件不存在")
			}
			return fmt.Errorf("读取所属软件失败")
		}
		var price 点卡周期价格
		if result := tx.Table("point_period_price").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND admin = ? AND software = ?", id, account.Name, hint.Software).First(&price); result.Error != nil {
			return fmt.Errorf("点卡计费方案不存在")
		}
		if settings.DefaultPeriodSeconds == price.PeriodSeconds {
			return fmt.Errorf("默认方案不能删除，请先设置另一个默认方案")
		}
		if result := tx.Table("point_period_price").Where("id = ? AND admin = ?", id, account.Name).Delete(&点卡周期价格{}); result.Error != nil || result.RowsAffected != 1 {
			return fmt.Errorf("删除点卡计费方案失败")
		}
		return nil
	})
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	清除点卡计费配置缓存(account.Name, hint.Software)
	成功提示管理端(ctx, gin.H{"msg": "删除成功"})
}

// 管理员_调整点卡余额调用服务层完成加锁、余额更新和流水落库。
func 管理员_调整点卡余额(ctx *gin.Context) {
	var request struct {
		Card   string `json:"card"`
		Amount int64  `json:"amount"`
		Reason string `json:"reason"`
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
	balance, err := 调整点卡余额(account.Name, request.Card, request.Amount, request.Reason, ctx.ClientIP())
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	成功提示管理端(ctx, gin.H{"msg": "调整成功", "balance": balance})
}

func 管理员_下线点卡设备(ctx *gin.Context) {
	account, ok := 管理员_取账号信息(ctx)
	if !ok || account.Name == "" {
		失败提示管理端(ctx, "登录状态错误")
		return
	}
	处理点卡设备下线(ctx, account.Name, 0)
}

func 代理账号_下线点卡设备(ctx *gin.Context) {
	account := 代理账号_取账号信息(ctx)
	if account.ID <= 0 || account.Admin == "" {
		失败提示管理端(ctx, "登录状态错误")
		return
	}
	处理点卡设备下线(ctx, account.Admin, account.ID)
}

func 处理点卡设备下线(ctx *gin.Context, admin string, agentID int) {
	var request struct {
		Card     string `json:"card"`
		DeviceID string `json:"device_id"`
		Needle   string `json:"needle"`
	}
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil {
		失败提示管理端(ctx, "数据错误")
		return
	}
	if err := 下线点卡设备(admin, agentID, request.Card, request.DeviceID, request.Needle); err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	成功提示管理端(ctx, gin.H{"msg": "设备已下线，未到期授权仍保留"})
}

type 点数流水展示 struct {
	ID            uint64 `json:"id"`
	Admin         string `json:"admin,omitempty"`
	Card          string `json:"card"`
	Software      int    `json:"software"`
	EventType     string `json:"event_type"`
	Change        int64  `json:"change"`
	BalanceBefore int64  `json:"balance_before"`
	BalanceAfter  int64  `json:"balance_after"`
	Remark        string `json:"remark"`
	CreatedAt     string `json:"created_at"`
}

func 点卡流水展示列表(rows []点数流水, includeAdmin bool) []点数流水展示 {
	result := make([]点数流水展示, 0, len(rows))
	for _, row := range rows {
		item := 点数流水展示{ID: row.ID, Card: row.Card, Software: row.Software, EventType: row.EventType, Change: row.Change, BalanceBefore: row.BalanceBefore, BalanceAfter: row.BalanceAfter, Remark: row.Remark, CreatedAt: row.CreatedAt.Format(timeLayout)}
		if includeAdmin {
			item.Admin = row.Admin
		}
		result = append(result, item)
	}
	return result
}

const timeLayout = "2006-01-02 15:04:05"

// 规范化流水分页确保实际查询和响应中的页码完全一致，同时限制极端页码
// 造成的整数溢出或无意义超大 OFFSET。
func 规范化流水分页(page int, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	} else if page > 1000000 {
		page = 1000000
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}
	return page, pageSize
}

// 读取流水的通用分页逻辑。调用方已经限制了管理员或当前卡密范围。
func 查询点卡流水记录(query *gorm.DB, page int, pageSize int) ([]点数流水, int64, error) {
	page, pageSize = 规范化流水分页(page, pageSize)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []点数流水
	if err := query.Order("id DESC").Limit(pageSize).Offset((page - 1) * pageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// 管理员_查询点卡流水仅提供余额变化所需字段，设备信息从 remark 读取。
func 管理员_查询点卡流水(ctx *gin.Context) {
	account, ok := 管理员_取账号信息(ctx)
	if !ok {
		失败提示管理端(ctx, "登录状态错误")
		return
	}
	softwareID, _ := strconv.Atoi(input(ctx, "software"))
	page, _ := strconv.Atoi(input(ctx, "page"))
	pageSize, _ := strconv.Atoi(input(ctx, "page_size"))
	page, pageSize = 规范化流水分页(page, pageSize)
	card := strings.ToLower(strings.TrimSpace(input(ctx, "card")))
	if len([]rune(card)) > 63 {
		失败提示管理端(ctx, "卡密筛选条件过长")
		return
	}
	query := db_point_ledger.Where("admin = ?", account.Name)
	if softwareID > 0 {
		query = query.Where("software = ?", softwareID)
	}
	if card != "" {
		query = query.Where("card LIKE ?", "%"+转义Like文本(card)+"%")
	}
	if eventType := strings.TrimSpace(input(ctx, "event_type")); eventType != "" {
		if eventType != 点数事件_扣点 && eventType != 点数事件_补点 {
			失败提示管理端(ctx, "流水类型不正确")
			return
		}
		query = query.Where("event_type = ?", eventType)
	}
	rows, total, err := 查询点卡流水记录(query, page, pageSize)
	if err != nil {
		失败提示管理端(ctx, "查询点数流水失败")
		return
	}
	成功提示管理端(ctx, gin.H{"data": 点卡流水展示列表(rows, true), "num": total, "page": page, "page_size": pageSize})
}

func 转义Like文本(value string) string {
	return strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(value)
}

// 失败提示管理端和成功提示管理端不添加卡密接口签名，专供后台 JSON API 使用。
func 失败提示管理端(ctx *gin.Context, message string) {
	ctx.JSON(http.StatusOK, gin.H{"state": false, "code": 0, "msg": message})
}

func 成功提示管理端(ctx *gin.Context, data interface{}) {
	if object, ok := data.(gin.H); ok {
		object["state"] = true
		object["code"] = 1
		ctx.JSON(http.StatusOK, object)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"state": true, "code": 1, "data": data})
}
