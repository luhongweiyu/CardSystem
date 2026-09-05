package main

import (
	"strings"
	"testing"
	"time"
)

// Test会话仍可能在线覆盖“最后心跳 + 两个心跳间隔”的边界，避免清理任务
// 在刚好到达推断截止时误删会话或重复从当前时间起算。
func Test会话仍可能在线(t *testing.T) {
	位置 := time.FixedZone("CST", 8*60*60)
	到期 := time.Date(2026, 9, 4, 11, 0, 0, 0, 位置)
	会话 := 点卡设备会话{
		AuthorizedUntil: 到期,
		LastHeartbeatAt: 到期.Add(-6 * time.Minute), // 10:54，5分钟心跳的推断截止为11:04
	}

	if !会话仍可能在线(会话, 5*60, 到期.Add(3*time.Minute)) {
		t.Fatal("推断窗口内的设备应视为仍可能在线")
	}
	if !会话仍可能在线(会话, 5*60, 到期.Add(4*time.Minute)) {
		t.Fatal("推断截止时刻应包含在推断窗口内")
	}
	if 会话仍可能在线(会话, 5*60, 到期.Add(4*time.Minute+time.Nanosecond)) {
		t.Fatal("超过推断截止后不应继续视为在线")
	}

	没有跨过到期 := 会话
	没有跨过到期.AuthorizedUntil = 到期.Add(5 * time.Minute)
	if 会话仍可能在线(没有跨过到期, 5*60, 到期) {
		t.Fatal("推断窗口没有跨过授权截止时间时不应续费")
	}
}

func Test计算续费起点(t *testing.T) {
	位置 := time.FixedZone("CST", 8*60*60)
	当前 := time.Date(2026, 9, 4, 11, 3, 0, 0, 位置)
	到期 := time.Date(2026, 9, 4, 11, 0, 0, 0, 位置)

	活动会话 := 点卡设备会话{AuthorizedUntil: 当前.Add(time.Minute), LastHeartbeatAt: 当前}
	if got := 计算续费起点(活动会话, 5*60, 当前); !got.Equal(当前) {
		t.Fatalf("未到期会话的起点应为当前时间，得到%v", got)
	}

	连续在线 := 点卡设备会话{AuthorizedUntil: 到期, LastHeartbeatAt: 到期.Add(-6 * time.Minute)}
	if got := 计算续费起点(连续在线, 5*60, 当前); !got.Equal(到期) {
		t.Fatalf("推断仍在线时应从旧到期时间续费，得到%v", got)
	}

	已停止 := 连续在线
	if got := 计算续费起点(已停止, 5*60, 到期.Add(10*time.Minute)); !got.Equal(到期.Add(10 * time.Minute)) {
		t.Fatalf("推断离线时应从当前时间续费，得到%v", got)
	}
}

func Test生成扣点备注包含设备快照(t *testing.T) {
	remark := 生成扣点备注(点卡扣费参数{
		RemarkPrefix: "登录扣点",
		DeviceID:     "device-123456",
		DeviceAlias:  "办公室电脑",
	}, 3600, 2)
	for _, expected := range []string{"登录扣点", "授权时长=3600秒", "扣点=2", "设备ID=device-123456", "设备别名=办公室电脑"} {
		if !strings.Contains(remark, expected) {
			t.Fatalf("流水备注缺少%q: %s", expected, remark)
		}
	}
}
