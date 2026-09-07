package main

import (
	"strings"
	"testing"
	"time"
)

// Test会话仍可能在线覆盖“最后心跳 + 自动离线时间”的边界，避免清理任务
// 在刚好到达推断截止时误删会话或重复从当前时间起算。
func Test会话仍可能在线(t *testing.T) {
	位置 := time.FixedZone("CST", 8*60*60)
	到期 := time.Date(2026, 9, 4, 11, 0, 0, 0, 位置)
	会话 := 点卡设备会话{
		AuthorizedUntil: 到期,
		LastHeartbeatAt: 到期.Add(-6 * time.Minute), // 10:54，10分钟自动离线时间的推断截止为11:04
	}

	if !会话仍可能在线(会话, 10, 到期.Add(3*time.Minute)) {
		t.Fatal("推断窗口内的设备应视为仍可能在线")
	}
	if !会话仍可能在线(会话, 10, 到期.Add(4*time.Minute)) {
		t.Fatal("推断截止时刻应包含在推断窗口内")
	}
	if 会话仍可能在线(会话, 10, 到期.Add(10*time.Minute+time.Nanosecond)) {
		t.Fatal("超过推断截止后不应继续视为在线")
	}

	没有跨过到期 := 会话
	没有跨过到期.AuthorizedUntil = 到期.Add(5 * time.Minute)
	if 会话仍可能在线(没有跨过到期, 10, 到期) {
		t.Fatal("推断窗口没有跨过授权截止时间时不应续费")
	}
}

func Test计算续费起点(t *testing.T) {
	位置 := time.FixedZone("CST", 8*60*60)
	当前 := time.Date(2026, 9, 4, 11, 3, 0, 0, 位置)
	到期 := time.Date(2026, 9, 4, 11, 0, 0, 0, 位置)

	活动会话 := 点卡设备会话{AuthorizedUntil: 当前.Add(time.Minute), LastHeartbeatAt: 当前}
	if got := 计算续费起点(活动会话, 10, 当前); !got.Equal(当前) {
		t.Fatalf("未到期会话的起点应为当前时间，得到%v", got)
	}

	连续在线 := 点卡设备会话{AuthorizedUntil: 到期, LastHeartbeatAt: 到期.Add(-6 * time.Minute)}
	if got := 计算续费起点(连续在线, 10, 当前); !got.Equal(到期) {
		t.Fatalf("推断仍在线时应从旧到期时间续费，得到%v", got)
	}

	已停止 := 连续在线
	if got := 计算续费起点(已停止, 10, 到期.Add(20*time.Minute)); !got.Equal(到期.Add(20 * time.Minute)) {
		t.Fatalf("推断离线时应从当前时间续费，得到%v", got)
	}
}

func Test软件自动离线时间默认与自定义值(t *testing.T) {
	默认请求, err := 解析软件设置(软件请求{Software: "demo"}, true)
	if err != nil || 默认请求.OnlineGraceMinutes == nil || *默认请求.OnlineGraceMinutes != 默认自动离线时间分钟 {
		t.Fatalf("自动离线时间默认值错误: request=%+v err=%v", 默认请求, err)
	}

	自定义值 := int64(15)
	自定义请求, err := 解析软件设置(软件请求{Software: "demo", OnlineGraceMinutes: &自定义值}, true)
	if err != nil || 自定义请求.OnlineGraceMinutes == nil || *自定义请求.OnlineGraceMinutes != 自定义值 {
		t.Fatalf("自动离线时间自定义值错误: request=%+v err=%v", 自定义请求, err)
	}

	非法值 := int64(4)
	if _, err := 解析软件设置(软件请求{Software: "demo", OnlineGraceMinutes: &非法值}, true); err == nil {
		t.Fatal("小于5分钟的自动离线时间必须被拒绝")
	}
}

func Test生成扣点备注包含设备快照(t *testing.T) {
	remark := 生成扣点备注(点卡扣费参数{
		RemarkPrefix: "登录扣点",
		DeviceID:     "device-123456",
		DeviceAlias:  "办公室电脑",
	}, 3600, 2)
	// 流水只要求保留设备信息快照，不把展示标签绑定为接口或业务契约，
	// 避免以后调整“ID”“设备”等文案时破坏与计费无关的测试。
	for _, expected := range []string{"登录扣点", "授权时长=60分钟", "扣点=2", "device-123456", "办公室电脑"} {
		if !strings.Contains(remark, expected) {
			t.Fatalf("流水备注缺少%q: %s", expected, remark)
		}
	}
}
