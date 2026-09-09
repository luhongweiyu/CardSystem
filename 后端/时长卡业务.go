package main

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 时长卡模式与点卡模式共用管理员认证、软件和请求签名，但不共用卡密表。
// 本文件只处理“卡密自身带固定时长”的旧业务语义：首次登录激活，后续
// 登录沿用当前到期时间，心跳只校验同一个服务端 needle。

type 时长卡生成请求 struct {
	Software                int    `json:"software"`
	DurationMinutes         int64  `json:"duration_minutes"`
	Num                     int    `json:"num"`
	Cards                   string `json:"cards"`
	Random                  bool   `json:"random"`
	Notes                   string `json:"notes"`
	ConfigContent           string `json:"config_content"`
	LatestActivationMinutes int64  `json:"latest_activation_minutes"`
}

type 时长卡列表排序 struct {
	字段 string
	方向 string
}

// 规范化时长卡基础参数集中约束时长、数量、激活期限和展示文本，管理员
// 与未来的代理发卡入口必须复用它，不能各自形成不同的时长范围。
func 规范化时长卡基础参数(admin string, request 时长卡生成请求) (时长卡生成请求, error) {
	admin = strings.TrimSpace(admin)
	if !验证管理员名称(admin) || request.Software <= 0 {
		return request, fmt.Errorf("管理员或软件参数不正确")
	}
	if request.DurationMinutes < 时长卡最小时长分钟 || request.DurationMinutes > 时长卡永久分钟 {
		return request, fmt.Errorf("时长卡时长必须在%d分钟至%d天之间", 时长卡最小时长分钟, 时长卡永久分钟/1440)
	}
	if request.Num <= 0 || request.Num > 时长卡最大单次数量 {
		return request, fmt.Errorf("单次生成数量必须在1至%d之间", 时长卡最大单次数量)
	}
	if request.LatestActivationMinutes < -1 {
		return request, fmt.Errorf("最晚激活时间参数不正确")
	}
	if request.LatestActivationMinutes > 0 && request.LatestActivationMinutes > 时长卡永久分钟 {
		return request, fmt.Errorf("最晚激活时间不能超过%d天", 时长卡永久分钟/1440)
	}
	var valid bool
	request.Notes, valid = 规范化可显示文本(request.Notes, 时长卡最大备注字符数)
	if !valid {
		return request, fmt.Errorf("时长卡备注不能包含控制字符且不能超过%d个字符", 时长卡最大备注字符数)
	}
	if err := 校验卡密配置内容(request.ConfigContent); err != nil {
		return request, err
	}
	return request, nil
}

// 解析时长卡生成卡密统一随机和指定卡密两种来源，返回值始终为规范化的
// 小写卡密；跨模式允许同名，时长卡表内部不允许重复。
func 解析时长卡生成卡密(custom string, random bool, softwareID, count int) ([]string, error) {
	if random {
		result := make([]string, 0, count)
		seen := make(map[string]struct{}, count)
		for len(result) < count {
			card := strings.ToLower(生成随机卡密文本(softwareID))
			if _, exists := seen[card]; exists {
				continue
			}
			seen[card] = struct{}{}
			result = append(result, card)
		}
		return result, nil
	}
	return 解析卡密文本列表(custom)
}

// 准备时长卡只做输入解析、随机值生成和重复预检查，避免把不可信的大
// 文本处理放进数据库锁。最终主键约束仍负责兜住并发请求的竞态。
func 准备生成时长卡(admin string, request 时长卡生成请求) (string, []string, 时长卡生成请求, error) {
	var err error
	request, err = 规范化时长卡基础参数(admin, request)
	if err != nil {
		return "", nil, request, err
	}
	var softwareRow 软件
	query := db.Table("software").Select("id").Where("name = ? AND id = ?", admin, request.Software).First(&softwareRow)
	if errors.Is(query.Error, gorm.ErrRecordNotFound) {
		return "", nil, request, fmt.Errorf("软件不存在")
	}
	if query.Error != nil {
		return "", nil, request, fmt.Errorf("检查软件失败")
	}
	tableName, err := 时长卡数据表名(admin)
	if err != nil {
		return "", nil, request, err
	}
	cards, err := 解析时长卡生成卡密(request.Cards, request.Random, request.Software, request.Num)
	if err != nil {
		return "", nil, request, err
	}
	if len(cards) != request.Num {
		return "", nil, request, fmt.Errorf("指定卡密数量与生成数量不一致")
	}
	var existing int64
	if err := db.Table(tableName).Where("card IN ?", cards).Count(&existing).Error; err != nil {
		return "", nil, request, fmt.Errorf("检查时长卡是否重复失败")
	}
	if existing > 0 {
		return "", nil, request, fmt.Errorf("时长卡已存在，不能重复生成")
	}
	return tableName, cards, request, nil
}

