package main

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	点卡会话清理锁  sync.Mutex
	点卡会话清理游标 uint64
)

// 规范化点卡会话令牌限制外部输入长度。needle 由服务器生成，客户端在后续
// 心跳中原样携带；退出时可以附带校验，但它不是设备的唯一身份。
func 规范化点卡会话令牌(needle string) (string, bool) {
	needle = strings.TrimSpace(needle)
	if needle == "" || len(needle) > 64 {
		return "", false
	}
	for _, r := range needle {
		if r < 0x20 || r == 0x7f {
			return "", false
		}
	}
	return needle, true
}

// 规范化点卡设备参数集中处理登录输入。device_id 允许省略并统一保存为空字符串；
// device_alias 仅由登录设置，允许为空且不参与会话唯一性判断。
func 规范化点卡设备参数(admin string, card string, deviceID string, alias string) (string, string, string, error) {
	admin = strings.TrimSpace(admin)
	card = strings.ToLower(strings.TrimSpace(card))
	if !验证管理员名称(admin) || !卡密格式规则.MatchString(card) {
		return "", "", "", fmt.Errorf("会话参数不完整")
	}
	deviceID, valid := 规范化可选设备标识(deviceID)
	if !valid {
		return "", "", "", fmt.Errorf("device_id格式不正确")
	}
	if _, valid := 规范化设备别名(alias); !valid {
		return "", "", "", fmt.Errorf("device_alias格式不正确或超过64个字符")
	}
	return admin, card, deviceID, nil
}

// 查询点卡设备会话按设备是所有会话修改的唯一入口。卡密在管理员范围内唯一且
// 已绑定软件，因此会话身份只使用管理员、卡密和 device_id；lock=true 时加行锁。
func 查询点卡设备会话按设备(tx *gorm.DB, admin string, card string, deviceID string, lock bool) (点卡设备会话, bool, error) {
	var session 点卡设备会话
	query := tx.Table("point_device_session")
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	query = query.Where("admin = ? AND card = ? AND device_id = ?", admin, card, deviceID).First(&session)
	if errors.Is(query.Error, gorm.ErrRecordNotFound) {
		return session, false, nil
	}
	if query.Error != nil {
		return session, false, fmt.Errorf("读取设备会话失败")
	}
	return session, true, nil
}

// 会话是否仍可能在线：只有在“最后心跳 + 自动离线时间”尚未过去，且
// 这个推断窗口确实跨过旧到期时间时，才认为设备可能只是暂时漏报心跳。
//
// 同时检查 now 是为了避免服务停机很久后，把多年以前的会话误判为仍在线，
// 从而在设备重新启动时从旧到期点连续补扣一大段时间。后台清理、登录和心跳
// 都必须使用同一个判断函数，保证三条路径的计费起点一致。
func 会话仍可能在线(session 点卡设备会话, graceMinutes int64, now time.Time) bool {
	if session.ForcedOffline || session.AuthorizedUntil.IsZero() || session.LastHeartbeatAt.IsZero() {
		return false
	}
	// 只有已经到达授权截止时间才需要推断是否漏报心跳；未到期会话
	// 不应被这个函数误判为“需要续费”。
	if session.AuthorizedUntil.After(now) {
		return false
	}
	if graceMinutes <= 0 {
		graceMinutes = 默认自动离线时间分钟
	}
	推断截止 := session.LastHeartbeatAt.Add(time.Duration(graceMinutes) * time.Minute)
	推断窗口跨过到期 := 推断截止.After(session.AuthorizedUntil) && !now.After(推断截止)
	return 推断窗口跨过到期
}

// 计算续费起点同时供登录、心跳和后台清理使用。返回旧到期点表示连续在线，
// 返回当前时间表示已停止后重新启动。
func 计算续费起点(session 点卡设备会话, graceMinutes int64, now time.Time) time.Time {
	if session.AuthorizedUntil.IsZero() || !session.AuthorizedUntil.Before(now) {
		return now
	}
	if 会话仍可能在线(session, graceMinutes, now) {
		return session.AuthorizedUntil
	}
	return now
}

