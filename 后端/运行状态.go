package main

import "sync"

// 用户运行状态集中维护仅在当前进程内有效的缓存数据。
//
// 这些数据会被 Gin 的多个请求协程并发访问，因此所有读写必须经过本类型
// 提供的方法，禁止在业务代码中直接操作内部 map。数据库仍然是用户设置的
// 最终数据源，本缓存仅用于减少每次卡密请求都查询数据库的开销。
type 用户运行状态 struct {
	锁      sync.RWMutex
	用户设置   map[string]user_info
	用户ID映射 map[int]string
	每小时请求数 map[string]int
}

var 全局_运行状态 = 用户运行状态{
	用户设置:   make(map[string]user_info),
	用户ID映射: make(map[int]string),
	每小时请求数: make(map[string]int),
}

// 保存用户设置会同时更新用户名和用户 ID 两种索引，避免两份缓存出现不一致。
func (s *用户运行状态) 保存用户设置(info user_info) {
	s.锁.Lock()
	defer s.锁.Unlock()

	s.用户设置[info.Name] = info
	s.用户ID映射[info.ID] = info.Name
}

// 读取用户设置返回缓存值及是否存在，不向调用方暴露内部 map。
func (s *用户运行状态) 读取用户设置(name string) (user_info, bool) {
	s.锁.RLock()
	defer s.锁.RUnlock()

	info, ok := s.用户设置[name]
	return info, ok
}

// 根据用户 ID 读取用户名，供 center_id 兼容接口使用。
func (s *用户运行状态) 读取用户名(id int) string {
	s.锁.RLock()
	defer s.锁.RUnlock()

	return s.用户ID映射[id]
}

// 增加请求次数在同一把锁内完成读取、上限判断和递增，防止并发漏计。
func (s *用户运行状态) 增加请求次数(name string, limit int) bool {
	s.锁.Lock()
	defer s.锁.Unlock()

	count := s.每小时请求数[name]
	if count >= limit {
		return false
	}
	s.每小时请求数[name] = count + 1
	return true
}

// 读取请求次数用于管理端展示当前小时的 API 使用量。
func (s *用户运行状态) 读取请求次数(name string) int {
	s.锁.RLock()
	defer s.锁.RUnlock()

	return s.每小时请求数[name]
}

// 清空请求次数由整点任务调用，为下一小时重新计数。
func (s *用户运行状态) 清空请求次数() {
	s.锁.Lock()
	defer s.锁.Unlock()

	for name := range s.每小时请求数 {
		delete(s.每小时请求数, name)
	}
}
