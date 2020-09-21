package datasdk

import (
	"encoding/binary"
	"fmt"
	"sync"
	"time"
)

import (
	"github.com/golang/protobuf/proto"
	dao "xxgame.com/component/dao/types"
	"xxgame.com/component/log"
	"xxgame.com/component/net/client"
	"xxgame.com/component/uuid"
	msg "xxgame.com/types/proto"
	"xxgame.com/types/proto/interior"
	"xxgame.com/utils"
	"xxgame.com/utils/errors"
)

// 数据集操作规则
type Rule struct {
	DataID       uint64             // 数据id
	IndexType    interior.IndexType // 索引类型
	IndexNumber2 uint64             // 二级数值索引值,默认值为0
	DataType     interior.DataType  // 数据类型
	OpMask       uint64             // 操作类型集合掩码
}

var IndexNumber2Default = proto.Uint64(0)

// 会话类型
type dataSession struct {
	id   uint64                   // 会话ID
	t    *time.Timer              // 会话定时器
	tq   chan int                 // 用于通知定时器已销毁
	do   DataOperator             // 异步操作回调对象
	res  *interior.DataOpResponse // 同步返回的数据库操作结果
	err  error                    // 同步操作错误
	sync chan int                 // 同步等待管道
}

// SDK实例类型
type SDK struct {
	inited       bool                    // 初始化标志
	cli          *client.Client          // 用于访问datalogic服务的客户端组件
	logger       *log.Logger             // 日志对象
	rmap         map[string]Rule         // 针对每个data id 加二级索引值保存其操作规则的拷贝
	rmLock       sync.Mutex              // 保护规则注册表
	resChan      chan *msg.Msg           // 消息接收管道
	reqChan      chan *msg.Msg           // 请求发送管道
	sessionIdGen uuid.UuidGen            // 会话id生成器
	smap         map[uint64]*dataSession // 会话映射表
	smLock       sync.Mutex              // 保护会话映射表
	srcSvrId     uint32                  // 源服务id
	dstSvrId     uint32                  // 数据服务id
	callbackChan chan func()             //回调队列
}

// 操作类型掩码
const (
	OP_NEW = 1 << uint64(interior.OpType_New)
	OP_GET = 1 << uint64(interior.OpType_Get)
	OP_ADD = 1 << uint64(interior.OpType_Add)
	OP_SUB = 1 << uint64(interior.OpType_Sub)
	OP_SET = 1 << uint64(interior.OpType_Set)
	OP_DEL = 1 << uint64(interior.OpType_Del)
)

// 数据操作错误返回
var (
	SDKERR_INVALID_PARAM    = errors.New("invalid parameter!")
	SDKERR_INVALID_OPMASK   = errors.New("illegal op mask!")
	SDKERR_INVALID_DATAID   = errors.New("data id not registered!")
	SDKERR_INVALID_ACTION   = errors.New("operation not permitted!")
	SDKERR_INVALID_DATATYPE = errors.New("invalid data type!")
	SDKERR_INVALID_INDEX    = errors.New("invalid data index!")
	SDKERR_INVALID_32INT    = errors.New("invalid 32bit integer!")
	SDKERR_INVALID_64INT    = errors.New("invalid 64bit integer!")
	SDKERR_INVALID_BUFFER   = errors.New("null data buffer!")
)

var (
	MsgChanCap      = 1000             // 消息管道容量
	DataOpsWaitTime = time.Second * 30 // 数据操作超时时间
	logTag          = "datasdk"
	sdkInstance     *SDK
	RecvJobNum      = 1
)

// 获取单件
func Instance() *SDK {
	if sdkInstance == nil {
		sdkInstance = new(SDK)
		sdkInstance.rmap = make(map[string]Rule, 100)
		sdkInstance.resChan = make(chan *msg.Msg, MsgChanCap)
		sdkInstance.reqChan = make(chan *msg.Msg, MsgChanCap)
		sdkInstance.smap = make(map[uint64]*dataSession, 10000)
		sdkInstance.callbackChan = make(chan func(), 1000)
	}
	return sdkInstance
}

