package main

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	时长充值卡表名     = "duration_recharge_card"
	时长充值卡最大使用次数 = 1000
)

// 时长充值卡与普通时长卡分表保存。它只代表“可给已激活时长卡增加几次
// 固定分钟数”的凭证，本身不能登录，也不会出现在点卡余额或点数流水中。
type 时长充值卡 struct {
	ID              uint       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Admin           string     `gorm:"column:admin;size:32;not null;uniqueIndex:uk_duration_recharge_card,priority:1;index" json:"admin"`
	Card            string     `gorm:"column:card;size:63;not null;uniqueIndex:uk_duration_recharge_card,priority:2" json:"card"`
	CreateTime      time.Time  `gorm:"column:create_time;not null;index" json:"create_time"`
	Software        int        `gorm:"column:software;not null;index" json:"software"`
	DurationMinutes int64      `gorm:"column:duration_minutes;not null" json:"duration_minutes"`
	InitialUses     int        `gorm:"column:initial_uses;not null" json:"initial_uses"`
	RemainingUses   int        `gorm:"column:remaining_uses;not null" json:"remaining_uses"`
	ExpiresAt       *time.Time `gorm:"column:expires_at;index" json:"expires_at"`
	CardState       int        `gorm:"column:card_state;not null;default:2;index" json:"card_state"`
	Record          string     `gorm:"column:record;type:longtext" json:"record"`
	Notes           string     `gorm:"column:notes;size:500" json:"notes"`
	AgentID         int        `gorm:"column:agent_id;not null;default:0;index" json:"agent_id"`
}

func (时长充值卡) TableName() string {
	return 时长充值卡表名
}

type 时长充值卡生成请求 struct {
	Software        int        `json:"software"`
	DurationMinutes int64      `json:"duration_minutes"`
	Uses            int        `json:"uses"`
	Num             int        `json:"num"`
	Cards           string     `json:"cards"`
	Random          bool       `json:"random"`
	ExpiresAt       *time.Time `json:"expires_at"`
	Notes           string     `json:"notes"`
	// 以下字段仅兼容旧管理页面；新页面统一使用分钟、uses 和 random。
	LegacyAddDays   float64    `json:"add_time"`
	LegacyUses      int        `json:"充值次数"`
	LegacyRandom    int        `json:"指定类型"`
	LegacyExpiresAt *time.Time `json:"有效期至"`
}

type 生成时长充值卡结果 struct {
	Cards   []string
	Charge  int64
	Balance int64
}

// 规范化时长充值卡生成请求同时兼容旧页面的天数和中文字段。转换只发生在
// 接口边界，数据库内部始终保存整数分钟和整数次数。
func 规范化时长充值卡生成请求(admin string, request 时长充值卡生成请求) (时长充值卡生成请求, error) {
	admin = strings.TrimSpace(admin)
	if !验证管理员名称(admin) || request.Software <= 0 {
		return request, fmt.Errorf("管理员或软件参数不正确")
	}
	if request.DurationMinutes == 0 && request.LegacyAddDays > 0 {
		minutes := request.LegacyAddDays * 1440
		if math.Trunc(minutes) != minutes || minutes > float64(math.MaxInt64) {
			return request, fmt.Errorf("旧版充值天数无法精确换算为分钟")
		}
		request.DurationMinutes = int64(minutes)
	}
	if request.Uses == 0 {
		request.Uses = request.LegacyUses
	}
	if request.LegacyRandom == 1 {
		request.Random = true
	}
	if request.ExpiresAt == nil {
		request.ExpiresAt = request.LegacyExpiresAt
	}
	if request.DurationMinutes < 时长卡最小时长分钟 || request.DurationMinutes > 时长卡永久分钟 {
		return request, fmt.Errorf("单次充值时长必须在%d分钟至%d天之间", 时长卡最小时长分钟, 时长卡永久分钟/1440)
	}
	if request.Uses <= 0 || request.Uses > 时长充值卡最大使用次数 {
		return request, fmt.Errorf("可用次数必须在1至%d之间", 时长充值卡最大使用次数)
	}
	if request.Num <= 0 || request.Num > 时长卡最大单次数量 {
		return request, fmt.Errorf("生成数量必须在1至%d之间", 时长卡最大单次数量)
	}
	if int64(request.Num) > int64(时长卡代理计价最大数量)/int64(request.Uses) {
		return request, fmt.Errorf("本次生成的总可用次数过多")
	}
	var valid bool
	request.Notes, valid = 规范化可显示文本(request.Notes, 时长卡最大备注字符数)
	if !valid {
		return request, fmt.Errorf("充值卡备注不能包含控制字符且不能超过%d个字符", 时长卡最大备注字符数)
	}
	if request.ExpiresAt != nil && !request.ExpiresAt.After(time.Now()) {
		return request, fmt.Errorf("有效期必须晚于当前时间")
	}
	return request, nil
}

