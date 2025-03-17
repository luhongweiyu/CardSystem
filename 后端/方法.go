package main

import (
	"bufio"
	"crypto/md5"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

var 卡密缓存 = make(map[string]卡密表样式)

type 心跳记录 struct {
	登录时间 time.Time
	心跳标识 string
	ip   string
}

var 全局_用户每小时请求次数 = make(map[string]int)
var 全局_卡密心跳记录2 sync.Map
var 全局_id对应name = make(map[int]string)
var 全局_用户设置_name = make(map[string]user_info)

func 写入文件_追加(filePath string, str string) {
	//创建一个新文件，写入内容 5 句 “http://c.biancheng.net/golang/”
	// filePath := "卡密操作日志.txt"
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		fmt.Println("文件打开失败", err)
	}
	//及时关闭file句柄
	defer file.Close()
	//写入文件时，使用带缓存的 *Writer
	write := bufio.NewWriter(file)
	write.WriteString(str)
	//Flush将缓存的文件真正写入到文件中
	write.Flush()
}
func 日志(filePath string, info string) {
	s := fmt.Sprintf("%s:%s", time.Now().Format("2006-01-02 15:04:05"), info)
	写入文件_追加(filePath, "\n"+s)
}
func 拒绝日志(ctx *gin.Context) {
	ip := ctx.ClientIP()
	card := input(ctx, "card")
	请求方式 := ctx.Request.Method
	请求链接 := ctx.Request.RequestURI
	日志("log/拒绝"+time.Now().Format("200601"), fmt.Sprintf("ip:%-15v;请求方式:%-6v;card:%-20v;请求链接:%v", ip, 请求方式, card, 请求链接))
}
func GetRandomString(n int, 大小写 string) string {
	a := []byte{}
	for i := 0; i < n; i++ {
		c := byte(rand.Intn(26) + 'A')
		a = append(a, c)
	}
	return string(a)
}

func card_id获取用户设置(ctx *gin.Context) {
	card := input(ctx, "card")
	if card == "" {
		失败提示(ctx, "card错误")
		拒绝日志(ctx)
		ctx.Abort()
		return
	}
	var userinfo = user_info{}
	var ok = false
	name := input(ctx, "name")
	if name == "" {
		id, _ := strconv.Atoi(input(ctx, "center_id"))
		name = 全局_id对应name[id]
	}
	if !请求防火墙(name) {
		失败提示(ctx, "api次数超限")
		ctx.Abort()
		return
	}
	if name != "" {
		userinfo, ok = 全局_用户设置_name[name]
		// if !ok {
		// 	user_刷新用户设置(name)
		// 	userinfo, ok = 全局_用户设置_name[name]
		// }
	}
	if !ok {
		失败提示(ctx, "center_id或name错误")
		拒绝日志(ctx)
		ctx.Abort()
		return
	}
	ctx.Set("card", gin线程_变量_user_ifo{
		card:      card,
		user_info: userinfo})
	ctx.Next()
}
func 加入时间戳(ctx *gin.Context, H gin.H) gin.H {
	t, err := strconv.Atoi(input(ctx, "timestamp"))
	if err != nil {
		t = int(time.Now().Unix())
	}
	t = t + 10
	c, _ := ctx.Get("card")
	用户设置, _ := c.(gin线程_变量_user_ifo)
	code, _ := H["code"].(int)
	str := strconv.Itoa(t) + 用户设置.Api_password + strconv.Itoa(code)
	s := md5.Sum([]byte(str))
	H["sign"] = fmt.Sprintf("%x", s)
	H["timestamp"] = t
	return H
}
func 失败提示(ctx *gin.Context, 提示 interface{}) {
	data, ok := 提示.(gin.H)
	if ok {
		data["code"] = 0
		data["state"] = false
	} else {
		data = gin.H{"code": 0, "state": false, "msg": 提示}
	}
	ctx.JSON(http.StatusOK, 加入时间戳(ctx, data))
}
func 成功提示(ctx *gin.Context, 提示 interface{}) {
	data, ok := 提示.(gin.H)
	if ok {
		data["code"] = 1
		data["state"] = true
	} else {
		data = gin.H{"code": 1, "state": true, "data": 提示}
	}
	ctx.JSON(http.StatusOK, 加入时间戳(ctx, data))
}
func 卡密md5验证(ctx *gin.Context) {
	c, _ := ctx.Get("card")
	用户设置, _ := c.(gin线程_变量_user_ifo)
	if !用户设置.Api_safe {
		return
	}
	加密参数, _ := 取参数表(ctx, []string{"timestamp", "sign"})
	t, _ := strconv.Atoi(加密参数["timestamp"])
	if (int64(t)-time.Now().Unix()) > 10*60 || (int64(t)-time.Now().Unix()) < -10*60 {
		// 超时
		失败提示(ctx, "时间不正确")
		拒绝日志(ctx)
		ctx.Abort()
		return
	}
	srcCode := md5.Sum([]byte(加密参数["timestamp"] + 用户设置.Api_password))
	if fmt.Sprintf("%x", srcCode) == 加密参数["sign"] {
		return
	} else {
		失败提示(ctx, "sign错误")
		拒绝日志(ctx)
		ctx.Abort()
		return
	}
}

