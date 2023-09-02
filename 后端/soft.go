package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

type software struct {
	ID        int
	Name      string
	Software  string
	CreatedAt time.Time
	Bulletin  string
}

func user_query_soft_list(ctx *gin.Context) {
	var a struct {
		Name     string
		Password string
	}
	err := ctx.ShouldBindBodyWith(&a, binding.JSON)
	if err != nil {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "数据错误"})
		return
	}
	var results []struct {
		ID       int
		Software string
		Bulletin *string
	}
	db_software.Where("name = ?", a.Name).Order("ID").Find(&results)
	ctx.JSON(http.StatusOK, gin.H{"state": true, "data": results})
}
func user_add_soft(ctx *gin.Context) {
	var a struct {
		Name     string //`gorm:"column:user"`
		Password string
		Software string
	}

	err := ctx.ShouldBindBodyWith(&a, binding.JSON)
	if err != nil {
		return
	}
	var results []map[string]interface{}
	db_software.Where("name = ?", a.Name).Where("software = ?", a.Software).Find(&results)
	if len(results) != 0 {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "重复的软件名"})
		return
	}
	db_software.Create(map[string]interface{}{
		"name":       a.Name,
		"software":   a.Software,
		"created_at": time.Now(),
	})
	ctx.JSON(http.StatusOK, gin.H{"state": true, "msg": "创建成功"})

}
func user_del_soft(ctx *gin.Context) {
	var a struct {
		Name string //`gorm:"column:user"`
		ID   int
	}

	err := ctx.ShouldBindBodyWith(&a, binding.JSON)
	if err != nil {
		return
	}
	db_software.Where("id = ?", a.ID).Where("name = ?", a.Name).Delete(&a)
	db.Table("card_"+a.Name).Where("software = ?", a.ID).Delete(&gin.H{})
	ctx.JSON(http.StatusOK, gin.H{"state": true, "msg": "删除成功"})

}
func user_modify_bulletin(ctx *gin.Context) {
	var a struct {
		Name     string //`gorm:"column:user"`
		ID       int
		Bulletin *string
		Software string
	}
	err := ctx.ShouldBindBodyWith(&a, binding.JSON)
	if err != nil {
		return
	}
	db_software.Where("id = ?", a.ID).Where("name = ?", a.Name).Updates(a)
	ctx.JSON(http.StatusOK, gin.H{"state": true, "msg": "修改成功"})

}
func user_soft_验证(name string, soft int) bool {
	var a []map[string]interface{}
	db_software.Where("name = ?", name).Where("ID = ?", soft).Find(&a)
	if len(a) > 0 {
		return true
	} else {
		return false
	}
}
func card_get_bulletin(ctx *gin.Context) {
	software := input(ctx, "software")
	if software == "" {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "code": 0, "data": "software错误"})
		return
	}
	a := struct {
		Bulletin string
	}{}
	db_software.Where("id = ?", software).Find(&a)
	ctx.JSON(http.StatusOK, gin.H{"state": true, "code": 1, "bulletin": a.Bulletin})

}