// 保存点卡设备会话只更新会话状态，不触碰卡密余额；余额变更必须由扣点事务完成。
func 保存点卡设备会话(tx *gorm.DB, session *点卡设备会话, alias string, renewalPeriod int64, now time.Time) error {
	updates := map[string]interface{}{
		"authorized_until":       session.AuthorizedUntil,
		"last_heartbeat_at":      now,
		"renewal_period_seconds": renewalPeriod,
		"forced_offline":         session.ForcedOffline,
		"needle":                 session.Needle,
	}
	if alias != "" {
		updates["device_alias"] = alias
		session.DeviceAlias = alias
	}
	session.LastHeartbeatAt = now
	session.RenewalPeriodSeconds = renewalPeriod
	result := tx.Table("point_device_session").Where("id = ?", session.ID).Updates(updates)
	// 同一秒内的重复请求在低精度数据库列上可能是无变化更新；会话已在事务内
	// 锁定并确认存在，因此这里只判断数据库错误，不能把 RowsAffected=0 误判为失败。
	if result.Error != nil {
		return fmt.Errorf("保存设备会话失败")
	}
	return nil
}

func 获取或创建点卡设备会话(tx *gorm.DB, admin string, card string, softwareID int, deviceID string, alias string, renewalPeriod int64) (点卡设备会话, bool, error) {
	session, found, err := 查询点卡设备会话按设备(tx, admin, card, deviceID, true)
	if err != nil || found {
		return session, found, err
	}
	// 卡密行已经在登录事务中锁定；在创建新设备前统计该卡现有会话，
	// 使并发登录不能绕过设备上限。重复登录已有 device_id 不受此限制。
	var deviceCount int64
	if err := tx.Table("point_device_session").Where("admin = ? AND card = ?", admin, card).Count(&deviceCount).Error; err != nil {
		return 点卡设备会话{}, false, fmt.Errorf("统计卡密设备数量失败")
	}
	if deviceCount >= 最大卡密设备数 {
		return 点卡设备会话{}, false, fmt.Errorf("卡密授权设备已达到上限%d台", 最大卡密设备数)
	}
	session, err = 创建点卡设备会话(tx, admin, card, softwareID, deviceID, alias, renewalPeriod)
	return session, false, err
}

func 创建点卡设备会话(tx *gorm.DB, admin string, card string, softwareID int, deviceID string, alias string, renewalPeriod int64) (点卡设备会话, error) {
	now := time.Now()
	session := 点卡设备会话{Admin: admin, Card: card, Software: softwareID, DeviceID: deviceID,
		DeviceAlias: alias, Needle: GetRandomString(32, "a"), RenewalPeriodSeconds: renewalPeriod,
		AuthorizedUntil: now, LastHeartbeatAt: now}
	if err := tx.Table("point_device_session").Create(&session).Error; err != nil {
		// 登录事务已先锁定卡密行，正常登录路径不会并发创建同一卡密设备；
		// 其他数据库错误也不应通过重复写入来掩盖，直接交由事务回滚。
		return 点卡设备会话{}, fmt.Errorf("创建设备会话失败")
	}
	return session, nil
}

// 登录授权时长选择规则：显式 period_seconds 必须存在且启用；不显式指定时，
// 已有会话沿用其下一次续费时长，新会话使用软件默认授权时长。
func 选择登录周期(tx *gorm.DB, admin string, softwareID int, requested int64, existing *点卡设备会话) (点卡周期价格, software, int64, error) {
	settings, err := 读取软件设置(tx, admin, softwareID)
	if err != nil {
		return 点卡周期价格{}, settings, 0, err
	}
	if requested == 0 && existing != nil && 点卡授权时长秒有效(existing.RenewalPeriodSeconds, false) {
		price, _, priceErr := 查询点卡周期价格(tx, admin, softwareID, existing.RenewalPeriodSeconds)
		if priceErr == nil {
			return price, settings, price.PeriodSeconds, nil
		}
		// 授权时长对应的方案被停用时，活跃会话仍可继续使用；真正续费时仍返回原时长，
		// 由调用方再次查询可扣费方案并明确报错，不静默切换到软件默认授权时长。
		return 点卡周期价格{PeriodSeconds: existing.RenewalPeriodSeconds}, settings, existing.RenewalPeriodSeconds, nil
	}
	price, settings, err := 查询点卡周期价格(tx, admin, softwareID, requested)
	if err != nil {
		return 点卡周期价格{}, settings, 0, err
	}
	return price, settings, price.PeriodSeconds, nil
}

