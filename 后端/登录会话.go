package main

import (
	"sync"
	"time"
)

// 登录会话有效期控制管理员和代理账号令牌的最长存活时间；令牌只保存在当前进程内。
const 登录会话有效期 = 12 * time.Hour

// 登录会话保存令牌对应的账号和角色，角色信息用于阻止管理员与代理账号互相越权。
type 登录会话 struct {
	Token       string
	AccountName string
	IsAgent     bool
	ExpiresAt   time.Time
}

// 登录会话管理器集中管理内存会话，并用读写锁保证并发登录、校验和退出安全。
type 登录会话管理器 struct {
	锁  sync.RWMutex
	会话 map[string]登录会话
}

var 全局_登录会话 = 登录会话管理器{会话: make(map[string]登录会话)}

// 创建会话使用安全随机令牌。会话仅保存在当前进程，服务重启后重新登录即可。
func (manager *登录会话管理器) 创建会话(accountName string, isAgent bool) 登录会话 {
	session := 登录会话{
		Token: GetRandomString(48, "a"), AccountName: accountName,
		IsAgent: isAgent, ExpiresAt: time.Now().Add(登录会话有效期),
	}
	manager.锁.Lock()
	// 创建新会话时顺带清理过期项，避免长期运行且用户不再访问时残留无效令牌。
	manager.清理过期会话_已加锁(time.Now())
	manager.会话[session.Token] = session
	manager.锁.Unlock()
	return session
}

// 验证会话会顺带清理当前命中的过期令牌，避免无效会话继续占用内存。
func (manager *登录会话管理器) 验证会话(token string, isAgent bool) (登录会话, bool) {
	manager.锁.RLock()
	session, ok := manager.会话[token]
	manager.锁.RUnlock()
	if !ok || session.IsAgent != isAgent {
		return 登录会话{}, false
	}
	if !session.ExpiresAt.After(time.Now()) {
		manager.删除会话(token)
		return 登录会话{}, false
	}
	return session, true
}

func (manager *登录会话管理器) 删除会话(token string) {
	manager.锁.Lock()
	delete(manager.会话, token)
	manager.锁.Unlock()
}

// 删除账号会话用于退出登录或修改密码后立即使旧令牌失效。
func (manager *登录会话管理器) 删除账号会话(accountName string, isAgent bool) {
	manager.锁.Lock()
	defer manager.锁.Unlock()

	for token, session := range manager.会话 {
		if session.AccountName == accountName && session.IsAgent == isAgent {
			delete(manager.会话, token)
		}
	}
}

// 清理过期会话供定时任务调用，使长期没有新登录请求的进程也能及时释放过期令牌。
func (manager *登录会话管理器) 清理过期会话(now time.Time) {
	manager.锁.Lock()
	manager.清理过期会话_已加锁(now)
	manager.锁.Unlock()
}

// 清理过期会话_已加锁只能在持有写锁时调用。
func (manager *登录会话管理器) 清理过期会话_已加锁(now time.Time) {
	for token, session := range manager.会话 {
		if !session.ExpiresAt.After(now) {
			delete(manager.会话, token)
		}
	}
}
