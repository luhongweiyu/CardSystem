package main

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 暂停时长卡把扣除手续费后的剩余时间转成整数分钟保存，并清空当前到期
// 时间和在线凭证。暂停状态下登录和心跳会被拒绝；续费或充值只累加暂停
// 剩余分钟，仍需显式恢复后才能继续使用。
func 暂停时长卡(admin, card string, now time.Time) (int64, error) {
	tableName, err := 时长卡数据表名(admin)
	if err != nil {
		return 0, err
	}
	if cacheErr := 同步并删除时长卡心跳缓存(admin, card, ""); cacheErr != nil {
		return 0, cacheErr
	}
	remainingMinutes := int64(0)
	var softwareID, agentID int
	var beforeEnd time.Time
	var pauseDeductMinutes int64
	err = db.Transaction(func(tx *gorm.DB) error {
		row, err := 读取时长卡范围(tx, admin, 0, card, true)
		if err != nil {
			return err
		}
		if row.CardState != 卡密状态_正常 || row.EndTime == nil || !row.EndTime.After(now) {
			return fmt.Errorf("只有尚未到期的已激活时长卡可以暂停")
		}
		// 暂停会永久扣除卡内时长，因此直接读取事务中的最新软件设置，
		// 不使用允许短期陈旧的只读计费缓存。
		var settings 软件
		if result := tx.Table("software").Where("name = ? AND id = ?", admin, row.Software).First(&settings); result.Error != nil {
			return fmt.Errorf("读取软件暂停设置失败")
		}
		if settings.PauseDeductMinutes <= 0 {
			return fmt.Errorf("该软件未开启时长卡暂停")
		}
		softwareID, agentID = row.Software, row.AgentID
		beforeEnd = *row.EndTime
		pauseDeductMinutes = settings.PauseDeductMinutes
		remainingSeconds := row.EndTime.Sub(now).Seconds() - float64(settings.PauseDeductMinutes*60)
		if remainingSeconds <= 0 {
			return fmt.Errorf("扣除暂停费用后没有剩余时长")
		}
		// 向上取整可避免分钟字段舍弃用户不足一分钟的剩余授权。
		remainingMinutes = int64(math.Ceil(remainingSeconds / 60))
		updates := map[string]interface{}{
			"card_state":               时长卡状态_暂停,
			"paused_remaining_minutes": remainingMinutes,
			"end_time":                 nil,
			"needle":                   "",
			"last_heartbeat_at":        nil,
		}
		result := tx.Table(tableName).Where("card = ?", row.Card).Updates(updates)
		if result.Error != nil || result.RowsAffected != 1 {
			return fmt.Errorf("暂停时长卡失败")
		}
		return nil
	})
	if err != nil {
		return remainingMinutes, err
	}
	if cacheErr := 同步并删除时长卡心跳缓存(admin, card, ""); cacheErr != nil {
		日志("log/启动记录.txt", "暂停时长卡后同步心跳缓存失败:"+cacheErr.Error())
	}
	记录时长卡业务流水(admin, []int{agentID}, "原因:时长卡暂停", "卡密:"+card, fmt.Sprintf("软件:%d", softwareID), fmt.Sprintf("变更:-%d分钟", pauseDeductMinutes), fmt.Sprintf("暂停剩余:%d分钟", remainingMinutes), "原授权截止:"+业务流水时间(beforeEnd))
	return remainingMinutes, nil
}

// 恢复时长卡从当前时间重新计算到期时间，成功后立即清空暂停余额，确保
// 后续重复调用不能再次获得同一段时长。
func 恢复时长卡(admin, card string, now time.Time) (time.Time, error) {
	tableName, err := 时长卡数据表名(admin)
	if err != nil {
		return time.Time{}, err
	}
	if cacheErr := 同步并删除时长卡心跳缓存(admin, card, ""); cacheErr != nil {
		return time.Time{}, cacheErr
	}
	var end time.Time
	var softwareID, agentID int
	var beforeRemaining int64
	err = db.Transaction(func(tx *gorm.DB) error {
		row, err := 读取时长卡范围(tx, admin, 0, card, true)
		if err != nil {
			return err
		}
		if row.CardState != 时长卡状态_暂停 || row.PausedRemainingMinutes <= 0 {
			return fmt.Errorf("时长卡不在暂停状态")
		}
		softwareID, agentID = row.Software, row.AgentID
		beforeRemaining = row.PausedRemainingMinutes
		end = now.Add(time.Duration(row.PausedRemainingMinutes) * time.Minute)
		updates := map[string]interface{}{"card_state": 卡密状态_正常, "paused_remaining_minutes": 0, "end_time": end}
		result := tx.Table(tableName).Where("card = ?", row.Card).Updates(updates)
		if result.Error != nil || result.RowsAffected != 1 {
			return fmt.Errorf("恢复时长卡失败")
		}
		return nil
	})
	if err != nil {
		return end, err
	}
	if cacheErr := 同步并删除时长卡心跳缓存(admin, card, ""); cacheErr != nil {
		日志("log/启动记录.txt", "恢复时长卡后同步心跳缓存失败:"+cacheErr.Error())
	}
	记录时长卡业务流水(admin, []int{agentID}, "原因:时长卡恢复", "卡密:"+card, fmt.Sprintf("软件:%d", softwareID), fmt.Sprintf("变更:+%d分钟", beforeRemaining), "授权截止:"+业务流水时间(end))
	return end, nil
}

