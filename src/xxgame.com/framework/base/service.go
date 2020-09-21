//基础服务实现
package base

import (
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"runtime"
	"runtime/debug"
	"strings"
	"time"
)

import (
	"github.com/golang/protobuf/proto"
	"xxgame.com/component/cfg"
	"xxgame.com/component/log"
	"xxgame.com/component/net"
	"xxgame.com/component/net/cfgsync"
	"xxgame.com/component/net/client"
	"xxgame.com/component/net/server"
	"xxgame.com/component/oplog"
	"xxgame.com/component/process"
	"xxgame.com/component/reportdata"
	"xxgame.com/component/routinepool"
	"xxgame.com/component/timer"
	"xxgame.com/component/webserver"
	"xxgame.com/framework"
	msg "xxgame.com/types/proto"
	"xxgame.com/types/proto/config"
	"xxgame.com/utils"
	"xxgame.com/utils/errors"
)

var (
	LOGLEVEL_MAP = map[string]log.Level{
		"FATAL": log.FATAL,
		"ERROR": log.ERROR,
		"WARN":  log.WARN,
		"INFO":  log.INFO,
		"DEBUG": log.DEBUG,
	}
)

type TimerInfo struct {
	startTime int64
	pauseTime int64
	waitTime  time.Duration
	action    func(string)
}

//基础服务
type BaseService struct {
	Proc                  *process.Process          //进程管理组件
	Log                   *log.Logger               //运行日志组件
	Cfg                   *cfg.CfgMng               //本地配置管理组件
	CfgSync               *cfgsync.CfgSync          //远程配置同步组件
	UseLocalCfg           bool                      //使用本地配置
	TM                    *timer.TimerManager       //定时器管理组件
	Svr                   *server.Server            //网络服务组件
	CM                    *ConnMng                  //服务连接管理器
	ReloadChan            chan int                  //用于通讯的各种管道, 接收重载信号的管道
	ExitChan              chan int                  //接收退出信号的管道
	HttpChan              chan *process.HttpContext //接收http命令的管道
	TNChan                chan *timer.TimeoutNotify //接收定时器超时通知的管道
	MP                    MsgMap                    //消息映射
	FW                    *framework.FrameWork      //记录框架实例
	ServiceID             uint32                    //保存服务id
	cliCfgMap             map[string]uint32         //记录客户端组件的配置名和服务类型的映射关系
	CP                    map[uint32]*client.Client //所有客户端组件，key为服务类型
	OL                    *oplog.OpLogger           //运营日志组件
	TaskPool              *routinepool.RoutinePool  //消息处理协程池
	TaskChan              chan func()               //任务管道
	StatusWrapper         *reportdata.StatusWrapper //上报数据到状态服
	Perf                  *PerfMon                  //性能监控
	ProcessStartTimeStamp int64                     //进行启动时间戳

	allocTimeoutActionID      uint64                  //分配超时事件ID
	allocTimeoutActionIDMutex chan bool               //分配超时事件ID保护锁
	delTimeoutAction          map[string]chan bool    //删除管道字典
	timeoutAction             chan *TimeoutAction     //添加超时事件ID
	timeoutActionMap          map[string]func(string) //超时函数字典
	timerInfo                 map[string]*TimerInfo   //定时器信息
	pauseTimerEvent           map[string]*TimerInfo   //暂停的定时器
}

const (
	LogTag                      = "Service" //日志tag
	DefaultTNChanSize           = 10000     //默认定时器管道大小
	DefaultTaskPoolSize         = 20000     //默认协程池大小
	DefaultTaskChanSize         = 10000     //默认协程管道大小
	DefaultDelTimeoutActionSize = 10        //默认删除定时器管道大小
	DefaultTimeoutActionSize    = 10000     //默认定时器管道大小
)

