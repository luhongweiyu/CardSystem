package main

import (
	"strings"
	"time"
)

// 业务流水字段按 Tab 分隔；每个字段单独清理，避免备注或卡密内容破坏列结构。
func 业务流水文本(fields ...string) string {
	cleaned := make([]string, 0, len(fields))
	for _, field := range fields {
		cleaned = append(cleaned, 清理拒绝日志字段(field, 2000))
	}
	return strings.Join(cleaned, "\t")
}

// 记录时长卡业务流水只记录真实的时长变化，不记录查询、心跳和设备状态操作。
// 管理员日志保留完整租户视角；卡属于代理时，再同步一份到相关代理日志，
// 便于管理员和代理分别回溯同一笔时长变化。
func 记录时长卡业务流水(admin string, agentIDs []int, fields ...string) {
	admin = strings.TrimSpace(admin)
	if !验证管理员名称(admin) {
		return
	}
	日志("log/"+admin+time.Now().Format("200601"), 业务流水文本(fields...))

	seen := make(map[int]struct{}, len(agentIDs))
	for _, agentID := range agentIDs {
		if agentID <= 0 {
			continue
		}
		if _, exists := seen[agentID]; exists {
			continue
		}
		seen[agentID] = struct{}{}
		代理账号日志(agentID, fields...)
	}
}

func 业务流水时间(value time.Time) string {
	if value.IsZero() {
		return "无"
	}
	return value.Format(time.RFC3339)
}