// 访客_暂停时长卡保留旧网页能力；软件编号始终从卡密读取，访客不能提交
// software 覆盖真实归属。
func 访客_暂停时长卡(ctx *gin.Context) {
	admin, ok := 访客管理员(ctx)
	if !ok {
		失败提示访客(ctx, "访客上下文错误")
		return
	}
	remaining, err := 暂停时长卡(admin, input(ctx, "card"), time.Now())
	if err != nil {
		失败提示访客(ctx, err.Error())
		return
	}
	成功提示访客(ctx, gin.H{"msg": "暂停成功", "remaining_minutes": remaining})
}

// 访客_恢复时长卡恢复成功后返回新的绝对到期时间，页面无需自行换算。
func 访客_恢复时长卡(ctx *gin.Context) {
	admin, ok := 访客管理员(ctx)
	if !ok {
		失败提示访客(ctx, "访客上下文错误")
		return
	}
	end, err := 恢复时长卡(admin, input(ctx, "card"), time.Now())
	if err != nil {
		失败提示访客(ctx, err.Error())
		return
	}
	成功提示访客(ctx, gin.H{"msg": "恢复成功", "authorized_until": end})
}

// 时长卡给时长卡充值使用一张未激活时长卡作为一次性充值来源。目标卡可以
// 是已激活卡，也可以是暂停卡；两张卡按卡密顺序加锁，避免两个并发互充请求
// 以相反顺序等待形成死锁。
func 时长卡互充(admin, targetCard, sourceCard string, now time.Time) (time.Time, int64, error) {
	targetCard = strings.ToLower(strings.TrimSpace(targetCard))
	sourceCard = strings.ToLower(strings.TrimSpace(sourceCard))
	if !卡密格式规则.MatchString(targetCard) || !卡密格式规则.MatchString(sourceCard) || targetCard == sourceCard {
		return time.Time{}, 0, fmt.Errorf("目标卡密或充值卡密不正确")
	}
	tableName, err := 时长卡数据表名(admin)
	if err != nil {
		return time.Time{}, 0, err
	}
	// 目标卡可能有尚未落库的最新心跳；来源卡通常未激活没有缓存，
	// 但一并处理可以保证异常数据或并发登录不会留下旧快照。
	if cacheErr := 同步并删除时长卡心跳缓存(admin, targetCard, ""); cacheErr != nil {
		return time.Time{}, 0, cacheErr
	}
	if cacheErr := 同步并删除时长卡心跳缓存(admin, sourceCard, ""); cacheErr != nil {
		return time.Time{}, 0, cacheErr
	}
	var end time.Time
	var added int64
	var softwareID, targetAgentID, sourceAgentID int
	var targetBeforeEnd time.Time
	var targetBeforePaused, targetAfterPaused int64
	err = db.Transaction(func(tx *gorm.DB) error {
		cards := []string{targetCard, sourceCard}
		if cards[0] > cards[1] {
			cards[0], cards[1] = cards[1], cards[0]
		}
		var rows []时长卡表样式
		query := tx.Table(tableName).Clauses(clause.Locking{Strength: "UPDATE"}).Where("card IN ?", cards).Order("card ASC").Find(&rows)
		if query.Error != nil {
			return fmt.Errorf("读取时长卡失败")
		}
		if len(rows) != 2 {
			return fmt.Errorf("目标卡密或充值卡密不存在")
		}
		byCard := make(map[string]时长卡表样式, 2)
		for _, row := range rows {
			byCard[row.Card] = row
		}
		target, targetExists := byCard[targetCard]
		source, sourceExists := byCard[sourceCard]
		if !targetExists || !sourceExists {
			return fmt.Errorf("目标卡密或充值卡密不存在")
		}
		if target.Software != source.Software {
			return fmt.Errorf("两张时长卡所属软件不一致")
		}
		targetActive := target.CardState == 卡密状态_正常 && target.EndTime != nil
		targetPaused := target.CardState == 时长卡状态_暂停 && target.PausedRemainingMinutes > 0 && target.EndTime == nil
		if !targetActive && !targetPaused {
			return fmt.Errorf("目标时长卡必须已经激活或处于暂停状态")
		}
		if source.CardState != 卡密状态_正常 || source.EndTime != nil || source.PausedRemainingMinutes != 0 {
			return fmt.Errorf("充值来源必须是未激活且状态正常的时长卡")
		}
		if source.LatestActivationAt != nil && !source.LatestActivationAt.After(now) {
			return fmt.Errorf("充值来源已超过最晚激活时间")
		}
		if source.DurationMinutes < 时长卡最小时长分钟 || source.DurationMinutes > 时长卡永久分钟 {
			return fmt.Errorf("充值来源时长不正确")
		}
		softwareID, targetAgentID, sourceAgentID = target.Software, target.AgentID, source.AgentID
		targetBeforePaused = target.PausedRemainingMinutes
		if target.EndTime != nil {
			targetBeforeEnd = *target.EndTime
		}
		added = source.DurationMinutes
		if targetPaused {
			if added > math.MaxInt64-target.PausedRemainingMinutes {
				return fmt.Errorf("目标时长卡剩余时长溢出")
			}
			remaining := target.PausedRemainingMinutes + added
			targetAfterPaused = remaining
			if result := tx.Table(tableName).Where("card = ?", target.Card).Update("paused_remaining_minutes", remaining); result.Error != nil || result.RowsAffected != 1 {
				return fmt.Errorf("保存目标时长卡失败")
			}
			// 暂停卡没有绝对到期时间，保持 end_time=NULL 和暂停状态。
			end = time.Time{}
		} else {
			base := *target.EndTime
			if base.Before(now) {
				base = now
			}
			end = base.Add(time.Duration(added) * time.Minute)
			if result := tx.Table(tableName).Where("card = ?", target.Card).Update("end_time", end); result.Error != nil || result.RowsAffected != 1 {
				return fmt.Errorf("保存目标时长卡失败")
			}
		}
		sourceNotes := strings.TrimSpace(source.Notes)
		if sourceNotes != "" {
			sourceNotes += "；"
		}
		sourceNotes += "已充值给:" + target.Card
		if len([]rune(sourceNotes)) > 时长卡最大备注字符数 {
			runes := []rune(sourceNotes)
			sourceNotes = string(runes[:时长卡最大备注字符数])
		}
		updates := map[string]interface{}{"card_state": 时长卡状态_已充值, "notes": sourceNotes, "needle": "", "last_heartbeat_at": nil}
		if result := tx.Table(tableName).Where("card = ?", source.Card).Updates(updates); result.Error != nil || result.RowsAffected != 1 {
			return fmt.Errorf("保存充值来源状态失败")
		}
		return nil
	})
	if err != nil {
		return end, added, err
	}
	for _, card := range []string{targetCard, sourceCard} {
		if cacheErr := 同步并删除时长卡心跳缓存(admin, card, ""); cacheErr != nil {
			日志("log/启动记录.txt", "时长卡充值后同步心跳缓存失败:"+cacheErr.Error())
		}
	}
	记录时长卡业务流水(admin, []int{targetAgentID, sourceAgentID}, "原因:时长卡充值", "目标卡:"+targetCard, "来源卡:"+sourceCard, fmt.Sprintf("软件:%d", softwareID), fmt.Sprintf("变更:+%d分钟", added), "原授权截止:"+业务流水时间(targetBeforeEnd), "新授权截止:"+业务流水时间(end), fmt.Sprintf("原暂停剩余:%d分钟", targetBeforePaused), fmt.Sprintf("新暂停剩余:%d分钟", targetAfterPaused))
	return end, added, nil
}

// durationCardRecharge 使用当前登录卡作为目标，card2 为兼容旧客户端的
// 充值来源参数，同时接受含义更明确的 source_card。
func durationCardRecharge(ctx *gin.Context) {
	value, ok := ctx.Get("card")
	cardContext, valid := value.(卡密请求上下文)
	if !ok || !valid {
		失败提示(ctx, "卡密上下文错误")
		return
	}
	source := input(ctx, "source_card")
	if source == "" {
		source = input(ctx, "card2")
	}
	end, added, err := 时长卡互充(cardContext.Name, cardContext.Card, source, time.Now())
	if err != nil {
		失败提示(ctx, err.Error())
		return
	}
	response := gin.H{"msg": "充值成功", "added_minutes": added}
	if end.IsZero() {
		// 暂停中的时长卡没有绝对到期时间，避免返回误导性的授权截止时间。
		response["authorized_until"] = nil
	} else {
		response["authorized_until"] = end
	}
	成功提示(ctx, response)
}
