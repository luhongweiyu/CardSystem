package main

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

const 管理员注册配置刷新间隔 = 10 * time.Minute

// 管理员注册配置采用请求触发的惰性缓存。没有注册请求时不会定时读取文件；
// 有请求时，只有距离上次检查已满十分钟才重新加载独立配置实例，避免并发修改
// 全局 Viper 导致其他正在读取端口、数据库或限流配置的协程发生数据竞争。
var 管理员注册配置缓存 = struct {
	sync.Mutex
	上次检查 time.Time
	启用   bool
}{}

func 读取管理员注册开关(configPath string) (bool, error) {
	reader := viper.New()
	reader.SetConfigFile(configPath)
	if err := reader.ReadInConfig(); err != nil {
		return false, fmt.Errorf("读取注册配置失败: %w", err)
	}
	return reader.GetBool("管理员注册.启用"), nil
}

func 管理员注册已启用(now time.Time) (bool, error) {
	管理员注册配置缓存.Lock()
	defer 管理员注册配置缓存.Unlock()
	if !管理员注册配置缓存.上次检查.IsZero() {
		elapsed := now.Sub(管理员注册配置缓存.上次检查)
		if elapsed >= 0 && elapsed < 管理员注册配置刷新间隔 {
			return 管理员注册配置缓存.启用, nil
		}
	}
	configPath := viper.ConfigFileUsed()
	if configPath == "" {
		configPath = "./config.yaml"
	}
	管理员注册配置缓存.上次检查 = now
	enabled, err := 读取管理员注册开关(configPath)
	if err != nil {
		管理员注册配置缓存.启用 = false
		return false, err
	}
	管理员注册配置缓存.启用 = enabled
	return enabled, nil
}

type user struct {
	// 管理员名称会参与登录、动态卡密表名和访客链接定位，必须全局唯一。
	Name string `gorm:"size:32;not null;uniqueIndex:uk_admin_name"`
	// 按业务要求保留 password 原值，便于管理端做账号资料留档；任何登录
	// 校验都只使用 PasswordHash，且该字段永远不通过接口返回。
	Password     string `json:"-" gorm:"column:password;size:255;default:''"`
	PasswordHash string `json:"-" gorm:"column:password_hash;size:255;default:''"`
	ID           int    `gorm:"primaryKey;AUTO_INCREMENT;"`
}

// 登录
func user_login(ctx *gin.Context) {
	account, ok := 管理员_取账号信息(ctx)
	if !ok {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "登录状态错误"})
		return
	}
	session := 全局_登录会话.创建会话(account.Name, false)
	ctx.JSON(http.StatusOK, gin.H{
		"state": true, "msg": "登录成功", "id": account.ID, "name": account.Name,
		"api": 全局_运行状态.读取请求次数(account.Name), "token": session.Token,
		"expires_at": session.ExpiresAt,
	})
	user_info_登录记录(account.Name, ctx.ClientIP())
}

// 注册
func user_register(ctx *gin.Context) {
	enabled, configErr := 管理员注册已启用(time.Now())
	if configErr != nil {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "读取管理员注册配置失败"})
		return
	}
	if !enabled {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "管理员注册暂未开放"})
		return
	}
	var request struct {
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "请求数据错误"})
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	if !验证管理员名称(request.Name) {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "用户名只能使用3至32位字母、数字或下划线"})
		return

	}
	if err := 校验新密码(request.Password); err != nil {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": err.Error()})
		return

	}
	b := db_user.Where("name = ?", request.Name).First(&user{})

	if b.RowsAffected != 0 {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "注册失败,用户名已存在"})
		return
	}
	if !errors.Is(b.Error, gorm.ErrRecordNotFound) {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "未知错误"})
		return
	}
	hash, err := 生成密码哈希(request.Password)
	if err != nil {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "密码处理失败"})
		return
	}
	a := user{Name: request.Name, Password: request.Password, PasswordHash: hash}
	tableName, _ := 卡密数据表名(a.Name)
	// 先确认卡密表能够创建，再提交账号事务；失败时不会留下一个无法使用的账号。
	if err := db.Table(tableName).AutoMigrate(&卡密表样式{}); err != nil {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "初始化卡密数据失败"})
		return
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("user").Create(&a).Error; err != nil {
			return err
		}
		return user_info_创建(tx, a)
	}); err != nil {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "注册失败"})
		return
	}
	if err := user_刷新用户设置(a.Name); err != nil {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "账号已创建，但初始化运行设置失败"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"state": true, "msg": "注册成功"})
}

// 管理员_取账号信息集中读取认证中间件写入的账号，避免各接口重复类型断言。
func 管理员_取账号信息(ctx *gin.Context) (user, bool) {
	value, ok := ctx.Get("管理员账号")
	if !ok {
		return user{}, false
	}
	account, ok := value.(user)
	return account, ok
}

// 管理员_用户名返回数据库中的规范账号名。受保护接口应使用它访问动态卡密表，
// 避免 MySQL 大小写不敏感登录后，客户端输入大小写与真实表名不同。
func 管理员_用户名(ctx *gin.Context) string {
	account, ok := 管理员_取账号信息(ctx)
	if !ok {
		return ""
	}
	return account.Name
}

// user_change_password 要求再次提供当前密码。修改成功后会清除该账号的全部
// 旧会话并签发一个新令牌，当前页面无需退出后再重新登录。
func user_change_password(ctx *gin.Context) {
	account, ok := 管理员_取账号信息(ctx)
	if !ok {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "登录状态错误"})
		return
	}
	var request struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "数据错误"})
		return
	}
	if _, valid := 验证管理员账号(account.Name, request.CurrentPassword); !valid {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "当前密码错误"})
		return
	}
	if err := 校验新密码(request.NewPassword); err != nil {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": err.Error()})
		return
	}
	if request.CurrentPassword == request.NewPassword {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "新密码不能与当前密码相同"})
		return
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		return 保存管理员密码(tx, account.ID, request.NewPassword)
	}); err != nil {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "msg": "修改密码失败"})
		return
	}

	全局_登录会话.删除账号会话(account.Name, false)
	session := 全局_登录会话.创建会话(account.Name, false)
	ctx.JSON(http.StatusOK, gin.H{
		"state": true, "msg": "密码修改成功", "token": session.Token,
		"expires_at": session.ExpiresAt,
	})
}

// user_logout 删除当前令牌；重复退出是幂等操作。
func user_logout(ctx *gin.Context) {
	token := input(ctx, "token")
	if token != "" {
		全局_登录会话.删除会话(token)
	}
	ctx.JSON(http.StatusOK, gin.H{"state": true, "msg": "已退出登录"})
}
