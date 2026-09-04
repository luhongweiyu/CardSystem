package main

import (
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	新密码最小字节数 = 6
	// bcrypt 只处理前 72 个字节，显式限制可避免用户设置了实际无法完整校验的密码。
	新密码最大字节数 = 72
)

// 校验新密码用于注册或修改密码，并与 bcrypt 的最大输入长度保持一致。
func 校验新密码(password string) error {
	passwordLength := len([]byte(password))
	if passwordLength < 新密码最小字节数 {
		return fmt.Errorf("密码不能少于%d个字节", 新密码最小字节数)
	}
	if passwordLength > 新密码最大字节数 {
		return fmt.Errorf("密码不能超过%d个字节", 新密码最大字节数)
	}
	return nil
}

// 生成密码哈希统一使用 bcrypt 默认成本，兼顾当前系统规模下的安全性和登录速度。
func 生成密码哈希(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// 验证密码只比较 bcrypt 哈希。按当前业务要求，数据库另外保留 password
// 原值，但它不参与认证，也不会从接口返回。
func 验证密码(passwordHash string, inputPassword string) bool {
	return passwordHash != "" && bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(inputPassword)) == nil
}

// 验证管理员账号按名称读取账号并比较密码哈希。
func 验证管理员账号(name string, inputPassword string) (user, bool) {
	var account user
	name = strings.TrimSpace(name)
	if err := db_user.Where("name = ?", name).First(&account).Error; err != nil {
		return account, false
	}
	if !验证密码(account.PasswordHash, inputPassword) {
		return account, false
	}
	return account, true
}

// 验证代理账号采用与管理员相同的哈希校验策略。
func 验证代理账号(name string, inputPassword string) (代理账号记录, bool) {
	var account 代理账号记录
	name = strings.TrimSpace(name)
	if err := db_代理账号.Where("name = ?", name).First(&account).Error; err != nil {
		return account, false
	}
	if !验证密码(account.PasswordHash, inputPassword) {
		return account, false
	}
	return account, true
}

// 保存代理账号密码只在管理员明确填写新密码时调用，空字符串表示保持原密码。
func 保存代理账号密码(tx *gorm.DB, accountID int, admin string, password string) error {
	if password == "" {
		return nil
	}
	if err := 校验新密码(password); err != nil {
		return err
	}
	hash, err := 生成密码哈希(password)
	if err != nil {
		return err
	}
	result := tx.Model(&代理账号记录{}).Where("id = ? AND admin = ?", accountID, admin).
		Updates(map[string]interface{}{"password": password, "password_hash": hash})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("代理账号不存在")
	}
	return nil
}

// 保存管理员密码只写入哈希，业务模型从不接触明文密码列。
func 保存管理员密码(tx *gorm.DB, accountID int, password string) error {
	if err := 校验新密码(password); err != nil {
		return err
	}
	hash, err := 生成密码哈希(password)
	if err != nil {
		return err
	}
	result := tx.Table("user").Where("id = ?", accountID).
		Updates(map[string]interface{}{"password": password, "password_hash": hash})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("管理员账号不存在")
	}
	return nil
}
