package main

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 卡端_查询点数流水只允许当前卡密读取自己的流水。设备 ID 和别名已经
// 合并到 remark，持卡用户看到的每一行正好对应一次余额变化。
func 卡端_查询点数流水(ctx *gin.Context) {
	cardContext, ok := ctx.Get("card")
	if !ok {
		失败提示(ctx, "卡密上下文错误")
		return
	}
	userContext, ok := cardContext.(卡密请求上下文)
	if !ok || userContext.Name == "" {
		失败提示(ctx, "卡密上下文错误")
		return
	}
	card := strings.ToLower(strings.TrimSpace(userContext.Card))
	tableName, err := 卡密数据表名(userContext.Name)
	if err != nil {
		失败提示(ctx, err.Error())
		return
	}
	var cardRow 卡密表样式
	query := db.Table(tableName).Where("card = ?", card).First(&cardRow)
	if errors.Is(query.Error, gorm.ErrRecordNotFound) {
		失败提示(ctx, "卡密不存在")
		return
	}
	if query.Error != nil {
		失败提示(ctx, "读取卡密失败")
		return
	}
	softwareID, _ := strconv.Atoi(input(ctx, "software"))
	if softwareID > 0 && softwareID != cardRow.Software {
		失败提示(ctx, "软件与卡密不匹配")
		return
	}
	page, _ := strconv.Atoi(input(ctx, "page"))
	pageSize, _ := strconv.Atoi(input(ctx, "page_size"))
	page, pageSize = 规范化流水分页(page, pageSize)
	rows, total, err := 查询点数流水记录(db_point_ledger.Where("admin = ? AND card = ?", userContext.Name, card), page, pageSize)
	if err != nil {
		失败提示(ctx, "查询点数流水失败")
		return
	}
	成功提示(ctx, gin.H{
		"data": 点数流水展示列表(rows, false), "num": total,
		"page": page, "page_size": pageSize, "balance": cardRow.Point_balance,
	})
}
