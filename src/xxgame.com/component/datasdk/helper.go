package datasdk

import (
	"encoding/binary"
	"fmt"
)

import (
	"github.com/golang/protobuf/proto"
	"xxgame.com/types/proto/interior"
	"xxgame.com/utils/errors"
)

// 分配新的数据块
//	dataId: 已注册的数据id
//	ver: 数据版本号
//	numIdx: 数值索引，长度最长为2
//	strIdx: 字符串索引，若传入的数值索引切片的长度不为0则忽略该参数
func NewData(dataId uint64, ver uint64, numIdx []uint64, strIdx []byte) *interior.Data {
	k := ""
	if len(numIdx) == 2 {
		k = fmt.Sprintf("%v.%v", dataId, numIdx[1])
	} else {
		k = fmt.Sprintf("%v", dataId)
	}
	r, exists := sdkInstance.rmap[k]
	if !exists {
		panic(fmt.Sprintf("no rule registered for data %d!", dataId))
	}
	data := new(interior.Data)
	data.DataID = proto.Uint64(dataId)
	data.Version = proto.Uint64(ver)

	indexLen := len(numIdx)
	switch r.IndexType {
	case interior.IndexType_Number1:
		if indexLen == 0 {
			panic("no number index is available for data!")
		}
		data.NumIndex1 = proto.Uint64(numIdx[0])
	case interior.IndexType_Number2:
		if indexLen == 0 {
			panic("2 number index is available for data!")
		}
		data.NumIndex1 = proto.Uint64(numIdx[0])

		if (r.OpMask&OP_GET) == 0 && indexLen < 2 {
			panic("2 number index should be given for data!")
		}
		if indexLen >= 2 {
			data.NumIndex2 = proto.Uint64(numIdx[1])
		}
	case interior.IndexType_String:
		if len(strIdx) == 0 {
			panic("invalid string index for data!")
		}
		data.StrIndex = strIdx
	}
	data.DataType = &r.DataType
	return data
}

func PutInt32(d *interior.Data, v int32) error {
	if d == nil {
		return errors.New("invalid parameter")
	}
	if d.GetDataType() != interior.DataType_Int32 {
		return errors.New("invalid data type %s", d.GetDataType())
	}
	d.DataBuffer = make([]byte, 4)
	binary.BigEndian.PutUint32(d.DataBuffer, uint32(v))
	return nil
}

func PutUInt32(d *interior.Data, v uint32) error {
	if d == nil {
		return errors.New("invalid parameter")
	}
	if d.GetDataType() != interior.DataType_UInt32 {
		return errors.New("invalid data type %s", d.GetDataType())
	}
	d.DataBuffer = make([]byte, 4)
	binary.BigEndian.PutUint32(d.DataBuffer, v)
	return nil
}

func PutInt64(d *interior.Data, v int64) error {
	if d == nil {
		return errors.New("invalid parameter")
	}
	if d.GetDataType() != interior.DataType_Int64 {
		return errors.New("invalid data type %s", d.GetDataType())
	}
	d.DataBuffer = make([]byte, 8)
	binary.BigEndian.PutUint64(d.DataBuffer, uint64(v))
	return nil
}

func PutUInt64(d *interior.Data, v uint64) error {
	if d == nil {
		return errors.New("invalid parameter")
	}
	if d.GetDataType() != interior.DataType_UInt64 {
		return errors.New("invalid data type %s", d.GetDataType())
	}
	d.DataBuffer = make([]byte, 8)
	binary.BigEndian.PutUint64(d.DataBuffer, v)
	return nil
}

func PutBuffer(d *interior.Data, b []byte) error {
	if d == nil {
		return errors.New("invalid parameter")
	}
	if d.GetDataType() != interior.DataType_Buffer {
		return errors.New("invalid data type %s", d.GetDataType())
	}
	d.DataBuffer = make([]byte, len(b))
	copy(d.DataBuffer, b)
	return nil
}

func GetInt32(d *interior.Data) (int32, error) {
	if d == nil {
		return 0, errors.New("invalid parameter")
	}
	if d.GetDataType() != interior.DataType_Int32 {
		return 0, errors.New("invalid data type %s", d.GetDataType())
	}
	if len(d.DataBuffer) != 4 {
		return 0, errors.New("invalid data %v", d.DataBuffer)
	}
	return int32(binary.BigEndian.Uint32(d.DataBuffer)), nil
}

func GetUInt32(d *interior.Data) (uint32, error) {
	if d == nil {
		return 0, errors.New("invalid parameter")
	}
	if d.GetDataType() != interior.DataType_UInt32 {
		return 0, errors.New("invalid data type %s", d.GetDataType())
	}
	if len(d.DataBuffer) != 4 {
		return 0, errors.New("invalid data %v", d.DataBuffer)
	}
	return binary.BigEndian.Uint32(d.DataBuffer), nil
}

func GetInt64(d *interior.Data) (int64, error) {
	if d == nil {
		return 0, errors.New("invalid parameter")
	}
	if d.GetDataType() != interior.DataType_Int64 {
		return 0, errors.New("invalid data type %s", d.GetDataType())
	}
	if len(d.DataBuffer) != 8 {
		return 0, errors.New("invalid data %v", d.DataBuffer)
	}
	return int64(binary.BigEndian.Uint64(d.DataBuffer)), nil
}

func GetUInt64(d *interior.Data) (uint64, error) {
	if d == nil {
		return 0, errors.New("invalid parameter")
	}
	if d.GetDataType() != interior.DataType_UInt64 {
		return 0, errors.New("invalid data type %s", d.GetDataType())
	}
	if len(d.DataBuffer) != 8 {
		return 0, errors.New("invalid data %v", d.DataBuffer)
	}
	return binary.BigEndian.Uint64(d.DataBuffer), nil
}

func GetBuffer(d *interior.Data) ([]byte, error) {
	if d == nil {
		return nil, errors.New("invalid parameter")
	}
	if d.GetDataType() != interior.DataType_Buffer {
		return nil, errors.New("invalid data type %s", d.GetDataType())
	}
	return d.DataBuffer, nil
}
