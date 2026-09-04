package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/spf13/viper"
)

// 卡密请求上下文由卡端中间件写入，只保存已经规范化的卡密和所属管理员设置。
// 后续处理函数必须从这里取得租户范围，不能直接信任客户端提交的管理员名称。
type 卡密请求上下文 struct {
	Card string
	user_info
}

// 参数转字符串统一处理 JSON 数字，避免较大的整数被 fmt 输出成科学计数法。
func 参数转字符串(value interface{}) string {
	switch number := value.(type) {
	case float64:
		if !math.IsNaN(number) && !math.IsInf(number, 0) && math.Trunc(number) == number && number >= math.MinInt64 && number < math.MaxInt64 {
			return strconv.FormatInt(int64(number), 10)
		}
		return strconv.FormatFloat(number, 'f', -1, 64)
	case json.Number:
		return number.String()
	default:
		return fmt.Sprint(value)
	}
}

// input 按表单、查询参数、JSON body 的顺序读取同名参数，兼容客户端的
// POST JSON 和旧版 GET 调用。ShouldBindBodyWith 会缓存 body，后续中间件仍可读取。
func input(ctx *gin.Context, key string) string {
	if value, ok := ctx.GetPostForm(key); ok {
		return value
	}
	if value, ok := ctx.GetQuery(key); ok {
		return value
	}
	var body map[string]interface{}
	if err := ctx.ShouldBindBodyWith(&body, binding.JSON); err == nil {
		if value, ok := body[key]; ok && value != nil {
			return 参数转字符串(value)
		}
	}
	return ""
}

// use 设置基础安全响应头和请求体上限。系统支持 HTTP 部署，安全头不会强制 HTTPS。
func use(ctx *gin.Context) {
	ctx.Header("X-Content-Type-Options", "nosniff")
	ctx.Header("X-Frame-Options", "SAMEORIGIN")
	ctx.Header("Referrer-Policy", "same-origin")
	maxBodyBytes := viper.GetInt64("api.最大请求字节")
	if maxBodyBytes <= 0 {
		maxBodyBytes = 2 * 1024 * 1024
	}
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxBodyBytes)
	ctx.Next()
}

// 管理员验证在登录请求没有令牌时校验密码，后续请求只校验内存令牌。
// 非空令牌一旦失效会明确要求重新登录，不再用空密码回退校验。账号始终从
// 数据库重新读取，因此密码修改或账号删除后不会继续使用旧快照。
func 管理员验证(ctx *gin.Context) {
	var credentials struct {
		Name     string `json:"name"`
		Password string `json:"password"`
		Token    string `json:"token"`
	}
	if err := ctx.ShouldBindBodyWith(&credentials, binding.JSON); err != nil {
		失败提示管理端(ctx, "登录信息错误")
		ctx.Abort()
		return
	}
	if credentials.Token != "" {
		session, valid := 全局_登录会话.验证会话(credentials.Token, false)
		if !valid {
			失败提示管理端(ctx, "登录已过期，请重新登录")
			ctx.Abort()
			return
		}
		if credentials.Name != "" && !strings.EqualFold(credentials.Name, session.AccountName) {
			失败提示管理端(ctx, "登录会话与账号不匹配")
			ctx.Abort()
			return
		}
		var account user
		if err := db_user.Where("name = ?", session.AccountName).First(&account).Error; err != nil {
			失败提示管理端(ctx, "管理员账号不存在")
			ctx.Abort()
			return
		}
		ctx.Set("管理员账号", account)
		ctx.Next()
		return
	}
	account, ok := 验证管理员账号(credentials.Name, credentials.Password)
	if !ok {
		失败提示管理端(ctx, "管理员密码错误")
		ctx.Abort()
		return
	}
	ctx.Set("管理员账号", account)
	ctx.Next()
}

