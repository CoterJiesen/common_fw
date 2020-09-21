package webserver

// 提供网络连接的管理字典
// 提供连接的回调函数
// 提供断线的回调函数

// 提供发送函数
// 提供关闭函数
// 提供接收的回调函数
// 提供连接的远程地址
// 提供连接分配的id
// 提供ping-pong心跳，断线机制

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

import (
	"github.com/gorilla/websocket"
	"xxgame.com/component/log"
	"xxgame.com/component/uuid"
	"xxgame.com/utils/errors"
)

var webNodeUuidGen = new(uuid.UuidGen) // 客户端连接id生成器

const (
	readBufferSize  = 32 << 10            // 读消息缓冲区32K
	writeBufferSize = 32 << 10            // 写消息缓冲区32K
	chanWait4Send   = 10 * time.Second    // 发送到管道最长等待时间
	writeWait       = 10 * time.Second    // 写等待时间
	readWait        = 5 * time.Second     // 最长等待读时间
	pongWait        = 15 * time.Second    // 基于ping-pong机制判断断线
	pingPeriod      = pongWait - readWait // 基于ping-pong机制判断断线
)

// 删除连接的管道
type DelNodeChan interface {
	getDelNodeChan() chan *WebNetNode // 获取异常断开的连接对象的管道
}

// 网络事件回调函数
type WebNetEvent interface {
	OnWebNetNewNode(node *WebNetNode)
	OnWebNetDelNode(node *WebNetNode)
}

const (
	LOGTAG = "webnet"
)

type WebNetNode struct {
	conn           *websocket.Conn                          // websocket 连接对象
	id             uint32                                   // 连接id
	closed         uint32                                   // 连接id
	recvFunc       func(node *WebNetNode, msg []byte) error // 接收消息的回调
	sendChan       chan []byte                              // 发送消息通道
	delNodeChan    DelNodeChan                              // 获取异常断开的连接对象的管道
	logger         *log.Logger                              //日志组件
	isAboutToClose uint32                                   // 是否已经要关闭
	closedbyServer uint32                                   // 服务器关闭
	closedbyClient uint32                                   // 服务器关闭
}

func (wnn *WebNetNode) recv() {
	wnn.logger.Debugf(LOGTAG, "client %v connected", wnn.conn.RemoteAddr().String())
	for {
		if atomic.LoadUint32(&wnn.closed) == 1 {
			//如果已经关闭了连接则直接返回，结束该线程
			return
		}
		if atomic.LoadUint32(&wnn.isAboutToClose) == 1 {
			wnn.logger.Debugf(LOGTAG, "client recv1 isAboutToClose : %v", wnn.conn.RemoteAddr().String())
			break
		}
		_, message, err := wnn.conn.ReadMessage()
		if atomic.LoadUint32(&wnn.closed) == 1 {
			//如果已经关闭了连接则直接返回，结束该线程
			return
		}
		if atomic.LoadUint32(&wnn.isAboutToClose) == 1 {
			wnn.logger.Debugf(LOGTAG, "client recv2 isAboutToClose : %v", wnn.conn.RemoteAddr().String())
			break
		}
		if err != nil {
			//当玩家强行关闭浏览器 便不能读取
			wnn.logger.Errorf(LOGTAG, "client %v close : %v", wnn.conn.RemoteAddr().String(), err)
			break
		}
		//调用用户注册的接收回调函数
		if recvErr := wnn.recvFunc(wnn, message); recvErr != nil {
			wnn.logger.Errorf(LOGTAG, "client %v recvFunc failed: %v", wnn.conn.RemoteAddr().String(), recvErr)
			break
		}
	}
	//atomic.StoreUint32(&wnn.closed, 1)
	wnn.delNodeChan.getDelNodeChan() <- wnn
}

func (wnn *WebNetNode) sendWithDeadline(mt int, msg []byte) error {
	//wnn.conn.SetWriteDeadline(time.Now().Add(writeWait))
	return wnn.conn.WriteMessage(mt, msg)
}