//实现Service接口:初始化函数
func (s *BaseService) Init(FW *framework.FrameWork) (int, error) {
	//创建各个管道
	s.ReloadChan = make(chan int, 1)
	s.ExitChan = make(chan int, 1)
	s.HttpChan = make(chan *process.HttpContext)
	s.TNChan = make(chan *timer.TimeoutNotify, DefaultTNChanSize)
	s.allocTimeoutActionID = 0
	s.allocTimeoutActionIDMutex = make(chan bool, 1)
	s.delTimeoutAction = make(map[string]chan bool, DefaultDelTimeoutActionSize)
	s.timeoutAction = make(chan *TimeoutAction, DefaultTimeoutActionSize)
	s.timeoutActionMap = make(map[string]func(string))
	s.TaskChan = make(chan func(), DefaultTaskChanSize)
	s.timerInfo = make(map[string]*TimerInfo)
	s.pauseTimerEvent = make(map[string]*TimerInfo)
	s.ProcessStartTimeStamp = time.Now().Unix()

	s.MP = NewMsgMap()
	s.FW = FW

	//初始化客户端组件表
	s.cliCfgMap = make(map[string]uint32)
	s.CP = make(map[uint32]*client.Client)

	//初始化进程管理组件
	s.Proc = process.Instance()
	s.Proc.HttpServicePretreater = FW.Service.HttpServicePretreater
	_, e := s.Proc.Initialize("", "", "", "", s)
	if e != nil {
		panic(e.Error())
	}

	//初始化日志组件
	s.Log = log.NewFileLogger(s.Proc.Name, s.Proc.DefaultLogDir, "")
	if s.Log == nil {
		panic("init logger failed")
	}

	defer log.FlushAll()

	//先设置打印所有日志
	s.Log.SetLevel(log.DEBUG)

	//记录服务启动事件
	s.Log.Infof(LogTag, "%s starting...", s.Proc.Name)

	//初始化配置管理组件
	s.Cfg = cfg.Instance()
	s.Cfg.Initialize("", s.Log)

	//远程配置同步组件初始化
	s.CfgSync = cfgsync.Instance()
	s.CfgSync.Initialize(s.Proc, s.Log, s.UseLocalCfg)
	s.CfgSync.Start(s)

	//注册配置
	_, e = s.FW.Service.RegisterCfg()
	if e != nil {
		s.Log.Errorf(LogTag, "%s register cfg failed: %s", s.Proc.Name, e.Error())
		return -1, e
	}

	//初始装载所有配置
	_, e = s.Cfg.InitLoadAll()
	if e != nil {
		s.Log.Errorf(LogTag, "%s load cfg failed: %s", s.Proc.Name, e.Error())
		return -1, e
	}

	//设置日志等级
	s.FW.Service.SetLogLevel()

	//初始化定时器管理组件
	s.TM = timer.Instance()
	s.TM.Initialize(s.TNChan, s.Log)

	//打印服务基本信息
	fmt.Print(s.Proc)

	//打印所有配置
	s.Cfg.PrintAll()

	//服务后台化
	_, e = s.Proc.Daemonize()
	if e != nil {
		s.Log.Errorf(LogTag, "%s daemonize failed: %s", s.Proc.Name, e.Error())
		return -1, e
	}

	//注册消息处理
	s.FW.Service.RegisterMsgHandle()

	//启动任务处理协程池
	s.TaskPool = routinepool.NewPool(DefaultTaskPoolSize).Start()

	//初始化性能监控
	s.Perf = NewPerfMon(s)

	//启动网络组件
	_, e = s.FW.Service.SetupNetwork()
	if e != nil {
		s.Log.Errorf(LogTag, "%s set up network failed: %s", s.Proc.Name, e.Error())
		return -1, e
	}

	//记录服务启动完成日志
	s.Log.Infof(LogTag, "%s started", s.Proc.Name)

	return 0, nil
}

//实现Service接口:注册所有配置
func (s *BaseService) RegisterCfg() (int, error) {
	return 0, nil
}