// 准备生成时长充值卡在事务外完成软件校验、随机生成和查重，缩短代理
// 余额事务持有时间；并发重复最终仍由管理员+卡密唯一索引兜底。
func 准备生成时长充值卡(admin string, request 时长充值卡生成请求) ([]string, 时长充值卡生成请求, error) {
	request, err := 规范化时长充值卡生成请求(admin, request)
	if err != nil {
		return nil, request, err
	}
	if err := 检查时长卡软件(db, admin, request.Software); err != nil {
		return nil, request, err
	}
	cards, err := 解析时长卡生成卡密(request.Cards, request.Random, request.Software, request.Num)
	if err != nil {
		return nil, request, err
	}
	if len(cards) != request.Num {
		return nil, request, fmt.Errorf("指定卡密数量与生成数量不一致")
	}
	var existing int64
	if err := db.Table(时长充值卡表名).Where("admin = ? AND card IN ?", admin, cards).Count(&existing).Error; err != nil {
		return nil, request, fmt.Errorf("检查充值卡是否重复失败")
	}
	if existing > 0 {
		return nil, request, fmt.Errorf("充值卡已存在，不能重复生成")
	}
	return cards, request, nil
}

// 保存时长充值卡批次只写入已完成校验的记录，调用方决定是否同时扣除
// 代理余额。整批 INSERT 失败时由外层事务统一回滚。
func 保存时长充值卡批次(tx *gorm.DB, admin string, agentID int, request 时长充值卡生成请求, cards []string, now time.Time) error {
	rows := make([]时长充值卡, 0, len(cards))
	for _, card := range cards {
		rows = append(rows, 时长充值卡{Admin: admin, Card: card, CreateTime: now, Software: request.Software,
			DurationMinutes: request.DurationMinutes, InitialUses: request.Uses, RemainingUses: request.Uses,
			ExpiresAt: request.ExpiresAt, CardState: 卡密状态_正常, Notes: request.Notes, AgentID: agentID})
	}
	if err := tx.Table(时长充值卡表名).CreateInBatches(&rows, 200).Error; err != nil {
		return fmt.Errorf("写入时长充值卡失败")
	}
	return nil
}

// 管理员生成充值卡不涉及余额扣款，但软件存在性复查与整批写入仍放在
// 一个短事务中，避免软件删除和发卡并发时留下孤立记录。
func 管理员_添加时长充值卡(ctx *gin.Context) {
	var request 时长充值卡生成请求
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil {
		失败提示管理端(ctx, "数据错误")
		return
	}
	admin := 管理员_用户名(ctx)
	cards, normalized, err := 准备生成时长充值卡(admin, request)
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := 锁定发卡软件(tx, admin, normalized.Software); err != nil {
			return err
		}
		return 保存时长充值卡批次(tx, admin, 0, normalized, cards, time.Now())
	})
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	成功提示管理端(ctx, gin.H{"msg": fmt.Sprintf("成功生成%d张时长充值卡", len(cards)), "data": strings.Join(cards, "\n")})
}

