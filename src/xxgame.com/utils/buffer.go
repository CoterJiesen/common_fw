/*
byte buffer的辅助函数
*/

package utils

import (
	"encoding/binary"
	"xxgame.com/types/proto/interior"
	"xxgame.com/utils/errors"
)

const (
	int32Len  = 4
	int64Len  = 8
	int32Max  = int32(0x7fffffff)
	int32Min  = int32(-0x80000000)
	int64Max  = int64(0x7fffffffffffffff)
	int64Min  = int64(-0x8000000000000000)
	uint32Max = uint32(0xffffffff)
	uint32Min = uint32(0)
	uint64Max = uint64(0xffffffffffffffff)
	uint64Min = uint64(0)
)

var (
	addMap = map[interior.DataType]func([]byte, []byte) error{
		interior.DataType_Int32:  Int32Add,
		interior.DataType_UInt32: Uint32Add,
		interior.DataType_Int64:  Int64Add,
		interior.DataType_UInt64: Uint64Add,
	}
	subMap = map[interior.DataType]func([]byte, []byte) error{
		interior.DataType_Int32:  Int32Sub,
		interior.DataType_UInt32: Uint32Sub,
		interior.DataType_Int64:  Int64Sub,
		interior.DataType_UInt64: Uint64Sub,
	}
	compareMap = map[interior.DataType]func([]byte, []byte) (bool, error){
		interior.DataType_Int32:  Int32Compare,
		interior.DataType_UInt32: Uint32Compare,
		interior.DataType_Int64:  Int64Compare,
		interior.DataType_UInt64: Uint64Compare,
	}
)

func buffOp(d []byte, s []byte, t interior.DataType, op map[interior.DataType]func([]byte, []byte) error) error {
	if d == nil || s == nil {
		return errors.New("d %v or s %v is nil", d, s)
	}

	f, exist := op[t]
	if f == nil || !exist {
		return errors.New("data type %d not support add", t)
	}

	return f(d, s)
}

//[]byte加法
func BufferAdd(d []byte, s []byte, t interior.DataType) error {
	return buffOp(d, s, t, addMap)
}

//[]byte减法
func BufferSub(d []byte, s []byte, t interior.DataType) error {
	return buffOp(d, s, t, subMap)
}

//[]byte减法
func BufferCompare(d []byte, s []byte, t interior.DataType) (bool, error) {
	if d == nil || s == nil {
		return false, errors.New("d %v or s %v is nil", d, s)
	}

	f, exist := compareMap[t]
	if f == nil || !exist {
		return false, errors.New("data type %d not support add", t)
	}

	return f(d, s)
}

//int32加法
func Int32Add(d []byte, s []byte) error {
	if len(d) != int32Len || len(s) != int32Len {
		return errors.New("invalid data length")
	}

	dv := int32(binary.BigEndian.Uint32(d))
	ds := int32(binary.BigEndian.Uint32(s))

	//溢出处理
	if dv+ds < dv {
		dv = int32Max //强制设置为最大值
	} else {
		dv += ds
	}

	binary.BigEndian.PutUint32(d, uint32(dv))

	return nil
}

//uint32加法
func Uint32Add(d []byte, s []byte) error {
	if len(d) != int32Len || len(s) != int32Len {
		return errors.New("invalid data length")
	}

	dv := binary.BigEndian.Uint32(d)
	ds := binary.BigEndian.Uint32(s)

	//溢出处理
	if dv+ds < dv {
		dv = uint32Max //强制设置为最大值
	} else {
		dv += ds
	}

	binary.BigEndian.PutUint32(d, dv)

	return nil
}

//int64加法
func Int64Add(d []byte, s []byte) error {
	if len(d) != int64Len || len(s) != int64Len {
		return errors.New("invalid data length")
	}

	dv := int64(binary.BigEndian.Uint64(d))
	ds := int64(binary.BigEndian.Uint64(s))

	//溢出处理
	if dv+ds < dv {
		dv = int64Max //强制设置为最大值
	} else {
		dv += ds
	}

	binary.BigEndian.PutUint64(d, uint64(dv))

	return nil
}

