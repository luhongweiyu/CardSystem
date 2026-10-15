package main

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"
)

type VerifyRequest struct {
	UserID int
	Cents  int64
	Suffix string
	Stamp  int64
}

type Result struct {
	Paid     bool   `json:"msg"`
	Cooldown int64  `json:"冷却"`
	Error    string `json:"error,omitempty"`
}

type userState struct {
	Initialized bool
	Remaining   int
	LastQuery   time.Time
}

type Service struct {
	mu                 sync.Mutex
	cfg                Config
	source             PaymentSource
	now                func() time.Time
	started            time.Time
	users              [100]userState
	lastGlobal         time.Time
	inFlight           bool
	cache              []Payment
	usedLogs           map[string]time.Time
	usedOrders         map[string]time.Time
	usedMerchantOrders map[string]time.Time
	replays            map[string]time.Time
}

func newService(cfg Config, source PaymentSource, now func() time.Time) *Service {
	started := now()
	// 支付宝入账时间只有秒精度。向上对齐启动时间，排除启动前同秒的付款。
	boundary := started.Truncate(time.Second)
	if boundary.Before(started) {
		boundary = boundary.Add(time.Second)
	}
	return &Service{
		cfg: cfg, source: source, now: now, started: boundary,
		usedLogs: make(map[string]time.Time), usedOrders: make(map[string]time.Time),
		usedMerchantOrders: make(map[string]time.Time),
		replays:            make(map[string]time.Time),
	}
}

func (s *Service) user(id int) *userState {
	u := &s.users[id]
	if !u.Initialized {
		u.Initialized, u.Remaining = true, s.cfg.FastQuota
	}
	return u
}

func (s *Service) startAt(now time.Time) time.Time {
	start := now.Truncate(time.Second).Add(-s.cfg.Lookback)
	if s.started.After(start) {
		start = s.started
	}
	return start
}

func (s *Service) cleanup(now time.Time) {
	for _, entries := range []map[string]time.Time{s.usedLogs, s.usedOrders, s.usedMerchantOrders, s.replays} {
		for key, expires := range entries {
			if !now.Before(expires) {
				delete(entries, key)
			}
		}
	}
	start := s.startAt(now)
	valid := s.cache[:0]
	for _, payment := range s.cache {
		if !payment.At.Before(start) && !payment.At.After(now) {
			valid = append(valid, payment)
		}
	}
	clear(s.cache[len(valid):])
	s.cache = valid
}

func (s *Service) cooldown(user *userState, now time.Time) int64 {
	until := now
	if !s.lastGlobal.IsZero() {
		until = maxTime(until, s.lastGlobal.Add(s.cfg.GlobalCooldown))
	}
	if user.Remaining == 0 && !user.LastQuery.IsZero() {
		until = maxTime(until, user.LastQuery.Add(s.cfg.UserCooldown))
	}
	left := until.Sub(now)
	return int64((left + time.Second - 1) / time.Second)
}

func maxTime(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

func (s *Service) acceptReplay(signature string, expires, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanup(now)
	if _, exists := s.replays[signature]; exists {
		return false
	}
	s.replays[signature] = expires
	return true
}

// match 在锁内完成核对、认领和额度恢复，确保并发最多一个 true。
// 第二个返回值表示已有确定结果（成功、重复或歧义），无需请求支付宝。
func (s *Service) match(request VerifyRequest, user *userState, now time.Time) (bool, bool) {
	var found *Payment
	for i := range s.cache {
		p := &s.cache[i]
		// 两种订单号任一尾号匹配即可；同一条记录即使两个都匹配也只计一次。
		if p.Cents != request.Cents ||
			(!strings.HasSuffix(p.OrderID, request.Suffix) && !strings.HasSuffix(p.MerchantOrderID, request.Suffix)) {
			continue
		}
		if found != nil {
			slog.Info("核对未确认", "user", request.UserID, "reason", "金额尾号存在多笔候选")
			return false, true
		}
		found = p
	}
	if found == nil {
		return false, false
	}
	_, usedLog := s.usedLogs[found.LogID]
	_, usedOrder := s.usedOrders[found.OrderID]
	_, usedMerchantOrder := s.usedMerchantOrders[found.MerchantOrderID]
	if usedLog || usedOrder || usedMerchantOrder {
		slog.Info("核对未确认", "user", request.UserID, "reason", "支付已使用")
		return false, true
	}
	expires := now.Add(s.cfg.UsedTTL)
	// 保存同一支付的全部可用编号，防止换另一种尾号再次认领；空编号不入表。
	s.usedLogs[found.LogID] = expires
	if found.OrderID != "" {
		s.usedOrders[found.OrderID] = expires
	}
	if found.MerchantOrderID != "" {
		s.usedMerchantOrders[found.MerchantOrderID] = expires
	}
	user.Remaining, user.LastQuery = s.cfg.FastQuota, time.Time{}
	slog.Info("支付认领成功", "user", request.UserID)
	return true, true
}

func (s *Service) Verify(ctx context.Context, request VerifyRequest) Result {
	s.mu.Lock()
	now := s.now()
	s.cleanup(now)
	user := s.user(request.UserID)
	if paid, done := s.match(request, user, now); done {
		result := Result{Paid: paid, Cooldown: s.cooldown(user, now)}
		s.mu.Unlock()
		return result
	}
	left := s.cooldown(user, now)
	start, end := s.startAt(now), now.Truncate(time.Second)
	if left > 0 || s.inFlight || !start.Before(end) {
		s.mu.Unlock()
		return Result{Cooldown: left}
	}
	if ctx.Err() != nil {
		s.mu.Unlock()
		return Result{Cooldown: left}
	}
	// 预占全局查询机会后释放锁，网络等待期间其他请求仍可核对缓存。
	s.inFlight, s.lastGlobal, user.LastQuery = true, now, now
	if user.Remaining > 0 {
		user.Remaining--
	}
	s.mu.Unlock()

	payments, err := s.source.Query(ctx, start, end)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.inFlight = false
	now = s.now()
	s.cleanup(now)
	if err != nil {
		slog.Warn("支付宝查询失败，保留有效旧缓存", "error", err)
		return Result{Cooldown: s.cooldown(user, now)}
	}
	// 仅用本轮第一页覆盖缓存；不合并历史结果，也不关心是否还有下一页。
	s.cache = make([]Payment, 0, len(payments))
	seen := make(map[string]bool, len(payments))
	for _, p := range payments {
		if p.LogID == "" || (p.OrderID == "" && p.MerchantOrderID == "") || p.Cents <= 0 || seen[p.LogID] ||
			p.At.Before(start) || !p.At.Before(end) {
			continue
		}
		seen[p.LogID] = true
		s.cache = append(s.cache, p)
	}
	s.cleanup(now)
	slog.Info("第一页查询完成，已替换缓存", "income_count", len(s.cache))
	paid, done := s.match(request, user, now)
	if !done {
		slog.Info("核对未确认", "user", request.UserID, "reason", "缓存无匹配收款")
	}
	return Result{Paid: paid, Cooldown: s.cooldown(user, now)}
}