func (sdk *SDK) GetSvrType() int32 {
	return int32(msg.SvrType_COREDATA_DATA)
}

// 初始化
func (sdk *SDK) Initialize(cli *client.Client, logger *log.Logger) {
	if sdk.inited || cli == nil || logger == nil {
		panic("sdk already initialized or parameters aren't valid!")
	}

	// 校验客户端组件的通信目标是否是coredata logic集群
	if cli.GetDstSvrType() != uint32(sdk.GetSvrType()) {
		panic("invalid client type for sdk!")
	}

	// 为客户端组件添加消息过滤管道，将所有数据操作返回结果接收到内部管道中
	cli.AddFilterChan(uint32(interior.MsgID_DATAOP_RES), sdk.resChan)

	sdk.cli = cli
	sdk.logger = logger
	sdk.srcSvrId = sdk.cli.GetSrcSvrId()
	sdk.dstSvrId = utils.ConvertFields2ServerIDNum(
		[]uint32{uint32(sdk.GetSvrType()), 0})
	sdk.inited = true

	for i := 0; i < RecvJobNum; i++ {
		go utils.GoMain(sdk.recvJob, nil)
	}

	go utils.GoMain(sdk.callbackJob, nil)
}

//回调协程
func (sdk *SDK) callbackJob() {
	for {
		callback := <-sdk.callbackChan
		callback()
	}
}

// 收包协程
func (sdk *SDK) recvJob() {
	for {
		// 收取回复
		res := <-sdk.resChan

		// 校验会话是否超时
		sdk.smLock.Lock()
		sid := res.Header.SHeader.GetDstSessionID()
		session, exists := sdk.smap[sid]
		if !exists {
			sdk.logger.Errorf(logTag,
				"got late response from data server, %d, %s", sid, res)
			sdk.smLock.Unlock()
			continue
		}
		delete(sdk.smap, sid)
		session.t.Stop()
		session.tq <- 0
		sdk.smLock.Unlock()

		// 处理回复
		var body *interior.DataOpResponse
		if session.res != nil {
			body = session.res
		} else {
			body = new(interior.DataOpResponse)
		}
		err := proto.Unmarshal(res.Body, body)
		if err != nil {
			sdk.logger.Errorf(logTag, "decode response failed, %s", err)
			continue
		}
		if body.ResultID == nil || body.DataOpResultValues == nil ||
			len(body.DataOpResultValues) == 0 {
			sdk.logger.Errorf(logTag, "invalid response body %s", body)
			continue
		}
		if session.res != nil {
			session.sync <- 0 //同步返回
		} else {
			sdk.callbackChan <- func() {
				session.do.OnFinished(body) //异步回调
			}
		}
	}
}

// 注册数据ID、类型及其允许的操作集合
func (sdk *SDK) RegRules(rs []Rule) error {
	if !sdk.inited {
		panic("sdk not initialized yet!")
	}

	// 检查操作类型是否合法
	for _, r := range rs {
		if r.DataType == interior.DataType_Buffer {
			if (r.OpMask&OP_ADD) != 0 || (r.OpMask&OP_SUB) != 0 {
				return SDKERR_INVALID_OPMASK
			}
		}
	}

	// 加入注册表
	sdk.rmLock.Lock()
	defer sdk.rmLock.Unlock()
	for _, r := range rs {
		if r.IndexNumber2 != *IndexNumber2Default {
			sdk.rmap[fmt.Sprintf("%v.%v", r.DataID, r.IndexNumber2)] = r
		}
		sdk.rmap[fmt.Sprintf("%v", r.DataID)] = r
	}

	return nil
}

