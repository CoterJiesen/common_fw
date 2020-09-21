// 运营日志模块，提供向日志服务器发送运营日志的功能
package oplog

import (
	"fmt"
	"reflect"
	"sync"
	"time"
)

import (
	"github.com/golang/protobuf/proto"
	"gopkg.in/mgo.v2/bson"
	"xxgame.com/component/log"
	"xxgame.com/component/net/client"
	"xxgame.com/component/uuid"
	msg "xxgame.com/types/proto"
	"xxgame.com/types/proto/interior"
	"xxgame.com/utils"
)

// 会话类型
type dataSession struct {
	id  uint64                 // 会话ID
	t   *time.Timer            // 会话定时器
	tq  chan int               // 用于通知定时器已销毁
	res *interior.CoreResponse // 同步返回的数据库操作结果
	err error                  // 同步操作错误
}

// 运营日志模块实例类型
type OpLogger struct {
	inited       bool                    // 初始化标志
	cli          *client.Client          // 用于访问LOG服务的客户端组件
	logger       *log.Logger             // 系统日志对象
	srcSvrId     uint32                  // 源服务id
	dstSvrId     uint32                  // 日志服务id
	resChan      chan *msg.Msg           // 消息接收管道
	sessionIdGen uuid.UuidGen            // 会话id生成器
	smap         map[uint64]*dataSession // 会话映射表
	smLock       sync.Mutex              // 保护会话映射表
}

var (
	logTag          = "oplog"
	logInstance     *OpLogger
	MsgChanCap      = 1000            // 消息管道容量
	CoreOpsWaitTime = time.Second * 5 // 数据操作超时时间
)

// 获取单件
func Instance() *OpLogger {
	if logInstance == nil {
		logInstance = new(OpLogger)
		logInstance.resChan = make(chan *msg.Msg, MsgChanCap)
		logInstance.smap = make(map[uint64]*dataSession, 10000)
	}
	return logInstance
}

// 初始化
func (ol *OpLogger) Initialize(cli *client.Client, logger *log.Logger) {
	if ol.inited || cli == nil || logger == nil {
		panic("ol already initialized or parameters aren't valid!")
	}

	// 校验客户端组件的通信目标是否是LOG集群
	if cli.GetDstSvrType() != uint32(msg.SvrType_LOG) {
		panic("invalid client type for ol!")
	}

	// 为客户端组件添加消息过滤管道，将所有数据操作返回结果接收到内部管道中
	cli.AddFilterChan(uint32(interior.MsgID_CORE_RES), ol.resChan)

	ol.cli = cli
	ol.logger = logger
	ol.srcSvrId = ol.cli.GetSrcSvrId()
	ol.dstSvrId = utils.ConvertFields2ServerIDNum(
		[]uint32{uint32(msg.SvrType_LOG), 0})
	ol.inited = true

	go utils.GoMain(ol.recvJob, nil)
}

// 收包协程
func (ol *OpLogger) recvJob() {
	for {
		// 收取回复
		res := <-ol.resChan

		// 校验会话是否超时
		ol.smLock.Lock()
		sid := res.Header.SHeader.GetDstSessionID()
		session, exists := ol.smap[sid]
		if !exists {
			ol.logger.Errorf(logTag,
				"got late response from data server, %d, %s", sid, res)
			ol.smLock.Unlock()
			continue
		}
		delete(ol.smap, sid)
		session.t.Stop()
		session.tq <- 0
		ol.smLock.Unlock()

		// 处理回复
		body := new(interior.CoreResponse)
		session.res = body
		err := proto.Unmarshal(res.Body, body)
		if err != nil {
			ol.logger.Errorf(logTag, "decode response failed, %s", err)
			continue
		}
	}
}

// 发送日志
func (ol *OpLogger) SendLog(log interface{}) {
	if !ol.inited {
		ol.logger.Errorf(logTag, "not initialized yet!")
		return
	}

	// 解析日志格式，并给对应的日志成员赋值
	msgBody := new(interior.OpLogNotify)
	bodyValue := reflect.ValueOf(msgBody).Elem()
	bodyType := bodyValue.Type()
	logValue := reflect.ValueOf(log)
	logType := logValue.Type()

	// 检查传入的参数是否是指针类型
	if logType.Kind() != reflect.Ptr {
		ol.logger.Errorf(logTag, "not a pointer type!")
		return
	}

	n := bodyType.NumField()
	hit := false
	for i := 0; i < n; i++ {
		if logType == bodyType.Field(i).Type {
			hit = true
			fieldValue := bodyValue.Field(i)
			fieldValue.Set(logValue)
			break
		}
	}
	if !hit {
		ol.logger.Errorf(logTag, "unsupported type %s", reflect.TypeOf(log))
		return
	}

	// 向日志集群发送通知
	body, err := proto.Marshal(msgBody)
	if err != nil {
		ol.logger.Errorf(logTag, "encode msg body failed, %s", err)
		return
	}
	logMsg := new(msg.Msg)
	logMsg.Body = body
	logMsg.Header = new(msg.MsgHeader)
	logMsg.Header.Id = proto.Int32(0)
	logMsg.Header.PHeader = new(msg.ProtoHeader)
	logMsg.Header.PHeader.SessionType = proto.Uint32(0)
	logMsg.Header.PHeader.Flag = proto.Uint32(0)
	logMsg.Header.PHeader.Version = proto.Uint32(uint32(msg.ProtoVersion_PV))
	logMsg.Header.SHeader = new(msg.SessionHeader)
	logMsg.Header.SHeader.SrcServiceID = &ol.srcSvrId
	logMsg.Header.SHeader.SrcSessionID = proto.Uint64(0)
	logMsg.Header.SHeader.DstServiceID = &ol.dstSvrId
	logMsg.Header.SHeader.DstSessionID = proto.Uint64(0)
	logMsg.Header.SHeader.TimeStamp = proto.Uint64(uint64(time.Now().UnixNano()))
	logMsg.Header.SHeader.RouteType = msg.RouteTypeID_RD.Enum()
	logMsg.Header.SHeader.Seq = proto.Uint32(0)
	logMsg.Header.SHeader.MsgID = proto.Uint32(uint32(interior.MsgID_OPLOG_NOTIFY))

	var data []byte
	data, err = proto.Marshal(logMsg)
	if err != nil {
		ol.logger.Errorf(logTag, "encode msg failed, %s, %s", err, msgBody)
		return
	}
	err = ol.cli.SendMsg(msg.RouteTypeID_RD, nil, data, nil)
	if err != nil {
		ol.logger.Errorf(logTag, "send msg failed, %s, %s", err, msgBody)
		return
	}

	return
}

