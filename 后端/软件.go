package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 软件请求只包含软件自身设置。点卡计费方案由 point_period_price 单独管理，
// 因而修改软件默认授权时长不会意外覆盖已有的其他计费方案。
type 软件请求 struct {
	ID                       int    `json:"id"`
	Software                 string `json:"software"`
	Bulletin                 string `json:"bulletin"`
	DefaultPeriodSeconds     int64  `json:"default_period_seconds"`
	HeartbeatIntervalSeconds int64  `json:"heartbeat_interval_seconds"`
}

// 软件列表项是管理端和代理账号共用的轻量返回结构。
type 软件列表项 struct {
	ID                       int       `json:"ID"`
	Software                 string    `json:"Software"`
	Bulletin                 string    `json:"Bulletin"`
	DefaultPeriodSeconds     int64     `json:"default_period_seconds"`
	HeartbeatIntervalSeconds int64     `json:"heartbeat_interval_seconds"`
	CreatedAt                time.Time `json:"created_at"`
}

func 读取软件列表(admin string) ([]软件列表项, error) {
	admin = strings.TrimSpace(admin)
	if !验证管理员名称(admin) {
		return nil, fmt.Errorf("管理员名称格式不正确")
	}
	var rows []软件
	if err := db_software.Where("name = ?", admin).Order("id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("查询软件列表失败")
	}
	result := make([]软件列表项, 0, len(rows))
	for _, row := range rows {
		result = append(result, 软件列表项{ID: row.ID, Software: row.Software, Bulletin: row.Bulletin,
			DefaultPeriodSeconds: row.DefaultPeriodSeconds, HeartbeatIntervalSeconds: row.HeartbeatIntervalSeconds, CreatedAt: row.CreatedAt})
	}
	return result, nil
}

// user_query_soft_list 保留原路由函数名，响应字段统一为纯点卡设置。
func user_query_soft_list(ctx *gin.Context) {
	admin := 管理员_用户名(ctx)
	rows, err := 读取软件列表(admin)
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	成功提示管理端(ctx, gin.H{"data": rows})
}

func 解析软件设置(request 软件请求, requireName bool) (软件请求, error) {
	normalizedName, nameValid := 规范化可显示文本(request.Software, 64)
	if !nameValid || (requireName && normalizedName == "") {
		return request, fmt.Errorf("软件名称不能为空且不能超过64个字符")
	}
	request.Software = normalizedName
	var bulletinValid bool
	request.Bulletin, bulletinValid = 规范化多行文本(request.Bulletin, 5000)
	if !bulletinValid {
		return request, fmt.Errorf("软件公告不能包含非法控制字符且不能超过5000个字符")
	}
	if request.DefaultPeriodSeconds == 0 {
		request.DefaultPeriodSeconds = 默认登录周期秒
	}
	if request.HeartbeatIntervalSeconds == 0 {
		request.HeartbeatIntervalSeconds = 默认心跳周期秒
	}
	if request.DefaultPeriodSeconds <= 0 || request.DefaultPeriodSeconds > 最大计费周期秒 {
		return request, fmt.Errorf("默认授权时长必须在1至%d秒之间", 最大计费周期秒)
	}
	if request.HeartbeatIntervalSeconds <= 0 || request.HeartbeatIntervalSeconds > 最大心跳周期秒 {
		return request, fmt.Errorf("心跳间隔必须在1至%d秒之间", 最大心跳周期秒)
	}
	return request, nil
}

