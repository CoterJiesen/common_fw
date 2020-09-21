// 软件包net提供通用的网络通信功能，可用于创建网络服务器和客户端，主要特性如下：
//	1.监听多个端口，建立TCP/UDP(v6)连接并分配协程处理各个连接的消息收发请求；
//	2.与指定服务器建立TCP/UDP(v6)连接进行消息发送和接收；
//
// NOTE: 由于没有较好的方法获取客户端的地址，udp服务器暂不支持向客户端回复消息
package net

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

import (
	"xxgame.com/component/log"
	"xxgame.com/component/safechan"
	"xxgame.com/component/uuid"
	"xxgame.com/types/proto"
	"xxgame.com/utils/errors"
)

const (
	SocketBufSize = 64 << 10 // 消息缓冲区64K
	HeadSize      = 8        // 消息头占用字节数 size(4) + magic_number(2) + msg_type(2)
	MagicNumber   = 0x786d   // 协议头魔数"xm"
)

// 网络通信类型
type Commu struct {
	isEnabled   bool           // 标明是否启动网络通信
	isServer    bool           // 标明是否是服务端
	isTCP       bool           // 标明是否是tcp通信方式
	connType    string         // 网络连接类型(tcp/tcp4/tcp6/udp/udp4/udp6)
	tcpAddrs    []*net.TCPAddr // tcp服务地址
	udpAddrs    []*net.UDPAddr // udp服务地址
	inMsgLimit  int            // 接收消息管道容量上限
	outMsgLimit int            // 发送消息管道容量上限
	closeDone   chan int       // 用于通知调用模块所有网络连接已断开
	listenDone  chan int       // 用于通知调用模块所有端口监听已结束
	wgListeners sync.WaitGroup // 用于同步listen协程的退出
	wgSendConns sync.WaitGroup // 记录同步连接send协程的退出
	wgRecvConns sync.WaitGroup // 记录同步连接recv协程的退出
}

type NetMsg struct {
	MsgType uint16 // 消息类型
	Data    []byte // 消息体数据
}

type CloseReason int

const (
	NONE        CloseReason = iota // 未关闭
	BY_SELF                        // 本端主动关闭
	BY_OTHER                       // 被对端关闭
	BY_ACCIDENT                    // 异常关闭
)

var (
	maxSilentTime uint64       = 60 * 3 // 连接静默的最长时间，3分钟
	uuidGen       uuid.UuidGen          //消息序列号生成器
)

// 建立的网络通信连接(tcp/udp)
type Conn struct {
	commu               *Commu            // 网络通信类型
	isClosed            bool              // 标明该连接是否已关闭
	closeReason         CloseReason       // 连接关闭原因
	inPipe              safechan.AnyChan  // 接收消息通道
	outPipe             safechan.AnyChan  // 发送消息通道
	conn                net.Conn          // tcp/udp连接
	id                  uint32            // 连接id
	disconnCallbackFunc func(interface{}) // 连接异常断开的通知回调函数
	disconnCallbackArg  interface{}       // 调用模块的私有参数
	lastRecvTimeStamp   int64             // 记录socket上次收包的时间戳
	aboutToClose        bool              // 标记该连接为将管道中的消息发送后即关闭
	sendbuf             []byte            // 消息发送缓存
	recvbuf             []byte            // 消息收取缓存
}

var (
	logger           = log.NewFileLogger("net_log", "./netlogs", "")
	currentTimeStamp = time.Now().Unix()
)

func init() {
	go func() {
		for {
			// 定时刷新当前时间戳
			time.Sleep(5 * time.Second)
			currentTimeStamp = time.Now().Unix()
		}
	}()
}

// 分配新的消息包
func NewNetMsg(msgtype uint16, data []byte) *NetMsg {
	return &NetMsg{MsgType: msgtype, Data: data}
}

// 连接关闭原因的字符串表示
func (r CloseReason) String() string {
	switch r {
	case BY_SELF:
		return "by_self"
	case BY_OTHER:
		return "by_other"
	case BY_ACCIDENT:
		return "by_accident"
	default:
		return "none"
	}
}

// 设置单个连接的最大静默时间，单位为秒
func SetMaxSilentTime(seconds uint64) {
	maxSilentTime = seconds
}