// 代理生成充值卡按“每张卡的充值分钟数 × 可用次数”计价。余额扣减、
// 价格复核和充值卡写入处于同一事务，任一步失败都不会产生半成功状态。
func 代理账号_添加时长充值卡(ctx *gin.Context) {
	var request 时长充值卡生成请求
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil {
		失败提示管理端(ctx, "数据错误")
		return
	}
	account := 代理账号_取账号信息(ctx)
	cards, normalized, err := 准备生成时长充值卡(account.Admin, request)
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	result := 生成时长充值卡结果{}
	err = db.Transaction(func(tx *gorm.DB) error {
		current, err := 锁定时长卡代理账号(tx, account.Admin, account.ID)
		if err != nil {
			return err
		}
		if err := 锁定发卡软件(tx, current.Admin, normalized.Software); err != nil {
			return err
		}
		prices, err := 读取时长卡代理价格(tx, current.Admin, current.ID, normalized.Software, true, true)
		if err != nil {
			return err
		}
		totalUses := normalized.Num * normalized.Uses
		quote, err := 计算时长卡代理费用(prices, normalized.DurationMinutes, totalUses)
		if err != nil {
			return err
		}
		if current.Balance < quote.Charge {
			return fmt.Errorf("代理余额不足，需要%d点，当前%d点", quote.Charge, current.Balance)
		}
		if update := tx.Table(代理账号表名).Where("id = ? AND admin = ? AND balance >= ?", current.ID, current.Admin, quote.Charge).
			UpdateColumn("balance", gorm.Expr("balance - ?", quote.Charge)); update.Error != nil || update.RowsAffected != 1 {
			return fmt.Errorf("扣除代理余额失败")
		}
		if err := 保存时长充值卡批次(tx, current.Admin, current.ID, normalized, cards, time.Now()); err != nil {
			return err
		}
		result = 生成时长充值卡结果{Cards: cards, Charge: quote.Charge, Balance: current.Balance - quote.Charge}
		return nil
	})
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	代理账号日志(account.ID, fmt.Sprintf("余额:%d", result.Balance), fmt.Sprintf("变更:-%d", result.Charge), "原因:生成时长充值卡", fmt.Sprintf("软件:%d", normalized.Software), fmt.Sprintf("时长:%d分钟", normalized.DurationMinutes), fmt.Sprintf("每张次数:%d", normalized.Uses), fmt.Sprintf("数量:%d", len(result.Cards)))
	成功提示管理端(ctx, gin.H{"msg": fmt.Sprintf("成功生成%d张时长充值卡", len(result.Cards)), "data": strings.Join(result.Cards, "\n"), "charge": result.Charge, "balance": result.Balance})
}

// 查询时长充值卡列表由管理员和代理共用。代理范围在 SQL 中固定，记录文本
// 只在单卡详情返回，列表不会因充值历史增长而变成超大响应。
func 查询时长充值卡列表(ctx *gin.Context, admin string, agentID int) {
	softwareID, _ := strconv.Atoi(input(ctx, "software"))
	state, _ := strconv.Atoi(input(ctx, "card_state"))
	keyword := strings.ToLower(strings.TrimSpace(input(ctx, "card")))
	if len([]rune(keyword)) > 63 {
		失败提示管理端(ctx, "筛选条件过长")
		return
	}
	query := db.Table(时长充值卡表名).Where("admin = ?", admin)
	if agentID > 0 {
		query = query.Where("agent_id = ?", agentID)
	}
	if softwareID > 0 {
		query = query.Where("software = ?", softwareID)
	}
	if state == 卡密状态_正常 || state == 卡密状态_冻结 {
		query = query.Where("card_state = ?", state)
	}
	if keyword != "" {
		query = query.Where("card LIKE ?", "%"+转义Like文本(keyword)+"%")
	}
	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		失败提示管理端(ctx, "查询充值卡数量失败")
		return
	}
	page, pageSize := 读取通用分页参数(ctx)
	var rows []时长充值卡
	if err := query.Omit("record").Order("create_time DESC, card ASC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		失败提示管理端(ctx, "查询时长充值卡失败")
		return
	}
	成功提示管理端(ctx, gin.H{"data": rows, "num": total, "page": page, "page_size": pageSize})
}

func 管理员_查询时长充值卡列表(ctx *gin.Context) {
	查询时长充值卡列表(ctx, 管理员_用户名(ctx), 0)
}

func 代理账号_查询时长充值卡列表(ctx *gin.Context) {
	account := 代理账号_取账号信息(ctx)
	查询时长充值卡列表(ctx, account.Admin, account.ID)
}

