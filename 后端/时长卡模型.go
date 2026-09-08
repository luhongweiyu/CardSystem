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

	// 旧版时长卡的永久卡使用 36500 天作为约定值。这里保留该约定，
	// 但不把永久卡字段混入纯点卡表。
	时长卡永久分钟    = int64(36500 * 24 * 60)
	时长卡最小时长分钟  = int64(5)
	时长卡最大单次数量  = 500
	时长卡最大备注字符数 = 500
)

// 时长卡记录完全独立于卡密表样式。时长卡的固定时长和到期时间属于卡
// 本身；点卡仍只使用 point_balance 和 point_device_session，两个模式不
// 共享任何会改变计费语义的字段。
type 时长卡记录 struct {
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
	AgentID            int        `gorm:"column:agent_id;not null;default:0;index" json:"agent_id"`
}

// 时长卡列表项不直接返回 needle，详情/登录接口才按权限返回它。
type 时长卡列表项 struct {
	Card               string     `json:"card"`
	CreateTime         time.Time  `json:"create_time"`
	UseTime            *time.Time `json:"use_time"`
	EndTime            *time.Time `json:"end_time"`
	Software           int        `json:"software"`
	CardState          int        `json:"card_state"`
	DurationMinutes    int64      `json:"duration_minutes"`
	LatestActivationAt *time.Time `json:"latest_activation_at"`
	Notes              string     `json:"notes"`
	ConfigContent      string     `json:"config_content"`
	AgentID            int        `json:"agent_id"`
	Online             bool       `json:"online"`
}

// 时长卡数据表名采用与点卡相同的管理员分表策略，但使用不同前缀，
// 确保同名点卡和时长卡可以同时存在且互不读取。
func 时长卡数据表名(admin string) (string, error) {
	if !验证管理员名称(admin) {
		return "", fmt.Errorf("管理员名称格式不正确")
	}
	return "duration_card_" + admin, nil
}
