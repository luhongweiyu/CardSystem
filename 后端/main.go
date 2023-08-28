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
	go func() {
		i := 0
		for i < 5 {
			if time.Now().Minute() == 0 {
				整点执行_任务()
				time.Sleep(time.Minute * 58)
			}
			time.Sleep(time.Second * 10)
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
	// type b struct {
	// 	dd string
	// }
	// a := make(map[string]b)
	// fmt.Println("---------------")
	// fmt.Println(a["abc"].dd == "")

	// d := a["abc"]
	// d.dd = "13456"
	// fmt.Println(a)
	// a["abc"] = d
	// fmt.Println(a)
	// fmt.Println("---------------")

	// a := gin.H{}
	// b, _ := a["code"].(string)
	// b = b + "**"
	// fmt.Println(b)
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

}

// could not import github.com/gin-gonic/gin (no required module provides package "github.com/gin-gonic/gin")
// go env -w GOOS=linux;go build .
