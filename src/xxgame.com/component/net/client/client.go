// client组件实现的功能如下:
//	1. 通过配置格式加载、重加载服务器列表、路由方式等；
//	2. 与配置指定的服务器建立连接；
//	3. 按配置指定的路由方式向服务器集群发送消息包；
package client

import (
	"fmt"
	"sync"
	"time"
)

import (
	protobuf "github.com/golang/protobuf/proto"
	cfgMgr "xxgame.com/component/cfg"
	dao "xxgame.com/component/dao/types"
	"xxgame.com/component/log"
	"xxgame.com/component/net"
	"xxgame.com/framework"
	"xxgame.com/types/proto"
	protoCfg "xxgame.com/types/proto/config"
	"xxgame.com/utils"
	"xxgame.com/utils/errors"
)

// 客户端组件
type Client struct {
	comm        *net.Commu                 // 网络组件
	logger      *log.Logger                // 日志组件
	mu          sync.Mutex                 // 保护锁
	cfgName     string                     // 全局配置名
	isStarted   bool                       // 是否已启动
	robinIdx    int                        // 记录下次采用轮询调度时派发的服务器ID
	allSvrId    []uint32                   // 所有服务ID
	svrIdMap    map[string]uint32          // 服务地址到服务id的映射关系
	connMap     map[uint32]*net.Conn       // 连接管理
	routeTable  []int                      // 路由表
	msgProc     framework.MsgProcessor     // 消息处理方
	serverId    uint32                     // 客户端组件所属服务的id
	finiCheck   chan int                   // 通知巡检协程退出
	filterChans map[uint32]chan *proto.Msg // 消息过滤管道
	reloadChan  chan int                   // 通知重加载配置
	connOK      chan int                   // 通知所有连接建立成功
}

const (
	maxServerNum = dao.TableNum
	logTag       = "cli_com"
)

// 分配并初始化客户端组件
//	cfgName: 配置标识符
//	logger: 日志管理器
//	serverId: 服务id
func NewClient(cfgName string, logger *log.Logger,
	serverId uint32, proc framework.MsgProcessor) *Client {
	net.SetLogDir(logger.GetDir())
	c := new(Client)
	c.logger = logger
	c.cfgName = cfgName
	c.serverId = serverId
	c.msgProc = proc
	c.finiCheck = make(chan int, 1)
	c.filterChans = make(map[uint32]chan *proto.Msg, 10)
	c.reloadChan = make(chan int, 1)
	c.connOK = make(chan int, 1)
	return c
}

// 客户端组件的字符串表示
func (c *Client) String() string {
	cfg, _ := cfgMgr.Instance().Get(c.cfgName).(*protoCfg.ClientCfg)
	if cfg != nil {
		return fmt.Sprintf("<client,%s>", *(cfg.ServerEntries[0].Id))
	}
	return fmt.Sprintf("<client>")
}