func (ol *OpLogger) DoDataOps(opType interior.CoreOpType, selector, data bson.M,
	colname string, args ...string) {
	if !ol.inited {
		ol.logger.Errorf(logTag, "not initialized yet!")
		return
	}

	//参数检查
	if selector == nil {
		panic("invalid parameter!")
	}
	switch opType {
	case interior.CoreOpType_COT_READ:
	case interior.CoreOpType_COT_REMOVE:
	case interior.CoreOpType_COT_UPSERT:
		if data == nil {
			panic("invalid parameter!")
		}
	default:
		panic("invalid op type!")
	}

	dbname := "core"
	if len(args) > 0 {
		dbname = args[0]
	}
	msgBody := new(interior.CoreRequest)
	msgBody.DBName = proto.String(dbname)
	msgBody.ColName = proto.String(colname)
	msgBody.Type = opType.Enum()
	msgBody.Selector, _ = bson.Marshal(selector)
	msgBody.Data, _ = bson.Marshal(data)

	// 向日志集群发送通知
	body, err := proto.Marshal(msgBody)
	if err != nil {
		ol.logger.Errorf(logTag, "encode msg body failed, %s, %s", err, msgBody)
		return
	}
	sid := uint64(ol.sessionIdGen.GenID())
	req := new(msg.Msg)
	req.Body = body
	req.Header = new(msg.MsgHeader)
	req.Header.Id = proto.Int32(0)
	req.Header.PHeader = new(msg.ProtoHeader)
	req.Header.PHeader.SessionType = proto.Uint32(0)
	req.Header.PHeader.Flag = proto.Uint32(0)
	req.Header.PHeader.Version = proto.Uint32(uint32(msg.ProtoVersion_PV))
	req.Header.SHeader = new(msg.SessionHeader)
	req.Header.SHeader.SrcServiceID = &ol.srcSvrId
	req.Header.SHeader.SrcSessionID = &sid
	req.Header.SHeader.DstServiceID = &ol.dstSvrId
	req.Header.SHeader.DstSessionID = proto.Uint64(0)
	req.Header.SHeader.TimeStamp = proto.Uint64(uint64(time.Now().UnixNano()))
	req.Header.SHeader.RouteType = msg.RouteTypeID_RD.Enum()
	req.Header.SHeader.Seq = proto.Uint32(0)
	req.Header.SHeader.MsgID = proto.Uint32(uint32(interior.MsgID_CORE_REQ))

	// 记录会话
	session := &dataSession{
		id: sid,
		tq: make(chan int, 1),
	}
	ol.smLock.Lock()
	ol.smap[sid] = session
	ol.smLock.Unlock()

	// 启动业务定时器
	session.t = time.NewTimer(CoreOpsWaitTime)
	go func() {
		select {
		case <-session.tq:
			ol.logger.Debugf(logTag, "session %d timer is normally destoryed.", sid)
			return
		case <-session.t.C:
		}
		ol.smLock.Lock()
		if _, exists := ol.smap[sid]; exists {
			delete(ol.smap, sid)
			ol.smLock.Unlock()

			// 通知操作超时
			ol.logger.Errorf(logTag, "session %d has been timeout!, msg %s", sid, msgBody)
			session.err = fmt.Errorf("data ops timeout")
		} else {
			ol.logger.Errorf(logTag, "can't find timeout session %d!, msg %s", sid, msgBody)
			ol.smLock.Unlock()
		}
	}()

	// 发送请求
	mb, me := proto.Marshal(req)
	if me != nil {
		ol.logger.Errorf(logTag, "encode req failed, %s, %s", me, msgBody)
		ol.smLock.Lock()
		delete(ol.smap, sid)
		ol.smLock.Unlock()
		return
	}
	err = ol.cli.SendMsg(msg.RouteTypeID_RD, nil, mb, nil)
	if err != nil {
		ol.logger.Errorf(logTag, "send req failed, %s, %s", err, msgBody)
		ol.smLock.Lock()
		delete(ol.smap, sid)
		ol.smLock.Unlock()
		return
	}
	return
}