// 检查操作合法性
func (sdk *SDK) CheckOps(dataOps []*interior.DataOp) (err error, indexType interior.IndexType, routeKey []byte) {
	var dataNodeId, dataNodeId2 int
	for i, do := range dataOps {
		if do.OpType == nil || do.Data == nil {
			err = SDKERR_INVALID_PARAM
			return
		}
		ops := *do.OpType
		data := do.Data

		// 查找data id对应的操作规则
		sdk.rmLock.Lock()
		k := ""
		if data.NumIndex2 != nil {
			k = fmt.Sprintf("%v.%v", data.GetDataID(), data.GetNumIndex2())
		} else {
			k = fmt.Sprintf("%v", data.GetDataID())
		}
		rule, exists := sdk.rmap[k]
		if !exists {
			sdk.rmLock.Unlock()
			err = SDKERR_INVALID_DATAID
			return
		}
		sdk.rmLock.Unlock()
		do.IndexType = &rule.IndexType
		sdk.logger.Debugf(logTag,
			"find data rule,rule key = %s, dataid = %s, indextype = %s ",
			k, rule.DataID, rule.IndexType)

		// 检查路由数据节点是否一致
		indexType = rule.IndexType
		if indexType == interior.IndexType_Number1 ||
			indexType == interior.IndexType_Number2 {
			routeKey = make([]byte, 8)
			binary.BigEndian.PutUint64(routeKey, *data.NumIndex1)
			dataNodeId2 = utils.TMHashNum(dao.TableNum, routeKey)
		} else if indexType == interior.IndexType_String {
			routeKey = data.StrIndex[:]
			dataNodeId2 = utils.TMHashStr(dao.TableNum, routeKey)
		} else {
			panic("invalid index type!")
		}
		if i == 0 {
			dataNodeId = dataNodeId2
		} else if dataNodeId != dataNodeId2 {
			sdk.logger.Errorf(logTag, "inconsistent data node destination!")
			err = SDKERR_INVALID_PARAM
			return
		}

		// 检查操作是否合法
		if rule.OpMask&(1<<uint64(ops)) == 0 {
			err = SDKERR_INVALID_ACTION
			return
		}
		sdk.logger.Debugf(logTag,
			"compare data type , data.DataType = %s, rule.DataType = %s",
			data.DataType, data.GetDataType())

		// 检查数据类型是否匹配
		if data.DataType == nil || rule.DataType != data.GetDataType() {
			sdk.logger.Errorf(logTag, "unmatched data type!")
			err = SDKERR_INVALID_DATATYPE
			return
		}

		// 检查数据索引是否提供完整
		if indexType == interior.IndexType_Number1 && data.NumIndex1 == nil {
			sdk.logger.Errorf(logTag, "number index1 should be given!")
			err = SDKERR_INVALID_INDEX
			return
		}
		if indexType == interior.IndexType_Number2 && ops != interior.OpType_Get &&
			(data.NumIndex1 == nil || data.NumIndex2 == nil) {
			sdk.logger.Errorf(logTag, "number index1 and index2 should be given!")
			err = SDKERR_INVALID_INDEX
			return
		}
		if indexType == interior.IndexType_String && len(data.StrIndex) == 0 {
			sdk.logger.Errorf(logTag, "invalid string index!")
			err = SDKERR_INVALID_INDEX
			return
		}

		// 检查数据是否完整
		if ops != interior.OpType_Get && ops != interior.OpType_Del {
			switch *data.DataType {
			case interior.DataType_Int32, interior.DataType_UInt32:
				if len(data.DataBuffer) != 4 {
					err = SDKERR_INVALID_32INT
					return
				}
			case interior.DataType_Int64, interior.DataType_UInt64:
				if len(data.DataBuffer) != 8 {
					err = SDKERR_INVALID_64INT
					return
				}
			case interior.DataType_Buffer:
				if len(data.DataBuffer) == 0 {
					err = SDKERR_INVALID_BUFFER
					return
				}
			default:
				panic(fmt.Sprintf("unknow data type %v", *data.DataType))
			}
		}
	}

	return
}