// 客户端组件每个连接的消息循环
func (c *Client) connJob(conn *net.Conn) {
	defer conn.Close()

	// 保存连接
	c.mu.Lock()
	ip, port, _ := conn.RemoteAddr()
	id, exists := c.svrIdMap[fmt.Sprintf("%s:%d", ip, port)]
	if !exists {
		c.logger.Errorf(logTag, "got unexpected conn %s!", conn)
		c.mu.Unlock()
		return
	}
	conn.SetId(id)
	c.connMap[id] = conn
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.connMap, id)
		c.mu.Unlock()
	}()

	c.logger.Debugf(logTag, "established new conn %s, svrIdMap:%v, connMap:%v, allSvrId:%v",
		conn, c.svrIdMap, c.connMap, c.allSvrId)

	// 检查是否所有连接都已建立，是则发出通知
	if len(c.connMap) == len(c.allSvrId) && len(c.connOK) == 0 {
		c.connOK <- 0
	}

	// 发送注册信令
	cmd := new(proto.CmdMsg)
	cmd.Cmd = proto.CmdID_REG.Enum()
	cmd.SrcServiceID = &c.serverId
	cmd.DstServiceID = protobuf.Uint32(id)
	buf, _ := protobuf.Marshal(cmd)
	conn.SendCmd(buf, 0)

	// 进入消息循环
	aliveTimer := time.NewTicker(time.Second * 5)
	defer aliveTimer.Stop()
	cmd.Cmd = proto.CmdID_KEEPALIVE.Enum()
	cmd.SrcServiceID = &c.serverId
	cmd.DstServiceID = protobuf.Uint32(id)
	buf, _ = protobuf.Marshal(cmd)
	for {
		if conn.IsClosed() {
			return
		}

		// 每5秒钟发送一次保活信令
		select {
		case <-aliveTimer.C:
			err := conn.SendCmd(buf, 0)
			if err != nil {
				c.logger.Errorf(logTag, "send keepalive cmd failed %s", err)
			}
		default:
		}

		data, recvErr := conn.RecvMsg(time.Second)
		if recvErr != nil {
			continue
		}

		if data.MsgType == uint16(proto.MsgType_CMD) {
			c.logger.Errorf(logTag, "client com doesn't accept cmd!", conn)
			return
		} else {
			// 根据过滤规则对消息进行过滤
			if len(c.filterChans) > 0 {
				msg := new(proto.Msg)
				decodeErr := protobuf.Unmarshal(data.Data, msg)
				if decodeErr != nil {
					c.logger.Errorf(logTag, "decode msg failed, %s", decodeErr)
					continue
				}
				sh := msg.Header.GetSHeader()
				if sh == nil {
					c.logger.Errorf(logTag, "get session header failed: %s", msg)
					continue
				}
				msgId := sh.GetMsgID()
				ch, exists := c.filterChans[msgId]
				if exists && ch != nil {
					ch <- msg
					continue
				}
			}

			if e := c.msgProc.OnNewMsg(data.Data); e != nil {
				c.logger.Errorf(logTag, "process msg failed: %s", e)
				continue
			}
		}
	}
}

// 等待所有连接成功建立
func (c *Client) WaitOK() {
	select {
	case <-c.connOK:
		break
	case <-time.After(time.Second * 5):
		panic(fmt.Errorf("can't establish connection to cluster nodes! %#v", c.svrIdMap))
	}
}

// 重加载配置
func (c *Client) reloadCfg() {
	cfg, ok := cfgMgr.Instance().Get(c.cfgName).(*protoCfg.ClientCfg)
	if !ok {
		panic("get cfg failed")
	}

	// 重加载路由表
	rt := make([]int, maxServerNum)
	for _, entry := range cfg.RouteEntries {
		for i := *entry.HashBegin; i <= *entry.HashEnd && i < maxServerNum; i++ {
			rt[i] = int(*entry.ServerIdx)
		}
	}
	c.mu.Lock()
	c.routeTable = rt
	c.mu.Unlock()

	// 重加载服务列表
	allSvrId := make([]uint32, 0)
	svrIdMap := make(map[string]uint32, maxServerNum)
	addrs := make([]string, 0, len(cfg.ServerEntries))
	for _, entry := range cfg.ServerEntries {
		id, e := utils.ConvertServerIDString2Number(*entry.Id)
		if e != nil {
			c.logger.Errorf(logTag, "invalid server id %s", *entry.Id)
			panic("invalid configuration!")
		}
		allSvrId = append(allSvrId, id)
		for _, d := range entry.Addrs {
			svrIdMap[d] = id
		}
		addrs = append(addrs, entry.Addrs...)
	}
	c.mu.Lock()
	// 关闭和无用服务的通信链路
	for addr, svrId := range c.svrIdMap {
		if _, exists := svrIdMap[addr]; !exists {
			conn, ok := c.connMap[svrId]
			if ok {
				conn.Close()
				delete(c.connMap, svrId)
			}
		}
	}
	// 更新服务器映射表
	c.allSvrId = allSvrId
	c.svrIdMap = svrIdMap
	c.mu.Unlock()
}

// 每10秒钟检查所有与服务端组件的连接，当发现某个连接被关闭或不存在时则尝试创建
func (c *Client) checkConns() {
	for {
		select {
		case <-c.finiCheck:
			return
		default:
			time.Sleep(time.Second * 10)
		}

		select {
		case <-c.finiCheck:
			return
		default:
		}

		select {
		case <-c.reloadChan:
			c.reloadCfg()
		default:
		}

		c.mu.Lock()
		for ad, id := range c.svrIdMap {
			conn, exists := c.connMap[id]
			if !exists || conn.IsClosed() {
				c.logger.Errorf(logTag, "reconnecting with server %s<%s, %s>",
					c.cfgName, ad, utils.ConvertServerIDNumber2String(id))

				delete(c.connMap, id)
				c.comm.RetrySingleConn(func(conn *net.Conn) {
					go c.connJob(conn)
				}, ad, true)
			}
		}
		c.mu.Unlock()
	}
}