// 时长卡激活截止时间把相对分钟转换成生成时的绝对时间；-1 和 0
// 分别由“不限制”和“立即激活”分支处理，因此都不保存截止值。
func 时长卡激活截止时间(now time.Time, request 时长卡生成请求) *time.Time {
	if request.LatestActivationMinutes <= 0 {
		return nil
	}
	deadline := now.Add(time.Duration(request.LatestActivationMinutes) * time.Minute)
	return &deadline
}

// 时长卡计算到期时间只在首次激活时执行，普通卡按分钟计算，旧版约定的
// 36500天永久卡统一固定到2099年。
func 时长卡计算到期时间(now time.Time, row 时长卡表样式) (time.Time, error) {
	if row.DurationMinutes < 时长卡最小时长分钟 || row.DurationMinutes > 时长卡永久分钟 {
		return time.Time{}, fmt.Errorf("时长卡时长配置不正确")
	}
	if row.LatestActivationAt != nil && !row.LatestActivationAt.After(now) {
		return time.Time{}, fmt.Errorf("时长卡已超过最晚激活时间")
	}
	// 36500天是旧版本永久卡约定值，统一固定到2099年，避免时间溢出
	// 或不同数据库对极远时间的处理不一致。
	if row.DurationMinutes >= 时长卡永久分钟 {
		return time.Date(2099, 1, 1, 0, 0, 0, 0, now.Location()), nil
	}
	endTime := now.Add(time.Duration(row.DurationMinutes) * time.Minute)
	// 保留旧版“最晚激活”规则：卡在期限前仍可激活，但授权截止时间
	// 不得晚于生成时计算出的最晚时间，因此临近期限激活时可用时长会缩短。
	if row.LatestActivationAt != nil && row.LatestActivationAt.Before(endTime) {
		endTime = *row.LatestActivationAt
	}
	return endTime, nil
}

// 激活时长卡必须在行锁内执行。latest_activation_at 只限制首次激活，
// 后续登录不重新计算时长，也不会把已有到期时间延长。
func 激活时长卡_已加锁(tx *gorm.DB, tableName string, row *时长卡表样式, now time.Time) error {
	if row.EndTime != nil {
		return nil
	}
	endTime, err := 时长卡计算到期时间(now, *row)
	if err != nil {
		return err
	}
	row.EndTime = &endTime
	updates := map[string]interface{}{"end_time": endTime}
	if result := tx.Table(tableName).Where("card = ?", row.Card).Updates(updates); result.Error != nil || result.RowsAffected != 1 {
		return fmt.Errorf("激活时长卡失败")
	}
	return nil
}

// 保存时长卡批次只持久化已经完成校验和查重的记录。调用方负责在事务中
// 锁定软件，避免软件删除与发卡并发产生孤立数据。
func 保存时长卡批次(tx *gorm.DB, tableName string, agentID int, request 时长卡生成请求, cards []string, now time.Time) error {
	rows := make([]时长卡表样式, 0, len(cards))
	for _, card := range cards {
		row := 时长卡表样式{
			Card: card, CreateTime: now, Software: request.Software,
			CardState: 卡密状态_正常, DurationMinutes: request.DurationMinutes,
			LatestActivationAt: 时长卡激活截止时间(now, request), Notes: request.Notes,
			ConfigContent: request.ConfigContent, AgentID: agentID,
		}
		// latest_activation_minutes=0 表示生成后立即激活；-1 表示不限制。
		if request.LatestActivationMinutes == 0 {
			endTime, err := 时长卡计算到期时间(now, row)
			if err != nil {
				return err
			}
			row.EndTime = &endTime
		}
		rows = append(rows, row)
	}
	if err := tx.Table(tableName).CreateInBatches(&rows, 200).Error; err != nil {
		return fmt.Errorf("写入时长卡失败")
	}
	return nil
}

// 生成并保存时长卡把最终的软件存在性检查和整批写入放进短事务；随机
// 生成、文本解析和重复预检查均已在事务外完成。
func 生成并保存时长卡(admin string, agentID int, request 时长卡生成请求) ([]string, error) {
	tableName, cards, request, err := 准备生成时长卡(admin, request)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := 锁定发卡软件(tx, admin, request.Software); err != nil {
			return err
		}
		return 保存时长卡批次(tx, tableName, agentID, request, cards, now)
	})
	if err != nil {
		return nil, err
	}
	return cards, nil
}