// 读取时长充值卡范围与时长卡范围规则一致：agentID 为非零时必须同时
// 命中归属条件，避免详情、修改和删除接口各自遗漏权限校验。
func 读取时长充值卡范围(tx *gorm.DB, admin string, agentID int, card string, lock bool) (时长充值卡, error) {
	var row 时长充值卡
	card = strings.ToLower(strings.TrimSpace(card))
	if tx == nil || !验证管理员名称(admin) || agentID < 0 || !卡密格式规则.MatchString(card) {
		return row, fmt.Errorf("时长充值卡参数不正确")
	}
	query := tx.Table(时长充值卡表名).Where("admin = ? AND card = ?", admin, card)
	if agentID > 0 {
		query = query.Where("agent_id = ?", agentID)
	}
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	result := query.First(&row)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return row, fmt.Errorf("时长充值卡不存在或无权操作")
	}
	if result.Error != nil {
		return row, fmt.Errorf("读取时长充值卡失败")
	}
	return row, nil
}

func 查询时长充值卡详情(ctx *gin.Context, admin string, agentID int) {
	row, err := 读取时长充值卡范围(db, admin, agentID, input(ctx, "card"), false)
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	成功提示管理端(ctx, gin.H{"data": row})
}

func 管理员_查询时长充值卡详情(ctx *gin.Context) {
	查询时长充值卡详情(ctx, 管理员_用户名(ctx), 0)
}

func 代理账号_查询时长充值卡详情(ctx *gin.Context) {
	account := 代理账号_取账号信息(ctx)
	查询时长充值卡详情(ctx, account.Admin, account.ID)
}

// 修改时长充值卡范围只开放备注和正常/冻结状态；面额、剩余次数和时长
// 属于发卡事实，不能通过普通编辑接口改写。
func 修改时长充值卡范围(admin string, agentID int, card string, notes *string, state *int) error {
	updates := make(map[string]interface{})
	if notes != nil {
		value, valid := 规范化可显示文本(*notes, 时长卡最大备注字符数)
		if !valid {
			return fmt.Errorf("备注格式不正确")
		}
		updates["notes"] = value
	}
	if state != nil {
		if *state != 卡密状态_正常 && *state != 卡密状态_冻结 {
			return fmt.Errorf("充值卡状态不正确")
		}
		updates["card_state"] = *state
	}
	if len(updates) == 0 {
		return fmt.Errorf("没有需要修改的内容")
	}
	return db.Transaction(func(tx *gorm.DB) error {
		row, err := 读取时长充值卡范围(tx, admin, agentID, card, true)
		if err != nil {
			return err
		}
		if result := tx.Table(时长充值卡表名).Where("id = ?", row.ID).Updates(updates); result.Error != nil {
			return fmt.Errorf("修改时长充值卡失败")
		}
		return nil
	})
}

func 处理时长充值卡修改(ctx *gin.Context, admin string, agentID int) {
	var request struct {
		Card      string  `json:"card"`
		Notes     *string `json:"notes"`
		CardState *int    `json:"card_state"`
	}
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil {
		失败提示管理端(ctx, "数据错误")
		return
	}
	if err := 修改时长充值卡范围(admin, agentID, request.Card, request.Notes, request.CardState); err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	成功提示管理端(ctx, gin.H{"msg": "修改成功"})
}

func 管理员_修改时长充值卡(ctx *gin.Context) {
	处理时长充值卡修改(ctx, 管理员_用户名(ctx), 0)
}

func 代理账号_修改时长充值卡(ctx *gin.Context) {
	account := 代理账号_取账号信息(ctx)
	处理时长充值卡修改(ctx, account.Admin, account.ID)
}

func 处理时长充值卡删除(ctx *gin.Context, admin string, agentID int) {
	var request struct {
		Cards []string `json:"cards"`
	}
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil || len(request.Cards) == 0 || len(request.Cards) > 1000 {
		失败提示管理端(ctx, "卡密数量不正确")
		return
	}
	success, failed := make([]string, 0, len(request.Cards)), make([]string, 0)
	for _, raw := range request.Cards {
		card := strings.ToLower(strings.TrimSpace(raw))
		if !卡密格式规则.MatchString(card) {
			failed = append(failed, raw)
			continue
		}
		query := db.Table(时长充值卡表名).Where("admin = ? AND card = ?", admin, card)
		if agentID > 0 {
			query = query.Where("agent_id = ?", agentID)
		}
		result := query.Delete(&时长充值卡{})
		if result.Error != nil || result.RowsAffected != 1 {
			failed = append(failed, raw)
			continue
		}
		success = append(success, card)
	}
	成功提示管理端(ctx, gin.H{"msg": fmt.Sprintf("成功%d张，失败%d张", len(success), len(failed)), "success": success, "failed": failed})
}

