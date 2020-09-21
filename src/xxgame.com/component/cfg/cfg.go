/*
配置管理组件：负责配置的注册，加载和获取
	1.加载配置时，采用双缓冲区交换的方式进行异步加载
	2.支持单个配置单独加载和全部配置加载
	3.支持只能加载一次，和可反复加载两种配置
	4.不支持配置之间的依赖关系！配置加载的顺序随机
*/
package cfg

import (
	"bytes"
	"crypto/md5"
	"io/ioutil"
	"time"
)

import (
	"github.com/golang/protobuf/proto"
	"xxgame.com/component/log"
	"xxgame.com/utils"
	"xxgame.com/utils/errors"
)

//单个配置结构
type cfg struct {
	Reloadable bool                                     //是否可以重载
	Path       string                                   //配置的相对路径
	md5        []byte                                   //md5签名
	Data       [2]proto.Message                         //配置内容双缓冲区(保存的是指针)
	CurrIdx    uint8                                    //当前缓冲区的索引[0,1]
	Timestamp  int64                                    //上次加载成功时间
	Prev       func(string, proto.Message) (int, error) //加载配置前执行的操作函数，注册时传入，可以为nil
	Post       func(string, proto.Message) (int, error) //加载配置后执行的操作函数，注册时传入，可以为nil
	//当Prev和Post都不为nil时，如果返回失败，就不执行后面操作(如交换缓冲区)
}

//配置管理器
type CfgMng struct {
	Root   string          //配置的根路径(相对路径，相对于可执行程序的路径)
	Cfg    map[string]*cfg //配置存放的map
	Log    *log.Logger     //日志接口
	Loaded bool            //是否已经初始化装载过，已经装载过则不允许在注册新的配置
	Result chan error      //用于与异步加载routine通信的管道
}

//全局惟一的配置管理器
var (
	cfgMng *CfgMng
)

//配置重加载错误码
var (
	unchange_error = errors.New("unchanged")
)

//内部初始化函数
func init() {
	cfgMng = new(CfgMng)
	cfgMng.Cfg = make(map[string]*cfg)
	cfgMng.Result = make(chan error)
}

//获取配置管理器
func Instance() *CfgMng {
	return cfgMng
}

//初始化函数
//参数root:配置根路径(相对于可执行文件的相对路径)
//参数log:日志接口，用于打印配置组件输出的相关日志
func (cm *CfgMng) Initialize(root string, log *log.Logger) {
	cm.Root = root
	cm.Log = log
}

//功能描述，向配置管理组件注册一个配置文件
//参数name:配置名，必须全局惟一，否则注册时返回失败
//参数path:配置文件相对路径(相对于root路径)
//参数reloadable:配置是否可以重载
//参数alloc:分配配置结构的回调函数，不能为空，每次调用必须返回新创建的具体message结构的指针
//参数prev:装载配置前执行的回调函数，会传入当前的配置结构指针
//参数post:装载配置成功后执行的回调函数，会传入装载后的配置结构指针
func (cm *CfgMng) Register(
	name string,
	path string,
	reloadable bool,
	alloc func() proto.Message,
	prev func(string, proto.Message) (int, error),
	post func(string, proto.Message) (int, error)) (int, error) {

	if alloc == nil {
		return -1, errors.New("register failed,alloc function is nil")
	}

	//已经装载过，不允许再注册
	if cm.Loaded {
		return -1, errors.New("register %s failed,already loaded", name)
	}

	//检查配置项是否已经存在
	_, exist := cm.Cfg[name]
	if exist {
		return -1, errors.New("register %s failed,already exist", name)
	}

	//注册配置项
	c := new(cfg)
	c.Path = path
	c.Reloadable = reloadable
	c.Data[0] = alloc()
	c.Data[1] = alloc()

	//分配双缓存区空间
	if c.Data[0] == nil || c.Data[1] == nil {
		return -1, errors.New("register %s failed,alloc cfg failed", name)
	}

	//注册回调函数
	c.Prev = prev
	c.Post = post

	cm.Cfg[name] = c

	return 0, nil
}

//初始装载函数，装载所有已经注册的配置项
func (cm *CfgMng) InitLoadAll() (int, error) {
	//已经装载过了，禁止再次初始化装载
	if cm.Loaded {
		return -1, errors.New("InitLoadAll failed,already loaded")
	}

	res, e := cm.loadAll(true)
	if e == nil {
		cm.Loaded = true
	}
	return res, e
}

//重载all函数，重载所有已经注册的，并且reloadable为true的配置项
//todo 改为异步执行
func (cm *CfgMng) ReLoadAll() (int, error) {
	return cm.loadAll(false)
}

//重载指定name的配置项，如果该配置项不能重载，返回错误
//todo 改为异步执行
func (cm *CfgMng) ReLoad(name string) (int, error) {
	c, exist := cm.Cfg[name]
	if c == nil || !exist {
		return -1, errors.New("load %s failed,cfg pointer is nil", name)
	}

	if !c.Reloadable {
		return -1, errors.New("load %s failed,cfg not reloadable", name)
	}

	return cm.load(name, c, false)
}

