package main

import (
	"fmt"
	"net/http"
	"regexp"
	"sync"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

type gin线程_变量_user_ifo struct {
	card string
	user_info
}

func input(ctx *gin.Context, id string) string {
	user_id, ok := ctx.GetPostForm(id)
	if !ok {
		user_id = ctx.Query(id)
	}
	return user_id
}
func 取参数表(ctx *gin.Context, query []string) (map[string]string, bool) {
	成功取出 := true
	t := map[string]string{}
	for i := 0; i < len(query); i++ {
		//query[i]
		user_id, err := ctx.GetPostForm(query[i])
		if !err {
			user_id, err = ctx.GetQuery(query[i])
		}
		if err {
			t[query[i]] = user_id
		} else {
			成功取出 = false
		}
	}
	return t, 成功取出
}

func 匹配卡密(ctx *gin.Context) []string {
	cards := input(ctx, "cards")
	// s, _ := regexp.Compile(`[\w+d]+`)
	cards_tab := regexp.MustCompile(`[\w+d]{7,}`).FindAllString(cards, -1)
	return cards_tab
}
func use(ctx *gin.Context) {
	fmt.Println("-----------------------------------")
	ctx.Header("Access-Control-Allow-Origin", "*")
}
func 管理员验证(ctx *gin.Context) {
	if !user_验证用户(ctx) {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "管理员密码错误"})
		ctx.Abort()
	}
}

var 锁_请求防火墙 sync.Mutex

func 请求防火墙(name string) (res bool) {
	锁_请求防火墙.Lock()
	defer 锁_请求防火墙.Unlock()
	count := 全局_用户每小时请求次数[name]
	if count > 50000 {
		return false
	}
	全局_用户每小时请求次数[name] = count + 1
	return true
}

func 启动网络() {
	router := gin.Default()
	router.Use(use)
	router.Use(cors.Default())
	// center_id
	// software
	// card
	// router.Any("/card_login", card_login)

	// center_id
	// software
	// card
	//
	// router.Any("/card_ping", card_ping)
	// // router.
	// router.POST("/user_login", 管理员验证, user_login)
	// router.POST("/user_register", 管理员验证, user_register)
	// router.POST("/user_get_info", 管理员验证, user_get_info)
	// router.POST("/user_update_info", 管理员验证, user_update_info)
	// // router.POST("/user_query_card",管理员验证, user_query_card)
	// router.POST("/user_query_card", 管理员验证, 查询所有卡密)
	// router.POST("/user_add_soft", 管理员验证, user_add_soft)
	// router.POST("/user_del_soft", 管理员验证, user_del_soft)
	// router.POST("/user_query_soft_list", 管理员验证, user_query_soft_list)
	// // router.POST("/user_query_soft_list", 管理员验证,user_query_soft_list)
	user := router.Group("/admin", 管理员验证)
	router.POST("/admin/user_register", user_register)
	{
		user.POST("/user_login", user_login)
		user.POST("/user_get_info", user_get_info)
		user.POST("/user_update_info", user_update_info)
		// user.POST("/query_card", user_query_card)
		user.POST("/user_query_card", 查询所有卡密)
		user.POST("/user_add_soft", user_add_soft)
		user.POST("/user_del_soft", user_del_soft)
		user.POST("/user_query_soft_list", user_query_soft_list)
		user.POST("/user_modify_bulletin", user_modify_bulletin)
		// user.POST("/query_soft_list", user_query_soft_list)

		user.POST("/add_new_card", add_new_card)
		user.POST("/delete_card", delete_card)
		user.POST("/modify_card", modify_card)
		user.POST("/add_card_time", add_card_time)

	}
	card := router.Group("/card", card_id获取用户设置)
	{
		card.Any("/card_login", 卡密md5验证, card_login)
		card.Any("/card_ping", 卡密md5验证, card_ping)
		card.Any("/query", 卡密_查询心跳)
		card.Any("/config", 卡密md5验证, modify_card_configContent)
		// router.Any("/card/card_time_dec", 管理员验证)
		card.Any("/bulletin", card_get_bulletin)
	}

	// admin := router.Group("/admin", 管理员验证)
	// {

	// 	admin.Any("/查询所有卡密", 查询所有卡密)
	// 	//center_id
	// 	//software
	// 	//cards
	// 	//cards_num		数量
	// 	//cards_time
	// 	admin.Any("/add_new_card", add_new_card)

	// 	//cards
	// 	//center_id
	// 	// add_time
	// 	admin.Any("/add_card_time", add_card_time)

	// 	//cards
	// 	//center_id
	// 	admin.Any("/delete_card", delete_card)

	// 	admin.Any("/modify_card", modify_card)
	// }
	router.Any("", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, viper.GetString("网站.管理页"))
	})
	// router.Run(":802")
	router.Run(":" + viper.GetString("网站.端口"))
}
