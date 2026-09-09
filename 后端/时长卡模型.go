package main

import (
	"fmt"
	"time"
)

const (
	// 时长卡拥有独立状态。未激活和已到期只用于管理端展示，数据库中的
	// card_state 仍只保存可操作状态；到期状态由 end_time 动态推断。
	时长卡状态_未激活 = 1
	时长卡状态_到期  = 3
	时长卡状态_暂停  = 5
	时长卡状态_已充值 = 6

	// 旧版时长卡的永久卡使用 36500 天作为约定值。这里保留该约定，
	// 但不把永久卡字段混入纯点卡表。
	时长卡永久分钟    = int64(36500 * 24 * 60)
	时长卡最小时长分钟  = int64(5)
	时长卡最大单次数量  = 500
	时长卡最大备注字符数 = 500
	// 代理时长价格以“百分之一代理余额点”为最小单位保存。接口仍接收
	// 最多两位小数的普通价格，落库时转换成整数，避免浮点误差影响扣款。
	时长卡代理价格最小单位 = int64(100)
	// 单个软件的价格锚点数量设置上限，避免配置错误导致查询和计价遍历过大。
	时长卡代理价格最大锚点数 = 50
	// 计价数量还会用于充值卡的“卡数 × 可用次数”，因此允许大于单次
	// 时长卡生成数量；实际请求仍由各自入口限制卡数和次数。
	时长卡代理计价最大数量 = 500000
)

// 时长卡表样式完全独立于点卡表样式。时长卡的固定时长和到期时间属于卡
// 本身；点卡仍只使用 point_balance 和 point_device_session，两个模式不
// 共享任何会改变计费语义的字段。
type 时长卡表样式 struct {
	Card               string     `gorm:"column:card;size:63;primaryKey;autoIncrement:false" json:"card"`
	CreateTime         time.Time  `gorm:"column:create_time;not null;index" json:"create_time"`
	UseTime            *time.Time `gorm:"column:use_time" json:"use_time"`
	EndTime            *time.Time `gorm:"column:end_time;index" json:"end_time"`
	Software           int        `gorm:"column:software;not null;index" json:"software"`
	CardState          int        `gorm:"column:card_state;not null;default:2;index" json:"card_state"`
	DurationMinutes    int64      `gorm:"column:duration_minutes;not null" json:"duration_minutes"`
	LatestActivationAt *time.Time `gorm:"column:latest_activation_at;index" json:"latest_activation_at"`
	Needle             string     `gorm:"column:needle;size:64;index" json:"-"`
	LastHeartbeatAt    *time.Time `gorm:"column:last_heartbeat_at" json:"last_heartbeat_at"`
	Notes              string     `gorm:"column:notes;size:500" json:"notes"`
	ConfigContent      string     `gorm:"column:config_content;type:longtext" json:"config_content"`
	// PausedRemainingMinutes 只在主动暂停时保存剩余分钟数。恢复后立即清零，
	// 避免同时存在 end_time 和暂停余额两套有效授权来源。
	PausedRemainingMinutes int64 `gorm:"column:paused_remaining_minutes;not null;default:0" json:"paused_remaining_minutes"`
	AgentID                int   `gorm:"column:agent_id;not null;default:0;index" json:"agent_id"`
}

// 时长卡列表项不直接返回 needle，详情/登录接口才按权限返回它。
type 时长卡列表项 struct {
	Card                   string     `json:"card"`
	CreateTime             time.Time  `json:"create_time"`
	UseTime                *time.Time `json:"use_time"`
	EndTime                *time.Time `json:"end_time"`
	Software               int        `json:"software"`
	CardState              int        `json:"card_state"`
	DurationMinutes        int64      `json:"duration_minutes"`
	LatestActivationAt     *time.Time `json:"latest_activation_at"`
	Notes                  string     `json:"notes"`
	ConfigContent          string     `json:"config_content"`
	PausedRemainingMinutes int64      `json:"paused_remaining_minutes"`
	AgentID                int        `json:"agent_id"`
	Online                 bool       `json:"online"`
}

// 时长卡数据表名采用与点卡相同的管理员分表策略，但使用不同前缀，
// 确保同名点卡和时长卡可以同时存在且互不读取。
func 时长卡数据表名(admin string) (string, error) {
	if !验证管理员名称(admin) {
		return "", fmt.Errorf("管理员名称格式不正确")
	}
	return "duration_card_" + admin, nil
}

// 时长卡代理价格表名保存代理账号的时长卡价格锚点。它与点卡的代理价格
// JSON、软件的点卡计费方案完全分离：时长卡可以按卡面分钟数灵活折算，
// 不会把两种卡的价格语义混在一起。
const 时长卡代理价格表名 = "duration_card_agent_price"

// 时长卡代理价格是一条“软件 + 时长”的价格锚点。Price 使用百分之一
// 代理余额点保存，例如 125 表示 1.25 点；Enabled=false 的记录保留
// 供管理员暂时停用，但计价时不会参与。
type 时长卡代理价格 struct {
	ID              uint      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Admin           string    `gorm:"column:admin;size:32;not null;index:idx_duration_agent_price_scope,priority:1;uniqueIndex:uk_duration_agent_price,priority:1" json:"admin"`
	AgentID         int       `gorm:"column:agent_id;not null;index:idx_duration_agent_price_scope,priority:2;uniqueIndex:uk_duration_agent_price,priority:2" json:"agent_id"`
	Software        int       `gorm:"column:software;not null;index:idx_duration_agent_price_scope,priority:3;uniqueIndex:uk_duration_agent_price,priority:3" json:"software"`
	DurationMinutes int64     `gorm:"column:duration_minutes;not null;uniqueIndex:uk_duration_agent_price,priority:4" json:"duration_minutes"`
	Price           int64     `gorm:"column:price;not null" json:"-"`
	Enabled         bool      `gorm:"column:enabled;not null;default:true;index:idx_duration_agent_price_scope,priority:4" json:"enabled"`
	CreatedAt       time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName 固定价格锚点表名，避免中文模型名被 GORM 推导成不可控的表名。
func (时长卡代理价格) TableName() string {
	return 时长卡代理价格表名
}
