package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func 整点执行_任务() {
	now := time.Now()
	全局_运行状态.清空请求次数()
	全局_登录会话.清理过期会话(now)
}

// 启动点卡会话清理每分钟处理一批到期设备。清理任务只负责结算已经
// 到期的会话，真正的扣点仍由点卡服务事务完成，和登录/心跳走同一套规则。
func 启动点卡会话清理() {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			清理过期点卡复用候选缓存()
			清理点卡设备会话()
		}
	}()
}

// 启动整点任务使用一次性定时器对齐下一个整点，避免轮询造成重复执行或时间漂移。
func 启动整点任务() {
	go func() {
		for {
			now := time.Now()
			nextHour := now.Truncate(time.Hour).Add(time.Hour)
			timer := time.NewTimer(time.Until(nextHour))
			<-timer.C
			整点执行_任务()
		}
	}()
}

func 初始化() error {
	time.Local = time.FixedZone("CST", 8*3600) // 东八

	viper.SetConfigFile("./config.yaml")
	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}
	初始化心跳缓存同步间隔()
	if !viper.GetBool("dev") {
		gin.SetMode(gin.ReleaseMode)
	}
	if err := os.MkdirAll("log", 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}
	日志("log/启动记录.txt", "启动啦")
	return nil
}
func main() {
	if err := 初始化(); err != nil {
		log.Fatal(err)
	}
	// 迁移函数内部还会检查配置开关；只有明确设置迁移: true，且旧表存在、
	// 目标表不存在时，才会写入并改名。
	if err := 迁移旧时长卡数据(); err != nil {
		log.Fatal(err)
	}
	fmt.Println("开始运行:")
	if err := 连接数据库(); err != nil {
		log.Fatal(err)
	}
	启动整点任务()
	启动点卡会话清理()
	启动点数流水清理()
	fmt.Println("启动网络服务:")
	if err := 启动网络(); err != nil {
		log.Fatal(err)
	}
}
