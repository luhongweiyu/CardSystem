package main

import (
	"strings"
	"testing"
)

func Test验证管理员名称(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{name: "abc", want: true},
		{name: "user_001", want: true},
		{name: "ab", want: false},
		{name: "../admin", want: false},
		{name: "带空格 user", want: false},
	}
	for _, test := range tests {
		if got := 验证管理员名称(test.name); got != test.want {
			t.Fatalf("验证管理员名称(%q)=%v，期望%v", test.name, got, test.want)
		}
	}
}

func Test规范化设备标识(t *testing.T) {
	deviceID, ok := 规范化设备标识("  7F6A0A38-2B1C-4F21  ")
	if !ok || deviceID != "7f6a0a38-2b1c-4f21" {
		t.Fatalf("设备标识规范化结果错误: %q, %v", deviceID, ok)
	}
	if _, ok := 规范化设备标识("short"); ok {
		t.Fatal("过短设备标识不应通过校验")
	}
	if _, ok := 规范化设备标识("设备-编号-123"); ok {
		t.Fatal("包含非白名单字符的设备标识不应通过校验")
	}
}

func Test规范化设备别名(t *testing.T) {
	alias, ok := 规范化设备别名("  办公室电脑  ")
	if !ok || alias != "办公室电脑" {
		t.Fatalf("设备别名规范化结果错误: %q, %v", alias, ok)
	}
	if alias, ok := 规范化设备别名(""); !ok || alias != "" {
		t.Fatal("空设备别名应当作为可选字段通过")
	}
	if _, ok := 规范化设备别名("名称\n伪造"); ok {
		t.Fatal("包含换行的设备别名不应通过校验")
	}
	if _, ok := 规范化设备别名(strings.Repeat("a", 65)); ok {
		t.Fatal("超过64个字符的设备别名不应通过校验")
	}
}

func Test规范化可显示文本按字符限制(t *testing.T) {
	value, ok := 规范化可显示文本("  中文备注  ", 4)
	if !ok || value != "中文备注" {
		t.Fatalf("中文展示文本规范化错误: value=%q, ok=%v", value, ok)
	}
	if _, ok := 规范化可显示文本("五个字符啊", 4); ok {
		t.Fatal("超过字符上限的展示文本不应通过校验")
	}
	if _, ok := 规范化可显示文本("正常\t伪造", 20); ok {
		t.Fatal("包含制表符的展示文本不应通过校验")
	}
}

func Test规范化点卡扣费参数(t *testing.T) {
	params := 点卡扣费参数{
		Admin: " tester ", Card: " ABC12345 ", Software: 1,
		DeviceID: " 7F6A0A38-2B1C-4F21 ", DeviceAlias: " 办公室电脑 ",
	}
	if err := 规范化点卡扣费参数(&params); err != nil {
		t.Fatalf("合法扣费参数不应失败: %v", err)
	}
	if params.Admin != "tester" || params.Card != "abc12345" || params.DeviceID != "7f6a0a38-2b1c-4f21" || params.DeviceAlias != "办公室电脑" {
		t.Fatalf("扣费参数规范化结果错误: %+v", params)
	}

	invalid := 点卡扣费参数{Admin: "tester", Card: "abc12345", Software: 1, PeriodSeconds: -1}
	if err := 规范化点卡扣费参数(&invalid); err == nil {
		t.Fatal("负数计费周期应当被拒绝")
	}
}

func Test安全随机字符串(t *testing.T) {
	first := GetRandomString(32, "a")
	second := GetRandomString(32, "a")
	if len(first) != 32 || strings.ToLower(first) != first {
		t.Fatalf("小写随机字符串格式错误: %q", first)
	}
	if first == second {
		t.Fatal("连续生成的安全随机字符串不应相同")
	}
}

func Test清理拒绝日志字段(t *testing.T) {
	actual := 清理拒绝日志字段("card\r\nforged-entry", 8)
	if strings.ContainsAny(actual, "\r\n\t") || actual != "card  fo..." {
		t.Fatalf("拒绝日志字段清理结果错误: %q", actual)
	}
}

func Test密码哈希验证(t *testing.T) {
	hash, err := 生成密码哈希("correct-password")
	if err != nil {
		t.Fatalf("生成密码哈希失败: %v", err)
	}
	if !验证密码(hash, "correct-password") {
		t.Fatal("正确密码未通过 bcrypt 验证")
	}
	if 验证密码(hash, "wrong-password") {
		t.Fatal("错误密码不应通过 bcrypt 验证")
	}
	if 验证密码("", "correct-password") {
		t.Fatal("空密码哈希不应通过验证")
	}
	if err := 校验新密码("12345"); err == nil {
		t.Fatal("少于6个字节的新密码应当被拒绝")
	}
	if err := 校验新密码(strings.Repeat("a", 73)); err == nil {
		t.Fatal("超过72个字节的新密码应当被拒绝")
	}
	if err := 校验新密码("安全password"); err != nil {
		t.Fatalf("合法新密码被拒绝: %v", err)
	}
}