// 解析时长卡排序只允许固定数据库字段和方向，避免把客户端字符串直接
// 拼成 SQL；非卡密字段都追加卡密升序保证分页稳定。
func 解析时长卡排序(ctx *gin.Context) 时长卡列表排序 {
	field := strings.TrimSpace(input(ctx, "sort_by"))
	direction := strings.ToLower(strings.TrimSpace(input(ctx, "sort_order")))
	allowed := map[string]bool{
		"card": true, "software": true, "duration_minutes": true,
		"card_state": true, "create_time": true, "use_time": true, "end_time": true,
	}
	if !allowed[field] {
		field, direction = "create_time", "desc"
	}
	if direction != "asc" && direction != "desc" {
		direction = "desc"
	}
	return 时长卡列表排序{字段: field, 方向: direction}
}

// 查询时长卡列表按管理员和可选代理范围分页。未激活、已激活和已到期
// 都由 end_time 实时判断，不把派生状态反复写回数据库。
func 查询时长卡列表(ctx *gin.Context, admin string, agentID int) {
	tableName, err := 时长卡数据表名(admin)
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	softwareID, _ := strconv.Atoi(input(ctx, "software"))
	state, _ := strconv.Atoi(input(ctx, "card_state"))
	keyword := strings.ToLower(strings.TrimSpace(input(ctx, "card")))
	notes := strings.TrimSpace(input(ctx, "notes"))
	if len([]rune(keyword)) > 63 || len([]rune(notes)) > 时长卡最大备注字符数 {
		失败提示管理端(ctx, "筛选条件过长")
		return
	}
	now := time.Now()
	query := db.Table(tableName)
	if softwareID > 0 {
		query = query.Where("software = ?", softwareID)
	}
	if agentID > 0 {
		query = query.Where("agent_id = ?", agentID)
	}
	if keyword != "" {
		query = query.Where("card LIKE ?", "%"+转义Like文本(keyword)+"%")
	}
	if notes != "" {
		query = query.Where("notes LIKE ?", "%"+转义Like文本(notes)+"%")
	}
	switch state {
	case 时长卡状态_未激活:
		query = query.Where("card_state = ? AND end_time IS NULL", 卡密状态_正常)
	case 卡密状态_正常:
		query = query.Where("card_state = ? AND end_time > ?", 卡密状态_正常, now)
	case 时长卡状态_到期:
		query = query.Where("card_state = ? AND end_time IS NOT NULL AND end_time <= ?", 卡密状态_正常, now)
	case 卡密状态_冻结:
		query = query.Where("card_state = ?", 卡密状态_冻结)
	case 时长卡状态_暂停:
		query = query.Where("card_state = ?", 时长卡状态_暂停)
	case 时长卡状态_已充值:
		query = query.Where("card_state = ?", 时长卡状态_已充值)
	}
	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		失败提示管理端(ctx, "查询时长卡数量失败")
		return
	}
	page, pageSize := 读取通用分页参数(ctx)
	sorting := 解析时长卡排序(ctx)
	query = query.Order("`" + sorting.字段 + "` " + sorting.方向)
	if sorting.字段 != "card" {
		query = query.Order("`card` ASC")
	}
	var rows []时长卡表样式
	if err := query.Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		失败提示管理端(ctx, "查询时长卡失败")
		return
	}
	data := make([]时长卡列表项, 0, len(rows))
	for _, row := range rows {
		// 心跳缓存中的时间可能尚未到同步期限，列表仍需展示当前进程
		// 已收到的最新心跳；卡密其他字段继续使用数据库结果。
		合并时长卡心跳缓存(admin, &row)
		data = append(data, 时长卡列表项转换带管理员(admin, row, now))
	}
	成功提示管理端(ctx, gin.H{"data": data, "num": total, "page": page, "page_size": pageSize})
}

// 列表转换需要管理员名读取软件配置。通过参数传入而不是使用全局变量，
// 避免并发请求在不同租户之间串用在线判定窗口。
func 时长卡列表项转换带管理员(admin string, row 时长卡表样式, now time.Time) 时长卡列表项 {
	online := false
	if row.CardState == 卡密状态_正常 && row.LastHeartbeatAt != nil && row.EndTime != nil && row.EndTime.After(now) && row.Needle != "" {
		if settings, err := 读取软件设置(db, admin, row.Software); err == nil {
			online = row.LastHeartbeatAt.After(now.Add(-time.Duration(settings.OnlineGraceMinutes) * time.Minute))
		}
	}
	return 时长卡列表项{Card: row.Card, CreateTime: row.CreateTime, UseTime: row.UseTime,
		EndTime: row.EndTime, Software: row.Software, CardState: row.CardState,
		DurationMinutes: row.DurationMinutes, LatestActivationAt: row.LatestActivationAt,
		Notes: row.Notes, ConfigContent: row.ConfigContent, PausedRemainingMinutes: row.PausedRemainingMinutes,
		AgentID: row.AgentID, Online: online}
}

// 管理员_查询时长卡列表只绑定当前认证管理员，客户端提交的 name 不参与
// 动态表选择。
func 管理员_查询时长卡列表(ctx *gin.Context) {
	查询时长卡列表(ctx, 管理员_用户名(ctx), 0)
}