// 设置日志路径
func SetLogDir(path string) {
	if path != logger.GetDir() {
		logger.ChangeDir(path + "/netlogs")
	}
}

// 设置调试打印级别
func SetDebugLevel(lvl log.Level) {
	logger.SetLevel(lvl)
}

// 分配网络通信实例
//	isServer：该通信方是否为服务方
//	connType：连接方式（tcp/tcp4/tcp6/udp/udp4/udp6）
//	addrs: 通信地址，地址格式为"host:port"或者"[ipv6-host%zone]:port"
//	inMsgLimit：每个连接的接收消息管道可缓存的消息个数上限
//	outMsgLimit：每个连接的发送消息管道可缓存的消息个数上限
func NewCommu(isServer bool, connType string, addrs []string,
	inMsgLimit, outMsgLimit int) *Commu {
	c := new(Commu)
	c.isEnabled = false
	c.isServer = isServer
	c.connType = connType
	c.inMsgLimit = inMsgLimit
	c.outMsgLimit = outMsgLimit
	c.closeDone = make(chan int, 1)
	c.listenDone = make(chan int, 1)

	switch c.connType {
	case "tcp", "tcp4", "tcp6":
		c.isTCP = true
		for _, s := range addrs {
			d, err := net.ResolveTCPAddr(c.connType, s)
			if err != nil {
				logger.Errorf(c.connType, "resolve tcp addr %s failed %s", s, err)
				return nil
			}
			c.tcpAddrs = append(c.tcpAddrs, d)
		}
	case "udp", "udp4", "udp6":
		c.isTCP = false
		for _, s := range addrs {
			d, err := net.ResolveUDPAddr(c.connType, s)
			if err != nil {
				logger.Errorf(c.connType, "resolve udp addr %s failed %s", s, err)
				return nil
			}
			c.udpAddrs = append(c.udpAddrs, d)
		}
	default:
		logger.Errorf(c.connType, "invalid connType")
		return nil
	}

	return c

}

// 连接建立时调用的回调函数原型
type OnNewConnFunc func(conn *Conn)

// 启动网络通信，开始监听/连接端口，连接建立成功后通过回调函数回传给调用模块
func (c *Commu) Start(f OnNewConnFunc) {
	if f == nil {
		logger.Errorf(c.connType, "invalid parameter")
		return
	}

	if c.isEnabled {
		logger.Warnf(c.connType, "communication instance already enabled")
		return
	}

	c.isEnabled = true
	if c.isTCP {
		if c.isServer {
			startTCPServer(c, f)
		} else {
			startTCPClient(c, f)
		}
	} else {
		if c.isServer {
			startUDPServer(c, f)
		} else {
			startUDPClient(c, f)
		}
	}
}

// 停止网络通信，断开所有已建立的连接
func (c *Commu) Stop() {
	if c.isEnabled {
		c.isEnabled = false
		if c.isTCP && c.isServer {
			c.wgListeners.Wait() // 等待所有端口监听结束
		}
		// 等待所有连接断开
		c.wgSendConns.Wait()
		c.wgRecvConns.Wait()
	} else {
		logger.Warnf(c.connType, "communication instance already stopped")
	}
}

// 重启网络通信
func (c *Commu) Restart(f OnNewConnFunc) {
	if f == nil {
		logger.Errorf(c.connType, "invalid parameter")
		return
	}

	c.Stop()
	c.Start(f)
}

// 单个tcp连接重连
func (c *Commu) RetrySingleConn(f OnNewConnFunc, addr string, acceptNonExist bool) error {
	if !c.isEnabled {
		logger.Errorf(c.connType, "communication not enabled!")
		return errors.New("c not enabled")
	}

	if c.isServer && c.isTCP {
		logger.Errorf(c.connType, "non-tcp-client doesn't support reconnect")
		return errors.New("not supported")
	}

	// 校验地址是否合法
	exists := false
	for _, ad := range c.tcpAddrs {
		if ad.String() == addr {
			exists = true
		}
	}
	if !exists && !acceptNonExist {
		logger.Errorf(c.connType, "given addr %s isn't configure in c", addr)
		return errors.New("given addr %s isn't configure in c", addr)
	}

	// 开始连接
	d, resolveErr := net.ResolveTCPAddr(c.connType, addr)
	if resolveErr != nil {
		logger.Errorf(c.connType, "resolve tcp addr %s failed %s", addr, resolveErr)
		return errors.New("resolve tcp addr %s failed %s", addr, resolveErr)
	}
	tcpcon, dialErr := net.DialTCP(c.connType, nil, d)
	if dialErr != nil {
		logger.Errorf(c.connType, "dial tcp addr %v failed %s", addr, dialErr)
		return errors.New("diap tcp %s failed %s", addr, dialErr)
	}
	con := newConn(c, tcpcon)

	// 添加新的服务地址
	if !exists {
		c.tcpAddrs = append(c.tcpAddrs, d)
	}

	// 通知调用模块新连接的建立
	f(con)
	return nil
}

