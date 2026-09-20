package main

import (
	"fmt"
	"net/http"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"gorm.io/gorm"
)

type user_info struct {
	ID           int
	Name         string `gorm:"primaryKey"`
	Ip           string
	CreatedAt    time.Time
	Login_time   time.Time
	Api_safe     bool   `gorm:"default:false"`
	Api_password string `gorm:"default:''"`
	// 联系方式
	Contact_information string
	// 开发者公告
	Notice string
}

// user_info_创建与管理员主账号放在同一事务中，避免只创建其中一张表的数据。
func user_info_创建(tx *gorm.DB, a user) error {
	return tx.Table("user_info").Select("name", "created_at", "id").
		Create(&user_info{Name: a.Name, CreatedAt: time.Now(), ID: a.ID}).Error
}
func user_info_登录记录(用户名 string, ip string) {
	// 明确限定管理员名称，避免 GORM 因模型主键为空而生成无条件 UPDATE。
	db_user_info.Where("name = ?", 用户名).Updates(map[string]interface{}{
		"ip": ip, "login_time": time.Now(),
	})
}

// 管理员设置响应返回接口安全密码，供管理员找回并继续配置旧客户端。
// 管理员登录密码不属于此响应，避免混淆两类密码。
type 管理员设置响应 struct {
	Name               string `json:"name"`
	ApiSafe            bool   `json:"api_safe"`
	ApiPasswordSet     bool   `json:"api_password_set"`
	ApiPassword        string `json:"api_password"`
	ContactInformation string `json:"contact_information"`
	Notice             string `json:"notice"`
}

// 设置文本校验保留用户输入的空格；公告允许换行，密码和联系方式不允许
// 控制字符，避免控制台、日志或签名参数出现不可见内容。
func 设置文本合法(value string, maxRunes int, 允许换行 bool) bool {
	if maxRunes <= 0 || len([]rune(value)) > maxRunes {
		return false
	}
	for _, r := range value {
		if !unicode.IsControl(r) {
			continue
		}
		if 允许换行 && (r == '\n' || r == '\r' || r == '\t') {
			continue
		}
		return false
	}
	return true
}

func user_get_info(ctx *gin.Context) {
	account, ok := 管理员_取账号信息(ctx)
	if !ok {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "登录状态错误"})
		return
	}
	var current user_info
	if err := db_user_info.Where("name = ?", account.Name).First(&current).Error; err != nil {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "读取设置失败"})
		return
	}
	data := 管理员设置响应{
		Name: current.Name, ApiSafe: current.Api_safe, ApiPasswordSet: current.Api_password != "", ApiPassword: current.Api_password,
		ContactInformation: current.Contact_information, Notice: current.Notice,
	}
	ctx.JSON(http.StatusOK, gin.H{"state": true, "data": data})
}
func user_update_info(ctx *gin.Context) {
	var a struct {
		Type  string
		Value interface{}
		Name  string
	}
	err := ctx.ShouldBindBodyWith(&a, binding.JSON)
	if err != nil {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "数据错误"})
		return
	}
	account, ok := 管理员_取账号信息(ctx)
	if !ok {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "登录状态错误"})
		return
	}
	a.Name = account.Name
	允许修改字段 := map[string]bool{
		"api_safe":            true,
		"api_password":        true,
		"contact_information": true,
		"notice":              true,
	}
	if !允许修改字段[a.Type] {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "不允许修改该设置"})
		return
	}
	switch a.Type {
	case "api_password":
		password, valid := a.Value.(string)
		if !valid {
			ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "接口安全密码必须是文本"})
			return
		}
		if len([]byte(password)) > 256 || !设置文本合法(password, 256, false) {
			ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "接口安全密码不能超过256个字节"})
			return
		}
		if password == "" {
			var current user_info
			if err := db_user_info.Where("name = ?", a.Name).Select("api_safe").First(&current).Error; err != nil {
				ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "读取API安全设置失败"})
				return
			}
			if current.Api_safe {
				ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "请先关闭API安全模式，再清空安全密码"})
				return
			}
		}
		// 统一转成字符串写入数据库，避免 JSON 数字被数据库隐式转换后产生不直观的签名密码。
		a.Value = password
	case "contact_information":
		value, valid := a.Value.(string)
		if !valid || !设置文本合法(value, 500, false) {
			ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "联系方式格式不正确或超过500个字符"})
			return
		}
		a.Value = value
	case "notice":
		value, valid := a.Value.(string)
		if !valid || !设置文本合法(value, 5000, true) {
			ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "开发者公告格式不正确或超过5000个字符"})
			return
		}
		a.Value = value
	case "api_safe":
		// 前端历史上使用0/1，接口也接受布尔值，其他类型一律拒绝。
		enabled := false
		switch value := a.Value.(type) {
		case bool:
			enabled = value
		case float64:
			if value != 0 && value != 1 {
				ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "API安全开关只能为0或1"})
				return
			}
			enabled = value == 1
		default:
			ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "API安全开关格式不正确"})
			return
		}
		if enabled {
			var current user_info
			if err := db_user_info.Where("name = ?", a.Name).Select("api_password").First(&current).Error; err != nil {
				ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "读取API安全设置失败"})
				return
			}
			if current.Api_password == "" {
				ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "请先设置接口安全密码，再开启API安全模式"})
				return
			}
		}
	}
	if err := db_user_info.Where("name = ?", a.Name).Update(a.Type, a.Value).Error; err != nil {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "保存设置失败"})
		return
	}
	if err := user_刷新用户设置(a.Name); err != nil {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "设置已保存，但刷新运行配置失败"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"state": true})
}
func user_刷新用户设置(name string) error {
	a := user_info{}
	if err := db_user_info.Where("name = ?", name).First(&a).Error; err != nil {
		return fmt.Errorf("读取管理员设置失败: %w", err)
	}
	全局_运行状态.保存用户设置(a)
	return nil
}