// 管理员_添加时长卡是管理端生成入口，生成成功后只写操作日志，不写
// 点数流水，因为时长卡没有点数余额。
func 管理员_添加时长卡(ctx *gin.Context) {
	var request 时长卡生成请求
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil {
		失败提示管理端(ctx, "数据错误")
		return
	}
	admin := 管理员_用户名(ctx)
	cards, err := 生成并保存时长卡(admin, 0, request)
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	日志("log/"+admin+time.Now().Format("200601"), fmt.Sprintf("新增时长卡;软件:%d;数量:%d;时长:%d分钟", request.Software, len(cards), request.DurationMinutes))
	成功提示管理端(ctx, gin.H{"msg": fmt.Sprintf("成功生成%d张时长卡", len(cards)), "data": strings.Join(cards, "\n")})
}

// 读取时长卡按管理员独立表和完整卡密主键查询，不回退到点卡表。
func 读取时长卡(admin, card string) (时长卡表样式, bool, error) {
	tableName, err := 时长卡数据表名(admin)
	if err != nil {
		return 时长卡表样式{}, false, err
	}
	card = strings.ToLower(strings.TrimSpace(card))
	var row 时长卡表样式
	query := db.Table(tableName).Where("card = ?", card).First(&row)
	if errors.Is(query.Error, gorm.ErrRecordNotFound) {
		return row, false, nil
	}
	if query.Error != nil {
		return row, false, fmt.Errorf("读取时长卡失败")
	}
	return row, true, nil
}

// 时长卡详情统一生成管理端、卡端和访客端共用的状态数据。调用方可根据
// 权限移除 needle 等敏感字段。
func 时长卡详情(admin, card string, now time.Time) (gin.H, error) {
	row, found, err := 读取时长卡(admin, card)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("时长卡不存在")
	}
	return 构建时长卡详情(admin, row, now), nil
}

// 构建时长卡详情只负责把数据库记录转换为接口结果，管理员、代理和访客
// 在完成各自的权限查询后复用它，避免三套状态文字逐渐不一致。
func 构建时长卡详情(admin string, row 时长卡表样式, now time.Time) gin.H {
	// 详情需要返回当前 needle，因此在转换列表字段前合并尚未落库的
	// 心跳快照；访客随后会移除不应公开的字段。
	合并时长卡心跳缓存(admin, &row)
	item := 时长卡列表项转换带管理员(admin, row, now)
	status := "未激活"
	if row.CardState == 卡密状态_冻结 {
		status = "冻结"
	} else if row.CardState == 时长卡状态_暂停 {
		status = "已暂停"
	} else if row.CardState == 时长卡状态_已充值 {
		status = "已用于充值"
	} else if row.EndTime != nil {
		if row.EndTime.After(now) {
			status = "已激活"
		} else {
			status = "已到期"
		}
	}
	return gin.H{"card": item.Card, "software": item.Software, "duration_minutes": item.DurationMinutes,
		"create_time": item.CreateTime, "use_time": item.UseTime, "end_time": item.EndTime,
		"latest_activation_at": item.LatestActivationAt, "card_state": item.CardState,
		"status": status, "online": item.Online, "needle": row.Needle,
		"last_heartbeat_at": row.LastHeartbeatAt, "notes": row.Notes, "config_content": row.ConfigContent,
		"paused_remaining_minutes": row.PausedRemainingMinutes, "agent_id": row.AgentID}
}

// 管理员_查询时长卡详情允许当前管理员查看完整在线校验信息，便于排查
// 客户端被挤下线或心跳停止的问题。
func 管理员_查询时长卡详情(ctx *gin.Context) {
	admin := 管理员_用户名(ctx)
	data, err := 时长卡详情(admin, input(ctx, "card"), time.Now())
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	成功提示管理端(ctx, gin.H{"data": data})
}

// 管理员_修改时长卡只允许修改状态、备注和配置。固定卡面时长不会通过
// 编辑接口暗中改变，延长到期时间必须走明确的续费接口。
func 管理员_修改时长卡(ctx *gin.Context) {
	var request struct {
		Card          string  `json:"card"`
		Notes         *string `json:"notes"`
		ConfigContent *string `json:"config_content"`
		CardState     *int    `json:"card_state"`
	}
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil {
		失败提示管理端(ctx, "数据错误")
		return
	}
	if err := 修改时长卡范围(管理员_用户名(ctx), 0, request.Card, request.Notes, request.ConfigContent, request.CardState); err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	成功提示管理端(ctx, gin.H{"msg": "修改成功"})
}

