package main

import (
	"strconv"
	"strings"
)

// 按计费时长显示非零单位，不把一天强制展开成 1440 分钟。
func 格式化授权时长(seconds int64) string {
	if seconds <= 0 {
		return "0秒"
	}
	units := []struct {
		seconds int64
		name    string
	}{{86400, "天"}, {3600, "小时"}, {60, "分"}, {1, "秒"}}
	var result strings.Builder
	for _, unit := range units {
		if count := seconds / unit.seconds; count > 0 {
			result.WriteString(strconv.FormatInt(count, 10))
			result.WriteString(unit.name)
		}
		seconds %= unit.seconds
	}
	return result.String()
}
