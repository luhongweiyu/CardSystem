package main

import (
	"testing"
	"time"
)

func Test登录会话管理器(t *testing.T) {
	manager := 登录会话管理器{会话: make(map[string]登录会话)}
	session := manager.创建会话("admin_test", false)

	if session.Token == "" {
		t.Fatal("创建会话后令牌不能为空")
	}
	if _, ok := manager.验证会话(session.Token, false); !ok {
		t.Fatal("管理员会话应当能够通过管理员类型校验")
	}
	if _, ok := manager.验证会话(session.Token, true); ok {
		t.Fatal("管理员会话不能被当作代理账号会话使用")
	}

	manager.删除账号会话("admin_test", false)
	if _, ok := manager.验证会话(session.Token, false); ok {
		t.Fatal("删除账号会话后旧令牌必须立即失效")
	}
}

func Test过期会话会被清理(t *testing.T) {
	manager := 登录会话管理器{会话: map[string]登录会话{
		"expired": {
			Token: "expired", AccountName: "admin_test",
			ExpiresAt: time.Now().Add(-time.Minute),
		},
	}}

	if _, ok := manager.验证会话("expired", false); ok {
		t.Fatal("过期会话不应通过验证")
	}
	manager.锁.RLock()
	_, exists := manager.会话["expired"]
	manager.锁.RUnlock()
	if exists {
		t.Fatal("验证过期会话后应从内存中删除该会话")
	}
}
