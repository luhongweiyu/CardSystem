package main

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
)

const (
	默认心跳缓存同步间隔分钟 = 15
	最小心跳缓存同步间隔分钟 = 15
	最大心跳缓存同步间隔分钟 = 60
)

// 点卡心跳缓存键严格沿用设备会话的唯一组合。needle 只负责校验当前会话，
// 不参与缓存定位，避免把服务端令牌误当成设备身份。
type 点卡心跳缓存键 struct {
	管理员  string
	卡密   string
	设备ID string
}

// 点卡心跳缓存条目只保存普通心跳快速返回所需的会话快照。余额、扣点和流水
// 始终留在数据库事务中；单条目锁只会阻塞同一设备，不会阻塞其他设备心跳。
type 点卡心跳缓存条目 struct {
	sync.Mutex
	会话     点卡设备会话
	心跳间隔秒  int64
	上次同步时间 time.Time
	脏      bool
	已失效    bool
}

var 全局点卡心跳缓存 = struct {
	sync.RWMutex
	数据 map[点卡心跳缓存键]*点卡心跳缓存条目
}{数据: make(map[点卡心跳缓存键]*点卡心跳缓存条目)}

// 心跳同步间隔在服务启动时读取一次，运行期间直接使用该值，避免高频心跳
// 反复访问配置；修改配置后重启服务生效。
var 心跳缓存同步间隔值 = time.Duration(默认心跳缓存同步间隔分钟) * time.Minute

func 生成点卡心跳缓存键(admin string, card string, deviceID string) 点卡心跳缓存键 {
	return 点卡心跳缓存键{
		管理员:  strings.TrimSpace(admin),
		卡密:   strings.ToLower(strings.TrimSpace(card)),
		设备ID: strings.TrimSpace(deviceID),
	}
}

func 查找点卡心跳缓存(key 点卡心跳缓存键) *点卡心跳缓存条目 {
	全局点卡心跳缓存.RLock()
	entry := 全局点卡心跳缓存.数据[key]
	全局点卡心跳缓存.RUnlock()
	return entry
}

// 写入点卡心跳缓存只在数据库已经返回有效会话后执行。并发请求已经建立同键
// 缓存时保留现有条目，避免用较早的数据库快照覆盖刚收到的新心跳。
func 写入点卡心跳缓存(session 点卡设备会话, heartbeatSeconds int64) {
	if session.ID == 0 || session.Admin == "" || session.Card == "" || session.Needle == "" || heartbeatSeconds <= 0 {
		return
	}
	key := 生成点卡心跳缓存键(session.Admin, session.Card, session.DeviceID)
	entry := &点卡心跳缓存条目{
		会话:     session,
		心跳间隔秒:  heartbeatSeconds,
		上次同步时间: time.Now(),
	}
	全局点卡心跳缓存.Lock()
	if _, exists := 全局点卡心跳缓存.数据[key]; !exists {
		全局点卡心跳缓存.数据[key] = entry
	}
	全局点卡心跳缓存.Unlock()
}

