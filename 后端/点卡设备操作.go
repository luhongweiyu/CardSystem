package main

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func 点卡设备在线(session 点卡设备会话, graceMinutes int64, now time.Time) bool {
	if session.ForcedOffline || session.LastHeartbeatAt.IsZero() {
		return false
	}
	if graceMinutes <= 0 {
		graceMinutes = 默认自动离线时间分钟
	}
	return session.LastHeartbeatAt.Add(time.Duration(graceMinutes) * time.Minute).After(now)
}

// 调用方已锁定卡密行。这里不锁全部会话行，避免同步单条心跳缓存时发生锁倒置。
func 查询点卡登录设备(tx *gorm.DB, admin string, card string) ([]点卡设备会话, error) {
	var rows []点卡设备会话
	err := tx.Table("point_device_session").
		Select("id", "admin", "card", "software", "device_id", "device_alias", "needle", "renewal_period_seconds", "authorized_until", "last_heartbeat_at", "forced_offline").
		Where("admin = ? AND card = ?", admin, card).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("读取设备会话失败")
	}
	return rows, nil
}

// 候选只在内存中筛选；拿到目标缓存锁后再次检查最新心跳。成功选中后返回
// 清理函数，由外层在事务结束后释放，确保转移与旧设备心跳不会交错生效。
func 尝试复用点卡授权(tx *gorm.DB, rows []点卡设备会话, current 点卡设备会话, deviceID string, alias string, period int64, softwareID int, graceMinutes int64) (点卡设备会话, bool, func(bool), error) {
	now := time.Now()
	candidates := make([]点卡设备会话, 0)
	for _, row := range rows {
		if row.DeviceID == deviceID || row.Software != softwareID || !row.AuthorizedUntil.After(now) {
			continue
		}
		合并点卡心跳缓存(&row)
		if !点卡设备在线(row, graceMinutes, now) {
			candidates = append(candidates, row)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].LastHeartbeatAt.Equal(candidates[j].LastHeartbeatAt) {
			return candidates[i].ID < candidates[j].ID
		}
		return candidates[i].LastHeartbeatAt.Before(candidates[j].LastHeartbeatAt)
	})
	for _, candidate := range candidates {
		key := 生成点卡心跳缓存键(candidate.Admin, candidate.Card, candidate.DeviceID)
		entry := 锁定点卡心跳缓存(key)
		if !entry.已失效 && entry.会话.ID == candidate.ID && entry.会话.Needle == candidate.Needle && entry.会话.LastHeartbeatAt.After(candidate.LastHeartbeatAt) {
			candidate.LastHeartbeatAt = entry.会话.LastHeartbeatAt
		}
		now = time.Now()
		if !candidate.AuthorizedUntil.After(now) || 点卡设备在线(candidate, graceMinutes, now) {
			entry.Unlock()
			清除点卡心跳缓存占位(key, entry)
			continue // 只检查下一内存候选，不重查数据库、不重试扣款。
		}
		finish := func(committed bool) { 结束点卡心跳缓存变更(key, entry, committed) }
		// 新设备可能已有过期行；先移除该行，再把原授权行转给新设备，
		// 保证唯一键不冲突、会话数量不增加，任一步失败均回滚。
		if current.ID != 0 {
			if err := tx.Table("point_device_session").Where("id = ?", current.ID).Delete(&点卡设备会话{}).Error; err != nil {
				return 点卡设备会话{}, false, finish, fmt.Errorf("移除旧设备会话失败")
			}
		}
		newNeedle := GetRandomString(32, "a")
		updates := map[string]interface{}{
			"device_id": deviceID, "device_alias": alias, "needle": newNeedle,
			"renewal_period_seconds": period, "last_heartbeat_at": now, "forced_offline": false,
		}
		// 条件更新还会检查数据库的最新心跳，防止候选查询后恰好落库并删除
		// 缓存的心跳被旧事务快照遗漏。冲突直接回滚，不转而额外扣费。
		query := tx.Table("point_device_session").Where("id = ? AND needle = ? AND authorized_until > ?", candidate.ID, candidate.Needle, now).
			Where("(forced_offline = ? OR last_heartbeat_at <= ?)", true, now.Add(-time.Duration(graceMinutes)*time.Minute)).Updates(updates)
		if query.Error != nil {
			return 点卡设备会话{}, false, finish, fmt.Errorf("转移设备授权失败")
		}
		if query.RowsAffected != 1 {
			return 点卡设备会话{}, false, finish, fmt.Errorf("设备状态已变化，请重新登录")
		}
		candidate.DeviceID, candidate.DeviceAlias, candidate.Needle = deviceID, alias, newNeedle
		candidate.RenewalPeriodSeconds, candidate.LastHeartbeatAt, candidate.ForcedOffline = period, now, false
		return candidate, true, finish, nil
	}
	return 点卡设备会话{}, false, nil, nil
}

// 下线不是删除或封禁：保留授权和真实心跳，撤销凭证并停止后台续费。
// 归属校验在卡密行锁内完成，代理不能通过指定其他卡密操作别人的设备。
func 下线点卡设备(admin string, agentID int, card string, deviceID string, needle string) error {
	admin, card, deviceID, err := 规范化点卡设备参数(admin, card, deviceID, "")
	if err != nil {
		return err
	}
	needle, valid := 规范化点卡会话令牌(needle)
	if !valid {
		return fmt.Errorf("设备凭证不正确，请刷新详情")
	}
	tableName, err := 点卡数据表名(admin)
	if err != nil {
		return err
	}
	var finish func(bool)
	defer func() {
		if finish != nil {
			finish(false)
		}
	}()
	err = db.Transaction(func(tx *gorm.DB) error {
		var cardRow 点卡表样式
		query := tx.Table(tableName).Clauses(clause.Locking{Strength: "UPDATE"}).Where("card = ?", card).First(&cardRow)
		if errors.Is(query.Error, gorm.ErrRecordNotFound) || (query.Error == nil && agentID > 0 && cardRow.AgentID != agentID) {
			return fmt.Errorf("卡密不存在或无权操作")
		}
		if query.Error != nil {
			return fmt.Errorf("读取点卡失败")
		}
		session, found, err := 查询点卡设备会话按设备(tx, admin, card, deviceID, false)
		if err != nil {
			return err
		}
		if !found || session.ForcedOffline {
			return nil
		}
		if session.Needle != needle {
			return fmt.Errorf("设备已重新登录，请刷新详情")
		}
		key := 生成点卡心跳缓存键(admin, card, deviceID)
		entry := 锁定点卡心跳缓存(key)
		finish = func(committed bool) { 结束点卡心跳缓存变更(key, entry, committed) }
		updates := map[string]interface{}{"forced_offline": true, "needle": GetRandomString(32, "a")}
		if !entry.已失效 && entry.会话.ID == session.ID && entry.会话.Needle == session.Needle {
			// 只向前同步真实心跳，与下线状态原子提交，不伪造离线时间。
			updates["last_heartbeat_at"] = gorm.Expr("GREATEST(last_heartbeat_at, ?)", entry.会话.LastHeartbeatAt)
		}
		query = tx.Table("point_device_session").Where("id = ? AND needle = ?", session.ID, needle).
			Updates(updates)
		if query.Error != nil || query.RowsAffected != 1 {
			return fmt.Errorf("下线设备失败，请刷新详情")
		}
		return nil
	})
	if finish != nil {
		finish(err == nil)
		finish = nil
	}
	return err
}
