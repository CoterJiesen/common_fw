/*
进程管理组件，提供如下功能：
1.进程后台化
2.进程控制接口(自定义信号，http命令)
3.进程pid持久化
4.进程互斥
5.程序编译版本信息
*/
package process

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

import (
	"xxgame.com/utils"
	"xxgame.com/utils/errors"
)

//保存一些进程相关信息
type Process struct {
	Name                  string            //进程名
	Version               string            //版本描述
	DefaultRunDir         string            //默认运行目录
	DefaultLogDir         string            //默认日志目录
	DefaultCfgDir         string            //默认配置目录
	DefaultDeployDir      string            //默认部署目录
	HttpUrl               string            //进程开放的http命令url，格式如：hostname:port
	HttpServicePretreater func()            //HttpService预处理函数
	Commander             ProcessCmd        //响应命令的接口
	httpc                 chan *HttpContext //用于传递http上下文
	inited                bool              //是否已经初始化过
}

//单例
var (
	process  *Process
	lockfile *os.File
)

func init() {
	process = new(Process)
	process.httpc = make(chan *HttpContext)
}

//获取实例
func Instance() *Process {
	return process
}

//初始化实例
//参数defaultLogDir:默认日志路径(传入空值，由组件自动设置，值应该为相对运行路径的相对路径)
//参数defaultCfgDir:默认配置路径(传入空值，由组件自动设置，值应该为相对运行路径的相对路径)
//参数defaultDeployDir:默认部署路径(传入空值，由组件自动设置，值应该为相对运行路径的相对路径)
//参数httpUrl:http服务的监听url，格式如:127.0.0.1:8000
//参数cmd:响应信号和http命令的接口
func (p *Process) Initialize(
	defaultLogDir string,
	defaultCfgDir string,
	defaultDeployDir string,
	httpUrl string,
	cmd ProcessCmd) (int, error) {

	//判断是否已经初始化过
	if p.inited {
		return -1, errors.New("already inited")
	}

	//解析程序名和路径
	n, path := utils.GetProcessNameAndPath()
	_ = path

	//设置程序名
	p.Name = n

	//设置版本号
	p.Version = utils.ProcessVersion()

	//设置默认运行路径
	p.DefaultRunDir = path

	//设置默认日志路径
	if defaultLogDir == "" {
		p.DefaultLogDir = p.DefaultRunDir +
			utils.GetPathDel() + "../../log" + utils.GetPathDel() + p.Name
	} else {
		p.DefaultLogDir = defaultLogDir
	}

	//设置默认配置路径
	if defaultCfgDir == "" {
		p.DefaultCfgDir = p.DefaultRunDir + utils.GetPathDel() + "../config"
	} else {
		p.DefaultCfgDir = defaultCfgDir
	}

	//设置默认部署文件路径
	if defaultDeployDir == "" {
		p.DefaultDeployDir = p.DefaultRunDir + utils.GetPathDel() + "../deploy"
	} else {
		p.DefaultDeployDir = defaultDeployDir
	}

	//设置http监听url
	if httpUrl == "" {
		p.HttpUrl = "127.0.0.1:8000"
	} else {
		p.HttpUrl = httpUrl
	}

	p.Commander = cmd

	//标记为已经初始化过
	p.inited = true
	return 0, nil
}

//进程后台化
//1.进程脱离当前终端
//2.关闭终端输出输入(标准输入输出，错误输出)
//3.尝试互斥条件，并写入pid文件
//4.屏蔽无关信号,注册关心的信号
//5.启动http服务
func (p *Process) Daemonize() (int, error) {

	/*
		//脱离当前终端
		if utils.Daemonize() != 0 {
			return -1, errors.New("daemonize failed,system error")
		}
	*/

	//关闭终端输出输入
	//utils.CloseTermOutput()

	if e := p.checkMutex(); e != nil {
		return -1, e
	}

	//开启信号监听routine
	go p.signalProc()

	//开启http服务routine
	go p.httpService()

	return 0, nil
}

//输出字符串信息
func (p *Process) String() string {
	s := fmt.Sprintf("Name: %s\nVersion: %s\nDefaultRunDir: %s\nDefaultLogDir: %s\nDefaultCfgDir: %s\nDefaultDeployDir: %s\nHttpUrl: %s\n\n",
		p.Name,
		p.Version,
		p.DefaultRunDir,
		p.DefaultLogDir,
		p.DefaultCfgDir,
		p.DefaultDeployDir,
		p.HttpUrl)

	return s
}

//检查互斥条件，并写入pid文件
func (p *Process) checkMutex() error {
	//获取pid文件路径
	path := p.getPidPath()

	//尝试加锁
	var e error
	lockfile, e = os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0640)
	if lockfile == nil || e != nil {
		return e
	}
	e = utils.Flock(int(lockfile.Fd()))
	if e != nil {
		return errors.New("lock file %s failed", path)
	}

	//写入pid
	s := fmt.Sprintf("%d", utils.GetPid())
	_, e = lockfile.WriteString(s)

	return e
}

//处理信号的函数
func (p *Process) signalProc() {
	ch := make(chan os.Signal, 1)

	//拦截hup信号和term信号，分别用于重载和退出
	//使用这两个信号的原因是各个操作系统都有实现
	signal.Notify(ch, syscall.SIGHUP, syscall.SIGTERM)
	for {
		s := <-ch
		switch s {
		case syscall.SIGHUP:
			//触发重载信号
			p.Commander.SignalReload()
		case syscall.SIGTERM:
			//触发退出信号
			p.Commander.SignalQuit()
		}
	}
}

//开启http服务
func (p *Process) httpService() {
	/*
		if p.HttpServicePretreater != nil {
			p.HttpServicePretreater()
		}
	*/
	//注册处理函数
	http.HandleFunc("/cmd", httpCmdHandle)
	err := http.ListenAndServe(p.HttpUrl, nil)
	if err != nil {
		panic(err.Error())
	}
}

//获取pid文件路径
func (p *Process) getPidPath() string {
	return p.DefaultRunDir + utils.GetPathDel() + p.Name + ".pid"
}

//http命令处理
func httpCmdHandle(w http.ResponseWriter, req *http.Request) {
	p := Instance()

	//创建上下文
	req.ParseForm()
	var context HttpContext = HttpContext{w, req}

	//通过管道回调给真正的处理者
	c := p.Commander.GetHttpChan()
	c <- &context

	//等待结果返回
	_ = <-c
}
