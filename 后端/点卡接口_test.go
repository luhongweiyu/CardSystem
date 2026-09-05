package main

import (
	"strings"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// Test点卡周期价格写入值保留禁用状态验证新增计费方案会把 false 作为明确值
// 交给数据库，而不是让 Enabled 字段的 default:true 覆盖业务输入。
func Test点卡周期价格写入值保留禁用状态(t *testing.T) {
	price := 点卡周期价格{
		Admin:         "admin",
		Software:      1,
		PeriodSeconds: 3600,
		Cost:          1,
		Enabled:       false,
	}
	values := 点卡周期价格写入值(price)
	if enabled, ok := values["enabled"].(bool); !ok || enabled {
		t.Fatalf("新增禁用方案必须显式写入 enabled=false，得到 %#v", values["enabled"])
	}

	// DryRun 不连接数据库，只检查 GORM 最终生成的 INSERT 参数，确保 map Create
	// 真的把 false 带入 SQL；这正是结构体 Create 在 default:true 下会丢失的值。
	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN:                       "root:password@tcp(127.0.0.1:3306)/gocard?charset=utf8mb4&parseTime=True&loc=Local",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true, SkipDefaultTransaction: true})
	if err != nil {
		t.Fatalf("初始化 GORM DryRun 失败: %v", err)
	}
	created := db.Model(&点卡周期价格{}).Table("point_period_price").Create(values)
	if created.Error != nil {
		t.Fatalf("GORM DryRun 生成新增方案 SQL 失败: %v", created.Error)
	}
	statement := created.Statement
	if !strings.Contains(strings.ToLower(statement.SQL.String()), "enabled") {
		t.Fatalf("新增方案 SQL 未包含 enabled 字段: %s", statement.SQL.String())
	}
	foundFalse := false
	for _, value := range statement.Vars {
		if enabled, ok := value.(bool); ok && !enabled {
			foundFalse = true
			break
		}
	}
	if !foundFalse {
		t.Fatalf("新增方案 SQL 参数未包含 enabled=false: %#v", statement.Vars)
	}
}