// 处理时长卡批量状态逐卡执行，使格式错误或不存在的单项不会阻止其他
// 合法卡密；冻结时同步清除在线 needle。
func 处理时长卡批量状态(admin string, cards []string, state int) ([]string, []string, error) {
	if len(cards) == 0 || len(cards) > 1000 {
		return nil, cards, fmt.Errorf("单次操作卡密数量必须在1至1000之间")
	}
	if state != 卡密状态_正常 && state != 卡密状态_冻结 {
		return nil, cards, fmt.Errorf("卡密状态不正确")
	}
	tableName, err := 时长卡数据表名(admin)
	if err != nil {
		return nil, cards, err
	}
	success, failed := make([]string, 0, len(cards)), make([]string, 0)
	for _, raw := range cards {
		card := strings.ToLower(strings.TrimSpace(raw))
		if !卡密格式规则.MatchString(card) {
			failed = append(failed, raw)
			continue
		}
		if cacheErr := 同步并删除时长卡心跳缓存(admin, card, ""); cacheErr != nil {
			failed = append(failed, raw)
			continue
		}
		updates := map[string]interface{}{"card_state": state}
		if state == 卡密状态_冻结 {
			updates["needle"] = ""
			updates["last_heartbeat_at"] = nil
		}
		// 暂停和已用于充值属于专用状态，只能由对应业务接口转换，不能
		// 通过批量冻结/解冻把一次性充值来源重新恢复为可用卡。
		result := db.Table(tableName).Where("card = ? AND card_state IN ?", card, []int{卡密状态_正常, 卡密状态_冻结}).Updates(updates)
		if result.Error == nil && (result.RowsAffected == 1 || 时长卡记录存在(tableName, card)) {
			if cacheErr := 同步并删除时长卡心跳缓存(admin, card, ""); cacheErr != nil {
				日志("log/启动记录.txt", "修改时长卡状态后同步心跳缓存失败:"+cacheErr.Error())
			}
			success = append(success, card)
		} else {
			failed = append(failed, raw)
		}
	}
	return success, failed, nil
}

// 时长卡记录存在只用于把“值没有变化”的零影响行更新识别为幂等成功；
// 暂停或已用于充值的专用状态不属于可编辑记录，不能被误判为成功。
func 时长卡记录存在(tableName, card string) bool {
	var count int64
	return db.Table(tableName).Where("card = ? AND card_state IN ?", card, []int{卡密状态_正常, 卡密状态_冻结}).Count(&count).Error == nil && count == 1
}

// 管理员_批量修改时长卡状态返回成功和失败清单，方便页面明确展示部分
// 成功结果。
func 管理员_批量修改时长卡状态(ctx *gin.Context) {
	var request struct {
		Cards     []string `json:"cards"`
		CardState int      `json:"card_state"`
	}
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil {
		失败提示管理端(ctx, "数据错误")
		return
	}
	success, failed, err := 处理时长卡批量状态(管理员_用户名(ctx), request.Cards, request.CardState)
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	成功提示管理端(ctx, gin.H{"msg": fmt.Sprintf("成功%d张，失败%d张", len(success), len(failed)), "success": success, "failed": failed})
}

// 删除时长卡记录只删除独立时长卡表中的目标行，不会删除同名点卡。
func 删除时长卡记录(admin string, cards []string) ([]string, []string, error) {
	if len(cards) == 0 || len(cards) > 1000 {
		return nil, cards, fmt.Errorf("单次删除数量必须在1至1000之间")
	}
	tableName, err := 时长卡数据表名(admin)
	if err != nil {
		return nil, cards, err
	}
	success, failed := make([]string, 0, len(cards)), make([]string, 0)
	for _, raw := range cards {
		card := strings.ToLower(strings.TrimSpace(raw))
		if !卡密格式规则.MatchString(card) {
			failed = append(failed, raw)
			continue
		}
		if cacheErr := 同步并删除时长卡心跳缓存(admin, card, ""); cacheErr != nil {
			failed = append(failed, raw)
			continue
		}
		result := db.Table(tableName).Where("card = ?", card).Delete(&时长卡表样式{})
		if result.Error == nil && result.RowsAffected == 1 {
			if cacheErr := 同步并删除时长卡心跳缓存(admin, card, ""); cacheErr != nil {
				日志("log/启动记录.txt", "删除时长卡后同步心跳缓存失败:"+cacheErr.Error())
			}
			success = append(success, card)
		} else {
			failed = append(failed, raw)
		}
	}
	return success, failed, nil
}

// 管理员_删除时长卡绑定当前认证管理员并返回逐卡结果。
func 管理员_删除时长卡(ctx *gin.Context) {
	var request struct {
		Cards []string `json:"cards"`
	}
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil {
		失败提示管理端(ctx, "数据错误")
		return
	}
	success, failed, err := 删除时长卡记录(管理员_用户名(ctx), request.Cards)
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	成功提示管理端(ctx, gin.H{"msg": fmt.Sprintf("成功%d张，失败%d张", len(success), len(failed)), "success": success, "failed": failed})
}

