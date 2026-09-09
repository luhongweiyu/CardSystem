package main

import (
	"math"
	"testing"
)

func Test计算代理点卡费用(t *testing.T) {
	tests := []struct {
		name   string
		price  float64
		points int64
		count  int
		want   int64
	}{
		{name: "整数单价", price: 10, points: 2, count: 3, want: 60},
		{name: "不足一点向上取整", price: 0.25, points: 1, count: 1, want: 1},
		{name: "整批合并后取整", price: 0.25, points: 2, count: 2, want: 1},
		{name: "小数结果向上取整", price: 2.5, points: 3, count: 1, want: 8},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := 计算代理点卡费用(test.price, test.points, test.count)
			if err != nil {
				t.Fatalf("计算失败: %v", err)
			}
			if got != test.want {
				t.Fatalf("计算结果=%d，期望=%d", got, test.want)
			}
		})
	}

	invalidValues := []float64{-1, 0, math.NaN(), math.Inf(1)}
	for _, value := range invalidValues {
		if _, err := 计算代理点卡费用(value, 1, 1); err == nil {
			t.Fatalf("异常消费值 %v 应当被拒绝", value)
		}
	}
	if _, err := 计算代理点卡费用(1000000000, 1000000000, 1000); err == nil {
		t.Fatal("超出int64范围的代理费用应当被拒绝")
	}
}

func Test校验代理价格配置(t *testing.T) {
	validValues := []string{
		`{}`,
		`{"1":2.5}`,
		`{"1":0.01,"2":1000000000}`,
	}
	for _, value := range validValues {
		if err := 校验代理价格配置(value); err != nil {
			t.Fatalf("合法价格配置 %s 被拒绝: %v", value, err)
		}
	}

	invalidValues := []string{
		``,
		`not-json`,
		`{"abc":1}`,
		`{"01":1}`,
		`{"1":-1}`,
		`{"1":0}`,
		`{"1":null}`,
		`null`,
		`{"1":1.001}`,
		`{"1":{"point":10}}`,
		`{"1":1000000001}`,
	}
	for _, value := range invalidValues {
		if err := 校验代理价格配置(value); err == nil {
			t.Fatalf("异常价格配置 %s 应当被拒绝", value)
		}
	}
}

func Test解析代理价格忽略非规范软件ID(t *testing.T) {
	prices := 解析代理价格(`{"1":2.5,"01":9,"+2":8,"3":1.25}`)
	if len(prices) != 2 || prices[1] != 2.5 || prices[3] != 1.25 {
		t.Fatalf("代理价格解析结果错误: %#v", prices)
	}
}
