package main

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

var beijing = time.FixedZone("UTC+8", 8*60*60)

type Payment struct {
	LogID           string
	OrderID         string
	MerchantOrderID string
	Cents           int64
	At              time.Time
}

type PaymentSource interface {
	Query(context.Context, time.Time, time.Time) ([]Payment, error)
}

type AlipayClient struct {
	appID      string
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	http       *http.Client
	endpoint   string
}

func newAlipayClient(c Credentials, timeout time.Duration) (*AlipayClient, error) {
	privateKey, err := parsePrivateKey(c.PrivateKey)
	if err != nil {
		return nil, err
	}
	publicKey, err := parsePublicKey(c.PublicKey)
	if err != nil {
		return nil, err
	}
	return &AlipayClient{
		appID: c.AppID, privateKey: privateKey, publicKey: publicKey,
		http:     &http.Client{Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		endpoint: "https://openapi.alipay.com/gateway.do",
	}, nil
}

func (a *AlipayClient) Query(ctx context.Context, start, end time.Time) ([]Payment, error) {
	// 按用户确认的范围只请求第一页，不检查 total_size，不发起翻页。
	biz, err := json.Marshal(map[string]string{
		"start_time": start.In(beijing).Format("2006-01-02 15:04:05"),
		"end_time":   end.In(beijing).Format("2006-01-02 15:04:05"),
		"page_no":    "1", "page_size": "1000",
	})
	if err != nil {
		return nil, err
	}
	params := url.Values{
		"app_id": {a.appID}, "method": {"alipay.data.bill.accountlog.query"},
		"format": {"JSON"}, "charset": {"utf-8"}, "sign_type": {"RSA2"},
		"timestamp": {time.Now().In(beijing).Format("2006-01-02 15:04:05")},
		"version":   {"1.0"}, "biz_content": {string(biz)},
	}
	// 先对原始参数值签名，再对整个表单 URL 编码。
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+params.Get(key))
	}
	digest := sha256.Sum256([]byte(strings.Join(parts, "&")))
	signature, err := rsa.SignPKCS1v15(rand.Reader, a.privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return nil, fmt.Errorf("请求签名失败")
	}
	params.Set("sign", base64.StdEncoding.EncodeToString(signature))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")
	resp, err := a.http.Do(req)
	if err != nil {
		// 不将可能含有请求凭据/账务信息的原始 HTTP 错误透传给调用方。
		return nil, fmt.Errorf("支付宝网络请求失败")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("支付宝 HTTP 状态 %d", resp.StatusCode)
	}
	const maxResponse = 8 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse+1))
	if err != nil || len(data) > maxResponse {
		return nil, fmt.Errorf("支付宝响应读取失败或过大")
	}
	return a.parseResponse(data)
}

func (a *AlipayClient) parseResponse(data []byte) ([]Payment, error) {
	var envelope struct {
		Response json.RawMessage `json:"alipay_data_bill_accountlog_query_response"`
		Sign     string          `json:"sign"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil || len(envelope.Response) == 0 {
		return nil, fmt.Errorf("支付宝响应格式无效")
	}
	signature, err := base64.StdEncoding.DecodeString(envelope.Sign)
	if err != nil || len(signature) == 0 {
		return nil, fmt.Errorf("支付宝响应签名缺失或无效")
	}
	// RawMessage 保留原始 JSON 字节；重新序列化会破坏响应签名。
	digest := sha256.Sum256(envelope.Response)
	if err := rsa.VerifyPKCS1v15(a.publicKey, crypto.SHA256, digest[:], signature); err != nil {
		return nil, fmt.Errorf("支付宝响应验签失败")
	}
	var result struct {
		Code    string `json:"code"`
		SubCode string `json:"sub_code"`
		Details []struct {
			LogID           string `json:"account_log_id"`
			OrderID         string `json:"alipay_order_no"`
			MerchantOrderID string `json:"merchant_order_no"`
			Amount          string `json:"trans_amount"`
			Direction       string `json:"direction"`
			At              string `json:"trans_dt"`
		} `json:"detail_list"`
	}
	if err := json.Unmarshal(envelope.Response, &result); err != nil {
		return nil, fmt.Errorf("支付宝业务响应无法解析")
	}
	if result.Code != "10000" {
		return nil, fmt.Errorf("支付宝拒绝查询 code=%s sub_code=%s", result.Code, result.SubCode)
	}
	payments := make([]Payment, 0, len(result.Details))
	for _, item := range result.Details {
		if item.Direction != "收入" {
			continue
		}
		cents, err := yuanToCents(item.Amount)
		at, timeErr := time.ParseInLocation("2006-01-02 15:04:05", item.At, beijing)
		if err != nil || timeErr != nil || item.LogID == "" || (item.OrderID == "" && item.MerchantOrderID == "") {
			// 无法可靠识别的收入不参与自动认领。
			continue
		}
		payments = append(payments, Payment{
			LogID: item.LogID, OrderID: item.OrderID, MerchantOrderID: item.MerchantOrderID,
			Cents: cents, At: at,
		})
	}
	return payments, nil
}

func digits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func yuanToCents(value string) (int64, error) {
	parts := strings.Split(value, ".")
	if len(parts) > 2 || !digits(parts[0]) {
		return 0, fmt.Errorf("金额格式错误")
	}
	fraction := "00"
	if len(parts) == 2 {
		if !digits(parts[1]) || len(parts[1]) > 2 {
			return 0, fmt.Errorf("金额精度错误")
		}
		fraction = (parts[1] + "0")[:2]
	}
	// 直接拼接十进制分，避免浮点误差和乘法溢出。
	cents, err := strconv.ParseInt(parts[0]+fraction, 10, 64)
	if err != nil || cents <= 0 {
		return 0, fmt.Errorf("金额无效")
	}
	return cents, nil
}
