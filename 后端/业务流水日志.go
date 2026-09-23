package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// 业务流水字段按 Tab 分隔；只清理控制字符，不截断卡密清单等回溯信息。
func 业务流水文本(fields ...string) string {
	cleaned := make([]string, 0, len(fields))
	for _, field := range fields {
		cleaned = append(cleaned, 清理拒绝日志字段(field, 0))
	}
	return strings.Join(cleaned, "\t")
}

// 记录管理员和代理可回溯的业务变化。管理员月志包含代理 ID；代理日志只写
// 自己相关的记录。调用方应在业务事务提交后调用，避免回滚操作留下虚假日志。
func 记录管理员代理业务流水(admin string, agentIDs []int, fields ...string) {
	admin = strings.TrimSpace(admin)
	if !验证管理员名称(admin) {
		return
	}

	seen := make(map[int]struct{}, len(agentIDs))
	uniqueAgentIDs := make([]int, 0, len(agentIDs))
	for _, agentID := range agentIDs {
		if agentID <= 0 {
			continue
		}
		if _, exists := seen[agentID]; exists {
			continue
		}
		seen[agentID] = struct{}{}
		uniqueAgentIDs = append(uniqueAgentIDs, agentID)
	}
	sort.Ints(uniqueAgentIDs)

	adminFields := append([]string(nil), fields...)
	if len(uniqueAgentIDs) > 0 {
		ids := make([]string, 0, len(uniqueAgentIDs))
		for _, agentID := range uniqueAgentIDs {
			ids = append(ids, fmt.Sprint(agentID))
		}
		adminFields = append(adminFields, "代理ID:"+strings.Join(ids, ","))
	}
	now := time.Now()
	日志("log/"+admin+now.Format("200601"), 业务流水文本(adminFields...))
	for _, agentID := range uniqueAgentIDs {
		代理账号日志(agentID, fields...)
	}
}

// 记录代理余额充值汇总，保留旧系统管理员集中回查所有代理充值的文件入口。
func 记录代理余额充值汇总(admin string, agentID int, fields ...string) {
	if !验证管理员名称(admin) || agentID <= 0 {
		return
	}
	allFields := append([]string{"管理员:" + admin, fmt.Sprintf("代理ID:%d", agentID)}, fields...)
	日志("log/子账号充值记录_"+time.Now().Format("200601"), 业务流水文本(allFields...))
}

func 业务流水时间(value time.Time) string {
	if value.IsZero() {
		return "无"
	}
	return value.Format(time.RFC3339)
}
