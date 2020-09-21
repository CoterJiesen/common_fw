/*
基础服务框架
*/
package framework

import (
	"flag"
	"os"
	"runtime/pprof"
)

import (
	"github.com/golang/protobuf/proto"
	"xxgame.com/component/net"
	"xxgame.com/component/process"
	"xxgame.com/component/timer"
	"xxgame.com/component/webserver"
	msg "xxgame.com/types/proto"
	"xxgame.com/utils"
)

//服务框架
type FrameWork struct {
	Service Service //服务接口
}

//框架实例
var (
	fw                 *FrameWork
	PerfProfileEnabled = flag.Bool("pprof", false, "enable cpu/heap profiler")
)

func init() {
	fw = new(FrameWork)
}

//获取服务框架实例
func Instance() *FrameWork {
	return fw
}

//设置服务接口
func (fw *FrameWork) SetService(s Service) {
	fw.Service = s
}

//启动服务(除非服务退出，该函数永远不返回)
func (fw *FrameWork) Run() {
	flag.Parse()

	//判断是否打开性能分析器
	if *PerfProfileEnabled {
		cf, _ := os.Create("cpu.pprof")
		defer cf.Close()
		err := pprof.StartCPUProfile(cf)
		if err != nil {
			panic(err)
		}
		defer pprof.StopCPUProfile()

		hf, _ := os.Create("heap.pprof")
		err = pprof.WriteHeapProfile(hf)
		if err != nil {
			panic(err)
		}

		bf, _ := os.Create("block.pprof")
		defer bf.Close()
		err = pprof.Lookup("block").WriteTo(bf, 0)
		if err != nil {
			panic(err)
		}
	}

	//初始化服务接口
	_, e := fw.Service.Init(fw)
	if e != nil {
		panic(e.Error())
	}

	//调用服务主循环函数，该函数应该一直循环直到收到命令退出
	utils.GoMain(fw.Service.MainLoop, func() {
		fw.Service.OnPanic()
	})
}

//消息通知接口
type MsgProcessor interface {
	OnNewMsg(buff []byte) error
}

//服务接口
type Service interface {
	MsgProcessor
	webserver.WebNetEvent
	Init(*FrameWork) (int, error)                                //初始化
	RegisterCfg() (int, error)                                   //注册配置
	SetLogLevel()                                                //设置日志等级
	SetupNetwork() (int, error)                                  //启动网络
	ProcessHttpCmd(h *process.HttpContext)                       //处理http命令
	ProcessTimer(tn *timer.TimeoutNotify)                        //处理定时器超时
	ProcessMsg(buff []byte) error                                //处理消息
	OnReload()                                                   //重载
	OnExit()                                                     //退出
	OnPanic()                                                    //主线程panic
	OnNetDisconn(conn *net.Conn)                                 //网络连接异常断开
	MainLoop()                                                   //主循环
	RegisterMsgHandle()                                          //注册所有消息处理
	RegOneMsgHandle(msgId uint32, handle MsgHandle) (int, error) //注册一个消息处理
	HttpServicePretreater()                                      //Http预处理
	ProcessMsgSingleOrMultiple() bool                            //处理消息使用多线程还是单线程
	PreProcessMsg(msg *msg.Msg) error
}

//消息上下文接口
type MsgContext interface {
}

//消息处理接口
type MsgHandle interface {
	Process(header *msg.MsgHeader, body proto.Message, rawBody []byte) (int, error) //处理消息的函数
	SetContext(c MsgContext)                                                        //设置消息上下文
	NewMsg() proto.Message                                                          //分配消息结构的函数
}

//基础消息处理结构
type BaseMsgHandle struct {
	Context MsgContext
}

//实现MsgHandle接口：处理消息的函数
func (h *BaseMsgHandle) Process(header *msg.MsgHeader, body proto.Message, rawBody []byte) (int, error) {
	return 0, nil
}

//实现MsgHandle接口：设置消息上下文
func (h *BaseMsgHandle) SetContext(c MsgContext) {
	h.Context = c
}

//实现MsgHandle接口：分配消息结构的函数
func (h *BaseMsgHandle) NewMsg() proto.Message {
	return nil
}