// 点卡登录并扣费把卡密锁、会话锁、余额更新、流水和授权截止时间放在同一事务中。
// 软件编号必须从事务内锁定的卡密行读取，客户端不能选择或覆盖卡密所属软件。
func 点卡登录并扣费(admin string, card string, deviceID string, deviceAlias string, periodSeconds int64, 请求优先复用 bool) (点卡登录结果, error) {
	admin, card, deviceID, err := 规范化点卡设备参数(admin, card, deviceID, deviceAlias)
	if err != nil {
		return 点卡登录结果{}, err
	}
	if !点卡授权时长秒有效(periodSeconds, true) {
		return 点卡登录结果{}, fmt.Errorf("授权时长必须为0或%d至%d分钟，0表示使用默认授权时长", 最小点卡计费周期分钟, 最大点卡计费周期分钟)
	}
	tableName, err := 点卡数据表名(admin)
	if err != nil {
		return 点卡登录结果{}, err
	}
	// 登录会读取并修改完整设备会话。先持久化此前仅存在于内存中的普通心跳，
	// 再删除快照，让本次事务始终以数据库中的最新状态开始判断。
	if err := 同步并删除点卡心跳缓存(admin, card, deviceID, ""); err != nil {
		return 点卡登录结果{}, err
	}
	var result 点卡登录结果
	// 被接手设备的缓存锁必须保持到事务提交/回滚以后，不能在 SQL 更新后提前释放。
	var 完成复用缓存变更 func(bool)
	defer func() {
		if 完成复用缓存变更 != nil {
			完成复用缓存变更(false)
		}
	}()
	err = db.Transaction(func(tx *gorm.DB) error {
		var cardRow 点卡表样式
		query := tx.Table(tableName).Clauses(clause.Locking{Strength: "UPDATE"}).Where("card = ?", card).First(&cardRow)
		if errors.Is(query.Error, gorm.ErrRecordNotFound) {
			return fmt.Errorf("点卡不存在")
		}
		if query.Error != nil {
			return fmt.Errorf("读取点卡失败")
		}
		softwareID := cardRow.Software
		if softwareID <= 0 {
			return fmt.Errorf("卡密所属软件不正确")
		}
		if err := 校验点卡状态(cardRow, softwareID); err != nil {
			return err
		}
		params := 点卡扣费参数{Admin: admin, Card: card, Software: softwareID, PeriodSeconds: periodSeconds, DeviceID: deviceID, DeviceAlias: deviceAlias, RemarkPrefix: "登录扣点"}
		if err := 规范化点卡扣费参数(&params); err != nil {
			return err
		}
		// 1. 准备会话：客户端请求复用，且软件允许复用时，才读取整张卡的设备列表。
		允许复用 := false
		if 请求优先复用 {
			settings, err := 读取软件设置(tx, admin, softwareID)
			if err != nil {
				return err
			}
			允许复用 = settings.PointCardReuseEnabled
		}
		var 当前会话 点卡设备会话
		var 登录前已有会话 bool // 本次新建的会话不算已有会话，不能沿用其续费周期和起点。
		var 卡密设备列表 []点卡设备会话
		if 允许复用 {
			// 一次读取当前会话、复用候选和数量上限所需字段，不逐台查询。
			卡密设备列表, err = 查询点卡登录设备(tx, admin, card)
			for _, 设备会话 := range 卡密设备列表 {
				if 设备会话.DeviceID == deviceID {
					当前会话, 登录前已有会话 = 设备会话, true
					break
				}
			}
		} else {
			当前会话, 登录前已有会话, err = 获取或创建点卡设备会话(tx, admin, card, softwareID, deviceID, params.DeviceAlias, 0)
		}
		if err != nil {
			return err
		}
		// 卡密行已锁定后再取时间，避免排队等待锁的旧请求
		// 用较早时间覆盖已经完成的新心跳，导致在线推断窗口意外缩短。
		now := time.Now()
		var 周期参考会话 *点卡设备会话
		if 登录前已有会话 {
			周期参考会话 = &当前会话
		}
		price, settings, period, err := 选择登录周期(tx, admin, softwareID, periodSeconds, 周期参考会话)
		if err != nil {
			return err
		}
		if 登录前已有会话 && params.DeviceAlias == "" {
			// 本次未重新提交别名时，扣点流水仍应记录会话中已有的
			// 别名快照，避免同一设备后续扣点变成“无别名”。
			params.DeviceAlias = 当前会话.DeviceAlias
		}

		// 2. 自己的授权未到期，直接使用，不尝试接手其他设备。
		if 登录前已有会话 && 当前会话.AuthorizedUntil.After(now) {
			// 当前授权时长尚未结束：不扣点。若客户端明确提出新时长，
			// 只更新下一次续费时长，当前授权截止时间不变。
			if periodSeconds == 0 {
				period = 当前会话.RenewalPeriodSeconds
				if !点卡授权时长秒有效(period, false) {
					period = settings.DefaultPeriodSeconds
				}
			}
			if 当前会话.ForcedOffline {
				当前会话.ForcedOffline = false
				当前会话.Needle = GetRandomString(32, "a")
			}
			if err := 保存点卡设备会话(tx, &当前会话, params.DeviceAlias, period, now); err != nil {
				return err
			}
			result = 点卡登录结果{Card: cardRow, Session: 当前会话, Charge: 点卡扣费结果{Balance: cardRow.Point_balance, AuthorizedUntil: 当前会话.AuthorizedUntil}, PeriodSeconds: period, HeartbeatSeconds: settings.HeartbeatIntervalSeconds}
			return nil
		}

		// 3. 尝试接手离线设备的剩余授权；成功就返回，不扣点。
		if 允许复用 {
			var 复用成功 bool
			var 接手会话 点卡设备会话
			接手会话, 复用成功, 完成复用缓存变更, err = 尝试复用点卡授权(tx, 卡密设备列表, 当前会话, deviceID, params.DeviceAlias, period, softwareID, settings.OnlineGraceMinutes)
			if err != nil {
				return err
			}
			if 复用成功 {
				result = 点卡登录结果{Card: cardRow, Session: 接手会话, Charge: 点卡扣费结果{Balance: cardRow.Point_balance, AuthorizedUntil: 接手会话.AuthorizedUntil}, PeriodSeconds: period, HeartbeatSeconds: settings.HeartbeatIntervalSeconds}
				return nil
			}
		}

		// 4. 没有可用授权，正常扣费。复用路径未提前建会话，在这里按需补建。
		if 允许复用 && !登录前已有会话 {
			if int64(len(卡密设备列表)) >= 最大卡密设备数 {
				return fmt.Errorf("卡密授权设备已达到上限%d台", 最大卡密设备数)
			}
			当前会话, err = 创建点卡设备会话(tx, admin, card, softwareID, deviceID, params.DeviceAlias, period)
			if err != nil {
				return err
			}
		}
		now = time.Now()
		start := now
		if 登录前已有会话 {
			start = 计算续费起点(当前会话, settings.OnlineGraceMinutes, now)
		}
		// 过期会话必须使用可扣费的实际方案；活跃会话才允许使用停用时长对应的
		// 临时占位价格。这样不会在过期时绕过管理员的停用设置。
		if price.Cost <= 0 {
			price, _, err = 查询点卡周期价格(tx, admin, softwareID, period)
			if err != nil {
				return err
			}
		}
		charge, err := 扣除点数事务(tx, tableName, &cardRow, params, price, period, now)
		if err != nil {
			// 事务回滚后已有会话保持原样。管理员补点或客户端改用新的
			// 提交新的有效授权时长后可以继续重试，不会破坏已签发的 needle。
			return err
		}
		until := start.Add(time.Duration(period) * time.Second)
		if !until.After(now) {
			until = now.Add(time.Duration(period) * time.Second)
		}
		当前会话.AuthorizedUntil = until
		当前会话.RenewalPeriodSeconds = period
		当前会话.LastHeartbeatAt = now
		updates := map[string]interface{}{"authorized_until": until, "renewal_period_seconds": period, "last_heartbeat_at": now}
		if 当前会话.ForcedOffline {
			当前会话.ForcedOffline = false
			当前会话.Needle = GetRandomString(32, "a")
			updates["forced_offline"] = false
			updates["needle"] = 当前会话.Needle
		}
		if params.DeviceAlias != "" {
			updates["device_alias"] = params.DeviceAlias
			当前会话.DeviceAlias = params.DeviceAlias
		}
		if result := tx.Table("point_device_session").Where("id = ?", 当前会话.ID).Updates(updates); result.Error != nil || result.RowsAffected != 1 {
			return fmt.Errorf("保存授权时长失败")
		}
		charge.AuthorizedUntil = until
		cardRow.Point_balance = charge.Balance
		result = 点卡登录结果{Card: cardRow, Session: 当前会话, Charge: charge, PeriodSeconds: period, HeartbeatSeconds: settings.HeartbeatIntervalSeconds}
		return nil
	})
	if err != nil {
		return 点卡登录结果{}, err
	}
	if 完成复用缓存变更 != nil {
		完成复用缓存变更(true)
		完成复用缓存变更 = nil
	}
	记录点卡代扣日志(result.Charge, result.Session)
	// 初次失效与事务之间可能有并发心跳重新建立缓存；事务提交后再清理一次，
	// 保证后续请求一定从本次登录提交的数据库状态重新加载。
	if cacheErr := 同步并删除点卡心跳缓存(admin, card, deviceID, ""); cacheErr != nil {
		日志("log/启动记录.txt", "登录后同步心跳缓存失败:"+cacheErr.Error())
	}
	return result, nil
}

