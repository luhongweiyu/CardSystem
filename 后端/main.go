package main

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

//	func GetRandomString2(n int) string {
//		randBytes := make([]byte, n/2)
//		rand.Read(randBytes)
//		return fmt.Sprintf("%x", randBytes)
//	}
func 整点执行_任务() {
	锁_请求防火墙.Lock()
	defer 锁_请求防火墙.Unlock()
	for k := range 全局_用户每小时请求次数 {
		delete(全局_用户每小时请求次数, k)
	}
}
func 初始化() {
	time.Local = time.FixedZone("CST", 8*3600) // 东八

	viper.SetConfigFile("./config.yaml")
	viper.ReadInConfig()
	fmt.Println(viper.GetString("数据库.账号"))
	if !viper.GetBool("dev") {
		gin.SetMode(gin.ReleaseMode)
	}
	日志("log/启动记录.txt", "启动啦")
	go func() {
		for {
			if time.Now().Minute() == 0 {
				整点执行_任务()
				time.Sleep(time.Duration(time.Minute * 58))
			}
			time.Sleep(time.Duration(time.Second * 25))
		}
	}()
	// config = viper.New()
	// config.SetConfigFile("./config.yaml")
	// fmt.Println(config.GetString("abc"))
	// config.Set("123", "456")
	// viper.WriteConfig()
	// fmt.Println(config.GetString("123"))
}
func main() {

	初始化()
	fmt.Println("开始运行:")
	连接数据库()
	fmt.Println("启动网络服务:")
	启动网络()

	// for i := 0; i < 100000; i++ {
	// 	fmt.Println(GetRandomString2(10))

	// }
	// s :=
	// fmt.Println(s)
	// L := sync.Mutex
	// L.Lock()

}

// could not import github.com/gin-gonic/gin (no required module provides package "github.com/gin-gonic/gin")
// go env -w GOOS=linux;go build .
