package main

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// 时长卡心跳缓存按“管理员 + 卡密”定位。时长卡同一时间只保留一个
// needle，因此不需要像点卡设备会话那样再增加 device_id 维度。
type 时长卡心跳缓存键 struct {
	管理员 string
	卡密  string
}

// 时长卡心跳缓存条目只保存登录和心跳返回所需的会话快照。卡密状态、固定
// 时长、暂停和充值等业务仍以数据库为准；心跳产生的最后时间按同步间隔
// 延后写入数据库，避免每次心跳都执行一次读写。
type 时长卡心跳缓存条目 struct {
	sync.Mutex
	管理员    string
	记录     时长卡表样式
	心跳间隔秒  int64
	上次同步时间 time.Time
	脏      bool
	已失效    bool
}

var 全局时长卡心跳缓存 = struct {
	sync.RWMutex
	数据 map[时长卡心跳缓存键]*时长卡心跳缓存条目
}{数据: make(map[时长卡心跳缓存键]*时长卡心跳缓存条目)}

func 生成时长卡心跳缓存键(admin, card string) 时长卡心跳缓存键 {
	return 时长卡心跳缓存键{
		管理员: strings.TrimSpace(admin),
		卡密:  strings.ToLower(strings.TrimSpace(card)),
	}
}

func 查找时长卡心跳缓存(key 时长卡心跳缓存键) *时长卡心跳缓存条目 {
	全局时长卡心跳缓存.RLock()
	entry := 全局时长卡心跳缓存.数据[key]
	全局时长卡心跳缓存.RUnlock()
	return entry
}

// 写入时长卡心跳缓存只接受数据库已经确认的正常、未到期会话。若并发
// 请求已经产生同一个 needle 的更新，则保留缓存中较新的心跳和脏状态，
// 避免数据库旧快照覆盖刚收到的心跳。
func 写入时长卡心跳缓存(admin string, row 时长卡表样式, heartbeatSeconds int64) {
	now := time.Now()
	if strings.TrimSpace(admin) == "" || row.Card == "" || row.CardState != 卡密状态_正常 || row.Needle == "" || row.EndTime == nil || !row.EndTime.After(now) || heartbeatSeconds <= 0 {
		return
	}
	if row.LastHeartbeatAt == nil {
		heartbeat := now
		row.LastHeartbeatAt = &heartbeat
	}
	admin = strings.TrimSpace(admin)
	key := 生成时长卡心跳缓存键(admin, row.Card)
	entry := &时长卡心跳缓存条目{
		管理员:    admin,
		记录:     row,
		心跳间隔秒:  heartbeatSeconds,
		上次同步时间: now,
	}

	// 持有全局锁时短暂锁住旧条目，保证替换时不会丢掉并发心跳已经推进的
	// 时间。其他路径都遵循“条目锁完成后再操作全局 map”的顺序，不会反向死锁。
	全局时长卡心跳缓存.Lock()
	if old := 全局时长卡心跳缓存.数据[key]; old != nil {
		old.Lock()
		if !old.已失效 && old.记录.Needle == row.Needle {
			if old.记录.LastHeartbeatAt != nil && old.记录.LastHeartbeatAt.After(*entry.记录.LastHeartbeatAt) {
				entry.记录.LastHeartbeatAt = old.记录.LastHeartbeatAt
			}
			if old.脏 {
				entry.脏 = true
				entry.上次同步时间 = old.上次同步时间
			}
		}
		old.已失效 = true
		old.Unlock()
	}
	全局时长卡心跳缓存.数据[key] = entry
	全局时长卡心跳缓存.Unlock()
}