// 发送异步数据操作请求，暂不支持向多个数据节点同时发送请求，
// 所有操作最终路由的数据节点必须保持一致
//	dataOps: 数据操作请求
//	do: 数据操作回调接口实现
func (sdk *SDK) StartDataOps(dataOps []*interior.DataOp, do DataOperator) error {
	if !sdk.inited {
		panic("sdk not initialized yet!")
	}

	if dataOps == nil || len(dataOps) == 0 {
		return SDKERR_INVALID_PARAM
	}

	checkErr, indexType, routeKey := sdk.CheckOps(dataOps)
	if checkErr != nil {
		return checkErr
	}

	// 构建发送请求
	reqBody := new(interior.DataOpRequest)
	reqBody.DataOpValues = dataOps
	body, err := proto.Marshal(reqBody)
	if err != nil {
		sdk.logger.Errorf(logTag, "encode req body failed, %s", err)
		return err
	}
	sid := uint64(sdk.sessionIdGen.GenID())
	req := new(msg.Msg)
	req.Body = body
	req.Header = new(msg.MsgHeader)
	req.Header.Id = proto.Int32(0)
	req.Header.PHeader = new(msg.ProtoHeader)
	req.Header.PHeader.SessionType = proto.Uint32(0)
	req.Header.PHeader.Flag = proto.Uint32(0)
	req.Header.PHeader.Version = proto.Uint32(uint32(msg.ProtoVersion_PV))
	req.Header.SHeader = new(msg.SessionHeader)
	req.Header.SHeader.SrcServiceID = &sdk.srcSvrId
	req.Header.SHeader.SrcSessionID = &sid
	req.Header.SHeader.DstServiceID = &sdk.dstSvrId
	req.Header.SHeader.DstSessionID = proto.Uint64(0)
	req.Header.SHeader.TimeStamp = proto.Uint64(uint64(time.Now().UnixNano()))
	switch indexType {
	case interior.IndexType_Number1, interior.IndexType_Number2:
		req.Header.SHeader.RouteType = msg.RouteTypeID_RTN.Enum()
	case interior.IndexType_String:
		req.Header.SHeader.RouteType = msg.RouteTypeID_RT.Enum()
	}
	req.Header.SHeader.RouteKey = routeKey
	req.Header.SHeader.Seq = proto.Uint32(0)
	req.Header.SHeader.MsgID = proto.Uint32(uint32(interior.MsgID_DATAOP_REQ))

	// 记录会话
	session := &dataSession{
		id: sid,
		do: do,
		tq: make(chan int, 1),
	}
	sdk.smLock.Lock()
	sdk.smap[sid] = session
	sdk.smLock.Unlock()

	// 启动业务定时器
	session.t = time.NewTimer(DataOpsWaitTime)
	go func() {
		select {
		case <-session.tq:
			sdk.logger.Debugf(logTag, "session %d timer is normally destoryed.", sid)
			return
		case <-session.t.C:
		}
		sdk.smLock.Lock()
		if _, exists := sdk.smap[sid]; exists {
			delete(sdk.smap, sid)
			sdk.smLock.Unlock()

			// 通知其操作失败
			sdk.logger.Errorf(logTag, "session %d has been timeout!", sid)
			sdk.callbackChan <- func() {
				session.do.OnFinished(&interior.DataOpResponse{
					ResultID: interior.DOResultID_LogicTimeout.Enum(),
				})
			}
		} else {
			sdk.logger.Errorf(logTag, "can't find timeout session %d!", sid)
			sdk.smLock.Unlock()
		}
	}()

	// 发送请求
	data, me := proto.Marshal(req)
	if me != nil {
		sdk.logger.Errorf(logTag, "encode req failed, %s", me)
		sdk.smLock.Lock()
		delete(sdk.smap, sid)
		sdk.smLock.Unlock()
		return err
	}
	err = sdk.cli.SendMsg(msg.RouteTypeID_RD, nil, data, nil)
	if err != nil {
		sdk.logger.Errorf(logTag, "send req failed, %s", err)
		sdk.smLock.Lock()
		delete(sdk.smap, sid)
		sdk.smLock.Unlock()
		return err
	}
	sdk.logger.Debugf(logTag, "session %d start send test!", sid)
	return nil
}

