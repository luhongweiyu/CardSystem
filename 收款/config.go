package main

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Listen          string
	Path            string
	Timeout         time.Duration
	Lookback        time.Duration
	GlobalCooldown  time.Duration
	FastQuota       int
	UserCooldown    time.Duration
	UsedTTL         time.Duration
	Secret          string
	TimestampSkew   time.Duration
	SignatureHeader string
}

func loadConfig(path string) (Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	if err := v.ReadInConfig(); err != nil {
		return Config{}, fmt.Errorf("读取配置失败: %w", err)
	}
	c := Config{
		Listen: v.GetString("服务.监听地址"), Path: v.GetString("服务.请求路径"),
		FastQuota: v.GetInt("查询.快速查询额度"), Secret: v.GetString("鉴权.共享密钥"),
		SignatureHeader: v.GetString("鉴权.签名请求头"),
	}
	for name, dst := range map[string]*time.Duration{
		"查询.接口超时": &c.Timeout, "查询.回查范围": &c.Lookback,
		"查询.全局冷却": &c.GlobalCooldown, "查询.额度耗尽后冷却": &c.UserCooldown,
		"内存.保留时间": &c.UsedTTL, "鉴权.时间戳允许偏差": &c.TimestampSkew,
	} {
		d, err := time.ParseDuration(v.GetString(name))
		if err != nil || d < time.Second || d%time.Second != 0 {
			return Config{}, fmt.Errorf("配置 %s 必须为至少 1 秒的整秒时长", name)
		}
		*dst = d
	}
	if _, _, err := net.SplitHostPort(c.Listen); err != nil {
		return Config{}, fmt.Errorf("监听地址格式错误: %w", err)
	}
	if !strings.HasPrefix(c.Path, "/") || strings.ContainsAny(c.Path, "*:?\r\n ") {
		return Config{}, fmt.Errorf("请求路径必须是以 / 开头的固定路径")
	}
	if c.FastQuota < 1 || c.FastQuota > 1000 {
		return Config{}, fmt.Errorf("快速查询额度必须为 1 至 1000")
	}
	if c.Lookback > 31*24*time.Hour || c.UsedTTL < c.Lookback {
		return Config{}, fmt.Errorf("回查范围不能超过 31 天，保留时间不能短于回查范围")
	}
	if c.TimestampSkew > 24*time.Hour {
		return Config{}, fmt.Errorf("时间戳允许偏差过大")
	}
	if len(c.Secret) < 32 || c.Secret == "__GENERATE_SECRET__" {
		return Config{}, fmt.Errorf("共享密钥必须设置为至少 32 字节的随机字符串")
	}
	if c.SignatureHeader == "" || !validHeaderName(c.SignatureHeader) {
		return Config{}, fmt.Errorf("签名请求头名称无效")
	}
	return c, nil
}

func validHeaderName(s string) bool {
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("!#$%&'*+-.^_`|~", c)) {
			return false
		}
	}
	return true
}