// 尝试记录时长卡缓存心跳。缓存命中时只在内存推进 last_heartbeat_at，达到
// 同步间隔才由当前这次心跳写回数据库；缓存不存在、会话已到期或已失效时
// 返回 handled=false，由原有数据库逻辑重新校验完整卡密状态。
func 尝试记录时长卡缓存心跳(admin, card, needle string, now time.Time) (时长卡表样式, int64, bool, error) {
	key := 生成时长卡心跳缓存键(admin, card)
	entry := 查找时长卡心跳缓存(key)
	if entry == nil {
		return 时长卡表样式{}, 0, false, nil
	}

	entry.Lock()
	defer entry.Unlock()
	if entry.已失效 {
		return 时长卡表样式{}, 0, false, nil
	}
	if entry.管理员 != key.管理员 || entry.记录.Card != key.卡密 || entry.记录.Needle != needle {
		return 时长卡表样式{}, 0, true, fmt.Errorf("needle验证失败，可能已在其他设备登录")
	}
	if entry.记录.CardState != 卡密状态_正常 || entry.记录.EndTime == nil || !entry.记录.EndTime.After(now) {
		return 时长卡表样式{}, 0, false, nil
	}
	if entry.记录.LastHeartbeatAt == nil || now.After(*entry.记录.LastHeartbeatAt) {
		heartbeat := now
		entry.记录.LastHeartbeatAt = &heartbeat
		entry.脏 = true
	}
	if entry.脏 && now.Sub(entry.上次同步时间) >= 心跳缓存同步间隔值 {
		if err := 同步时长卡心跳缓存条目(entry); err != nil {
			return 时长卡表样式{}, 0, true, err
		}
		if entry.已失效 {
			return 时长卡表样式{}, 0, false, nil
		}
	}
	return entry.记录, entry.心跳间隔秒, true, nil
}

// 同步时长卡心跳缓存条目要求调用方已经持有条目锁。更新条件同时校验
// 卡密状态和 needle，防止冻结、退出或重新登录后的旧缓存覆盖新会话。
func 同步时长卡心跳缓存条目(entry *时长卡心跳缓存条目) error {
	if entry == nil || entry.已失效 || !entry.脏 {
		return nil
	}
	if db == nil {
		return fmt.Errorf("数据库尚未初始化")
	}
	if entry.记录.LastHeartbeatAt == nil {
		entry.脏 = false
		entry.上次同步时间 = time.Now()
		return nil
	}
	tableName, err := 时长卡数据表名(entry.管理员)
	if err != nil {
		return err
	}
	lastHeartbeat := *entry.记录.LastHeartbeatAt
	result := db.Table(tableName).
		Where("card = ? AND card_state = ? AND needle = ? AND (last_heartbeat_at IS NULL OR last_heartbeat_at < ?)", entry.记录.Card, 卡密状态_正常, entry.记录.Needle, lastHeartbeat).
		Update("last_heartbeat_at", lastHeartbeat)
	if result.Error != nil {
		return fmt.Errorf("同步时长卡心跳失败: %w", result.Error)
	}
	// 影响行数为0时，数据库可能已经有相同或更新的时间，也可能卡密
	// 已被删除/改状态。两种情况都让缓存失效，下一次请求重新读库最安全。
	if result.RowsAffected == 0 {
		entry.已失效 = true
	}
	entry.脏 = false
	entry.上次同步时间 = time.Now()
	return nil
}

func 从时长卡心跳缓存删除(key 时长卡心跳缓存键, expected *时长卡心跳缓存条目) {
	全局时长卡心跳缓存.Lock()
	if 全局时长卡心跳缓存.数据[key] == expected {
		delete(全局时长卡心跳缓存.数据, key)
	}
	全局时长卡心跳缓存.Unlock()
}

