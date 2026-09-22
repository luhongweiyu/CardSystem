package main

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	mygorm "hicard/my_gorm"

	"github.com/spf13/viper"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	// 旧版 card 表使用这组状态值。新表的未激活和到期状态由时间字段推断，
	// 因此迁移后只需要把它们保存为可操作的正常状态。
	旧卡密状态_未激活 = 1
	旧卡密状态_正常  = 2
	旧卡密状态_到期  = 3
	旧卡密状态_冻结  = 4

	// 单次批量写入限制在 500 行，避免一次迁移占用过多客户端内存和数据库
	// 参数空间；事务仍然覆盖该管理员的完整迁移批次。
	旧时长卡迁移写入批次 = 500
)

// 旧时长卡记录只用于读取旧 card_<管理员名> 表。旧表中的时间字段和
// ID子账号字段名称保留在迁移模型里，正式业务模型不会再次依赖这些旧字段。
type 旧时长卡迁移记录 struct {
	Card                 string    `gorm:"column:card"`
	CreateTime           time.Time `gorm:"column:create_time"`
	UseTime              time.Time `gorm:"column:use_time"`
	EndTime              time.Time `gorm:"column:end_time"`
	Software             int       `gorm:"column:software"`
	CardState            int       `gorm:"column:card_state"`
	AvailableTime        float64   `gorm:"column:available_time"`
	Needle               string    `gorm:"column:needle"`
	Notes                string    `gorm:"column:notes"`
	ConfigContent        string    `gorm:"column:config_content"`
	LatestActivationTime float64   `gorm:"column:latest_activation_time"`
	StorageTime          float64   `gorm:"column:storage_time"`
	LegacyAgentID        int       `gorm:"column:ID子账号"`
}

// 旧代理账号只从 user_son 读取。旧表没有密码哈希，所以新建账号必须用旧
// 明文密码重新生成当前 bcrypt 哈希；该明文只写入新账号资料，不进入日志。
type 旧代理账号迁移记录 struct {
	Name     string `gorm:"column:name"`
	Password string `gorm:"column:password"`
	ID       int    `gorm:"column:ID子账号;primaryKey"`
	Balance  int64  `gorm:"column:余额"`
	Admin    string `gorm:"column:父Name"`
}

type 旧时长卡代理迁移计划 struct {
	旧账号   旧代理账号迁移记录
	已有账号  bool
	新账号ID int
	密码哈希  string
}

type 旧时长卡迁移计划 struct {
	管理员  string
	旧表名  string
	新表名  string
	备份表名 string
	卡密   []旧时长卡迁移记录
	代理   []旧时长卡代理迁移计划
}

// 迁移旧时长卡数据是唯一的迁移入口。服务启动前显式调用它，按管理员
// 独立处理旧 card_<管理员名>，不会读取或改写正式 point_card_<管理员名>。
func 迁移旧时长卡数据() error {
	// 迁移属于一次性、可能修改旧表的操作，必须由部署配置显式开启；
	// 未配置或设置为 false 时，普通服务启动绝不访问旧卡表。
	if !viper.GetBool("原时间卡密数据迁移") {
		fmt.Println("未开启旧时长卡迁移，跳过迁移")
		return nil
	}

	migrationDB, closeDB, err := 打开迁移数据库()
	if err != nil {
		return err
	}
	defer closeDB()

	if !migrationDB.Migrator().HasTable("user") {
		// 新数据库第一次启动时公共表尚未创建，这是正常情况；
		// 直接交给后面的连接数据库流程初始化，不把它当成迁移错误。
		fmt.Println("管理员表 user 不存在，跳过旧时长卡迁移")
		return nil
	}
	// 代理账号表是新服务的公共表。迁移流程只补齐表结构，不迁移任何
	// 旧价格 JSON，避免依赖普通服务启动顺序。
	if err := migrationDB.Table(代理账号表名).AutoMigrate(&代理账号记录{}); err != nil {
		return fmt.Errorf("初始化代理账号表失败: %w", err)
	}

	plans, skipped, err := 预检查旧时长卡迁移(migrationDB)
	if err != nil {
		return err
	}
	for _, admin := range skipped {
		fmt.Printf("跳过管理员 %s：目标时长卡表已存在或旧表不存在\n", admin)
	}
	if len(plans) == 0 {
		fmt.Println("没有符合迁移条件的旧时长卡表")
		return nil
	}

	迁移时间 := time.Now()
	for index := range plans {
		if err := 执行单个管理员旧时长卡迁移(migrationDB, &plans[index], 迁移时间); err != nil {
			return err
		}
	}
	return nil
}

