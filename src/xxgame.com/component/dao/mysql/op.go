/*
实现具体的mysql dao的操作
*/

package mysql

import (
	_ "database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"xxgame.com/component/dao/types"
	"xxgame.com/types/proto/interior"
	"xxgame.com/utils/errors"
)

//hash函数实现
//3级id索引hash函数
func ThreeNumIndexHashFunc1(num int, input interface{}) int {
	if num <= 0 {
		return 0
	}

	if v, ok := input.(*interior.Data); ok {
		return int(v.GetNumIndex1() % uint64(num))
	} else {
		return 0
	}
}

//1级字符串索引hash函数
//todo 优化算法
func OneStringIndexHashFunc(num int, input interface{}) int {
	if num <= 0 {
		return 0
	}

	var result uint64
	if v, ok := input.(*interior.Data); ok {
		if v.GetStrIndex() == nil {
			return 0
		}

		for _, v := range []byte(v.GetStrIndex()) {
			result += uint64(v)
		}

		return int(result % uint64(num))
	} else {
		return 0
	}
}

//DataOp接口实现
type MysqlDataOp struct{}

//准备查询语句函数
func (op *MysqlDataOp) Prepare(idx int, indexType interior.IndexType, opType int) *string {
	switch opType {
	case types.Select:
		return prepare_select(idx, indexType)
	case types.Insert:
		return prepare_insert(idx, indexType)
	case types.Update:
		return prepare_update(idx, indexType)
	case types.Delete:
		return prepare_delete(idx, indexType)
	default:
		return nil
	}
}

//执行查询函数
func (op *MysqlDataOp) Exec(d types.Dao, indexType interior.IndexType, opType int, input interface{}) (int64, error) {
	switch opType {
	case types.Select:
		return exec_select(d, indexType, input)
	case types.Insert:
		return exec_insert(d, indexType, input)
	case types.Update:
		return exec_update(d, indexType, input)
	case types.Delete:
		return exec_delete(d, indexType, input)
	default:
		return -1, errors.New("invalid opType %d", opType)
	}
}

func prepare_select(idx int, indexType interior.IndexType) *string {
	var res string
	switch indexType {
	case interior.IndexType_Number1:
		res = fmt.Sprintf("select dataid,index1,index2,type,version,buffer from tbl_num_index_%03d where dataid=? and index1=?", idx+1)
	case interior.IndexType_Number2:
		res = fmt.Sprintf("select dataid,index1,index2,type,version,buffer from tbl_num_index_%03d where dataid=? and index1=? and index2=?", idx+1)
	case interior.IndexType_String:
		res = fmt.Sprintf("select dataid,index1,type,version,buffer from tbl_str_index_%03d where dataid=? and index1=?", idx+1)
	default:
		return nil
	}

	return &res
}

func prepare_insert(idx int, indexType interior.IndexType) *string {
	var res string
	switch indexType {
	case interior.IndexType_Number1:
		fallthrough
	case interior.IndexType_Number2:
		res = fmt.Sprintf("insert tbl_num_index_%03d  (dataid,index1,index2,type,version,buffer,createtime,updatetime) values (?,?,?,?,?,?,?,?)", idx+1)
	case interior.IndexType_String:
		res = fmt.Sprintf("insert tbl_str_index_%03d  (dataid,index1,type,version,buffer,createtime,updatetime) values (?,?,?,?,?,?,?)", idx+1)
	default:
		return nil
	}

	return &res
}

func prepare_update(idx int, indexType interior.IndexType) *string {
	var res string
	switch indexType {
	case interior.IndexType_Number1:
		res = fmt.Sprintf("update tbl_num_index_%03d set version=?,buffer=?,updatetime=? where dataid = ? and index1 = ?", idx+1)
	case interior.IndexType_Number2:
		res = fmt.Sprintf("update tbl_num_index_%03d set version=?,buffer=?,updatetime=? where dataid = ? and index1 = ? and index2 = ?", idx+1)
	case interior.IndexType_String:
		res = fmt.Sprintf("update tbl_str_index_%03d set version=?,buffer=?,updatetime=? where dataid = ? and index1 = ?", idx+1)
	default:
		return nil
	}
	return &res
}

