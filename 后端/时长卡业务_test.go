package main

import (
	"strings"
	"testing"
	"time"
)

func Test时长卡数据表名与点卡表隔离(t *testing.T) {
	tableName, err := 时长卡数据表名("tester")
	if err != nil || tableName != "duration_card_tester" {
		t.Fatalf("时长卡表名错误: %q, %v", tableName, err)
	}
	if _, err := 时长卡数据表名("../tester"); err == nil {
		t.Fatal("非法管理员名不应生成动态表名")
	}
}

func Test规范化时长卡参数(t *testing.T) {
	valid, err := 规范化时长卡基础参数("tester", 时长卡生成请求{
		Software: 1, DurationMinutes: 1440, Num: 2, Random: true,
		LatestActivationMinutes: -1, Notes: "备注", ConfigContent: "配置",
	})
	if err != nil || valid.DurationMinutes != 1440 {
		t.Fatalf("合法时长卡参数被拒绝: %+v, %v", valid, err)
	}
	for _, value := range []int64{0, -2, 时长卡永久分钟 + 1} {
		_, err := 规范化时长卡基础参数("tester", 时长卡生成请求{Software: 1, DurationMinutes: value, Num: 1, Random: true, LatestActivationMinutes: -1})
		if err == nil {
			t.Fatalf("非法时长%d分钟未被拒绝", value)
		}
	}
	if _, err := 规范化时长卡基础参数("tester", 时长卡生成请求{Software: 1, DurationMinutes: 60, Num: 1, Random: true, LatestActivationMinutes: -1, Notes: strings.Repeat("a", 时长卡最大备注字符数+1)}); err == nil {
		t.Fatal("超长备注未被拒绝")
	}
}

func Test时长卡计算到期时间(t *testing.T) {
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.FixedZone("CST", 8*3600))
	row := 时长卡记录{DurationMinutes: 60}
	end, err := 时长卡计算到期时间(now, row)
	if err != nil || !end.Equal(now.Add(time.Hour)) {
		t.Fatalf("普通时长卡到期时间错误: %v, %v", end, err)
	}
	permanent := 时长卡记录{DurationMinutes: 时长卡永久分钟}
	end, err = 时长卡计算到期时间(now, permanent)
	if err != nil || end.Year() != 2099 {
		t.Fatalf("永久时长卡到期时间错误: %v, %v", end, err)
	}
	deadline := now.Add(-time.Minute)
	row.LatestActivationAt = &deadline
	if _, err := 时长卡计算到期时间(now, row); err == nil {
		t.Fatal("超过最晚激活时间仍被允许")
	}
	deadline = now.Add(30 * time.Minute)
	row.LatestActivationAt = &deadline
	end, err = 时长卡计算到期时间(now, row)
	if err != nil || !end.Equal(deadline) {
		t.Fatalf("临近最晚激活时间时应缩短到截止时间: %v, %v", end, err)
	}
}