//uint64加法
func Uint64Add(d []byte, s []byte) error {
	if len(d) != int64Len || len(s) != int64Len {
		return errors.New("invalid data length")
	}

	dv := binary.BigEndian.Uint64(d)
	ds := binary.BigEndian.Uint64(s)

	//溢出处理
	if dv+ds < dv {
		dv = uint64Max //强制设置为最大值
	} else {
		dv += ds
	}

	binary.BigEndian.PutUint64(d, dv)

	return nil
}

//int32减法
func Int32Sub(d []byte, s []byte) error {
	if len(d) != int32Len || len(s) != int32Len {
		return errors.New("invalid data length")
	}

	dv := int32(binary.BigEndian.Uint32(d))
	ds := int32(binary.BigEndian.Uint32(s))

	//溢出处理
	if dv-ds > dv {
		dv = int32Min //强制设置为最大值
	} else {
		dv -= ds
	}

	binary.BigEndian.PutUint32(d, uint32(dv))

	return nil
}

//uint32减法
func Uint32Sub(d []byte, s []byte) error {
	if len(d) != int32Len || len(s) != int32Len {
		return errors.New("invalid data length")
	}

	dv := binary.BigEndian.Uint32(d)
	ds := binary.BigEndian.Uint32(s)

	//溢出处理
	if dv-ds > dv {
		dv = uint32Min //强制设置为最大值
	} else {
		dv -= ds
	}

	binary.BigEndian.PutUint32(d, dv)

	return nil
}

//int64减法
func Int64Sub(d []byte, s []byte) error {
	if len(d) != int64Len || len(s) != int64Len {
		return errors.New("invalid data length")
	}

	dv := int64(binary.BigEndian.Uint64(d))
	ds := int64(binary.BigEndian.Uint64(s))

	//溢出处理
	if dv-ds > dv {
		dv = int64Min //强制设置为最大值
	} else {
		dv -= ds
	}

	binary.BigEndian.PutUint64(d, uint64(dv))

	return nil
}

//uint64减法
func Uint64Sub(d []byte, s []byte) error {
	if len(d) != int64Len || len(s) != int64Len {
		return errors.New("invalid data length")
	}

	dv := binary.BigEndian.Uint64(d)
	ds := binary.BigEndian.Uint64(s)

	//溢出处理
	if dv-ds > dv {
		dv = uint64Min //强制设置为最大值
	} else {
		dv -= ds
	}

	binary.BigEndian.PutUint64(d, dv)

	return nil
}

//int32比较
func Int32Compare(d []byte, s []byte) (bool, error) {
	if len(d) != int32Len || len(s) != int32Len {
		return false, errors.New("invalid data length")
	}

	dv := int32(binary.BigEndian.Uint32(d))
	ds := int32(binary.BigEndian.Uint32(s))

	//溢出处理
	if dv-ds > dv {
		return false, nil
	} else {
		return true, nil
	}
}

//uint32比较
func Uint32Compare(d []byte, s []byte) (bool, error) {
	if len(d) != int32Len || len(s) != int32Len {
		return false, errors.New("invalid data length")
	}

	dv := binary.BigEndian.Uint32(d)
	ds := binary.BigEndian.Uint32(s)

	//溢出处理
	if dv-ds > dv {
		return false, nil
	} else {
		return true, nil
	}
}

//int64比较
func Int64Compare(d []byte, s []byte) (bool, error) {
	if len(d) != int64Len || len(s) != int64Len {
		return false, errors.New("invalid data length")
	}

	dv := int64(binary.BigEndian.Uint64(d))
	ds := int64(binary.BigEndian.Uint64(s))

	//溢出处理
	if dv-ds > dv {
		return false, nil
	} else {
		return true, nil
	}
}

//uint64比较
func Uint64Compare(d []byte, s []byte) (bool, error) {
	if len(d) != int64Len || len(s) != int64Len {
		return false, errors.New("invalid data length")
	}

	dv := binary.BigEndian.Uint64(d)
	ds := binary.BigEndian.Uint64(s)

	//溢出处理
	if dv-ds > dv {
		return false, nil
	} else {
		return true, nil
	}
}