func 管理员_删除时长充值卡(ctx *gin.Context) {
	处理时长充值卡删除(ctx, 管理员_用户名(ctx), 0)
}

func 代理账号_删除时长充值卡(ctx *gin.Context) {
	account := 代理账号_取账号信息(ctx)
	处理时长充值卡删除(ctx, account.Admin, account.ID)
}

// 访客_查询时长充值卡只按完整卡密查询公开状态，不返回内部使用记录、
// 备注或代理归属。
func 访客_查询时长充值卡(ctx *gin.Context) {
	admin, ok := 访客管理员(ctx)
	if !ok {
		失败提示访客(ctx, "访客上下文错误")
		return
	}
	row, err := 读取时长充值卡范围(db, admin, 0, input(ctx, "card"), false)
	if err != nil {
		失败提示访客(ctx, err.Error())
		return
	}
	status := "正常"
	if row.CardState == 卡密状态_冻结 {
		status = "冻结"
	} else if row.RemainingUses <= 0 {
		status = "已用完"
	} else if row.ExpiresAt != nil && !row.ExpiresAt.After(time.Now()) {
		status = "已过期"
	}
	成功提示访客(ctx, gin.H{"data": gin.H{"card": row.Card, "software": row.Software,
		"duration_minutes": row.DurationMinutes, "initial_uses": row.InitialUses,
		"remaining_uses": row.RemainingUses, "expires_at": row.ExpiresAt,
		"card_state": row.CardState, "status": status}})
}