// 发送同步数据操作请求，暂不支持向多个数据节点同时发送请求，
// 所有操作最终路由的数据节点必须保持一致
//	dataOps: 数据操作请求
//	result: 数据操作返回
func (sdk *SDK) DoDataOps(dataOps []*interior.DataOp) (error, *interior.DataOpResponse) {
	if !sdk.inited {
		panic("sdk not initialized yet!")
	}

	if dataOps == nil || len(dataOps) == 0 {
		return SDKERR_INVALID_PARAM, nil
	}

	checkErr, indexType, routeKey := sdk.CheckOps(dataOps)
	if checkErr != nil {
		return checkErr, nil
	}

	// 构建发送请求
	reqBody := new(interior.DataOpRequest)
	reqBody.DataOpValues = dataOps
	body, err := proto.Marshal(reqBody)
	if err != nil {
		sdk.logger.Errorf(logTag, "encode req body failed, %s", err)
		return err, nil
	}
	sid := uint64(sdk.sessionIdGen.GenID())
	req := new(msg.Msg)
	req.Body = body
	req.Header = new(msg.MsgHeader)
	req.Header.Id = proto.Int32(0)
	req.Header.PHeader = new(msg.ProtoHeader)
	req.Header.PHeader.SessionType = proto.Uint32(0)
	req.Header.PHeader.Flag = proto.Uint32(0)
	req.Header.PHeader.Version = proto.Uint32(uint32(msg.ProtoVersion_PV))
	req.Header.SHeader = new(msg.SessionHeader)
	req.Header.SHeader.SrcServiceID = &sdk.srcSvrId
	req.Header.SHeader.SrcSessionID = &sid
	req.Header.SHeader.DstServiceID = &sdk.dstSvrId
	req.Header.SHeader.DstSessionID = proto.Uint64(0)
	req.Header.SHeader.TimeStamp = proto.Uint64(uint64(time.Now().UnixNano()))
	switch indexType {
	case interior.IndexType_Number1, interior.IndexType_Number2:
		req.Header.SHeader.RouteType = msg.RouteTypeID_RTN.Enum()
	case interior.IndexType_String:
		req.Header.SHeader.RouteType = msg.RouteTypeID_RT.Enum()
	}
	req.Header.SHeader.RouteKey = routeKey
	req.Header.SHeader.Seq = proto.Uint32(0)
	req.Header.SHeader.MsgID = proto.Uint32(uint32(interior.MsgID_DATAOP_REQ))

	// 记录会话
	result := new(interior.DataOpResponse)
	session := &dataSession{
		id:   sid,
		tq:   make(chan int, 1),
		res:  result,
		sync: make(chan int, 1),
	}
	sdk.smLock.Lock()
	sdk.smap[sid] = session
	sdk.smLock.Unlock()

	// 启动业务定时器
	session.t = time.NewTimer(DataOpsWaitTime)
	go func() {
		select {
		case <-session.tq:
			sdk.logger.Debugf(logTag, "session %d timer is normally destoryed.", sid)
			return
		case <-session.t.C:
		}
		sdk.smLock.Lock()
		if _, exists := sdk.smap[sid]; exists {
			delete(sdk.smap, sid)
			sdk.smLock.Unlock()

			// 通知操作超时
			sdk.logger.Errorf(logTag, "session %d has been timeout!", sid)
			session.err = fmt.Errorf("data ops timeout")
			session.sync <- 0
		} else {
			sdk.logger.Errorf(logTag, "can't find timeout session %d!", sid)
			sdk.smLock.Unlock()
		}
	}()

	// 发送请求
	data, me := proto.Marshal(req)
	if me != nil {
		sdk.logger.Errorf(logTag, "encode req failed, %s", me)
		sdk.smLock.Lock()
		delete(sdk.smap, sid)
		sdk.smLock.Unlock()
		return me, nil
	}
	err = sdk.cli.SendMsg(msg.RouteTypeID_RD, nil, data, nil)
	if err != nil {
		sdk.logger.Errorf(logTag, "send req failed, %s", err)
		sdk.smLock.Lock()
		delete(sdk.smap, sid)
		sdk.smLock.Unlock()
		return err, nil
	}

	// 等待操作返回
	<-session.sync
	return session.err, session.res
}