// 管理员_续费时长卡处理已激活或暂停中的时长卡。已到期卡从当前时间重新
// 增加，未到期卡从原截止时间增加，暂停卡累加暂停剩余分钟；未激活卡不能
// 通过续费偷换卡面时长。
func 管理员_续费时长卡(ctx *gin.Context) {
	var request struct {
		Cards           []string `json:"cards"`
		DurationMinutes int64    `json:"duration_minutes"`
	}
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil || request.DurationMinutes < 时长卡最小时长分钟 || request.DurationMinutes > 时长卡永久分钟 {
		失败提示管理端(ctx, fmt.Sprintf("续费时长必须在%d分钟至%d天之间", 时长卡最小时长分钟, 时长卡永久分钟/1440))
		return
	}
	if len(request.Cards) == 0 || len(request.Cards) > 1000 {
		失败提示管理端(ctx, "单次续费数量必须在1至1000之间")
		return
	}
	tableName, err := 时长卡数据表名(管理员_用户名(ctx))
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	success, failed := make([]string, 0, len(request.Cards)), make([]string, 0)
	for _, raw := range request.Cards {
		card := strings.ToLower(strings.TrimSpace(raw))
		if !卡密格式规则.MatchString(card) {
			failed = append(failed, raw)
			continue
		}
		if cacheErr := 同步并删除时长卡心跳缓存(管理员_用户名(ctx), card, ""); cacheErr != nil {
			failed = append(failed, raw)
			continue
		}
		err := db.Transaction(func(tx *gorm.DB) error {
			var row 时长卡表样式
			query := tx.Table(tableName).Clauses(clause.Locking{Strength: "UPDATE"}).Where("card = ?", card).First(&row)
			if errors.Is(query.Error, gorm.ErrRecordNotFound) {
				return fmt.Errorf("不存在")
			}
			if query.Error != nil {
				return query.Error
			}
			if row.CardState == 卡密状态_冻结 {
				return fmt.Errorf("已冻结")
			}
			if row.CardState == 时长卡状态_暂停 && row.EndTime == nil && row.PausedRemainingMinutes > 0 {
				if request.DurationMinutes > math.MaxInt64-row.PausedRemainingMinutes {
					return fmt.Errorf("暂停剩余时长溢出")
				}
				return tx.Table(tableName).Where("card = ?", card).Update("paused_remaining_minutes", row.PausedRemainingMinutes+request.DurationMinutes).Error
			}
			if row.CardState != 卡密状态_正常 {
				return fmt.Errorf("状态不正常")
			}
			if row.EndTime == nil {
				return fmt.Errorf("尚未激活")
			}
			base := *row.EndTime
			now := time.Now()
			if base.Before(now) {
				base = now
			}
			end := base.Add(time.Duration(request.DurationMinutes) * time.Minute)
			return tx.Table(tableName).Where("card = ?", card).Update("end_time", end).Error
		})
		if err != nil {
			failed = append(failed, raw)
		} else {
			if cacheErr := 同步并删除时长卡心跳缓存(管理员_用户名(ctx), card, ""); cacheErr != nil {
				日志("log/启动记录.txt", "续费时长卡后同步心跳缓存失败:"+cacheErr.Error())
			}
			success = append(success, card)
		}
	}
	成功提示管理端(ctx, gin.H{"msg": fmt.Sprintf("成功%d张，失败%d张", len(success), len(failed)), "success": success, "failed": failed})
}

