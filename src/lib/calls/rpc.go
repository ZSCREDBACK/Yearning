package calls

import (
	"Yearning-go/src/model"
	"github.com/spf13/viper"
	"log"
	"net/rpc"
)

var GeneralConfig General

type General struct {
	SecretKey string
	RpcAddr   string
	LogLevel  string
	Lang      string
}

func LoadConfig() {
	viper.SetConfigName("conf")      // 文件名 (不带扩展名)
	viper.SetConfigType("toml")      // 文件类型
	viper.AddConfigPath(".")         // 检索当前目录
	viper.AddConfigPath("../")       // 检索上一级目录
	viper.AddConfigPath("../../")    // 检索上一级目录
	viper.AddConfigPath("../../../") // 检索上一级目录

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}

	// 映射到结构体
	if err := viper.UnmarshalKey("General", &GeneralConfig); err != nil {
		log.Fatalf("Unable to decode into struct: %v", err)
	}
}

// 总是获取不到rpc地址，这边使用viper重新写一下
//func NewRpc() *rpc.Client {
//	client, err := rpc.DialHTTP("tcp", model.C.General.RpcAddr)
//	if err != nil {
//		model.DefaultLogger.Debugf("当前juno地址为: %v\n", model.C.General.RpcAddr)
//		log.Println(err)
//	}
//	return client
//}

func NewRpc() *rpc.Client {
	LoadConfig()

	client, err := rpc.DialHTTP("tcp", GeneralConfig.RpcAddr)
	if err != nil {
		model.DefaultLogger.Debugf("当前juno地址为: %v\n", GeneralConfig.RpcAddr)
		log.Println(err)
	}
	return client
}
