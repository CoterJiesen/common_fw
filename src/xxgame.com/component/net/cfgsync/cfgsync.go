// cfgsync组件实现的功能如下:
//	1. 系统启动时连接配置中心并查询最新部署配置(客户端和服务端组件配置)；
//	2. 保持向前兼容：连接配置中心不成功时直接使用本地配置；
//	3. 周期性向配置中心发起最新配置的查询请求；
package cfgsync

import (
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"time"
)

import (
	"github.com/golang/protobuf/proto"
	"xxgame.com/component/cfg"
	"xxgame.com/component/log"
	"xxgame.com/component/net"
	"xxgame.com/component/process"
	msg "xxgame.com/types/proto"
	"xxgame.com/types/proto/config"
	"xxgame.com/types/proto/interior"
	"xxgame.com/utils"
)

//常量定义
const (
	LogTag         = "cfgsync"
	CfgSyncFile    = "cfgsync.cfg" //配置同步组件配置文件名
	SvrDeployFile  = "server.cfg"  //服务组件部署文件名
	ClientsCfgPath = "clients"     //客户端组件配置存放目录
)

// 远程配置同步组件
type CfgSync struct {
	isStarted  bool              // 标志配置同步是否已开始
	proc       *process.Process  // 服务进程组件
	cfgMng     *cfg.CfgMng       // 配置管理组件
	logger     *log.Logger       // 日志组件
	comm       *net.Commu        // 网络组件
	conn       *net.Conn         // 与配置中心的tcp连接
	cfg        *config.CfgSync   // 配置同步组件的配置
	owner      CfgOwner          // 配置的所有者
	cfgMD5     map[string][]byte // 所有同步到的配置名称与其md5对应关系
	svrID      uint32            // 服务id
	forceLocal bool              // 强制使用本地配置
}

var (
	cfgSync *CfgSync // 配置同步组件单件实例
)

// 获取配置同步配置组件实例
func Instance() *CfgSync {
	if cfgSync == nil {
		cfgSync = new(CfgSync)
		cfgSync.cfgMD5 = make(map[string][]byte, 10)
	}
	return cfgSync
}

// 初始化
func (cs *CfgSync) Initialize(proc *process.Process, l *log.Logger, forceLocal bool) {
	cs.cfgMng = cfg.Instance()
	cs.proc = proc
	cs.logger = l
	cs.forceLocal = forceLocal

	if forceLocal {
		return
	}

	// 读取组件的配置
	cfgPath := proc.DefaultDeployDir + "/" + CfgSyncFile
	buf, err := ioutil.ReadFile(cfgPath)
	if err != nil {
		cs.logger.Errorf(LogTag, "can't read %s, %s", cfgPath, err)
		panic(fmt.Errorf("can't read %s, %s", cfgPath, err))
	}
	cfg := new(config.CfgSync)
	err = proto.UnmarshalText(string(buf), cfg)
	if err != nil {
		cs.logger.Errorf(LogTag, "unmarshal cfg failed, %s", err)
		panic(fmt.Errorf("unmarshal cfg failed, %s", err))
	}
	cs.cfg = cfg

	// 分配网络通讯组件
	cs.comm = net.NewCommu(false, "tcp", []string{cs.cfg.GetAddr()}, 100, 100)
	if cs.comm == nil {
		cs.logger.Errorf(LogTag, "alloc net.comm failed")
		panic("alloc net.comm failed")
	}
}

