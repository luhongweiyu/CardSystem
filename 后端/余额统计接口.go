package main

import (
	"context"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

type 余额统计查询 struct {
	Kind        string `json:"kind"`
	Card        string `json:"card"`
	Software    int    `json:"software"`
	AgentID     int    `json:"agent_id"`
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
	Granularity string `json:"granularity"`
}

type 余额统计曲线点 struct {
	Time     int64  `json:"time"`
	Increase string `json:"increase"`
	Decrease string `json:"decrease"`
}

type 余额统计响应 struct {
	Increase  string    `json:"increase"`
	Decrease  string    `json:"decrease"`
	Trend     []余额统计曲线点 `json:"trend"`
	StartedAt int64     `json:"started_at"`
}

type 余额统计查询键 struct {
	Admin string
	余额统计查询
}

type 余额统计缓存项 struct {
	Result 余额统计响应
	Until  time.Time
}

var 余额统计查询缓存 = struct {
	sync.Mutex
	Items map[余额统计查询键]余额统计缓存项
}{Items: make(map[余额统计查询键]余额统计缓存项)}

func 管理员_查询余额统计(ctx *gin.Context) {
	account, ok := 管理员_取账号信息(ctx)
	if !ok {
		失败提示管理端(ctx, "登录状态错误")
		return
	}
	查询余额统计(ctx, account.Name, 0)
}

func 代理账号_查询余额统计(ctx *gin.Context) {
	account := 代理账号_取账号信息(ctx)
	查询余额统计(ctx, account.Admin, account.ID)
}

func 查询余额统计(ctx *gin.Context, admin string, agentID int) {
	if 全局余额统计.开始时间 == 0 {
		失败提示管理端(ctx, "统计暂不可用")
		return
	}
	var request 余额统计查询
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil {
		失败提示管理端(ctx, "统计参数不正确")
		return
	}
	if request.Kind != "point" && request.Kind != "agent" {
		失败提示管理端(ctx, "请选择点卡或代理账户")
		return
	}
	if request.Software < 0 || request.AgentID < 0 {
		失败提示管理端(ctx, "软件或账户参数不正确")
		return
	}
	request.Card = strings.ToLower(strings.TrimSpace(request.Card))
	if request.Card != "" && !卡密格式规则.MatchString(request.Card) {
		失败提示管理端(ctx, "卡密格式不正确")
		return
	}
	// 代理范围必须来自登录态；历史归属已保存在统计中，无需关联可能被删除的卡。
	if agentID > 0 {
		request.AgentID = agentID
	}
	if request.Kind == "agent" {
		request.Card, request.Software = "", 0
	}
	zone := time.FixedZone("CST", 8*3600)
	start, err1 := time.ParseInLocation("2006-01-02", request.StartDate, zone)
	end, err2 := time.ParseInLocation("2006-01-02", request.EndDate, zone)
	end = end.AddDate(0, 0, 1)
	if err1 != nil || err2 != nil || !end.After(start) || end.Sub(start) > 366*24*time.Hour {
		失败提示管理端(ctx, "请选择有效日期范围，每次最多查询366天")
		return
	}
	step := int64(86400)
	bucket := "((hour + 28800) DIV 86400) * 86400 - 28800"
	if request.Granularity == "hour" {
		if end.Sub(start) > 31*24*time.Hour {
			失败提示管理端(ctx, "按小时展示最多查询31天，请改为按天展示")
			return
		}
		step, bucket = 3600, "hour"
	} else if request.Granularity != "day" {
		失败提示管理端(ctx, "请选择按小时或按天展示")
		return
	}
	key := 余额统计查询键{admin, request}
	余额统计查询缓存.Lock()
	cached, exists := 余额统计查询缓存.Items[key]
	余额统计查询缓存.Unlock()
	if exists && time.Now().Before(cached.Until) {
		成功提示管理端(ctx, gin.H{"data": cached.Result})
		return
	}
	queryCtx, cancel := context.WithTimeout(ctx.Request.Context(), 5*time.Second)
	defer cancel()
	query := db.WithContext(queryCtx).Table(余额统计表名).
		Where("admin = ? AND kind = ? AND hour >= ? AND hour < ?", admin, request.Kind, start.Unix(), end.Unix())
	if request.AgentID > 0 {
		query = query.Where("agent_id = ?", request.AgentID)
	}
	if request.Software > 0 {
		query = query.Where("software = ?", request.Software)
	}
	if request.Card != "" {
		query = query.Where("card = ?", request.Card)
	}
	var rows []余额统计曲线点
	// bucket 只取上面的固定表达式，不拼接用户输入。金额按十进制字符串返回，
	// 避免 JavaScript 大整数精度丢失；总额与曲线使用同一次查询的结果。
	err := query.Select(bucket + " AS time, SUM(increase) AS increase, SUM(decrease) AS decrease").Group(bucket).Order("time").Scan(&rows).Error
	if err != nil {
		失败提示管理端(ctx, "查询统计失败，请稍后重试")
		return
	}
	result := 余额统计响应{StartedAt: 全局余额统计.开始时间, Trend: make([]余额统计曲线点, 0)}
	byTime := make(map[int64]余额统计曲线点, len(rows))
	increase, decrease := new(big.Int), new(big.Int)
	for _, row := range rows {
		byTime[row.Time] = row
		if amount, ok := new(big.Int).SetString(row.Increase, 10); ok {
			increase.Add(increase, amount)
		}
		if amount, ok := new(big.Int).SetString(row.Decrease, 10); ok {
			decrease.Add(decrease, amount)
		}
	}
	// 启用前和未来没有统计覆盖，不补成“零消费”；启用当日只包含启用后的数据。
	first := start.Unix()
	coverage := ((result.StartedAt+28800)/step)*step - 28800
	if coverage > first {
		first = coverage
	}
	for timestamp := first; timestamp < end.Unix() && timestamp <= time.Now().Unix(); timestamp += step {
		row, ok := byTime[timestamp]
		if !ok {
			row = 余额统计曲线点{Time: timestamp, Increase: "0", Decrease: "0"}
		}
		result.Trend = append(result.Trend, row)
	}
	result.Increase, result.Decrease = increase.String(), decrease.String()
	余额统计查询缓存.Lock()
	if len(余额统计查询缓存.Items) >= 64 {
		for key := range 余额统计查询缓存.Items {
			delete(余额统计查询缓存.Items, key)
		}
	}
	余额统计查询缓存.Items[key] = 余额统计缓存项{result, time.Now().Add(30 * time.Second)}
	余额统计查询缓存.Unlock()
	成功提示管理端(ctx, gin.H{"data": result})
}