// 同步并删除时长卡心跳缓存供登录、退出、暂停、恢复、充值、续费、冻结和
// 删除统一调用。同步完成后无论业务是否继续，都删除旧快照，避免状态操作
// 期间并发心跳重新使用旧状态。
func 同步并删除时长卡心跳缓存(admin, card, expectedNeedle string) error {
	key := 生成时长卡心跳缓存键(admin, card)
	entry := 查找时长卡心跳缓存(key)
	if entry == nil {
		return nil
	}

	entry.Lock()
	if entry.已失效 {
		entry.Unlock()
		从时长卡心跳缓存删除(key, entry)
		return nil
	}
	if expectedNeedle != "" && entry.记录.Needle != expectedNeedle {
		entry.Unlock()
		return fmt.Errorf("登录令牌与时长卡会话不匹配")
	}
	syncErr := 同步时长卡心跳缓存条目(entry)
	entry.已失效 = true
	entry.Unlock()
	从时长卡心跳缓存删除(key, entry)
	return syncErr
}

func 时长卡心跳缓存条目快照() map[时长卡心跳缓存键]*时长卡心跳缓存条目 {
	全局时长卡心跳缓存.RLock()
	entries := make(map[时长卡心跳缓存键]*时长卡心跳缓存条目, len(全局时长卡心跳缓存.数据))
	for key, entry := range 全局时长卡心跳缓存.数据 {
		entries[key] = entry
	}
	全局时长卡心跳缓存.RUnlock()
	return entries
}

// 同步并删除指定卡密的时长卡心跳缓存只扫描一次内存快照，适用于批量
// 删除、冻结或代理批量续费，避免每张卡都重复遍历整个缓存 map。
func 同步并删除时长卡心跳缓存_按卡返回错误(admin string, cards []string) map[string]error {
	admin = strings.TrimSpace(admin)
	cardSet := make(map[string]struct{}, len(cards))
	for _, card := range cards {
		card = strings.ToLower(strings.TrimSpace(card))
		if card != "" {
			cardSet[card] = struct{}{}
		}
	}
	if len(cardSet) == 0 {
		return nil
	}
	errorsByCard := make(map[string]error)
	for key := range 时长卡心跳缓存条目快照() {
		if key.管理员 != admin {
			continue
		}
		if _, exists := cardSet[key.卡密]; !exists {
			continue
		}
		if err := 同步并删除时长卡心跳缓存(key.管理员, key.卡密, ""); err != nil {
			if _, exists := errorsByCard[key.卡密]; !exists {
				errorsByCard[key.卡密] = err
			}
		}
	}
	return errorsByCard
}

func 同步并删除软件时长卡心跳缓存(admin string, softwareID int) error {
	admin = strings.TrimSpace(admin)
	var firstErr error
	for key, entry := range 时长卡心跳缓存条目快照() {
		if key.管理员 != admin {
			continue
		}
		entry.Lock()
		matches := !entry.已失效 && entry.记录.Software == softwareID
		entry.Unlock()
		if !matches {
			continue
		}
		if err := 同步并删除时长卡心跳缓存(key.管理员, key.卡密, ""); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// 合并时长卡心跳缓存只覆盖详情和列表中的最新最后心跳，不触发数据库
// 写入；needle 必须先与数据库记录一致，避免退出或重新登录并发时把旧令牌
// 合并回响应。这样管理端可以看到尚未到同步期限的在线状态，且查询接口
// 不会因为展示数据而增加写入压力。
func 合并时长卡心跳缓存(admin string, row *时长卡表样式) {
	if row == nil {
		return
	}
	key := 生成时长卡心跳缓存键(admin, row.Card)
	entry := 查找时长卡心跳缓存(key)
	if entry == nil {
		return
	}
	entry.Lock()
	defer entry.Unlock()
	if entry.已失效 || row.CardState != 卡密状态_正常 || row.EndTime == nil || row.Needle == "" || entry.记录.Needle == "" || entry.记录.Needle != row.Needle {
		return
	}
	if entry.记录.LastHeartbeatAt != nil && (row.LastHeartbeatAt == nil || entry.记录.LastHeartbeatAt.After(*row.LastHeartbeatAt)) {
		row.LastHeartbeatAt = entry.记录.LastHeartbeatAt
	}
}
