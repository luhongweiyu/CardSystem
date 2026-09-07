package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func Test计算卡密接口签名使用原始JSON(t *testing.T) {
	rawJSON := []byte(`{"card":"demo", "timestamp":1}`)
	expectedBytes := md5.Sum(append([]byte("secret"), rawJSON...))
	if actual := 计算卡密接口签名("secret", rawJSON); actual != hex.EncodeToString(expectedBytes[:]) {
		t.Fatalf("签名结果错误: %s", actual)
	}
	if 计算卡密接口签名("secret", rawJSON) == 计算卡密接口签名("secret", []byte(`{"timestamp":1,"card":"demo"}`)) {
		t.Fatal("字段顺序或空格变化后必须重新按实际 JSON 字节签名")
	}
}

func Test提取查询签名内容(t *testing.T) {
	tests := []struct {
		query, content string
	}{
		{"sign=abc123&card=a%20b", "card=a%20b"},
		{"card=a%20b&sign=abc123&timestamp=1&nonce=n", "card=a%20b&timestamp=1&nonce=n"},
		{"card=a%20b&sign=abc123", "card=a%20b"},
		{"sign=abc123", ""},
	}
	for _, test := range tests {
		content, sign, err := 提取查询签名内容(test.query)
		if err != nil || sign != "abc123" || content != test.content {
			t.Fatalf("查询签名内容提取错误: query=%q content=%q sign=%q err=%v", test.query, content, sign, err)
		}
	}
	if _, _, err := 提取查询签名内容("sign=a&sign=b"); err == nil {
		t.Fatal("重复 sign 必须拒绝")
	}
}

// Test卡密安全模式请求签名覆盖协议主路径：业务参数只能取已签名的 JSON
// 正文；响应在 JSON 内返回签名，不依赖客户端读取自定义响应头。
func Test卡密安全模式请求签名(t *testing.T) {
	gin.SetMode(gin.TestMode)
	password := "secret"
	router := gin.New()
	router.POST("/test", func(ctx *gin.Context) {
		_ = input(ctx, "card") // 模拟租户中间件读取参数，同时缓存原始正文。
		ctx.Set("card", 卡密请求上下文{Card: "body-card", user_info: user_info{Api_safe: true, Api_password: password}})
	}, 卡密md5验证, func(ctx *gin.Context) {
		成功提示(ctx, gin.H{"card": input(ctx, "card")})
	})

	rawJSON := []byte(`{"card":"body-card","timestamp":` + strconv.FormatInt(time.Now().Unix(), 10) + `,"nonce":"request-1"}`)
	request := httptest.NewRequest(http.MethodPost, "/test?card=query-card&sign="+计算卡密接口签名(password, rawJSON), bytes.NewReader(rawJSON))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	var response map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("响应不是合法 JSON: %v", err)
	}
	if response["state"] != true || response["nonce"] != "request-1" || response["card"] != "body-card" {
		t.Fatalf("安全请求响应错误: %s", recorder.Body.String())
	}
	responseSign, ok := response["sign"].(string)
	if !ok || responseSign == "" {
		t.Fatalf("响应缺少签名: %s", recorder.Body.String())
	}
	unsignedResponse := bytes.Replace(recorder.Body.Bytes(), []byte(`"sign":"`+responseSign+`"`), []byte(`"sign":""`), 1)
	if actual, expected := responseSign, 计算卡密接口签名(password, unsignedResponse); actual != expected {
		t.Fatalf("响应签名错误: actual=%s expected=%s", actual, expected)
	}
}

func Test卡密安全模式拒绝失效请求(t *testing.T) {
	gin.SetMode(gin.TestMode)
	password := "secret"
	serve := func(rawJSON []byte, sign string) string {
		router := gin.New()
		router.POST("/test", func(ctx *gin.Context) {
			_ = input(ctx, "card")
			ctx.Set("card", 卡密请求上下文{user_info: user_info{Api_safe: true, Api_password: password}})
		}, 卡密md5验证, func(ctx *gin.Context) { 成功提示(ctx, gin.H{"msg": "不应执行"}) })
		request := httptest.NewRequest(http.MethodPost, "/test?sign="+sign, bytes.NewReader(rawJSON))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		return recorder.Body.String()
	}

	oldJSON := []byte(`{"card":"demo","timestamp":` + strconv.FormatInt(time.Now().Add(-6*time.Minute).Unix(), 10) + `,"nonce":"old"}`)
	if body := serve(oldJSON, 计算卡密接口签名(password, oldJSON)); !strings.Contains(body, "时间不正确") {
		t.Fatalf("过期请求未被拒绝: %s", body)
	}
	missingNonce := []byte(`{"card":"demo","timestamp":` + strconv.FormatInt(time.Now().Unix(), 10) + `}`)
	if body := serve(missingNonce, 计算卡密接口签名(password, missingNonce)); !strings.Contains(body, "nonce格式不正确") {
		t.Fatalf("缺少 nonce 的请求未被拒绝: %s", body)
	}
	validJSON := []byte(`{"card":"demo","timestamp":` + strconv.FormatInt(time.Now().Unix(), 10) + `,"nonce":"changed"}`)
	if body := serve(validJSON, 计算卡密接口签名(password, []byte(`{}`))); !strings.Contains(body, "sign错误") {
		t.Fatalf("正文变更后的旧签名未被拒绝: %s", body)
	}
}