// 时长卡客户端登录。软件从卡密记录读取，不接受客户端覆盖；重复登录
// 只刷新服务端 needle，不重新增加到期时间。
func durationCardLogin(ctx *gin.Context) {
	value, ok := ctx.Get("card")
	cardContext, valid := value.(卡密请求上下文)
	if !ok || !valid {
		失败提示(ctx, "卡密上下文错误")
		return
	}
	admin, card := cardContext.Name, cardContext.Card
	tableName, err := 时长卡数据表名(admin)
	if err != nil {
		失败提示(ctx, err.Error())
		return
	}
	// 登录会刷新服务端 needle。先同步并删除旧缓存，避免旧心跳在
	// 登录事务之后重新覆盖新会话的状态。
	if cacheErr := 同步并删除时长卡心跳缓存(admin, card, ""); cacheErr != nil {
		失败提示(ctx, cacheErr.Error())
		return
	}
	now := time.Now()
	var row 时长卡表样式
	var settings 软件
	err = db.Transaction(func(tx *gorm.DB) error {
		query := tx.Table(tableName).Clauses(clause.Locking{Strength: "UPDATE"}).Where("card = ?", card).First(&row)
		if errors.Is(query.Error, gorm.ErrRecordNotFound) {
			return fmt.Errorf("时长卡不存在")
		}
		if query.Error != nil {
			return fmt.Errorf("读取时长卡失败")
		}
		if row.CardState == 卡密状态_冻结 {
			return fmt.Errorf("时长卡被冻结")
		}
		if row.CardState != 卡密状态_正常 {
			return fmt.Errorf("时长卡状态不正常")
		}
		settings, err = 读取软件设置(tx, admin, row.Software)
		if err != nil {
			return err
		}
		if row.EndTime == nil {
			if err := 激活时长卡_已加锁(tx, tableName, &row, now); err != nil {
				return err
			}
		}
		if row.EndTime == nil || !row.EndTime.After(now) {
			return fmt.Errorf("时长卡已到期")
		}
		needle := GetRandomString(32, "a")
		heartbeat := now
		updates := map[string]interface{}{"needle": needle, "last_heartbeat_at": heartbeat}
		// use_time 表示第一次真正登录时间。重复登录只替换 needle，不能
		// 把首次使用时间反复覆盖成最近一次登录时间。
		if row.UseTime == nil {
			useTime := now
			updates["use_time"] = useTime
			row.UseTime = &useTime
		}
		if result := tx.Table(tableName).Where("card = ?", card).Updates(updates); result.Error != nil || result.RowsAffected != 1 {
			return fmt.Errorf("登录时长卡失败")
		}
		row.Needle, row.LastHeartbeatAt = needle, &heartbeat
		return nil
	})
	if err != nil {
		失败提示(ctx, err.Error())
		return
	}
	写入时长卡心跳缓存(admin, row, settings.HeartbeatIntervalSeconds)
	成功提示(ctx, gin.H{"needle": row.Needle, "authorized_until": row.EndTime, "software": row.Software, "heartbeat_interval_seconds": settings.HeartbeatIntervalSeconds})
}

// durationCardPing 优先走内存心跳缓存。缓存未命中、已到期或被状态操作
// 失效时，才回退到数据库读取并用 needle 条件更新，防止旧客户端覆盖新会话。
func durationCardPing(ctx *gin.Context) {
	value, ok := ctx.Get("card")
	cardContext, valid := value.(卡密请求上下文)
	if !ok || !valid {
		失败提示(ctx, "卡密上下文错误")
		return
	}
	needle := strings.TrimSpace(input(ctx, "needle"))
	if needle == "" {
		失败提示(ctx, "needle不能为空")
		return
	}
	now := time.Now()
	if row, heartbeatSeconds, handled, cacheErr := 尝试记录时长卡缓存心跳(cardContext.Name, cardContext.Card, needle, now); handled {
		if cacheErr != nil {
			失败提示(ctx, cacheErr.Error())
			return
		}
		成功提示(ctx, gin.H{"needle": row.Needle, "authorized_until": row.EndTime, "heartbeat_interval_seconds": heartbeatSeconds})
		return
	}
	// 缓存命中但已到期或状态不再允许心跳时，先把最后一次缓存心跳
	// 写回数据库，再由下面的完整查询返回准确的业务错误。
	if cacheErr := 同步并删除时长卡心跳缓存(cardContext.Name, cardContext.Card, needle); cacheErr != nil {
		失败提示(ctx, cacheErr.Error())
		return
	}
	tableName, err := 时长卡数据表名(cardContext.Name)
	if err != nil {
		失败提示(ctx, err.Error())
		return
	}
	var row 时长卡表样式
	query := db.Table(tableName).Where("card = ?", cardContext.Card).First(&row)
	if errors.Is(query.Error, gorm.ErrRecordNotFound) {
		失败提示(ctx, "时长卡不存在")
		return
	}
	if query.Error != nil {
		失败提示(ctx, "读取时长卡失败")
		return
	}
	if row.CardState == 卡密状态_冻结 {
		失败提示(ctx, "时长卡被冻结")
		return
	}
	if row.CardState != 卡密状态_正常 {
		失败提示(ctx, "时长卡状态不正常")
		return
	}
	if row.Needle == "" || row.Needle != needle {
		失败提示(ctx, "needle验证失败，可能已在其他设备登录")
		return
	}
	if row.EndTime == nil || !row.EndTime.After(now) {
		失败提示(ctx, "时长卡已到期")
		return
	}
	settings, err := 读取软件设置(db, cardContext.Name, row.Software)
	if err != nil {
		失败提示(ctx, err.Error())
		return
	}
	if result := db.Table(tableName).Where("card = ? AND needle = ?", cardContext.Card, needle).Updates(map[string]interface{}{"last_heartbeat_at": now}); result.Error != nil || result.RowsAffected != 1 {
		失败提示(ctx, "更新心跳失败")
		return
	}
	row.LastHeartbeatAt = &now
	写入时长卡心跳缓存(cardContext.Name, row, settings.HeartbeatIntervalSeconds)
	成功提示(ctx, gin.H{"needle": needle, "authorized_until": row.EndTime, "heartbeat_interval_seconds": settings.HeartbeatIntervalSeconds})
}

