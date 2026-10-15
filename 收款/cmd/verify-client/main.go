// 调用示例：仅使用 HTTP 和共享密钥，不需要支付宝凭据。
package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func main() {
	endpoint := flag.String("url", "http://127.0.0.1:8080/verify", "核对服务地址")
	user := flag.String("user", "", "数字用户 ID，0～99")
	cents := flag.Int64("cents", 0, "支付金额，整数分")
	suffix := flag.String("suffix", "", "支付宝订单号或商户订单号后 6 位数字")
	header := flag.String("header", "X-Signature", "签名请求头")
	flag.Parse()
	secret := os.Getenv("RECEIPT_SECRET")
	if secret == "" {
		fmt.Fprintln(os.Stderr, "请将服务端的共享密钥设置到 RECEIPT_SECRET 环境变量")
		os.Exit(1)
	}
	// 只序列化一次，签名和发送使用同一份字节。b 必须是 JSON 整数。
	body, err := json.Marshal(struct {
		User   string `json:"a"`
		Cents  int64  `json:"b"`
		Suffix string `json:"c"`
		Stamp  int64  `json:"d"`
	}{*user, *cents, *suffix, time.Now().Unix()})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	mac := hmac.New(md5.New, []byte(secret))
	_, _ = mac.Write(body)
	req, err := http.NewRequest(http.MethodPost, *endpoint, bytes.NewReader(body))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(*header, hex.EncodeToString(mac.Sum(nil)))
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "请求失败:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(os.Stdout, io.LimitReader(resp.Body, 16<<10))
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "\nHTTP 状态:", resp.StatusCode)
		os.Exit(1)
	}
}