//实现远端配置加载回调接口
func (s *BaseService) OnNewCfg(cfgName string, cfg proto.Message) (int, error) {
	switch cfg.(type) {
	case *config.ServerCfg:
		svrCfg := cfg.(*config.ServerCfg)
		s.Proc.HttpUrl = *svrCfg.HttpAddr
		serverID, err := utils.ConvertServerIDString2Number(*svrCfg.Id)
		if err != nil {
			panic(err)
		}
		s.ServiceID = serverID
	case *config.ClientCfg:
		//判断配置是否合法，主要判断服务id是否合法且类型是否重复
		cliCfg := cfg.(*config.ClientCfg)
		if len(cliCfg.ServerEntries) == 0 {
			s.Log.Errorf(LogTag, "empty server entries!")
			return -1, errors.New("empty server entries!")
		}
		fields, err := utils.ConvertServerIDString2Fields(*cliCfg.ServerEntries[0].Id)
		if err != nil {
			s.Log.Errorf(LogTag, "invalid server id, %s", *cliCfg.ServerEntries[0].Id)
			return -1, errors.New("invalid server id, %s", *cliCfg.ServerEntries[0].Id)
		}
		svrType := fields[0]
		idMap := map[string]bool{}
		idMap[*cliCfg.ServerEntries[0].Id] = true
		for i, entry := range cliCfg.ServerEntries {
			if i == 0 {
				continue
			}
			fields, err = utils.ConvertServerIDString2Fields(*entry.Id)
			if err != nil {
				s.Log.Errorf(LogTag, "invalid server id, %s", *entry.Id)
				return -1, errors.New("invalid server id, %s", *entry.Id)
			}
			if fields[0] != svrType {
				s.Log.Errorf(LogTag, "inconsistent server type %s!", *entry.Id)
				return -1, errors.New("inconsistent server type %s!", *entry.Id)
			}
			if _, exists := idMap[*entry.Id]; exists {
				s.Log.Errorf(LogTag, "duplicate server id %s!", *entry.Id)
				return -1, errors.New("duplicate server id %s!", *entry.Id)
			}
		}
		s.cliCfgMap[cfgName] = svrType
		if com, exists := s.CP[svrType]; exists {
			com.ReloadCfg() // 通知各个客户端组件重加载配置
		}
	default:
		s.Log.Errorf(LogTag, "unknown cfg type, %s", reflect.TypeOf(cfg))
		return -1, errors.New("unknown cfg type, %s", reflect.TypeOf(cfg))
	}

	return 0, nil
}

//实现Service接口:设置日志等级
func (s *BaseService) SetLogLevel() {
	//获取配置
	c, _ := s.Cfg.Get("server").(*config.ServerCfg)
	if c != nil {
		lname := c.GetLogLevel()
		l, found := LOGLEVEL_MAP[lname]
		if found {
			s.Log.SetLevel(l)
			net.SetDebugLevel(l)
			return
		}
	}

	s.Log.SetLevel(log.INFO)
	net.SetDebugLevel(log.INFO)
}

//实现Service接口:启动网络
func (s *BaseService) SetupNetwork() (int, error) {
	//创建并初始化连接管理器
	s.CM = new(ConnMng)
	if e := s.CM.Init(s.FW.Service, s.Log); e != nil {
		return -1, e
	}

	//启动服务端组件
	s.Svr = server.Instance()
	if s.Svr == nil {
		return -1, errors.New("get server instance failed")
	}
	if !s.Svr.Initialize(s.CM,
		strings.TrimSuffix(cfgsync.SvrDeployFile, ".cfg"), s.Log, s) {
		return -1, errors.New("server init failed")
	}
	e := s.Svr.Start()
	if e != nil {
		return -1, e
	}

	//启动客户端组件
	serverId := s.Svr.GetServerId()
	for cfgName, svrType := range s.cliCfgMap {
		s.CP[svrType] = client.NewClient(cfgName, s.Log, serverId, s)
		s.CP[svrType].Start()
	}

	//初始化运营日志组件
	logCli, exists := s.CP[uint32(msg.SvrType_LOG)]
	if exists {
		s.OL = oplog.Instance()
		s.OL.Initialize(logCli, s.Log)
	}
	//状态服务组件
	{
		statusKeeperCli, exists := s.CP[uint32(msg.SvrType_StatusKeeper)]
		if exists {
			s.StatusWrapper = reportdata.Instance()
			s.StatusWrapper.Initialize(statusKeeperCli, s.Log)
		}
	}

	return 0, nil
}

//实现Service接口:新消息到来
func (s *BaseService) OnNewMsg(buff []byte) error {
	return s.FW.Service.ProcessMsg(buff)
}

