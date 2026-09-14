package main

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	// 这两类错误是正常业务结果，客户端会收到完整提示；后台清理遇到它们
	// 会直接删除已过期会话，不会反复尝试扣费。
	错误_点卡余额不足  = errors.New("点卡余额不足")
	错误_周期价格不可用 = errors.New("点卡计费方案不可用")
)

const (
	点卡代扣模式_跟随总开关 = "inherit"
	点卡代扣模式_允许    = "allow"
	点卡代扣模式_禁止    = "deny"
)

// 规范化点卡代扣模式兼容空值旧数据；新写入只允许三种明确状态。
func 规范化点卡代扣模式(mode string) (string, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		return 点卡代扣模式_跟随总开关, nil
	}
	switch mode {
	case 点卡代扣模式_跟随总开关, 点卡代扣模式_允许, 点卡代扣模式_禁止:
		return mode, nil
	default:
		return "", fmt.Errorf("点卡代扣开关设置不正确")
	}
}

// 点卡扣费参数是服务层的内部参数。客户端只提交授权时长，费用始终由服务端
// 查询点卡计费方案表得到，避免客户端篡改价格。
type 点卡扣费参数 struct {
	Admin         string
	Card          string
	Software      int
	PeriodSeconds int64
	DeviceID      string
	DeviceAlias   string
	RemarkPrefix  string
	// 两个实际扣款数只用于补充代扣流水备注。
	CardCharged  int64
	AgentCharged int64
}

// 规范化点卡扣费参数集中处理所有外部输入，确保登录、心跳和后台任务
// 使用完全一致的卡密、设备和授权时长校验规则。
func 规范化点卡扣费参数(params *点卡扣费参数) error {
	params.Admin = strings.TrimSpace(params.Admin)
	params.Card = strings.ToLower(strings.TrimSpace(params.Card))
	if !验证管理员名称(params.Admin) {
		return fmt.Errorf("管理员名称格式不正确")
	}
	if !卡密格式规则.MatchString(params.Card) {
		return fmt.Errorf("卡密格式不正确")
	}
	if params.Software <= 0 {
		return fmt.Errorf("软件参数错误")
	}
	if !点卡授权时长秒有效(params.PeriodSeconds, true) {
		return fmt.Errorf("授权时长必须为0或%d至%d分钟，0表示使用默认授权时长", 最小点卡计费周期分钟, 最大点卡计费周期分钟)
	}
	if params.DeviceID != "" {
		deviceID, valid := 规范化设备标识(params.DeviceID)
		if !valid {
			return fmt.Errorf("device_id格式不正确")
		}
		params.DeviceID = deviceID
	}
	deviceAlias, valid := 规范化设备别名(params.DeviceAlias)
	if !valid {
		return fmt.Errorf("device_alias格式不正确或超过64个字符")
	}
	params.DeviceAlias = deviceAlias
	remarkPrefix, valid := 规范化可显示文本(params.RemarkPrefix, 200)
	if !valid {
		return fmt.Errorf("扣点备注格式不正确")
	}
	params.RemarkPrefix = remarkPrefix
	return nil
}

// 读取软件设置只返回计费和心跳相关配置。配置允许短期只读缓存，但不存在
// 的软件和数据库中的非法值仍直接报错，不会悄悄换用另一套默认值。
func 读取软件设置(tx *gorm.DB, admin string, softwareID int) (software, error) {
	config, err := 读取点卡计费配置(tx, admin, softwareID)
	return config.设置, err
}

