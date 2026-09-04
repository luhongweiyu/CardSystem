package main

import "testing"

func Test规范化流水分页(t *testing.T) {
	page, pageSize := 规范化流水分页(0, 0)
	if page != 1 || pageSize != 50 {
		t.Fatalf("默认分页错误: page=%d pageSize=%d", page, pageSize)
	}

	page, pageSize = 规范化流水分页(2000000, 200)
	if page != 1000000 || pageSize != 200 {
		t.Fatalf("分页上限错误: page=%d pageSize=%d", page, pageSize)
	}
}
