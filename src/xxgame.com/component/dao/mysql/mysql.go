//封装mysql数据操作的dao
package mysql

import (
	"database/sql"
	"xxgame.com/component/dao/types"
	"xxgame.com/types/proto/interior"
	"xxgame.com/utils/errors"
)

const (
	Name string = "mysql"
)

var (
	DB    *sql.DB           //共享数据库连接池
	Stmts [][][]*sql.Stmt   //预编译sql语句
	DsMap *types.DataSetMap //数据集说明
)

//mysql的dao
type MysqlDao struct {
	curStmt *sql.Stmt //当前处理的stmt
	curRows *sql.Rows //当前保存的结果集合
}

//url的格式为user:pass@tcp(host:port)/dbname
//连接数据库，并准备好stmt
func Init(url string, dsMap *types.DataSetMap) error {
	var e error
	DB, e = sql.Open(Name, url)
	if e != nil {
		panic(e)
	}
	DB.SetMaxOpenConns(5000)
	DB.SetMaxIdleConns(1000)
	DB.Ping()

	DsMap = dsMap

	//初始化预编译sql语句
	var indexType interior.IndexType
	var ds *types.DataSet
	Stmts = make([][][]*sql.Stmt, len(dsMap.Data))
	for indexType, ds = range dsMap.Data {
		//操作类型
		Stmts[indexType] = make([][]*sql.Stmt, len(ds.DataOp))
		for opType, op := range ds.DataOp {
			if op == nil {
				panic(errors.New("create stmt for type %d op %d failed,op is nil", indexType, opType))
			}

			//分表
			Stmts[indexType][opType] = make([]*sql.Stmt, ds.Num)
			for i := int(0); i < ds.Num; i++ {
				sql := op.Prepare(i, indexType, opType)
				if sql == nil {
					panic(errors.New("create stmt for type %d op %d idx %d failed,get prepare sql failed", indexType, opType, i))
				}

				Stmts[indexType][opType][i], e = DB.Prepare(*sql)
				if e != nil {
					panic(errors.New("create stmt for type %d op %d idx %d failed,prepare %s failed: %s", indexType, opType, i, *sql, e))
				}
			}
		}
	}

	return nil
}

func New() *MysqlDao {
	return new(MysqlDao)
}

//数据操作
func (d *MysqlDao) Op(indexType interior.IndexType, opType int, input interface{}) (int64, error) {
	//获取数据操作
	op, e := d.getstmt(indexType, opType, input)
	if e != nil || op == nil {
		return -1, e
	}

	//执行sql语句
	return op.Exec(d, indexType, opType, input)
}

//是否有更多数据返回
func (d *MysqlDao) Next() bool {
	if d.curRows == nil {
		return false
	}
	return d.curRows.Next()
}

//获取一条数据
func (d *MysqlDao) Get(indexType interior.IndexType, output interface{}) error {
	if d.curRows == nil {
		return errors.New("get result failed,curRows is nil")
	}

	v, ok := output.(*interior.Data)
	if !ok {
		return errors.New("get result failed,unsupported output type")
	}

	switch indexType {
	case interior.IndexType_Number1:
		fallthrough
	case interior.IndexType_Number2:
		d.curRows.Scan(v.DataID, v.NumIndex1, v.NumIndex2, v.DataType, v.Version, &v.DataBuffer)
	case interior.IndexType_String:
		d.curRows.Scan(v.DataID, &v.StrIndex, v.DataType, v.Version, &v.DataBuffer)
	default:
		return errors.New("get result failed,unsupported output type")
	}

	return nil
}

//获取数据完成
func (d *MysqlDao) Done() {
	d.curStmt = nil
	d.curRows.Close()
	d.curRows = nil
}

//销毁
func (d *MysqlDao) Fin() {
	d.curStmt = nil
	d.curRows = nil
}

//获取sql预编译语句
func (d *MysqlDao) getstmt(indexType interior.IndexType, opType int, input interface{}) (types.DataOp, error) {
	var ds *types.DataSet
	var exist bool
	if ds, exist = DsMap.Data[indexType]; !exist || ds == nil {
		return nil, errors.New("Op(%d,%d) failed,can't find ds", indexType, opType)
	}

	var op types.DataOp
	if op, exist = ds.DataOp[opType]; !exist || op == nil {
		return nil, errors.New("Op(%d,%d) failed,can't find op", indexType, opType)
	}

	//计算分表索引
	idx := ds.HashFunc(ds.Num, input)

	//获取stmt
	d.curStmt = Stmts[indexType][opType][idx]
	if d.curStmt == nil {
		return nil, errors.New("Op(%d,%d,%d) failed,can't find stmt", indexType, opType, idx)
	}

	return op, nil
}