var 卡密_缓存锁 sync.RWMutex

func 卡密_删除缓存(管理员用户名 string, card string) {
	card = strings.ToLower(card)
	卡密_缓存锁.Lock()
	defer 卡密_缓存锁.Unlock()
	delete(卡密缓存, 管理员用户名+"_"+card)
}
func 卡密_读取缓存(管理员用户名 string, card string) (卡密表样式, bool) {
	card = strings.ToLower(card)
	卡密_刷新缓存 := func(管理员用户名 string, card string) {
		var a 卡密表样式
		err := db.Table("card_"+管理员用户名).Where("card=?", card).First(&a).Error
		if err == nil {
			卡密缓存[管理员用户名+"_"+card] = a
		}
	}
	卡密_缓存锁.Lock()
	defer 卡密_缓存锁.Unlock()
	a, ok := 卡密缓存[管理员用户名+"_"+card]
	if !ok {
		卡密_刷新缓存(管理员用户名, card)
		a, ok = 卡密缓存[管理员用户名+"_"+card]
	}
	return a, ok
}
func 卡密_记录心跳(name string, card string, 心跳标识 string, ip string) int {
	c, ok := 全局_卡密心跳记录2.Load(name + card)
	a, _ := c.(*[]心跳记录)
	if !ok {
		b := []心跳记录{}
		全局_卡密心跳记录2.Store(name+card, &b)
		a = &b
	}
	b := append([]心跳记录{{time.Now(), 心跳标识, ip}}, (*a)...)
	if len(b) > 100 {
		b = b[:100]
	}
	上次 := 999999
	if len(b) > 1 {
		上次 = int(b[1].登录时间.Unix() - b[0].登录时间.Unix())
	}
	*a = b
	return 上次

}
func 卡密_查询心跳(ctx *gin.Context) {
	// fmt.Println(gin线程_变量[ctx])
	c, _ := ctx.Get("card")
	用户设置, _ := c.(gin线程_变量_user_ifo)
	name := 用户设置.Name
	card := 用户设置.card
	list, _ := 卡密_读取缓存(name, card)
	if list.Card == "" {
		失败提示(ctx, "没有记录")
		return
	}
	s := []string{}
	临时, ok := 全局_卡密心跳记录2.Load(name + card)
	if ok {
		心跳记录, _ := 临时.(*[]心跳记录)
		for _, v := range *心跳记录 {
			s = append(s, fmt.Sprintf("%v:  %v  %v", v.登录时间.Format("2006-01-02 15:04:05"), v.心跳标识, v.ip))
		}
	}
	状态 := "正常"
	使用时间 := list.Use_time.Format("2006-01-02 15:04:05")
	到期时间 := list.End_time.Format("2006-01-02 15:04:05")
	if list.Use_time.IsZero() {
		使用时间 = ""
	}
	if list.End_time.IsZero() {
		到期时间 = ""
	} else if list.End_time.Unix() < time.Now().Unix() {
		状态 = "到期"
	}
	if list.Card_state == 卡密状态_冻结 {
		状态 = "冻结"
	}
	s2 := fmt.Sprintf("卡密:%v\n使用时间:  %v\n到期时间:  %v\n激活天数:%v天,状态:%v\n登录记录:\n%v", list.Card, 使用时间, 到期时间, list.Available_time, 状态, strings.Join(s, "\n"))
	成功提示(ctx, gin.H{"data": s2, "id2": list.ID子账号})
	// ctx.String(http.StatusOK, s2)
}
func card_login(ctx *gin.Context) {
	登录参数, ok := 取参数表(ctx, []string{"software"})
	if !ok {
		失败提示(ctx, "software参数错误")
		return
	}
	c, _ := ctx.Get("card")
	用户设置, _ := c.(gin线程_变量_user_ifo)
	name := 用户设置.Name
	card := 用户设置.card
	software, _ := strconv.Atoi(登录参数["software"])
	if name == "" {
		失败提示(ctx, "center_id不能为空")
		return
	}
	list, ok := 卡密_读取缓存(name, card)
	if !ok || list.Software != software {
		失败提示(ctx, "卡密不存在或者已过期")
		return
	}
	if list.Card != card {
		失败提示(ctx, "错误,请检查大小写或者空格")
		return
	}
	if list.Card_state == 卡密状态_冻结 {
		失败提示(ctx, "卡密被冻结")
		return
	}
	if list.Storage_time > 0 {
		失败提示(ctx, "时长被暂停")
	}
	if list.End_time.Unix() == (time.Time{}).Unix() {
		list = 激活卡密(name, card)
	}
	if list.End_time.Unix() > time.Now().Unix() {
		list.Needle = GetRandomString(6, "a")
		list.Use_time = time.Now()
		db.Table("card_" + name).Updates(list)
		卡密_删除缓存(name, card)
		卡密_记录心跳(name, card, list.Needle, ctx.ClientIP())
		成功提示(ctx, gin.H{
			"needle":            list.Needle,
			"endtime_timestamp": list.End_time.Unix(),
			"less_time":         fmt.Sprintf("%.2f天", float32(list.End_time.Unix()-time.Now().Unix())/(60*60*24)),
			"endtime":           fmt.Sprint(list.End_time.Format("2006年01月02日03:04:05")),
			"id2":               list.ID子账号,
		})

	} else {
		失败提示(ctx, "卡密过期")
	}

}
func card_ping(ctx *gin.Context) {
	登录参数, ok := 取参数表(ctx, []string{"needle", "card", "center_id"})
	if !ok {
		失败提示(ctx, "center_id,card,needle参数错误")
		return
	}
	c, _ := ctx.Get("card")
	用户设置, _ := c.(gin线程_变量_user_ifo)
	name := 用户设置.Name
	card := 登录参数["card"]
	needle := 登录参数["needle"]

	if name == "" {
		失败提示(ctx, "center_id不能为空")
		return
	}
	if card == "" {
		失败提示(ctx, "card不能为空")
		return
	}
	if needle == "" {
		失败提示(ctx, "needle不能为空")
		return
	}
	list, ok := 卡密_读取缓存(name, card)
	if !ok {
		失败提示(ctx, "卡密不存在")
		return
	}
	if list.Card_state == 卡密状态_冻结 {
		失败提示(ctx, "卡密被冻结")
		return
	}
	if list.Storage_time > 0 {
		失败提示(ctx, "时长被暂停")
	}
	if list.End_time.Unix() < time.Now().Unix() {
		失败提示(ctx, "过期啦")
		return
	}
	if list.Needle != "" && list.Needle == needle {
		上次 := 999999
		上次 = 卡密_记录心跳(name, card, needle, ctx.ClientIP())
		成功提示(ctx, gin.H{"last": 上次})
		return
	} else {
		失败提示(ctx, "验证失败,可能被挤下线")
		return
	}

}
func slicePage(page int, pageSize int, nums int) (sliceStart, sliceEnd int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 100 //设置一页默认显示的记录数
	}
	if pageSize > nums {
		return 0, nums
	}
	// 总页数
	pageCount := int(math.Ceil(float64(nums) / float64(pageSize)))
	if page > pageCount {
		return 0, 0
	}
	sliceStart = (page - 1) * pageSize
	sliceEnd = sliceStart + pageSize

	if sliceEnd > nums {
		sliceEnd = nums
	}
	return sliceStart, sliceEnd
}
func 管理员_查询所有卡密(ctx *gin.Context) {
	// center_id := input(ctx, "center_id")
	// if center_id == "" {
	// 	ctx.JSON(http.StatusOK, gin.H{"code": 0, "msg": "用户名不存在"})
	// 	return
	// }
	// software := input(ctx, "software")
	// if software == "" {
	// 	ctx.JSON(http.StatusOK, gin.H{"code": 0, "msg": "软件id不存在"})
	// 	return
	// }
	// list := []map[string]interface{}{}
	// db.Table(center_id).Where("software = ?", software).Find(&list)
	// ctx.JSON(http.StatusOK, gin.H{"code": 1, "data": list})
	// -----

	var a struct {
		Name           string
		Software       int
		Card_state     int
		Available_time float64
		Card           string
		Notes          string
	}
	err := ctx.ShouldBindBodyWith(&a, binding.JSON)
	fmt.Println(err)
	if err != nil {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "数据错误"})
		return
	}
	查询所有卡密(ctx, 0, a.Name, a.Software, a.Card_state, a.Available_time, a.Card, a.Notes)
}

