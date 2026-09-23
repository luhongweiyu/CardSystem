package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	代理账号表名      = "agent_account"
	最大代理价格配置字节数 = 4096
	最大代理价格软件数量  = 100
	最大代理余额下限绝对值 = int64(1000000000)
)

// 代理账号记录保存代理身份、独立余额和价格配置。代理余额用于发卡和
// 主动开启的点卡余额不足代扣，不等同于已生成卡密的点数余额。
type 代理账号记录 struct {
	ID    int    `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	Admin string `json:"admin" gorm:"column:admin;size:32;not null;index:idx_agent_admin"`
	// 代理账号登录按名称查询，因此名称必须全局唯一；应用层返回友好提示，
	// 唯一索引负责兜住并发创建的竞态。
	Name string `json:"name" gorm:"column:name;size:32;not null;uniqueIndex:uk_agent_name"`
	// password 只作为账号资料留档，认证仍然只比较 PasswordHash；接口模型
	// 使用 json:"-"，避免列表、登录和代理上下文意外泄露原值。
	Password     string `json:"-" gorm:"column:password;size:255;default:''"`
	PasswordHash string `json:"-" gorm:"column:password_hash;size:255;default:''"`
	Balance      int64  `json:"balance" gorm:"column:balance;not null;default:0"`
	Prices       string `json:"prices" gorm:"column:prices;type:text;not null"`
	// PointCardAutoDeduct 由代理自己控制，决定卡内点数不足时是否尝试
	// 使用自己的代理余额补足；管理员只控制统一余额下限。
	PointCardAutoDeduct bool `json:"point_card_auto_deduct" gorm:"column:point_card_auto_deduct;not null;default:false"`
	// 复用旧的数值列，不迁移现有数据；负数可欠费，正数表示必须保留的余额。
	MinBalance int64 `json:"min_balance" gorm:"column:point_debt_limit;not null;default:0"`
}

// TableName 固定代理账号模型的物理表名。业务查询即使没有显式调用 Table，
// 也不会因为中文类型名被 GORM 推导成其他表名。
func (代理账号记录) TableName() string {
	return 代理账号表名
}

type 代理账号列表项 struct {
	ID                  int    `json:"id"`
	Name                string `json:"name"`
	Balance             int64  `json:"balance"`
	Prices              string `json:"prices"`
	PointCardAutoDeduct bool   `json:"point_card_auto_deduct"`
	MinBalance          int64  `json:"min_balance"`
}

func 代理账号列表项转换(account 代理账号记录) 代理账号列表项 {
	return 代理账号列表项{
		ID: account.ID, Name: account.Name, Balance: account.Balance, Prices: account.Prices,
		PointCardAutoDeduct: account.PointCardAutoDeduct, MinBalance: account.MinBalance,
	}
}

func 代理账号登录(ctx *gin.Context) {
	account := 代理账号_取账号信息(ctx)
	if account.ID <= 0 {
		失败提示管理端(ctx, "登录状态错误")
		return
	}
	if !验证管理员名称(account.Admin) {
		失败提示管理端(ctx, "所属管理员不存在")
		return
	}
	// 代理账号仍使用所属管理员的访客查询入口，因此登录时一并返回管理员 ID，
	// 前端无需猜测或把管理员名称错误地当成 center_id。
	var parent user
	if err := db_user.Select("id", "name").Where("name = ?", account.Admin).First(&parent).Error; err != nil {
		失败提示管理端(ctx, "所属管理员不存在")
		return
	}
	session := 全局_登录会话.创建会话(account.Name, true)
	成功提示管理端(ctx, gin.H{"msg": "登录成功", "id": account.ID, "admin": account.Admin, "center_id": parent.ID, "name": account.Name, "balance": account.Balance, "prices": 解析代理价格(account.Prices), "point_card_auto_deduct": account.PointCardAutoDeduct, "min_balance": account.MinBalance, "token": session.Token, "expires_at": session.ExpiresAt})
}

func 管理员_创建代理账号(ctx *gin.Context) {
	// 业务字段使用 agent_ 前缀，避免管理端认证中间件把待创建账号的 name
	// 误认为当前管理员名称。
	var request struct {
		Name     string `json:"agent_name"`
		Password string `json:"agent_password"`
	}
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil {
		失败提示管理端(ctx, "数据错误")
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	if !验证管理员名称(request.Name) {
		失败提示管理端(ctx, "渠道合伙人账号只能使用3至32位字母、数字或下划线")
		return
	}
	if err := 校验新密码(request.Password); err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	parent, ok := 管理员_取账号信息(ctx)
	if !ok {
		失败提示管理端(ctx, "登录状态错误")
		return
	}
	var count int64
	if err := db_代理账号.Where("name = ?", request.Name).Count(&count).Error; err != nil {
		失败提示管理端(ctx, "检查渠道合伙人失败")
		return
	}
	if count > 0 {
		失败提示管理端(ctx, "渠道合伙人账号名称已存在")
		return
	}
	hash, err := 生成密码哈希(request.Password)
	if err != nil {
		失败提示管理端(ctx, "密码处理失败")
		return
	}
	account := 代理账号记录{Name: request.Name, Password: request.Password, PasswordHash: hash, Admin: parent.Name, Prices: "{}"}
	if err := db_代理账号.Create(&account).Error; err != nil {
		失败提示管理端(ctx, "创建渠道合伙人失败")
		return
	}
	成功提示管理端(ctx, gin.H{"msg": "创建成功", "data": 代理账号列表项转换(account)})
}

func 代理账号_取账号信息(ctx *gin.Context) 代理账号记录 {
	value, _ := ctx.Get("代理账号信息")
	account, _ := value.(代理账号记录)
	return account
}

// 解析代理价格读取“软件ID -> 每1点卡点数的代理余额价格”。纯点卡只需
// 这一层映射，不再保留时长档位等嵌套结构。
func 解析代理价格(raw string) map[int]float64 {
	result := make(map[int]float64)
	var values map[string]*float64
	if json.Unmarshal([]byte(raw), &values) != nil || values == nil {
		return result
	}
	for key, price := range values {
		id, err := strconv.Atoi(key)
		// 只接受规范十进制软件 ID。否则 "1" 和 "01" 会在解析后指向
		// 同一软件，并因 map 遍历顺序不固定而得到不确定的代理价格。
		if err != nil || id <= 0 || strconv.Itoa(id) != key || price == nil || !代理每点价格有效(*price) {
			continue
		}
		result[id] = *price
	}
	return result
}

// 校验代理价格配置验证管理员为代理账号设置的“每点成本” JSON。
func 校验代理价格配置(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("价格配置不能为空")
	}
	if len([]byte(raw)) > 最大代理价格配置字节数 {
		return fmt.Errorf("价格配置不能超过%d字节", 最大代理价格配置字节数)
	}
	var values map[string]*float64
	if err := json.Unmarshal([]byte(raw), &values); err != nil || values == nil {
		return fmt.Errorf("价格配置必须是软件ID到每点价格的JSON对象")
	}
	if len(values) > 最大代理价格软件数量 {
		return fmt.Errorf("价格配置最多只能包含%d个软件", 最大代理价格软件数量)
	}
	for key, price := range values {
		id, err := strconv.Atoi(key)
		if err != nil || id <= 0 || strconv.Itoa(id) != key {
			return fmt.Errorf("软件ID %q 不正确", key)
		}
		if price == nil || !代理每点价格有效(*price) {
			return fmt.Errorf("软件%d的每点价格必须大于0、不超过%d且最多保留两位小数", id, 最大单次点数)
		}
	}
	return nil
}

// 代理每点价格有效统一约束后端 JSON 和前端输入框的两位小数规则，避免页面
// 显示值与实际扣除值不同。浮点解析存在微小误差，因此比较时保留安全容差。
func 代理每点价格有效(price float64) bool {
	if price <= 0 || price > float64(最大单次点数) || math.IsNaN(price) || math.IsInf(price, 0) {
		return false
	}
	rounded := math.Round(price*100) / 100
	return math.Abs(price-rounded) <= 1e-9
}

func 代理账号日志(accountID int, fields ...string) {
	日志(fmt.Sprintf("log/代理账号%v_%v", accountID, time.Now().Format("200601")), 业务流水文本(fields...))
}

// 各扣款路径已在事务中锁定同一代理账号行；用该行最新余额和统一下限校验，
// 不额外查询数据库。管理员手动调整余额不经过此函数。
func 检查代理扣款余额(account 代理账号记录, charge int64) (int64, error) {
	if charge < 0 || account.Balance < math.MinInt64+charge {
		return 0, fmt.Errorf("代理余额超出允许范围")
	}
	after := account.Balance - charge
	if after < account.MinBalance {
		return 0, fmt.Errorf("代理余额不足，需要%d点，当前%d点，余额下限%d点", charge, account.Balance, account.MinBalance)
	}
	return after, nil
}

// 计算代理点卡费用把两位小数单价转为“百分之一点”后按整数计算，避免
// 大批量发卡时 float64 丢失整数精度。最终不足 1 点的部分统一向上取整。
func 计算代理点卡费用(unitPrice float64, pointsPerCard int64, cardCount int) (int64, error) {
	if !代理每点价格有效(unitPrice) || pointsPerCard < 0 || cardCount <= 0 {
		return 0, fmt.Errorf("渠道发卡计费参数不正确")
	}
	// 零点卡不收发卡费用，但仍需有效单价来确认该软件的发卡权限。
	if pointsPerCard == 0 {
		return 0, nil
	}
	count := int64(cardCount)
	if pointsPerCard > math.MaxInt64/count {
		return 0, fmt.Errorf("消费金额超出允许范围")
	}
	pointCount := pointsPerCard * count
	priceHundredths := int64(math.Round(unitPrice * 100))
	wholePrice := priceHundredths / 100
	fractionPrice := priceHundredths % 100

	wholeCharge := int64(0)
	if wholePrice > 0 {
		if pointCount > math.MaxInt64/wholePrice {
			return 0, fmt.Errorf("消费金额超出允许范围")
		}
		wholeCharge = pointCount * wholePrice
	}
	// fractionPrice 最大为 99；先做乘法上限检查，再用商和余数完成向上取整，
	// 不使用“+99”写法，避免靠近 int64 上限时发生加法溢出。
	fractionCharge := int64(0)
	if fractionPrice > 0 {
		if pointCount > math.MaxInt64/fractionPrice {
			return 0, fmt.Errorf("消费金额超出允许范围")
		}
		fractionProduct := pointCount * fractionPrice
		fractionCharge = fractionProduct / 100
		if fractionProduct%100 != 0 {
			fractionCharge++
		}
	}
	if wholeCharge > math.MaxInt64-fractionCharge {
		return 0, fmt.Errorf("消费金额超出允许范围")
	}
	return wholeCharge + fractionCharge, nil
}

func 代理账号_查询点卡卡密列表(ctx *gin.Context) {
	account := 代理账号_取账号信息(ctx)
	查询点卡卡密列表(ctx, account.Admin, account.ID)
}

// 代理账号_查询点卡流水只允许查看自己生成的点卡流水。流水表按管理员和点卡
// 查询，卡密归属校验放在前置查询中，避免代理通过修改 software 或卡密参数
// 读取其他代理的审计记录。
func 代理账号_查询点卡流水(ctx *gin.Context) {
	account := 代理账号_取账号信息(ctx)
	card := strings.ToLower(strings.TrimSpace(input(ctx, "card")))
	if !卡密格式规则.MatchString(card) {
		失败提示管理端(ctx, "卡密格式不正确")
		return
	}
	tableName, err := 点卡数据表名(account.Admin)
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	var cardRow 点卡表样式
	query := db.Table(tableName).Where("card = ? AND agent_id = ?", card, account.ID).First(&cardRow)
	if errors.Is(query.Error, gorm.ErrRecordNotFound) {
		失败提示管理端(ctx, "卡密不存在或无权查看")
		return
	}
	if query.Error != nil {
		失败提示管理端(ctx, "读取卡密失败")
		return
	}
	softwareID, _ := strconv.Atoi(input(ctx, "software"))
	if softwareID > 0 && softwareID != cardRow.Software {
		失败提示管理端(ctx, "软件与卡密不匹配")
		return
	}
	page, _ := strconv.Atoi(input(ctx, "page"))
	pageSize, _ := strconv.Atoi(input(ctx, "page_size"))
	page, pageSize = 规范化流水分页(page, pageSize)
	// 不按当前软件过滤，确保卡密删除后重用时，保留期内的新旧代际流水仍可查看；
	// 当前卡密归属校验已经保证代理不会看到其他卡密的记录。
	rows, total, err := 查询点卡流水记录(db_point_ledger.Where("admin = ? AND card = ?", account.Admin, card), page, pageSize)
	if err != nil {
		失败提示管理端(ctx, "查询点数流水失败")
		return
	}
	成功提示管理端(ctx, gin.H{"data": 点卡流水展示列表(rows, false), "num": total, "page": page, "page_size": pageSize, "balance": cardRow.Point_balance})
}

type 代理生成点卡卡密请求 struct {
	Software           int    `json:"software"`
	Points             int64  `json:"points"`
	Num                int    `json:"num"`
	Cards              string `json:"cards"`
	Random             bool   `json:"random"`
	Notes              string `json:"notes"`
	ConfigContent      string `json:"config_content"`
	AgentDeductionMode string `json:"point_card_auto_deduct_mode"`
}

// 代理生成点卡卡密结果把生成的点卡卡密和扣款后的渠道余额一起返回，便于页面立即刷新显示。
// 卡密写入和渠道余额扣减在同一数据库事务中完成，进程即使在请求中途崩溃，
// 也不会出现“余额已扣但卡密未生成”的半成功状态。
type 代理生成点卡卡密结果 struct {
	Cards   []string
	Charge  int64
	Balance int64
}

// 代理生成点卡卡密是代理端唯一的生成入口。代理价格从事务内锁定后的
// 最新账号记录读取，避免管理员刚修改价格时继续使用登录时的旧快照。
func 代理生成点卡卡密(account 代理账号记录, request 代理生成点卡卡密请求) (代理生成点卡卡密结果, error) {
	if account.ID <= 0 || !验证管理员名称(account.Admin) {
		return 代理生成点卡卡密结果{}, fmt.Errorf("渠道合伙人状态错误")
	}
	mode, err := 规范化点卡代扣模式(request.AgentDeductionMode)
	if err != nil {
		return 代理生成点卡卡密结果{}, err
	}
	tableName, cards, err := 准备生成点卡卡密(account.Admin, request.Software, request.Points, request.Num, request.Cards, request.Random, request.Notes, request.ConfigContent)
	if err != nil {
		return 代理生成点卡卡密结果{}, err
	}
	var result 代理生成点卡卡密结果
	err = db.Transaction(func(tx *gorm.DB) error {
		var current 代理账号记录
		query := tx.Table(代理账号表名).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND admin = ?", account.ID, account.Admin).First(&current)
		if query.Error != nil {
			return fmt.Errorf("读取渠道合伙人失败")
		}
		prices := 解析代理价格(current.Prices)
		unitPrice, exists := prices[request.Software]
		if !exists {
			return fmt.Errorf("该软件尚未配置渠道点数价格")
		}
		charge, err := 计算代理点卡费用(unitPrice, request.Points, request.Num)
		if err != nil {
			return err
		}
		if err := 锁定发卡软件(tx, current.Admin, request.Software); err != nil {
			return err
		}
		if charge > 0 {
			if _, err := 检查代理扣款余额(current, charge); err != nil {
				return err
			}
			if update := tx.Table(代理账号表名).Where("id = ? AND admin = ?", current.ID, current.Admin).UpdateColumn("balance", gorm.Expr("balance - ?", charge)); update.Error != nil || update.RowsAffected != 1 {
				return fmt.Errorf("扣除渠道余额失败")
			}
		}
		if err := 创建点卡并记录初始流水(tx, tableName, current.Admin, current.ID, request.Software, request.Points, cards, request.Notes, request.ConfigContent, mode, time.Now()); err != nil {
			return fmt.Errorf("生成卡密失败: %w", err)
		}
		balance := current.Balance - charge
		result = 代理生成点卡卡密结果{Cards: cards, Charge: charge, Balance: balance}
		return nil
	})
	if err != nil {
		return 代理生成点卡卡密结果{}, err
	}
	if result.Charge > 0 {
		代理账号日志(account.ID, fmt.Sprintf("余额:%d", result.Balance), fmt.Sprintf("变更:-%d", result.Charge), "原因:生成卡密", fmt.Sprintf("软件:%d", request.Software), fmt.Sprintf("点数:%d", request.Points), fmt.Sprintf("数量:%d", len(result.Cards)))
	}
	return result, nil
}

func 代理账号_添加点卡卡密(ctx *gin.Context) {
	var request 代理生成点卡卡密请求
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil {
		失败提示管理端(ctx, "数据错误")
		return
	}
	account := 代理账号_取账号信息(ctx)
	if request.Software <= 0 || request.Points < 0 || request.Points > 最大单次点数 || request.Num <= 0 || request.Num > 最大单次生成点卡数量 {
		失败提示管理端(ctx, "软件、点数或生成数量不正确")
		return
	}
	generated, err := 代理生成点卡卡密(account, request)
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	成功提示管理端(ctx, gin.H{"msg": fmt.Sprintf("成功生成%d张卡密", len(generated.Cards)), "data": strings.Join(generated.Cards, "\n"), "charge": generated.Charge, "balance": generated.Balance})
}

func 代理账号_删除点卡卡密(ctx *gin.Context) {
	var request struct {
		Cards []string `json:"cards"`
	}
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil {
		失败提示管理端(ctx, "数据错误")
		return
	}
	account := 代理账号_取账号信息(ctx)
	success, failed, err := 删除点卡记录(account.Admin, account.ID, request.Cards)
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	成功提示管理端(ctx, gin.H{"msg": fmt.Sprintf("成功%d张，失败%d张", len(success), len(failed)), "success": success, "failed": failed})
}

func 代理账号_修改点卡(ctx *gin.Context) {
	var request struct {
		Card               string  `json:"card"`
		Notes              *string `json:"notes"`
		Config             *string `json:"config_content"`
		CardState          int     `json:"card_state"`
		AgentDeductionMode *string `json:"point_card_auto_deduct_mode"`
	}
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil {
		失败提示管理端(ctx, "数据错误")
		return
	}
	account := 代理账号_取账号信息(ctx)
	if err := 修改点卡记录(account.Admin, account.ID, request.Card, request.Notes, request.Config, request.CardState, request.AgentDeductionMode); err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	成功提示管理端(ctx, gin.H{"msg": "修改成功"})
}

func 代理账号_批量修改点卡状态(ctx *gin.Context) {
	var request struct {
		Cards     []string `json:"cards"`
		CardState int      `json:"card_state"`
	}
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil {
		失败提示管理端(ctx, "数据错误")
		return
	}
	account := 代理账号_取账号信息(ctx)
	success, failed, err := 修改点卡_批量(account.Admin, account.ID, request.Cards, request.CardState)
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	成功提示管理端(ctx, gin.H{"msg": fmt.Sprintf("成功%d张，失败%d张", len(success), len(failed)), "success": success, "failed": failed})
}

func 代理账号_批量修改点卡代扣模式(ctx *gin.Context) {
	var request struct {
		Cards []string `json:"cards"`
		Mode  string   `json:"point_card_auto_deduct_mode"`
	}
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil {
		失败提示管理端(ctx, "数据错误")
		return
	}
	account := 代理账号_取账号信息(ctx)
	count, err := 批量修改点卡代扣模式(account.Admin, account.ID, request.Cards, request.Mode)
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	成功提示管理端(ctx, gin.H{"msg": fmt.Sprintf("已设置%d张点卡的代扣方式", count), "count": count})
}

func 代理账号_查询软件列表(ctx *gin.Context) {
	// 认证中间件每次请求都会读取账号，此处直接使用当前请求的数据。
	account := 代理账号_取账号信息(ctx)
	rows, err := 读取软件列表(account.Admin)
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	// 代理每次进入点卡页都会查询软件；顺带返回数据库中的最新余额和价格，
	// 让管理员刚完成的充值或改价无需代理退出重登即可显示在页面上。
	成功提示管理端(ctx, gin.H{"data": rows, "balance": account.Balance, "prices": 解析代理价格(account.Prices), "point_card_auto_deduct": account.PointCardAutoDeduct, "min_balance": account.MinBalance})
}

// 代理账号_修改点卡代扣设置只允许代理修改自己的总开关；余额下限
// 由管理员设置，不能通过代理接口自行放开。
func 代理账号_修改点卡代扣设置(ctx *gin.Context) {
	var request struct {
		Enabled *bool `json:"point_card_auto_deduct"`
	}
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil || request.Enabled == nil {
		失败提示管理端(ctx, "点卡代扣开关参数不正确")
		return
	}
	account := 代理账号_取账号信息(ctx)
	if account.ID <= 0 || account.Admin == "" {
		失败提示管理端(ctx, "登录状态错误")
		return
	}
	result := db.Table(代理账号表名).Where("id = ? AND admin = ?", account.ID, account.Admin).Update("point_card_auto_deduct", *request.Enabled)
	if result.Error != nil {
		失败提示管理端(ctx, "保存点卡代扣开关失败")
		return
	}
	// 相同开关值在 MySQL 中可能返回零条变更；重新读取确认账号仍存在，
	// 同时返回当前余额和余额下限，重复保存也能正常成功。
	if err := db.Table(代理账号表名).Where("id = ? AND admin = ?", account.ID, account.Admin).First(&account).Error; err != nil {
		失败提示管理端(ctx, "读取渠道合伙人设置失败")
		return
	}
	成功提示管理端(ctx, gin.H{"msg": "保存成功", "point_card_auto_deduct": account.PointCardAutoDeduct, "min_balance": account.MinBalance, "balance": account.Balance})
}

// 管理员和代理共用同一份代理日志读取范围，避免两处的月份规则不一致。
func 读取代理账号最近两月日志(accountID int) string {
	now := time.Now()
	read := func(month time.Time) string {
		content, err := os.ReadFile(fmt.Sprintf("log/代理账号%v_%v", accountID, month.Format("200601")))
		if err != nil {
			return "没有其他内容"
		}
		return string(content)
	}
	return read(now) + "\n" + read(now.AddDate(0, -1, 0))
}

func 代理账号_查询操作日志(ctx *gin.Context) {
	account := 代理账号_取账号信息(ctx)
	ctx.String(http.StatusOK, 读取代理账号最近两月日志(account.ID))
}

// 设置代理账号保存密码、软件价格和统一余额下限，不修改代理自己的代扣开关。
func 设置代理账号(ctx *gin.Context) {
	var request struct {
		Data struct {
			ID         int     `json:"id"`
			Password   string  `json:"password"`
			Prices     *string `json:"prices"`
			MinBalance *int64  `json:"min_balance"`
		} `json:"data"`
	}
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil {
		失败提示管理端(ctx, "数据错误")
		return
	}
	parent, ok := 管理员_取账号信息(ctx)
	if !ok || request.Data.ID <= 0 {
		失败提示管理端(ctx, "参数不正确")
		return
	}
	var configuredPrices map[int]float64
	// 未提交价格时保留原配置，避免仅调整欠费额度却清空软件价格。
	if request.Data.Prices != nil {
		if *request.Data.Prices == "" {
			*request.Data.Prices = "{}"
		}
		if err := 校验代理价格配置(*request.Data.Prices); err != nil {
			失败提示管理端(ctx, err.Error())
			return
		}
		configuredPrices = 解析代理价格(*request.Data.Prices)
	}
	if request.Data.Password != "" {
		if err := 校验新密码(request.Data.Password); err != nil {
			失败提示管理端(ctx, err.Error())
			return
		}
	}
	if request.Data.MinBalance != nil && (*request.Data.MinBalance < -最大代理余额下限绝对值 || *request.Data.MinBalance > 最大代理余额下限绝对值) {
		失败提示管理端(ctx, fmt.Sprintf("余额下限必须在-%d至%d之间", 最大代理余额下限绝对值, 最大代理余额下限绝对值))
		return
	}
	var agentName string
	err := db.Transaction(func(tx *gorm.DB) error {
		var account 代理账号记录
		if err := tx.Table(代理账号表名).Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND admin = ?", request.Data.ID, parent.Name).First(&account).Error; err != nil {
			return fmt.Errorf("渠道合伙人不存在")
		}
		agentName = account.Name
		if len(configuredPrices) > 0 {
			softwareIDs := make([]int, 0, len(configuredPrices))
			for softwareID := range configuredPrices {
				softwareIDs = append(softwareIDs, softwareID)
			}
			var softwareCount int64
			if err := tx.Table("software").Where("name = ? AND id IN ?", parent.Name, softwareIDs).Count(&softwareCount).Error; err != nil {
				return fmt.Errorf("校验渠道合伙人软件价格失败")
			}
			if softwareCount != int64(len(softwareIDs)) {
				return fmt.Errorf("价格配置中包含不存在或不属于当前管理员的软件")
			}
		}
		updates := map[string]interface{}{}
		if request.Data.Prices != nil {
			updates["prices"] = *request.Data.Prices
		}
		if request.Data.MinBalance != nil {
			updates["point_debt_limit"] = *request.Data.MinBalance
		}
		if err := tx.Table(代理账号表名).Where("id = ? AND admin = ?", request.Data.ID, parent.Name).Updates(updates).Error; err != nil {
			return fmt.Errorf("保存代理配置失败")
		}
		return 保存代理账号密码(tx, request.Data.ID, parent.Name, request.Data.Password)
	})
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	if request.Data.Password != "" {
		// 管理员重置代理密码后立即撤销旧令牌，避免已登录设备继续使用旧权限。
		全局_登录会话.删除账号会话(agentName, true)
	}
	成功提示管理端(ctx, gin.H{"msg": "修改成功"})
}

func 代理账号充值(ctx *gin.Context) {
	var request struct {
		ID     int    `json:"id"`
		Amount int64  `json:"amount"`
		Note   string `json:"note"`
	}
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil || request.ID <= 0 {
		失败提示管理端(ctx, "余额调整请求错误")
		return
	}
	if normalized, valid := 规范化可显示文本(request.Note, 200); !valid {
		失败提示管理端(ctx, "充值备注不能包含控制字符且不能超过200个字符")
		return
	} else {
		request.Note = normalized
	}
	parent, ok := 管理员_取账号信息(ctx)
	if !ok {
		失败提示管理端(ctx, "登录状态错误")
		return
	}
	var balance int64
	err := db.Transaction(func(tx *gorm.DB) error {
		var account 代理账号记录
		query := tx.Table(代理账号表名).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND admin = ?", request.ID, parent.Name).First(&account)
		if query.Error != nil {
			return fmt.Errorf("渠道合伙人不存在")
		}
		if request.Amount > 0 && account.Balance > math.MaxInt64-request.Amount {
			return fmt.Errorf("充值后余额超出允许范围")
		}
		if request.Amount < 0 && account.Balance < math.MinInt64-request.Amount {
			return fmt.Errorf("充值后余额超出允许范围")
		}
		balance = account.Balance + request.Amount
		if result := tx.Table(代理账号表名).Where("id = ? AND admin = ?", request.ID, parent.Name).Update("balance", balance); result.Error != nil || result.RowsAffected != 1 {
			return fmt.Errorf("充值失败")
		}
		return nil
	})
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	代理账号日志(request.ID, fmt.Sprintf("余额:%d", balance), fmt.Sprintf("变更:%+d", request.Amount), "原因:管理员充值", "备注:"+request.Note)
	成功提示管理端(ctx, gin.H{"msg": "充值成功", "balance": balance})
}

func 查询代理账号(ctx *gin.Context) {
	parent, ok := 管理员_取账号信息(ctx)
	if !ok {
		失败提示管理端(ctx, "登录状态错误")
		return
	}
	var accounts []代理账号记录
	if err := db_代理账号.Where("admin = ?", parent.Name).Order("id ASC").Find(&accounts).Error; err != nil {
		失败提示管理端(ctx, "查询渠道合伙人失败")
		return
	}
	result := make([]代理账号列表项, 0, len(accounts))
	for _, account := range accounts {
		result = append(result, 代理账号列表项转换(account))
	}
	成功提示管理端(ctx, gin.H{"data": result})
}

// 管理员只能按自己名下的代理 ID 读取日志，不能凭文件名访问其他管理员的记录。
func 管理员_查询代理账号日志(ctx *gin.Context) {
	var request struct {
		ID int `json:"id"`
	}
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil || request.ID <= 0 {
		失败提示管理端(ctx, "渠道合伙人编号不正确")
		return
	}
	parent, ok := 管理员_取账号信息(ctx)
	if !ok {
		失败提示管理端(ctx, "登录状态错误")
		return
	}
	var account 代理账号记录
	err := db_代理账号.Select("id").Where("id = ? AND admin = ?", request.ID, parent.Name).First(&account).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		失败提示管理端(ctx, "渠道合伙人不存在")
		return
	}
	if err != nil {
		失败提示管理端(ctx, "查询渠道合伙人失败")
		return
	}
	ctx.String(http.StatusOK, 读取代理账号最近两月日志(account.ID))
}

// 删除代理账号只删除代理登录主体，已经生成的点卡和保留期内的流水继续归管理员所有。
// 删除完成后撤销该代理的全部管理端令牌，避免旧页面继续操作。
func 删除代理账号(ctx *gin.Context) {
	var request struct {
		ID int `json:"id"`
	}
	if err := ctx.ShouldBindBodyWith(&request, binding.JSON); err != nil || request.ID <= 0 {
		失败提示管理端(ctx, "渠道合伙人编号不正确")
		return
	}
	parent, ok := 管理员_取账号信息(ctx)
	if !ok {
		失败提示管理端(ctx, "登录状态错误")
		return
	}
	var agentName string
	err := db.Transaction(func(tx *gorm.DB) error {
		var account 代理账号记录
		query := tx.Table(代理账号表名).Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND admin = ?", request.ID, parent.Name).First(&account)
		if errors.Is(query.Error, gorm.ErrRecordNotFound) {
			return fmt.Errorf("渠道合伙人不存在")
		}
		if query.Error != nil {
			return fmt.Errorf("读取渠道合伙人失败")
		}
		agentName = account.Name
		result := tx.Table(代理账号表名).Where("id = ? AND admin = ?", request.ID, parent.Name).Delete(&代理账号记录{})
		if result.Error != nil || result.RowsAffected != 1 {
			return fmt.Errorf("删除渠道合伙人失败")
		}
		// 代理账号删除后不再有主体可以使用其时长卡价格；价格锚点与
		// 账号在同一事务清理，避免留下可被新账号 ID 误用的孤立配置。
		if err := tx.Table(时长卡代理价格表名).Where("admin = ? AND agent_id = ?", parent.Name, request.ID).Delete(&时长卡代理价格{}).Error; err != nil {
			return fmt.Errorf("清理代理时长卡价格失败")
		}
		return nil
	})
	if err != nil {
		失败提示管理端(ctx, err.Error())
		return
	}
	全局_登录会话.删除账号会话(agentName, true)
	日志("log/"+parent.Name+time.Now().Format("200601"), fmt.Sprintf("删除渠道合伙人;ID:%d;账号:%s", request.ID, agentName))
	成功提示管理端(ctx, gin.H{"msg": "删除成功"})
}
