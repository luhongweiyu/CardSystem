package main

import (
	"testing"
	"time"
)

func Test点数流水清理截止时间(t *testing.T) {
	位置 := time.FixedZone("CST", 8*60*60)
	当前 := time.Date(2026, 9, 7, 12, 30, 0, 0, 位置)
	期望 := time.Date(2026, 8, 8, 12, 30, 0, 0, 位置)
	if got := 点数流水清理截止时间(当前); !got.Equal(期望) {
		t.Fatalf("流水清理截止时间错误: got=%v want=%v", got, 期望)
	}
}
