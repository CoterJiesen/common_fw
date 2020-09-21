/*
对象管理组件，routine安全
*/
package obj

import (
	"sync"
	"xxgame.com/component/uuid"
	"xxgame.com/utils/errors"
)

//对象接口定义
type Object interface {
	GetID() uint32 //获取对象id

	Init() error     //初始化对象
	Fin()            //销毁对象
	setID(id uint32) //设置对象id(不对外暴露)
}

//对象管理器接口定义
type ObMng interface {
	Init(cap int, idGen *uuid.UuidGen, creator func() Object) error //初始化对象管理器
	Create() (Object, error)                                        //创建对象
	Del(id uint32) error                                            //销毁对象
	Get(id uint32) (Object, error)                                  //获取对象
	Clear()                                                         //清除所有对象
	Cap() int                                                       //获取容量
	Len() int                                                       //获取大小
	IsFull() bool                                                   //是否满
}

//基础对象定义
type BaseObject struct {
	id uint32 //uuid
}

//获取对象uuid
func (b *BaseObject) GetID() uint32 {
	return b.id
}

//初始化对象时的操作
func (b *BaseObject) Init() error {
	return nil
}

//销毁对象时的操作
func (b *BaseObject) Fin() {
}

//设置对象id
func (b *BaseObject) setID(id uint32) {
	if b.id != 0 {
		panic("set object id more than one time")
	}

	b.id = id
}

//基础对象管理器定义
type BaseObMng struct {
	cap     int               //容量
	inited  bool              //是否已经初始化过
	idGen   *uuid.UuidGen     //id生成器
	obMap   map[uint32]Object //存储对象的map
	creator func() Object     //创建对象的方法
	lock    sync.RWMutex      //读写锁
}

//初始化对象管理器
//参数cap:最大管理对象个数,小于等于0表示不限大小
//参数idGen:id生成器
//参数createor:创建对象的方法
//失败返回错误
func (b *BaseObMng) Init(cap int, idGen *uuid.UuidGen, creator func() Object) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	if b.inited {
		return errors.New("already inited")
	}

	if idGen == nil {
		return errors.New("idGen is nil")
	}

	if creator == nil {
		return errors.New("creator is nil")
	}

	if cap < 0 {
		b.cap = 0
	} else {
		b.cap = cap
	}

	b.idGen = idGen
	b.inited = true
	b.creator = creator

	//根据容量创建map
	b.obMap = make(map[uint32]Object, b.cap)

	return nil
}

//创建对象
func (b *BaseObMng) Create() (Object, error) {
	b.lock.Lock()
	defer b.lock.Unlock()
	if b.creator == nil {
		return nil, errors.New("creator is nil")
	}

	if b.idGen == nil {
		return nil, errors.New("idGen is nil")
	}

	//检查容量
	if b.isFull() {
		return nil, errors.New("object num exceeded %d", b.cap)
	}

	//创建对象
	o := b.creator()
	//设置id
	o.setID(b.idGen.GenID())
	//初始化对象
	e := o.Init()
	if e != nil {
		return nil, e
	}
	//保存到map中
	b.obMap[o.GetID()] = o

	return o, nil
}

//销毁对象
func (b *BaseObMng) Del(id uint32) error {
	b.lock.Lock()
	defer b.lock.Unlock()
	o, e := b.get(id)
	if o == nil || e != nil {
		return errors.New("del obj %d failed", id)
	}

	o.Fin()
	delete(b.obMap, id)

	return nil
}

//获取对象，不加锁供内部使用
func (b *BaseObMng) get(id uint32) (Object, error) {
	o, exist := b.obMap[id]
	if !exist {
		return nil, errors.New("obj %d not exist", id)
	}

	if o == nil {
		delete(b.obMap, id) //异常处理
		return nil, errors.New("obj %d is nil", id)
	}

	return o, nil
}

//获取对象
func (b *BaseObMng) Get(id uint32) (Object, error) {
	b.lock.RLock()
	defer b.lock.RUnlock()

	return b.get(id)
}

//清除所有对象
func (b *BaseObMng) Clear() {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.obMap = make(map[uint32]Object, b.cap)
}

//获取容量
func (b *BaseObMng) Cap() int {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.cap
}

//获取大小，不加锁供内部使用
func (b *BaseObMng) len() int {
	return len(b.obMap)
}

//获取大小
func (b *BaseObMng) Len() int {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.len()
}

//是否满，不加锁供内部使用
func (b *BaseObMng) isFull() bool {
	return b.cap > 0 && b.len() >= b.cap
}

//是否满
func (b *BaseObMng) IsFull() bool {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.isFull()
}