func (s *BaseService) ProcessMsgWithSingleThread(buff []byte) {

	pdata := new(PerfData)
	start := time.Now()
	s.TaskChan <- func() {
		waitChan := make(chan int, 1)
		pdata.TaskWaitTime = time.Now().Sub(start)
		start = time.Now()
		defer func() {
			pdata.MsgProcTime = time.Now().Sub(start)
			s.Perf.PerfChan <- pdata
			waitChan <- 0
		}()

		//解码消息包
		var m msg.Msg
		if e := proto.Unmarshal(buff, &m); e != nil {
			s.Log.Errorf("PROTO", "decode buffer length %d failed", len(buff))
			return
		}

		go func() {
			select {
			case <-waitChan:
			case <-time.After(time.Second * 5):
				buf := make([]byte, 1024*3)
				size := runtime.Stack(buf, false)
				s.Log.Errorf("PROTO",
					"ProcessMsgWithSingleThread Task busy too long!\nProcessing msg: %s\n%s\n",
					&m, string(buf[:size]))
			}
		}()

		//处理消息
		err := s.FW.Service.PreProcessMsg(&m)
		if err == nil {
			_, e, body := s.MP.Process(&m, s)
			if e != nil {
				if body == nil {
					s.Log.Errorf("PROTO", "process msg %v failed:%s", m, e.Error())
				} else {
					s.Log.Errorf("PROTO", "process msg %v %v failed:%s", reflect.TypeOf(body).String(), body, e.Error())
				}
			} else {
				if body == nil {
					s.Log.Debugf("PROTO", "process msg %v ok", m)
				} else {
					s.Log.Debugf("PROTO", "process msg %v %v ok", reflect.TypeOf(body).String(), body)
				}
			}
		} else {
			s.Log.Errorf("PROTO", "preprocess msg %v failed:%v", m, err)
		}
	}
}

func (s *BaseService) ProcessMsgMultiThread(buff []byte) error {
	pdata := new(PerfData)
	start := time.Now()
	err := s.TaskPool.DispatchTask(func() {
		defer func() {
			if err := recover(); err != nil {
				s.CatchError(err)
			}
		}()
		waitChan := make(chan int, 1)
		pdata.TaskWaitTime = time.Now().Sub(start)
		start = time.Now()
		defer func() {
			pdata.MsgProcTime = time.Now().Sub(start)
			s.Perf.PerfChan <- pdata
			waitChan <- 0
		}()

		//解码消息包
		var m msg.Msg
		if e := proto.Unmarshal(buff, &m); e != nil {
			s.Log.Errorf("PROTO", "decode buffer length %d failed", len(buff))
			return
		}

		go func() {
			select {
			case <-waitChan:
			case <-time.After(time.Minute):
				buf := make([]byte, 1024*3)
				size := runtime.Stack(buf, false)
				s.Log.Errorf("PROTO",
					"ProcessMsgMultiThread Task busy too long!\nProcessing msg: %s\n%s\n",
					&m, string(buf[:size]))
			}
		}()

		//处理消息
		err := s.FW.Service.PreProcessMsg(&m)
		if err == nil {
			_, e, body := s.MP.Process(&m, s)
			if e != nil {
				if body == nil {
					s.Log.Errorf("PROTO", "process msg %v failed:%s", m, e.Error())
				} else {
					s.Log.Errorf("PROTO", "process msg %v %v %v failed:%s", m.GetHeader(), reflect.TypeOf(body).String(), body, e.Error())
				}
			} else {
				if body == nil {
					s.Log.Debugf("PROTO", "process msg %v ok", m)
				} else {
					s.Log.Debugf("PROTO", "process msg %v %v %v ok", m.GetHeader(), reflect.TypeOf(body).String(), body)
				}
			}
		} else {
			s.Log.Errorf("PROTO", "preprocess msg %v failed:%v", m, err)
		}
	})

	return err
}

//实现Service接口:处理消息
func (s *BaseService) ProcessMsg(buff []byte) error {
	if s.FW.Service.ProcessMsgSingleOrMultiple() {
		s.ProcessMsgWithSingleThread(buff)
	} else {
		return s.ProcessMsgMultiThread(buff)
	}
	return nil
}