// 代理账号验证与管理员采用相同的令牌规则，但角色位必须是代理账号，
// 从而保证两个 API 前缀之间不能混用同一个令牌。
func 代理账号验证(ctx *gin.Context) {
	var credentials struct {
		Name     string `json:"name"`
		Password string `json:"password"`
		Token    string `json:"token"`
	}
	if err := ctx.ShouldBindBodyWith(&credentials, binding.JSON); err != nil {
		失败提示管理端(ctx, "登录信息错误")
		ctx.Abort()
		return
	}
	if credentials.Token != "" {
		session, valid := 全局_登录会话.验证会话(credentials.Token, true)
		if !valid {
			失败提示管理端(ctx, "登录已过期，请重新登录")
			ctx.Abort()
			return
		}
		if credentials.Name != "" && !strings.EqualFold(credentials.Name, session.AccountName) {
			失败提示管理端(ctx, "登录会话与账号不匹配")
			ctx.Abort()
			return
		}
		var account 代理账号记录
		if err := db_代理账号.Where("name = ?", session.AccountName).First(&account).Error; err != nil {
			失败提示管理端(ctx, "代理账号不存在")
			ctx.Abort()
			return
		}
		ctx.Set("代理账号信息", account)
		ctx.Next()
		return
	}
	account, ok := 验证代理账号(credentials.Name, credentials.Password)
	if !ok {
		失败提示管理端(ctx, "代理账号密码错误")
		ctx.Abort()
		return
	}
	ctx.Set("代理账号信息", account)
	ctx.Next()
}

func 请求防火墙(name string) bool {
	limit := viper.GetInt("api.每小时请求上限")
	if limit <= 0 {
		limit = 500000
	}
	return 全局_运行状态.增加请求次数(name, limit)
}

func 静态文件夹(router *gin.Engine, path string) {
	files, err := os.ReadDir(path)
	if err != nil {
		// 没有前端构建目录时仍可启动纯 API 服务。
		return
	}
	for _, file := range files {
		if file.IsDir() {
			// /visitor 同时也是访客 API 前缀，不能再注册 /visitor/*filepath
			// 这种通配静态路由，否则 Gin 会在启动时报告路由冲突。访客构建
			// 目录目前只有入口页需要直接访问，脚本和样式由根 /assets 提供。
			if file.Name() == "visitor" {
				indexPath := path + "/visitor/index.html"
				if indexContent, err := os.ReadFile(indexPath); err == nil {
					router.StaticFile("/visitor", indexPath)
					router.StaticFile("/visitor/", indexPath)
					// net/http.ServeFile 会把以 /index.html 结尾的请求自动重定向到
					// ./，可能丢失访客链接中的 center_id。入口文件很小，启动时
					// 读入内存并直接响应，可让三个入口地址都稳定返回 200。
					router.GET("/visitor/index.html", func(ctx *gin.Context) {
						ctx.Data(http.StatusOK, "text/html; charset=utf-8", indexContent)
					})
				}
				continue
			}
			router.Static("/"+file.Name(), path+"/"+file.Name())
		} else {
			router.StaticFile("/"+file.Name(), path+"/"+file.Name())
		}
	}
	if _, err := os.Stat(path + "/index.html"); err == nil {
		router.StaticFile("/", path+"/index.html")
	}
}

