//基础服务实现
package base

import (
	"encoding/binary"
	"fmt"
	"net/http"
	"reflect"
)

import (
	"github.com/golang/protobuf/proto"
	"xxgame.com/component/net"
	"xxgame.com/component/webserver"
	"xxgame.com/framework"
	msg "xxgame.com/types/proto"
	"xxgame.com/types/proto/config"
	"xxgame.com/utils/errors"
)

//基础服务
type BaseWebService struct {
	BaseService
	WM                *webserver.WebNetMgr //WEB网络管理器
	WebSockServeMutex *http.ServeMux       //websockect的ServeMux
	WebCfg            *config.WebCfg
}

//实现Service接口:初始化函数
func (s *BaseWebService) Init(FW *framework.FrameWork) (int, error) {
	s.WM = new(webserver.WebNetMgr)
	s.WebSockServeMutex = http.NewServeMux()

	res, e := s.BaseService.Init(FW)
	if e != nil {
		return res, e
	}

	e = s.WM.Init(FW.Service, s.OnNewWebMsg, s.Log)
	return res, e
}

func (s *BaseWebService) HttpServicePretreater() {
	// 默认提供web-mobile目录作为加载目录，暂不支持配置
	s.WebSockServeMutex.Handle("/", http.FileServer(http.Dir("web-mobile/")))
	s.WebSockServeMutex.HandleFunc("/ws", s.WM.HttpHandleFunc)
}

func (s *BaseWebService) OnNewWebMsg(node *webserver.WebNetNode, buff []byte) error {
	if len(buff) < net.HeadSize {
		s.Log.Errorf(LogTag, "not enough size, close invalid connection, %v", node)
		node.Close()
		return fmt.Errorf("not enough size")
	}

	size := binary.BigEndian.Uint32(buff[0:4])
	if size >= net.SocketBufSize {
		s.Log.Errorf(LogTag, "packet size too large %d, close invalid connection, %v", size, node)
		node.Close()
		return fmt.Errorf("packet size too large")
	}

	magic := binary.BigEndian.Uint16(buff[4:6])
	if magic != uint16(net.MagicNumber) {
		s.Log.Errorf(LogTag, "invalid magic number %x", magic)
		node.Close()
		return fmt.Errorf("invalid magic number")
	}

	msgtype := binary.BigEndian.Uint16(buff[6:net.HeadSize])
	_ = msgtype
	// 解码消息并加添加nethead头信息
	var m msg.Msg
	if err := proto.Unmarshal(buff[net.HeadSize:], &m); err != nil {
		s.Log.Errorf(LogTag, "decode msg failed, %s", err)
		return err
	}
	// 在Auth协议直接发送过来时 上面的proto.Umarshal不会报错，但是此时m.GetHeader为nil
	header := m.GetHeader()
	if header == nil {
		err := "decode header failed"
		s.Log.Errorf(LogTag, err)
		return errors.New(err)
	}

	sh := header.GetSHeader()
	if sh == nil {
		return errors.New("decode session header failed")
	}

	header.NHeader = s.Svr.GenNetHead2(node.RemoteAddr(), node.GetId(), nil)
	data, err := proto.Marshal(&m)
	if err != nil {
		s.Log.Errorf(LogTag, "marshal msg failed, %s", err)
		return err
	}

	s.Log.Debugf(LogTag, "msg is %v", m)

	//交给用户定义的接收信息处理
	if e := s.FW.Service.ProcessMsg(data); e != nil {
		s.Log.Errorf(LogTag, "process msg failed: %s", e)
		return e
	}
	return nil
}