// 访客_使用时长充值卡在同一事务内锁定充值卡和目标时长卡。已激活目标延长
// end_time，暂停目标累加 paused_remaining_minutes；只有实际充值成功的目标
// 才扣减一次使用次数，任意数据库写入失败会回滚全部变化。
func 访客_使用时长充值卡(ctx *gin.Context) {
	var request struct {
		RechargeCard       string   `json:"recharge_card"`
		LegacyRechargeCard string   `json:"Rechargeable_card"`
		Cards              []string `json:"cards"`
	}
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil || len(request.Cards) == 0 || len(request.Cards) > 时长卡最大单次数量 {
		失败提示访客(ctx, "充值参数不正确")
		return
	}
	if request.RechargeCard == "" {
		request.RechargeCard = request.LegacyRechargeCard
	}
	admin, ok := 访客管理员(ctx)
	if !ok {
		失败提示访客(ctx, "访客上下文错误")
		return
	}
	rechargeCard := strings.ToLower(strings.TrimSpace(request.RechargeCard))
	validCards := make([]string, 0, len(request.Cards))
	failed := make([]string, 0)
	seen := make(map[string]struct{}, len(request.Cards))
	for _, raw := range request.Cards {
		card := strings.ToLower(strings.TrimSpace(raw))
		if !卡密格式规则.MatchString(card) {
			failed = append(failed, raw)
			continue
		}
		if _, exists := seen[card]; exists {
			failed = append(failed, raw)
			continue
		}
		seen[card] = struct{}{}
		validCards = append(validCards, card)
	}
	if !卡密格式规则.MatchString(rechargeCard) || len(validCards) == 0 {
		失败提示访客(ctx, "充值卡或目标卡密不正确")
		return
	}
	tableName, err := 时长卡数据表名(admin)
	if err != nil {
		失败提示访客(ctx, err.Error())
		return
	}
	// 充值会改变目标时长卡的剩余授权，先同步并失效其心跳快照；
	// 同步失败的目标不参与本次充值，避免缓存中的旧时间被后续查询使用。
	readyCards := make([]string, 0, len(validCards))
	for _, card := range validCards {
		if cacheErr := 同步并删除时长卡心跳缓存(admin, card, ""); cacheErr != nil {
			failed = append(failed, card)
			continue
		}
		readyCards = append(readyCards, card)
	}
	validCards = readyCards
	success := make([]string, 0, len(validCards))
	pausedSuccess := make([]string, 0)
	remaining := 0
	err = db.Transaction(func(tx *gorm.DB) error {
		recharge, err := 读取时长充值卡范围(tx, admin, 0, rechargeCard, true)
		if err != nil {
			return err
		}
		now := time.Now()
		if recharge.CardState != 卡密状态_正常 {
			return fmt.Errorf("时长充值卡被冻结")
		}
		if recharge.ExpiresAt != nil && !recharge.ExpiresAt.After(now) {
			return fmt.Errorf("时长充值卡已过期")
		}
		if recharge.RemainingUses < len(validCards) {
			return fmt.Errorf("充值卡剩余次数不足，需要%d次，当前%d次", len(validCards), recharge.RemainingUses)
		}
		sortedCards := append([]string(nil), validCards...)
		sort.Strings(sortedCards)
		var rows []时长卡表样式
		if result := tx.Table(tableName).Clauses(clause.Locking{Strength: "UPDATE"}).Where("card IN ?", sortedCards).Order("card ASC").Find(&rows); result.Error != nil {
			return fmt.Errorf("读取目标时长卡失败")
		}
		byCard := make(map[string]时长卡表样式, len(rows))
		for _, row := range rows {
			byCard[row.Card] = row
		}
		for _, card := range validCards {
			row, exists := byCard[card]
			if !exists || row.Software != recharge.Software {
				failed = append(failed, card)
				continue
			}
			active := row.CardState == 卡密状态_正常 && row.EndTime != nil
			paused := row.CardState == 时长卡状态_暂停 && row.EndTime == nil && row.PausedRemainingMinutes > 0
			if !active && !paused {
				failed = append(failed, card)
				continue
			}
			if paused {
				if recharge.DurationMinutes > math.MaxInt64-row.PausedRemainingMinutes {
					failed = append(failed, card)
					continue
				}
				newRemaining := row.PausedRemainingMinutes + recharge.DurationMinutes
				if result := tx.Table(tableName).Where("card = ?", row.Card).Update("paused_remaining_minutes", newRemaining); result.Error != nil || result.RowsAffected != 1 {
					return fmt.Errorf("保存目标时长卡失败")
				}
				pausedSuccess = append(pausedSuccess, row.Card)
			} else {
				base := *row.EndTime
				if base.Before(now) {
					base = now
				}
				end := base.Add(time.Duration(recharge.DurationMinutes) * time.Minute)
				if result := tx.Table(tableName).Where("card = ?", row.Card).Update("end_time", end); result.Error != nil || result.RowsAffected != 1 {
					return fmt.Errorf("保存目标时长卡失败")
				}
			}
			success = append(success, row.Card)
		}
		remaining = recharge.RemainingUses - len(success)
		if len(success) == 0 {
			return nil
		}
		record := recharge.Record + fmt.Sprintf("\n%s;充值%d分钟;成功:%s;暂停卡:%s;失败:%s", now.Format("2006-01-02 15:04:05"), recharge.DurationMinutes, strings.Join(success, ","), strings.Join(pausedSuccess, ","), strings.Join(failed, ","))
		result := tx.Table(时长充值卡表名).Where("id = ?", recharge.ID).Updates(map[string]interface{}{"remaining_uses": remaining, "record": record})
		if result.Error != nil || result.RowsAffected != 1 {
			return fmt.Errorf("保存充值卡余额失败")
		}
		return nil
	})
	if err != nil {
		失败提示访客(ctx, err.Error())
		return
	}
	for _, card := range success {
		if cacheErr := 同步并删除时长卡心跳缓存(admin, card, ""); cacheErr != nil {
			日志("log/启动记录.txt", "时长充值卡充值后同步心跳缓存失败:"+cacheErr.Error())
		}
	}
	成功提示访客(ctx, gin.H{"msg": fmt.Sprintf("成功%d张，失败%d张", len(success), len(failed)), "success": success, "paused": pausedSuccess, "failed": failed, "remaining_uses": remaining})
}