// 查询可用点卡计费方案。requested=0 时使用软件表中的默认授权时长；is_default
// 只是便于管理端展示的同步标记，不作为第二套默认值来源。
// 明确提交了授权时长但该方案未启用时直接报错，不静默换成另一个方案。
func 查询点卡周期价格(tx *gorm.DB, admin string, softwareID int, requested int64) (点卡周期价格, software, error) {
	config, err := 读取点卡计费配置(tx, admin, softwareID)
	settings := config.设置
	if err != nil {
		return 点卡周期价格{}, settings, err
	}
	period := requested
	if period == 0 {
		period = settings.DefaultPeriodSeconds
	}
	if !点卡授权时长秒有效(period, false) {
		return 点卡周期价格{}, settings, fmt.Errorf("授权时长不正确")
	}
	price, exists := config.价格[period]
	if !exists || !price.Enabled {
		return 点卡周期价格{}, settings, fmt.Errorf("%w：该授权时长未配置或已停用", 错误_周期价格不可用)
	}
	if price.Cost <= 0 || price.Cost > 最大单次点数 {
		return 点卡周期价格{}, settings, fmt.Errorf("点卡计费方案配置不正确")
	}
	return price, settings, nil
}

// 校验点卡基础状态必须在卡密行锁持有期间执行，保证检查和扣点属于同一事务。
func 校验点卡状态(card 点卡表样式, softwareID int) error {
	if card.Card == "" {
		return fmt.Errorf("点卡不存在")
	}
	if card.Software != softwareID {
		return fmt.Errorf("卡密与软件不匹配")
	}
	if card.Card_state == 卡密状态_冻结 {
		return fmt.Errorf("卡密被冻结")
	}
	if card.Card_state != 卡密状态_正常 {
		return fmt.Errorf("卡密状态不正常")
	}
	return nil
}

// 生成扣点流水备注。设备信息刻意放在文本备注中，流水表仍然只承担余额审计。
func 生成扣点备注(params 点卡扣费参数, period int64, price int64) string {
	parts := make([]string, 0, 7)
	if params.RemarkPrefix != "" {
		parts = append(parts, params.RemarkPrefix)
	} else {
		parts = append(parts, "登录扣点")
	}
	parts = append(parts, fmt.Sprintf("授权时长=%d分钟", 秒转分钟(period)), fmt.Sprintf("扣点=%d", price))
	if params.AgentCharged > 0 {
		parts = append(parts, fmt.Sprintf("卡内扣点=%d", params.CardCharged), fmt.Sprintf("代理代扣=%d", params.AgentCharged))
	}
	if params.DeviceID != "" {
		parts = append(parts, "ID="+params.DeviceID)
	}
	if params.DeviceAlias != "" {
		parts = append(parts, "设备="+params.DeviceAlias)
	}
	return strings.Join(parts, "；")
}

// 保存点数流水必须和余额更新在同一事务中调用。
func 保存点数流水(tx *gorm.DB, admin string, card string, softwareID int, eventType string, change int64, before int64, after int64, remark string, now time.Time) (点数流水, error) {
	if eventType != 点数事件_扣点 && eventType != 点数事件_补点 {
		return 点数流水{}, fmt.Errorf("流水类型不正确")
	}
	if change == 0 || before < 0 || after < 0 || after != before+change || (eventType == 点数事件_扣点 && change >= 0) || (eventType == 点数事件_补点 && change <= 0) {
		return 点数流水{}, fmt.Errorf("流水余额变动不一致")
	}
	流水 := 点数流水{
		Admin: admin, Card: card, Software: softwareID, EventType: eventType,
		Change: change, BalanceBefore: before, BalanceAfter: after,
		Remark: remark, CreatedAt: now,
	}
	if err := tx.Table("point_ledger").Create(&流水).Error; err != nil {
		return 点数流水{}, fmt.Errorf("保存点数流水失败")
	}
	return 流水, nil
}