// durationCardLogout 清除时长卡的在线校验信息，不修改 end_time。提供 needle
// 时只退出匹配会话；省略时允许持卡客户端强制结束当前会话。
func durationCardLogout(ctx *gin.Context) {
	value, ok := ctx.Get("card")
	cardContext, valid := value.(卡密请求上下文)
	if !ok || !valid {
		失败提示(ctx, "卡密上下文错误")
		return
	}
	tableName, err := 时长卡数据表名(cardContext.Name)
	if err != nil {
		失败提示(ctx, err.Error())
		return
	}
	needle := strings.TrimSpace(input(ctx, "needle"))
	if cacheErr := 同步并删除时长卡心跳缓存(cardContext.Name, cardContext.Card, needle); cacheErr != nil {
		失败提示(ctx, cacheErr.Error())
		return
	}
	query := db.Table(tableName).Where("card = ?", cardContext.Card)
	if needle != "" {
		query = query.Where("needle = ?", needle)
	}
	if result := query.Updates(map[string]interface{}{"needle": "", "last_heartbeat_at": nil}); result.Error != nil {
		失败提示(ctx, "退出时长卡失败")
		return
	}
	// 退出更新提交后再次失效，防止并发心跳在数据库更新前后重新建立旧缓存。
	if cacheErr := 同步并删除时长卡心跳缓存(cardContext.Name, cardContext.Card, ""); cacheErr != nil {
		日志("log/启动记录.txt", "退出时长卡后同步心跳缓存失败:"+cacheErr.Error())
	}
	成功提示(ctx, gin.H{"msg": "退出成功"})
}

// durationCardQuery 返回当前时长卡状态但不触发激活；只有登录接口能把
// 未激活卡转为已激活。
func durationCardQuery(ctx *gin.Context) {
	value, ok := ctx.Get("card")
	cardContext, valid := value.(卡密请求上下文)
	if !ok || !valid {
		失败提示(ctx, "卡密上下文错误")
		return
	}
	data, err := 时长卡详情(cardContext.Name, cardContext.Card, time.Now())
	if err != nil {
		失败提示(ctx, err.Error())
		return
	}
	成功提示(ctx, gin.H{"data": data})
}

// 时长卡公告按卡密绑定的软件读取，和点卡公告使用同一软件表但不读取
// 点卡卡密记录，避免同名卡在两种模式间串数据。
func durationCardBulletin(ctx *gin.Context) {
	value, ok := ctx.Get("card")
	cardContext, valid := value.(卡密请求上下文)
	if !ok || !valid {
		失败提示(ctx, "卡密上下文错误")
		return
	}
	row, found, err := 读取时长卡(cardContext.Name, cardContext.Card)
	if err != nil {
		失败提示(ctx, err.Error())
		return
	}
	if !found {
		失败提示(ctx, "时长卡不存在")
		return
	}
	var softwareRow 软件
	query := db.Table("software").Where("name = ? AND id = ?", cardContext.Name, row.Software).First(&softwareRow)
	if errors.Is(query.Error, gorm.ErrRecordNotFound) {
		失败提示(ctx, "软件不存在")
		return
	}
	if query.Error != nil {
		失败提示(ctx, "读取软件公告失败")
		return
	}
	成功提示(ctx, gin.H{"data": softwareRow.Bulletin, "software": row.Software})
}

// durationCardConfig 沿用点卡的 read/write 参数和长度限制，但只更新独立
// 时长卡表，不能借同名卡修改点卡配置。
func durationCardConfig(ctx *gin.Context) {
	value, ok := ctx.Get("card")
	cardContext, valid := value.(卡密请求上下文)
	if !ok || !valid {
		失败提示(ctx, "卡密上下文错误")
		return
	}
	tableName, err := 时长卡数据表名(cardContext.Name)
	if err != nil {
		失败提示(ctx, err.Error())
		return
	}
	operation := input(ctx, "type")
	if operation == "write" {
		content := input(ctx, "value")
		if err := 校验卡密配置内容(content); err != nil {
			失败提示(ctx, err.Error())
			return
		}
		if result := db.Table(tableName).Where("card = ?", cardContext.Card).Update("config_content", content); result.Error != nil {
			失败提示(ctx, "保存配置失败")
			return
		}
	} else if operation != "" && operation != "read" {
		失败提示(ctx, "type参数不正确")
		return
	}
	row, found, err := 读取时长卡(cardContext.Name, cardContext.Card)
	if err != nil {
		失败提示(ctx, err.Error())
		return
	}
	if !found {
		失败提示(ctx, "时长卡不存在")
		return
	}
	成功提示(ctx, row.ConfigContent)
}
