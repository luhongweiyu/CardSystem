package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Test读取管理员注册开关验证配置文件中的开关可以独立刷新，不依赖服务重启，
// 同时确保缺少配置文件时采用关闭注册的失败安全行为。
func Test读取管理员注册开关(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "config.yaml")
	if err := os.WriteFile(path, []byte("管理员注册:\n  启用: true\n"), 0600); err != nil {
		t.Fatalf("写入测试配置失败: %v", err)
	}
	enabled, err := 读取管理员注册开关(path)
	if err != nil || !enabled {
		t.Fatalf("应读取到启用注册，enabled=%v err=%v", enabled, err)
	}
	if enabled, err = 读取管理员注册开关(filepath.Join(directory, "missing.yaml")); err == nil || enabled {
		t.Fatalf("配置文件缺失时必须关闭注册，enabled=%v err=%v", enabled, err)
	}
}
