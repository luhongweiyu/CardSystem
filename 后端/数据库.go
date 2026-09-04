package main

import (
	"fmt"
	mygorm "hicard/my_gorm"
	"time"

	"github.com/spf13/viper"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 卡密状态只描述卡密是否允许继续使用。纯点卡没有“激活”和“到期”状态，
// 余额用尽时仍保留正常状态，下一次请求会得到明确的余额不足提示。
const (
	卡密状态_正常 = 2
	卡密状态_冻结 = 4
)

// 卡密表样式是每个管理员独立的 card_<管理员名> 表中的记录。
//
// 这张表只保存卡密本身的长期属性和当前点数余额。设备授权状态放在
// 点卡设备会话表，扣点历史放在点数流水表，避免一张卡上混入多种状态。
// 字段名保留了项目原有的数据库列名，新增的代理账号字段使用清晰的
// agent_id 列。
type 卡密表样式 struct {
	Card        string     `gorm:"column:card;size:63;primaryKey;autoIncrement:false" json:"card"`
	Create_time time.Time  `gorm:"column:create_time;not null;index" json:"create_time"`
	Use_time    *time.Time `gorm:"column:use_time" json:"use_time"`
	Software    int        `gorm:"column:software;not null;index" json:"software"`
	Card_state  int        `gorm:"column:card_state;not null;default:2;index" json:"card_state"`
	// 余额会在每次扣点时更新，但没有按余额筛选的业务查询，因此不建索引，
	// 避免高频扣点无意义地维护一棵额外索引。
	Point_balance  int64  `gorm:"column:point_balance;not null;default:0" json:"point_balance"`
	Notes          string `gorm:"column:notes;size:500" json:"notes"`
	Config_content string `gorm:"column:config_content;type:longtext" json:"config_content"`
	AgentID        int    `gorm:"column:agent_id;not null;default:0;index" json:"agent_id"`
}

// software 保存客户端默认计费周期和心跳判定所需的周期。
// 周期价格仍按软件单独保存在点卡周期价格表中。
type software struct {
	ID                       int       `gorm:"column:id;primaryKey;autoIncrement" json:"ID"`
	Name                     string    `gorm:"column:name;size:32;not null;uniqueIndex:uk_software_owner,priority:1" json:"Name"`
	Software                 string    `gorm:"column:software;size:64;not null;uniqueIndex:uk_software_owner,priority:2" json:"Software"`
	CreatedAt                time.Time `gorm:"column:created_at" json:"CreatedAt"`
	Bulletin                 string    `gorm:"column:bulletin;type:text" json:"Bulletin"`
	DefaultPeriodSeconds     int64     `gorm:"column:default_period_seconds;not null;default:3600" json:"default_period_seconds"`
	HeartbeatIntervalSeconds int64     `gorm:"column:heartbeat_interval_seconds;not null;default:300" json:"heartbeat_interval_seconds"`
}

// 软件是 software 的中文别名，业务代码使用中文类型名，数据库表名保持稳定。
type 软件 = software

var db *gorm.DB
var db_user *gorm.DB
var db_代理账号 *gorm.DB
var db_user_info *gorm.DB
var db_software *gorm.DB
var db_point_period_price *gorm.DB
var db_point_ledger *gorm.DB
var db_point_device_session *gorm.DB

// 连接数据库建立连接、初始化公共表，并为每个管理员创建自己的卡密表。
// 物理分表是当前租户隔离方案；将来如果规模增长，可以在此处把表名解析
// 层替换为统一表或分库，而不会改变点卡计费服务的接口。
func 连接数据库() error {
	username := viper.GetString("数据库.username")
	password := viper.GetString("数据库.password")
	dbName := viper.GetString("数据库.dbname")
	if username == "" || dbName == "" {
		return fmt.Errorf("数据库.username 和 数据库.dbname 不能为空")
	}
	host := viper.GetString("数据库.host")
	if host == "" {
		host = "localhost"
	}
	port := viper.GetInt("数据库.port")
	if port == 0 {
		port = 3306
	}

	var err error
	db, err = mygorm.ConnectToTheDatabase(host, username, password, dbName, port)
	if err != nil {
		return err
	}
	if !viper.GetBool("dev") {
		db.Logger = logger.Default.LogMode(logger.Error)
	}

	db_user = db.Table("user").Session(&gorm.Session{})
	if err := db_user.AutoMigrate(&user{}); err != nil {
		return fmt.Errorf("初始化或更新管理员表结构失败: %w", err)
	}
	db_代理账号 = db.Table(代理账号表名).Session(&gorm.Session{})
	if err := db_代理账号.AutoMigrate(&代理账号记录{}); err != nil {
		return fmt.Errorf("初始化或更新代理账号表结构失败: %w", err)
	}
	db_user_info = db.Table("user_info").Session(&gorm.Session{})
	if err := db_user_info.AutoMigrate(&user_info{}); err != nil {
		return fmt.Errorf("初始化或更新管理员设置表结构失败: %w", err)
	}
	db_software = db.Table("software").Session(&gorm.Session{})
	if err := db_software.AutoMigrate(&software{}); err != nil {
		return fmt.Errorf("初始化或更新软件表结构失败: %w", err)
	}
	db_point_period_price = db.Table("point_period_price").Session(&gorm.Session{})
	if err := db_point_period_price.AutoMigrate(&点卡周期价格{}); err != nil {
		return fmt.Errorf("初始化或更新点卡周期价格表结构失败: %w", err)
	}
	db_point_ledger = db.Table("point_ledger").Session(&gorm.Session{})
	if err := db_point_ledger.AutoMigrate(&点数流水{}); err != nil {
		return fmt.Errorf("初始化或更新点数流水表结构失败: %w", err)
	}
	db_point_device_session = db.Table("point_device_session").Session(&gorm.Session{})
	if err := db_point_device_session.AutoMigrate(&点卡设备会话{}); err != nil {
		return fmt.Errorf("初始化或更新点卡设备会话表结构失败: %w", err)
	}

	var administrators []struct{ Name string }
	if err := db_user.Select("name").Find(&administrators).Error; err != nil {
		return fmt.Errorf("读取管理员列表失败: %w", err)
	}
	for _, administrator := range administrators {
		tableName, tableErr := 卡密数据表名(administrator.Name)
		if tableErr != nil {
			// 异常账号名无法安全拼接表名；不阻塞其他合法账号启动。
			日志("log/启动记录.txt", fmt.Sprintf("跳过非法管理员卡密表:%q", administrator.Name))
			continue
		}
		if err := db.Table(tableName).AutoMigrate(&卡密表样式{}); err != nil {
			return fmt.Errorf("初始化或更新管理员 %s 的卡密表结构失败: %w", administrator.Name, err)
		}
		if err := user_刷新用户设置(administrator.Name); err != nil {
			return fmt.Errorf("加载管理员 %s 的运行设置失败: %w", administrator.Name, err)
		}
	}
	return nil
}
