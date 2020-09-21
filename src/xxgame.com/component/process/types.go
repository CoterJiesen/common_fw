/*
process相关类型定义
*/

package process

import (
	"net/http"
)

//http请求触发时通过管道传递的上下文
type HttpContext struct {
	Res http.ResponseWriter //回复
	Req *http.Request       //请求
}

//进程命令接口
type ProcessCmd interface {
	SignalReload()                  //响应信号重载命令
	SignalQuit()                    //响应信号退出命令
	GetHttpChan() chan *HttpContext //获取http命令的管道
}
