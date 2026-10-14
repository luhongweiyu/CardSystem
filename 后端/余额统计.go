package main

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const 余额统计表名 = "balance_hourly_stat"

// 按变动发生时的归属保存历史；删除卡密、软件或代理不会删除统计。
// meta 行只保存首次启用时间，与统计共用一张表，不参与金额查询。
type 余额小时统计 struct {
	Admin     string `gorm:"size:32;primaryKey;autoIncrement:false;index:idx_balance_scope,priority:1;index:idx_balance_agent,priority:1"`
	Kind      string `gorm:"size:8;primaryKey;autoIncrement:false;index:idx_balance_scope,priority:2;index:idx_balance_agent,priority:2"`
	AgentID   int    `gorm:"primaryKey;autoIncrement:false;index:idx_balance_agent,priority:3"`
	Software  int    `gorm:"primaryKey;autoIncrement:false"`
	Card      string `gorm:"size:63;primaryKey;autoIncrement:false"`
	Hour      int64  `gorm:"primaryKey;autoIncrement:false;index:idx_balance_scope,priority:3;index:idx_balance_agent,priority:4"`
	Increase  uint64 `gorm:"not null;default:0"`
	Decrease  uint64 `gorm:"not null;default:0"`
	StartedAt int64  `gorm:"not null;default:0"`
}

func (余额小时统计) TableName() string { return 余额统计表名 }

type 余额统计键 struct {
	Admin    string
	Kind     string
	AgentID  int
	Software int
	Card     string
	Hour     int64
}

var 全局余额统计 = struct {
	sync.Mutex
	待保存  map[余额统计键]余额小时统计
	开始时间 int64 // 启动时初始化，提供请求服务后只读。
}{待保存: make(map[余额统计键]余额小时统计)}

type 余额统计上下文键 struct{}

// 沿用原有业务事务，仅暂存统计增量；提交成功后才发布到内存。
// 统计不在业务事务内写数据库，失败不会改变业务结果，也不为每笔业务启动协程。
func 执行余额事务(fn func(*gorm.DB) error) error {
	var pending []余额小时统计
	err := db.Transaction(func(tx *gorm.DB) error {
		ctx := context.WithValue(tx.Statement.Context, 余额统计上下文键{}, &pending)
		return fn(tx.WithContext(ctx))
	})
	if err == nil && len(pending) > 0 {
		全局余额统计.Lock()
		defer 全局余额统计.Unlock()
		for _, row := range pending {
			key := 余额统计键{row.Admin, row.Kind, row.AgentID, row.Software, row.Card, row.Hour}
			current, exists := 全局余额统计.待保存[key]
			// 统计允许丢失；限制未保存对象数，数据库故障时也不会无限占用内存。
			if !exists {
				if len(全局余额统计.待保存) >= 100000 {
					continue
				}
				current = row
			} else {
				if math.MaxUint64-current.Increase < row.Increase || math.MaxUint64-current.Decrease < row.Decrease {
					continue
				}
				current.Increase += row.Increase
				current.Decrease += row.Decrease
			}
			全局余额统计.待保存[key] = current
		}
	}
	return err
}

func 暂存余额统计(tx *gorm.DB, admin, kind string, agentID, software int, card string, change int64, now time.Time) {
	if change == 0 || 全局余额统计.开始时间 == 0 {
		return
	}
	pending, ok := tx.Statement.Context.Value(余额统计上下文键{}).(*[]余额小时统计)
	if !ok {
		return
	}
	row := 余额小时统计{Admin: admin, Kind: kind, AgentID: agentID, Software: software, Card: card, Hour: now.Truncate(time.Hour).Unix()}
	if change > 0 {
		row.Increase = uint64(change)
	} else {
		row.Decrease = uint64(-(change + 1)) + 1 // 包含最小 int64，避免取负溢出。
	}
	*pending = append(*pending, row)
}

func 初始化余额统计() error {
	if err := db.AutoMigrate(&余额小时统计{}); err != nil {
		return err
	}
	marker := 余额小时统计{Kind: "meta", StartedAt: time.Now().Unix()}
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&marker).Error; err != nil {
		return err
	}
	if err := db.Where("kind = ? AND admin = ?", "meta", "").First(&marker).Error; err != nil {
		return err
	}
	全局余额统计.开始时间 = marker.StartedAt
	return nil
}

// 先交换缓存再写库，数据库耗时不占用内存锁。每批只尝试一次，避免不确定
// 是否提交的数据库错误在重试时重复累加；失败批次按约定丢弃并记录错误。
func 保存余额统计缓存() {
	全局余额统计.Lock()
	pending := 全局余额统计.待保存
	全局余额统计.待保存 = make(map[余额统计键]余额小时统计)
	全局余额统计.Unlock()
	if len(pending) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	batch := make([]余额小时统计, 0, 200)
	write := func() {
		err := db.WithContext(ctx).Session(&gorm.Session{SkipDefaultTransaction: true}).
			Clauses(clause.OnConflict{DoUpdates: clause.Assignments(map[string]interface{}{
				"increase": gorm.Expr("`increase` + VALUES(`increase`)"),
				"decrease": gorm.Expr("`decrease` + VALUES(`decrease`)"),
			})}).Create(&batch).Error
		if err != nil {
			日志("log/启动记录.txt", fmt.Sprintf("余额统计保存失败，本批%d条已丢弃: %v", len(batch), err))
		}
		batch = batch[:0]
	}
	for _, row := range pending {
		batch = append(batch, row)
		if len(batch) == cap(batch) {
			write()
			if ctx.Err() != nil {
				return
			}
		}
	}
	if len(batch) > 0 {
		write()
	}
}

// 每分钟保存一次统计；进程退出时不额外保存，尚未落库的增量允许丢失。
func 启动余额统计() {
	if err := 初始化余额统计(); err != nil {
		日志("log/启动记录.txt", "余额统计暂不可用:"+err.Error())
		return
	}
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			保存余额统计缓存()
		}
	}()
}