// 点卡设备心跳按管理员、卡密和 device_id 查找会话，再校验 needle。device_id
// 允许为空，但其规范化结果必须与登录时保存的值完全一致；心跳不接收或更新设备别名。
func 点卡设备心跳(admin string, card string, needle string, deviceID string) (点卡心跳结果, error) {
	var result 点卡心跳结果
	needle, valid := 规范化点卡会话令牌(needle)
	if !valid {
		return result, fmt.Errorf("登录令牌格式不正确")
	}
	admin = strings.TrimSpace(admin)
	card = strings.ToLower(strings.TrimSpace(card))
	if !验证管理员名称(admin) || !卡密格式规则.MatchString(card) {
		return result, fmt.Errorf("会话参数不完整")
	}
	deviceID, valid = 规范化可选设备标识(deviceID)
	if !valid {
		return result, fmt.Errorf("device_id格式不正确")
	}
	now := time.Now()
	if cached, handled, cacheErr := 尝试记录点卡缓存心跳(admin, card, deviceID, needle, now); handled {
		return cached, cacheErr
	}
	// 缓存存在但授权已经到期时，必须先把到期前最后一次真实心跳写入数据库，
	// 再由原有事务判断是连续在线续费还是离线后重新计费。
	if err := 同步并删除点卡心跳缓存(admin, card, deviceID, needle); err != nil {
		return result, err
	}
	cacheKey := 生成点卡心跳缓存键(admin, card, deviceID)
	cacheEntry := 预留点卡心跳缓存(cacheKey)
	defer 清除点卡心跳缓存占位(cacheKey, cacheEntry)
	tableName, err := 点卡数据表名(admin)
	if err != nil {
		return result, err
	}
	var terminalErr error
	err = db.Transaction(func(tx *gorm.DB) error {
		var cardRow 点卡表样式
		query := tx.Table(tableName).Clauses(clause.Locking{Strength: "UPDATE"}).Where("card = ?", card).First(&cardRow)
		if errors.Is(query.Error, gorm.ErrRecordNotFound) {
			return fmt.Errorf("点卡不存在")
		}
		if query.Error != nil {
			return fmt.Errorf("读取点卡失败")
		}
		if cardRow.Software <= 0 {
			return fmt.Errorf("卡密所属软件不正确")
		}
		if err := 校验点卡状态(cardRow, cardRow.Software); err != nil {
			return err
		}
		session, found, err := 查询点卡设备会话按设备(tx, admin, card, deviceID, true)
		if err != nil {
			return err
		}
		if !found || session.ForcedOffline || session.DeviceID != deviceID || session.Needle != needle || session.Software != cardRow.Software {
			return fmt.Errorf("登录会话不存在或已失效")
		}
		// 会话行已经锁定，此时生成的时间不会被更早进入但仍在等待锁的请求倒写。
		now := time.Now()
		settings, err := 读取软件设置(tx, admin, cardRow.Software)
		if err != nil {
			return err
		}
		period := session.RenewalPeriodSeconds
		if !点卡授权时长秒有效(period, false) {
			period = settings.DefaultPeriodSeconds
		}
		charge := 点卡扣费结果{Balance: cardRow.Point_balance, AuthorizedUntil: session.AuthorizedUntil}
		if !session.AuthorizedUntil.After(now) {
			price, _, priceErr := 查询点卡周期价格(tx, admin, cardRow.Software, period)
			if priceErr != nil {
				if errors.Is(priceErr, 错误_周期价格不可用) {
					if deleteErr := tx.Table("point_device_session").Where("id = ?", session.ID).Delete(&点卡设备会话{}).Error; deleteErr != nil {
						return fmt.Errorf("删除不可续费会话失败")
					}
					terminalErr = priceErr
					return nil
				}
				return priceErr
			}
			params := 点卡扣费参数{Admin: admin, Card: card, Software: cardRow.Software, PeriodSeconds: period, DeviceID: session.DeviceID, DeviceAlias: session.DeviceAlias, RemarkPrefix: "心跳续费"}
			charge, err = 扣除点数事务(tx, tableName, &cardRow, params, price, period, now)
			if err != nil {
				if errors.Is(err, 错误_点卡余额不足) {
					if deleteErr := tx.Table("point_device_session").Where("id = ?", session.ID).Delete(&点卡设备会话{}).Error; deleteErr != nil {
						return fmt.Errorf("删除余额不足会话失败")
					}
					terminalErr = err
					return nil
				}
				return err
			}
			until := 计算续费起点(session, settings.OnlineGraceMinutes, now).Add(time.Duration(period) * time.Second)
			if !until.After(now) {
				until = now.Add(time.Duration(period) * time.Second)
			}
			session.AuthorizedUntil = until
			charge.AuthorizedUntil = until
		}
		if err := 保存点卡设备会话(tx, &session, "", period, now); err != nil {
			return err
		}
		result = 点卡心跳结果{Session: session, Charge: charge, HeartbeatSeconds: settings.HeartbeatIntervalSeconds}
		return nil
	})
	if err != nil {
		return 点卡心跳结果{}, err
	}
	if terminalErr != nil {
		return 点卡心跳结果{}, terminalErr
	}
	记录点卡代扣日志(result.Charge, result.Session)
	// 续费已经改变授权状态，按统一规则保持缓存为空；普通未到期心跳则缓存
	// 已经提交的数据库结果，后续心跳无需再次读取或更新数据库。
	if !result.Charge.Charged {
		写入点卡心跳缓存(cacheEntry, result.Session, result.HeartbeatSeconds)
	} else if cacheErr := 同步并删除点卡心跳缓存(admin, card, deviceID, needle); cacheErr != nil {
		// 续费事务已经提交，缓存同步错误不能把成功扣点伪装成失败；缓存本身
		// 已经强制失效，记录错误后继续返回新的授权结果。
		日志("log/启动记录.txt", "心跳续费后同步缓存失败:"+cacheErr.Error())
	}
	return result, nil
}

