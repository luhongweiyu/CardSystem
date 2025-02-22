package main

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sync"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
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
func 取josn参数表(ctx *gin.Context, obj any) error {
	// var obj struct {
	// 	Name     string
	// 	Password string
	// }
	err := ctx.ShouldBindBodyWith(obj, binding.JSON)
	if err != nil {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "数据错误"})
		ctx.Abort()
	}
	return err
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
	ctx.Writer.Header().Set("Connection", "close")
}
func 管理员验证(ctx *gin.Context) {
	if !user_验证用户(ctx) {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "管理员密码错误"})
		ctx.Abort()
	}
}
func ip验证(ctx *gin.Context) {
	if ctx.ClientIP() != viper.GetString("api.管理员ip") {
		ctx.Abort()
	}
}
func 子账号验证(ctx *gin.Context) {
	if !user_son_验证用户(ctx) {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "密码错误"})
		ctx.Abort()
	}
}

var 锁_请求防火墙 sync.Mutex

func 请求防火墙(name string) (res bool) {
	锁_请求防火墙.Lock()
	defer 锁_请求防火墙.Unlock()
	count := 全局_用户每小时请求次数[name]
	if count > 500000 {
		return false
	}
	全局_用户每小时请求次数[name] = count + 1
	return true
}

func 静态文件夹(router *gin.Engine, path string) {
	// files, _ := os.ReadDir("./assets")
	files, _ := os.ReadDir(path)
	for _, file := range files {
		if file.IsDir() {
			router.Static("/"+file.Name(), path+"/"+file.Name())
		} else {
			router.StaticFile("/"+file.Name(), path+"/"+file.Name())
		}
	}
	router.StaticFile("/", path+"/index.html") // 处理根路径的 index.html
}
func 启动网络() {
	if !viper.GetBool("dev") {
		fmt.Println("发行版")
		gin.SetMode(gin.ReleaseMode)
	}
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
		user.POST("/user_query_card", 管理员_查询所有卡密)
		user.POST("/user_add_soft", user_add_soft)
		user.POST("/user_del_soft", user_del_soft)
		user.POST("/user_query_soft_list", user_query_soft_list)
		user.POST("/user_modify_bulletin", user_modify_bulletin)
		// user.POST("/query_soft_list", user_query_soft_list)

		user.POST("/add_new_card", 管理员_add_new_card)
		user.POST("/delete_card", 管理员_delete_card)
		user.POST("/modify_card", modify_card)
		user.POST("/冻卡s", 管理员_冻卡s)
		user.POST("/add_card_time", 管理员_add_card_time)
		user.POST("/充值卡_生成", 充值卡_生成_管理员)
		user.POST("/充值卡_查询", 充值卡_查询_管理员)
		user.POST("/充值卡_修改", 充值卡_修改_管理员)
		user.POST("/query_log", 查询操作日志)
		// user.POST("/新增子账号", 新增子账号)
		user.POST("/设置子账号", 设置子账号)
		user.POST("/查询子账号", 查询子账号)
	}
	{
		api := router.Group("/api", ip验证)
		api.POST("/设置子账号_充值", 设置子账号_充值)
	}

	son := router.Group("/admin_son", 子账号验证)
	router.POST("/admin_son/user_register", user_son_register)
	{
		son.POST("/user_login", user_son_login)
		son.POST("/user_query_card", user_son_查询所有卡密)
		son.POST("/add_new_card", user_son_添加卡密)
		son.POST("/delete_card", user_son_删除卡密)
		son.POST("/add_card_time", user_son_加时长)
		son.POST("/modify_card", user_son_修改卡密)
		son.POST("/冻卡s", user_son_冻卡s)
		son.POST("/user_query_soft_list", user_son_查询软件列表)

		son.POST("/充值卡_生成", user_son_充值卡_生成)
		son.POST("/充值卡_查询", user_son_充值卡_查询)
		son.POST("/充值卡_修改", user_son_充值卡_修改)
		son.POST("/query_log", user_son_查询操作日志)

	}
	card := router.Group("/card", card_id获取用户设置, 卡密md5验证)
	{
		card.Any("/card_login", card_login)
		card.Any("/card_ping", card_ping)
		card.Any("/config", modify_card_configContent)
		// router.Any("/card/card_time_dec", 管理员验证)
	}
	card2 := router.Group("/card", card_id获取用户设置)
	{
		card2.Any("/query", 卡密_查询心跳)
		card2.Any("/bulletin", card_get_bulletin)
		card2.Any("/recharge", card_recharge)
	}
	visitor := router.Group("/visitor", visitor_验证对应id)
	{
		visitor.POST("/查询所有卡密", visitor_查询所有卡密)
		visitor.POST("/查询充值卡", visitor_查询充值卡)
		visitor.POST("/续费卡密", visitor_续费卡密)
		visitor.POST("/暂停时长", visitor_暂停时长)
		visitor.POST("/恢复时长", visitor_恢复时长)
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
	静态文件夹(router, "./assets")
	// router.Any("", func(c *gin.Context) {
	// 	c.Redirect(http.StatusMovedPermanently, viper.GetString("网站.管理页"))
	// })
	// router.Run(":802")
	router.Run(":" + viper.GetString("网站.端口"))
}