// 打开迁移数据库不调用连接数据库，防止普通启动流程先 AutoMigrate 出
// duration_card_<管理员名>，从而让迁移条件失效。
func 打开迁移数据库() (*gorm.DB, func(), error) {
	username := viper.GetString("数据库.username")
	password := viper.GetString("数据库.password")
	dbName := viper.GetString("数据库.dbname")
	if username == "" || dbName == "" {
		return nil, func() {}, fmt.Errorf("数据库.username 和 数据库.dbname 不能为空")
	}
	host := viper.GetString("数据库.host")
	if host == "" {
		host = "localhost"
	}
	port := viper.GetInt("数据库.port")
	if port == 0 {
		port = 3306
	}

	database, err := mygorm.ConnectToTheDatabase(host, username, password, dbName, port)
	if err != nil {
		return nil, func() {}, err
	}
	if !viper.GetBool("dev") {
		database.Logger = logger.Default.LogMode(logger.Error)
	}
	sqlDB, err := database.DB()
	if err != nil {
		return nil, func() {}, fmt.Errorf("获取迁移数据库连接池失败: %w", err)
	}
	return database, func() { _ = sqlDB.Close() }, nil
}

// 预检查旧时长卡迁移会先读取并校验所有候选管理员，任何一个管理员的
// 数据存在冲突都会在写入前终止，避免只迁移半个数据库后才发现问题。
func 预检查旧时长卡迁移(database *gorm.DB) ([]旧时长卡迁移计划, []string, error) {
	var administrators []struct {
		Name string `gorm:"column:name"`
	}
	if err := database.Table("user").Select("name").Order("name ASC").Find(&administrators).Error; err != nil {
		return nil, nil, fmt.Errorf("读取管理员列表失败: %w", err)
	}

	userSonExists := database.Migrator().HasTable("user_son")
	plans := make([]旧时长卡迁移计划, 0)
	skipped := make([]string, 0)
	globalAgentNames := make(map[string]string)
	for _, administrator := range administrators {
		if !验证管理员名称(administrator.Name) {
			return nil, nil, fmt.Errorf("管理员 %q 名称不符合当前表名规则", administrator.Name)
		}
		oldTable, err := 旧卡密数据表名(administrator.Name)
		if err != nil {
			return nil, nil, err
		}
		newTable, err := 时长卡数据表名(administrator.Name)
		if err != nil {
			return nil, nil, err
		}
		backupTable := oldTable + "_bak"
		oldExists := database.Migrator().HasTable(oldTable)
		newExists := database.Migrator().HasTable(newTable)
		backupExists := database.Migrator().HasTable(backupTable)

		if newExists && oldExists && !backupExists {
			return nil, nil, fmt.Errorf("管理员 %s 同时存在旧表和目标表，可能是上次迁移未完成，请核对后再启动服务", administrator.Name)
		}
		if backupExists && !newExists {
			return nil, nil, fmt.Errorf("管理员 %s 存在备份表 %s 但目标表不存在，迁移状态不完整", administrator.Name, backupTable)
		}
		if newExists || !oldExists {
			skipped = append(skipped, administrator.Name)
			continue
		}
		if backupExists {
			return nil, nil, fmt.Errorf("管理员 %s 的备份表 %s 已存在，拒绝覆盖", administrator.Name, backupTable)
		}

		var cards []旧时长卡迁移记录
		if err := database.Table(oldTable).Order("card ASC").Find(&cards).Error; err != nil {
			return nil, nil, fmt.Errorf("读取管理员 %s 的旧卡密失败: %w", administrator.Name, err)
		}

		legacyAgentIDs := make(map[int]struct{})
		for _, card := range cards {
			if card.LegacyAgentID < 0 {
				return nil, nil, fmt.Errorf("管理员 %s 的卡密 %s 使用了无效代理编号 %d", administrator.Name, card.Card, card.LegacyAgentID)
			}
			if card.LegacyAgentID > 0 {
				legacyAgentIDs[card.LegacyAgentID] = struct{}{}
			}
		}

		legacyAgents := make([]旧代理账号迁移记录, 0)
		if userSonExists {
			if err := database.Table("user_son").Where("父Name = ?", administrator.Name).
				Order("ID子账号 ASC").Find(&legacyAgents).Error; err != nil {
				return nil, nil, fmt.Errorf("读取管理员 %s 的旧代理账号失败: %w", administrator.Name, err)
			}
		} else if len(legacyAgentIDs) > 0 {
			return nil, nil, fmt.Errorf("管理员 %s 的旧卡密引用了代理，但旧代理表 user_son 不存在", administrator.Name)
		}

		plan := 旧时长卡迁移计划{管理员: administrator.Name, 旧表名: oldTable, 新表名: newTable, 备份表名: backupTable, 卡密: cards}
		legacyAgentMap := make(map[int]int, len(legacyAgents))
		for _, legacyAgent := range legacyAgents {
			if legacyAgent.ID <= 0 {
				return nil, nil, fmt.Errorf("管理员 %s 存在无效旧代理编号 %d", administrator.Name, legacyAgent.ID)
			}
			if _, exists := legacyAgentMap[legacyAgent.ID]; exists {
				return nil, nil, fmt.Errorf("管理员 %s 的旧代理编号 %d 重复", administrator.Name, legacyAgent.ID)
			}
			legacyAgentMap[legacyAgent.ID] = legacyAgent.ID
			if !验证管理员名称(legacyAgent.Name) {
				return nil, nil, fmt.Errorf("管理员 %s 的旧代理名称 %q 不符合当前规则", administrator.Name, legacyAgent.Name)
			}
			normalizedAgentName := strings.ToLower(legacyAgent.Name)
			if previousAdmin, exists := globalAgentNames[normalizedAgentName]; exists {
				return nil, nil, fmt.Errorf("代理账号名称 %q 在管理员 %s 和 %s 的旧数据中重复，无法映射到全局唯一账号", legacyAgent.Name, previousAdmin, administrator.Name)
			}
			globalAgentNames[normalizedAgentName] = administrator.Name

			var existing 代理账号记录
			query := database.Table(代理账号表名).Where("name = ?", legacyAgent.Name).First(&existing)
			agentPlan := 旧时长卡代理迁移计划{旧账号: legacyAgent}
			switch {
			case query.Error == nil:
				if existing.Admin != administrator.Name {
					return nil, nil, fmt.Errorf("代理账号 %s 已属于管理员 %s，不能迁移到管理员 %s", legacyAgent.Name, existing.Admin, administrator.Name)
				}
				// 已存在账号不覆盖现有密码、哈希、余额和价格，避免迁移
				// 覆盖管理员后来手动设置的账号资料。
				agentPlan.已有账号 = true
				agentPlan.新账号ID = existing.ID
			case errors.Is(query.Error, gorm.ErrRecordNotFound):
				if err := 校验新密码(legacyAgent.Password); err != nil {
					return nil, nil, fmt.Errorf("管理员 %s 的代理 %s 密码不符合当前规则: %w", administrator.Name, legacyAgent.Name, err)
				}
				hash, hashErr := 生成密码哈希(legacyAgent.Password)
				if hashErr != nil {
					return nil, nil, fmt.Errorf("生成代理 %s 密码哈希失败: %w", legacyAgent.Name, hashErr)
				}
				agentPlan.密码哈希 = hash
			default:
				return nil, nil, fmt.Errorf("查询代理账号 %s 失败: %w", legacyAgent.Name, query.Error)
			}
			plan.代理 = append(plan.代理, agentPlan)
		}

		for legacyAgentID := range legacyAgentIDs {
			if _, exists := legacyAgentMap[legacyAgentID]; !exists {
				return nil, nil, fmt.Errorf("管理员 %s 的卡密引用了不存在的旧代理编号 %d", administrator.Name, legacyAgentID)
			}
		}
		for _, card := range cards {
			if _, err := 转换旧时长卡记录(card, time.Now(), legacyAgentMap); err != nil {
				return nil, nil, fmt.Errorf("管理员 %s 的卡密 %s 校验失败: %w", administrator.Name, card.Card, err)
			}
		}
		plans = append(plans, plan)
	}
	return plans, skipped, nil
}

