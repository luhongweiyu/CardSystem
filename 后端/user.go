package main

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"gorm.io/gorm"
)

type user struct {
	Name     string
	Password string
	ID       int `gorm:"primaryKey;AUTO_INCREMENT;"`
}

// 注册时间
// 登录时间
// 登录ip

// 登录
func user_login(ctx *gin.Context) {
	var a user
	ctx.ShouldBindBodyWith(&a, binding.JSON)
	fmt.Println(a)
	b := db_user.Where("name=?", a.Name).Where("password=?", a.Password).First(&a).RowsAffected
	if b > 0 {
		ctx.JSON(http.StatusOK, gin.H{"state": true, "msg": "登录成功", "id": a.ID, "api": 全局_用户每小时请求次数[a.Name]})
	} else {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "用户名或者密码错误"})
	}
	fmt.Println(ctx.ClientIP())
	user_info_登录记录(a.Name, ctx.ClientIP())
}

// 注册
func user_register(ctx *gin.Context) {
	var a user
	ctx.BindJSON(&a)
	if len(a.Name) < 3 {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "用户名小于3位"})
		return

	}
	if len(a.Password) < 6 {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "密码小于6位"})
		return

	}
	b := db_user.Where("name=?", a.Name).First(&user{})

	if b.RowsAffected != 0 {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "注册失败,用户名已存在"})
		return
	}
	if b.Error != gorm.ErrRecordNotFound {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "未知错误"})
		return
	}
	db_user.Create(&a)
	user_info_创建(a)
	db.Table("card_" + a.Name).AutoMigrate(&卡密表样式{})
	ctx.JSON(http.StatusOK, gin.H{"state": true, "msg": "注册成功"})
}

func user_验证用户(ctx *gin.Context) bool {
	var a struct {
		Name     string
		Password string
	}
	err := ctx.ShouldBindBodyWith(&a, binding.JSON)
	if err != nil {
		return false
	}
	b := db_user.Where("name=?", a.Name).Where("password=?", a.Password).First(&user{}).RowsAffected
	if b > 0 {
		return true
	} else {
		return false
	}
}
