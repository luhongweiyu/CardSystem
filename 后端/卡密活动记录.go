package main

import (
	"container/list"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	卡密活动条数上限 = 20
	卡密活动保留时间 = 24 * time.Hour
	卡密活动分片数  = 32
	卡密活动分片容量 = 256
	卡密活动结果键  = "card_activity_result"
)

type 卡密活动键 struct {
	模式  string
	管理员 string
	卡密  string
	设备  string
}

type 卡密活动项 struct {
	Time      time.Time `json:"time"`
	IP        string    `json:"ip"`
	Needle    string    `json:"needle,omitempty"`
	Operation string    `json:"operation"`
	Success   bool      `json:"success"`
	Message   string    `json:"message,omitempty"`
}

type 卡密活动组 struct {
	键    卡密活动键
	记录   [卡密活动条数上限]卡密活动项
	下标   int
	数量   int
	最近活动 time.Time
}

type 卡密活动分片 struct {
	sync.Mutex
	索引 map[卡密活动键]*list.Element
	顺序 list.List
}

type 卡密活动存储 struct {
	分片 [卡密活动分片数]卡密活动分片
}

var 全局卡密活动 卡密活动存储

// 固定分片分摊不同设备的心跳锁竞争；每组固定 20 条，每片最多 256 组。
// 所有记录都在内存，不读取余额、不落库，也不随心跳缓存失效而删除。
func (store *卡密活动存储) 取分片(key 卡密活动键) *卡密活动分片 {
	hash := uint64(14695981039346656037)
	for _, value := range []string{key.模式, key.管理员, key.卡密, key.设备} {
		for index := 0; index < len(value); index++ {
			hash = (hash ^ uint64(value[index])) * 1099511628211
		}
		hash = (hash ^ 255) * 1099511628211
	}
	return &store.分片[hash%卡密活动分片数]
}

func (store *卡密活动存储) 写入(key 卡密活动键, item 卡密活动项) {
	// 错误提示截断并复制，不能通过长请求文本占用无上限内存。
	if item.Message != "" {
		message := []rune(item.Message)
		if len(message) > 128 {
			message = message[:128]
		}
		item.Message = string(message)
	}
	shard := store.取分片(key)
	shard.Lock()
	defer shard.Unlock()
	if shard.索引 == nil {
		shard.索引 = make(map[卡密活动键]*list.Element)
	}
	element := shard.索引[key]
	if element == nil {
		if len(shard.索引) >= 卡密活动分片容量 {
			oldest := shard.顺序.Front()
			delete(shard.索引, oldest.Value.(*卡密活动组).键)
			shard.顺序.Remove(oldest)
		}
		element = shard.顺序.PushBack(&卡密活动组{键: key})
		shard.索引[key] = element
	}
	group := element.Value.(*卡密活动组)
	group.记录[group.下标] = item
	group.下标 = (group.下标 + 1) % 卡密活动条数上限
	if group.数量 < 卡密活动条数上限 {
		group.数量++
	}
	if item.Time.After(group.最近活动) {
		group.最近活动 = item.Time
	}
	shard.顺序.MoveToBack(element)
}

func (store *卡密活动存储) 查询(key 卡密活动键, now time.Time) []卡密活动项 {
	shard := store.取分片(key)
	shard.Lock()
	defer shard.Unlock()
	result := make([]卡密活动项, 0, 卡密活动条数上限)
	element := shard.索引[key]
	if element == nil {
		return result
	}
	group := element.Value.(*卡密活动组)
	cutoff := now.Add(-卡密活动保留时间)
	// 返回副本，前端查询不会持有心跳写入所用的环形缓冲。
	for offset := 1; offset <= group.数量; offset++ {
		item := group.记录[(group.下标-offset+卡密活动条数上限)%卡密活动条数上限]
		if item.Time.After(cutoff) {
			result = append(result, item)
		}
	}
	return result
}

func (store *卡密活动存储) 清理(now time.Time) {
	cutoff := now.Add(-卡密活动保留时间)
	for index := range store.分片 {
		shard := &store.分片[index]
		shard.Lock()
		for key, element := range shard.索引 {
			group := element.Value.(*卡密活动组)
			if !group.最近活动.After(cutoff) {
				delete(shard.索引, key)
				shard.顺序.Remove(element)
				continue
			}
			for index := range group.记录 {
				if !group.记录[index].Time.After(cutoff) {
					group.记录[index] = 卡密活动项{}
				}
			}
		}
		shard.Unlock()
	}
}

type 卡密活动响应 struct {
	完成     bool
	成功     bool
	原因     string
	Needle string
}

// 只包裹通过卡端上下文和验签的登录/心跳业务；快缓存和数据库分支都只记一次。
// 写入放在处理结束后，不在数据库事务或心跳缓存锁内获取活动记录锁。
func 记录卡密活动(mode, operation string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		value, _ := ctx.Get("card")
		cardContext, valid := value.(卡密请求上下文)
		deviceID := ""
		if valid && mode == "point" {
			deviceID, valid = 规范化可选设备标识(input(ctx, "device_id"))
		}
		if !valid {
			ctx.Next()
			return
		}
		result := &卡密活动响应{}
		ctx.Set(卡密活动结果键, result)
		ctx.Next()
		if result.完成 {
			needle := strings.TrimSpace(result.Needle)
			if needle == "" {
				needle = strings.TrimSpace(input(ctx, "needle"))
			}
			if needleRunes := []rune(needle); len(needleRunes) > 6 {
				needle = string(needleRunes[:6])
			}
			全局卡密活动.写入(卡密活动键{mode, cardContext.Name, cardContext.Card, deviceID}, 卡密活动项{
				Time: time.Now(), IP: ctx.ClientIP(), Needle: needle,
				Operation: operation, Success: result.成功, Message: result.原因,
			})
		}
	}
}

func 设置卡密活动结果(ctx *gin.Context, data gin.H) {
	value, exists := ctx.Get(卡密活动结果键)
	if !exists {
		return
	}
	result := value.(*卡密活动响应)
	result.完成 = true
	if needle, ok := data["needle"].(string); ok {
		result.Needle = needle
	}
	result.成功, _ = data["state"].(bool)
	if !result.成功 && data["msg"] != nil {
		result.原因 = fmt.Sprint(data["msg"])
	}
}

func 查询点卡活动(admin, card, deviceID string) ([]卡密活动项, error) {
	deviceID, valid := 规范化可选设备标识(deviceID)
	if !valid {
		return nil, fmt.Errorf("device_id格式不正确")
	}
	return 全局卡密活动.查询(卡密活动键{"point", admin, strings.ToLower(strings.TrimSpace(card)), deviceID}, time.Now()), nil
}

func 查询时长卡活动(admin, card string) []卡密活动项 {
	return 全局卡密活动.查询(卡密活动键{"duration", admin, strings.ToLower(strings.TrimSpace(card)), ""}, time.Now())
}
