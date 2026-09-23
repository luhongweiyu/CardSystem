package main

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 时长卡代理价格输入是管理端保存价格时使用的接口结构。价格以代理余额
// 点表示，允许两位小数；落库前会转换为百分之一点的整数。
type 时长卡代理价格输入 struct {
	DurationMinutes int64   `json:"duration_minutes"`
	Price           float64 `json:"price"`
	Enabled         *bool   `json:"enabled"`
}

// 时长卡代理价格查询请求按代理账号查询全部软件，software 为 0 时不限制
// 软件；管理员只能查询自己名下的代理账号。
type 时长卡代理价格查询请求 struct {
	AgentID  int `json:"agent_id"`
	Software int `json:"software"`
}

// 时长卡代理价格保存请求一次替换一个代理账号的一个软件价格集合。
// 空 prices 表示清空该软件的全部价格锚点，从而直接撤销该代理的发卡权限。
type 时长卡代理价格保存请求 struct {
	AgentID  int         `json:"agent_id"`
	Software int         `json:"software"`
	Prices   []时长卡代理价格输入 `json:"prices"`
}

// 删除请求使用完整作用域而不是只接受数据库自增 ID，避免管理员误删其他
// 代理的价格，也让接口在数据库迁移后仍然容易排查。
type 时长卡代理价格删除请求 struct {
	AgentID         int   `json:"agent_id"`
	Software        int   `json:"software"`
	DurationMinutes int64 `json:"duration_minutes"`
}

// 时长卡代理价格展示项隐藏数据库内部的百分之一点存储格式，接口返回
// 适合页面显示的普通价格。
type 时长卡代理价格展示项 struct {
	ID              uint    `json:"id"`
	AgentID         int     `json:"agent_id"`
	Software        int     `json:"software"`
	DurationMinutes int64   `json:"duration_minutes"`
	Price           float64 `json:"price"`
	Enabled         bool    `json:"enabled"`
}

// 时长卡代理报价同时用于价格预览和生成成功响应。Charge 是本次批量
// 需要扣除的整数余额；PricePerCard 仅供页面展示，不作为扣款依据。
type 时长卡代理报价 struct {
	DurationMinutes           int64   `json:"duration_minutes"`
	Num                       int     `json:"num"`
	PricePerCard              float64 `json:"price_per_card"`
	Charge                    int64   `json:"charge"`
	PricingMode               string  `json:"pricing_mode"`
	LowerDurationMinutes      int64   `json:"lower_duration_minutes,omitempty"`
	UpperDurationMinutes      int64   `json:"upper_duration_minutes,omitempty"`
	RateSourceDurationMinutes int64   `json:"rate_source_duration_minutes,omitempty"`
}

// 代理时长卡生成结果把实际扣款和生成的卡密一起返回。价格会在事务内
// 重新计算，因此这里的报价始终对应本次真实扣款，而不是预览快照。
type 代理生成时长卡结果 struct {
	Cards                     []string `json:"-"`
	Charge                    int64    `json:"charge"`
	Balance                   int64    `json:"balance"`
	PricePerCard              float64  `json:"price_per_card"`
	PricingMode               string   `json:"pricing_mode"`
	LowerDurationMinutes      int64    `json:"lower_duration_minutes,omitempty"`
	UpperDurationMinutes      int64    `json:"upper_duration_minutes,omitempty"`
	RateSourceDurationMinutes int64    `json:"rate_source_duration_minutes,omitempty"`
}

// 时长卡代理价格输入转为整数点。金额上限沿用单次点数上限，避免一个
// 锚点价格在批量计算时造成 int64 溢出；真正的批量总额还会在计价函数中复核。
func 代理时长价格转整数(price float64) (int64, error) {
	if math.IsNaN(price) || math.IsInf(price, 0) || price <= 0 || price > float64(最大单次点数) {
		return 0, fmt.Errorf("时长卡价格必须大于0且不超过%d点", 最大单次点数)
	}
	scaled := price * float64(时长卡代理价格最小单位)
	rounded := math.Round(scaled)
	if math.Abs(scaled-rounded) > 1e-8 || rounded > float64(math.MaxInt64) {
		return 0, fmt.Errorf("时长卡价格最多保留两位小数")
	}
	return int64(rounded), nil
}