// 创建新的连接
func newConn(c *Commu, conn net.Conn) *Conn {
	newconn := new(Conn)
	newconn.commu = c
	newconn.isClosed = false
	newconn.inPipe = make(safechan.AnyChan, c.inMsgLimit)
	newconn.outPipe = make(safechan.AnyChan, c.outMsgLimit)
	newconn.conn = conn
	logger.Debugf(newconn.commu.connType, "established new con %s", newconn)
	c.wgRecvConns.Add(1)
	go newconn.recv()
	if c.isTCP || !c.isServer {
		// udp服务器暂时只能作为数据接收方，因此不启动发送协程
		c.wgSendConns.Add(1)
		go newconn.send()
	}
	return newconn
}

// 收包routine
func (c *Conn) recv() {
	defer c.commu.wgRecvConns.Done()
	c.recvbuf = make([]byte, SocketBufSize)
	offset := uint32(0)
	readOffset := uint32(0)
	c.lastRecvTimeStamp = currentTimeStamp

	for {
		if !c.commu.isEnabled {
			// 网络通信已被关闭
			c.closeNow(true, false)
		}

		if c.IsClosed() || c.aboutToClose {
			return
		}

		// 若缓存已满则不再继续收包
		if offset < SocketBufSize {
			c.conn.SetReadDeadline(time.Now().Add(time.Second))
			recvLen, err := c.conn.Read(c.recvbuf[offset:])
			if err == io.EOF {
				// 对端主动关闭网络通信，通知调用模块
				logger.Debugf(c.commu.connType, "detected %v closed by the other end", c)
				c.closeNow(false, true)
				return
			} else if err != nil {
				// 检查是否存在系统错误
				ne, ok := err.(net.Error)
				if !ok || (!ne.Temporary() && !ne.Timeout()) {
					logger.Errorf(c.commu.connType,
						"read con %v failed %v, close invalid connection", c, err)
					c.closeNow(false, false)
					return
				}
			}

			if c.commu.isServer && c.commu.isTCP {
				if err == nil {
					c.lastRecvTimeStamp = currentTimeStamp
				} else if currentTimeStamp-c.lastRecvTimeStamp > int64(maxSilentTime) {
					// 关闭静默时间过长的连接
					logger.Errorf(c.commu.connType, "conn %v has been silent too long!", c)
					c.closeNow(false, false)
					return
				}
			}

			offset += uint32(recvLen)
			if offset < HeadSize {
				// 至少要收到8个字节才能校验消息头
				continue
			}

			// 判断是否接收到完整的消息报文，没有则继续收取
			size := binary.BigEndian.Uint32(c.recvbuf[:4])
			if size <= HeadSize || size >= SocketBufSize {
				logger.Errorf(c.commu.connType,
					"invalid packet size %d, close invalid connection %v", size, c)
				c.closeNow(false, false)
				return
			}
			if offset < size {
				continue
			}
		}

		//处理消息缓冲区
		readOffset = 0
		for {
			size := binary.BigEndian.Uint32(c.recvbuf[readOffset : readOffset+4])
			if size >= SocketBufSize {
				logger.Errorf(c.commu.connType,
					"packet size too large %d, close invalid connection, %v", size, c)
				c.closeNow(false, false)
				return
			}

			magic := binary.BigEndian.Uint16(c.recvbuf[readOffset+4 : readOffset+6])
			if magic != uint16(MagicNumber) {
				logger.Errorf(c.commu.connType, "invalid magic number %x", magic)
				c.closeNow(false, false)
				return
			}

			msgtype := binary.BigEndian.Uint16(c.recvbuf[readOffset+6 : readOffset+HeadSize])

			// 向收取管道中添加新的消息体
			data := make([]byte, size-HeadSize)
			copy(data, c.recvbuf[readOffset+HeadSize:readOffset+size])
			err := c.inPipe.Write(NewNetMsg(msgtype, data), logger, time.Second*5)
			if err != nil {
				if err == safechan.CHERR_TIMEOUT {
					logger.Debugf(c.commu.connType, "send msg into pipe timeout! %d, %d, %s",
						len(c.inPipe), cap(c.inPipe), c)
					continue
				} else if err == safechan.CHERR_CLOSED {
					logger.Debugf(c.commu.connType, "channel is closed! con: %s %s", c, c.closeReason)
					return
				} else {
					logger.Errorf(c.commu.connType, "write channel failed! con: %s %s", c, c.closeReason)
					c.closeNow(false, false)
					return
				}
			}

			// 分析缓冲区中是否还有消息可以继续处理
			readOffset += size
			if offset-readOffset > HeadSize {
				size := binary.BigEndian.Uint32(c.recvbuf[readOffset : readOffset+4])
				if size >= SocketBufSize {
					logger.Errorf(c.commu.connType,
						"packet size too large %d, close invalid connection %v", size, c)
					c.closeNow(false, false)
					return
				}

				if size+readOffset <= offset {
					continue
				}
			}
			copy(c.recvbuf, c.recvbuf[readOffset:offset])
			offset = offset - readOffset
			break
		}
	}
}

