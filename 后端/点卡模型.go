package main

import "time"

const (
	// 点数流水类型只表示余额的真实变化方向。
	点数事件_扣点 = "debit"
	点数事件_补点 = "credit"

	默认登录周期秒 = int64(3600)
	默认心跳周期秒 = int64(300)
	最大计费周期秒 = int64(365 * 24 * 60 * 60)
	最大心跳周期秒 = int64(24 * 60 * 60)
	最大单次点数  = int64(1000000000)
)

// 点卡周期价格定义一个软件允许客户端请求的计费周期及其点数价格。
// 同一管理员、软件、周期只能有一条记录；is_default 仅是与软件默认周期同步的
// 管理端展示标记，无周期参数的登录始终读取 software.default_period_seconds。
type 点卡周期价格 struct {
	ID            uint      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Admin         string    `gorm:"column:admin;size:32;not null;uniqueIndex:uk_period_price,priority:1" json:"admin"`
	Software      int       `gorm:"column:software;not null;uniqueIndex:uk_period_price,priority:2" json:"software"`
	PeriodSeconds int64     `gorm:"column:period_seconds;not null;uniqueIndex:uk_period_price,priority:3" json:"period_seconds"`
	Cost          int64     `gorm:"column:cost;not null" json:"cost"`
	IsDefault     bool      `gorm:"column:is_default;not null;default:false" json:"is_default"`
	Enabled       bool      `gorm:"column:enabled;not null;default:true" json:"enabled"`
	CreatedAt     time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// 点数流水一行只对应一次真实余额变化。
// change 为有符号值，扣点为负、补点为正；balance_after 是变动完成后的余额。
// 设备 ID 和别名快照统一写入 remark，避免流水表承担设备状态职责。
type 点数流水 struct {
	ID            uint64    `gorm:"column:id;primaryKey;autoIncrement;index:idx_ledger_admin_id,priority:2;index:idx_ledger_card_id,priority:3;index:idx_ledger_software_id,priority:3" json:"id"`
	Admin         string    `gorm:"column:admin;size:32;not null;index:idx_ledger_admin_id,priority:1;index:idx_ledger_card_id,priority:1;index:idx_ledger_software_id,priority:1" json:"admin"`
	Card          string    `gorm:"column:card;size:63;not null;index:idx_ledger_card_id,priority:2" json:"card"`
	Software      int       `gorm:"column:software;not null;index:idx_ledger_software_id,priority:2" json:"software"`
	EventType     string    `gorm:"column:event_type;size:16;not null" json:"event_type"`
	Change        int64     `gorm:"column:change;not null" json:"change"`
	BalanceBefore int64     `gorm:"column:balance_before;not null" json:"balance_before"`
	BalanceAfter  int64     `gorm:"column:balance_after;not null" json:"balance_after"`
	Remark        string    `gorm:"column:remark;type:text" json:"remark"`
	CreatedAt     time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

// 点卡设备会话保存一张卡在一台设备上的当前授权周期。
// 一张卡可同时拥有多台设备；设备别名不参与唯一性判断，也不要求唯一。
type 点卡设备会话 struct {
	ID                   uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Admin                string    `gorm:"column:admin;size:32;not null;uniqueIndex:uk_device_session,priority:1" json:"admin"`
	Card                 string    `gorm:"column:card;size:63;not null;uniqueIndex:uk_device_session,priority:2" json:"card"`
	Software             int       `gorm:"column:software;not null;uniqueIndex:uk_device_session,priority:3" json:"software"`
	DeviceID             string    `gorm:"column:device_id;size:128;not null;uniqueIndex:uk_device_session,priority:4" json:"device_id"`
	DeviceAlias          string    `gorm:"column:device_alias;size:64" json:"device_alias"`
	Needle               string    `gorm:"column:needle;size:64;not null;uniqueIndex:uk_device_needle" json:"-"`
	RenewalPeriodSeconds int64     `gorm:"column:renewal_period_seconds;not null" json:"renewal_period_seconds"`
	AuthorizedUntil      time.Time `gorm:"column:authorized_until;not null;index" json:"authorized_until"`
	// last_heartbeat_at 每次心跳都会更新，仅参与已锁定单行的计费判断，
	// 没有按该字段扫描的查询，因此不建立高写入成本的索引。
	LastHeartbeatAt time.Time `gorm:"column:last_heartbeat_at;not null" json:"last_heartbeat_at"`
	CreatedAt       time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// 点卡扣费结果供登录和心跳接口返回。Charged=false 表示本次请求在当前
// 授权周期内，没有发生余额变化。
type 点卡扣费结果 struct {
	Charged         bool      `json:"charged"`
	Cost            int64     `json:"cost"`
	Balance         int64     `json:"balance"`
	AuthorizedUntil time.Time `json:"authorized_until"`
	LedgerID        uint64    `json:"ledger_id,omitempty"`
}

type 点卡登录结果 struct {
	Card             卡密表样式  `json:"-"`
	Session          点卡设备会话 `json:"-"`
	Charge           点卡扣费结果 `json:"-"`
	PeriodSeconds    int64  `json:"period_seconds"`
	HeartbeatSeconds int64  `json:"heartbeat_seconds"`
}

// 点卡心跳结果包含更新后的会话和可能发生的续费结果。
type 点卡心跳结果 struct {
	Session          点卡设备会话
	Charge           点卡扣费结果
	HeartbeatSeconds int64
}