//获取指定name的配置项的函数
func (cm *CfgMng) Get(name string) proto.Message {
	c, exist := cm.Cfg[name]
	if c == nil || !exist {
		return nil
	}

	return c.currBuffer()
}

//打印所有配置项到日志文件
func (cm *CfgMng) PrintAll() {
	if cm.Log == nil {
		return
	}

	for name, c := range cm.Cfg {
		if c != nil {
			b := c.currBuffer()
			if b != nil {
				cm.Log.SimpleInfof("CFG", "------------------- cfg [%s] begin ------------------", name)
				cm.Log.SimpleInfof("CFG", "%s", b.String())
				cm.Log.SimpleInfof("CFG", "------------------- cfg [%s] end   ------------------", name)
			}
		}
	}
}

//内部读取所有配置的接口，任何一个配置项装载失败，即返回失败
//参数initLoad，是否是初始装载
func (cm *CfgMng) loadAll(initLoad bool) (int, error) {
	for name, c := range cm.Cfg {
		if c == nil {
			return -1, errors.New("load %s failed,cfg pointer is nil", name)
		}

		res, e := cm.load(name, c, initLoad)
		if e != nil {
			return res, e
		}
	}

	return 0, nil
}

func (cm *CfgMng) load(name string, c *cfg, initLoad bool) (int, error) {
	//初始装载时全部装载；重载时只加载能重载的配置项
	if c.Reloadable || initLoad {
		//执行prev函数
		if c.Prev != nil {
			res, e := c.Prev(name, c.currBuffer())
			if e != nil {
				return res, nil
			}
		}

		//异步加载
		go loadimpl(c, name, cm.Root, cm.Result)
		e := <-cm.Result //读取管道，等待load结果
		if e != nil {
			if e == unchange_error {
				return 0, nil
			} else {
				return -1, e
			}
		}

		//执行post函数
		if c.Post != nil {
			res, e := c.Post(name, c.nextBuffer())
			if e != nil {
				return res, nil
			}
		}

		//设置装载时间戳
		c.Timestamp = time.Now().Unix()

		//交换缓冲区
		c.switchBuffer()
	}

	return 0, nil
}

//重新加载并保存指定配置
func (cm *CfgMng) ReloadAndSave(name string, text string) error {
	c := cm.Cfg[name]
	if c == nil {
		return errors.New("can't find cfg %s", name)
	}

	buffer := c.nextBuffer()
	if buffer == nil {
		return errors.New("load %s failed, next buffer pointer is nil", name)
	}

	//反序列化
	e := proto.UnmarshalText(text, buffer)
	if e != nil {
		return errors.New("load %s failed, unmarshal failed:%s", name, e)
	}

	//判断配置是否改变
	sign := md5.Sum([]byte(text))
	if bytes.Equal(sign[:], c.md5) {
		return nil
	}
	c.md5 = sign[:]

	//执行post函数
	if c.Post != nil {
		_, e := c.Post(name, buffer)
		if e != nil {
			return e
		}
	}

	//保存文件内容
	e = ioutil.WriteFile(c.Path, []byte(text), 0666)
	if e != nil {
		return errors.New("write file %s failed, %s", c.Path, e)
	}

	//设置装载时间戳
	c.Timestamp = time.Now().Unix()

	//交换缓冲区
	c.switchBuffer()

	return nil
}

//内部读取一个配置的接口,通过routine异步执行
func loadimpl(c *cfg, name string, root string, result chan error) {
	if c == nil {
		result <- errors.New("load %s failed,cfg pointer is nil", name)
		return
	}

	buffer := c.nextBuffer()
	if buffer == nil {
		result <- errors.New("load %s failed,next buffer pointer is nil", name)
		return
	}

	//拼接路径
	var path string
	if root == "" {
		path = c.Path
	} else {
		path = root + utils.GetPathDel() + c.Path
	}

	//读取文件内容
	data, e := ioutil.ReadFile(path)
	if e != nil {
		result <- errors.New("load %s failed,read file %s failed:%s", name, path, e.Error())
		return
	}

	//反序列化
	e = proto.UnmarshalText(string(data), buffer)
	if e != nil {
		result <- errors.New("load %s failed,unmarshal file %s failed:%s", name, path, e.Error())
		return
	}

	//判断配置是否改变
	sign := md5.Sum(data)
	if bytes.Equal(sign[:], c.md5) {
		result <- unchange_error
		return
	} else {
		c.md5 = sign[:]
	}

	result <- nil
}

//内部获取当前缓存区的函数
func (c *cfg) currBuffer() proto.Message {
	return c.Data[c.CurrIdx]
}

//内部获取下一个缓存区的函数
func (c *cfg) nextBuffer() proto.Message {
	return c.Data[(c.CurrIdx+1)%2]
}

//内部切换缓冲区的函数
func (c *cfg) switchBuffer() {
	c.CurrIdx = (c.CurrIdx + 1) % 2
}
