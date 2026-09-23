package main

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 读取时长卡范围把代理归属条件放进同一条 SQL。agentID 为 0 时代表管理员
// 范围；非零时只能读取该代理自己生成的时长卡，不能先查到记录再做权限判断。
func 读取时长卡范围(tx *gorm.DB, admin string, agentID int, card string, lock bool) (时长卡表样式, error) {
	var row 时长卡表样式
	if tx == nil || !验证管理员名称(strings.TrimSpace(admin)) || agentID < 0 {
		return row, fmt.Errorf("时长卡查询范围不正确")
	}
	card = strings.ToLower(strings.TrimSpace(card))
	if !卡密格式规则.MatchString(card) {
		return row, fmt.Errorf("时长卡格式不正确")
	}
	tableName, err := 时长卡数据表名(admin)
	if err != nil {
		return row, err
	}
	query := tx.Table(tableName).Where("card = ?", card)
	if agentID > 0 {
		query = query.Where("agent_id = ?", agentID)
	}
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	result := query.First(&row)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return row, fmt.Errorf("时长卡不存在或无权操作")
	}
	if result.Error != nil {
		return row, fmt.Errorf("读取时长卡失败")
	}
	return row, nil
}

// 代理账号_查询时长卡列表复用管理端分页和排序逻辑，仅增加当前代理范围。
func 代理账号_查询时长卡列表(ctx *gin.Context) {
	account := 代理账号_取账号信息(ctx)
	查询时长卡列表(ctx, account.Admin, account.ID)
}

// 代理账号_查询时长卡详情返回自己卡密的完整诊断信息，包括当前 needle。
func 代理账号_查询时长卡详情(ctx *gin.Context) {
	account := 代理账号_取账号信息(ctx)
	row, err := 读取时长卡范围(db, account.Admin, account.ID, input(ctx, "card"), false)
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	成功提示管理端(ctx, gin.H{"data": 构建时长卡详情(account.Admin, row, time.Now())})
}

// 修改时长卡范围只允许维护备注、配置和正常/冻结状态。暂停必须走恢复接口，
// 已用于充值的卡不能复活，避免普通编辑绕过这两类一次性业务状态。
func 修改时长卡范围(admin string, agentID int, cardValue string, notes, config *string, state *int) error {
	updates := make(map[string]interface{})
	if notes != nil {
		value, valid := 规范化可显示文本(*notes, 时长卡最大备注字符数)
		if !valid {
			return fmt.Errorf("备注格式不正确")
		}
		updates["notes"] = value
	}
	if config != nil {
		if err := 校验卡密配置内容(*config); err != nil {
			return err
		}
		updates["config_content"] = *config
	}
	if state != nil {
		if *state != 卡密状态_正常 && *state != 卡密状态_冻结 {
			return fmt.Errorf("卡密状态不正确")
		}
		updates["card_state"] = *state
		if *state == 卡密状态_冻结 {
			updates["needle"] = ""
			updates["last_heartbeat_at"] = nil
		}
	}
	if len(updates) == 0 {
		return fmt.Errorf("没有需要修改的内容")
	}
	tableName, err := 时长卡数据表名(admin)
	if err != nil {
		return err
	}
	// 备注、配置和状态修改都可能与在线会话同时发生。先把缓存中的
	// 最新心跳同步到数据库并失效，事务提交后再做一次失效兜底。
	if cacheErr := 同步并删除时长卡心跳缓存(admin, cardValue, ""); cacheErr != nil {
		return cacheErr
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		row, err := 读取时长卡范围(tx, admin, agentID, cardValue, true)
		if err != nil {
			return err
		}
		if row.CardState == 时长卡状态_暂停 || row.CardState == 时长卡状态_已充值 {
			return fmt.Errorf("当前状态不能通过编辑接口修改")
		}
		result := tx.Table(tableName).Where("card = ?", row.Card).Updates(updates)
		if result.Error != nil {
			return fmt.Errorf("修改时长卡失败")
		}
		return nil
	}); err != nil {
		return err
	}
	if cacheErr := 同步并删除时长卡心跳缓存(admin, cardValue, ""); cacheErr != nil {
		日志("log/启动记录.txt", "修改时长卡后同步心跳缓存失败:"+cacheErr.Error())
	}
	return nil
}

// 代理账号_修改时长卡与管理员编辑字段保持一致，但始终附带 agent_id 范围。
func 代理账号_修改时长卡(ctx *gin.Context) {
	var request struct {
		Card          string  `json:"card"`
		Notes         *string `json:"notes"`
		ConfigContent *string `json:"config_content"`
		CardState     *int    `json:"card_state"`
	}
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil {
		失败提示管理端(ctx, "数据错误")
		return
	}
	account := 代理账号_取账号信息(ctx)
	if err := 修改时长卡范围(account.Admin, account.ID, request.Card, request.Notes, request.ConfigContent, request.CardState); err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	成功提示管理端(ctx, gin.H{"msg": "修改成功"})
}

