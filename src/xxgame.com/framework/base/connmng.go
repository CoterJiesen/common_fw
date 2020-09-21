/*
连接管理器：routine安全
*/

package base

import (
	"sync"
)

import (
	_ "time"
	"xxgame.com/component/log"
	"xxgame.com/component/net"
	"xxgame.com/framework"
	"xxgame.com/utils/errors"
)

//连接管理器:实现server.ConnManager接口
type ConnMng struct {
	inited      bool                 //是否已经初始化过
	newConnChan chan *net.Conn       //接收新建连接事件的管道
	disConnChan chan *net.Conn       //接收断开连接事件的管道
	conns       map[uint32]*net.Conn //保存连接的map
	m           sync.RWMutex         //保护map的读写锁
	s           framework.Service    //服务实现
	l           *log.Logger          //日志接口
}

//初始化函数
func (c *ConnMng) Init(s framework.Service, l *log.Logger) error {
	//加读写锁保护
	c.m.Lock()
	defer c.m.Unlock()

	if c.inited {
		return errors.New("already inited")
	}

	if l == nil {
		return errors.New("logger is nil")
	}

	c.newConnChan = make(chan *net.Conn)
	c.disConnChan = make(chan *net.Conn, 100)
	c.conns = make(map[uint32]*net.Conn)
	c.s = s
	c.l = l

	c.inited = true

	go c.Listen()
	return nil
}

//获取连接
func (c *ConnMng) GetConn(sid uint32) *net.Conn {
	//加读锁保护
	c.m.RLock()
	defer c.m.RUnlock()

	if !c.inited || c.conns == nil {
		return nil
	}

	conn, exist := c.conns[sid]
	if conn != nil && exist {
		return conn
	}
	return nil
}

//事件监听
func (c *ConnMng) Listen() {
	if !c.inited {
		panic("ConnMng not inited")
	}

	for {
		select {
		case conn := <-c.newConnChan: //新连接
			c.m.Lock()
			c.conns[conn.GetId()] = conn
			c.m.Unlock()
			c.l.Infof("CONN", "%s connected", conn.String())
		case conn := <-c.disConnChan: //连接断开
			c.m.Lock()
			delete(c.conns, conn.GetId())
			c.m.Unlock()
			c.s.OnNetDisconn(conn)
			c.l.Infof("CONN", "%s disconnected", conn.String())
			/*
				default:
					time.Sleep(5 * time.Millisecond) //休眠5毫秒
			*/
		}
	}
}

//实现server.ConnManager接口:获取新连接对象的管道
func (c *ConnMng) GetNewConnChan() chan *net.Conn {
	return c.newConnChan
}

//实现server.ConnManager接口:获取异常断开的连接对象的管道
func (c *ConnMng) GetDisconnChan() chan *net.Conn {
	return c.disConnChan
}
