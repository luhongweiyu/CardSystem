package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"gorm.io/gorm"
)

type user_son struct {
	Name     string
	Password string
	ID子账号    int    `gorm:"column:ID子账号;primaryKey;AUTO_INCREMENT;"`
	O余额      int    `json:"余额" gorm:"column:余额;default:0"`
	O价格      string `json:"价格" gorm:"column:价格"`
	O父Name   string `json:"父Name" gorm:"column:父Name;default:0"`
}

func user_son_login(ctx *gin.Context) {
	var a user_son
	ctx.ShouldBindBodyWith(&a, binding.JSON)
	b := db_user_son.Where("name=?", a.Name).Where("password=?", a.Password).First(&a).RowsAffected
	父 := user{}
	db_user.Where("name=?", a.O父Name).First(&父)
	if b > 0 {
		ctx.JSON(http.StatusOK, gin.H{"state": true, "msg": "登录成功", "id": a.ID子账号, "id2": 父.ID, "余额": a.O余额, "价格": a.O价格})
	} else {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "用户名或者密码错误"})
	}
	// user_info_登录记录(a.Name, ctx.ClientIP())
}

// 注册
func user_son_register(ctx *gin.Context) {
	var a user_son
	ctx.BindJSON(&a)
	if len(a.Name) < 3 {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "用户名小于3位"})
		return

	}
	if len(a.Password) < 6 {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "密码小于6位"})
		return

	}
	b := db_user_son.Where("name=?", a.Name).First(&user{})

	if b.RowsAffected != 0 {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "注册失败,用户名已存在"})
		return
	}
	if b.Error != gorm.ErrRecordNotFound {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "未知错误"})
		return
	}
	db_user_son.Create(&a)
	ctx.JSON(http.StatusOK, gin.H{"state": true, "msg": "注册成功"})
}

func user_son_验证用户(ctx *gin.Context) bool {
	var a struct {
		Name     string
		Password string
	}
	err := ctx.ShouldBindBodyWith(&a, binding.JSON)
	if err != nil {
		return false
	}
	子账号信息 := user_son{}
	b := db_user_son.Where("name=?", a.Name).Where("password=?", a.Password).First(&子账号信息).RowsAffected
	if len(a.Password) < 3 || 子账号信息.Password != a.Password {
		ctx.Abort()
		return false
	}

	var data interface{}
	json.Unmarshal([]byte(子账号信息.O价格), &data)
	ctx.Set("子账号信息", 子账号信息)
	if b > 0 {
		return true
	} else {
		ctx.Abort()
		return false
	}
}
func user_son_取账号信息(ctx *gin.Context) user_son {
	// user_son_验证用户(ctx)
	c, _ := ctx.Get("子账号信息")
	子账号信息, _ := c.(user_son)
	return 子账号信息
}
func user_son_取价格(ctx *gin.Context) map[int]int {
	c, _ := ctx.Get("子账号信息")
	子账号信息, _ := c.(user_son)
	价格 := make(map[int]int)
	json.Unmarshal([]byte(子账号信息.O价格), &价格)
	return 价格
}
func user_son_日志(ID子账号 interface{}, 内容 string) {
	日志(fmt.Sprintf("log/子账号%v_%v", ID子账号, time.Now().Format("200601")), 内容)
}
func user_son_消费(账号 user_son, 金额 int, log string) {
	账号.O余额 = 账号.O余额 - 金额
	db_user_son.Select("余额").Updates(账号)
	user_son_日志(账号.ID子账号, fmt.Sprintf("余额:%v;%v", 账号.O余额, log))
}

func user_son_查询所有卡密(ctx *gin.Context) {
	账号 := user_son_取账号信息(ctx)
	var a struct {
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
	// b := db.Table("card_" + 账号.O父Name)
	查询所有卡密(ctx, 账号.ID子账号, 账号.O父Name, a.Software, a.Card_state, a.Available_time, a.Card, a.Notes)
}
func user_son_添加卡密(ctx *gin.Context) {
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
	账号 := user_son_取账号信息(ctx)
	价格表 := user_son_取价格(ctx)
	价格, _ := 价格表[a.Software]
	消费 := 价格 * a.Num * int(a.Available_time)
	if 账号.O余额 < 消费 {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "余额不足"})
		return
	}
	if !add_new_card(ctx, 账号.ID子账号, 账号.O父Name, a.Software, a.Available_time, a.Num, a.Latest_activation_time, a.Cards, a.Notes, a.Config_content, a.O指定类型) {
		return
	}
	user_son_消费(账号, 消费, fmt.Sprintf("加卡消费 %v=价格%v * 数量%v * 天%v", 消费, 价格, a.Num, a.Available_time))

}

func user_son_加时长(ctx *gin.Context) {
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
	账号 := user_son_取账号信息(ctx)
	if !add_card_time(ctx, 账号.ID子账号, 账号.O父Name, a.Cards, a.Add_time) {
		return
	}
	// 价格表 := user_son_取价格(ctx)
	// 价格, _ := 价格表[a.Software]
	user_son_日志(账号.ID子账号, fmt.Sprintf("加时消费 价格x * 数量%v * 天%v", len(a.Cards), a.Add_time))
}
func user_son_删除卡密(ctx *gin.Context) {
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
	账号 := user_son_取账号信息(ctx)
	delete_card(ctx, 账号.ID子账号, 账号.O父Name, a.Cards)
	user_son_日志(账号.ID子账号, fmt.Sprintf("删除卡密;数量%v;%v", len(a.Cards), a.Cards))
}