// 启动客户端组件
func (c *Client) Start() (err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isStarted {
		c.logger.Errorf(logTag, "client already started!")
		err = errors.New("client already started")
		return
	}

	// 读取配置
	cfg, ok := cfgMgr.Instance().Get(c.cfgName).(*protoCfg.ClientCfg)
	if !ok {
		c.logger.Errorf(logTag, "get cfg failed")
		err = errors.New("get cfg failed")
		return
	}
	c.allSvrId = make([]uint32, 0)
	c.svrIdMap = make(map[string]uint32, maxServerNum)
	c.connMap = make(map[uint32]*net.Conn, maxServerNum)
	addrs := make([]string, 0, len(cfg.ServerEntries))
	for _, entry := range cfg.ServerEntries {
		id, e := utils.ConvertServerIDString2Number(*entry.Id)
		if e != nil {
			c.logger.Errorf(logTag, "invalid server id %s", *entry.Id)
			err = errors.New("invalid server id")
			return
		}
		c.allSvrId = append(c.allSvrId, id)
		for _, d := range entry.Addrs {
			c.svrIdMap[d] = id
		}
		addrs = append(addrs, entry.Addrs...)
	}
	c.routeTable = make([]int, maxServerNum)
	for _, entry := range cfg.RouteEntries {
		for i := *entry.HashBegin; i <= *entry.HashEnd && i < maxServerNum; i++ {
			c.routeTable[i] = int(*entry.ServerIdx)
		}
	}

	// 初始化网络组件
	c.comm = net.NewCommu(false, "tcp", addrs, 0, int(*cfg.SendLimit))
	if c.comm == nil {
		c.logger.Errorf(logTag, "allocate commu failed")
		err = errors.New("alloc commu failed")
		return
	}

	// 启动网络通信
	c.logger.Infof(logTag, "client %s is started", c)
	c.comm.Start(func(conn *net.Conn) {
		go c.connJob(conn)
	})

	// 启动巡检协程
	go c.checkConns()

	c.isStarted = true
	return nil
}

// 停止客户端组件
func (c *Client) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isStarted {
		c.logger.Infof(logTag, "client %s is stopped", c)
		c.comm.Stop()
		c.finiCheck <- 0
		c.isStarted = false
	}
}

// 重启客户端组件
func (c *Client) Restart() error {
	c.logger.Debugf(logTag, "client %s is restarted", c)
	c.Stop()
	return c.Start()
}

// 重加载配置通知
func (c *Client) ReloadCfg() {
	c.reloadChan <- 0
}

// 获取源服务id
func (c *Client) GetSrcSvrId() uint32 {
	return c.serverId
}

// 获取目标服务类型
func (c *Client) GetDstSvrType() uint32 {
	if len(c.allSvrId) == 0 {
		panic("invalid client component!")
	}

	return utils.ConvertServerIDNumber2Fields(c.allSvrId[0])[0]
}

// 添加消息过滤管道
func (c *Client) AddFilterChan(msgId uint32, ch chan *proto.Msg) {
	c.filterChans[msgId] = ch
}

// 获取与某个服务端的连接
func (c *Client) GetConn(id uint32) *net.Conn {
	conn, exists := c.connMap[id]
	if exists {
		return conn
	}
	return nil
}

