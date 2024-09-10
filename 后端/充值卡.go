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

func 充值卡_生成(ctx *gin.Context) {
	var a struct {
		Name     string
		Password string
		Software int
		Add_time float64
		Num      int
		O充值次数    int       `json:"充值次数"`
		O有效期至    time.Time `json:"有效期至"`
		Cards    string
		O指定类型    int    `json:"指定类型"`
		Notes    string `json:"备注"`
	}
	// software 需要判断下name的
	err := ctx.ShouldBindBodyWith(&a, binding.JSON)
	if err != nil {
		fmt.Println(err)
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "数据错误!!"})
		return
	}
	if !user_soft_验证(a.Name, a.Software) {
		ctx.JSON(http.StatusOK, gin.H{"state": true, "msg": "添加成功!!"})
		return
	}
	if a.Name == "" {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "code": 0, "msg": "用户名不存在"})
		return
	}
	if a.Software == 0 {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "code": 0, "msg": "软件id不存在"})
		return
	}
	if a.Add_time <= 0 {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "code": 0, "msg": "充值天数不正确"})
		return
	}
	if a.Num <= 0 {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "code": 0, "msg": "生成数量不正确"})
		return
	}
	if a.Cards == "" && a.O指定类型 != 1 {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "code": 0, "msg": "卡密不存在"})
		return
	}
	if a.O指定类型 == 1 {
		a.Cards = ""
		for i := 0; i < a.Num; i++ {
			// 循环10次
			a.Cards = a.Cards + "\n" + strconv.FormatInt(int64(a.Software), 36) + "" + GetRandomString(16, "A")
		}
	}
	a.Cards = strings.ToUpper(a.Cards)
	// s, _ := regexp.Compile(`[\w+d]+`)
	cards_tab := regexp.MustCompile(`[\w]+`).FindAllString(a.Cards, -1)
	if len(cards_tab) != a.Num {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "code": 0, "msg": "卡密或数量不正确"})
		return
	}
	存在的卡密 := []string{}
	for _, card := range cards_tab {
		// 判断是否有重复的card
		list := 数据库表_充值卡{}
		重复 := db.Table("visitor_recharge").Where("card = ?", card).First(&list).RowsAffected
		if 重复 > 0 {
			存在的卡密 = append(存在的卡密, card)
		}
	}
	if len(存在的卡密) > 0 {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "code": 0, "msg": "有存在的卡密,请检查:" + strings.Join(存在的卡密, ",")})
		return
	}
	失败的卡密 := []string{}
	成功的卡密 := []string{}
	// 立即激活

	for _, card := range cards_tab {
		err := db.Table("visitor_recharge").Create(map[string]interface{}{
			"card":            card,
			"software":        a.Software,
			"create_time":     time.Now(),
			"add_time":        a.Add_time,
			"face_value":      a.O充值次数,
			"balance":         a.O充值次数,
			"expiration_date": a.O有效期至,
			"admin":           a.Name,
			"notes":           a.Notes,
		}).Error
		if err == nil {
			成功的卡密 = append(成功的卡密, card)
		} else {
			失败的卡密 = append(失败的卡密, card)
		}
	}
	ctx.JSON(http.StatusOK, gin.H{"state": true, "code": 1, "data": strings.Join(成功的卡密, "\n"), "msg": "成功生成" + strconv.Itoa(len(成功的卡密)) + "个:\n" + strings.Join(成功的卡密, "\n") + "\n失败:\n" + strings.Join(失败的卡密, ",")})
	日志("log/"+a.Name+time.Now().Format("200601"), fmt.Sprintf("新增充值卡;软件:%v;数量:%v个;时长:%v天%v次;成功:%v", a.Software, len(成功的卡密), a.Add_time, a.O充值次数, strings.Join(成功的卡密, ",")))
}
func 充值卡_查询(ctx *gin.Context) {
	var a struct {
		Name       string
		Password   string
		Software   int
		Card       string
		Similarity int
	}
	err := ctx.ShouldBindBodyWith(&a, binding.JSON)
	if err != nil {
		fmt.Println(err)
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "数据错误!!"})
		return
	}
	order := "create_time desc"
	b := db.Table("visitor_recharge").Order(order)
	if a.Software != 0 {
		b.Where("software", a.Software)
	}
	if a.Card != "" {
		if a.Similarity == 0 {
			b.Where("card LIKE ?", "%"+a.Card+"%")
		} else {
			b.Where("card = ?", a.Card)
		}
		order = "card"
	}
	list := []map[string]interface{}{}
	b.Where("admin = ?", a.Name).Find(&list)
	if len(list) > 1 {
		for _, m := range list {
			delete(m, "record")
		}
	}
	ctx.JSON(http.StatusOK, gin.H{"code": 1, "data": list, "num": len(list)})
}
func 充值卡_修改(ctx *gin.Context) {
	var a struct {
		Name     string
		Password string
		Software int
		Card     string
		Command  string
	}
	err := ctx.ShouldBindBodyWith(&a, binding.JSON)
	if err != nil {
		fmt.Println(err)
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "数据错误!!"})
		return
	}
	data := map[string]interface{}{}
	db.Table("visitor_recharge").Where("admin = ?", a.Name).Where("card = ?", a.Card).Find(&data)
	if data["card"] == "" {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "数据错误!!"})
		return
	}
	if a.Command == "del" {
		db.Table("visitor_recharge").Where("admin = ?", a.Name).Where("card = ?", a.Card).Delete(&data)
		ctx.JSON(http.StatusOK, gin.H{"state": true, "msg": "修改成功"})
		日志("log/"+a.Name+time.Now().Format("200601"), "删除充值卡:"+a.Card)
		return
	}
	// data["state"], _ = strconv.Atoi(a.Command)
	data["state"] = a.Command
	db.Table("visitor_recharge").Where("admin = ?", a.Name).Where("card = ?", a.Card).Updates(data)
	ctx.JSON(http.StatusOK, gin.H{"state": true, "msg": "修改成功"})
}
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
	b.Select("card", "use_time", "end_time", "card_state", "software s", "storage_time left_time")
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
		Notes           string
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
	db.Table("visitor_recharge").Where("card = ?", a.Rechargeable_card).Where("admin = ?", name).Find(&充值卡)
	if 充值卡.State == 卡密状态_冻结 {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "充值卡被冻结"})
		return
	}
	if 充值卡.Balance < len(cards) {
		// 充值卡余额不足
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "剩余次数余额不足或充值卡不正确"})
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
	if 充值卡.Card != a.Rechargeable_card {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "请检测下充值卡大小写"})
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
func visitor_暂停时长(ctx *gin.Context) {
	// 充值卡天数,充值卡
	var data struct {
		Card     string
		Software int
	}
	err := ctx.ShouldBindBodyWith(&data, binding.JSON)
	fmt.Println(err)
	if err != nil {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "数据错误"})
		return
	}
	软件设置 := software{}
	db_software.Where("id = ?", data.Software).Find(&软件设置)
	if 软件设置.O暂停扣时 <= 0 {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "冻结被禁用"})
		return
	}
	name := ctx.GetString("name")
	card := data.Card
	card2 := 卡密表样式{}
	db.Table("card_"+name).Where("card=?", card).Where("software=?", data.Software).Find(&card2)
	// 判断卡密是否正常
	if card2.Card_state != 卡密状态_正常 {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "卡密状态不正常"})
		return
	}
	if card2.Storage_time != 0 {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "冻结状态不正常"})
		return
	}
	if card2.End_time.IsZero() {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "未激活不能冻结"})
		return
	}
	剩余时间 := float64(int(card2.End_time.Unix()-time.Now().Unix())-int(软件设置.O暂停扣时*24*60*60)) / 60 / 60 / 24
	if 剩余时间 < 2 {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "扣时后剩余时间不足(低于2天)"})
		return
	}
	if 剩余时间 > 365 {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "还有很久到期,不用冻啊"})
		return
	}
	card2.Storage_time = 剩余时间
	db.Table("card_"+name).Where("card=?", card2.Card).Where("software=?", data.Software).Select("Storage_time").Updates(card2)
	卡密_删除缓存(name, card2.Card)
	卡密_记录心跳(name, card2.Card, fmt.Sprintf("扣%v,冻结时长%v", 软件设置.O暂停扣时, card2.Storage_time), ctx.ClientIP())
	ctx.JSON(http.StatusOK, gin.H{"state": true, "msg": "操作成功"})
}
func visitor_恢复时长(ctx *gin.Context) {
	// 充值卡天数,充值卡
	var data struct {
		Card     string
		Software int
	}
	err := ctx.ShouldBindBodyWith(&data, binding.JSON)
	fmt.Println(err)
	if err != nil {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "数据错误"})
		return
	}
	name := ctx.GetString("name")
	// card := data.Card
	card2 := 卡密表样式{}
	db.Table("card_"+name).Where("card=?", data.Card).Where("software=?", data.Software).Find(&card2)
	// 判断卡密是否正常
	if card2.Card_state != 卡密状态_正常 {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "卡密状态不正常"})
		return
	}
	if card2.Storage_time <= 1 {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "可解冻时间不正确"})
		return
	}
	解冻时间 := card2.Storage_time
	card2.End_time = time.Now().Add(time.Duration(解冻时间*24) * time.Hour)
	card2.Storage_time = 0
	db.Table("card_"+name).Where("card=?", card2.Card).Where("software=?", data.Software).Select("End_time", "Storage_time").Updates(card2)
	卡密_删除缓存(name, card2.Card)
	卡密_记录心跳(name, card2.Card, fmt.Sprintf("解冻%v天", 解冻时间), ctx.ClientIP())
	ctx.JSON(http.StatusOK, gin.H{"state": true, "msg": "操作成功"})
}