func prepare_delete(idx int, indexType interior.IndexType) *string {
	var res string
	switch indexType {
	case interior.IndexType_Number1:
		res = fmt.Sprintf("delete from tbl_num_index_%03d where dataid=? and index1=?", idx+1)
	case interior.IndexType_Number2:
		res = fmt.Sprintf("delete from tbl_num_index_%03d where dataid=? and index1=? and index2=?", idx+1)
	case interior.IndexType_String:
		res = fmt.Sprintf("delete from tbl_str_index_%03d where dataid=? and index1=?", idx+1)
	default:
		return nil
	}

	return &res
}

func exec_select(d types.Dao, indexType interior.IndexType, input interface{}) (int64, error) {
	if v, ok := d.(*MysqlDao); ok && v != nil && v.curStmt != nil {
		switch indexType {
		case interior.IndexType_Number1:
			if i, ok2 := input.(*interior.Data); ok2 && i != nil {
				res, e := v.curStmt.Query(i.GetDataID(), i.GetNumIndex1())
				if e == nil {
					//保存结果
					v.curRows = res
					return 0, nil
				} else {
					return -1, errors.New("select failed,%s", e)
				}
			} else {
				return -1, errors.New("select failed,unsupported input")
			}
		case interior.IndexType_Number2:
			if i, ok2 := input.(*interior.Data); ok2 && i != nil {
				res, e := v.curStmt.Query(i.GetDataID(), i.GetNumIndex1(), i.GetNumIndex2())
				if e == nil {
					//保存结果
					v.curRows = res
					return 0, nil
				} else {
					return -1, errors.New("select failed,%s", e)
				}
			} else {
				return -1, errors.New("select failed,unsupported input")
			}
		case interior.IndexType_String:
			if i, ok2 := input.(*interior.Data); ok2 && i != nil {
				res, e := v.curStmt.Query(i.GetDataID(), i.GetStrIndex())
				if e == nil {
					//保存结果
					v.curRows = res
					return 0, nil
				} else {
					return -1, errors.New("select failed,%s", e)
				}
			} else {
				return -1, errors.New("select failed,unsupported input")
			}
		default:
			return -1, errors.New("select failed,invalid dsType %d", indexType)
		}
	} else {
		return -1, errors.New("select failed,dao is nil")
	}
}

func exec_insert(d types.Dao, indexType interior.IndexType, input interface{}) (int64, error) {
	timestamp := time.Now().Unix()
	if v, ok := d.(*MysqlDao); ok && v != nil && v.curStmt != nil {
		switch indexType {
		case interior.IndexType_Number1:
			fallthrough
		case interior.IndexType_Number2:
			if i, ok2 := input.(*interior.Data); ok2 && i != nil {
				res, e := v.curStmt.Exec(i.GetDataID(), i.GetNumIndex1(), i.GetNumIndex2(), i.GetDataType(), i.GetVersion(), i.GetDataBuffer(), timestamp, 0)
				if e == nil {
					//获取结果
					insertid, _ := res.LastInsertId()
					return insertid, nil
				} else {
					return -1, errors.New("insert failed,%s", e)
				}
			} else {
				return -1, errors.New("insert failed,unsupported input")
			}
		case interior.IndexType_String:
			if i, ok2 := input.(*interior.Data); ok2 && i != nil {
				res, e := v.curStmt.Exec(i.GetDataID(), i.GetStrIndex(), i.GetDataType(), i.GetVersion(), i.GetDataBuffer(), timestamp, 0)
				if e == nil {
					//获取结果
					insertid, _ := res.LastInsertId()
					return insertid, nil
				} else {
					return -1, errors.New("insert failed,%s", e)
				}
			} else {
				return -1, errors.New("insert failed,unsupported input")
			}
		default:
			return -1, errors.New("insert failed,invalid dsType %d", indexType)
		}
	} else {
		return -1, errors.New("insert failed,dao is nil")
	}
}

