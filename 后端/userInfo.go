package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

type user_info struct {
	ID           int
	Name         string `gorm:"primaryKey"`
	Ip           string
	CreatedAt    time.Time
	Login_time   time.Time
	Api_safe     bool   `gorm:"default:false"`
	Api_password string `gorm:"default:''"`
	// 联系方式
	Contact_information string
	// 开发者公告
	Notice string
	// Api_switch bool `gorm:"default:'123'"`
	// O手续费支付方    bool
	// O支付宝结算方式   bool

	// Token      string
}

func user_info_创建(a user) {
	db_user_info.Select("name", "CreatedAt", "id").Create(user_info{Name: a.Name, CreatedAt: time.Now(), ID: a.ID})
	user_刷新用户设置(a.Name)
	// db_user_info.Select("name", "create").Create(gin.H{})
}
func user_info_登录记录(用户名 string, ip string) {
	// a := map[string]interface{}{}
	// c := &a{}

	db_user_info.Updates(&user_info{Name: 用户名, Ip: ip, Login_time: time.Now()})
	// db_user.Model()
}
func user_get_info(ctx *gin.Context) {
	var a user
	ctx.ShouldBindBodyWith(&a, binding.JSON)
	data := map[string]interface{}{}
	db_user_info.Where("name = ?", a.Name).Select("name", "api_safe", "api_password", "contact_information", "notice").Find(&data)
	ctx.JSON(http.StatusOK, gin.H{"state": true, "data": data})
}
func user_update_info(ctx *gin.Context) {
	var a struct {
		Type     string
		Value    interface{}
		Name     string
		Password string
	}
	err := ctx.ShouldBindBodyWith(&a, binding.JSON)
	if err != nil {
		return
	}
	db_user_info.Where("name = ?", a.Name).Update(a.Type, a.Value)
	ctx.JSON(http.StatusOK, gin.H{"state": true})
	user_刷新用户设置(a.Name)
}
func user_刷新用户设置(name string) {
	a := user_info{}
	db_user_info.Where("name=?", name).First(&a)
	全局_用户设置_name[a.Name] = a
	全局_id对应name[a.ID] = a.Name
}
func user_query_card(ctx *gin.Context) {
	// var a struct {
	// 	Soft  string
	// 	State string
	// 	Value string
	// 	Card  string
	// 	Notes string
	// }
	// err := ctx.BindJSON(&a)
	// if err != nil {
	// 	return
	// }
	// if !user_验证用户(a.Name, a.Password) {
	// 	ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "用户名或者密码错误"})
	// 	return
	// }
	// db_user_info.Where("name = ?", a.Name).Update(a.Type, a.Value)
	// ctx.JSON(http.StatusOK, gin.H{"state": true})

}