func user_add_soft(ctx *gin.Context) {
	var request 软件请求
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil {
		失败提示管理端(ctx, "数据错误")
		return
	}
	request, err := 解析软件设置(request, true)
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	admin := 管理员_用户名(ctx)
	var count int64
	if err := db_software.Where("name = ? AND software = ?", admin, request.Software).Count(&count).Error; err != nil {
		失败提示管理端(ctx, "检查软件名称失败")
		return
	}
	if count > 0 {
		失败提示管理端(ctx, "重复的软件名")
		return
	}
	var created 软件
	err = db.Transaction(func(tx *gorm.DB) error {
		created = 软件{Name: admin, Software: request.Software, Bulletin: request.Bulletin,
			CreatedAt: time.Now(), DefaultPeriodSeconds: request.DefaultPeriodSeconds,
			HeartbeatIntervalSeconds: request.HeartbeatIntervalSeconds}
		if err := tx.Table("software").Create(&created).Error; err != nil {
			return fmt.Errorf("创建软件失败")
		}
		// 新软件自动提供一个可用的默认点卡计费方案，管理员可在“点卡计费方案”中修改。
		defaultPrice := 点卡周期价格{Admin: admin, Software: created.ID, PeriodSeconds: request.DefaultPeriodSeconds, Cost: 1, IsDefault: true, Enabled: true}
		if err := tx.Table("point_period_price").Create(&defaultPrice).Error; err != nil {
			return fmt.Errorf("初始化默认点卡计费方案失败")
		}
		return nil
	})
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	成功提示管理端(ctx, gin.H{"msg": "创建成功", "data": 软件列表项{ID: created.ID, Software: created.Software, Bulletin: created.Bulletin, DefaultPeriodSeconds: created.DefaultPeriodSeconds, HeartbeatIntervalSeconds: created.HeartbeatIntervalSeconds, CreatedAt: created.CreatedAt}})
}

func user_del_soft(ctx *gin.Context) {
	var request struct {
		ID int `json:"id"`
	}
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil || request.ID <= 0 {
		失败提示管理端(ctx, "软件编号不正确")
		return
	}
	admin := 管理员_用户名(ctx)
	tableName, err := 卡密数据表名(admin)
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	var deletedCardCount int64
	err = db.Transaction(func(tx *gorm.DB) error {
		var current 软件
		// 软件行是删除、生成卡密和修改计费设置之间的同步点。
		// 先锁软件再处理卡密，避免删除过程中并发生成出孤立卡密。
		if err := tx.Table("software").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND name = ?", request.ID, admin).First(&current).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("软件不存在")
			}
			return fmt.Errorf("读取软件失败")
		}
		if err := tx.Table(tableName).Where("software = ?", request.ID).Count(&deletedCardCount).Error; err != nil {
			return fmt.Errorf("统计关联卡密失败")
		}
		if err := tx.Table(tableName).Where("software = ?", request.ID).Delete(&卡密表样式{}).Error; err != nil {
			return fmt.Errorf("删除关联卡密失败")
		}
		if err := tx.Table("point_device_session").Where("admin = ? AND software = ?", admin, request.ID).Delete(&点卡设备会话{}).Error; err != nil {
			return fmt.Errorf("删除关联设备会话失败")
		}
		if err := tx.Table("point_period_price").Where("admin = ? AND software = ?", admin, request.ID).Delete(&点卡周期价格{}).Error; err != nil {
			return fmt.Errorf("删除点卡计费方案失败")
		}
		// 流水是审计历史，即使软件被删除也保留，不影响其他软件查询。
		if result := tx.Table("software").Where("id = ? AND name = ?", request.ID, admin).Delete(&软件{}); result.Error != nil || result.RowsAffected != 1 {
			return fmt.Errorf("删除软件失败")
		}
		return nil
	})
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	成功提示管理端(ctx, gin.H{"msg": "删除成功", "deleted_card_count": deletedCardCount})
}