// 执行单个管理员旧时长卡迁移只在目标表首次创建时写入。账号创建和卡密
// 写入在同一事务中；旧表重命名放在事务成功之后，避免失败时破坏迁移来源。
func 执行单个管理员旧时长卡迁移(database *gorm.DB, plan *旧时长卡迁移计划, migrationTime time.Time) error {
	targetWasAbsent := !database.Migrator().HasTable(plan.新表名)
	if !targetWasAbsent {
		return fmt.Errorf("管理员 %s 的目标表 %s 在执行期间已存在，停止迁移", plan.管理员, plan.新表名)
	}
	if !database.Migrator().HasTable(plan.旧表名) {
		return fmt.Errorf("管理员 %s 的旧表 %s 在执行期间不存在，停止迁移", plan.管理员, plan.旧表名)
	}
	if database.Migrator().HasTable(plan.备份表名) {
		return fmt.Errorf("管理员 %s 的备份表 %s 在执行期间已存在，停止迁移", plan.管理员, plan.备份表名)
	}

	if err := database.Table(plan.新表名).AutoMigrate(&时长卡表样式{}); err != nil {
		if targetWasAbsent && database.Migrator().HasTable(plan.新表名) {
			// AutoMigrate 失败时数据库可能已经创建了部分目标表；
			// 清掉本次新建的表，避免下次启动误认为迁移已完成。
			if dropErr := database.Migrator().DropTable(plan.新表名); dropErr != nil {
				return fmt.Errorf("创建管理员 %s 的目标时长卡表失败: %v；清理目标表失败: %w", plan.管理员, err, dropErr)
			}
		}
		return fmt.Errorf("创建管理员 %s 的目标时长卡表失败: %w", plan.管理员, err)
	}
	createdTarget := targetWasAbsent
	createdAgentIDs := make([]int, 0, len(plan.代理))
	legacyAgentToNew := make(map[int]int, len(plan.代理))
	transactionErr := database.Transaction(func(tx *gorm.DB) error {
		for index := range plan.代理 {
			agent := &plan.代理[index]
			if !agent.已有账号 {
				account := 代理账号记录{
					Admin:        plan.管理员,
					Name:         agent.旧账号.Name,
					Password:     agent.旧账号.Password,
					PasswordHash: agent.密码哈希,
					Balance:      agent.旧账号.Balance,
					Prices:       "{}",
				}
				if err := tx.Table(代理账号表名).Create(&account).Error; err != nil {
					return fmt.Errorf("创建代理账号 %s 失败: %w", agent.旧账号.Name, err)
				}
				agent.新账号ID = account.ID
				createdAgentIDs = append(createdAgentIDs, account.ID)
			}
			legacyAgentToNew[agent.旧账号.ID] = agent.新账号ID
		}

		converted := make([]时长卡表样式, 0, len(plan.卡密))
		for _, oldCard := range plan.卡密 {
			card, err := 转换旧时长卡记录(oldCard, migrationTime, legacyAgentToNew)
			if err != nil {
				return fmt.Errorf("转换卡密 %s 失败: %w", oldCard.Card, err)
			}
			converted = append(converted, card)
		}
		if len(converted) > 0 {
			if err := tx.Table(plan.新表名).CreateInBatches(&converted, 旧时长卡迁移写入批次).Error; err != nil {
				return fmt.Errorf("写入管理员 %s 的时长卡失败: %w", plan.管理员, err)
			}
		}
		var count int64
		if err := tx.Table(plan.新表名).Count(&count).Error; err != nil {
			return fmt.Errorf("校验管理员 %s 的迁移数量失败: %w", plan.管理员, err)
		}
		if count != int64(len(converted)) {
			return fmt.Errorf("管理员 %s 的迁移数量不一致，期望%d条，实际%d条", plan.管理员, len(converted), count)
		}
		return nil
	})
	if transactionErr != nil {
		if createdTarget {
			if dropErr := database.Migrator().DropTable(plan.新表名); dropErr != nil {
				return fmt.Errorf("管理员 %s 迁移失败: %v；清理新表失败: %w", plan.管理员, transactionErr, dropErr)
			}
		}
		return fmt.Errorf("管理员 %s 迁移失败: %w", plan.管理员, transactionErr)
	}

	if err := database.Migrator().RenameTable(plan.旧表名, plan.备份表名); err != nil {
		// RENAME TABLE 失败时，来源表仍应保持原名。删除本次新建的
		// 目标表并回收本次创建的代理账号，保证下次启动可以安全重试；
		// 已存在的代理账号不会被删除。
		if cleanupErr := 回滚已提交旧时长卡迁移(database, plan, createdAgentIDs, createdTarget); cleanupErr != nil {
			return fmt.Errorf("管理员 %s 旧表重命名失败: %v；回滚迁移结果失败，请勿启动服务并人工处理: %w", plan.管理员, err, cleanupErr)
		}
		return fmt.Errorf("管理员 %s 旧表重命名失败，迁移结果已回滚: %w", plan.管理员, err)
	}
	if database.Migrator().HasTable(plan.旧表名) || !database.Migrator().HasTable(plan.备份表名) {
		return fmt.Errorf("管理员 %s 旧表重命名结果校验失败，请勿启动服务", plan.管理员)
	}
	fmt.Printf("管理员 %s 迁移完成：时长卡 %d 条，新建代理 %d 个，旧表已备份为 %s\n", plan.管理员, len(plan.卡密), len(createdAgentIDs), plan.备份表名)
	return nil
}