// 规范化时长卡代理价格输入负责校验时长、价格和重复锚点，并按时长排序。
// 排序和重复检查在事务外完成，避免管理员保存价格时长时间持有数据库锁。
func 规范化时长卡代理价格输入(inputs []时长卡代理价格输入) ([]时长卡代理价格, error) {
	if len(inputs) > 时长卡代理价格最大锚点数 {
		return nil, fmt.Errorf("单个软件最多配置%d个时长价格", 时长卡代理价格最大锚点数)
	}
	result := make([]时长卡代理价格, 0, len(inputs))
	seen := make(map[int64]struct{}, len(inputs))
	for _, input := range inputs {
		if input.DurationMinutes < 时长卡最小时长分钟 || input.DurationMinutes > 时长卡永久分钟 {
			return nil, fmt.Errorf("价格锚点时长必须在%d分钟至%d天之间", 时长卡最小时长分钟, 时长卡永久分钟/1440)
		}
		if _, exists := seen[input.DurationMinutes]; exists {
			return nil, fmt.Errorf("价格锚点时长%d分钟重复", input.DurationMinutes)
		}
		price, err := 代理时长价格转整数(input.Price)
		if err != nil {
			return nil, err
		}
		enabled := true
		if input.Enabled != nil {
			enabled = *input.Enabled
		}
		seen[input.DurationMinutes] = struct{}{}
		result = append(result, 时长卡代理价格{DurationMinutes: input.DurationMinutes, Price: price, Enabled: enabled})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].DurationMinutes < result[j].DurationMinutes
	})
	return result, nil
}

// 正整数乘法在计价前主动检查溢出，避免极端配置把余额变成负数。
func 时长卡代理价格安全乘法(left, right int64) (int64, error) {
	if left < 0 || right < 0 {
		return 0, fmt.Errorf("价格计算参数不正确")
	}
	if left != 0 && right > math.MaxInt64/left {
		return 0, fmt.Errorf("消费金额超出允许范围")
	}
	return left * right, nil
}

// 正整数向上整除不使用“分子+分母-1”，避免接近 int64 上限时再次溢出。
func 时长卡代理价格向上整除(numerator, denominator int64) (int64, error) {
	if numerator < 0 || denominator <= 0 {
		return 0, fmt.Errorf("价格计算参数不正确")
	}
	result := numerator / denominator
	if numerator%denominator != 0 {
		if result == math.MaxInt64 {
			return 0, fmt.Errorf("消费金额超出允许范围")
		}
		result++
	}
	return result, nil
}

