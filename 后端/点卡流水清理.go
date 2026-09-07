package main

import (
	"fmt"
	"time"
)

const (
	// 点数流水只保留最近 30 天，避免审计表无限增长。
	点数流水保留天数 = 30
	// 分批删除可以缩短单次 DELETE 持锁时间，降低与流水写入之间的相互影响。
	点数流水清理批次大小 = 1000
	点数流水清理间隔   = 24 * time.Hour
)

// 点数流水清理截止时间统一使用日历天计算，调用方传入的当前时间不被修改。
func 点数流水清理截止时间(now time.Time) time.Time {
	return now.AddDate(0, 0, -点数流水保留天数)
}

// 清理过期点数流水只删除 point_ledger 中超过保留期的记录。
// 流水写入与余额更新仍由原有事务负责，本函数不触碰卡密余额、卡密记录或设备会话。
func 清理过期点数流水() error {
	if db_point_ledger == nil {
		return nil
	}
	截止时间 := 点数流水清理截止时间(time.Now())
	for {
		var ids []uint64
		if err := db_point_ledger.Select("id").Where("created_at < ?", 截止时间).
			Order("id ASC").Limit(点数流水清理批次大小).Pluck("id", &ids).Error; err != nil {
			return fmt.Errorf("查询过期点数流水失败: %w", err)
		}
		if len(ids) == 0 {
			return nil
		}
		result := db_point_ledger.Where("id IN ?", ids).Delete(&点数流水{})
		if result.Error != nil {
			return fmt.Errorf("删除过期点数流水失败: %w", result.Error)
		}
		// 极端情况下记录可能已被其他清理实例删除，避免无进展地重复查询。
		if result.RowsAffected == 0 {
			return nil
		}
	}
}

// 启动点数流水清理在服务启动时先执行一次，之后每天执行一次。
// 清理失败只记录日志，不影响登录、心跳和其他业务继续运行。
func 启动点数流水清理() {
	go func() {
		执行清理 := func() {
			if err := 清理过期点数流水(); err != nil {
				日志("log/启动记录.txt", err.Error())
			}
		}
		执行清理()
		ticker := time.NewTicker(点数流水清理间隔)
		defer ticker.Stop()
		for range ticker.C {
			执行清理()
		}
	}()
}