//实现Service接口:处理http命令
func (s *BaseService) ProcessHttpCmd(h *process.HttpContext) {
	if h == nil {
		return
	}

	s.Log.Debugf("HTTP", "got http cmd %s", h.Req.URL)

	switch h.Req.FormValue("loglevel") {
	case "debug":
		s.Log.SetLevel(log.DEBUG)
		net.SetDebugLevel(log.DEBUG)
		h.Res.Write([]byte(fmt.Sprintf("loglevel is %v now!\n", log.DEBUG)))
		return
	case "info":
		s.Log.SetLevel(log.INFO)
		net.SetDebugLevel(log.INFO)
		h.Res.Write([]byte(fmt.Sprintf("loglevel is %v now!\n", log.INFO)))
		return
	case "warn":
		s.Log.SetLevel(log.WARN)
		net.SetDebugLevel(log.WARN)
		h.Res.Write([]byte(fmt.Sprintf("loglevel is %v now!\n", log.WARN)))
		return
	case "error":
		s.Log.SetLevel(log.ERROR)
		net.SetDebugLevel(log.ERROR)
		h.Res.Write([]byte(fmt.Sprintf("loglevel is %v now!\n", log.ERROR)))
		return
	}

	switch h.Req.FormValue("stack") {
	case "main":
		buf := make([]byte, 8*1024)
		size := runtime.Stack(buf, false)
		h.Res.Write(buf[:size])
		return
	case "all":
		buf := make([]byte, 10*1024*1024)
		size := runtime.Stack(buf, true)
		h.Res.Write(buf[:size])
		return
	}

	switch h.Req.FormValue("trace") {
	case "start":
		if s.Perf.StartTrace() == nil {
			h.Res.Write([]byte(fmt.Sprintf("trace is started now!\n")))
		} else {
			h.Res.Write([]byte(fmt.Sprintf("trace is started already!\n")))
		}
		return
	case "stop":
		s.Perf.StopTrace()
		h.Res.Write([]byte(fmt.Sprintf("trace is stopped now!\n")))
		return
	}

	switch h.Req.FormValue("cfg") {
	case "set":
		//更新配置
		//cfg=set&name=?&data=?
		name := h.Req.FormValue("name")
		data, _ := ioutil.ReadAll(h.Req.Body)
		if name == "" || data == nil || len(data) == 0 {
			h.Res.Write([]byte(fmt.Sprintf("status=err&result=invalid parameter!")))
			return
		}
		e := s.Cfg.ReloadAndSave(name, string(data))
		if e != nil {
			h.Res.Write([]byte(fmt.Sprintf("status=err&result=%s", e)))
			return
		}
		h.Res.Write([]byte(fmt.Sprintf("status=ok", e)))
		return
	case "get":
		//获取配置
		//cfg=get&name=?
		name := h.Req.FormValue("name")
		if name == "" {
			cfgName := ""
			for k, _ := range s.Cfg.Cfg {
				cfgName = cfgName + "," + k
			}
			h.Res.Write([]byte(fmt.Sprintf("status=err&result=invalid parameter!has cfgs[%s]", cfgName)))
			return
		}
		m := s.Cfg.Get(name)
		if m == nil {
			h.Res.Write([]byte(fmt.Sprintf("status=err&result=can't find %s!", name)))
			return
		}
		s := proto.MarshalTextString(m)
		h.Res.Write([]byte(fmt.Sprintf("status=ok&result=%s", s)))
		return
	}
}

//实现Service接口：处理定时器超时通知
func (s *BaseService) ProcessTimer(tn *timer.TimeoutNotify) {
	s.Log.Debugf("TIMEOUT", "got time notify %#v", tn)
}

//实现Service接口:其他每循环要执行的update
func (s *BaseService) Update() {
	//s.Log.Debugf("DEBUG","Update")
	log.FlushAll()
}

//实现Service接口:重载
func (s *BaseService) OnReload() {
	s.Log.Infof("CFG", "reload all begin")
	s.Cfg.ReLoadAll()
	//重新设置日志等级
	s.FW.Service.SetLogLevel()
}

//实现Service接口:退出
func (s *BaseService) OnExit() {
	s.Log.Infof(LogTag, "%s exit", s.Proc.Name)
	log.FlushAll()
}

//实现Service接口:主线程Panic
func (s *BaseService) OnPanic() {
	s.Log.Errorf(LogTag, "%s panic", s.Proc.Name)
}

//实现Service接口：网络连接异常断开
func (s *BaseService) OnNetDisconn(conn *net.Conn) {

}