// 退出点卡设备会话是幂等操作。device_id 与登录、心跳使用同一套可选值规范化：
// 省略或显式传空字符串都归一为 ""，再按管理员、卡密和 device_id 删除会话。
// 软件编号从卡密和会话自身维护，退出不接收 software，避免客户端用错误的软件
// 编号阻止合法会话退出。needle 不是设备标识，若提交只作为附加校验。
func 退出点卡设备会话(admin string, card string, deviceID string, needle string) error {
	admin = strings.TrimSpace(admin)
	card = strings.ToLower(strings.TrimSpace(card))
	if !验证管理员名称(admin) || !卡密格式规则.MatchString(card) {
		return fmt.Errorf("退出参数不正确")
	}
	normalizedDeviceID, ok := 规范化可选设备标识(deviceID)
	if !ok {
		return fmt.Errorf("退出时device_id格式不正确")
	}
	normalizedNeedle := ""
	if strings.TrimSpace(needle) != "" {
		normalized, ok := 规范化点卡会话令牌(needle)
		if !ok {
			return fmt.Errorf("登录令牌格式不正确")
		}
		normalizedNeedle = normalized
	}
	tableName, err := 点卡数据表名(admin)
	if err != nil {
		return err
	}
	if err := 同步并删除点卡心跳缓存(admin, card, normalizedDeviceID, normalizedNeedle); err != nil {
		return err
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		// 和登录、复用、下线保持“卡密 -> 会话”的顺序，防止候选读取后
		// 被退出请求删掉；卡密已删除时仍允许幂等清理残留会话。
		var cardRow 点卡表样式
		cardErr := tx.Table(tableName).Select("card").Clauses(clause.Locking{Strength: "UPDATE"}).Where("card = ?", card).Take(&cardRow).Error
		if cardErr != nil && !errors.Is(cardErr, gorm.ErrRecordNotFound) {
			return fmt.Errorf("读取点卡失败")
		}
		session, found, err := 查询点卡设备会话按设备(tx, admin, card, normalizedDeviceID, true)
		if err != nil {
			return err
		}
		if !found {
			return nil // 会话已经退出时重复调用仍然成功。
		}
		if normalizedNeedle != "" && session.Needle != normalizedNeedle {
			return fmt.Errorf("登录令牌与设备会话不匹配")
		}
		result := tx.Table("point_device_session").Where("id = ?", session.ID).Delete(&点卡设备会话{})
		if result.Error != nil || result.RowsAffected != 1 {
			return fmt.Errorf("退出设备会话失败")
		}
		return nil
	})
	if err != nil {
		return err
	}
	// 防止同步与删除事务之间的并发心跳重新建立缓存。数据库会话已经删除，
	// 因此这里只需确保残留快照失效，更新零行也视为正常。
	if cacheErr := 同步并删除点卡心跳缓存(admin, card, normalizedDeviceID, normalizedNeedle); cacheErr != nil {
		日志("log/启动记录.txt", "退出后同步心跳缓存失败:"+cacheErr.Error())
	}
	return nil
}

