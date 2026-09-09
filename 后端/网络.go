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

// input 对 JSON 请求优先读取正文，其他请求按表单、查询参数读取，兼容客户端的
// POST JSON 和旧版 GET 调用。ShouldBindBodyWith 会缓存 body，后续中间件仍可读取。
func input(ctx *gin.Context, key string) string {
	// JSON 接口优先使用请求正文。尤其在卡密安全模式下，签名覆盖的是原始 JSON；
	// 如果查询参数可以覆盖正文，攻击者就能保留合法签名却替换实际执行业务的参数。
	if strings.Contains(strings.ToLower(ctx.GetHeader("Content-Type")), "application/json") {
		var body map[string]interface{}
		if err := ctx.ShouldBindBodyWith(&body, binding.JSON); err == nil {
			if value, ok := body[key]; ok && value != nil {
				return 参数转字符串(value)
			}
		}
	}
	if value, ok := ctx.GetPostForm(key); ok {
		return value
	}
	if value, ok := ctx.GetQuery(key); ok {
		return value
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
		admin.POST("/point_card/list", 管理员_查询点卡卡密列表)
		admin.POST("/point_card/create", 管理员_添加点卡卡密)
		admin.POST("/point_card/delete", 管理员_删除点卡卡密)
		admin.POST("/point_card/save", 管理员_修改点卡)
		admin.POST("/point_card/state", 管理员_批量修改点卡状态)
		admin.POST("/query_log", 查询操作日志)
		admin.POST("/user_query_soft_list", user_query_soft_list)
		admin.POST("/user_add_soft", user_add_soft)
		admin.POST("/user_del_soft", user_del_soft)
		admin.POST("/user_modify_bulletin", user_modify_bulletin)
		admin.POST("/point_card/adjust", 管理员_调整点卡余额)
		admin.POST("/point_card/ledger", 管理员_查询点卡流水)
		admin.POST("/point_card/price/list", 管理员_查询点卡周期价格)
		admin.POST("/point_card/price/save", 管理员_保存点卡周期价格)
		admin.POST("/point_card/price/delete", 管理员_删除点卡周期价格)
		// 时长卡与点卡完全分开，使用独立表和独立管理接口。
		admin.POST("/duration_card/list", 管理员_查询时长卡列表)
		admin.POST("/duration_card/create", 管理员_添加时长卡)
		admin.POST("/duration_card/detail", 管理员_查询时长卡详情)
		admin.POST("/duration_card/save", 管理员_修改时长卡)
		admin.POST("/duration_card/delete", 管理员_删除时长卡)
		admin.POST("/duration_card/state", 管理员_批量修改时长卡状态)
		admin.POST("/duration_card/renew", 管理员_续费时长卡)
		admin.POST("/duration_card/agent_price/list", 管理员_查询时长卡代理价格)
		admin.POST("/duration_card/agent_price/save", 管理员_保存时长卡代理价格)
		admin.POST("/duration_card/agent_price/delete", 管理员_删除时长卡代理价格)
		admin.POST("/duration_recharge_card/list", 管理员_查询时长充值卡列表)
		admin.POST("/duration_recharge_card/create", 管理员_添加时长充值卡)
		admin.POST("/duration_recharge_card/detail", 管理员_查询时长充值卡详情)
		admin.POST("/duration_recharge_card/save", 管理员_修改时长充值卡)
		admin.POST("/duration_recharge_card/delete", 管理员_删除时长充值卡)
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
		agent.POST("/point_card/list", 代理账号_查询点卡卡密列表)
		agent.POST("/point_card/create", 代理账号_添加点卡卡密)
		agent.POST("/point_card/delete", 代理账号_删除点卡卡密)
		agent.POST("/point_card/save", 代理账号_修改点卡)
		agent.POST("/point_card/state", 代理账号_批量修改点卡状态)
		agent.POST("/point_card/ledger", 代理账号_查询点卡流水)
		agent.POST("/user_query_soft_list", 代理账号_查询软件列表)
		agent.POST("/query_log", 代理账号_查询操作日志)
		agent.POST("/duration_card/price/list", 代理账号_查询时长卡价格)
		agent.POST("/duration_card/price_preview", 代理账号_预览时长卡价格)
		agent.POST("/duration_card/create", 代理账号_添加时长卡)
		agent.POST("/duration_card/list", 代理账号_查询时长卡列表)
		agent.POST("/duration_card/detail", 代理账号_查询时长卡详情)
		agent.POST("/duration_card/save", 代理账号_修改时长卡)
		agent.POST("/duration_card/delete", 代理账号_删除时长卡)
		agent.POST("/duration_card/state", 代理账号_批量修改时长卡状态)
		agent.POST("/duration_card/renew", 代理账号_续费时长卡)
		agent.POST("/duration_recharge_card/list", 代理账号_查询时长充值卡列表)
		agent.POST("/duration_recharge_card/create", 代理账号_添加时长充值卡)
		agent.POST("/duration_recharge_card/detail", 代理账号_查询时长充值卡详情)
		agent.POST("/duration_recharge_card/save", 代理账号_修改时长充值卡)
		agent.POST("/duration_recharge_card/delete", 代理账号_删除时长充值卡)
	}

	// 客户端点卡接口只使用 /point_card 前缀，避免与时长卡模式产生任何
	// 路径歧义。登录/心跳/退出/配置可启用可选的 MD5 签名，系统支持 HTTP。
	pointCard := router.Group("/point_card", 卡端读取用户设置, 卡密md5验证)
	pointCard.Match([]string{"POST", "GET"}, "/card_login", 点卡登录)
	pointCard.Match([]string{"POST", "GET"}, "/card_ping", 点卡心跳)
	pointCard.Match([]string{"POST", "GET"}, "/card_logout", 点卡退出)
	pointCard.Match([]string{"POST", "GET"}, "/config", 点卡修改配置内容)
	pointCardRead := router.Group("/point_card", 卡端读取用户设置)
	pointCardRead.Match([]string{"POST", "GET"}, "/query", 点卡查询详情)
	pointCardRead.Match([]string{"POST", "GET"}, "/bulletin", 点卡获取公告)
	pointCardRead.Match([]string{"POST", "GET"}, "/period_prices", 点卡端_查询周期价格)
	pointCardRead.Match([]string{"POST", "GET"}, "/point_ledger/query", 点卡端_查询点卡流水)

	// 独立时长卡客户端接口。卡密和管理员的解析、签名规则与点卡一致，
	// 但后续业务只读取 duration_card_<管理员> 表。
	durationCard := router.Group("/duration_card", 卡端读取用户设置, 卡密md5验证)
	durationCard.Match([]string{"POST", "GET"}, "/card_login", durationCardLogin)
	durationCard.Match([]string{"POST", "GET"}, "/card_ping", durationCardPing)
	durationCard.Match([]string{"POST", "GET"}, "/card_logout", durationCardLogout)
	durationCard.Match([]string{"POST", "GET"}, "/query", durationCardQuery)
	durationCard.Match([]string{"POST", "GET"}, "/bulletin", durationCardBulletin)
	durationCard.Match([]string{"POST", "GET"}, "/config", durationCardConfig)
	durationCard.Match([]string{"POST", "GET"}, "/recharge", durationCardRecharge)

	// 访客接口保留旧中文路径，同时提供含义明确的新路径；两套路径调用
	// 同一实现，不会形成两份业务规则。
	visitor := router.Group("/visitor", visitor_验证对应id)
	visitor.POST("/查询所有卡密", 访客_查询所有点卡卡密)
	visitor.POST("/查询卡密", 访客_查询点卡卡密详情)
	visitor.POST("/point_ledger/query", 访客_查询点卡流水)
	visitor.POST("/查询时长卡", 访客_查询时长卡)
	visitor.POST("/duration_recharge_card/query", 访客_查询时长充值卡)
	visitor.POST("/duration_recharge_card/redeem", 访客_使用时长充值卡)
	visitor.POST("/duration_card/pause", 访客_暂停时长卡)
	visitor.POST("/duration_card/resume", 访客_恢复时长卡)
	visitor.POST("/查询充值卡", 访客_查询时长充值卡)
	visitor.POST("/续费卡密", 访客_使用时长充值卡)
	visitor.POST("/暂停时长", 访客_暂停时长卡)
	visitor.POST("/恢复时长", 访客_恢复时长卡)

	静态文件夹(router, "./assets")
	port := strings.TrimSpace(viper.GetString("网站.端口"))
	if port == "" {
		port = "802"
	}
	server := &http.Server{Addr: ":" + port, Handler: router, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 120 * time.Second}
	return server.ListenAndServe()
}
