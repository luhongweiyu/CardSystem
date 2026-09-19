package main

import (
	"sort"
	"sync"
	"time"

	"gorm.io/gorm"
)

const 点卡复用候选缓存有效期 = time.Minute

type 点卡复用候选缓存键 struct {
	管理员 string
	卡密  string
}

type 点卡复用候选缓存条目 struct {
	sync.Mutex
	查询时间 time.Time
	候选   []点卡设备会话
	查询错误 error
}

var 全局点卡复用候选缓存 = struct {
	sync.Mutex
	数据 map[点卡复用候选缓存键]*点卡复用候选缓存条目
}{数据: make(map[点卡复用候选缓存键]*点卡复用候选缓存条目)}

// 调用方已锁定卡密行；成功返回后持有本卡候选锁，由调用方解锁。
// 全局锁只负责定位条目，不跨越数据库操作，不阻塞其他卡密的候选查询。
func 锁定点卡复用候选缓存(tx *gorm.DB, admin string, card string) (*点卡复用候选缓存条目, error) {
	键 := 点卡复用候选缓存键{管理员: admin, 卡密: card}
	var 缓存 *点卡复用候选缓存条目
	for {
		全局点卡复用候选缓存.Lock()
		缓存 = 全局点卡复用候选缓存.数据[键]
		if 缓存 == nil {
			缓存 = &点卡复用候选缓存条目{}
			全局点卡复用候选缓存.数据[键] = 缓存
		}
		全局点卡复用候选缓存.Unlock()
		缓存.Lock()
		全局点卡复用候选缓存.Lock()
		仍在缓存中 := 全局点卡复用候选缓存.数据[键] == 缓存
		全局点卡复用候选缓存.Unlock()
		if 仍在缓存中 {
			break
		}
		缓存.Unlock() // 等锁期间条目已被过期清理；重新定位，不执行数据库重试。
	}

	if 缓存.查询时间.IsZero() || time.Since(缓存.查询时间) >= 点卡复用候选缓存有效期 {
		设备列表, err := 查询点卡登录设备(tx, admin, card)
		if err == nil {
			// 查询结果原样保留，只按最新已知心跳排序；在线、过期等判断留到取用时。
			for i := range 设备列表 {
				合并点卡心跳缓存(&设备列表[i])
			}
			sort.Slice(设备列表, func(i, j int) bool {
				if 设备列表[i].LastHeartbeatAt.Equal(设备列表[j].LastHeartbeatAt) {
					return 设备列表[i].ID < 设备列表[j].ID
				}
				return 设备列表[i].LastHeartbeatAt.Before(设备列表[j].LastHeartbeatAt)
			})
		}
		// 空结果、候选耗尽和查询失败都保留时间，不因后续访问而延长或提前刷新。
		缓存.查询时间, 缓存.候选, 缓存.查询错误 = time.Now(), 设备列表, err
	}
	if 缓存.查询错误 != nil {
		err := 缓存.查询错误
		缓存.Unlock()
		return nil, err
	}
	return 缓存, nil
}

// 调用方持有候选锁。只移除已经检查过的首项，保留查询时间，避免下一次重新查库。
func (缓存 *点卡复用候选缓存条目) 剔除首个候选() {
	缓存.候选[0] = 点卡设备会话{}
	缓存.候选 = 缓存.候选[1:]
	if len(缓存.候选) == 0 {
		缓存.候选 = nil
	}
}

// 借用已有每分钟清理任务回收过期内存，不访问数据库；正在使用的条目跳过，
// 避免后台清理等待登录事务。下次请求也会自行检查有效期。
func 清理过期点卡复用候选缓存() {
	now := time.Now()
	全局点卡复用候选缓存.Lock()
	defer 全局点卡复用候选缓存.Unlock()
	for 键, 缓存 := range 全局点卡复用候选缓存.数据 {
		if !缓存.TryLock() {
			continue
		}
		if !缓存.查询时间.IsZero() && now.Sub(缓存.查询时间) >= 点卡复用候选缓存有效期 {
			delete(全局点卡复用候选缓存.数据, 键)
		}
		缓存.Unlock()
	}
}