func (wnn *WebNetNode) send() {
	/*
		ticker := time.NewTicker(pingPeriod)
		defer func() {
			ticker.Stop()
		}()
	*/
	for {
		if atomic.LoadUint32(&wnn.closed) == 1 {
			//如果已经关闭了连接则直接返回，结束该线程
			return
		}

		if atomic.LoadUint32(&wnn.isAboutToClose) == 1 && len(wnn.sendChan) == 0 {
			//即将关闭，并且数据已经发完，退出
			wnn.logger.Debugf(LOGTAG, "client send isAboutToClose : %v", wnn.conn.RemoteAddr().String())
			break
		}

		select {
		case content, ok := <-wnn.sendChan:
			if !ok {
				wnn.sendWithDeadline(websocket.CloseMessage, []byte{})
				wnn.logger.Errorf(LOGTAG, "client %v get send msg failed", wnn.conn.RemoteAddr().String())
				break
			}
			if err := wnn.sendWithDeadline(websocket.BinaryMessage, content); err != nil {
				//当玩家强行关闭浏览器 便不能写入
				wnn.logger.Errorf(LOGTAG, "client %v send msg failed: %v", wnn.conn.RemoteAddr().String(), err)
				break
			}
		case <-time.After(time.Second * 5):
			//wnn.logger.Debugf(LOGTAG, "client send timeout isAboutToClose : %v", wnn.conn.RemoteAddr().String())
		}
	}
	//atomic.StoreUint32(&wnn.closed, 1)
	wnn.delNodeChan.getDelNodeChan() <- wnn
}

func (wnn *WebNetNode) Send(buff []byte) (err error) {
	defer func() {
		if ex := recover(); ex != nil {
			str := fmt.Sprintf("%v", ex)
			if strings.Contains(str, "send on closed channel") {
				err = errors.New("recover : webnet connection closed")
			} else {
				panic(str)
			}
		}
	}()

	if atomic.LoadUint32(&wnn.closed) == 1 || atomic.LoadUint32(&wnn.isAboutToClose) == 1 {
		err = errors.New("webnet connection closed")
		return
	}
	select {
	case wnn.sendChan <- buff:
		err = nil
	case <-time.After(chanWait4Send):
		err = errors.New("webnet send timeout")
	}
	return
}
func (wnn *WebNetNode) IsClose() bool {
	//	wnn.logger.Debugf(LOGTAG, "IsClose %v,%v close by server", atomic.LoadUint32(&wnn.closed),
	//		atomic.LoadUint32(&wnn.isAboutToClose))

	return atomic.LoadUint32(&wnn.closed) == 1 || atomic.LoadUint32(&wnn.isAboutToClose) == 1
}

func (wnn *WebNetNode) Close() {
	atomic.StoreUint32(&wnn.closedbyServer, 1) //服务器主动断开
	wnn.logger.Errorf(LOGTAG, "client %v close by server", wnn.conn.RemoteAddr().String())
	wnn.delNodeChan.getDelNodeChan() <- wnn
}

// 获取连接的服务id
func (wnn *WebNetNode) GetId() uint32 {
	return wnn.id
}

// 提供连接的远程地址
func (wnn *WebNetNode) RemoteAddr() net.Addr {
	return wnn.conn.RemoteAddr()
}

// WEB网络通信类型
type WebNetMgr struct {
	upgrader    *websocket.Upgrader
	inited      bool                                     // 是否已经初始化过
	m4init      sync.RWMutex                             // 保护init的读写锁
	newNodeChan chan *WebNetNode                         // 接收新建连接事件的管道
	delNodeChan chan *WebNetNode                         // 接收断开连接事件的管道
	nodes       map[uint32]*WebNetNode                   // 保存连接的map
	m4nodes     sync.RWMutex                             // 保护nodes的读写锁
	recvFunc    func(node *WebNetNode, msg []byte) error // 接收消息的回调
	webNetEvent WebNetEvent                              // 服务实现
	logger      *log.Logger
}

func (wnm *WebNetMgr) Init(w WebNetEvent, recvFunc func(node *WebNetNode, msg []byte) error, logger *log.Logger) error {
	//加读写锁保护
	wnm.m4init.Lock()
	defer wnm.m4init.Unlock()

	if wnm.inited {
		return errors.New("WebNetMgr already inited")
	}

	wnm.logger = logger

	wnm.newNodeChan = make(chan *WebNetNode, 1)
	wnm.delNodeChan = make(chan *WebNetNode, 128)

	wnm.m4nodes.Lock()
	wnm.nodes = make(map[uint32]*WebNetNode)
	wnm.m4nodes.Unlock()

	wnm.recvFunc = recvFunc
	wnm.webNetEvent = w
	wnm.inited = true

	wnm.upgrader = &websocket.Upgrader{ReadBufferSize: readBufferSize, WriteBufferSize: writeBufferSize}

	//默认允许跨域访问
	wnm.upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}

	go wnm.listenNewNode()
	go wnm.listenDelNode()
	return nil
}