// 代理账号_批量修改时长卡状态保留逐卡部分成功语义，某张卡不存在或不属于
// 当前代理时只进入失败清单，不会影响同批次的其他卡。
func 代理账号_批量修改时长卡状态(ctx *gin.Context) {
	var request struct {
		Cards     []string `json:"cards"`
		CardState int      `json:"card_state"`
	}
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil || len(request.Cards) == 0 || len(request.Cards) > 1000 {
		失败提示管理端(ctx, "卡密数量不正确")
		return
	}
	account := 代理账号_取账号信息(ctx)
	success, failed := make([]string, 0, len(request.Cards)), make([]string, 0)
	for _, card := range request.Cards {
		state := request.CardState
		if err := 修改时长卡范围(account.Admin, account.ID, card, nil, nil, &state); err != nil {
			failed = append(failed, card)
			continue
		}
		success = append(success, strings.ToLower(strings.TrimSpace(card)))
	}
	成功提示管理端(ctx, gin.H{"msg": fmt.Sprintf("成功%d张，失败%d张", len(success), len(failed)), "success": success, "failed": failed})
}

// 代理账号_删除时长卡逐卡删除自己生成的记录。数据库条件同时包含 card 和
// agent_id，避免代理通过已知卡密名删除管理员或其他代理的卡。
func 代理账号_删除时长卡(ctx *gin.Context) {
	var request struct {
		Cards []string `json:"cards"`
	}
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil || len(request.Cards) == 0 || len(request.Cards) > 1000 {
		失败提示管理端(ctx, "卡密数量不正确")
		return
	}
	account := 代理账号_取账号信息(ctx)
	tableName, err := 时长卡数据表名(account.Admin)
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	success, failed := make([]string, 0, len(request.Cards)), make([]string, 0)
	for _, raw := range request.Cards {
		card := strings.ToLower(strings.TrimSpace(raw))
		if !卡密格式规则.MatchString(card) {
			failed = append(failed, raw)
			continue
		}
		if cacheErr := 同步并删除时长卡心跳缓存(account.Admin, card, ""); cacheErr != nil {
			failed = append(failed, raw)
			continue
		}
		result := db.Table(tableName).Where("card = ? AND agent_id = ?", card, account.ID).Delete(&时长卡表样式{})
		if result.Error != nil || result.RowsAffected != 1 {
			failed = append(failed, raw)
			continue
		}
		if cacheErr := 同步并删除时长卡心跳缓存(account.Admin, card, ""); cacheErr != nil {
			日志("log/启动记录.txt", "删除代理时长卡后同步心跳缓存失败:"+cacheErr.Error())
		}
		success = append(success, card)
	}
	成功提示管理端(ctx, gin.H{"msg": fmt.Sprintf("成功%d张，失败%d张", len(success), len(failed)), "success": success, "failed": failed})
}

// 代理续费报价按软件分别计算。批量里允许包含多个软件，每个软件都必须有
// 覆盖本次续费时长的启用锚点；最终一次扣除余额，避免部分扣款。
func 计算代理续费总额(tx *gorm.DB, account 代理账号记录, rows []时长卡表样式, durationMinutes int64) (int64, error) {
	counts := make(map[int]int)
	for _, row := range rows {
		counts[row.Software]++
	}
	softwareIDs := make([]int, 0, len(counts))
	for softwareID := range counts {
		softwareIDs = append(softwareIDs, softwareID)
	}
	sort.Ints(softwareIDs)
	total := int64(0)
	for _, softwareID := range softwareIDs {
		prices, err := 读取时长卡代理价格(tx, account.Admin, account.ID, softwareID, true, true)
		if err != nil {
			return 0, err
		}
		quote, err := 计算时长卡代理费用(prices, durationMinutes, counts[softwareID])
		if err != nil {
			return 0, fmt.Errorf("软件%d续费计价失败: %w", softwareID, err)
		}
		if quote.Charge > math.MaxInt64-total {
			return 0, fmt.Errorf("续费金额超出允许范围")
		}
		total += quote.Charge
	}
	return total, nil
}