// 清理点卡设备会话由后台每分钟调用。只有仍可能在线的过期会话才自动
// 续费一次；明确已停止的设备直接删除会话，下一次启动再从当前时间扣点。
func 清理点卡设备会话() {
	if db == nil {
		return
	}
	点卡会话清理锁.Lock()
	defer 点卡会话清理锁.Unlock()

	var sessions []点卡设备会话
	查询时间 := time.Now()
	query := func(afterID uint64) error {
		sessions = sessions[:0]
		return db.Table("point_device_session").Where("authorized_until <= ? AND id > ?", 查询时间, afterID).Order("id ASC").Limit(500).Find(&sessions).Error
	}
	if err := query(点卡会话清理游标); err != nil {
		日志("log/启动记录.txt", "清理点卡设备会话查询失败:"+err.Error())
		return
	}
	// 使用 ID 游标轮转批次，避免单批处理失败时长期卡住后续到期会话。
	if len(sessions) == 0 && 点卡会话清理游标 != 0 {
		点卡会话清理游标 = 0
		if err := query(0); err != nil {
			日志("log/启动记录.txt", "清理点卡设备会话查询失败:"+err.Error())
			return
		}
	}
	if len(sessions) > 0 {
		点卡会话清理游标 = sessions[len(sessions)-1].ID
	}
	for _, session := range sessions {
		// 批次查询使用统一时间只是为了确定候选范围；每个会话处理前重新取时间，
		// 避免前面的慢事务让后面的会话使用过期判断时间。
		if err := 结算过期点卡会话(session.ID, time.Now()); err != nil {
			日志("log/启动记录.txt", fmt.Sprintf("结算过期设备会话%d失败:%s", session.ID, err.Error()))
		}
	}
}