// 发送消息
//	rt: 路由方式
//	hashCalc: hash计算器
//	data: 待发送数据
//	arg：该参数的类型和意义由路由方式决定，具体如下：
//	     以p2p方式发送数据时该参数用于保存服务id(uint32类型)；
//	     以广播和轮询方式发送数据时忽略该参数；
//	     以取模和路由表查询方式发送数据时该参数用于保存hash key([]byte类型)
func (c *Client) SendMsg(rt proto.RouteTypeID, hashCalc HashCalculator, data []byte, arg interface{}) error {
	switch rt {
	case proto.RouteTypeID_P2P:
		// p2p方式，显式指定服务器id
		id, ok := arg.(uint32)
		if !ok {
			c.logger.Errorf(logTag, "invalid arg %v", arg)
			return errors.New("invalid argument")
		}
		c.mu.Lock()
		conn, exists := c.connMap[id]
		c.mu.Unlock()
		if !exists {
			c.logger.Errorf(logTag, "can't get conn for server %d, allSvrId:%#v, connMap:%#v", id, c.allSvrId, c.connMap)
			return errors.New("get conn failed")
		}
		if sendErr := conn.SendNormalMsg(data, 0); sendErr != nil {
			c.logger.Errorf(logTag, "send data failed on con %s %s", conn, sendErr)
			return errors.New("send data failed on con %s %s", conn, sendErr)
		}
		return nil
	case proto.RouteTypeID_BD:
		// 广播消息
		c.mu.Lock()
		for _, conn := range c.connMap {
			if sendErr := conn.SendNormalMsg(data, 0); sendErr != nil {
				c.logger.Errorf(logTag, "send data failed on con %s %s", conn, sendErr)
				continue
			}
		}
		c.mu.Unlock()
		return nil
	case proto.RouteTypeID_RD:
		// 轮询调度
		var conn *net.Conn
		var exists bool
		failCount := 0
		for {
			c.mu.Lock()
			idx := c.robinIdx
			c.robinIdx++
			if c.robinIdx >= len(c.allSvrId) {
				c.robinIdx = 0
			}
			id := c.allSvrId[idx]
			conn, exists = c.connMap[id]
			c.mu.Unlock()
			if !exists || conn.IsClosed() {
				failCount++
			} else {
				break // got a valid connection
			}
			if failCount == len(c.allSvrId) {
				c.logger.Errorf(logTag, "all connections from client %s are invalid now!", c)
				return errors.New("all connections from client %s are invalid now!", c)
			}
		}
		if sendErr := conn.SendNormalMsg(data, 0); sendErr != nil {
			c.logger.Errorf(logTag, "send data failed on con %s %s", conn, sendErr)
			return errors.New("send data failed on con %s %s", conn, sendErr)
		}
		return nil
	case proto.RouteTypeID_TM, proto.RouteTypeID_TMN:
		// hash值对服务器总数取模
		key, ok := arg.([]byte)
		if !ok {
			c.logger.Errorf(logTag, "invalid arg %v", arg)
			return errors.New("invalid argument")
		}
		hash := hashCalc.Hash(key) % uint64(len(c.allSvrId))
		c.mu.Lock()
		id := c.allSvrId[hash]
		conn, exists := c.connMap[id]
		c.mu.Unlock()
		if !exists {
			c.logger.Errorf(logTag, "can't get conn for server %d, allSvrId:%#v, connMap:%#v", id, c.allSvrId, c.connMap)
			return errors.New("get conn failed")
		}
		if sendErr := conn.SendNormalMsg(data, 0); sendErr != nil {
			c.logger.Errorf(logTag, "send data failed on con %s %s", conn, sendErr)
			return errors.New("send data failed on con %s %s", conn, sendErr)
		}
		return nil
	case proto.RouteTypeID_RT, proto.RouteTypeID_RTN:
		// 根据路由表配置选择服务器
		key, ok := arg.([]byte)
		if !ok {
			c.logger.Errorf(logTag, "invalid arg %v", arg)
			return errors.New("invalid argument")
		}
		hash := hashCalc.Hash(key) % uint64(maxServerNum)
		c.mu.Lock()
		idx := c.routeTable[hash]
		id := c.allSvrId[idx]
		conn, exists := c.connMap[uint32(id)]
		c.mu.Unlock()
		if !exists {
			c.logger.Errorf(logTag, "can't get conn for server %d, allSvrId:%#v, connMap:%#v", id, c.allSvrId, c.connMap)
			return errors.New("get conn failed")
		}
		if sendErr := conn.SendNormalMsg(data, 0); sendErr != nil {
			c.logger.Errorf(logTag, "send data failed on con %s %s", conn, sendErr)
			return errors.New("send data failed on con %s %s", conn, sendErr)
		}
		return nil
	}

	return errors.New("invalid route type %v", rt)
}