// 从消息管道收取报文
func (c *Conn) readPacketsFromPipe() error {
	c.sendbuf = c.sendbuf[:0]
	readSize := 0
	sleepSeconds := 0

	// 从发送管道中获取报文
	for readSize < SocketBufSize/2 {
		re, e := c.outPipe.Read(logger, time.Second*time.Duration(sleepSeconds))
		if e != nil {
			if e == safechan.CHERR_TIMEOUT {
				return nil
			} else if e == safechan.CHERR_EMPTY {
				if readSize > 0 {
					break //发送收到的消息
				} else {
					sleepSeconds = 1 //等待5s
					continue
				}
			} else if e == safechan.CHERR_CLOSED {
				logger.Debugf(c.commu.connType,
					"pipe is closed! con: %s %s", c, c.closeReason)
				return e
			} else {
				logger.Errorf(c.commu.connType,
					"read pipe failed! con: %s %s", c, c.closeReason)
				c.closeNow(false, false)
				return e
			}
		} else {
			msg := re.([]byte)
			c.sendbuf = append(c.sendbuf, msg...)
			readSize += len(msg)
			sleepSeconds = 0
		}
	}

	return nil
}

// 发包routine
func (c *Conn) send() {
	defer c.commu.wgSendConns.Done()
	c.sendbuf = make([]byte, 0, SocketBufSize*2)

	for {
		if c.IsClosed() {
			return
		}

		if c.aboutToClose && len(c.outPipe) == 0 {
			c.closeNow(true, false)
		}

		// 收取报文
		e := c.readPacketsFromPipe()
		if e != nil {
			logger.Errorf(c.commu.connType, "read packets failed: %s", e)
			return
		}
		if len(c.sendbuf) == 0 {
			continue
		}

		// 向socket发送报文
		sendLen, err := c.conn.Write(c.sendbuf)
		if err != nil || sendLen != len(c.sendbuf) {
			if !c.IsClosed() {
				logger.Errorf(c.commu.connType,
					"write con %v failed: %s, close invalid connection", c, err)
				c.closeNow(false, false)
			}
			return
		}
	}
}

// 关闭连接，内部函数
//	normalClose: 表示由服务主动关闭
//	isEOF: 表示通信对端已关闭
func (c *Conn) closeNow(normalClose bool, isEOF bool) {
	if c.isClosed {
		return
	}
	if normalClose && !c.aboutToClose {
		//关闭连接之前需将管道中的数据发送完毕
		c.aboutToClose = true
		return
	}

	err := c.conn.Close()
	if err == nil {
		c.isClosed = true
		if normalClose {
			c.closeReason = BY_SELF
		} else {
			if isEOF {
				c.closeReason = BY_OTHER
			} else {
				c.closeReason = BY_ACCIDENT
			}
		}
		close(c.inPipe)
		close(c.outPipe)
		logger.Debugf(c.commu.connType, "connection %s closed %s", c, c.closeReason)

		if c.disconnCallbackFunc != nil {
			c.disconnCallbackFunc(c.disconnCallbackArg)
		}
	}
}