// 计算时长卡代理费用按启用锚点计算一个批次的实际扣款。
//
// 精确命中锚点时保留管理员配置的总价；非精确时长必须落在最短和最长
// 启用锚点之间，取相邻左右锚点中较高的平均每分钟价格折算。所有金额
// 运算都使用整数分子/分母，只有返回给页面的展示价格才转换成 float64。
func 计算时长卡代理费用(rows []时长卡代理价格, requestedMinutes int64, count int) (时长卡代理报价, error) {
	if requestedMinutes < 时长卡最小时长分钟 || requestedMinutes > 时长卡永久分钟 {
		return 时长卡代理报价{}, fmt.Errorf("生成时长必须在%d分钟至%d天之间", 时长卡最小时长分钟, 时长卡永久分钟/1440)
	}
	if count <= 0 || count > 时长卡代理计价最大数量 {
		return 时长卡代理报价{}, fmt.Errorf("计价数量必须在1至%d之间", 时长卡代理计价最大数量)
	}

	active := make([]时长卡代理价格, 0, len(rows))
	seen := make(map[int64]struct{}, len(rows))
	for _, row := range rows {
		if row.DurationMinutes < 时长卡最小时长分钟 || row.DurationMinutes > 时长卡永久分钟 || row.Price <= 0 || row.Price > 最大单次点数*时长卡代理价格最小单位 {
			return 时长卡代理报价{}, fmt.Errorf("数据库中的时长卡代理价格配置不正确")
		}
		if _, exists := seen[row.DurationMinutes]; exists {
			return 时长卡代理报价{}, fmt.Errorf("数据库中的时长卡代理价格锚点重复")
		}
		seen[row.DurationMinutes] = struct{}{}
		if row.Enabled {
			active = append(active, row)
		}
	}
	if len(active) == 0 {
		return 时长卡代理报价{}, fmt.Errorf("该软件尚未配置可用的时长卡价格")
	}
	sort.Slice(active, func(i, j int) bool {
		return active[i].DurationMinutes < active[j].DurationMinutes
	})

	base := 时长卡代理报价{DurationMinutes: requestedMinutes, Num: count}
	for _, row := range active {
		if row.DurationMinutes != requestedMinutes {
			continue
		}
		numerator, err := 时长卡代理价格安全乘法(row.Price, int64(count))
		if err != nil {
			return 时长卡代理报价{}, err
		}
		charge, err := 时长卡代理价格向上整除(numerator, 时长卡代理价格最小单位)
		if err != nil {
			return 时长卡代理报价{}, err
		}
		base.PricePerCard = math.Round((float64(row.Price)/float64(时长卡代理价格最小单位))*100) / 100
		base.Charge = charge
		base.PricingMode = "exact"
		base.LowerDurationMinutes = requestedMinutes
		base.UpperDurationMinutes = requestedMinutes
		base.RateSourceDurationMinutes = requestedMinutes
		return base, nil
	}

	if requestedMinutes < active[0].DurationMinutes || requestedMinutes > active[len(active)-1].DurationMinutes {
		return 时长卡代理报价{}, fmt.Errorf("生成时长超出代理价格覆盖范围：%d至%d分钟", active[0].DurationMinutes, active[len(active)-1].DurationMinutes)
	}
	var lower, upper *时长卡代理价格
	for index := range active {
		if active[index].DurationMinutes < requestedMinutes {
			lower = &active[index]
			continue
		}
		if active[index].DurationMinutes > requestedMinutes {
			upper = &active[index]
			break
		}
	}
	if lower == nil || upper == nil {
		return 时长卡代理报价{}, fmt.Errorf("生成时长没有可用的相邻价格锚点")
	}

	// 比较 p1/d1 与 p2/d2 时交叉相乘，不做浮点除法，避免平均单价相等
	// 或接近时因精度误差选择错误的价格来源。
	leftProduct, err := 时长卡代理价格安全乘法(lower.Price, upper.DurationMinutes)
	if err != nil {
		return 时长卡代理报价{}, err
	}
	rightProduct, err := 时长卡代理价格安全乘法(upper.Price, lower.DurationMinutes)
	if err != nil {
		return 时长卡代理报价{}, err
	}
	source := lower
	if rightProduct > leftProduct {
		source = upper
	}
	targetNumerator, err := 时长卡代理价格安全乘法(requestedMinutes, source.Price)
	if err != nil {
		return 时长卡代理报价{}, err
	}
	batchNumerator, err := 时长卡代理价格安全乘法(targetNumerator, int64(count))
	if err != nil {
		return 时长卡代理报价{}, err
	}
	denominator, err := 时长卡代理价格安全乘法(source.DurationMinutes, 时长卡代理价格最小单位)
	if err != nil {
		return 时长卡代理报价{}, err
	}
	charge, err := 时长卡代理价格向上整除(batchNumerator, denominator)
	if err != nil {
		return 时长卡代理报价{}, err
	}
	base.PricePerCard = math.Round((float64(targetNumerator)/float64(denominator))*100) / 100
	base.Charge = charge
	base.PricingMode = "between"
	base.LowerDurationMinutes = lower.DurationMinutes
	base.UpperDurationMinutes = upper.DurationMinutes
	base.RateSourceDurationMinutes = source.DurationMinutes
	return base, nil
}

// 读取时长卡代理价格统一处理管理员、代理和事务连接。lock=true 只在发卡
// 或保存配置的短事务中使用，保证一次发卡不会跨越价格替换操作。
func 读取时长卡代理价格(tx *gorm.DB, admin string, agentID, softwareID int, enabledOnly, lock bool) ([]时长卡代理价格, error) {
	if tx == nil || !验证管理员名称(strings.TrimSpace(admin)) || agentID < 0 || softwareID < 0 {
		return nil, fmt.Errorf("读取时长卡价格参数不正确")
	}
	query := tx.Table(时长卡代理价格表名).Where("admin = ?", strings.TrimSpace(admin))
	if agentID > 0 {
		query = query.Where("agent_id = ?", agentID)
	}
	if softwareID > 0 {
		query = query.Where("software = ?", softwareID)
	}
	if enabledOnly {
		query = query.Where("enabled = ?", true)
	}
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var rows []时长卡代理价格
	if err := query.Order("duration_minutes ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("读取时长卡代理价格失败")
	}
	return rows, nil
}