func 启动网络() error {
	if !viper.GetBool("dev") {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.Default()
	trustedProxies := viper.GetStringSlice("网站.可信代理")
	if len(trustedProxies) == 0 {
		trustedProxies = []string{"127.0.0.1", "::1"}
	}
	if err := router.SetTrustedProxies(trustedProxies); err != nil {
		return fmt.Errorf("可信代理配置错误: %w", err)
	}
	router.Use(use, cors.Default())

	// 注册不需要已登录会话；管理页面登录接口仍通过同一 /admin 前缀访问。
	router.POST("/user_register", user_register)
	router.POST("/admin/user_register", user_register)

	admin := router.Group("/admin", 管理员验证)
	{
		admin.POST("/user_login", user_login)
		admin.POST("/user_logout", user_logout)
		admin.POST("/user_change_password", user_change_password)
		admin.POST("/user_get_info", user_get_info)
		admin.POST("/user_update_info", user_update_info)
		admin.POST("/user_query_card", 管理员_查询所有卡密)
		admin.POST("/add_new_card", 管理员_add_new_card)
		admin.POST("/delete_card", 管理员_delete_card)
		admin.POST("/modify_card", modify_card)
		admin.POST("/冻卡s", 管理员_冻卡s)
		admin.POST("/query_log", 查询操作日志)
		admin.POST("/user_query_soft_list", user_query_soft_list)
		admin.POST("/user_add_soft", user_add_soft)
		admin.POST("/user_del_soft", user_del_soft)
		admin.POST("/user_modify_bulletin", user_modify_bulletin)
		admin.POST("/point_card/adjust", 管理员_调整点卡余额)
		admin.POST("/point_ledger/query", 管理员_查询点数流水)
		admin.POST("/point_period_price/list", 管理员_查询点卡周期价格)
		admin.POST("/point_period_price/save", 管理员_保存点卡周期价格)
		admin.POST("/point_period_price/delete", 管理员_删除点卡周期价格)
		admin.POST("/创建代理账号", 管理员_创建代理账号)
		admin.POST("/设置代理账号", 设置代理账号)
		admin.POST("/查询代理账号", 查询代理账号)
		admin.POST("/删除代理账号", 删除代理账号)
		admin.POST("/代理账号充值", 代理账号充值)
	}

	// 代理账号管理端使用独立前缀和角色校验，不能混用管理员令牌。
	agent := router.Group("/agent", 代理账号验证)
	{
		agent.POST("/user_login", 代理账号登录)
		agent.POST("/user_logout", user_logout)
		agent.POST("/user_query_card", 代理账号_查询所有卡密)
		agent.POST("/add_new_card", 代理账号_添加卡密)
		agent.POST("/delete_card", 代理账号_删除卡密)
		agent.POST("/modify_card", 代理账号_修改卡密)
		agent.POST("/冻卡s", 代理账号_批量修改卡密状态)
		agent.POST("/point_ledger/query", 代理账号_查询点数流水)
		agent.POST("/user_query_soft_list", 代理账号_查询软件列表)
		agent.POST("/query_log", 代理账号_查询操作日志)
	}

	// 客户端点卡接口。登录/心跳/退出/配置可启用可选的 MD5 签名，系统支持 HTTP。
	card := router.Group("/card", card_id获取用户设置, 卡密md5验证)
	card.Match([]string{"POST", "GET"}, "/card_login", card_login)
	card.Match([]string{"POST", "GET"}, "/card_ping", card_ping)
	card.Match([]string{"POST", "GET"}, "/card_logout", card_logout)
	card.Match([]string{"POST", "GET"}, "/config", modify_card_configContent)
	cardRead := router.Group("/card", card_id获取用户设置)
	cardRead.Match([]string{"POST", "GET"}, "/query", 卡密_查询心跳)
	cardRead.Match([]string{"POST", "GET"}, "/bulletin", card_get_bulletin)
	cardRead.Match([]string{"POST", "GET"}, "/period_prices", 卡端_查询周期价格)
	cardRead.Match([]string{"POST", "GET"}, "/point_ledger/query", 卡端_查询点数流水)

	// 访客页只读卡密和点数流水，不再提供时长充值、暂停或恢复接口。
	visitor := router.Group("/visitor", visitor_验证对应id)
	visitor.POST("/查询所有卡密", visitor_查询所有卡密)
	visitor.POST("/查询卡密", visitor_查询卡密详情)
	visitor.POST("/point_ledger/query", visitor_查询点数流水)

	静态文件夹(router, "./assets")
	port := strings.TrimSpace(viper.GetString("网站.端口"))
	if port == "" {
		port = "802"
	}
	server := &http.Server{Addr: ":" + port, Handler: router, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 120 * time.Second}
	return server.ListenAndServe()
}
