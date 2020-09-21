package reportdata

//上报数据到

import (
	"time"
)

import (
	"github.com/golang/protobuf/proto"
	"xxgame.com/component/log"
	"xxgame.com/component/net/client"
	msg "xxgame.com/types/proto"
	"xxgame.com/utils"
	"xxgame.com/utils/errors"
)

// 访问状态服务接口
type StatusWrapper struct {
	inited   bool           // 初始化标志
	cli      *client.Client // 用于访问LOG服务的客户端组件
	logger   *log.Logger    // 系统日志对象
	srcSvrId uint32         // 源服务id
	dstSvrId uint32         // 日志服务id
}

var (
	statusWrapperTag      = "reportdata"
	statusWrapperInstance *StatusWrapper
)

// 获取单件
func Instance() *StatusWrapper {
	if statusWrapperInstance == nil {
		statusWrapperInstance = new(StatusWrapper)
	}
	return statusWrapperInstance
}

// 初始化
func (ol *StatusWrapper) Initialize(cli *client.Client, logger *log.Logger) {
	if ol.inited || cli == nil || logger == nil {
		panic("ol already initialized or parameters aren't valid!")
	}

	// 校验客户端组件的通信目标是否是LOG集群
	if cli.GetDstSvrType() != uint32(msg.SvrType_StatusKeeper) {
		panic("invalid client type for ol!")
	}

	ol.cli = cli
	ol.logger = logger
	ol.srcSvrId = ol.cli.GetSrcSvrId()
	ol.dstSvrId = utils.ConvertFields2ServerIDNum(
		[]uint32{uint32(msg.SvrType_StatusKeeper), 0})
	ol.inited = true
}

// 发送数据
func (ol *StatusWrapper) SendMsg(msgID uint32, msgBody proto.Message) error {
	if !ol.inited {
		ol.logger.Errorf(statusWrapperTag, "not initialized yet!")
		return errors.New("not initialized yet!")
	}

	// 向日志集群发送通知
	body, err := proto.Marshal(msgBody)
	if err != nil {
		ol.logger.Errorf(statusWrapperTag, "encode msg body failed, %s", err)
		return err
	}
	Msg := new(msg.Msg)
	Msg.Body = body
	Msg.Header = new(msg.MsgHeader)
	Msg.Header.Id = proto.Int32(0)
	Msg.Header.PHeader = new(msg.ProtoHeader)
	Msg.Header.PHeader.SessionType = proto.Uint32(0)
	Msg.Header.PHeader.Flag = proto.Uint32(0)
	Msg.Header.PHeader.Version = proto.Uint32(uint32(msg.ProtoVersion_PV))
	Msg.Header.SHeader = new(msg.SessionHeader)
	Msg.Header.SHeader.SrcServiceID = &ol.srcSvrId
	Msg.Header.SHeader.SrcSessionID = proto.Uint64(0)
	Msg.Header.SHeader.DstServiceID = &ol.dstSvrId
	Msg.Header.SHeader.DstSessionID = proto.Uint64(0)
	Msg.Header.SHeader.TimeStamp = proto.Uint64(uint64(time.Now().UnixNano()))
	Msg.Header.SHeader.RouteType = msg.RouteTypeID_RD.Enum()
	Msg.Header.SHeader.Seq = proto.Uint32(0)
	Msg.Header.SHeader.MsgID = proto.Uint32(uint32(msgID))

	var data []byte
	data, err = proto.Marshal(Msg)
	if err != nil {
		ol.logger.Errorf(statusWrapperTag, "encode msg failed, %s", err)
		return err
	}
	err = ol.cli.SendMsg(msg.RouteTypeID_RD, nil, data, nil)
	if err != nil {
		ol.logger.Errorf(statusWrapperTag, "send msg failed, %s", err)
		return err
	}

	return nil
}
