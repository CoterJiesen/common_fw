/*
测试基础服务框架
*/

package base

import (
	"testing"
	"xxgame.com/framework"
)

type XService struct {
	BaseService
}

func TestAll(t *testing.T) {

	//获取框架实例
	fw := framework.Instance()
	if fw == nil {
		t.Fatalf("get fw pointer failed")
	}

	//创建服务实例
	service := new(XService)
	//注册服务
	fw.SetService(service)
	//启动框架
	fw.Run()
}
