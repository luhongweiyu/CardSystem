package main

import (
	"sort"
	"strings"
	"sync"
)

type 时长卡业务锁键 struct {
	管理员 string
	卡密  string
}

type 时长卡业务锁条目 struct {
	锁   sync.Mutex
	引用数 int
}

// 时长卡每个“管理员 + 卡密”只使用一把互斥锁。锁表本身只保护查找，
// 不跨数据库操作；不同卡密可以并行，同一张卡从缓存同步到业务提交保持串行。
var 全局时长卡业务锁 = struct {
	sync.Mutex
	数据 map[时长卡业务锁键]*时长卡业务锁条目
}{数据: make(map[时长卡业务锁键]*时长卡业务锁条目)}

func 获取时长卡业务锁(admin, card string) (时长卡业务锁键, *时长卡业务锁条目) {
	key := 时长卡业务锁键{管理员: strings.TrimSpace(admin), 卡密: strings.ToLower(strings.TrimSpace(card))}
	全局时长卡业务锁.Lock()
	defer 全局时长卡业务锁.Unlock()
	lock := 全局时长卡业务锁.数据[key]
	if lock == nil {
		lock = &时长卡业务锁条目{}
		全局时长卡业务锁.数据[key] = lock
	}
	lock.引用数++
	return key, lock
}

// 锁定时长卡业务由业务入口在读写缓存、开启事务之前调用，返回值必须释放。
// 多卡一次性按规范化后的卡密排序加锁，避免两个互充或批量请求反向等待。
func 锁定时长卡业务(admin string, cards ...string) func() {
	admin = strings.TrimSpace(admin)
	keys := make([]string, 0, len(cards))
	seen := make(map[string]struct{}, len(cards))
	for _, card := range cards {
		card = strings.ToLower(strings.TrimSpace(card))
		if _, exists := seen[card]; !exists {
			seen[card] = struct{}{}
			keys = append(keys, card)
		}
	}
	sort.Strings(keys)
	type 已锁定条目 struct {
		键  时长卡业务锁键
		条目 *时长卡业务锁条目
	}
	locks := make([]已锁定条目, len(keys))
	for i, card := range keys {
		key, lock := 获取时长卡业务锁(admin, card)
		locks[i] = 已锁定条目{键: key, 条目: lock}
		lock.锁.Lock()
	}
	return func() {
		for i := len(locks) - 1; i >= 0; i-- {
			lock := locks[i]
			lock.条目.锁.Unlock()
			全局时长卡业务锁.Lock()
			lock.条目.引用数--
			if lock.条目.引用数 == 0 && 全局时长卡业务锁.数据[lock.键] == lock.条目 {
				delete(全局时长卡业务锁.数据, lock.键)
			}
			全局时长卡业务锁.Unlock()
		}
	}
}
