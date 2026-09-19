package main

import (
	"errors"
	"fmt"
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

// 按缓存顺序逐个检查，不预先过滤整批。候选锁和被接手设备的心跳锁都保持到
// 事务结束；提交后才消费候选，回滚则保留，避免并发登录重复接手或丢掉剩余授权。
func 尝试复用点卡授权(tx *gorm.DB, 参数 点卡扣费参数, 当前会话 点卡设备会话, 续费周期秒 int64, 自动离线分钟 int64) (点卡设备会话, bool, func(bool), error) {
	候选缓存, err := 锁定点卡复用候选缓存(tx, 参数.Admin, 参数.Card)
	if err != nil {
		return 点卡设备会话{}, false, nil, err
	}
	由事务结束解锁 := false
	defer func() {
		if !由事务结束解锁 {
			候选缓存.Unlock()
		}
	}()
	if 自动离线分钟 <= 0 {
		自动离线分钟 = 默认自动离线时间分钟
	}
	for len(候选缓存.候选) > 0 {
		待复用会话 := 候选缓存.候选[0]
		if 待复用会话.DeviceID == 参数.DeviceID || 待复用会话.Software != 参数.Software {
			候选缓存.剔除首个候选()
			continue
		}
		心跳键 := 生成点卡心跳缓存键(待复用会话.Admin, 待复用会话.Card, 待复用会话.DeviceID)
		心跳缓存 := 锁定点卡心跳缓存(心跳键)
		凭证已变化 := false
		if !心跳缓存.已失效 && 心跳缓存.会话.ID != 0 {
			凭证已变化 = 心跳缓存.会话.ID != 待复用会话.ID || 心跳缓存.会话.Needle != 待复用会话.Needle
			if !凭证已变化 {
				if 心跳缓存.会话.LastHeartbeatAt.After(待复用会话.LastHeartbeatAt) {
					待复用会话.LastHeartbeatAt = 心跳缓存.会话.LastHeartbeatAt
				}
				if 心跳缓存.会话.AuthorizedUntil.After(待复用会话.AuthorizedUntil) {
					待复用会话.AuthorizedUntil = 心跳缓存.会话.AuthorizedUntil
				}
			}
		}
		当前时间 := time.Now()
		if 凭证已变化 || !待复用会话.AuthorizedUntil.After(当前时间) || 点卡设备在线(待复用会话, 自动离线分钟, 当前时间) {
			候选缓存.剔除首个候选()
			心跳缓存.Unlock()
			清除点卡心跳缓存占位(心跳键, 心跳缓存)
			continue // 只取下一个内存候选；耗尽也不提前查库。
		}
		完成缓存变更 := func(已提交 bool) {
			if 已提交 {
				候选缓存.剔除首个候选()
			}
			结束点卡心跳缓存变更(心跳键, 心跳缓存, 已提交)
			候选缓存.Unlock()
		}
		由事务结束解锁 = true
		新凭证 := GetRandomString(32, "a")
		更新字段 := map[string]interface{}{
			"device_id": 参数.DeviceID, "device_alias": 参数.DeviceAlias, "needle": 新凭证,
			"renewal_period_seconds": 续费周期秒, "last_heartbeat_at": 当前时间, "forced_offline": false,
		}
		条件更新字段 := 更新字段
		if 当前会话.ID != 0 {
			// 本设备已有过期行时，先只更换候选凭证，确认可接手后再删除旧行。
			// 否则候选失效却先删了本设备，会破坏后续尝试或正常续费。
			条件更新字段 = map[string]interface{}{"needle": 新凭证}
		}
		// 数据库条件更新是最终校验：不能用一分钟前的候选覆盖已经重新登录、
		// 续费或退出的会话。零行更新只表示候选失效，不视为数据库错误。
		更新结果 := tx.Table("point_device_session").
			Where("id = ? AND admin = ? AND card = ? AND software = ? AND device_id = ? AND needle = ?", 待复用会话.ID, 参数.Admin, 参数.Card, 参数.Software, 待复用会话.DeviceID, 待复用会话.Needle).
			Where("authorized_until = ? AND authorized_until > ?", 待复用会话.AuthorizedUntil, 当前时间).
			Where("(forced_offline = ? OR last_heartbeat_at <= ?)", true, 当前时间.Add(-time.Duration(自动离线分钟)*time.Minute)).Updates(条件更新字段)
		if 更新结果.Error != nil {
			return 点卡设备会话{}, false, 完成缓存变更, fmt.Errorf("转移设备授权失败")
		}
		if 更新结果.RowsAffected != 1 {
			// 尚未修改任何会话，直接释放本候选的心跳锁并继续；不补查、不重试它。
			候选缓存.剔除首个候选()
			心跳缓存.Unlock()
			清除点卡心跳缓存占位(心跳键, 心跳缓存)
			由事务结束解锁 = false
			continue
		}
		if 当前会话.ID != 0 {
			// 候选已锁定并更换凭证；删除本设备过期行后完成转移，避免唯一键冲突。
			// 两步仍在同一事务，任一步失败都会恢复旧会话和候选的原凭证。
			if err := tx.Table("point_device_session").Where("id = ?", 当前会话.ID).Delete(&点卡设备会话{}).Error; err != nil {
				return 点卡设备会话{}, false, 完成缓存变更, fmt.Errorf("移除旧设备会话失败")
			}
			转移结果 := tx.Table("point_device_session").Where("id = ? AND needle = ?", 待复用会话.ID, 新凭证).Updates(更新字段)
			if 转移结果.Error != nil || 转移结果.RowsAffected != 1 {
				return 点卡设备会话{}, false, 完成缓存变更, fmt.Errorf("转移设备授权失败")
			}
		}
		待复用会话.DeviceID, 待复用会话.DeviceAlias, 待复用会话.Needle = 参数.DeviceID, 参数.DeviceAlias, 新凭证
		待复用会话.RenewalPeriodSeconds, 待复用会话.LastHeartbeatAt, 待复用会话.ForcedOffline = 续费周期秒, 当前时间, false
		return 待复用会话, true, 完成缓存变更, nil
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
