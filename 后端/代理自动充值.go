package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"gorm.io/gorm"
)

type 代理充值设置 struct {
	// 收款连接配置按管理员保存，只在管理员设置接口返回，代理响应显式挑选字段。
	VerifyURL    string `json:"verify_url" gorm:"size:1000;not null;default:''"`
	SharedSecret string `json:"shared_secret" gorm:"size:1000;not null;default:''"`
	Enabled      bool   `json:"enabled" gorm:"not null;default:false"`
	QRCode       string `json:"qr_code" gorm:"size:1000;not null;default:''"`
	Rate         string `json:"rate" gorm:"size:24;not null;default:'1'"`
	MinAmount    string `json:"min_amount" gorm:"size:24;not null;default:'0.01'"`
}

var 充值小数格式 = regexp.MustCompile(`^[0-9]{1,17}(\.[0-9]{1,2})?$`)
var 充值尾号格式 = regexp.MustCompile(`^[0-9]{6}$`)
var 收款核对客户端 = &http.Client{
	Timeout: 20 * time.Second,
	// 收款服务是固定地址，不跟随跳转转发共享密钥签名。
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

// 金额和倍率都按两位小数解析成整数，避免浮点误差影响到账余额。
func 充值小数转整数(value string) (int64, error) {
	if !充值小数格式.MatchString(value) {
		return 0, fmt.Errorf("必须填写正数，最多两位小数")
	}
	parts := strings.SplitN(value, ".", 2)
	fraction := "00"
	if len(parts) == 2 {
		fraction = (parts[1] + "0")[:2]
	}
	n, err := strconv.ParseInt(parts[0]+fraction, 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("数值必须大于0且不能超出范围")
	}
	return n, nil
}

func 计算自动充值余额(cents int64, rate string) (int64, error) {
	scaled, err := 充值小数转整数(rate)
	if err != nil {
		return 0, fmt.Errorf("充值倍率未正确配置")
	}
	amount := new(big.Int).Mul(big.NewInt(cents), big.NewInt(scaled))
	amount.Quo(amount, big.NewInt(10000))
	if !amount.IsInt64() || amount.Sign() <= 0 {
		return 0, fmt.Errorf("到账余额不足1或超出范围，请联系管理员")
	}
	return amount.Int64(), nil
}

func 读取代理充值设置(admin string) (代理充值设置, error) {
	var info user_info
	err := db_user_info.Select("recharge_enabled", "recharge_qr_code", "recharge_rate", "recharge_min_amount", "recharge_verify_url", "recharge_shared_secret").Where("name = ?", admin).First(&info).Error
	return info.Recharge, err
}

func 收款核对地址合法(endpoint string) bool {
	u, err := url.Parse(endpoint)
	return err == nil && len(endpoint) <= 1000 && 设置文本合法(endpoint, 1000, false) &&
		u.Host != "" && (u.Scheme == "http" || u.Scheme == "https") && u.User == nil && u.Fragment == ""
}

func 收款服务配置(settings 代理充值设置) (string, string, error) {
	if !收款核对地址合法(settings.VerifyURL) || settings.SharedSecret == "" {
		return "", "", fmt.Errorf("自动充值暂不可用，请联系管理员完善充值设置")
	}
	return settings.VerifyURL, settings.SharedSecret, nil
}

func 管理员_保存代理充值设置(ctx *gin.Context) {
	account, ok := 管理员_取账号信息(ctx)
	if !ok {
		失败提示管理端(ctx, "登录状态错误")
		return
	}
	var settings 代理充值设置
	if err := ctx.ShouldBindBodyWith(&settings, binding.JSON); err != nil {
		失败提示管理端(ctx, "充值设置格式不正确")
		return
	}
	settings.Rate = strings.TrimSpace(settings.Rate)
	settings.MinAmount = strings.TrimSpace(settings.MinAmount)
	if settings.MinAmount == "" {
		settings.MinAmount = "0.01"
	}
	if _, err := 充值小数转整数(settings.MinAmount); err != nil {
		失败提示管理端(ctx, "起充金额必须大于0，最多两位小数且不能超出范围")
		return
	}
	settings.QRCode = strings.TrimSpace(settings.QRCode)
	settings.VerifyURL = strings.TrimSpace(settings.VerifyURL)
	// 保存开关不要求服务已配置或在线；已填写的字段只检查格式。
	if settings.VerifyURL != "" && !收款核对地址合法(settings.VerifyURL) {
		失败提示管理端(ctx, "请填写收款查询接口的完整HTTP地址")
		return
	}
	if len(settings.SharedSecret) > 1000 || !设置文本合法(settings.SharedSecret, 1000, false) {
		失败提示管理端(ctx, "共享密钥不能包含控制字符且不能超过1000个字节")
		return
	}
	if _, err := 充值小数转整数(settings.Rate); err != nil {
		失败提示管理端(ctx, "充值倍率："+err.Error())
		return
	}
	if settings.QRCode != "" {
		u, err := url.Parse(settings.QRCode)
		local := strings.HasPrefix(settings.QRCode, "/") && !strings.HasPrefix(settings.QRCode, "//")
		if err != nil || len(settings.QRCode) > 1000 || !设置文本合法(settings.QRCode, 1000, false) || strings.Contains(settings.QRCode, "\\") || (!local && (u.Host == "" || (u.Scheme != "http" && u.Scheme != "https"))) {
			失败提示管理端(ctx, "请填写收款码图片的HTTP地址或以 / 开头的站内路径")
			return
		}
	}
	if err := db_user_info.Where("name = ?", account.Name).Updates(map[string]interface{}{
		"recharge_enabled": settings.Enabled, "recharge_qr_code": settings.QRCode, "recharge_rate": settings.Rate,
		"recharge_verify_url": settings.VerifyURL, "recharge_shared_secret": settings.SharedSecret,
		"recharge_min_amount": settings.MinAmount,
	}).Error; err != nil {
		失败提示管理端(ctx, "保存充值设置失败")
		return
	}
	if err := user_刷新用户设置(account.Name); err != nil {
		失败提示管理端(ctx, "设置已保存，但刷新运行配置失败")
		return
	}
	成功提示管理端(ctx, gin.H{"msg": "保存成功"})
}

func 代理账号_查询充值设置(ctx *gin.Context) {
	account := 代理账号_取账号信息(ctx)
	settings, err := 读取代理充值设置(account.Admin)
	if err != nil {
		失败提示管理端(ctx, "读取充值设置失败")
		return
	}
	// 代理只读取充值展示信息，不返回倍率及服务鉴权配置。
	成功提示管理端(ctx, gin.H{"enabled": settings.Enabled, "qr_code": settings.QRCode, "min_amount": settings.MinAmount})
}

func 代理账号_自动充值(ctx *gin.Context) {
	var request struct {
		Amount string `json:"amount"`
		Suffix string `json:"suffix"`
	}
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil || !充值尾号格式.MatchString(request.Suffix) {
		失败提示管理端(ctx, "请填写付款金额和6位订单号")
		return
	}
	cents, err := 充值小数转整数(request.Amount)
	if err != nil {
		失败提示管理端(ctx, "付款金额格式不正确")
		return
	}
	account := 代理账号_取账号信息(ctx)
	settings, err := 读取代理充值设置(account.Admin)
	if err != nil || !settings.Enabled {
		失败提示管理端(ctx, "自动充值未开放，请联系管理员")
		return
	}
	// 核对前检查起充金额，避免先认领付款再拒绝入账。
	minimum, err := 充值小数转整数(settings.MinAmount)
	if err != nil {
		失败提示管理端(ctx, "起充金额设置错误，请联系管理员")
		return
	}
	if cents < minimum {
		失败提示管理端(ctx, fmt.Sprintf("充值金额至少%d.%02d", minimum/100, minimum%100))
		return
	}
	// 先拒绝已知无法入账的金额，避免认领付款后才发现到账不足1。
	if _, err := 计算自动充值余额(cents, settings.Rate); err != nil {
		失败提示管理端(ctx, "充值金额过小或超出范围，请联系管理员")
		return
	}
	endpoint, secret, err := 收款服务配置(settings)
	if err != nil {
		失败提示管理端(ctx, "自动充值未配置，请联系管理员")
		return
	}
	body, _ := json.Marshal(struct {
		UserID string `json:"a"`
		Cents  int64  `json:"b"`
		Suffix string `json:"c"`
		Time   int64  `json:"d"`
	}{strconv.Itoa(account.ID), cents, request.Suffix, time.Now().Unix()})
	mac := hmac.New(md5.New, []byte(secret))
	mac.Write(body)
	req, err := http.NewRequestWithContext(ctx.Request.Context(), http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		失败提示管理端(ctx, "收款查询地址不正确，请联系管理员")
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", hex.EncodeToString(mac.Sum(nil)))
	resp, err := 收款核对客户端.Do(req)
	if err != nil {
		日志("log/启动记录.txt", fmt.Sprintf("自动充值核对请求异常;管理员:%s;代理:%d;金额分:%d", account.Admin, account.ID, cents))
		失败提示管理端(ctx, "暂时无法查询付款，请稍后重试")
		return
	}
	defer resp.Body.Close()
	var result struct {
		Success  bool `json:"msg"`
		Cooldown int  `json:"冷却"`
	}
	responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 4097))
	if readErr != nil || len(responseBody) > 4096 || resp.StatusCode != http.StatusOK || json.Unmarshal(responseBody, &result) != nil {
		日志("log/启动记录.txt", fmt.Sprintf("自动充值核对响应异常;管理员:%s;代理:%d;金额分:%d;HTTP:%d", account.Admin, account.ID, cents, resp.StatusCode))
		失败提示管理端(ctx, "暂时无法查询付款，请稍后重试")
		return
	}
	if !result.Success {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "暂未查到付款，请核对金额和订单号后重试", "cooldown": result.Cooldown})
		return
	}

	var balance, credited int64
	// 网络核对不占数据库事务。只有 true 才读取最新倍率；重复认领由收款服务拒绝。
	// 认领后入账失败按约定人工处理，不自动重试，也不使用请求取消信号中断入账。
	err = 执行余额事务(func(tx *gorm.DB) error {
		var currentSettings user_info
		if err := tx.Table("user_info").Select("recharge_rate").Where("name = ?", account.Admin).First(&currentSettings).Error; err != nil {
			return err
		}
		credited, err = 计算自动充值余额(cents, currentSettings.Recharge.Rate)
		if err != nil {
			return err
		}
		balance, err = 调整代理余额(tx, account.Admin, account.ID, credited,
			"操作:自动充值")
		return err
	})
	if err != nil {
		日志("log/启动记录.txt", fmt.Sprintf("自动充值已核对成功但入账失败;管理员:%s;代理:%d;金额分:%d;错误:%s", account.Admin, account.ID, cents, 清理拒绝日志字段(err.Error(), 500)))
		失败提示管理端(ctx, "已查到付款，但余额未增加，请联系管理员")
		return
	}
	成功提示管理端(ctx, gin.H{"msg": "充值成功", "balance": strconv.FormatInt(balance, 10), "credited": strconv.FormatInt(credited, 10)})
}