func 查询所有卡密(ctx *gin.Context, ID子账号 int, Name string, Software int, Card_state int, Available_time float64, Card string, Notes string) {
	a := struct {
		Name           string
		Software       int
		Card_state     int
		Available_time float64
		Card           string
		Notes          string
	}{
		Name:           Name,
		Software:       Software,
		Card_state:     Card_state,
		Available_time: Available_time,
		Card:           Card,
		Notes:          Notes,
	}
	b := db.Table("card_" + a.Name)
	order := "create_time desc"
	if a.Software != 0 {
		b.Where("software", a.Software)
	}
	if a.Card_state != 0 {
		if a.Card_state == 1 {
			b.Where("end_time", nil)
			b.Where("card_state = ?", 卡密状态_正常)
		} else if a.Card_state == 2 {
			// 已激活
			b.Where("end_time > ?", time.Now())
			b.Where("card_state = ?", 卡密状态_正常)
		} else if a.Card_state == 3 {
			b.Where("end_time <= ?", time.Now())
			b.Where("card_state = ?", 卡密状态_正常)
		} else if a.Card_state == 4 {
			b.Where("card_state = ?", 卡密状态_正常)
		} else if a.Card_state == 5 {
			b.Where("card_state = ?", 卡密状态_冻结)
		}
	}
	if a.Available_time != 0 {
		b.Where("available_time", a.Available_time)
	}
	if a.Card != "" {
		b.Where("card LIKE ?", "%"+a.Card+"%")
		order = "card"
	}
	if a.Notes != "" {
		b.Where("notes LIKE ?", "%"+a.Notes+"%")
	}
	if ID子账号 != 0 {
		b.Where("ID子账号 = ?", ID子账号)
	}
	list := []map[string]interface{}{}
	b.Order(order).Find(&list)

	var page struct {
		O当前页 int `json:"当前页"`
		O每页  int `json:"每页"`
	}
	err := ctx.ShouldBindBodyWith(&page, binding.JSON)
	if err != nil {
		fmt.Println(err)
		page.O当前页 = 1
		page.O每页 = 100
	}
	start, end := slicePage(page.O当前页, page.O每页, len(list)) //第一页1页显示3条数据
	list2 := list[start:end]
	for _, v := range list2 {
		card, ok := v["card"].(string)
		if ok {
			临时, ok := 全局_卡密心跳记录2.Load(a.Name + card)
			if ok {
				心跳记录, _ := 临时.(*[]心跳记录)
				if len(*心跳记录) > 0 {
					v2 := (*心跳记录)[0]
					v["history"] = fmt.Sprintf("%v,%v", v2.登录时间.Format("01-02 15:04"), v2.ip)
				}
			}
		}
	}
	ctx.JSON(http.StatusOK, gin.H{"code": 1, "data": list2, "num": len(list)})
}
func 查询操作日志(ctx *gin.Context) {
	var a struct {
		Name string
	}
	err := ctx.ShouldBindBodyWith(&a, binding.JSON)
	fmt.Println(err)
	if err != nil {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "数据错误"})
		return
	}
	name := a.Name
	if name == "" {
		ctx.JSON(http.StatusOK, gin.H{"code": 0, "msg": "用户名不存在"})
		return
	}
	// 文件名1 :=
	文件1, err := os.ReadFile("log/" + name + time.Now().Format("200601"))
	内容1 := "没有其他内容"
	if err == nil {
		内容1 = string(文件1)
	}
	文件2, err := os.ReadFile("log/" + name + time.Now().AddDate(0, -1, 0).Format("200601"))
	内容2 := "没有其他内容"
	if err == nil {
		内容2 = string(文件2)
	}
	ctx.String(http.StatusOK, 内容1+"\n"+内容2)
}
func 管理员_add_new_card(ctx *gin.Context) {
	var a struct {
		Name                   string
		Password               string
		Software               int
		Available_time         float64
		Num                    int
		Latest_activation_time int
		Cards                  string
		Notes                  string
		Config_content         string
		O指定类型                  int `json:"指定类型"`
	}
	// software 需要判断下name的
	err := ctx.ShouldBindBodyWith(&a, binding.JSON)
	if err != nil {
		fmt.Println(err)
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "数据错误!!"})
		return
	}
	add_new_card(ctx, 0, a.Name, a.Software, a.Available_time, a.Num, a.Latest_activation_time, a.Cards, a.Notes, a.Config_content, a.O指定类型)
}