// 检查时长卡软件只做非锁定的归属校验，供价格查询和预览使用；真正
// 发卡或替换价格时再由锁定发卡软件取得行锁。
func 检查时长卡软件(tx *gorm.DB, admin string, softwareID int) error {
	if tx == nil || !验证管理员名称(strings.TrimSpace(admin)) || softwareID <= 0 {
		return fmt.Errorf("软件参数不正确")
	}
	var row 软件
	query := tx.Table("software").Select("id").Where("name = ? AND id = ?", strings.TrimSpace(admin), softwareID).First(&row)
	if errors.Is(query.Error, gorm.ErrRecordNotFound) {
		return fmt.Errorf("软件不存在")
	}
	if query.Error != nil {
		return fmt.Errorf("检查软件失败")
	}
	return nil
}

// 锁定时长卡代理账号返回事务内的最新余额和所属管理员，发卡扣款不能
// 使用登录时保存的旧账号快照。
func 锁定时长卡代理账号(tx *gorm.DB, admin string, agentID int) (代理账号记录, error) {
	var account 代理账号记录
	if tx == nil || !验证管理员名称(strings.TrimSpace(admin)) || agentID <= 0 {
		return account, fmt.Errorf("代理账号参数不正确")
	}
	query := tx.Table(代理账号表名).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND admin = ?", agentID, strings.TrimSpace(admin)).First(&account)
	if errors.Is(query.Error, gorm.ErrRecordNotFound) {
		return account, fmt.Errorf("代理账号不存在")
	}
	if query.Error != nil {
		return account, fmt.Errorf("读取代理账号失败")
	}
	return account, nil
}

// 校验时长卡代理账号归属用于只读接口，避免管理员通过 agent_id 读取其他
// 管理员的数据；写接口会使用上面的行锁版本。
func 校验时长卡代理账号归属(tx *gorm.DB, admin string, agentID int) error {
	if tx == nil || !验证管理员名称(strings.TrimSpace(admin)) || agentID <= 0 {
		return fmt.Errorf("代理账号参数不正确")
	}
	var count int64
	if err := tx.Table(代理账号表名).Where("id = ? AND admin = ?", agentID, strings.TrimSpace(admin)).Count(&count).Error; err != nil {
		return fmt.Errorf("检查代理账号失败")
	}
	if count != 1 {
		return fmt.Errorf("代理账号不存在")
	}
	return nil
}

// 管理员_查询时长卡代理价格返回指定代理的全部锚点。管理端通常按代理
// 选择后查询，因此不提供无范围的全表导出，避免代理数量增长时响应过大。
func 管理员_查询时长卡代理价格(ctx *gin.Context) {
	var request 时长卡代理价格查询请求
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil || request.AgentID <= 0 || request.Software < 0 {
		失败提示管理端(ctx, "代理账号或软件参数不正确")
		return
	}
	parent, ok := 管理员_取账号信息(ctx)
	if !ok {
		失败提示管理端(ctx, "登录状态错误")
		return
	}
	if err := 校验时长卡代理账号归属(db, parent.Name, request.AgentID); err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	if request.Software > 0 {
		if err := 检查时长卡软件(db, parent.Name, request.Software); err != nil {
			失败提示管理端(ctx, err.Error())
			return
		}
	}
	rows, err := 读取时长卡代理价格(db, parent.Name, request.AgentID, request.Software, false, false)
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	data := make([]时长卡代理价格展示项, 0, len(rows))
	for _, row := range rows {
		data = append(data, 时长卡代理价格展示项{ID: row.ID, AgentID: row.AgentID, Software: row.Software, DurationMinutes: row.DurationMinutes, Price: float64(row.Price) / float64(时长卡代理价格最小单位), Enabled: row.Enabled})
	}
	成功提示管理端(ctx, gin.H{"data": data})
}

