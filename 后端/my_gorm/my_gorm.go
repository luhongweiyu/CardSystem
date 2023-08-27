package mygorm

import (
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// gorm.Model 定义
// type Model struct {
// 	ID        uint `gorm:"primary_key"`
// 	CreatedAt time.Time
// 	UpdatedAt time.Time
// 	DeletedAt *time.Time
// }

func ConnectToTheDatabase(host string, username string, password string, Dbname string, port int) *gorm.DB {
	// username := "cardcard"
	// password := "cardcard"
	// host := "localhost"
	// port := 3306
	// Dbname := "gocard"
	timeout := "10s"
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local&timeout=%s", username, password, host, port, Dbname, timeout)
	var db2 *gorm.DB
	for count := 1; count < 10; {
		count += count
		db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
			SkipDefaultTransaction: true,
			NamingStrategy: schema.NamingStrategy{
				TablePrefix:   "",    // 表名前缀
				SingularTable: true,  // 单数表名
				NoLowerCase:   false, // 关闭小写转换
			},
			Logger: logger.Default.LogMode(logger.Info),
		})
		if err != nil {
			// panic()
			fmt.Println("连接数据库失败,15秒后重试,error:+" + err.Error())
			time.Sleep(time.Second * 15)
		} else {
			db2 = db
			break
		}
	}
	// 连接成功
	fmt.Println("数据库连接成功", db2)
	// db = db2
	a, _ := db2.DB()
	a.SetConnMaxLifetime(110 * time.Second)
	return db2
	// 创建一个软件的表
	// db2.Table("123").AutoMigrate(&卡密表样式{})
	// time.Sleep(time.Second)
	// 插入记录("1223", "6666")
}