// 扣除点数事务在调用方已经锁定卡密行的前提下执行。卡内点数不足时，
// 仅对不足部分按代理当前单价扣除代理余额；卡密和代理余额在同一事务中更新。
func 扣除点数事务(tx *gorm.DB, tableName string, card *点卡表样式, params 点卡扣费参数, price 点卡周期价格, period int64, now time.Time) (点卡扣费结果, error) {
	if price.Cost <= 0 || price.Cost > 最大单次点数 {
		return 点卡扣费结果{}, fmt.Errorf("点卡计费价格不正确")
	}
	if card.Point_balance < 0 {
		return 点卡扣费结果{}, fmt.Errorf("点卡余额数据不正确")
	}
	cardCharged := card.Point_balance
	if cardCharged > price.Cost {
		cardCharged = price.Cost
	}
	agentCharged := int64(0)
	if cardCharged < price.Cost {
		shortfall := price.Cost - cardCharged
		var err error
		agentCharged, err = 点卡代理余额代扣(tx, card, params.Admin, params.Software, shortfall)
		if err != nil {
			return 点卡扣费结果{}, err
		}
	}
	after := card.Point_balance - cardCharged
	updates := map[string]interface{}{
		"point_balance": after,
		"use_time":      now,
	}
	result := tx.Table(tableName).Where("card = ? AND point_balance >= ?", card.Card, cardCharged).Updates(updates)
	// 全额由代理代扣时，卡内余额保持零；同一时间精度内 use_time 也可能
	// 没有变化。卡密行已锁定，此时零条变更不代表扣款失败。
	if result.Error != nil || (cardCharged > 0 && result.RowsAffected != 1) {
		return 点卡扣费结果{}, fmt.Errorf("扣点失败，请重试")
	}
	params.CardCharged, params.AgentCharged = cardCharged, agentCharged
	var ledgerID uint64
	if cardCharged > 0 {
		流水, err := 保存点数流水(tx, params.Admin, card.Card, card.Software, 点数事件_扣点, -cardCharged, card.Point_balance, after, 生成扣点备注(params, period, price.Cost), now)
		if err != nil {
			return 点卡扣费结果{}, err
		}
		ledgerID = 流水.ID
	}
	return 点卡扣费结果{Charged: true, Cost: price.Cost, Balance: after, AgentID: card.AgentID, AgentCharged: agentCharged, LedgerID: ledgerID}, nil
}

// 点卡代理余额代扣只处理点卡所属代理的不足部分。卡密开关由代理自己
// 控制，管理员只通过账号上的欠费权限和额度限制最终可扣范围。
func 点卡代理余额代扣(tx *gorm.DB, card *点卡表样式, admin string, softwareID int, shortfall int64) (int64, error) {
	if card.AgentID <= 0 || shortfall <= 0 {
		return 0, fmt.Errorf("%w，还差%d点", 错误_点卡余额不足, shortfall)
	}
	mode, err := 规范化点卡代扣模式(card.AgentDeductionMode)
	if err != nil {
		return 0, err
	}
	if mode == 点卡代扣模式_禁止 {
		return 0, fmt.Errorf("%w，还差%d点", 错误_点卡余额不足, shortfall)
	}
	var account 代理账号记录
	query := tx.Table(代理账号表名).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND admin = ?", card.AgentID, admin).First(&account)
	if errors.Is(query.Error, gorm.ErrRecordNotFound) {
		return 0, fmt.Errorf("%w：所属代理账号不存在", 错误_点卡余额不足)
	}
	if query.Error != nil {
		return 0, fmt.Errorf("读取代理账号失败")
	}
	if mode == 点卡代扣模式_跟随总开关 && !account.PointCardAutoDeduct {
		return 0, fmt.Errorf("%w，还差%d点", 错误_点卡余额不足, shortfall)
	}
	prices := 解析代理价格(account.Prices)
	unitPrice, exists := prices[softwareID]
	if !exists {
		return 0, fmt.Errorf("%w：代理未配置该软件的代扣价格", 错误_点卡余额不足)
	}
	charge, err := 计算代理点卡费用(unitPrice, shortfall, 1)
	if err != nil {
		return 0, fmt.Errorf("%w：代理代扣金额计算失败", 错误_点卡余额不足)
	}
	if account.Balance < math.MinInt64+charge {
		return 0, fmt.Errorf("%w：代理余额超出允许范围", 错误_点卡余额不足)
	}
	after := account.Balance - charge
	if after < 0 {
		if !account.AllowPointDebt || account.PointDebtLimit <= 0 || after < -account.PointDebtLimit {
			return 0, fmt.Errorf("%w：代理余额不足，需要%d点，当前%d点", 错误_点卡余额不足, charge, account.Balance)
		}
	}
	update := tx.Table(代理账号表名).Where("id = ? AND admin = ?", account.ID, admin).UpdateColumn("balance", gorm.Expr("balance - ?", charge))
	if update.Error != nil || update.RowsAffected != 1 {
		return 0, fmt.Errorf("扣除代理余额失败")
	}
	return charge, nil
}

