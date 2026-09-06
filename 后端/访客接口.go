package main

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 访客接口只允许通过管理员公开的 center_id 定位租户，不接受客户端直接
// 指定管理员名称。center_id 来自主账号 ID，查询结果不会泄露账号设置。
func visitor_验证对应id(ctx *gin.Context) {
	centerID, err := strconv.Atoi(input(ctx, "center_id"))
	if err != nil || centerID <= 0 {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "code": 0, "msg": "center_id错误"})
		ctx.Abort()
		return
	}
	name := 全局_运行状态.读取用户名(centerID)
	if name == "" {
		var account user
		if query := db_user.Where("id = ?", centerID).First(&account); query.Error != nil {
			ctx.JSON(http.StatusOK, gin.H{"state": false, "code": 0, "msg": "center_id错误"})
			ctx.Abort()
			return
		}
		name = account.Name
		_ = user_刷新用户设置(name)
	}
	settings, ok := 全局_运行状态.读取用户设置(name)
	if !ok {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "code": 0, "msg": "管理员设置不存在"})
		ctx.Abort()
		return
	}
	if !请求防火墙(name) {
		ctx.JSON(http.StatusOK, gin.H{"state": false, "code": 0, "msg": "api次数超限"})
		ctx.Abort()
		return
	}
	ctx.Set("访客管理员", name)
	ctx.Set("访客设置", settings)
	ctx.Next()
}

func 访客管理员(ctx *gin.Context) (string, bool) {
	name, ok := ctx.Get("访客管理员")
	if !ok {
		return "", false
	}
	admin, ok := name.(string)
	return admin, ok && 验证管理员名称(admin)
}

func 访客读取卡密(ctx *gin.Context) (string, string, 卡密表样式, error) {
	admin, ok := 访客管理员(ctx)
	if !ok {
		return "", "", 卡密表样式{}, fmt.Errorf("访客上下文错误")
	}
	card := strings.ToLower(strings.TrimSpace(input(ctx, "card")))
	if !卡密格式规则.MatchString(card) {
		return "", "", 卡密表样式{}, fmt.Errorf("卡密格式不正确")
	}
	tableName, err := 卡密数据表名(admin)
	if err != nil {
		return "", "", 卡密表样式{}, err
	}
	var row 卡密表样式
	query := db.Table(tableName).Where("card = ?", card).First(&row)
	if errors.Is(query.Error, gorm.ErrRecordNotFound) {
		return "", "", row, fmt.Errorf("卡密不存在")
	}
	if query.Error != nil {
		return "", "", row, fmt.Errorf("查询卡密失败")
	}
	return admin, card, row, nil
}

// visitor_查询所有卡密保留原路由名称，但只允许按完整卡密精确查询。
// 卡密本身是客户端凭证，公开页面绝不能通过空条件或模糊后缀枚举卡密。
func visitor_查询所有卡密(ctx *gin.Context) {
	admin, ok := 访客管理员(ctx)
	if !ok {
		失败提示访客(ctx, "访客上下文错误")
		return
	}
	keyword := strings.ToLower(strings.TrimSpace(input(ctx, "card")))
	if !卡密格式规则.MatchString(keyword) {
		失败提示访客(ctx, "请输入完整且格式正确的卡密")
		return
	}
	tableName, err := 卡密数据表名(admin)
	if err != nil {
		失败提示访客(ctx, err.Error())
		return
	}
	// 管理端备注可能包含内部客户信息，访客查询只返回卡密自身状态。
	query := db.Table(tableName).Select("card, create_time, use_time, software, card_state, point_balance").Where("card = ?", keyword)
	var rows []卡密访客项
	if err := query.Limit(1).Find(&rows).Error; err != nil {
		失败提示访客(ctx, "查询卡密失败")
		return
	}
	成功提示访客(ctx, gin.H{"data": rows})
}

type 卡密访客项 struct {
	Card         string     `json:"card"`
	CreateTime   time.Time  `json:"create_time"`
	UseTime      *time.Time `json:"use_time"`
	Software     int        `json:"software"`
	CardState    int        `json:"card_state"`
	PointBalance int64      `json:"point_balance"`
}

// visitor_查询卡密详情是新路径的实现；旧中文路径继续调用同一逻辑。
func visitor_查询卡密详情(ctx *gin.Context) {
	admin, card, row, err := 访客读取卡密(ctx)
	if err != nil {
		失败提示访客(ctx, err.Error())
		return
	}
	设备统计, err := 查询卡密设备统计(admin, card, row.Software, time.Now())
	if err != nil {
		失败提示访客(ctx, err.Error())
		return
	}
	status := "正常"
	if row.Card_state == 卡密状态_冻结 {
		status = "冻结"
	}
	text := fmt.Sprintf("卡密:%s\n软件:%d\n点数余额:%d\n授权设备:%d\n在线设备:%d\n状态:%s", row.Card, row.Software, row.Point_balance, 设备统计.AuthorizedCount, 设备统计.OnlineCount, status)
	成功提示访客(ctx, gin.H{"data": text, "card": row.Card, "software": row.Software, "point_balance": row.Point_balance, "card_state": row.Card_state, "authorized_device_count": 设备统计.AuthorizedCount, "online_device_count": 设备统计.OnlineCount, "devices": 设备统计.Devices})
}

func visitor_查询点数流水(ctx *gin.Context) {
	admin, card, row, err := 访客读取卡密(ctx)
	if err != nil {
		失败提示访客(ctx, err.Error())
		return
	}
	softwareID, _ := strconv.Atoi(input(ctx, "software"))
	if softwareID > 0 && softwareID != row.Software {
		失败提示访客(ctx, "软件与卡密不匹配")
		return
	}
	page, _ := strconv.Atoi(input(ctx, "page"))
	pageSize, _ := strconv.Atoi(input(ctx, "page_size"))
	page, pageSize = 规范化流水分页(page, pageSize)
	rows, total, err := 查询点数流水记录(db_point_ledger.Where("admin = ? AND card = ?", admin, card), page, pageSize)
	if err != nil {
		失败提示访客(ctx, "查询点数流水失败")
		return
	}
	成功提示访客(ctx, gin.H{"data": 点数流水展示列表(rows, false), "num": total, "page": page, "page_size": pageSize, "balance": row.Point_balance})
}

func 失败提示访客(ctx *gin.Context, message string) {
	ctx.JSON(http.StatusOK, gin.H{"state": false, "code": 0, "msg": message})
}

func 成功提示访客(ctx *gin.Context, data interface{}) {
	if object, ok := data.(gin.H); ok {
		object["state"] = true
		object["code"] = 1
		ctx.JSON(http.StatusOK, object)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"state": true, "code": 1, "data": data})
}
