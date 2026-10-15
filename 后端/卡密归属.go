package main

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func 管理员_批量修改点卡归属(ctx *gin.Context) {
	管理员_批量修改卡密归属(ctx, "点卡", 修改点卡归属)
}

func 管理员_批量修改时长卡归属(ctx *gin.Context) {
	管理员_批量修改卡密归属(ctx, "时长卡", 修改时长卡归属)
}

// 仅注册到管理员路由；逐卡提交，单张失败不影响其他卡密。
func 管理员_批量修改卡密归属(ctx *gin.Context, kind string, modify func(string, string, int) error) {
	var request struct {
		Cards   []string `json:"cards"`
		AgentID *int     `json:"agent_id"`
	}
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil || request.AgentID == nil || *request.AgentID < 0 {
		失败提示管理端(ctx, "请选择归属账号")
		return
	}
	if len(request.Cards) == 0 || len(request.Cards) > 1000 {
		失败提示管理端(ctx, "单次操作卡密数量必须在1至1000之间")
		return
	}
	account, ok := 管理员_取账号信息(ctx)
	if !ok {
		失败提示管理端(ctx, "登录状态错误")
		return
	}
	agentID := *request.AgentID
	if agentID > 0 {
		if err := 校验时长卡代理账号归属(db, account.Name, agentID); err != nil {
			失败提示管理端(ctx, err.Error())
			return
		}
	}
	success, failed := []string{}, []string{}
	seen := make(map[string]bool, len(request.Cards))
	for _, raw := range request.Cards {
		card := strings.ToLower(strings.TrimSpace(raw))
		if seen[card] {
			continue
		}
		seen[card] = true
		if !卡密格式规则.MatchString(card) {
			failed = append(failed, raw)
			continue
		}
		if err := modify(account.Name, card, agentID); err != nil {
			failed = append(failed, card)
		} else {
			success = append(success, card)
		}
	}
	if len(success) > 0 {
		记录管理员代理业务流水(account.Name, 0, "操作:修改"+kind+"归属",
			"卡密:"+strings.Join(success, ","), fmt.Sprintf("目标小伙伴ID:%d", agentID))
	}
	成功提示管理端(ctx, gin.H{"msg": fmt.Sprintf("成功%d张，失败%d张", len(success), len(failed)), "success": success, "failed": failed})
}

func 修改点卡归属(admin, card string, agentID int) error {
	tableName, err := 点卡数据表名(admin)
	if err != nil {
		return err
	}
	if err := 批量同步并删除点卡心跳缓存(admin, []string{card}); err != nil {
		return err
	}
	err = 保存卡密归属(admin, tableName, card, agentID, true)
	if err != nil {
		return err
	}
	// 清除事务期间重建的快照；保留设备授权和 needle，后续计费读取新归属。
	if err := 批量同步并删除点卡心跳缓存(admin, []string{card}); err != nil {
		日志("log/启动记录.txt", "修改点卡归属后同步心跳缓存失败:"+err.Error())
	}
	return nil
}

func 修改时长卡归属(admin, card string, agentID int) error {
	tableName, err := 时长卡数据表名(admin)
	if err != nil {
		return err
	}
	unlock := 锁定时长卡业务(admin, card)
	defer unlock()
	if err := 同步并删除时长卡心跳缓存(admin, card, ""); err != nil {
		return err
	}
	return 保存卡密归属(admin, tableName, card, agentID, false)
}

// 表名由对应卡种入口确定。锁住卡记录后再更新，避免与代理编辑、扣费互相覆盖。
func 保存卡密归属(admin, tableName, card string, agentID int, resetDeduction bool) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var current struct {
			Card    string
			AgentID int
		}
		if err := tx.Table(tableName).Select("card", "agent_id").Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("card = ?", card).Take(&current).Error; err != nil {
			return err
		}
		// 事务内复查并锁住目标账号，避免提交时代理已经被删除。
		if agentID > 0 {
			var target 代理账号记录
			if err := tx.Table(代理账号表名).Select("id").Clauses(clause.Locking{Strength: "SHARE"}).
				Where("id = ? AND admin = ?", agentID, admin).Take(&target).Error; err != nil {
				return err
			}
		}
		if current.AgentID == agentID {
			return nil
		}
		updates := map[string]interface{}{"agent_id": agentID}
		if resetDeduction {
			// 单卡代扣例外属于原代理的选择，转移后跟随新代理设置。
			updates["point_card_auto_deduct_mode"] = "inherit"
		}
		return tx.Table(tableName).Where("card = ?", current.Card).Updates(updates).Error
	})
}