// 关闭连接
func (c *Conn) Close() {
	c.closeNow(true, false)
}

// 检查连接的状态
func (c *Conn) IsClosed() bool {
	return c.isClosed
}

// 连接被关闭的原因
func (c *Conn) CloseReason() CloseReason {
	return c.closeReason
}

// 设置连接的服务id
func (c *Conn) SetId(id uint32) {
	c.id = id
}

// 获取连接的服务id
func (c *Conn) GetId() uint32 {
	return c.id
}

// 连接的字符串表示
func (c *Conn) String() string {
	if c.commu.isServer {
		return fmt.Sprintf("<server,%s,%v,%v,%d,%d,%d>",
			c.commu.connType, c.conn.LocalAddr(), c.conn.RemoteAddr(), c.id,
			len(c.inPipe), len(c.outPipe))
	} else {
		return fmt.Sprintf("<client,%s,%v,%v,%d,%d,%d>",
			c.commu.connType, c.conn.LocalAddr(), c.conn.RemoteAddr(), c.id,
			len(c.inPipe), len(c.outPipe))
	}
}

// 获取连接的本地地址
func (c *Conn) LocalAddr() (ip net.IP, port int, succeeded bool) {
	return ParseAddr(c.conn.LocalAddr())
}

// 获取连接的对端地址
func (c *Conn) RemoteAddr() (ip net.IP, port int, succeeded bool) {
	return ParseAddr(c.conn.RemoteAddr())
}

func ParseAddr(addr net.Addr) (ip net.IP, port int, succeeded bool) {
	switch addr.(type) {
	case *net.TCPAddr:
		addr := addr.(*net.TCPAddr)
		ip = addr.IP
		port = addr.Port
		succeeded = true
	case *net.UDPAddr:
		addr := addr.(*net.UDPAddr)
		ip = addr.IP
		port = addr.Port
		succeeded = true
	default:
		succeeded = false
	}
	return
}

// 发送消息包至消息管道，成功返回true，超时或失败则返回false;
// 参数d为0表示消息发送会阻塞直至成功返回
func (c *Conn) SendMsg(msg *NetMsg, d time.Duration) error {
	if c.aboutToClose {
		logger.Errorf(c.commu.connType, "connection about to close")
		return errors.New("connection about to close")
	}

	if c.isClosed {
		logger.Errorf(c.commu.connType, "connection already closed")
		return errors.New("connection closed")
	}

	if !c.commu.isTCP && c.commu.isServer {
		logger.Errorf(c.commu.connType, "udp server doesn't support sending message yet!")
		return errors.New("udp server can't send msg")
	}

	if len(msg.Data) == 0 || len(msg.Data) >= SocketBufSize-HeadSize {
		logger.Errorf(c.commu.connType, "invalid msg len %d", len(msg.Data))
		return errors.New("invalid msg format")
	}

	// 添加消息头
	packet := make([]byte, len(msg.Data)+HeadSize)
	binary.BigEndian.PutUint32(packet[0:4], uint32(len(msg.Data)+HeadSize))
	binary.BigEndian.PutUint16(packet[4:6], uint16(MagicNumber))
	binary.BigEndian.PutUint16(packet[6:HeadSize], msg.MsgType)
	copy(packet[HeadSize:], msg.Data)

	err := c.outPipe.Write(packet, logger, d)
	if err != nil {
		logger.Errorf(c.commu.connType, "write failed: %s", err)
		return err
	}
	return nil
}

// 发送信令，主要用于客户端组件和服务端组件之间的控制协议
func (c *Conn) SendCmd(data []byte, d time.Duration) error {
	return c.SendMsg(NewNetMsg(uint16(proto.MsgType_CMD), data), d)
}

// 发送正常通信协议
func (c *Conn) SendNormalMsg(data []byte, d time.Duration) error {
	return c.SendMsg(NewNetMsg(uint16(proto.MsgType_NORMAL), data), d)
}