// 回滚已提交旧时长卡迁移只清理本次新建的目标表和代理账号。旧来源表
// 没有被删除或更新，因此清理成功后下次启动可以重新迁移。
func 回滚已提交旧时长卡迁移(database *gorm.DB, plan *旧时长卡迁移计划, createdAgentIDs []int, createdTarget bool) error {
	if createdTarget {
		if err := database.Migrator().DropTable(plan.新表名); err != nil {
			return fmt.Errorf("删除目标表失败: %w", err)
		}
	}
	if len(createdAgentIDs) == 0 {
		return nil
	}
	result := database.Table(代理账号表名).Where("admin = ? AND id IN ?", plan.管理员, createdAgentIDs).Delete(&代理账号记录{})
	if result.Error != nil {
		return fmt.Errorf("删除本次创建的代理账号失败: %w", result.Error)
	}
	if result.RowsAffected != int64(len(createdAgentIDs)) {
		return fmt.Errorf("删除本次创建的代理账号数量不一致，期望%d条，实际%d条", len(createdAgentIDs), result.RowsAffected)
	}
	return nil
}

// 转换旧时长卡记录负责把旧版“天数”和旧状态转换成新时长卡模型。
// latest_activation_time 仍以旧 create_time 计算绝对截止时间，但新表的
// create_time 由调用方传入迁移时间；两者不能混用。
func 转换旧时长卡记录(oldCard 旧时长卡迁移记录, migrationTime time.Time, agentIDs map[int]int) (时长卡表样式, error) {
	if !卡密格式规则.MatchString(oldCard.Card) {
		return 时长卡表样式{}, fmt.Errorf("卡密格式不符合当前规则")
	}
	durationMinutes, usedAsRechargeSource, err := 旧时长天数转分钟(oldCard.AvailableTime, "available_time", true, true)
	if err != nil {
		return 时长卡表样式{}, err
	}
	if durationMinutes != 0 && (durationMinutes < 时长卡最小时长分钟 || durationMinutes > 时长卡永久分钟) {
		return 时长卡表样式{}, fmt.Errorf("available_time 换算后的时长必须在%d分钟至%d天之间", 时长卡最小时长分钟, 时长卡永久分钟/1440)
	}
	if oldCard.StorageTime < 0 || math.IsNaN(oldCard.StorageTime) || math.IsInf(oldCard.StorageTime, 0) {
		return 时长卡表样式{}, fmt.Errorf("storage_time 不正确")
	}
	pausedMinutes := int64(0)
	if oldCard.StorageTime > 0 {
		pausedMinutes, _, err = 旧时长天数转分钟(oldCard.StorageTime, "storage_time", false, false)
		if err != nil {
			return 时长卡表样式{}, err
		}
	}
	// 旧版 -1 和 0 都表示不保存最晚激活截止时间；旧页面默认传 -1。
	// 只有小于 -1 的值才属于无法解释的旧数据。
	if oldCard.LatestActivationTime < -1 || math.IsNaN(oldCard.LatestActivationTime) || math.IsInf(oldCard.LatestActivationTime, 0) {
		return 时长卡表样式{}, fmt.Errorf("latest_activation_time 不正确")
	}
	var latestActivationAt *time.Time
	if oldCard.LatestActivationTime > 0 {
		if oldCard.CreateTime.IsZero() {
			return 时长卡表样式{}, fmt.Errorf("latest_activation_time 有值但 create_time 为空")
		}
		minutes, _, conversionErr := 旧时长天数转分钟(oldCard.LatestActivationTime, "latest_activation_time", false, false)
		if conversionErr != nil {
			return 时长卡表样式{}, conversionErr
		}
		deadline := oldCard.CreateTime.Add(time.Duration(minutes) * time.Minute)
		latestActivationAt = &deadline
	}

	newAgentID := 0
	if oldCard.LegacyAgentID > 0 {
		if agentIDs == nil {
			return 时长卡表样式{}, fmt.Errorf("卡密引用了代理编号 %d，但没有代理映射", oldCard.LegacyAgentID)
		}
		var mapped bool
		newAgentID, mapped = agentIDs[oldCard.LegacyAgentID]
		if !mapped || newAgentID <= 0 {
			return 时长卡表样式{}, fmt.Errorf("找不到旧代理编号 %d 的新账号映射", oldCard.LegacyAgentID)
		}
	}

	cardState := 卡密状态_正常
	if usedAsRechargeSource {
		// 这是旧 card 表里的普通时长卡曾被用作充值来源的历史行，
		// 不是 visitor_recharge 独立充值卡；独立充值卡本次不迁移。
		if !oldCard.UseTime.IsZero() || !oldCard.EndTime.IsZero() || pausedMinutes > 0 {
			return 时长卡表样式{}, fmt.Errorf("已用作充值来源的卡密同时存在激活、到期或暂停数据")
		}
		cardState = 时长卡状态_已充值
	} else if pausedMinutes > 0 {
		if oldCard.CardState == 旧卡密状态_冻结 {
			return 时长卡表样式{}, fmt.Errorf("冻结卡密同时存在暂停余额，状态冲突")
		}
		cardState = 时长卡状态_暂停
	} else {
		switch oldCard.CardState {
		case 旧卡密状态_未激活, 旧卡密状态_正常, 旧卡密状态_到期:
			// 新系统通过 end_time 是否为空或是否已过期推断这两种展示状态。
			cardState = 卡密状态_正常
		case 旧卡密状态_冻结:
			cardState = 卡密状态_冻结
		default:
			return 时长卡表样式{}, fmt.Errorf("未知旧卡密状态 %d", oldCard.CardState)
		}
	}

	return 时长卡表样式{
		Card:                   oldCard.Card,
		CreateTime:             migrationTime,
		UseTime:                迁移时间指针(oldCard.UseTime),
		EndTime:                迁移时长卡到期时间(oldCard.EndTime, pausedMinutes > 0),
		Software:               oldCard.Software,
		CardState:              cardState,
		DurationMinutes:        durationMinutes,
		LatestActivationAt:     latestActivationAt,
		Needle:                 "",
		LastHeartbeatAt:        nil,
		Notes:                  oldCard.Notes,
		ConfigContent:          oldCard.ConfigContent,
		PausedRemainingMinutes: pausedMinutes,
		AgentID:                newAgentID,
	}, nil
}

