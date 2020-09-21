/*
uuid组件：提供不重复的id生成器(其中0值保留，代表无效值)
*/
package uuid

import (
	"sync/atomic"
)

//uuid生成器
type UuidGen struct {
	id uint32 //内部id，32位无符号整数，0值保留，代表无效值
}

//返回一个uuid
func (u *UuidGen) GenID() uint32 {
	//这里可以保证惟一性，但不能保证分配的连续性
	if u.id == 0xffffffff {
		return atomic.AddUint32(&u.id, 2)
	} else {
		return atomic.AddUint32(&u.id, 1)
	}
}