func exec_update(d types.Dao, indexType interior.IndexType, input interface{}) (int64, error) {
	timestamp := time.Now().Unix()
	if v, ok := d.(*MysqlDao); ok && v != nil && v.curStmt != nil {
		switch indexType {
		case interior.IndexType_Number1:
			if i, ok2 := input.(*interior.Data); ok2 && i != nil {
				res, e := v.curStmt.Exec(i.GetVersion(), i.GetDataBuffer(), timestamp, i.GetDataID(), i.GetNumIndex1())
				if e == nil {
					//获取结果
					rownum, _ := res.RowsAffected()
					return rownum, nil
				} else {
					return -1, errors.New("update failed,%s", e)
				}
			} else {
				return -1, errors.New("update failed,unsupported input")
			}
		case interior.IndexType_Number2:
			if i, ok2 := input.(*interior.Data); ok2 && i != nil {
				res, e := v.curStmt.Exec(i.GetVersion(), i.GetDataBuffer(), timestamp, i.GetDataID(), i.GetNumIndex1(), i.GetNumIndex2())
				if e == nil {
					//获取结果
					rownum, _ := res.RowsAffected()
					return rownum, nil
				} else {
					return -1, errors.New("update failed,%s", e)
				}
			} else {
				return -1, errors.New("update failed,unsupported input")
			}
		case interior.IndexType_String:
			if i, ok2 := input.(*interior.Data); ok2 && i != nil {
				res, e := v.curStmt.Exec(i.GetVersion(), i.GetDataBuffer(), timestamp, i.GetDataID(), i.GetStrIndex())
				if e == nil {
					//获取结果
					rownum, _ := res.RowsAffected()
					return rownum, nil
				} else {
					return -1, errors.New("update failed,%s", e)
				}
			} else {
				return -1, errors.New("update failed,unsupported input")
			}
		default:
			return -1, errors.New("update failed,invalid dsType %d", indexType)
		}
	} else {
		return -1, errors.New("update failed,dao is nil")
	}
}

func exec_delete(d types.Dao, indexType interior.IndexType, input interface{}) (int64, error) {
	if v, ok := d.(*MysqlDao); ok && v != nil && v.curStmt != nil {
		switch indexType {
		case interior.IndexType_Number1:
			if i, ok2 := input.(*interior.Data); ok2 && i != nil {
				res, e := v.curStmt.Exec(i.GetDataID(), i.GetNumIndex1())
				if e == nil {
					//获取结果
					rownum, _ := res.RowsAffected()
					return rownum, nil
				} else {
					return -1, errors.New("delete failed,%s", e)
				}
			} else {
				return -1, errors.New("delete failed,unsupported input")
			}
		case interior.IndexType_Number2:
			if i, ok2 := input.(*interior.Data); ok2 && i != nil {
				res, e := v.curStmt.Exec(i.GetDataID(), i.GetNumIndex1(), i.GetNumIndex2())
				if e == nil {
					//获取结果
					rownum, _ := res.RowsAffected()
					return rownum, nil
				} else {
					return -1, errors.New("delete failed,%s", e)
				}
			} else {
				return -1, errors.New("delete failed,unsupported input")
			}
		case interior.IndexType_String:
			if i, ok2 := input.(*interior.Data); ok2 && i != nil {
				res, e := v.curStmt.Exec(i.GetDataID(), i.GetStrIndex())
				if e == nil {
					//获取结果
					rownum, _ := res.RowsAffected()
					return rownum, nil
				} else {
					return -1, errors.New("delete failed,%s", e)
				}
			} else {
				return -1, errors.New("delete failed,unsupported input")
			}
		default:
			return -1, errors.New("delete failed,invalid dsType %d", indexType)
		}
	} else {
		return -1, errors.New("delete failed,dao is nil")
	}
}
