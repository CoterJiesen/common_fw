package dao

import (
	"github.com/golang/protobuf/proto"
	"testing"
	"xxgame.com/component/dao/mysql"
	"xxgame.com/component/dao/types"
	"xxgame.com/types/proto/interior"
)

func testNumIndexData(t testing.TB, d types.Dao) {
	//测试插入操作
	var ret int64
	var e error

	var dt interior.DataType = interior.DataType_Int64
	data2 := interior.Data{proto.Uint64(1), proto.Uint64(2), proto.Uint64(1), nil, &dt, proto.Uint64(0), []byte{1, 2, 3}, nil}

	ret, e = d.Op(interior.IndexType_Number2, types.Insert, &data2)
	_ = ret
	if e != nil {
		t.Errorf(e.Error())
	}

	data1 := interior.Data{proto.Uint64(1), proto.Uint64(2), proto.Uint64(2), nil, &dt, proto.Uint64(0), []byte{1, 2, 3}, nil}
	ret, e = d.Op(interior.IndexType_Number1, types.Insert, &data1)
	_ = ret
	if e != nil {
		t.Errorf(e.Error())
	}

	//测试更新操作

	*data2.Version++
	ret, e = d.Op(interior.IndexType_Number2, types.Update, &data2)
	_ = ret
	if e != nil {
		t.Errorf(e.Error())
	}

	*data1.Version++
	ret, e = d.Op(interior.IndexType_Number1, types.Update, &data1)
	_ = ret
	if e != nil {
		t.Errorf(e.Error())
	}

	//测试查询
	ret, e = d.Op(interior.IndexType_Number1, types.Select, &data1)
	if e != nil {
		t.Errorf(e.Error())
	}
	for d.Next() {
		var i interior.DataType
		out := interior.Data{proto.Uint64(0), proto.Uint64(0), proto.Uint64(0), nil, &i, proto.Uint64(0), nil, nil}
		out.DataBuffer = make([]byte, 0, types.BufferLen)
		e = d.Get(interior.IndexType_Number1, &out)
		if e != nil {
			t.Errorf(e.Error())
		}
		t.Logf("data: %d %d %d %d %d %v\n", out.GetDataID(), out.GetNumIndex1(), out.GetNumIndex2(), out.GetDataType(), out.GetVersion(), out.GetDataBuffer())
	}
	d.Done()

	//测试删除
	ret, e = d.Op(interior.IndexType_Number2, types.Delete, &data2)
	if e != nil || ret != 1 {
		t.Errorf(e.Error())
	}

	ret, e = d.Op(interior.IndexType_Number2, types.Delete, &data1)
	if e != nil || ret != 1 {
		t.Errorf(e.Error())
	}
}

