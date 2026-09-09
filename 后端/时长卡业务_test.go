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
	row := 时长卡表样式{DurationMinutes: 60}
	end, err := 时长卡计算到期时间(now, row)
	if err != nil || !end.Equal(now.Add(time.Hour)) {
		t.Fatalf("普通时长卡到期时间错误: %v, %v", end, err)
	}
	permanent := 时长卡表样式{DurationMinutes: 时长卡永久分钟}
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

func Test计算时长卡代理费用(t *testing.T) {
	anchors := []时长卡代理价格{
		{DurationMinutes: 2 * 24 * 60, Price: 2000, Enabled: true},
		{DurationMinutes: 5 * 24 * 60, Price: 3600, Enabled: true},
	}
	quote, err := 计算时长卡代理费用(anchors, 3*24*60, 1)
	if err != nil {
		t.Fatalf("计算时长卡代理价格失败: %v", err)
	}
	// 2天锚点的平均价为10点/天，高于5天锚点的7.2点/天，
	// 因此3天应按2天锚点折算为30点。
	if quote.Charge != 30 || quote.PricingMode != "between" || quote.RateSourceDurationMinutes != 2*24*60 {
		t.Fatalf("较高平均价选择错误: %+v", quote)
	}

	quote, err = 计算时长卡代理费用(anchors, 5*24*60, 2)
	if err != nil || quote.Charge != 72 || quote.PricingMode != "exact" {
		t.Fatalf("精确锚点计价错误: %+v, %v", quote, err)
	}

	upperMoreExpensive := []时长卡代理价格{
		{DurationMinutes: 2 * 24 * 60, Price: 1000, Enabled: true},
		{DurationMinutes: 5 * 24 * 60, Price: 10000, Enabled: true},
	}
	quote, err = 计算时长卡代理费用(upperMoreExpensive, 3*24*60, 1)
	if err != nil || quote.Charge != 60 || quote.RateSourceDurationMinutes != 5*24*60 {
		t.Fatalf("较高上侧平均价选择错误: %+v, %v", quote, err)
	}
}

func Test时长卡代理价格边界和向上取整(t *testing.T) {
	anchors := []时长卡代理价格{
		{DurationMinutes: 2 * 24 * 60, Price: 1010, Enabled: true},
		{DurationMinutes: 5 * 24 * 60, Price: 2500, Enabled: true},
	}
	quote, err := 计算时长卡代理费用(anchors, 3*24*60, 2)
	if err != nil || quote.Charge != 31 {
		t.Fatalf("批量金额向上取整错误: %+v, %v", quote, err)
	}
	for _, minutes := range []int64{24 * 60, 6 * 24 * 60} {
		if _, err := 计算时长卡代理费用(anchors, minutes, 1); err == nil {
			t.Fatalf("范围外时长%d分钟未被拒绝", minutes)
		}
	}

	anchors[0].Enabled = false
	if _, err := 计算时长卡代理费用(anchors, 3*24*60, 1); err == nil {
		t.Fatal("没有左右启用锚点时不应允许折算")
	}
}

func Test规范化时长卡代理价格输入(t *testing.T) {
	prices, err := 规范化时长卡代理价格输入([]时长卡代理价格输入{
		{DurationMinutes: 5 * 24 * 60, Price: 10},
		{DurationMinutes: 2 * 24 * 60, Price: 20},
	})
	if err != nil || len(prices) != 2 || prices[0].DurationMinutes != 2*24*60 || prices[0].Price != 2000 {
		t.Fatalf("价格输入规范化错误: %+v, %v", prices, err)
	}
	if _, err := 规范化时长卡代理价格输入([]时长卡代理价格输入{
		{DurationMinutes: 60, Price: 1},
		{DurationMinutes: 60, Price: 2},
	}); err == nil {
		t.Fatal("重复价格锚点未被拒绝")
	}
}
