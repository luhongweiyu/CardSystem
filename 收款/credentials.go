package main

import (
	"crypto/rsa"
	"crypto/x509"
	"embed"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"strings"
)

// 真实凭据文件只在本机构建时生成，不纳入版本控制。
//
//go:embed internal/credentials/assets/*
var credentialFiles embed.FS

type Credentials struct {
	AppID      string `json:"app_id"`
	PrivateKey string `json:"private_key"`
	PublicKey  string `json:"public_key"`
}

func loadCredentials() (Credentials, error) {
	data, err := credentialFiles.ReadFile("internal/credentials/assets/credentials.local.json")
	if err != nil {
		return Credentials{}, fmt.Errorf("程序未嵌入凭据，请使用 build.ps1 构建")
	}
	var c Credentials
	if err := json.Unmarshal(data, &c); err != nil || !digits(c.AppID) {
		return Credentials{}, fmt.Errorf("内置应用配置无效")
	}
	return c, nil
}

func keyDER(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if block, _ := pem.Decode([]byte(value)); block != nil {
		return block.Bytes, nil
	}
	return base64.StdEncoding.DecodeString(strings.Join(strings.Fields(value), ""))
}

func parsePrivateKey(value string) (*rsa.PrivateKey, error) {
	der, err := keyDER(value)
	if err != nil {
		return nil, fmt.Errorf("应用私钥编码无效")
	}
	if key, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return key, key.Validate()
	}
	parsed, err := x509.ParsePKCS8PrivateKey(der)
	if err == nil {
		if key, ok := parsed.(*rsa.PrivateKey); ok {
			return key, key.Validate()
		}
	}
	return nil, fmt.Errorf("应用私钥必须为 RSA PKCS#1 或 PKCS#8 私钥")
}

func parsePublicKey(value string) (*rsa.PublicKey, error) {
	der, err := keyDER(value)
	if err != nil {
		return nil, fmt.Errorf("支付宝公钥编码无效")
	}
	if key, err := x509.ParsePKCS1PublicKey(der); err == nil {
		return key, nil
	}
	if parsed, err := x509.ParsePKIXPublicKey(der); err == nil {
		if key, ok := parsed.(*rsa.PublicKey); ok {
			return key, nil
		}
	}
	return nil, fmt.Errorf("支付宝公钥必须为 RSA 公钥")
}