func testStrIndexData(t testing.TB, d types.Dao) {

	//测试插入操作
	var ret int64
	var e error

	dt := interior.DataType_Buffer

	data3 := interior.Data{proto.Uint64(1), nil, nil, []byte("test1"), &dt, proto.Uint64(0), []byte{1, 2, 3}, nil}
	ret, e = d.Op(interior.IndexType_String, types.Insert, &data3)
	_ = ret
	if e != nil {
		t.Errorf(e.Error())
	}

	data2 := interior.Data{proto.Uint64(1), nil, nil, []byte("test2"), &dt, proto.Uint64(0), []byte{1, 2, 3}, nil}
	ret, e = d.Op(interior.IndexType_String, types.Insert, &data2)
	_ = ret
	if e != nil {
		t.Errorf(e.Error())
	}

	data1 := interior.Data{proto.Uint64(1), nil, nil, []byte("test3"), &dt, proto.Uint64(0), []byte{1, 2, 3}, nil}
	ret, e = d.Op(interior.IndexType_String, types.Insert, &data1)
	_ = ret
	if e != nil {
		t.Errorf(e.Error())
	}

	//测试更新操作
	*data3.Version++
	ret, e = d.Op(interior.IndexType_String, types.Update, &data3)
	_ = ret
	if e != nil {
		t.Errorf(e.Error())
	}

	*data2.Version++
	ret, e = d.Op(interior.IndexType_String, types.Update, &data2)
	_ = ret
	if e != nil {
		t.Errorf(e.Error())
	}

	*data1.Version++
	ret, e = d.Op(interior.IndexType_String, types.Update, &data1)
	_ = ret
	if e != nil {
		t.Errorf(e.Error())
	}

	//测试查询
	ret, e = d.Op(interior.IndexType_String, types.Select, &data1)
	if e != nil {
		t.Errorf(e.Error())
	}
	for d.Next() {
		var i interior.DataType
		out := interior.Data{proto.Uint64(0), proto.Uint64(0), proto.Uint64(0), nil, &i, proto.Uint64(0), nil, nil}
		out.StrIndex = make([]byte, 0, types.IndexLen)
		out.DataBuffer = make([]byte, 0, types.BufferLen)
		e = d.Get(interior.IndexType_String, &out)
		if e != nil {
			t.Errorf(e.Error())
		}
		t.Logf("data: %d %s %d %d %v\n", out.GetDataID(), out.GetStrIndex(), out.GetDataType(), out.GetVersion(), out.GetDataBuffer())
	}
	d.Done()

	//测试删除
	ret, e = d.Op(interior.IndexType_String, types.Delete, &data3)
	if e != nil || ret != 1 {
		t.Errorf(e.Error())
	}

	ret, e = d.Op(interior.IndexType_String, types.Delete, &data2)
	if e != nil || ret != 1 {
		t.Errorf(e.Error())
	}

	ret, e = d.Op(interior.IndexType_String, types.Delete, &data1)
	if e != nil || ret != 1 {
		t.Errorf(e.Error())
	}

}

func initDB() types.Dao {
	d := New("mysql")

	if d == nil {
		return nil
	}

	//创建mysql op
	op := new(mysql.MysqlDataOp)

	//创建mysql data set
	dsMysqlNumIndex1 := types.DataSet{mysql.ThreeNumIndexHashFunc1, nil, types.TableNum}
	dsMysqlNumIndex1.AddOp(types.Select, op)
	dsMysqlNumIndex1.AddOp(types.Insert, op)
	dsMysqlNumIndex1.AddOp(types.Delete, op)
	dsMysqlNumIndex1.AddOp(types.Update, op)

	dsMysqlNumIndex2 := types.DataSet{mysql.ThreeNumIndexHashFunc1, nil, types.TableNum}
	dsMysqlNumIndex2.AddOp(types.Select, op)
	dsMysqlNumIndex2.AddOp(types.Insert, op)
	dsMysqlNumIndex2.AddOp(types.Delete, op)
	dsMysqlNumIndex2.AddOp(types.Update, op)

	dsMysqlStrIndex := types.DataSet{mysql.OneStringIndexHashFunc, nil, types.TableNum}
	dsMysqlStrIndex.AddOp(types.Select, op)
	dsMysqlStrIndex.AddOp(types.Insert, op)
	dsMysqlStrIndex.AddOp(types.Delete, op)
	dsMysqlStrIndex.AddOp(types.Update, op)

	//创建mysql data set map
	var dsMap types.DataSetMap
	dsMap.Register(interior.IndexType_Number1, &dsMysqlNumIndex1)
	dsMap.Register(interior.IndexType_Number2, &dsMysqlNumIndex2)
	dsMap.Register(interior.IndexType_String, &dsMysqlStrIndex)

	//初始化dao
	e := d.Init("ximi:ximi@tcp(127.0.0.1:3306)/coredata", &dsMap)
	if e != nil {
		panic(e)
		return nil
	}

	return d
}

var d types.Dao = initDB()

func TestAll(t *testing.T) {

	//测试三级数值索引
	testNumIndexData(t, d)
	//测试字符串索引
	testStrIndexData(t, d)
}

func BenchmarkAll(t *testing.B) {

	for i := 0; i < t.N; i++ {
		testNumIndexData(t, d)
		testStrIndexData(t, d)
	}
}
