/*
实现消息映射:routine不安全
*/

package base

import (
	"github.com/golang/protobuf/proto"
	"xxgame.com/framework"
	msg "xxgame.com/types/proto"
	"xxgame.com/utils/errors"
)

const (
	UniformMsgID = 0 //通用消息id，代表所有消息
)

type MsgMap map[uint32]framework.MsgHandle

//初始化
func NewMsgMap() MsgMap {
	return make(MsgMap)
}

//注册
//参数msgId:消息id
//参数handle:处理消息的handle，传入的是指针
func (m MsgMap) Reg(msgId uint32, handle framework.MsgHandle) (int, error) {
	_, exist := m[msgId]
	if exist {
		return -1, errors.New("msg %d already registered", msgId)
	}

	m[msgId] = handle
	return 0, nil
}

//处理消息
func (m MsgMap) Process(ms *msg.Msg, c framework.MsgContext) (int, error, proto.Message) {

	h := ms.GetHeader()
	if h == nil {
		return -1, errors.New("decode header failed"), nil
	}

	sh := h.GetSHeader()
	if sh == nil {
		return -1, errors.New("decode session header failed"), nil
	}

	// 其他客户端 不允许发送为0的MsgID
	if sh.GetMsgID() == 0 {
		return -1, errors.New("session header msgid error(msgid == 0)"), nil
	}

	//根据消息id获取消息handle
	handle, exist := m[sh.GetMsgID()]
	if handle == nil || !exist {
		//若通用消息handle存在则用其处理所有的消息
		uniHandle, uniExist := m[UniformMsgID]
		if uniExist {
			res, e := uniHandle.Process(h, nil, ms.GetBody())
			if res != 0 || e != nil {
				return -1, errors.New("process uni msg %d failed ret %d: %s",
					sh.GetMsgID(), res, e.Error()), nil
			}
			return 0, nil, nil
		} else {
			return -1, errors.New("get msg %d handle failed", sh.GetMsgID()), nil
		}
	}

	//创建消息体
	body := handle.NewMsg()
	//解码消息体
	if e := proto.Unmarshal(ms.GetBody(), body); e != nil {
		return -1, errors.New("decode msg %d body failed: %s", sh.GetMsgID(), e.Error()), body
	}

	//设置消息上下文
	handle.SetContext(c)

	//调用处理函数
	res, e := handle.Process(h, body, ms.GetBody())
	return res, e, body
}
