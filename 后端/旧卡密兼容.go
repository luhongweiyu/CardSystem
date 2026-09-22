package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// 旧客户端的 /card/query 只用于打开旧网页。这里仅返回旧客户端能识别的
// “正常”和“到期时间”，内部数据读取新的 duration_card_<管理员> 表，
// 未激活但仍可激活的卡也允许通过；查询本身不触发激活、不写入心跳，也不扣除余额。
func 旧客户端查询时长卡(ctx *gin.Context) {
	admin, ok := 访客管理员(ctx)
	if !ok {
		失败提示访客(ctx, "访客上下文错误")
		return
	}
	card := strings.ToLower(strings.TrimSpace(input(ctx, "card")))
	if !卡密格式规则.MatchString(card) {
		失败提示访客(ctx, "卡密格式不正确")
		return
	}

	now := time.Now()
	data, err := 时长卡详情(admin, card, now)
	if err != nil {
		失败提示访客(ctx, err.Error())
		return
	}

	text, err := 旧客户端时长卡查询文本(data, now)
	if err != nil {
		失败提示访客(ctx, err.Error())
		return
	}
	成功提示访客(ctx, gin.H{"data": text})
}

// 旧客户端只判断文本中是否包含“正常”，并从“到期时间”后截取展示内容。
// 未激活卡没有绝对到期时间，返回“未激活”占位文本即可，后续由旧客户端
// /card/card_login 按正常登录流程完成首次激活。
func 旧客户端时长卡查询文本(data map[string]interface{}, now time.Time) (string, error) {
	status, _ := data["status"].(string)
	endTime, _ := data["end_time"].(*time.Time)
	switch status {
	case "已激活":
		if endTime == nil || !endTime.After(now) {
			return "", fmt.Errorf("卡密状态不可用")
		}
		return fmt.Sprintf("状态:正常\n到期时间:  %s", endTime.Format(timeLayout)), nil
	case "未激活":
		latestActivationAt, _ := data["latest_activation_at"].(*time.Time)
		if latestActivationAt != nil && !latestActivationAt.After(now) {
			return "", fmt.Errorf("卡密已超过最晚激活时间")
		}
		return "状态:正常\n到期时间:  未激活", nil
	default:
		return "", fmt.Errorf("卡密状态不可用")
	}
}
