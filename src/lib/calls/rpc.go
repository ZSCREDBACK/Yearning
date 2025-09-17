package calls

import (
	"Yearning-go/src/model"
	"log"
	"net/rpc"
)

func NewRpc() *rpc.Client {
	client, err := rpc.DialHTTP("tcp", model.C.General.RpcAddr)
	if err != nil {
		model.DefaultLogger.Debugf("当前juno地址为: %v\n", model.C.General.RpcAddr)
		log.Println(err)
	}
	return client
}