func 结算过期点卡会话(sessionID uint64, now time.Time) error {
	if db == nil || db_point_device_session == nil || sessionID == 0 {
		return nil
	}
	// 先无锁读取租户和卡密，仅用于确定动态卡密表名。真正的处理事务
	// 统一按“卡密行 -> 会话行”加锁，和登录、心跳保持相同顺序，避免死锁。
	var hint 点卡设备会话
	if err := db_point_device_session.Where("id = ?", sessionID).First(&hint).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if err := 同步并删除点卡心跳缓存(hint.Admin, hint.Card, hint.DeviceID, hint.Needle); err != nil {
		return err
	}
	tableName, err := 点卡数据表名(hint.Admin)
	if err != nil {
		return err
	}
	var charge 点卡扣费结果
	var chargedSession 点卡设备会话
	err = db.Transaction(func(tx *gorm.DB) error {
		var card 点卡表样式
		cardQuery := tx.Table(tableName).Clauses(clause.Locking{Strength: "UPDATE"}).Where("card = ?", hint.Card).First(&card)
		if errors.Is(cardQuery.Error, gorm.ErrRecordNotFound) {
			return tx.Table("point_device_session").Where("id = ?", sessionID).Delete(&点卡设备会话{}).Error
		}
		if cardQuery.Error != nil {
			return fmt.Errorf("读取点卡失败: %w", cardQuery.Error)
		}
		var session 点卡设备会话
		sessionQuery := tx.Table("point_device_session").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", sessionID).First(&session)
		if errors.Is(sessionQuery.Error, gorm.ErrRecordNotFound) {
			return nil
		}
		if sessionQuery.Error != nil {
			return fmt.Errorf("读取设备会话失败: %w", sessionQuery.Error)
		}
		if session.Admin != hint.Admin || session.Card != hint.Card || session.AuthorizedUntil.After(now) {
			return nil
		}
		if session.ForcedOffline {
			return tx.Table("point_device_session").Where("id = ?", session.ID).Delete(&点卡设备会话{}).Error
		}
		if err := 校验点卡状态(card, session.Software); err != nil {
			return tx.Table("point_device_session").Where("id = ?", session.ID).Delete(&点卡设备会话{}).Error
		}
		settings, err := 读取软件设置(tx, session.Admin, session.Software)
		if err != nil {
			// 数据库或软件配置暂时不可读时不要悄悄删除会话；让下次清理、
			// 心跳或登录在配置恢复后继续处理。
			return err
		}
		if !会话仍可能在线(session, settings.OnlineGraceMinutes, now) {
			return tx.Table("point_device_session").Where("id = ?", session.ID).Delete(&点卡设备会话{}).Error
		}
		period := session.RenewalPeriodSeconds
		if !点卡授权时长秒有效(period, false) {
			period = settings.DefaultPeriodSeconds
		}
		price, _, err := 查询点卡周期价格(tx, session.Admin, session.Software, period)
		if err != nil {
			// 后台清理没有客户端可以返回错误。价格方案停用或不存在时直接删除
			// 已过期会话，避免每分钟重复读取和结算；恢复配置后由客户端重新登录。
			if errors.Is(err, 错误_周期价格不可用) {
				return tx.Table("point_device_session").Where("id = ?", session.ID).Delete(&点卡设备会话{}).Error
			}
			return err
		}
		params := 点卡扣费参数{Admin: session.Admin, Card: session.Card, Software: session.Software, PeriodSeconds: period, DeviceID: session.DeviceID, DeviceAlias: session.DeviceAlias, RemarkPrefix: "后台续费"}
		settled, err := 扣除点数事务(tx, tableName, &card, params, price, period, now)
		if err != nil {
			// 余额不足时同样删除已过期会话，避免清理任务反复重试。管理员补点后，
			// 客户端重新登录即可建立新的授权会话。
			if errors.Is(err, 错误_点卡余额不足) {
				return tx.Table("point_device_session").Where("id = ?", session.ID).Delete(&点卡设备会话{}).Error
			}
			return err
		}
		until := session.AuthorizedUntil.Add(time.Duration(period) * time.Second)
		if !until.After(now) {
			until = now.Add(time.Duration(period) * time.Second)
		}
		if err := tx.Table("point_device_session").Where("id = ?", session.ID).Updates(map[string]interface{}{"authorized_until": until, "renewal_period_seconds": period}).Error; err != nil {
			return err
		}
		charge = settled
		chargedSession = session
		chargedSession.AuthorizedUntil = until
		return nil
	})
	if err == nil {
		记录点卡代扣日志(charge, chargedSession)
		// 清理事务期间可能有并发请求重新建立缓存，提交后再次失效即可保证
		// 删除或续费后的数据库状态成为下一次心跳的唯一来源。
		if cacheErr := 同步并删除点卡心跳缓存(hint.Admin, hint.Card, hint.DeviceID, hint.Needle); cacheErr != nil {
			return cacheErr
		}
	}
	return err
}
