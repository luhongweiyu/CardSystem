package main

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

var visitor_锁 sync.RWMutex

func visitor_验证对应id(ctx *gin.Context) {
	visitor_锁.Lock()
	defer func() {
		time.Sleep(1 * time.Second)
		visitor_锁.Unlock()
	}()
	id, _ := strconv.Atoi(input(ctx, "center_id"))
	name := 全局_id对应name[id]
	if name == "" {
		ctx.Abort()
		return
	}
	ctx.Set("name", name)
	ctx.Next()
}
func visitor_查询所有卡密(ctx *gin.Context) {
	// card := input(ctx, "card")
	input := struct {
		Card string
	}{}
	ctx.ShouldBindBodyWith(&input, binding.JSON)
	card := input.Card
	fmt.Println(card)
	card = regexp.MustCompile("[^a-zA-Z0-9]").ReplaceAllString(card, "")
	name := ctx.GetString("name")
	if len(card) < 3 || name == "" {
		fmt.Println(card)
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "卡密格式不正确"})
		return
	}
	// 这里需要增加查询结果限制
	b := db.Table("card_" + name)
	// b.Where("card LIKE ?", card+"[0-9][0-9][0-9]")
	b.Where("card like ?", card+"___").Or("card = ?", card)
	b.Select("card", "use_time", "end_time", "card_state", "software s")
	order := "card"
	list := []map[string]interface{}{}
	b.Order(order).Find(&list)
	ctx.JSON(http.StatusOK, gin.H{"state": true, "data": list, "num": len(list)})
}
func visitor_查询充值卡(ctx *gin.Context) {
	input := struct {
		Rechargeable_card string
	}{}
	ctx.ShouldBindBodyWith(&input, binding.JSON)
	card := input.Rechargeable_card

	// card := input(ctx, "Rechargeable_card")
	name := ctx.GetString("name")
	// 查询充值卡余额
	var 充值卡 数据库表_充值卡
	充值卡.Card = card
	// list := map[string]interface{}{}
	b := struct {
		Card            string `gorm:"primaryKey"`
		S               int    `gorm:"column:software" json:"S"`
		AddTime         int
		Face_value      int
		Balance         int
		Expiration_date time.Time
		Record          string
		// Admin           string
	}{}
	db.Table("visitor_recharge").Where("admin=?", name).Where("card = ?", 充值卡.Card).Find(&b)
	// fmt.Println(list)
	if b.S == 0 {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "查询失败,请检查充值卡是否正确"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"state": true, "data": b})

}
func visitor_续费卡密(ctx *gin.Context) {
	// 充值卡天数,充值卡
	var a struct {
		Rechargeable_card string
		Cards             []string
	}
	err := ctx.ShouldBindBodyWith(&a, binding.JSON)
	fmt.Println(err)
	if err != nil {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "数据错误"})
		return
	}
	name := ctx.GetString("name")
	cards := a.Cards
	// 查询充值卡余额
	var 充值卡 数据库表_充值卡
	充值卡.Card = a.Rechargeable_card
	db.Table("visitor_recharge").Where("card = ?", 充值卡.Card).Where("admin = ?", name).Find(&充值卡)
	if 充值卡.Balance < len(cards) {
		// 充值卡余额不足
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "余额不足或充值卡不正确"})
		return
	}
	if 充值卡.Expiration_date.Unix() < time.Now().Unix() {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "充值卡过期"})
		return

	}
	if len(cards) < 1 {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "请输入需要充值的数据"})
		return
	}

	失败的卡密 := []string{}
	成功的卡密 := []string{}
	充值卡原次数 := 充值卡.Balance
	for _, card := range cards {
		list := map[string]interface{}{}
		修改行 := db.Table("card_"+name).Where("card=?", card).Where("software=?", 充值卡.Software).Find(&list).RowsAffected
		if 修改行 > 0 {
			修改行 = 0
			end_time, ok := list["end_time"].(time.Time)
			if ok {
				if end_time.Unix() < time.Now().Unix() {
					end_time = time.Now()
				}
				充值卡.Balance = 充值卡.Balance - 1
				end_time = time.Time(end_time).Add(time.Duration(充值卡.AddTime*24*60) * time.Minute)
				db.Table("visitor_recharge").Where("card=?", 充值卡.Card).Update("balance", 充值卡.Balance)
				修改行 = db.Table("card_" + name).Where(map[string]interface{}{"card": card}).Updates(map[string]interface{}{"end_time": end_time}).RowsAffected
			}
		}
		if 修改行 > 0 {
			成功的卡密 = append(成功的卡密, card)
		} else {
			失败的卡密 = append(失败的卡密, card)
		}
		卡密_删除缓存(name, card)
	}
	s := fmt.Sprintf("充值卡:%v;充值:%v天;本次后余额:%v;成功数量:%v个;失败数量:%v个;成功:%v;失败:%v;", 充值卡.Card, 充值卡.AddTime, 充值卡.Balance, len(成功的卡密), len(失败的卡密), strings.Join(成功的卡密, ","), strings.Join(失败的卡密, ","))
	s2 := fmt.Sprintf("\n%s;%s", time.Now().Format("2006-01-02 15:04:05"), s)
	if 充值卡原次数 != 充值卡.Balance || len(成功的卡密) > 0 {
		充值卡.Record = 充值卡.Record + s2
		db.Table("visitor_recharge").Updates(&充值卡)
		日志("log/"+name+time.Now().Format("200601"), "充值:"+s)
	}
	if len(失败的卡密) > 0 {
		s2 = "1:请检查 充值卡 和 卡密类型是否一致\n2:未激活的卡密不能充值\n" + s2
	}
	ctx.JSON(http.StatusOK, gin.H{"state": true, "msg": regexp.MustCompile(";").ReplaceAllString(s2, "\n")})
}
