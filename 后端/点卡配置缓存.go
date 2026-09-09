package main

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

// 点卡配置缓存只保存低频变化的软件设置和计费方案，不保存卡密余额、
// 设备会话、心跳或待扣费数据。数据库仍是所有业务数据的唯一可信来源。
const 点卡配置缓存有效期 = 30 * time.Minute

type 点卡配置缓存键 struct {
	管理员 string
	软件  int
}

type 点卡配置缓存条目 struct {
	设置   software
	价格   map[int64]点卡周期价格
	失效时间 time.Time
}

var 全局点卡配置缓存 = struct {
	sync.RWMutex
	版本 uint64
	数据 map[点卡配置缓存键]点卡配置缓存条目
}{数据: make(map[点卡配置缓存键]点卡配置缓存条目)}

// 读取点卡计费配置采用简单的“读取命中、未命中查库”模式。首次查询会一次性
// 读取软件的全部启用方案，后续登录、心跳和后台续费可直接按授权时长取价。
//
// 配置加载期间若管理端完成了修改，全局版本会变化，本次旧快照不会重新写回
// 缓存；这可避免并发加载覆盖刚刚执行的失效操作。即使数据库被外部直接修改，
// 缓存也会在三十分钟内自然过期并重新读取。
func 读取点卡计费配置(tx *gorm.DB, admin string, softwareID int) (点卡配置缓存条目, error) {
	if tx == nil {
		return 点卡配置缓存条目{}, fmt.Errorf("数据库尚未初始化")
	}
	admin = strings.TrimSpace(admin)
	key := 点卡配置缓存键{管理员: admin, 软件: softwareID}
	now := time.Now()

	全局点卡配置缓存.RLock()
	cached, exists := 全局点卡配置缓存.数据[key]
	version := 全局点卡配置缓存.版本
	全局点卡配置缓存.RUnlock()
	if exists && now.Before(cached.失效时间) {
		return cached, nil
	}

	var settings software
	query := tx.Table("software").Where("name = ? AND id = ?", admin, softwareID).First(&settings)
	if errors.Is(query.Error, gorm.ErrRecordNotFound) {
		return 点卡配置缓存条目{}, fmt.Errorf("软件不存在")
	}
	if query.Error != nil {
		return 点卡配置缓存条目{}, fmt.Errorf("读取软件设置失败")
	}
	if !点卡授权时长秒有效(settings.DefaultPeriodSeconds, false) {
		return 点卡配置缓存条目{}, fmt.Errorf("软件默认授权时长配置不正确")
	}
	if settings.HeartbeatIntervalSeconds <= 0 || settings.HeartbeatIntervalSeconds > 最大心跳周期秒 {
		return 点卡配置缓存条目{}, fmt.Errorf("软件心跳间隔配置不正确")
	}
	// 兼容新增字段前已经存在的软件记录：数据库中的 0 按默认 60 分钟解释，
	// 不在只读配置加载期间回写，避免缓存读取承担数据迁移职责。
	if settings.OnlineGraceMinutes == 0 {
		settings.OnlineGraceMinutes = 默认自动离线时间分钟
	}
	if settings.OnlineGraceMinutes < 最小自动离线时间分钟 || settings.OnlineGraceMinutes > 最大自动离线时间分钟 {
		return 点卡配置缓存条目{}, fmt.Errorf("软件自动离线时间配置不正确")
	}

	var prices []点卡周期价格
	if err := tx.Table("point_period_price").Where("admin = ? AND software = ? AND enabled = ?", admin, softwareID, true).Find(&prices).Error; err != nil {
		return 点卡配置缓存条目{}, fmt.Errorf("读取点卡计费方案失败")
	}
	loaded := 点卡配置缓存条目{
		设置:   settings,
		价格:   make(map[int64]点卡周期价格, len(prices)),
		失效时间: now.Add(点卡配置缓存有效期),
	}
	for _, price := range prices {
		loaded.价格[price.PeriodSeconds] = price
	}

	全局点卡配置缓存.Lock()
	if 全局点卡配置缓存.版本 == version {
		全局点卡配置缓存.数据[key] = loaded
	}
	全局点卡配置缓存.Unlock()
	return loaded, nil
}

// 清除点卡计费配置缓存必须在软件或计费方案事务成功后调用。它只删除只读快照，
// 不触发数据库查询或写入，因此配置管理接口不存在“先同步缓存再修改”的分支。
func 清除点卡计费配置缓存(admin string, softwareID int) {
	key := 点卡配置缓存键{管理员: strings.TrimSpace(admin), 软件: softwareID}
	全局点卡配置缓存.Lock()
	全局点卡配置缓存.版本++
	delete(全局点卡配置缓存.数据, key)
	全局点卡配置缓存.Unlock()
}