func user_modify_bulletin(ctx *gin.Context) {
	var request 软件请求
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil {
		失败提示管理端(ctx, "数据错误")
		return
	}
	if request.ID <= 0 {
		失败提示管理端(ctx, "软件编号不正确")
		return
	}
	admin := 管理员_用户名(ctx)
	// 软件默认授权时长是唯一可信来源。修改默认授权时长时必须已经存在对应的启用方案，
	// 并在同一事务内同步 is_default 展示标记，避免出现两套互相矛盾的默认值。
	err := db.Transaction(func(tx *gorm.DB) error {
		var current 软件
		if query := tx.Table("software").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND name = ?", request.ID, admin).First(&current); query.Error != nil {
			return fmt.Errorf("软件不存在")
		}
		if request.Software == "" {
			request.Software = current.Software
		}
		if request.DefaultPeriodSeconds == 0 {
			request.DefaultPeriodSeconds = current.DefaultPeriodSeconds
		}
		if request.HeartbeatIntervalSeconds == 0 {
			request.HeartbeatIntervalSeconds = current.HeartbeatIntervalSeconds
		}
		var parseErr error
		request, parseErr = 解析软件设置(request, true)
		if parseErr != nil {
			return parseErr
		}
		var duplicate int64
		if err := tx.Table("software").Where("name = ? AND software = ? AND id <> ?", admin, request.Software, request.ID).Count(&duplicate).Error; err != nil {
			return fmt.Errorf("检查软件名称失败")
		}
		if duplicate > 0 {
			return fmt.Errorf("重复的软件名")
		}
		defaultChanged := request.DefaultPeriodSeconds != current.DefaultPeriodSeconds
		if defaultChanged {
			var priceCount int64
			if err := tx.Table("point_period_price").Where("admin = ? AND software = ? AND period_seconds = ? AND enabled = ?", admin, request.ID, request.DefaultPeriodSeconds, true).Count(&priceCount).Error; err != nil {
				return fmt.Errorf("检查默认授权时长方案失败")
			}
			if priceCount != 1 {
				return fmt.Errorf("请先添加并启用%d秒的点卡计费方案", request.DefaultPeriodSeconds)
			}
		}
		updates := map[string]interface{}{"software": request.Software, "bulletin": request.Bulletin, "default_period_seconds": request.DefaultPeriodSeconds, "heartbeat_interval_seconds": request.HeartbeatIntervalSeconds}
		if result := tx.Table("software").Where("id = ? AND name = ?", request.ID, admin).Updates(updates); result.Error != nil {
			return fmt.Errorf("修改软件失败")
		}
		if defaultChanged {
			if err := tx.Table("point_period_price").Where("admin = ? AND software = ?", admin, request.ID).Update("is_default", false).Error; err != nil {
				return fmt.Errorf("同步默认授权时长失败")
			}
			if result := tx.Table("point_period_price").Where("admin = ? AND software = ? AND period_seconds = ?", admin, request.ID, request.DefaultPeriodSeconds).Update("is_default", true); result.Error != nil || result.RowsAffected != 1 {
				return fmt.Errorf("同步默认授权时长失败")
			}
		}
		return nil
	})
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	成功提示管理端(ctx, gin.H{"msg": "修改成功"})
}

func user_soft_验证(name string, soft int) bool {
	if !验证管理员名称(strings.TrimSpace(name)) || soft <= 0 {
		return false
	}
	var count int64
	return db_software.Where("name = ? AND id = ?", name, soft).Count(&count).Error == nil && count == 1
}

// card_get_bulletin 是客户端查询软件公告的公开接口。
func card_get_bulletin(ctx *gin.Context) {
	softwareID, err := strconv.Atoi(input(ctx, "software"))
	if err != nil || softwareID <= 0 {
		失败提示(ctx, "software错误")
		return
	}
	value, _ := ctx.Get("card")
	cardContext, ok := value.(卡密请求上下文)
	if !ok {
		失败提示(ctx, "卡密上下文错误")
		return
	}
	card, found, readErr := 读取卡密记录(cardContext.Name, cardContext.Card)
	if readErr != nil {
		失败提示(ctx, readErr.Error())
		return
	}
	if !found {
		失败提示(ctx, "卡密不存在")
		return
	}
	if card.Software != softwareID {
		失败提示(ctx, "software与卡密不匹配")
		return
	}
	var item 软件
	if err := db_software.Where("id = ? AND name = ?", softwareID, cardContext.Name).First(&item).Error; err != nil {
		失败提示(ctx, "software错误")
		return
	}
	成功提示(ctx, gin.H{"bulletin": item.Bulletin, "software": item.ID})
}