func add_new_card(ctx *gin.Context, ID子账号 int, Name string, Software int, Available_time float64, Num int, Latest_activation_time int, Cards string, Notes string, Config_content string, O指定类型 int) (结果 bool) {
	结果 = false
	a := struct {
		Name                   string
		Software               int
		Available_time         float64
		Num                    int
		Latest_activation_time int
		Cards                  string
		Notes                  string
		Config_content         string
		O指定类型                  int `json:"指定类型"`
	}{
		Name:                   Name,
		Software:               Software,
		Available_time:         Available_time,
		Num:                    Num,
		Latest_activation_time: Latest_activation_time,
		Cards:                  Cards,
		Notes:                  Notes,
		Config_content:         Config_content,
		O指定类型:                  O指定类型,
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
	if a.Available_time < 0 {
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
			a.Cards = a.Cards + "\n" + strconv.FormatInt(int64(a.Software), 36) + "" + GetRandomString(16, "a")
		}
	}
	a.Cards = strings.ToLower(a.Cards)
	// s, _ := regexp.Compile(`[\w+d]+`)
	cards_tab := regexp.MustCompile(`[\w]+`).FindAllString(a.Cards, -1)
	if len(cards_tab) != a.Num {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "code": 0, "msg": "卡密或数量不正确"})
		return
	}
	存在的卡密 := []string{}
	for _, card := range cards_tab {
		// 判断是否有重复的card
		list := 卡密表样式{}
		重复 := db.Table("card_"+a.Name).Where("card = ?", card).First(&list).RowsAffected
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
		err := db.Table("card_" + a.Name).Create(map[string]interface{}{
			"card":                   card,
			"card_state":             卡密状态_正常,
			"create_time":            time.Now(),
			"software":               a.Software,
			"available_time":         a.Available_time,
			"latest_activation_time": a.Latest_activation_time,
			"Notes":                  a.Notes,
			"Config_content":         a.Config_content,
			"Storage_time":           0,
			"ID子账号":                  ID子账号,
		}).Error
		if err == nil {
			成功的卡密 = append(成功的卡密, card)
		} else {
			失败的卡密 = append(失败的卡密, card)
		}
	}
	ctx.JSON(http.StatusOK, gin.H{"state": true, "code": 1, "data": strings.Join(成功的卡密, "\n"), "msg": "成功生成" + strconv.Itoa(len(成功的卡密)) + "个:\n" + strings.Join(成功的卡密, "\n") + "\n失败:\n" + strings.Join(失败的卡密, ",")})
	日志("log/"+a.Name+time.Now().Format("200601"), fmt.Sprintf("账号:%v;新增;软件:%v;数量:%v个;时长:%v天;成功:%v", ID子账号, a.Software, len(成功的卡密), a.Available_time, strings.Join(成功的卡密, ",")))
	if a.Latest_activation_time == 0 || a.Available_time <= 0 {
		for _, card := range cards_tab {
			激活卡密(a.Name, card)
		}
	}
	结果 = true
	return
}
func 激活卡密(管理员用户名 string, 卡密 string) 卡密表样式 {
	a := 卡密表样式{}
	记录数 := db.Table("card_"+管理员用户名).Where("card=?", 卡密).First(&a).RowsAffected
	if 记录数 == 0 {
		return a
	}
	到期时间 := time.Now().Add(time.Minute * time.Duration(a.Available_time*24*60))
	if a.Latest_activation_time > 0 {
		最晚到期时间 := a.Create_time.Add(time.Minute * time.Duration(a.Latest_activation_time*24*60))
		if 最晚到期时间.Unix() < 到期时间.Unix() {
			到期时间 = 最晚到期时间
		}
	}
	if a.Available_time == 36500 {
		到期时间, _ = time.Parse("2006年1月2日15:04:05", "2099年1月1日00:00:00")
	}
	a.End_time = 到期时间
	db.Table("card_" + 管理员用户名).Updates(&a)
	return a
}