func (wnm *WebNetMgr) listenNewNode() {
	for {
		node := <-wnm.newNodeChan
		wnm.m4nodes.Lock()
		wnm.nodes[node.id] = node
		wnm.m4nodes.Unlock()
		go node.recv()
		go node.send()
		wnm.webNetEvent.OnWebNetNewNode(node)
	}
}

func (wnm *WebNetMgr) realClose(node *WebNetNode) {
	if atomic.LoadUint32(&node.closedbyServer) == 1 &&
		atomic.LoadUint32(&node.isAboutToClose) == 0 {
		//如果即将关闭为0
		//		wnm.logger.Debugf(LOGTAG, "client realClose1 isAboutToClose : %v", node.conn.RemoteAddr().String())
		atomic.StoreUint32(&node.isAboutToClose, 1)
		return
	}
	//	wnm.logger.Debugf(LOGTAG, "client realClose2 isAboutToClose : %v", node.conn.RemoteAddr().String())

	//关闭连接通知上层
	atomic.StoreUint32(&node.closed, 1)
	wnm.m4nodes.Lock()
	defer wnm.m4nodes.Unlock()
	delete(wnm.nodes, node.id)
	close(node.sendChan)
	node.conn.Close()
	wnm.webNetEvent.OnWebNetDelNode(node)
}

func (wnm *WebNetMgr) listenDelNode() {
	for {
		node := <-wnm.delNodeChan
		wnm.m4nodes.RLock()
		if _, ok := wnm.nodes[node.id]; ok {
			wnm.m4nodes.RUnlock()
			wnm.realClose(node)
		} else {
			wnm.m4nodes.RUnlock()
		}

		//		wnm.m4nodes.Lock()
		//		// 这里有可能在同时收到 来自 node.recv 、node.send 、Close函数调入的，因此需要判断是否存在，否则OnWebNetDelNode将被调用两次
		//		if _, ok := wnm.nodes[node.id]; ok {
		//			delete(wnm.nodes, node.id)
		//			wnm.m4nodes.Unlock()
		//			close(node.sendChan)
		//			node.conn.Close()
		//			wnm.webNetEvent.OnWebNetDelNode(node)
		//		} else {
		//			wnm.m4nodes.Unlock()
		//		}
	}
}

// 获取连接
func (wnm *WebNetMgr) GetNode(sid uint32) *WebNetNode {
	wnm.m4init.RLock()
	if wnm.inited == false {
		wnm.m4init.RUnlock()
		return nil
	}
	wnm.m4init.RUnlock()

	wnm.m4nodes.RLock()
	defer wnm.m4nodes.RUnlock()
	node, exist := wnm.nodes[sid]
	if node != nil && exist &&
		atomic.LoadUint32(&node.closed) == 0 &&
		atomic.LoadUint32(&node.isAboutToClose) == 0 {
		return node
	}
	return nil
}

// 实现webserver.WebNetEvent接口:获取异常断开的连接对象的管道
func (wnm *WebNetMgr) getDelNodeChan() chan *WebNetNode {
	return wnm.delNodeChan
}

func (wnm *WebNetMgr) HttpHandleFunc(w http.ResponseWriter, r *http.Request) {
	//服务器还未初始化时直接返回，不做处理
	wnm.m4init.RLock()
	if wnm.inited == false {
		wnm.m4init.RUnlock()
		return
	}
	wnm.m4init.RUnlock()

	conn, err := wnm.upgrader.Upgrade(w, r, nil)
	//客户端不支持websocket 或 客户端websocket头部参数错误
	//握手的阶段如果客户端发送信息过来 err != nil 这里简单做返回
	if err != nil {
		return
	}
	conn.SetReadLimit(readBufferSize)
	//conn.SetReadDeadline(time.Now().Add(pongWait))
	//conn.SetPongHandler(func(string) error { conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	node := new(WebNetNode)
	node.id = webNodeUuidGen.GenID()
	node.conn = conn
	node.closed = 0
	node.closedbyServer = 0
	node.isAboutToClose = 0
	node.sendChan = make(chan []byte, 1024) // 发送消息通道
	node.delNodeChan = wnm
	node.recvFunc = wnm.recvFunc
	node.logger = wnm.logger
	wnm.newNodeChan <- node
}
