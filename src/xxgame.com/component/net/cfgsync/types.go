package cfgsync

import (
	"github.com/golang/protobuf/proto"
)

// 配置的所有者
type CfgOwner interface {
	OnNewCfg(cfgName string, cfg proto.Message) (int, error) // 获取到新的配置
}