// GET 与不带 JSON 正文的 POST 都按查询参数模式签名，保证旧客户端无需支持
// POST JSON 也能开启接口安全模式。
func Test卡密安全模式兼容GET和POST查询参数(t *testing.T) {
	gin.SetMode(gin.TestMode)
	password := "secret"
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	unsignedQuery := "card=query-card&timestamp=" + timestamp + "&nonce=query-nonce"
	sign := 计算卡密接口签名(password, []byte(unsignedQuery))
	router := gin.New()
	setContext := func(ctx *gin.Context) {
		ctx.Set("card", 卡密请求上下文{user_info: user_info{Api_safe: true, Api_password: password}})
	}
	handler := func(ctx *gin.Context) { 成功提示(ctx, gin.H{"card": input(ctx, "card")}) }
	router.GET("/test", setContext, 卡密md5验证, handler)
	router.POST("/test", setContext, 卡密md5验证, handler)

	for _, method := range []string{http.MethodGet, http.MethodPost} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(method, "/test?card=query-card&sign="+sign+"&timestamp="+timestamp+"&nonce=query-nonce", nil)
		router.ServeHTTP(recorder, request)
		if !strings.Contains(recorder.Body.String(), `"state":true`) || !strings.Contains(recorder.Body.String(), `"card":"query-card"`) {
			t.Fatalf("%s 查询参数模式失败: %s", method, recorder.Body.String())
		}
	}
}

// 即使关闭请求强制验签，只要配置了接口口令，响应仍可让客户端校验来源。
func Test未开启安全模式仍计算响应签名(t *testing.T) {
	gin.SetMode(gin.TestMode)
	password := "secret"
	router := gin.New()
	router.GET("/test", func(ctx *gin.Context) {
		ctx.Set("card", 卡密请求上下文{user_info: user_info{Api_safe: false, Api_password: password}})
		成功提示(ctx, gin.H{"msg": "ok"})
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/test", nil))
	var response map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("响应不是合法 JSON: %v", err)
	}
	sign, _ := response["sign"].(string)
	unsignedResponse := bytes.Replace(recorder.Body.Bytes(), []byte(`"sign":"`+sign+`"`), []byte(`"sign":""`), 1)
	if sign == "" || sign != 计算卡密接口签名(password, unsignedResponse) {
		t.Fatalf("未开启安全模式的响应签名错误: %s", recorder.Body.String())
	}
}

func Test卡密配置字符限制(t *testing.T) {
	if err := 校验卡密配置内容(strings.Repeat("中", 200)); err != nil {
		t.Fatalf("200个中文字符应允许保存: %v", err)
	}
	if err := 校验卡密配置内容(strings.Repeat("中", 201)); err == nil {
		t.Fatal("201个中文字符必须拒绝")
	}
}

func Test卡密列表排序规则(t *testing.T) {
	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN:                       "root:password@tcp(127.0.0.1:3306)/gocard?charset=utf8mb4&parseTime=True&loc=Local",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true, SkipDefaultTransaction: true})
	if err != nil {
		t.Fatalf("初始化排序测试数据库失败: %v", err)
	}
	makeSQL := func(sorting 卡密列表排序参数) string {
		var rows []卡密表样式
		query := 应用卡密列表排序(db.Table("card_demo"), "card_demo", sorting, "admin", time.Now())
		executed := query.Limit(20).Find(&rows)
		return strings.ToLower(executed.Statement.SQL.String())
	}
	// 模拟列表先 Count、再追加排序的真实调用顺序，确认普通排序不会
	// 生成授权设备数子查询。
	var total int64
	base := db.Table("card_demo")
	base.Session(&gorm.Session{}).Count(&total)
	var countedRows []卡密表样式
	counted := 应用卡密列表排序(base, "card_demo", 卡密列表排序参数{字段: "create_time", 方向: "desc", 卡密方向: "asc"}, "admin", time.Now())
	countedExecuted := counted.Limit(20).Find(&countedRows)
	if sql := strings.ToLower(countedExecuted.Statement.SQL.String()); !strings.Contains(sql, "order by `create_time` desc, `card` asc") || strings.Contains(sql, "point_device_session") {
		t.Fatalf("不支持的授权设备排序应回退默认排序: %s", sql)
	}
	if sql := makeSQL(卡密列表排序参数{字段: "card", 方向: "desc", 卡密方向: "asc"}); !strings.Contains(sql, "order by `card` desc") || strings.Contains(sql, "card asc") {
		t.Fatalf("单独卡密排序不应追加第二个卡密排序: %s", sql)
	}
	if sql := makeSQL(卡密列表排序参数{字段: "point_balance", 方向: "desc", 卡密方向: "desc"}); !strings.Contains(sql, "point_balance` desc, `card` desc") {
		t.Fatalf("其他字段排序应把卡密作为第二排序: %s", sql)
	}
	if sql := makeSQL(卡密列表排序参数{字段: "create_time", 方向: "desc", 卡密方向: "asc"}); strings.Contains(sql, "point_device_session") || !strings.Contains(sql, "order by `create_time` desc") {
		t.Fatalf("卡密列表排序不应生成设备数量子查询: %s", sql)
	}
}