// 尝试记录缓存心跳只处理仍在授权期内的普通心跳。授权已经到期时不能先把
// last_heartbeat_at 改成当前时间，否则会把真正离线后重新启动的设备误判为连续在线；
// 这类请求返回未处理，由现有数据库事务决定续费起点。
func 尝试记录缓存心跳(admin string, card string, deviceID string, needle string, now time.Time) (点卡心跳结果, bool, error) {
	key := 生成点卡心跳缓存键(admin, card, deviceID)
	entry := 查找点卡心跳缓存(key)
	if entry == nil {
		return 点卡心跳结果{}, false, nil
	}

	entry.Lock()
	defer entry.Unlock()
	if entry.已失效 {
		return 点卡心跳结果{}, false, nil
	}
	if entry.会话.Admin != key.管理员 || entry.会话.Card != key.卡密 || entry.会话.DeviceID != key.设备ID || entry.会话.Needle != needle {
		return 点卡心跳结果{}, true, fmt.Errorf("登录会话不存在或已失效")
	}
	if !entry.会话.AuthorizedUntil.After(now) {
		return 点卡心跳结果{}, false, nil
	}
	if now.After(entry.会话.LastHeartbeatAt) {
		entry.会话.LastHeartbeatAt = now
		entry.脏 = true
	}
	// 普通心跳不再由后台任务扫描。达到同步间隔时，由当前这次心跳
	// 负责把最新时间写入数据库；条目锁保证同一设备不会并发重复写入。
	if entry.脏 && now.Sub(entry.上次同步时间) >= 心跳缓存同步间隔值 {
		if err := 同步点卡心跳缓存条目(entry); err != nil {
			return 点卡心跳结果{}, true, err
		}
		if entry.已失效 {
			return 点卡心跳结果{}, false, nil
		}
	}
	session := entry.会话
	return 点卡心跳结果{
		Session:          session,
		Charge:           点卡扣费结果{AuthorizedUntil: session.AuthorizedUntil},
		HeartbeatSeconds: entry.心跳间隔秒,
	}, true, nil
}

// 同步缓存条目要求调用方已经持有条目锁。更新条件包含完整会话身份并且只允许
// 时间向前推进，防止并发数据库事务完成后被较旧的缓存时间倒写。
func 同步点卡心跳缓存条目(entry *点卡心跳缓存条目) error {
	if entry == nil || entry.已失效 || !entry.脏 {
		return nil
	}
	if db_point_device_session == nil {
		return fmt.Errorf("数据库尚未初始化")
	}
	session := entry.会话
	result := db_point_device_session.
		Where("id = ? AND admin = ? AND card = ? AND device_id = ? AND needle = ? AND last_heartbeat_at < ?", session.ID, session.Admin, session.Card, session.DeviceID, session.Needle, session.LastHeartbeatAt).
		Update("last_heartbeat_at", session.LastHeartbeatAt)
	if result.Error != nil {
		return fmt.Errorf("同步设备心跳失败: %w", result.Error)
	}
	// RowsAffected=0 可能是数据库已有相同或更新的时间，也可能是会话已删除。
	// 两种情况都让缓存失效，下一次心跳重新读取数据库最安全。
	if result.RowsAffected == 0 {
		entry.已失效 = true
	}
	entry.脏 = false
	entry.上次同步时间 = time.Now()
	return nil
}

func 从点卡心跳缓存删除(key 点卡心跳缓存键, expected *点卡心跳缓存条目) {
	全局点卡心跳缓存.Lock()
	if 全局点卡心跳缓存.数据[key] == expected {
		delete(全局点卡心跳缓存.数据, key)
	}
	全局点卡心跳缓存.Unlock()
}

// 同步并删除点卡心跳缓存供登录、续费、退出及后台清理统一调用。数据库同步
// 期间只持有当前设备的条目锁；随后无论同步成功与否都让快照失效，避免状态操作
// 失败后旧缓存继续对外授权。同步错误仍向调用方返回，由原业务决定是否继续事务。
func 同步并删除点卡心跳缓存(admin string, card string, deviceID string, expectedNeedle string) error {
	key := 生成点卡心跳缓存键(admin, card, deviceID)
	entry := 查找点卡心跳缓存(key)
	if entry == nil {
		return nil
	}

	entry.Lock()
	if entry.已失效 {
		entry.Unlock()
		从点卡心跳缓存删除(key, entry)
		return nil
	}
	if expectedNeedle != "" && entry.会话.Needle != expectedNeedle {
		entry.Unlock()
		return fmt.Errorf("登录令牌与设备会话不匹配")
	}
	syncErr := 同步点卡心跳缓存条目(entry)
	entry.已失效 = true
	entry.Unlock()
	从点卡心跳缓存删除(key, entry)
	return syncErr
}

func 心跳缓存条目快照() map[点卡心跳缓存键]*点卡心跳缓存条目 {
	全局点卡心跳缓存.RLock()
	entries := make(map[点卡心跳缓存键]*点卡心跳缓存条目, len(全局点卡心跳缓存.数据))
	for key, entry := range 全局点卡心跳缓存.数据 {
		entries[key] = entry
	}
	全局点卡心跳缓存.RUnlock()
	return entries
}

