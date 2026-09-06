package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func Test参数转字符串保留JSON整数格式(t *testing.T) {
	if actual := 参数转字符串(float64(1788060000)); actual != "1788060000" {
		t.Fatalf("时间戳不应转换为科学计数法: %q", actual)
	}
	if actual := 参数转字符串(float64(1.25)); actual != "1.25" {
		t.Fatalf("小数参数转换错误: %q", actual)
	}
}

// JSON 参数必须覆盖同名查询参数，否则签名虽然验证的是正文，业务实际使用的
// 却可能是 URL 中未签名的值，等同于允许绕过请求内容签名。
func TestInput对JSON请求优先读取正文(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/test?card=query-card", bytes.NewBufferString(`{"card":"body-card"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	if actual := input(ctx, "card"); actual != "body-card" {
		t.Fatalf("JSON正文参数应优先于查询参数，得到 %q", actual)
	}
}

// 访客入口和 /visitor API 共用前缀。这个测试确保静态文件只注册精确
// 的 GET 路径，不会与 API 路由冲突，同时兼容带或不带末尾斜杠的访问。
func Test静态文件夹兼容访客入口(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	visitorDir := filepath.Join(root, "visitor")
	if err := os.Mkdir(visitorDir, 0755); err != nil {
		t.Fatalf("创建访客目录失败: %v", err)
	}
	index := []byte("visitor-entry")
	if err := os.WriteFile(filepath.Join(visitorDir, "index.html"), index, 0644); err != nil {
		t.Fatalf("写入访客入口失败: %v", err)
	}

	router := gin.New()
	router.POST("/visitor/查询卡密", func(ctx *gin.Context) { ctx.Status(http.StatusNoContent) })
	静态文件夹(router, root)

	for _, path := range []string{"/visitor", "/visitor/", "/visitor/index.html"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK || recorder.Body.String() != string(index) {
			t.Fatalf("访客静态入口 %s 响应错误: status=%d body=%q", path, recorder.Code, recorder.Body.String())
		}
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/visitor/查询卡密", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("访客 API 路由被静态路由覆盖: status=%d", recorder.Code)
	}
}