// 从消息管道中获取消息包，成功返回消息切片和nil，超时或失败则返回nil和错误;
// 参数d为0表示在没有消息可收的情况下马上返回nil和空管道错误
func (c *Conn) RecvMsg(d time.Duration) (*NetMsg, error) {
	msg, err := c.inPipe.Read(logger, d)
	if err != nil {
		if err != safechan.CHERR_TIMEOUT && err != safechan.CHERR_EMPTY {
			logger.Errorf(c.commu.connType, "read failed: %s", err)
		}
		return nil, err
	}

	re, ok := msg.(*NetMsg)
	if !ok {
		logger.Errorf(c.commu.connType, "invalid msg type %v", msg)
		return nil, err
	}
	return re, nil
}

// 注册连接异常断开的回调函数及私有参数
func (c *Conn) RegisterDisconnCallback(fp func(interface{}), arg interface{}) {
	if c.commu.isTCP {
		c.disconnCallbackFunc = fp
		c.disconnCallbackArg = arg
	}
}

// 启动tcp连接协程
func restartTCPClient(c *Commu, f OnNewConnFunc) {
	go func() {
		for _, addr := range c.tcpAddrs {
			con, err := net.DialTCP(c.connType, nil, addr)
			if err != nil {
				logger.Errorf(c.connType, "dial tcp addr %v failed", addr)
				continue
			}

			// 通知调用模块新连接的建立
			f(newConn(c, con))
		}
	}()
}

// tcp端口监听routine
func listen(c *Commu, f OnNewConnFunc, index int) {
	defer c.wgListeners.Done()
	logger.Debugf(c.connType, "start listening to %s", c.tcpAddrs[index])

	l, err := net.ListenTCP(c.connType, c.tcpAddrs[index])
	if err != nil {
		panic(fmt.Sprintf("listen to tcp addr %v failed %s!", c.tcpAddrs[index], err))
		return
	}

	var tempDelay time.Duration
	for {
		if !c.isEnabled {
			l.Close() // 网络通信已被关闭，退出协程
			break
		}

		con, err := l.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				time.Sleep(tempDelay)
				continue
			}
			logger.Errorf(c.connType, "accept tcp addr %v failed, %s",
				c.tcpAddrs[index], err)
			return
		}
		tempDelay = 0

		// 启动保活, 9分钟(60 + 8 * 60)内没有任何通信则会自动断开连接
		con.(*net.TCPConn).SetKeepAlive(true)
		con.(*net.TCPConn).SetKeepAlivePeriod(time.Minute)

		// 通知调用模块新连接的建立
		f(newConn(c, con))
	}
}

// 启动tcp服务协程
func startTCPServer(c *Commu, f OnNewConnFunc) {
	for i, _ := range c.tcpAddrs {
		c.wgListeners.Add(1)
		go listen(c, f, i)
	}
}

// 启动tcp连接协程
func startTCPClient(c *Commu, f OnNewConnFunc) {
	go func() {
		for _, addr := range c.tcpAddrs {
			con, err := net.DialTCP(c.connType, nil, addr)
			if err != nil {
				logger.Errorf(c.connType, "dial tcp addr %v failed %s", addr, err)
				continue
			}

			// 通知调用模块新连接的建立
			f(newConn(c, con))
		}
	}()
}

// 启动udp服务协程
func startUDPServer(c *Commu, f OnNewConnFunc) {
	go func() {
		for _, addr := range c.udpAddrs {
			con, err := net.ListenUDP(c.connType, addr)
			if err != nil {
				logger.Errorf(c.connType, "listen udp addr %v failed %s", addr, err)
				continue
			}

			// 通知调用模块新连接的建立
			f(newConn(c, con))
		}
	}()
}

// 启动udp连接协程
func startUDPClient(c *Commu, f OnNewConnFunc) {
	go func() {
		for _, addr := range c.udpAddrs {
			con, err := net.DialUDP(c.connType, nil, addr)
			if err != nil {
				logger.Errorf(c.connType, "dial udp addr %v failed %s", addr, err)
				continue
			}

			// 通知调用模块新连接的建立
			f(newConn(c, con))
		}
	}()
}