// 同步并删除卡密心跳缓存_按卡返回错误只扫描一次缓存，并分别记录每张卡首次
// 遇到的同步错误。批量删除可据此跳过同步失败的卡密，同时继续处理其他卡密。
func 同步并删除卡密心跳缓存_按卡返回错误(admin string, cards []string) map[string]error {
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
	for key := range 心跳缓存条目快照() {
		if key.管理员 != admin {
			continue
		}
		if _, exists := cardSet[key.卡密]; !exists {
			continue
		}
		if err := 同步并删除点卡心跳缓存(key.管理员, key.卡密, key.设备ID, ""); err != nil {
			if _, exists := errorsByCard[key.卡密]; !exists {
				errorsByCard[key.卡密] = err
			}
		}
	}
	return errorsByCard
}

// 同步并删除卡密心跳缓存用于冻结、删除和同名卡密重新生成。即使一张卡有多台
// 设备，也逐设备短暂加锁和同步，不使用覆盖整张卡的大事务。
func 同步并删除卡密心跳缓存(admin string, cards []string) error {
	for _, err := range 同步并删除卡密心跳缓存_按卡返回错误(admin, cards) {
		return err
	}
	return nil
}

// 同步并删除软件心跳缓存用于软件修改或删除。软件编号只从已经加载的会话快照
// 判断；条目可能被并发移除，因此匹配和真正删除时都会重新检查条目状态。
func 同步并删除软件心跳缓存(admin string, softwareID int) error {
	admin = strings.TrimSpace(admin)
	var firstErr error
	for key, entry := range 心跳缓存条目快照() {
		if key.管理员 != admin {
			continue
		}
		entry.Lock()
		matches := !entry.已失效 && entry.会话.Software == softwareID
		entry.Unlock()
		if !matches {
			continue
		}
		if err := 同步并删除点卡心跳缓存(key.管理员, key.卡密, key.设备ID, ""); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// 合并点卡心跳缓存只覆盖同一会话的瞬时心跳状态，供卡密详情计算在线设备。
// 它不触发数据库写入，也不改变缓存的同步期限。
func 合并点卡心跳缓存(session *点卡设备会话) {
	if session == nil {
		return
	}
	key := 生成点卡心跳缓存键(session.Admin, session.Card, session.DeviceID)
	entry := 查找点卡心跳缓存(key)
	if entry == nil {
		return
	}
	entry.Lock()
	defer entry.Unlock()
	if entry.已失效 || entry.会话.ID != session.ID || entry.会话.Needle != session.Needle {
		return
	}
	if entry.会话.LastHeartbeatAt.After(session.LastHeartbeatAt) {
		session.LastHeartbeatAt = entry.会话.LastHeartbeatAt
	}
	if entry.会话.AuthorizedUntil.After(session.AuthorizedUntil) {
		session.AuthorizedUntil = entry.会话.AuthorizedUntil
	}
}

func 读取心跳缓存同步间隔() time.Duration {
	minutes := viper.GetInt("心跳缓存.同步间隔分钟")
	if minutes == 0 {
		minutes = 默认心跳缓存同步间隔分钟
	}
	if minutes < 最小心跳缓存同步间隔分钟 || minutes > 最大心跳缓存同步间隔分钟 {
		日志("log/启动记录.txt", fmt.Sprintf("心跳缓存同步间隔%d分钟不正确，使用默认%d分钟", minutes, 默认心跳缓存同步间隔分钟))
		minutes = 默认心跳缓存同步间隔分钟
	}
	return time.Duration(minutes) * time.Minute
}

// 初始化心跳缓存同步间隔必须在读取配置文件后调用一次；后续心跳只读取内存值。
func 初始化心跳缓存同步间隔() {
	心跳缓存同步间隔值 = 读取心跳缓存同步间隔()
}
