package datasdk

import (
	"xxgame.com/types/proto/interior"
)

// 数据操作接口
type DataOperator interface {
	OnFinished(result *interior.DataOpResponse) // 操作结果
}