func user_son_修改卡密(ctx *gin.Context) {
	var b struct {
		Name string
	}
	ctx.ShouldBindBodyWith(&b, binding.JSON)

	账号 := user_son_取账号信息(ctx)

	var a struct {
		// Software   int
		Card       string
		Card_state int
	}
	err := ctx.ShouldBindBodyWith(&a, binding.JSON)
	if err != nil {
		fmt.Println(err)
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "数据错误!!"})
		return
	}
	修改卡密(ctx, 账号.ID子账号, 账号.O父Name, a.Card, a)
	user_son_日志(账号.ID子账号, fmt.Sprintf("修改卡密;%v", a.Card))
}
func user_son_查询软件列表(ctx *gin.Context) {
	var a struct {
		Name     string
		Password string
	}
	err := ctx.ShouldBindBodyWith(&a, binding.JSON)
	if err != nil {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "数据错误"})
		return
	}
	var results []map[string]interface{}
	账号 := user_son_取账号信息(ctx)
	db_software.Where("name = ?", 账号.O父Name).Order("ID").Select("ID", "Software", "Bulletin", "暂停扣时").Find(&results)
	ctx.JSON(http.StatusOK, gin.H{"state": true, "data": results})
}
func user_son_充值卡_生成(ctx *gin.Context) {
	账号 := user_son_取账号信息(ctx)
	价格表 := user_son_取价格(ctx)
	a := struct {
		Name     string
		Software int
		Add_time float64
		Num      int
		O充值次数    int `json:"充值次数"`
	}{}
	取josn参数表(ctx, &a)
	价格, _ := 价格表[a.Software]
	消费 := 价格 * a.Num * int(a.Add_time) * a.O充值次数
	if 账号.O余额 < 消费 {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "余额不足"})
		return
	}
	if !充值卡_生成(ctx, 账号.ID子账号, 账号.O父Name) {
		return
	}
	user_son_消费(账号, 消费, fmt.Sprintf("充值消费 消费%v=价格%v * 数量%v * 天%v * 次数%v", 消费, 价格, a.Num, a.Add_time, a.O充值次数))
}
func user_son_充值卡_查询(ctx *gin.Context) {
	账号 := user_son_取账号信息(ctx)
	充值卡_查询(ctx, 账号.ID子账号, 账号.O父Name)
}
func user_son_充值卡_修改(ctx *gin.Context) {
	账号 := user_son_取账号信息(ctx)
	充值卡_修改(ctx, 账号.ID子账号, 账号.O父Name)
}
func user_son_查询操作日志(ctx *gin.Context) {
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
	账号 := user_son_取账号信息(ctx)
	// 文件名1 :=
	文件1, err := os.ReadFile(fmt.Sprintf("log/子账号%v_%v", 账号.ID子账号, time.Now().Format("200601")))
	内容1 := "没有其他内容"
	if err == nil {
		内容1 = string(文件1)
	}
	文件2, err := os.ReadFile(fmt.Sprintf("log/子账号%v_%v", 账号.ID子账号, time.Now().AddDate(0, -1, 0).Format("200601")))
	内容2 := "没有其他内容"
	if err == nil {
		内容2 = string(文件2)
	}
	ctx.String(http.StatusOK, 内容1+"\n"+内容2)
}

func 设置子账号(ctx *gin.Context) {
	// var a struct {
	// 	Name string
	// }
	// if 取josn参数表(ctx, &a) {
	// 	return
	// }
	// data := user_son{}
	// db_user_son.Where("O父Name = ?", a.Name).Find(&data)

	var a struct {
		Name string
		Data struct {
			Name     string
			Password string
			ID子账号    int    `gorm:"column:ID子账号;primaryKey;AUTO_INCREMENT;"`
			O余额      int    `json:"余额" gorm:"column:余额;default:0"`
			O价格      string `json:"价格" gorm:"column:价格"`
			// O父Name   string `json:"父Name" gorm:"column:父Name;default:0"`
			O原始余额 int `json:"原始余额"`
		}
		// ID子账号 int
	}
	if 取josn参数表(ctx, &a) != nil {
		return
	}
	var b user_son
	db_user_son.Where("ID子账号 = ?", a.Data.ID子账号).Where("父Name = ?", a.Name).Select("password", "余额", "价格").First(&b)
	if b.O余额 != a.Data.O余额 && b.O余额 != a.Data.O原始余额 {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "授权账户余额有变动,请刷新后再修改余额"})
		return
	}
	db_user_son.Where("ID子账号 = ?", a.Data.ID子账号).Where("父Name = ?", a.Name).Select("password", "余额", "价格").Updates(a.Data)
	ctx.JSON(http.StatusOK, gin.H{"state": true, "msg": "修改成功"})
	if b.O余额 != a.Data.O余额 {
		user_son_日志(a.Data.ID子账号, fmt.Sprintf("修改余额 修改前:%v;修改后:%v", b.O余额, a.Data.O余额))
	}
}

func 查询子账号(ctx *gin.Context) {
	var a struct {
		Name string
	}
	if 取josn参数表(ctx, &a) != nil {
		return
	}
	data := []map[string]interface{}{}
	// db_user_son.Where("父Name = ?", a.Name).Select("*").Omit("父Name").Find(&data)
	db_user_son.Where("父Name = ?", a.Name).Select("ID子账号", "name", "password", "价格", "余额").Omit("父Name").Find(&data)
	ctx.JSON(http.StatusOK, gin.H{"state": true, "data": data})

}