// 开始同步配置
// 系统启动时以阻塞方式完成一次配置拉取，后续在协程中完成周期性的配置更新
func (cs *CfgSync) Start(owner CfgOwner) {
	if cs.isStarted {
		panic("cfg sync already started!")
	}
	cs.isStarted = true
	cs.owner = owner

	if cs.comm == nil {
		// 尝试读取本地配置
		cs.registerLocalCfg()
	} else {
		// 从配置中心拉取配置
		tm := time.NewTimer(time.Second * 5)
		gotConn := make(chan int, 1)
		cs.comm.Start(func(conn *net.Conn) {
			cs.conn = conn
			gotConn <- 0
		})
		select {
		case <-tm.C:
			// 与配置中心建立连接超时
			panic("connect cfgcenter failed!")
		case <-gotConn:
			// 删除本地配置
			cmdStr := "rm -rf " + cs.proc.DefaultDeployDir + "/" +
				SvrDeployFile + " " + cs.proc.DefaultDeployDir + "/" + ClientsCfgPath
			cmd := exec.Command("sh", "-c", cmdStr)
			err := cmd.Start()
			if err != nil {
				cs.logger.Errorf(LogTag, "start cmd %s failed %s", cmdStr, err)
				panic(err)
			}
			err = cmd.Wait()
			if err != nil {
				cs.logger.Errorf(LogTag, "remove old cfg failed, %s", err)
				panic(err)
			}

			// 克隆远端配置
			reqBody := new(interior.CloneCfgRequest)
			reqBody.ServerType = proto.Uint32(cs.cfg.GetSvrType())
			reqMsg := new(msg.Msg)
			reqMsg.Body, _ = proto.Marshal(reqBody)
			reqMsg.Header = new(msg.MsgHeader)
			reqMsg.Header.PHeader = new(msg.ProtoHeader)
			reqMsg.Header.PHeader.Version = proto.Uint32(uint32(msg.ProtoVersion_PV))
			reqMsg.Header.SHeader = new(msg.SessionHeader)
			reqMsg.Header.SHeader.TimeStamp = proto.Uint64(uint64(time.Now().Unix()))
			reqMsg.Header.SHeader.MsgID = proto.Uint32(uint32(interior.CfgSyncMsgID_CLONE_REQ))
			data, _ := proto.Marshal(reqMsg)
			err = cs.conn.SendNormalMsg(data, time.Second*5)
			if err != nil {
				cs.logger.Errorf(LogTag, "send msg failed, %s", err)
				panic(err)
			}
			respMsg := new(msg.Msg)
			respBody := new(interior.CloneCfgResponse)
			for {
				data, re := cs.conn.RecvMsg(time.Second * 5)
				if re != nil {
					cs.logger.Errorf(LogTag, "recv msg failed, %s", re)
					panic(re)
				}
				err = proto.Unmarshal(data.Data, respMsg)
				if err != nil {
					cs.logger.Errorf(LogTag, "unmarshal response failed, %s", err)
					panic(err)
				}
				err = proto.Unmarshal(respMsg.Body, respBody)
				if err != nil {
					cs.logger.Errorf(LogTag, "unmarshal body failed, %s", err)
					panic(err)
				}
				cs.acceptNewCfg(respBody.GetCfgName(), respBody.GetCfgData())
				if respBody.GetIsEnd() {
					break
				}
			}

			// 注册克隆过来的配置
			cs.registerLocalCfg()
		}

		// 启动配置更新协程
		go cs.syncJob()
	}
}

// 周期性访问配置中心以获取新的部署配置
func (cs *CfgSync) syncJob() {
	for cs.conn == nil {
		cs.logger.Warnf(LogTag, "trying to connect cfg center %s", cs.cfg.GetAddr())
		cs.comm.Start(func(conn *net.Conn) {
			cs.conn = conn
		})
		time.Sleep(time.Second * 5)
	}

	var gotNewCfg bool
	reqBody := new(interior.PullCfgRequest)
	reqMsg := new(msg.Msg)
	reqMsg.Header = new(msg.MsgHeader)
	reqMsg.Header.PHeader = new(msg.ProtoHeader)
	reqMsg.Header.SHeader = new(msg.SessionHeader)
	for {
		time.Sleep(time.Second * 15)

		for cs.conn.IsClosed() {
			cs.logger.Warnf(LogTag, "trying to connect cfg center %s", cs.cfg.GetAddr())
			cs.comm.Restart(func(conn *net.Conn) {
				cs.conn = conn
			})
			time.Sleep(time.Second * 5)
		}

		// 拉取配置中心的最新配置
		gotNewCfg = false
		for cfgName, m5 := range cs.cfgMD5 {
			reqBody.ServerID = proto.Uint32(cs.svrID)
			reqBody.CfgName = proto.String(cfgName)
			reqBody.MD5 = m5
			reqMsg.Body, _ = proto.Marshal(reqBody)
			reqMsg.Header.PHeader.Version = proto.Uint32(uint32(msg.ProtoVersion_PV))
			reqMsg.Header.SHeader.TimeStamp = proto.Uint64(uint64(time.Now().Unix()))
			reqMsg.Header.SHeader.MsgID = proto.Uint32(uint32(interior.CfgSyncMsgID_PULL_REQ))
			data, _ := proto.Marshal(reqMsg)
			err := cs.conn.SendNormalMsg(data, time.Second*5)
			if err != nil {
				cs.logger.Errorf(LogTag, "send msg failed, %s", err)
				break
			}
			respMsg := new(msg.Msg)
			respBody := new(interior.PullCfgResponse)
			respData, re := cs.conn.RecvMsg(time.Second * 5)
			if re != nil {
				cs.logger.Errorf(LogTag, "recv msg failed, %s", re)
				break
			}
			err = proto.Unmarshal(respData.Data, respMsg)
			if err != nil {
				cs.logger.Errorf(LogTag, "unmarshal response failed, %s", err)
				break
			}
			err = proto.Unmarshal(respMsg.Body, respBody)
			if err != nil {
				cs.logger.Errorf(LogTag, "unmarshal body failed, %s", err)
				break
			}
			if respBody.GetNeedUpdate() {
				cs.acceptNewCfg(respBody.GetCfgName(), respBody.GetCfgData())
				gotNewCfg = true
			}
		}
		if gotNewCfg {
			cs.proc.Commander.SignalReload()
		}
	}
}

