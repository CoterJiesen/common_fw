/*
dao相关类型定义
*/
package types

import (
	"xxgame.com/types/proto/interior"
	"xxgame.com/utils/errors"
)

//操作类型
const (
	Insert = 0 //插入
	Select = 1 //查询
	Delete = 2 //删除
	Update = 3 //更新
)

//分表数
const (
	TableNum  = 512
	BufferLen = 255
	IndexLen  = 127
)

//基础dao类型
type Dao interface {
	//数据操作
	Op(indexType interior.IndexType, opType int, input interface{}) (int64, error)
	//是否有更多数据返回
	Next() bool
	//获取一条数据
	Get(indexType interior.IndexType, output interface{}) error
	//获取数据完成
	Done()
	//销毁
	Fin()
}

/*
//数据集合类型
const (
	NumIndex1    = 0 //一级数值索引
	NumIndex2    = 1 //二级数值索引
	NumIndex3    = 2 //三级数值索引
	StringIndex1 = 3 //一级字符串索引
)

//数据值
type DataValue struct {
	Type    uint8  //数据类型
	Version uint64 //数据版本
	Buff    []byte //数据buffer
}

//三级数值索引数据集合
type ThreeNumIndexDS struct {
	Index1 uint64    //一级索引
	Index2 uint64    //二级索引
	Index3 uint64    //三级索引
	Value  DataValue //数据值
}

//一级字符串索引数据集合
type OneStringIndexDS struct {
	Index string    //索引
	Value DataValue //数据值
}
*/

//数据操作
type DataOp interface {
	Prepare(idx int, indexType interior.IndexType, opType int) *string                      //提供准备语句的函数
	Exec(d Dao, indexType interior.IndexType, opType int, input interface{}) (int64, error) //执行函数
}

//数据集
type DataSet struct {
	HashFunc func(num int, input interface{}) int
	DataOp   map[int]DataOp
	Num      int
}

func (ds *DataSet) AddOp(opType int, d DataOp) {
	if ds.DataOp == nil {
		ds.DataOp = make(map[int]DataOp)
	}
	ds.DataOp[opType] = d
}

//数据集列表
type DataSetMap struct {
	Data map[interior.IndexType]*DataSet
}

func (d *DataSetMap) Register(indexType interior.IndexType, ds *DataSet) error {
	if d.Data == nil {
		d.Data = make(map[interior.IndexType]*DataSet)
	}

	if _, exist := d.Data[indexType]; exist {
		return errors.New("register type %d dataset failed,already exist", indexType)
	}

	d.Data[indexType] = ds

	return nil
}
