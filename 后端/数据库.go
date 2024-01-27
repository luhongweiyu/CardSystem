package main

import (
	"fmt"
	my_gorm "hicard/my_gorm"
	"time"

	"github.com/spf13/viper"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	卡密状态_未激活 = 1
	卡密状态_正常  = 2
	卡密状态_到期  = 3
	卡密状态_冻结  = 4
)

type 卡密表样式 struct {
	Card           string `gorm:"size:63;primaryKey;autoIncrement:false"`
	Create_time    time.Time
	Use_time       time.Time
	End_time       time.Time
	Software       int
	Card_state     int `gorm:"size:2"`
	Available_time float64
	// Add_time       int
	Needle                 string `gorm:"size:6"`
	Notes                  string
	Config_content         string
	Latest_activation_time float64
}
type 数据库表_充值卡 struct {
	Card            string `gorm:"primaryKey"`
	Software        int
	AddTime         float64
	Face_value      int
	Balance         int
	Expiration_date time.Time
	Record          string
	Admin           string
	Create_time     time.Time
}

var db *gorm.DB
var db_user *gorm.DB
var db_user_info *gorm.DB
var db_software *gorm.DB
var db_user_recharge *gorm.DB

func 连接数据库() {
	username := viper.GetString("数据库.username")
	password := viper.GetString("数据库.password")
	Dbname := viper.GetString("数据库.Dbname")
	fmt.Println(username, password, Dbname)

	// username := "cardcard"
	// password := "cardcard"
	// Dbname := "gocard"

	host := "localhost"
	port := 3306
	// // 创建一个软件的表
	db = my_gorm.ConnectToTheDatabase(host, username, password, Dbname, port)
	if !viper.GetBool("dev") {
		db.Logger = logger.Default.LogMode(logger.Error)
	}
	// 卡密数据库
	// db_card = db.Table("123").Session(&gorm.Session{})
	// db_card.AutoMigrate(&卡密表样式{})
	// db_card = db.Table("card").Session(&gorm.Session{})
	// 用户数据库
	db_user = db.Table("user").Session(&gorm.Session{})
	db_user.AutoMigrate(&user{})
	// 用户信息数据库
	db_user_info = db.Table("user_info").Session(&gorm.Session{})
	db_user_info.AutoMigrate(&user_info{})
	// 软件数据库
	db_software = db.Table("software").Session(&gorm.Session{})
	db_software.AutoMigrate(&software{})
	// 充值卡数据库
	db_user_recharge = db.Table("visitor_recharge").Session(&gorm.Session{})
	db_user_recharge.AutoMigrate(&数据库表_充值卡{})

	// 用户卡密数据表
	var a []struct {
		Name string
	}
	db_user.Find(&a)

	for _, v := range a {
		db.Table("card_" + v.Name).AutoMigrate(&卡密表样式{})
		user_刷新用户设置(v.Name)
	}

	fmt.Println(a)

	fmt.Println("-----------------------------")
	// db_card.Save(gin.H{"card": "123456"})
	// db_card.Where("card = ?", "789").Updates(gin.H{"备注信息": "nil555"})
	// db_card.Where("card = ?", "999").Updates(卡密表样式{Card: "6666", O备注信息: "7777"})
	// db_card.Save(&卡密表样式{Card: "123456"})

	// db_card.Create(map[string]interface{}{"Card": "789", "备注信息": "nil"})
	// db_card.Create(&卡密表样式{Card: "666", O备注信息: "777"})
	fmt.Println("-----------------------------")

}