// 管理员_保存时长卡代理价格以“代理账号 -> 软件 -> 价格行”的顺序加锁，
// 与代理发卡保持一致。整组替换使保存失败时不会留下半套价格。
func 管理员_保存时长卡代理价格(ctx *gin.Context) {
	var request 时长卡代理价格保存请求
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil || request.AgentID <= 0 || request.Software <= 0 {
		失败提示管理端(ctx, "代理账号或软件参数不正确")
		return
	}
	if request.Prices == nil {
		失败提示管理端(ctx, "prices不能为空；如需清空价格请传空数组")
		return
	}
	prices, err := 规范化时长卡代理价格输入(request.Prices)
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	parent, ok := 管理员_取账号信息(ctx)
	if !ok {
		失败提示管理端(ctx, "登录状态错误")
		return
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		if _, err := 锁定时长卡代理账号(tx, parent.Name, request.AgentID); err != nil {
			return err
		}
		if err := 锁定发卡软件(tx, parent.Name, request.Software); err != nil {
			return err
		}
		if err := tx.Table(时长卡代理价格表名).
			Where("admin = ? AND agent_id = ? AND software = ?", parent.Name, request.AgentID, request.Software).
			Delete(&时长卡代理价格{}).Error; err != nil {
			return fmt.Errorf("清理旧时长卡价格失败")
		}
		if len(prices) == 0 {
			return nil
		}
		rows := make([]时长卡代理价格, 0, len(prices))
		for _, price := range prices {
			price.Admin, price.AgentID, price.Software = parent.Name, request.AgentID, request.Software
			rows = append(rows, price)
		}
		if err := tx.Table(时长卡代理价格表名).CreateInBatches(&rows, 50).Error; err != nil {
			return fmt.Errorf("保存时长卡价格失败")
		}
		return nil
	})
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	日志("log/"+parent.Name+time.Now().Format("200601"), fmt.Sprintf("保存代理时长卡价格;代理ID:%d;软件:%d;锚点数:%d", request.AgentID, request.Software, len(prices)))
	成功提示管理端(ctx, gin.H{"msg": "保存成功", "count": len(prices)})
}

// 管理员_删除时长卡代理价格删除一个具体锚点。删除和发卡使用相同的
// 锁顺序，避免管理员改价与代理扣款互相等待形成死锁。
func 管理员_删除时长卡代理价格(ctx *gin.Context) {
	var request 时长卡代理价格删除请求
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil || request.AgentID <= 0 || request.Software <= 0 {
		失败提示管理端(ctx, "代理账号或软件参数不正确")
		return
	}
	if request.DurationMinutes < 时长卡最小时长分钟 || request.DurationMinutes > 时长卡永久分钟 {
		失败提示管理端(ctx, "价格锚点时长不正确")
		return
	}
	parent, ok := 管理员_取账号信息(ctx)
	if !ok {
		失败提示管理端(ctx, "登录状态错误")
		return
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		if _, err := 锁定时长卡代理账号(tx, parent.Name, request.AgentID); err != nil {
			return err
		}
		if err := 锁定发卡软件(tx, parent.Name, request.Software); err != nil {
			return err
		}
		result := tx.Table(时长卡代理价格表名).Where("admin = ? AND agent_id = ? AND software = ? AND duration_minutes = ?", parent.Name, request.AgentID, request.Software, request.DurationMinutes).Delete(&时长卡代理价格{})
		if result.Error != nil {
			return fmt.Errorf("删除时长卡价格失败")
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("时长卡价格锚点不存在")
		}
		return nil
	})
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	成功提示管理端(ctx, gin.H{"msg": "删除成功"})
}

// 代理账号_查询时长卡价格只返回当前代理启用的锚点。代理不需要也不应
// 看到管理员暂时停用的内部价格记录。
func 代理账号_查询时长卡价格(ctx *gin.Context) {
	account := 代理账号_取账号信息(ctx)
	rows, err := 读取时长卡代理价格(db, account.Admin, account.ID, 0, true, false)
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	data := make([]时长卡代理价格展示项, 0, len(rows))
	for _, row := range rows {
		data = append(data, 时长卡代理价格展示项{ID: row.ID, AgentID: row.AgentID, Software: row.Software, DurationMinutes: row.DurationMinutes, Price: float64(row.Price) / float64(时长卡代理价格最小单位), Enabled: row.Enabled})
	}
	成功提示管理端(ctx, gin.H{"data": data})
}

// 代理账号_预览时长卡价格只读取价格和软件归属，不生成卡密也不锁余额。
// Num 省略时按一张卡预览，方便页面在输入数量前先展示单卡价格。
func 代理账号_预览时长卡价格(ctx *gin.Context) {
	var request struct {
		Software        int   `json:"software"`
		DurationMinutes int64 `json:"duration_minutes"`
		Num             int   `json:"num"`
	}
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil {
		失败提示管理端(ctx, "数据错误")
		return
	}
	if request.Num == 0 {
		request.Num = 1
	}
	account := 代理账号_取账号信息(ctx)
	if account.ID <= 0 || !验证管理员名称(account.Admin) || request.Software <= 0 {
		失败提示管理端(ctx, "软件或代理账号参数不正确")
		return
	}
	if err := 检查时长卡软件(db, account.Admin, request.Software); err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	rows, err := 读取时长卡代理价格(db, account.Admin, account.ID, request.Software, true, false)
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	quote, err := 计算时长卡代理费用(rows, request.DurationMinutes, request.Num)
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	成功提示管理端(ctx, gin.H{"data": quote})
}