//发送回复消息
func (s *BaseWebService) W2C_BYTE(header *msg.MsgHeader, msgId uint32, body []byte) (int, error) {
	m := new(msg.Msg)
	m.Header = new(msg.MsgHeader)

	var connID uint32
	nh := header.GetNHeader()
	if nh != nil {
		msg.ConstructResHeader(header, m.Header, msgId, true)
		if s.Svr.IsGateWay() {
			connID = nh.GetSessionID()
		} else {
			connID = nh.GetServiceID()
		}
		s.Log.Debugf(LogTag, "net head is not nil,connID = %v", connID)
	} else {
		msg.ConstructResHeader(header, m.Header, msgId, false)
		connID = m.GetHeader().GetSHeader().GetDstServiceID()
		s.Log.Debugf(LogTag, "net head is nil,connID = %v", connID)
	}

	//	//编码消息体
	//	var e error
	//	m.Body, e = proto.Marshal(body)
	//	if e != nil {
	//		return -1, e
	//	}

	m.Body = body

	//编码消息
	var buff []byte
	var e error
	buff, e = proto.Marshal(m)
	if e != nil {
		return -1, e
	}

	//增加头部
	packet := make([]byte, len(buff)+net.HeadSize)
	binary.BigEndian.PutUint32(packet[0:4], uint32(len(buff)+net.HeadSize))
	binary.BigEndian.PutUint16(packet[4:6], uint16(net.MagicNumber))
	binary.BigEndian.PutUint16(packet[6:net.HeadSize], uint16(msg.MsgType_NORMAL))
	copy(packet[net.HeadSize:], buff)

	//找到连接并发送
	conn := s.WM.GetNode(connID)
	if conn == nil {
		return -1, errors.New("can't find connection by service id %d", connID)
	}

	e = conn.Send(packet)
	if e != nil {
		return -1, e
	}

	return 0, nil
}

//发送回复消息
func (s *BaseWebService) W2C(header *msg.MsgHeader, msgId uint32, body proto.Message) (int, error) {
	m := new(msg.Msg)
	m.Header = new(msg.MsgHeader)

	var connID uint32
	nh := header.GetNHeader()
	if nh != nil {
		msg.ConstructResHeader(header, m.Header, msgId, true)
		if s.Svr.IsGateWay() {
			connID = nh.GetSessionID()
		} else {
			connID = nh.GetServiceID()
		}
		//s.Log.Debugf(LogTag, "net head is not nil,connID = %v", connID)
	} else {
		msg.ConstructResHeader(header, m.Header, msgId, false)
		connID = m.GetHeader().GetSHeader().GetDstServiceID()
		//s.Log.Debugf(LogTag, "net head is nil,connID = %v", connID)
	}

	//编码消息体
	var e error
	m.Body, e = proto.Marshal(body)
	if e != nil {
		return -1, e
	}

	//编码消息
	var buff []byte
	buff, e = proto.Marshal(m)
	if e != nil {
		return -1, e
	}

	//增加头部
	packet := make([]byte, len(buff)+net.HeadSize)
	binary.BigEndian.PutUint32(packet[0:4], uint32(len(buff)+net.HeadSize))
	binary.BigEndian.PutUint16(packet[4:6], uint16(net.MagicNumber))
	binary.BigEndian.PutUint16(packet[6:net.HeadSize], uint16(msg.MsgType_NORMAL))
	copy(packet[net.HeadSize:], buff)

	//找到连接并发送
	conn := s.WM.GetNode(connID)
	if conn == nil {
		return -1, errors.New("can't find connection by service id %d", connID)
	}

	e = conn.Send(packet)
	if e != nil {
		return -1, e
	}

	s.Log.Debugf(LogTag, "send client msg %v %v %v", m.GetHeader(), reflect.TypeOf(body).String(), body)
	return 0, nil
}
func (s *BaseWebService) RegisterCfg() (int, error) {
	_, e := s.BaseService.RegisterCfg()
	if e != nil {
		s.Log.Errorf("service", "register cfg failed, %s", e)
		panic(e.Error())
		return -1, e
	}

	res, e := s.Cfg.Register(
		"web",
		s.Proc.DefaultDeployDir+"/"+"web.cfg",
		true,
		func() proto.Message { return new(config.WebCfg) },
		nil,
		func(cfgName string, msg proto.Message) (int, error) {
			var ok bool
			s.WebCfg, ok = msg.(*config.WebCfg)
			if !ok {
				panic("load web cfg err")
			}
			return 0, nil
		})
	if e != nil {
		return res, e
	}
	return 0, nil
}

func (s *BaseWebService) SetupNetwork() (int, error) {
	//启动websocket端口
	temp := s.Cfg.Get("web")
	if temp == nil {
		return -1, fmt.Errorf("can't get cfg web")
	}
	cfg, ok := temp.(*config.WebCfg)
	if !ok {
		return -1, fmt.Errorf("can't get cfg web")
	}

	s.FW.Service.HttpServicePretreater()

	go http.ListenAndServe(cfg.GetAddr(), s.WebSockServeMutex)

	return s.BaseService.SetupNetwork()
}