// 代理账号_续费时长卡在一个短事务内锁定代理余额和目标卡密。无权操作、
// 未激活、冻结或已用于充值的卡进入失败清单；已激活卡延长 end_time，暂停
// 卡累加 paused_remaining_minutes，所有有效卡统一计价、扣款和续费。
func 代理账号_续费时长卡(ctx *gin.Context) {
	var request struct {
		Cards           []string `json:"cards"`
		DurationMinutes int64    `json:"duration_minutes"`
	}
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil || request.DurationMinutes < 时长卡最小时长分钟 || request.DurationMinutes > 时长卡永久分钟 {
		失败提示管理端(ctx, fmt.Sprintf("续费时长必须在%d分钟至%d天之间", 时长卡最小时长分钟, 时长卡永久分钟/1440))
		return
	}
	if len(request.Cards) == 0 || len(request.Cards) > 时长卡最大单次数量 {
		失败提示管理端(ctx, fmt.Sprintf("单次续费数量必须在1至%d之间", 时长卡最大单次数量))
		return
	}
	account := 代理账号_取账号信息(ctx)
	tableName, err := 时长卡数据表名(account.Admin)
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
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
	// 先同步所有待续费卡的脏心跳，再锁定代理余额和卡密。同步失败的
	// 单卡不参与本次续费，其他卡仍按原有逐卡结果继续处理。
	readyCards := make([]string, 0, len(validCards))
	for _, card := range validCards {
		if cacheErr := 同步并删除时长卡心跳缓存(account.Admin, card, ""); cacheErr != nil {
			failed = append(failed, card)
			continue
		}
		readyCards = append(readyCards, card)
	}
	validCards = readyCards
	success := make([]string, 0, len(validCards))
	charge, balance := int64(0), account.Balance
	err = db.Transaction(func(tx *gorm.DB) error {
		current, err := 锁定时长卡代理账号(tx, account.Admin, account.ID)
		if err != nil {
			return err
		}
		var rows []时长卡表样式
		if len(validCards) > 0 {
			query := tx.Table(tableName).Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("card IN ? AND agent_id = ?", validCards, account.ID).Order("card ASC").Find(&rows)
			if query.Error != nil {
				return fmt.Errorf("读取续费时长卡失败")
			}
		}
		found := make(map[string]时长卡表样式, len(rows))
		for _, row := range rows {
			found[row.Card] = row
		}
		eligible := make([]时长卡表样式, 0, len(rows))
		for _, card := range validCards {
			row, exists := found[card]
			active := exists && row.CardState == 卡密状态_正常 && row.EndTime != nil
			paused := exists && row.CardState == 时长卡状态_暂停 && row.EndTime == nil && row.PausedRemainingMinutes > 0
			if !active && !paused {
				failed = append(failed, card)
				continue
			}
			eligible = append(eligible, row)
		}
		if len(eligible) == 0 {
			balance = current.Balance
			return nil
		}
		charge, err = 计算代理续费总额(tx, current, eligible, request.DurationMinutes)
		if err != nil {
			return err
		}
		if _, err := 检查代理扣款余额(current, charge); err != nil {
			return err
		}
		if result := tx.Table(代理账号表名).Where("id = ? AND admin = ?", current.ID, current.Admin).
			UpdateColumn("balance", gorm.Expr("balance - ?", charge)); result.Error != nil || result.RowsAffected != 1 {
			return fmt.Errorf("扣除代理余额失败")
		}
		now := time.Now()
		for _, row := range eligible {
			if row.CardState == 时长卡状态_暂停 {
				if request.DurationMinutes > math.MaxInt64-row.PausedRemainingMinutes {
					return fmt.Errorf("暂停剩余时长溢出")
				}
				if result := tx.Table(tableName).Where("card = ? AND agent_id = ?", row.Card, current.ID).Update("paused_remaining_minutes", row.PausedRemainingMinutes+request.DurationMinutes); result.Error != nil || result.RowsAffected != 1 {
					return fmt.Errorf("保存时长卡续费失败")
				}
			} else {
				base := *row.EndTime
				if base.Before(now) {
					base = now
				}
				end := base.Add(time.Duration(request.DurationMinutes) * time.Minute)
				if result := tx.Table(tableName).Where("card = ? AND agent_id = ?", row.Card, current.ID).Update("end_time", end); result.Error != nil || result.RowsAffected != 1 {
					return fmt.Errorf("保存时长卡续费失败")
				}
			}
			success = append(success, row.Card)
		}
		balance = current.Balance - charge
		return nil
	})
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	for _, card := range success {
		if cacheErr := 同步并删除时长卡心跳缓存(account.Admin, card, ""); cacheErr != nil {
			日志("log/启动记录.txt", "代理续费时长卡后同步心跳缓存失败:"+cacheErr.Error())
		}
	}
	if len(success) > 0 {
		代理账号日志(account.ID, fmt.Sprintf("余额:%d", balance), fmt.Sprintf("变更:-%d", charge), "原因:续费时长卡", fmt.Sprintf("时长:%d分钟", request.DurationMinutes), fmt.Sprintf("数量:%d", len(success)))
	}
	成功提示管理端(ctx, gin.H{"msg": fmt.Sprintf("成功%d张，失败%d张", len(success), len(failed)), "success": success, "failed": failed, "charge": charge, "balance": balance})
}