// 代理生成时长卡在事务内锁定最新代理余额、软件和价格锚点，重新计算实际
// 扣款后先扣代理余额，再写入时长卡。任意写入失败都会回滚整批操作。
func 代理生成时长卡(account 代理账号记录, request 时长卡生成请求) (代理生成时长卡结果, error) {
	if account.ID <= 0 || !验证管理员名称(account.Admin) {
		return 代理生成时长卡结果{}, fmt.Errorf("代理账号状态错误")
	}
	tableName, cards, normalized, err := 准备生成时长卡(account.Admin, request)
	if err != nil {
		return 代理生成时长卡结果{}, err
	}
	now := time.Now()
	var result 代理生成时长卡结果
	err = db.Transaction(func(tx *gorm.DB) error {
		current, err := 锁定时长卡代理账号(tx, account.Admin, account.ID)
		if err != nil {
			return err
		}
		if err := 锁定发卡软件(tx, current.Admin, normalized.Software); err != nil {
			return err
		}
		prices, err := 读取时长卡代理价格(tx, current.Admin, current.ID, normalized.Software, true, true)
		if err != nil {
			return err
		}
		quote, err := 计算时长卡代理费用(prices, normalized.DurationMinutes, len(cards))
		if err != nil {
			return err
		}
		if _, err := 检查代理扣款余额(current, quote.Charge); err != nil {
			return err
		}
		if update := tx.Table(代理账号表名).
			Where("id = ? AND admin = ?", current.ID, current.Admin).
			UpdateColumn("balance", gorm.Expr("balance - ?", quote.Charge)); update.Error != nil || update.RowsAffected != 1 {
			return fmt.Errorf("扣除代理余额失败")
		}
		// 余额扣减和时长卡写入处于同一事务；保存失败时上面的扣款会
		// 一并回滚，不会出现代理余额减少但卡密缺失的半成功状态。
		if err := 保存时长卡批次(tx, tableName, current.ID, normalized, cards, now); err != nil {
			return fmt.Errorf("生成时长卡失败: %w", err)
		}
		result = 代理生成时长卡结果{Cards: cards, Charge: quote.Charge, Balance: current.Balance - quote.Charge,
			PricePerCard: quote.PricePerCard, PricingMode: quote.PricingMode,
			LowerDurationMinutes: quote.LowerDurationMinutes, UpperDurationMinutes: quote.UpperDurationMinutes,
			RateSourceDurationMinutes: quote.RateSourceDurationMinutes}
		return nil
	})
	if err != nil {
		return 代理生成时长卡结果{}, err
	}
	代理账号日志(account.ID, fmt.Sprintf("余额:%d", result.Balance), fmt.Sprintf("变更:-%d", result.Charge), "原因:生成时长卡", fmt.Sprintf("软件:%d", normalized.Software), fmt.Sprintf("时长:%d分钟", normalized.DurationMinutes), fmt.Sprintf("数量:%d", len(result.Cards)), "计价方式:"+result.PricingMode, fmt.Sprintf("价格来源:%d分钟", result.RateSourceDurationMinutes))
	return result, nil
}

// 代理账号_添加时长卡是代理发放固定时长卡的唯一入口。前端可先调用
// price_preview，但最终价格和余额始终以本函数事务内的重新计算为准。
func 代理账号_添加时长卡(ctx *gin.Context) {
	var request 时长卡生成请求
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil {
		失败提示管理端(ctx, "数据错误")
		return
	}
	account := 代理账号_取账号信息(ctx)
	generated, err := 代理生成时长卡(account, request)
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	成功提示管理端(ctx, gin.H{
		"msg":                          fmt.Sprintf("成功生成%d张时长卡", len(generated.Cards)),
		"data":                         strings.Join(generated.Cards, "\n"),
		"charge":                       generated.Charge,
		"balance":                      generated.Balance,
		"price_per_card":               generated.PricePerCard,
		"pricing_mode":                 generated.PricingMode,
		"lower_duration_minutes":       generated.LowerDurationMinutes,
		"upper_duration_minutes":       generated.UpperDurationMinutes,
		"rate_source_duration_minutes": generated.RateSourceDurationMinutes,
	})
}