//实现Service接口:主循环函数
func (s *BaseService) MainLoop() {
	for {
		select {
		case <-s.ReloadChan:
			//重载配置
			s.FW.Service.OnReload()
		case <-s.ExitChan:
			//进程退出
			s.FW.Service.OnExit()
			return
		case http := <-s.HttpChan:
			//http命令
			s.FW.Service.ProcessHttpCmd(http)
			s.HttpChan <- nil
		case tn := <-s.TNChan:
			//定时器超时
			s.FW.Service.ProcessTimer(tn)
		case t := <-s.TaskChan:
			//任务管道
			func() {
				defer func() {
					if err := recover(); err != nil {
						s.CatchError(err)
					}
				}()
				t()
			}()
		case timeoutAction, chanOK := <-s.timeoutAction:
			//定时器
			if chanOK == false {
				return
			}
			switch timeoutAction.chanType {
			case 4: //暂停恢复
				info, infoExist := s.pauseTimerEvent[timeoutAction.actionID]
				if infoExist {
					delete(s.pauseTimerEvent, timeoutAction.actionID)
					timeoutAction.action = info.action
					timeoutAction.waitTime = info.waitTime - time.Duration(info.pauseTime-info.startTime)
				} else {
					break
				}
				fallthrough
			case 0: //添加
				actionID := timeoutAction.actionID
				waitTime := timeoutAction.waitTime
				repeat := timeoutAction.repeat
				_, exist := s.timeoutActionMap[actionID]
				if !exist {
					s.timeoutActionMap[actionID] = timeoutAction.action

					s.delTimeoutAction[actionID] = make(chan bool, 1)

					//保存为暂时使用
					s.timerInfo[actionID] = &TimerInfo{
						startTime: time.Now().UnixNano(),
						pauseTime: 0,
						waitTime:  waitTime,
						action:    timeoutAction.action,
					}
				} else if !repeat {
					break
				}
				delTimeoutActionChan := s.delTimeoutAction[actionID]
				go func() {
					select {
					case <-delTimeoutActionChan:
					case <-time.After(waitTime):
						s.WriteChanAddressTimeoutAction(s.timeoutAction, &TimeoutAction{
							chanType: 2,
							actionID: actionID,
							repeat:   repeat,
							waitTime: waitTime,
						})
					}
				}()
			case 2: //执行
				is_fallthrough := func() bool {
					//保护timer的handle函数处理发生的panic(因为这里也逻辑丰富，经常容易挂)
					defer func() {
						if err := recover(); err != nil {
							s.CatchError(err)
						}
					}()

					handle, exist := s.timeoutActionMap[timeoutAction.actionID]
					if exist {
						if timeoutAction.repeat {
							s.WriteChanAddressTimeoutAction(s.timeoutAction, &TimeoutAction{
								chanType: 0,
								actionID: timeoutAction.actionID,
								repeat:   timeoutAction.repeat,
								waitTime: timeoutAction.waitTime,
							})
						}
						handle(timeoutAction.actionID)
						if timeoutAction.repeat {
							return false
						}
					}

					return true
				}()
				if !is_fallthrough {
					break
				}
				fallthrough
			case 3: //暂停
				if timeoutAction.chanType == 3 {
					if v, exist := s.timerInfo[timeoutAction.actionID]; exist {
						s.pauseTimerEvent[timeoutAction.actionID] = v
						s.pauseTimerEvent[timeoutAction.actionID].pauseTime = time.Now().UnixNano()
					}
				}
				fallthrough
			case 1: //删除
				_, exist := s.timeoutActionMap[timeoutAction.actionID]
				if exist {
					delete(s.timeoutActionMap, timeoutAction.actionID)
				}
				_, delExist := s.delTimeoutAction[timeoutAction.actionID]
				if delExist {
					close(s.delTimeoutAction[timeoutAction.actionID])
					delete(s.delTimeoutAction, timeoutAction.actionID)
				}
				_, infoExist := s.timerInfo[timeoutAction.actionID]
				if infoExist {
					delete(s.timerInfo, timeoutAction.actionID)
				}
				if timeoutAction.chanType == 1 {
					if _, exist = s.pauseTimerEvent[timeoutAction.actionID]; exist {
						delete(s.pauseTimerEvent, timeoutAction.actionID)
					}
				}
			}
		}
	}
}

//实现Service接口:注册消息处理
func (s *BaseService) RegisterMsgHandle() {
}

//实现Service接口:注册一个消息处理
//参数msgId:消息id
//参数handle:处理消息的handle，传入的是指针
func (s *BaseService) RegOneMsgHandle(msgId uint32, handle framework.MsgHandle) (int, error) {
	return s.MP.Reg(msgId, handle)
}

