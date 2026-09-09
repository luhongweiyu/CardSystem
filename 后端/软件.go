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
	DefaultPeriodMinutes     int64  `json:"default_period_minutes"`
	DefaultPeriodSeconds     int64  `json:"-"`
	HeartbeatIntervalSeconds int64  `json:"heartbeat_interval_seconds"`
	OnlineGraceMinutes       *int64 `json:"online_grace_minutes"`
	PauseDeductMinutes       *int64 `json:"pause_deduct_minutes"`
}

// 软件列表项是管理端和代理账号共用的轻量返回结构。
type 软件列表项 struct {
	ID                       int       `json:"ID"`
	Software                 string    `json:"Software"`
	Bulletin                 string    `json:"Bulletin"`
	DefaultPeriodMinutes     int64     `json:"default_period_minutes"`
	HeartbeatIntervalSeconds int64     `json:"heartbeat_interval_seconds"`
	OnlineGraceMinutes       int64     `json:"online_grace_minutes"`
	PauseDeductMinutes       int64     `json:"pause_deduct_minutes"`
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
			DefaultPeriodMinutes: 秒转分钟(row.DefaultPeriodSeconds), HeartbeatIntervalSeconds: row.HeartbeatIntervalSeconds,
			OnlineGraceMinutes: row.OnlineGraceMinutes, PauseDeductMinutes: row.PauseDeductMinutes, CreatedAt: row.CreatedAt})
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
	if request.DefaultPeriodMinutes == 0 {
		request.DefaultPeriodMinutes = 默认点卡授权周期分钟
	}
	if request.HeartbeatIntervalSeconds == 0 {
		request.HeartbeatIntervalSeconds = 默认心跳周期秒
	}
	if request.OnlineGraceMinutes == nil {
		默认值 := 默认自动离线时间分钟
		request.OnlineGraceMinutes = &默认值
	} else if *request.OnlineGraceMinutes == 0 {
		*request.OnlineGraceMinutes = 默认自动离线时间分钟
	}
	if !点卡授权时长分钟有效(request.DefaultPeriodMinutes, false) {
		return request, fmt.Errorf("默认授权时长必须在%d至%d分钟之间", 最小点卡计费周期分钟, 最大点卡计费周期分钟)
	}
	request.DefaultPeriodSeconds = 分钟转秒(request.DefaultPeriodMinutes)
	if request.HeartbeatIntervalSeconds <= 0 || request.HeartbeatIntervalSeconds > 最大心跳周期秒 {
		return request, fmt.Errorf("心跳间隔必须在1至%d秒之间", 最大心跳周期秒)
	}
	if *request.OnlineGraceMinutes != 0 && (*request.OnlineGraceMinutes < 最小自动离线时间分钟 || *request.OnlineGraceMinutes > 最大自动离线时间分钟) {
		return request, fmt.Errorf("自动离线时间必须为0或%d至%d分钟", 最小自动离线时间分钟, 最大自动离线时间分钟)
	}
	if request.PauseDeductMinutes == nil {
		默认值 := int64(0)
		request.PauseDeductMinutes = &默认值
	}
	if *request.PauseDeductMinutes < 0 || *request.PauseDeductMinutes > 时长卡永久分钟 {
		return request, fmt.Errorf("暂停扣除时长必须为0至%d天", 时长卡永久分钟/1440)
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
			HeartbeatIntervalSeconds: request.HeartbeatIntervalSeconds, OnlineGraceMinutes: *request.OnlineGraceMinutes,
			PauseDeductMinutes: *request.PauseDeductMinutes}
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
	清除点卡计费配置缓存(admin, created.ID)
	成功提示管理端(ctx, gin.H{"msg": "创建成功", "data": 软件列表项{ID: created.ID, Software: created.Software, Bulletin: created.Bulletin, DefaultPeriodMinutes: 秒转分钟(created.DefaultPeriodSeconds), HeartbeatIntervalSeconds: created.HeartbeatIntervalSeconds, OnlineGraceMinutes: created.OnlineGraceMinutes, PauseDeductMinutes: created.PauseDeductMinutes, CreatedAt: created.CreatedAt}})
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
	tableName, err := 点卡数据表名(admin)
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	durationTableName, err := 时长卡数据表名(admin)
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	if err := 同步并删除软件点卡心跳缓存(admin, request.ID); err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	if err := 同步并删除软件时长卡心跳缓存(admin, request.ID); err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	var deletedCardCount int64
	var deletedDurationCardCount int64
	var current 软件
	if query := db.Table("software").Where("id = ? AND name = ?", request.ID, admin).First(&current); query.Error != nil {
		if errors.Is(query.Error, gorm.ErrRecordNotFound) {
			失败提示管理端(ctx, "软件不存在")
			return
		}
		失败提示管理端(ctx, "读取软件失败")
		return
	}
	if err := db.Table(tableName).Where("software = ?", request.ID).Count(&deletedCardCount).Error; err != nil {
		失败提示管理端(ctx, "统计关联卡密失败")
		return
	}
	if err := db.Table(durationTableName).Where("software = ?", request.ID).Count(&deletedDurationCardCount).Error; err != nil {
		失败提示管理端(ctx, "统计关联时长卡失败")
		return
	}
	// 删除软件使用多条独立语句，不把所有关联数据放进一个长事务。先删除软件
	// 行可阻止新的发卡或配置修改，后续语句再清理已经存在的关联数据。
	if result := db.Table("software").Where("id = ? AND name = ?", request.ID, admin).Delete(&软件{}); result.Error != nil || result.RowsAffected != 1 {
		失败提示管理端(ctx, "删除软件失败")
		return
	}
	if err := db.Table(tableName).Where("software = ?", request.ID).Delete(&点卡表样式{}).Error; err != nil {
		失败提示管理端(ctx, "删除关联卡密失败")
		return
	}
	if err := db.Table(durationTableName).Where("software = ?", request.ID).Delete(&时长卡表样式{}).Error; err != nil {
		失败提示管理端(ctx, "删除关联时长卡失败")
		return
	}
	if err := db.Table("point_device_session").Where("admin = ? AND software = ?", admin, request.ID).Delete(&点卡设备会话{}).Error; err != nil {
		失败提示管理端(ctx, "删除关联设备会话失败")
		return
	}
	if err := db.Table("point_period_price").Where("admin = ? AND software = ?", admin, request.ID).Delete(&点卡周期价格{}).Error; err != nil {
		失败提示管理端(ctx, "删除点卡计费方案失败")
		return
	}
	// 时长卡代理价格不属于卡密流水，软件删除后也必须一并清理；否则
	// 旧软件编号被重新使用时，代理可能意外继承旧价格锚点。
	if err := db.Table(时长卡代理价格表名).Where("admin = ? AND software = ?", admin, request.ID).Delete(&时长卡代理价格{}).Error; err != nil {
		失败提示管理端(ctx, "删除时长卡代理价格失败")
		return
	}
	if err := db.Table(时长充值卡表名).Where("admin = ? AND software = ?", admin, request.ID).Delete(&时长充值卡{}).Error; err != nil {
		失败提示管理端(ctx, "删除关联时长充值卡失败")
		return
	}
	// 流水是审计记录，软件删除后仍保留至 30 天清理任务执行，不影响其他软件查询。
	if err := 同步并删除软件点卡心跳缓存(admin, request.ID); err != nil {
		日志("log/启动记录.txt", "删除软件后同步心跳缓存失败:"+err.Error())
	}
	if err := 同步并删除软件时长卡心跳缓存(admin, request.ID); err != nil {
		日志("log/启动记录.txt", "删除软件后同步时长卡心跳缓存失败:"+err.Error())
	}
	清除点卡计费配置缓存(admin, request.ID)
	成功提示管理端(ctx, gin.H{"msg": "删除成功", "deleted_card_count": deletedCardCount, "deleted_duration_card_count": deletedDurationCardCount})
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
	// 心跳间隔和自动离线时间都属于软件设置。修改前统一同步并失效该软件的
	// 会话缓存，下一次心跳会读取修改后的完整配置。
	if err := 同步并删除软件点卡心跳缓存(admin, request.ID); err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	if err := 同步并删除软件时长卡心跳缓存(admin, request.ID); err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
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
		if request.DefaultPeriodMinutes == 0 {
			request.DefaultPeriodMinutes = 秒转分钟(current.DefaultPeriodSeconds)
		}
		if request.HeartbeatIntervalSeconds == 0 {
			request.HeartbeatIntervalSeconds = current.HeartbeatIntervalSeconds
		}
		if request.OnlineGraceMinutes == nil {
			当前值 := current.OnlineGraceMinutes
			if 当前值 == 0 {
				当前值 = 默认自动离线时间分钟
			}
			request.OnlineGraceMinutes = &当前值
		}
		if request.PauseDeductMinutes == nil {
			当前值 := current.PauseDeductMinutes
			request.PauseDeductMinutes = &当前值
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
		updates := map[string]interface{}{"software": request.Software, "bulletin": request.Bulletin, "default_period_seconds": request.DefaultPeriodSeconds, "heartbeat_interval_seconds": request.HeartbeatIntervalSeconds, "online_grace_minutes": *request.OnlineGraceMinutes, "pause_deduct_minutes": *request.PauseDeductMinutes}
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
	if cacheErr := 同步并删除软件点卡心跳缓存(admin, request.ID); cacheErr != nil {
		日志("log/启动记录.txt", "修改软件后同步心跳缓存失败:"+cacheErr.Error())
	}
	if cacheErr := 同步并删除软件时长卡心跳缓存(admin, request.ID); cacheErr != nil {
		日志("log/启动记录.txt", "修改软件后同步时长卡心跳缓存失败:"+cacheErr.Error())
	}
	清除点卡计费配置缓存(admin, request.ID)
	成功提示管理端(ctx, gin.H{"msg": "修改成功"})
}

func user_soft_验证(name string, soft int) bool {
	if !验证管理员名称(strings.TrimSpace(name)) || soft <= 0 {
		return false
	}
	var count int64
	return db_software.Where("name = ? AND id = ?", name, soft).Count(&count).Error == nil && count == 1
}

// 点卡获取公告是点卡客户端查询软件公告的公开接口。
func 点卡获取公告(ctx *gin.Context) {
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
	card, found, readErr := 读取点卡记录(cardContext.Name, cardContext.Card)
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