func 管理员_add_card_time(ctx *gin.Context) {
	var a struct {
		Name     string
		Password string
		Cards    []string
		Add_time float64
	}
	err := ctx.ShouldBindBodyWith(&a, binding.JSON)
	if err != nil {
		fmt.Println(err)
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "数据错误!!"})
		return
	}
	// cards := regexp.MustCompile(`[\w+d]{7,}`).FindAllString(a.Cards, -1)
	add_card_time(ctx, 0, a.Name, a.Cards, a.Add_time, 0)
}

func add_card_time(ctx *gin.Context, ID子账号 int, Name string, Cards []string, Add_time float64, Software int) (成功数量 int, 失败数量 int) {
	成功数量 = 0
	失败数量 = 0
	a := struct {
		Name     string
		Cards    []string
		Add_time float64
		Software int
	}{
		Name:     Name,
		Cards:    Cards,
		Add_time: Add_time,
		Software: Software,
	}
	// err := ctx.ShouldBindBodyWith(&a, binding.JSON)
	// if err != nil {
	// 	fmt.Println(err)
	// 	ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "数据错误!!"})
	// 	return
	// }
	// // cards := regexp.MustCompile(`[\w+d]{7,}`).FindAllString(a.Cards, -1)
	失败的卡密 := []string{}
	成功的卡密 := []string{}
	for _, card := range a.Cards {
		list := map[string]interface{}{}
		b := db.Table("card_" + a.Name)
		if ID子账号 != 0 {
			b.Where("ID子账号=?", ID子账号).Where("software=?", a.Software)
		}
		修改行 := b.Where("card=?", card).Find(&list).RowsAffected
		if 修改行 > 0 {
			修改行 = 0
			end_time, ok := list["end_time"].(time.Time)
			if ok {
				if end_time.Unix() < time.Now().Unix() {
					end_time = time.Now()
				}
				end_time = time.Time(end_time).Add(time.Duration(a.Add_time*24*60) * time.Minute)
				修改行 = db.Table("card_" + a.Name).Where(map[string]interface{}{"card": card}).Updates(map[string]interface{}{"end_time": end_time}).RowsAffected
			}
		}
		if 修改行 > 0 {
			成功的卡密 = append(成功的卡密, card)
		} else {
			失败的卡密 = append(失败的卡密, card)
		}
		卡密_删除缓存(a.Name, card)
	}
	ctx.JSON(http.StatusOK, gin.H{"state": true, "code": 1, "msg": "成功:\n" + strings.Join(成功的卡密, ",\n") + "\n失败:\n" + strings.Join(失败的卡密, ",\n")})
	日志("log/"+a.Name+time.Now().Format("200601"), fmt.Sprintf("账号:%v;加时;数量:%v个;时长:%v天;成功:%v", ID子账号, len(成功的卡密), a.Add_time, strings.Join(成功的卡密, ",")+";失败:"+strings.Join(失败的卡密, ",")))
	成功数量 = len(成功的卡密)
	失败数量 = len(失败的卡密)
	return
}
func 管理员_delete_card(ctx *gin.Context) {
	var a struct {
		Name     string
		Password string
		Cards    []string
	}
	// software 需要判断下name的
	err := ctx.ShouldBindBodyWith(&a, binding.JSON)
	if err != nil {
		fmt.Println(err)
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "数据错误!!"})
		return
	}
	delete_card(ctx, 0, a.Name, a.Cards)
}
func delete_card(ctx *gin.Context, ID子账号 int, Name string, Cards []string) {

	a := struct {
		Name  string
		Cards []string
	}{
		Name:  Name,
		Cards: Cards,
	}
	失败的卡密 := []string{}
	成功的卡密 := []string{}
	for _, card := range a.Cards {
		list := 卡密表样式{} //map[string]interface{}{}
		b := db.Table("card_" + a.Name)
		if ID子账号 != 0 {
			b.Where("ID子账号=?", ID子账号)
		}
		err := b.Where("card = ?", card).Delete(&list).Error
		if err == nil {
			成功的卡密 = append(成功的卡密, card)
		} else {
			失败的卡密 = append(失败的卡密, card)
		}
		卡密_删除缓存(a.Name, card)
	}
	ctx.JSON(http.StatusOK, gin.H{"state": true, "code": 1, "msg": "成功:\n" + strings.Join(成功的卡密, ",\n") + "\n失败:\n" + strings.Join(失败的卡密, ",")})
	日志("log/"+a.Name+time.Now().Format("200601"), fmt.Sprintf("账号:%v;删除;数量:%v个;成功:%v", ID子账号, len(成功的卡密), strings.Join(成功的卡密, ",")))

}
func modify_card(ctx *gin.Context) {
	var b struct {
		Name string
	}
	ctx.ShouldBindBodyWith(&b, binding.JSON)

	var a struct {
		Software       int
		Card           string
		End_time       *time.Time
		Config_content *string
		Notes          *string
		Card_state     int
	}
	err := ctx.ShouldBindBodyWith(&a, binding.JSON)
	if err != nil {
		fmt.Println(err)
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "数据错误!!"})
		return
	}
	修改卡密(ctx, 0, b.Name, a.Card, a)
}
func 修改卡密(ctx *gin.Context, ID子账号 int, Name string, Card string, data interface{}) {
	b := db.Table("card_"+Name).Where("card = ?", Card)
	if ID子账号 != 0 {
		b.Where("ID子账号=?", ID子账号)
	}
	b.Updates(data)
	ctx.JSON(http.StatusOK, gin.H{"state": true, "msg": "修改成功"})
	卡密_删除缓存(Name, Card)
	日志("log/"+Name+time.Now().Format("200601"), fmt.Sprintf("修改;%v;", Card))
}
func 管理员_冻卡s(ctx *gin.Context) {
	var a struct {
		Name       string
		Password   string
		Cards      []string
		Card_state int
	}
	// software 需要判断下name的
	err := ctx.ShouldBindBodyWith(&a, binding.JSON)
	if err != nil {
		fmt.Println(err)
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "数据错误!!"})
		return
	}
	修改卡密_批量(ctx, 0, a.Name, a.Cards, struct{ Card_state int }{Card_state: a.Card_state})
}
func 修改卡密_批量(ctx *gin.Context, ID子账号 int, Name string, Cards []string, data interface{}) {
	for _, Card := range Cards {
		b := db.Table("card_"+Name).Where("card = ?", Card)
		if ID子账号 != 0 {
			b.Where("ID子账号=?", ID子账号)
		}
		b.Updates(data)
		卡密_删除缓存(Name, Card)
	}
	ctx.JSON(http.StatusOK, gin.H{"state": true, "msg": "修改成功"})
	日志("log/"+Name+time.Now().Format("200601"), fmt.Sprintf("批量修改;数量%v个;%v;%v", len(Cards), data, Cards))
}
func modify_card_configContent(ctx *gin.Context) {
	// var b struct {
	// 	Type string
	// }
	// ctx.ShouldBindBodyWith(&b, binding.JSON)
	a := input(ctx, "type")
	c, _ := ctx.Get("card")
	用户设置, _ := c.(gin线程_变量_user_ifo)
	name := 用户设置.Name
	card := 用户设置.card
	if a == "write" {
		value := input(ctx, "value")
		db.Table("card_"+name).Where("card = ?", card).Update("config_content", value)
		卡密_删除缓存(name, card)
	}
	b, _ := 卡密_读取缓存(name, card)
	成功提示(ctx, b.Config_content)

}

