package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5" // 请求方指定 HMAC-MD5；支付宝侧使用 RSA2。
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// 请求字段固定在代码中，不写入 YAML：
// a：用户 ID，纯数字字符串，数值 0～99；01 和 1 使用相同额度。
// b：支付金额，JSON 正整数，单位分；不接受字符串、小数或指数形式。
// c：支付宝订单号或商户订单号后 6 位，数字字符串；保留前导零。
// d：Unix 秒时间戳，JSON 正整数。
func parseVerifyRequest(body []byte) (VerifyRequest, error) {
	d := json.NewDecoder(bytes.NewReader(body))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return VerifyRequest{}, fmt.Errorf("请求体不是 JSON 对象")
	}
	fields := make(map[string]json.RawMessage, 4)
	for d.More() {
		token, err = d.Token()
		if err != nil {
			return VerifyRequest{}, fmt.Errorf("字段解析失败")
		}
		key, ok := token.(string)
		if !ok || len(key) != 1 || !strings.Contains("abcd", key) {
			return VerifyRequest{}, fmt.Errorf("未知字段")
		}
		if _, exists := fields[key]; exists {
			return VerifyRequest{}, fmt.Errorf("重复字段")
		}
		var value json.RawMessage
		if err := d.Decode(&value); err != nil {
			return VerifyRequest{}, fmt.Errorf("字段值格式错误")
		}
		fields[key] = value
	}
	if _, err := d.Token(); err != nil || len(fields) != 4 {
		return VerifyRequest{}, fmt.Errorf("字段不完整")
	}
	if _, err := d.Token(); err != io.EOF {
		return VerifyRequest{}, fmt.Errorf("JSON 存在多余内容")
	}
	var id, suffix string
	if err := json.Unmarshal(fields["a"], &id); err != nil || !digits(id) || len(id) > 20 {
		return VerifyRequest{}, fmt.Errorf("用户编号格式错误")
	}
	uid, err := strconv.ParseUint(id, 10, 64)
	if err != nil || uid >= 100 {
		return VerifyRequest{}, fmt.Errorf("用户编号超出范围")
	}
	if err := json.Unmarshal(fields["c"], &suffix); err != nil || len(suffix) != 6 || !digits(suffix) {
		return VerifyRequest{}, fmt.Errorf("订单尾号格式错误")
	}
	amountText, stampText := string(fields["b"]), string(fields["d"])
	if !digits(amountText) || !digits(stampText) {
		return VerifyRequest{}, fmt.Errorf("金额或时间戳必须是整数")
	}
	cents, err := strconv.ParseInt(amountText, 10, 64)
	if err != nil || cents <= 0 {
		return VerifyRequest{}, fmt.Errorf("金额无效")
	}
	stamp, err := strconv.ParseInt(stampText, 10, 64)
	if err != nil || stamp <= 0 {
		return VerifyRequest{}, fmt.Errorf("时间戳无效")
	}
	return VerifyRequest{UserID: int(uid), Cents: cents, Suffix: suffix, Stamp: stamp}, nil
}

func newRouter(cfg Config, service *Service) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	r.Use(gin.CustomRecovery(func(c *gin.Context, recovered any) {
		slog.Error("请求处理异常")
		c.AbortWithStatusJSON(http.StatusInternalServerError, Result{})
	}))
	r.POST(cfg.Path, func(c *gin.Context) {
		invalid := func(status int, reason string) {
			slog.Warn("请求被拒绝", "reason", reason)
			c.AbortWithStatusJSON(status, Result{Error: "参数错误"})
		}
		mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
		if err != nil || mediaType != "application/json" {
			invalid(http.StatusBadRequest, "Content-Type 无效")
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			invalid(http.StatusBadRequest, "请求体过大或读取失败")
			return
		}
		signature, err := hex.DecodeString(c.GetHeader(cfg.SignatureHeader))
		mac := hmac.New(md5.New, []byte(cfg.Secret))
		_, _ = mac.Write(body)
		if err != nil || !hmac.Equal(signature, mac.Sum(nil)) {
			invalid(http.StatusUnauthorized, "请求验签失败")
			return
		}
		request, err := parseVerifyRequest(body)
		if err != nil {
			invalid(http.StatusBadRequest, err.Error())
			return
		}
		now := service.now()
		skew := int64(cfg.TimestampSkew / time.Second)
		if request.Stamp < now.Unix()-skew || request.Stamp > now.Unix()+skew {
			invalid(http.StatusBadRequest, "时间戳过期或超前")
			return
		}
		// 未来 60 秒的请求还可能在接收后 120 秒有效，按签名时间计算过期点。
		expires := time.Unix(request.Stamp, 0).Add(cfg.TimestampSkew + time.Second)
		if !service.acceptReplay(hex.EncodeToString(signature), expires, now) {
			invalid(http.StatusBadRequest, "签名重放")
			return
		}
		c.JSON(http.StatusOK, service.Verify(c.Request.Context(), request))
	})
	return r
}
