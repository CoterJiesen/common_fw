// 网络组件相关接口定义
package server

import (
	"xxgame.com/component/net"
)

// 连接管理器
type ConnManager interface {
	GetNewConnChan() chan *net.Conn // 获取新连接对象的管道
	GetDisconnChan() chan *net.Conn // 获取异常断开的连接对象的管道
}