// 卡密充值
func card_recharge(ctx *gin.Context) {
	// fmt.Println(gin线程_变量[ctx])
	c, _ := ctx.Get("card")
	用户设置, _ := c.(gin线程_变量_user_ifo)
	name := 用户设置.Name
	card1 := 用户设置.card
	card2 := input(ctx, "card2")
	if card2 == "" {
		失败提示(ctx, "card2错误")
		ctx.Abort()
		return
	}
	a1 := 卡密表样式{}
	a2 := 卡密表样式{}
	记录数1 := db.Table("card_"+name).Where("card=?", card1).First(&a1).RowsAffected
	记录数2 := db.Table("card_"+name).Where("card=?", card2).First(&a2).RowsAffected
	if 记录数1 < 1 || 记录数2 < 1 {
		失败提示(ctx, "卡密或充值卡不正确")
		ctx.Abort()
		return
	}
	if a1.Software != a2.Software {
		失败提示(ctx, "卡密类型不一样")
		ctx.Abort()
		return
	}
	if a2.Card_state != 2 {
		失败提示(ctx, "充值卡状态不正常")
		ctx.Abort()
		return
	}
	if a2.Available_time <= 0 {
		失败提示(ctx, "充值卡剩余时间不足")
		ctx.Abort()
		return
	}
	if a1.End_time.IsZero() {
		失败提示(ctx, "激活码["+a1.Card+"]必须是被激活使用的卡密")
		ctx.Abort()
		return
	}
	if !a2.End_time.IsZero() {
		失败提示(ctx, "充值卡["+a2.Card+"]必须是未激活未使用的卡密")
		ctx.Abort()
		return
	}
	end_time := a1.End_time
	if end_time.Unix() < time.Now().Unix() {
		end_time = time.Now()
	}
	end_time = time.Time(end_time).Add(time.Duration(a2.Available_time*24*60) * time.Minute)
	// 修改充值卡
	修改行2 := db.Table("card_" + name).Where(map[string]interface{}{"card": a2.Card}).Updates(map[string]interface{}{"card_state": 卡密状态_冻结, "available_time": -1 * a2.Available_time, "notes": a2.Notes + ">充值给:" + a1.Card}).RowsAffected
	if 修改行2 < 1 {
		失败提示(ctx, "保存充值卡记录失败")
		ctx.Abort()
		return
	}
	// 修改激活码 , "notes": fmt.Sprintf("%v>使用[%v]充值%v天", a1.Notes, a2.Card, a2.Available_time)
	修改行1 := db.Table("card_" + name).Where(map[string]interface{}{"card": a1.Card}).Updates(map[string]interface{}{"end_time": end_time}).RowsAffected
	卡密_删除缓存(name, a1.Card)
	if 修改行1 < 1 {
		失败提示(ctx, "保存激活码记录失败")
		ctx.Abort()
		return
	}
	日志("log/"+name+time.Now().Format("200601"), fmt.Sprintf("充值;数量:1个;时长:%v天;激活码:%v;充值卡:%v", a2.Available_time, a1.Card, a2.Card))
	成功提示(ctx, "充值成功")
}