// 记录点卡代扣日志只能在事务提交后调用，避免回滚扣款留下虚假记录。
// 即使卡内余额为零、没有点数流水，代理仍能在操作日志中核对每次扣款。
func 记录点卡代扣日志(charge 点卡扣费结果, session 点卡设备会话) {
	if charge.AgentCharged <= 0 {
		return
	}
	代理账号日志(charge.AgentID, fmt.Sprintf("点卡代扣;卡密:%s;软件:%d;扣除渠道余额:%d;ID=%s;设备=%s;授权截止:%s", session.Card, session.Software, charge.AgentCharged, session.DeviceID, session.DeviceAlias, session.AuthorizedUntil.Format(time.RFC3339)))
}

// 调整点卡余额供管理员补点或扣回点数。余额不能变成负数，且每次真实变更
// 都会留下可追溯的流水记录。
func 调整点卡余额(admin string, cardValue string, amount int64, reason string, clientIP string) (int64, error) {
	if amount == 0 || amount < -最大单次点数 || amount > 最大单次点数 {
		return 0, fmt.Errorf("单次调整点数必须在负%d至%d之间且不能为0", 最大单次点数, 最大单次点数)
	}
	admin = strings.TrimSpace(admin)
	cardValue = strings.ToLower(strings.TrimSpace(cardValue))
	if !验证管理员名称(admin) || !卡密格式规则.MatchString(cardValue) {
		return 0, fmt.Errorf("卡密参数不正确")
	}
	reason, valid := 规范化可显示文本(reason, 255)
	if !valid {
		return 0, fmt.Errorf("调整备注不能包含控制字符且不能超过255个字符")
	}
	if reason == "" {
		reason = "管理员调整"
	}
	if len([]rune(clientIP)) > 64 {
		clientIP = clientIP[:64]
	}
	tableName, err := 点卡数据表名(admin)
	if err != nil {
		return 0, err
	}
	var balance int64
	err = db.Transaction(func(tx *gorm.DB) error {
		var card 点卡表样式
		query := tx.Table(tableName).Clauses(clause.Locking{Strength: "UPDATE"}).Where("card = ?", cardValue).First(&card)
		if errors.Is(query.Error, gorm.ErrRecordNotFound) {
			return fmt.Errorf("点卡不存在")
		}
		if query.Error != nil {
			return fmt.Errorf("读取点卡失败")
		}
		if card.Card_state != 卡密状态_正常 && card.Card_state != 卡密状态_冻结 {
			return fmt.Errorf("卡密状态不正常")
		}
		if amount > 0 && card.Point_balance > math.MaxInt64-amount {
			return fmt.Errorf("调整后余额超出允许范围")
		}
		if amount < 0 && card.Point_balance < -amount {
			return fmt.Errorf("调整后余额不能小于0")
		}
		balance = card.Point_balance + amount
		updates := map[string]interface{}{"point_balance": balance}
		if result := tx.Table(tableName).Where("card = ?", card.Card).Updates(updates); result.Error != nil || result.RowsAffected != 1 {
			return fmt.Errorf("调整点卡余额失败")
		}
		eventType := 点数事件_补点
		if amount < 0 {
			eventType = 点数事件_扣点
		}
		remark := reason
		if clientIP != "" {
			remark += "；操作IP=" + clientIP
		}
		_, err := 保存点数流水(tx, admin, card.Card, card.Software, eventType, amount, card.Point_balance, balance, remark, time.Now())
		return err
	})
	if err != nil {
		return 0, err
	}
	return balance, nil
}
