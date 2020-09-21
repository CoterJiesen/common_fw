/*
数据持久化包的基础接口定义
*/

//数据持久化包
package dao

import (
	"xxgame.com/component/dao/mysql"
	"xxgame.com/component/dao/types"
)

//创建实例的函数
//	name为dao使用的数据引擎类型，目前只支持mysql
func New(name string) types.Dao {
	var d types.Dao
	switch name {
	case "mysql":
		d = mysql.New()
	default:
		d = nil
	}
	return d
}