//发送回复消息
func (s *BaseService) SendResponseToClient(header *msg.MsgHeader,
	msgId uint32, body proto.Message) (int, error) {
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

	//找到连接并发送
	conn := s.CM.GetConn(connID)
	if conn == nil {
		return -1, errors.New("can't find connection by connID %d", connID)
	}

	//	if s.Svr.IsGateWay() && header.GetPHeader().GetVersion() >= 2 {
	//		e = conn.SendNormalMsg(utils.Compress(buff), 0)
	//	} else {
	e = conn.SendNormalMsg(buff, time.Millisecond*500)
	//	}
	if e != nil {
		return -1, e
	}
	s.Log.Debugf(LogTag, "send client msg %v %v %v", m.GetHeader(), reflect.TypeOf(body).String(), body)

	return 0, nil
}

//获取服务id
func (s *BaseService) GetServiceID() uint32 {
	return s.ServiceID
}

//实现ProcessCommander接口:响应信号重载命令
func (s *BaseService) SignalReload() {
	s.ReloadChan <- 0
}

//实现ProcessCommander接口:响应信号退出命令
func (s *BaseService) SignalQuit() {
	s.ExitChan <- 0
}

//实现ProcessCommander接口:响应信号退出命令
func (s *BaseService) GetHttpChan() chan *process.HttpContext {
	return s.HttpChan
}

func (s *BaseService) HttpServicePretreater() {

}
func (s *BaseService) ProcessMsgSingleOrMultiple() bool {
	return false
}
func (s *BaseService) OnWebNetNewNode(node *webserver.WebNetNode) {
}

func (s *BaseService) OnWebNetDelNode(node *webserver.WebNetNode) {
}

func (s *BaseService) WriteChanAddressTimeoutAction(channel chan *TimeoutAction, value *TimeoutAction) (ret bool) {
	go func() {
		defer func() {
			if ex := recover(); ex != nil {
				str := fmt.Sprintf("%v", ex)
				if strings.Contains(str, "send on closed channel") {
				} else {
					panic(str)
				}
			}
		}()
		channel <- value
	}()
	return true
}

type TimeoutAction struct {
	chanType int8          //管道类型0添加1删除2执行
	actionID string        //超时事件ID
	waitTime time.Duration //等待时间
	action   func(string)  //事件函数
	repeat   bool          //是否重置执行
}

func (s *BaseService) PauseTimeoutAction(actionID string) bool {
	return s.WriteChanAddressTimeoutAction(s.timeoutAction, &TimeoutAction{
		chanType: 3,
		actionID: actionID,
	})
}

func (s *BaseService) ResumeTimeoutAction(actionID string) bool {
	return s.WriteChanAddressTimeoutAction(s.timeoutAction, &TimeoutAction{
		chanType: 4,
		actionID: actionID,
	})
}

//删除超时任务
func (s *BaseService) DelTimeoutAction(actionID string) bool {
	return s.WriteChanAddressTimeoutAction(s.timeoutAction, &TimeoutAction{
		chanType: 1,
		actionID: actionID,
	})
}

//添加超时任务
func (s *BaseService) AddTimeoutAction(waitTime time.Duration, fp func(actionID string), repeat bool) (string, bool) {
	s.allocTimeoutActionIDMutex <- true
	allocID := s.allocTimeoutActionID
	s.allocTimeoutActionID++
	<-s.allocTimeoutActionIDMutex
	id := fmt.Sprintf("%v", allocID)
	success := s.AddTimeoutActionEx(waitTime, id, fp, repeat)
	return id, success
}

//添加超时任务
func (s *BaseService) AddTimeoutActionEx(waitTime time.Duration, actionID string, fp func(actionID string), repeat bool) bool {
	return s.WriteChanAddressTimeoutAction(s.timeoutAction, &TimeoutAction{
		chanType: 0,
		actionID: actionID,
		waitTime: waitTime,
		action:   fp,
		repeat:   repeat,
	})
}

func (s *BaseService) PreProcessMsg(msg *msg.Msg) error {
	return nil
}

func (s *BaseService) CatchError(err interface{}) {
	if s.Log.GetLevel() == log.DEBUG {
		//debug模式，抛出panic
		panic(err)
	} else {
		//release模式，恢复
		fmt.Fprintln(os.Stderr, fmt.Sprintf("======catch error begin [%v]======", time.Now().Format(utils.TIME_FORMAT)))
		fmt.Fprintln(os.Stderr, err)
		//打印堆栈
		debug.PrintStack()
		fmt.Fprintln(os.Stderr, fmt.Sprintf("======catch error end   [%v]======", time.Now().Format(utils.TIME_FORMAT)))
		os.Stderr.Sync()
	}
}