// 接收新配置
func (cs *CfgSync) acceptNewCfg(cfgName string, cfgData []byte) {
	if cfgName == "" {
		return
	}

	// 缓存配置数据md5
	checksum := md5.Sum(cfgData)
	cs.cfgMD5[cfgName] = checksum[:]

	// 写入本地
	var textData string
	var path string
	if strings.HasPrefix(cfgName, ClientsCfgPath+".") {
		cfg := new(config.ClientCfg)
		err := proto.Unmarshal(cfgData, cfg)
		if err != nil {
			panic(err)
		}
		textData = proto.MarshalTextString(cfg)
		path = cs.proc.DefaultDeployDir + "/" + strings.Replace(cfgName, ".", "/", -1) + ".cfg"
	} else if cfgName+".cfg" == SvrDeployFile {
		cfg := new(config.ServerCfg)
		err := proto.Unmarshal(cfgData, cfg)
		if err != nil {
			panic(err)
		}
		textData = proto.MarshalTextString(cfg)
		path = cs.proc.DefaultDeployDir + "/" + SvrDeployFile
	} else {
		panic("invalid cfg name: " + cfgName)
	}

	x := strings.LastIndex(path, "/")
	if x >= 0 {
		os.MkdirAll(path[:x], 0777)
	}
	err := ioutil.WriteFile(path, []byte(textData), 0666)
	if err != nil {
		cs.logger.Errorf(LogTag, "write file %s failed, %s", path, err)
	}
}

// 注册本地配置
func (cs *CfgSync) registerLocalCfg() {
	//加载服务端组件配置
	svrCfg := new(config.ServerCfg)
	buf, err := ioutil.ReadFile(cs.proc.DefaultDeployDir + "/" + SvrDeployFile)
	if err != nil {
		panic(err)
	}
	proto.UnmarshalText(string(buf), svrCfg)
	cs.svrID, err = utils.ConvertServerIDString2Number(svrCfg.GetId())
	if err != nil {
		panic(err)
	}
	_, err = cs.cfgMng.Register(strings.TrimSuffix(SvrDeployFile, ".cfg"),
		cs.proc.DefaultDeployDir+"/"+SvrDeployFile, false,
		func() proto.Message { return new(config.ServerCfg) },
		nil, cs.owner.OnNewCfg)
	if err != nil {
		cs.logger.Errorf(LogTag, "register server cfg failed, %s", err)
		panic(err)
	}

	//扫描并加载客户端组件所有配置
	cfgPath := cs.proc.DefaultDeployDir + "/" + ClientsCfgPath
	f, openErr := os.OpenFile(cfgPath, os.O_RDONLY, 0666)
	if openErr != nil {
		if os.IsNotExist(openErr) {
			cs.logger.Infof(LogTag, "directory %s isn't found", cfgPath)
			return
		} else {
			cs.logger.Errorf(LogTag, "open directory %s failed, %s", cfgPath, openErr)
			return
		}
	}
	defer f.Close()
	names, readErr := f.Readdirnames(0)
	if readErr != nil {
		cs.logger.Errorf(LogTag, "read directory %s failed, %s", cfgPath, readErr)
		return
	}
	for _, n := range names {
		if !strings.HasSuffix(n, ".cfg") {
			continue
		}
		cfgName := ClientsCfgPath + "." + strings.TrimSuffix(n, ".cfg")
		_, regErr := cs.cfgMng.Register(cfgName, cfgPath+"/"+n, true,
			func() proto.Message { return new(config.ClientCfg) },
			nil, cs.owner.OnNewCfg)
		if regErr != nil {
			cs.logger.Errorf(LogTag, "register cfg %s failed, %s", cfgName, regErr)
			continue
		}
	}
}
