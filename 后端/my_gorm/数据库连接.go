package mygorm

import (
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// ConnectToTheDatabase 创建 MySQL 连接并配置连接池。
// 调用方负责处理返回错误，禁止在连接失败后继续使用空的数据库对象。
func ConnectToTheDatabase(host string, username string, password string, dbName string, port int) (*gorm.DB, error) {
	timeout := "10s"
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local&timeout=%s", username, password, host, port, dbName, timeout)
	var lastErr error
	for attempt := 1; attempt <= 5; attempt++ {
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
			lastErr = err
			if attempt < 5 {
				fmt.Printf("连接数据库失败（第%d次），3秒后重试: %v\n", attempt, err)
				time.Sleep(3 * time.Second)
			}
			continue
		}

		sqlDB, err := db.DB()
		if err != nil {
			return nil, fmt.Errorf("获取数据库连接池失败: %w", err)
		}
		sqlDB.SetConnMaxLifetime(110 * time.Second)
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetMaxOpenConns(50)
		return db, nil
	}

	return nil, fmt.Errorf("数据库连接失败，已重试5次: %w", lastErr)
}