func 迁移时间指针(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copyValue := value
	return &copyValue
}

func 迁移时长卡到期时间(value time.Time, paused bool) *time.Time {
	if paused || value.IsZero() {
		return nil
	}
	copyValue := value
	return &copyValue
}

// 旧时长天数转分钟沿用旧实现的截断规则，而不是四舍五入，避免迁移后
// 授权时长或最晚激活时间被无意增加。允许负数时仅用于识别旧充值来源卡；
// 只有 available_time 允许以 0 迁移，其他字段仍要求非零。
func 旧时长天数转分钟(days float64, field string, allowNegative, allowZero bool) (int64, bool, error) {
	if math.IsNaN(days) || math.IsInf(days, 0) || (days == 0 && !allowZero) {
		return 0, false, fmt.Errorf("%s 必须是非零有限数", field)
	}
	if days == 0 {
		return 0, false, nil
	}
	usedAsRechargeSource := days < 0
	if usedAsRechargeSource && !allowNegative {
		return 0, false, fmt.Errorf("%s 不能为负数", field)
	}
	minutesFloat := math.Trunc(math.Abs(days) * 24 * 60)
	maxInt64 := float64(int64(^uint64(0) >> 1))
	if minutesFloat < 1 || minutesFloat > maxInt64 {
		return 0, false, fmt.Errorf("%s 换算后的分钟数超出范围", field)
	}
	return int64(minutesFloat), usedAsRechargeSource, nil
}
