package utils

import (
	"fmt"
	"strconv"
	"strings"
)

import (
	"xxgame.com/utils/errors"
)

// 将会话id由点分十进制格式转换为数字
func ConvertSessionIDString2Number(s string) (uint64, error) {
	var f1, f2 uint32
	var id uint64

	fields := strings.Split(s, ".")
	if len(fields) != 2 {
		return 0, errors.New("invalid format")
	}

	var e error
	var i int
	i, e = strconv.Atoi(fields[0])
	if e != nil {
		return 0, e
	}
	f1 = uint32(i)
	i, e = strconv.Atoi(fields[1])
	if e != nil {
		return 0, e
	}
	f2 = uint32(i)

	id = uint64(f1)<<32 | uint64(f2)
	return id, nil
}

// 将会话id由数字转换为点分十进制
func ConvertSessionIDNumber2String(d uint64) string {
	var f1, f2 uint64

	f1 = (d & 0xffffffff00000000) >> 32
	f2 = d & 0xffffffff

	return fmt.Sprintf("%d.%d", f1, f2)
}

// 将会话id由点分十进制格式转换为数组
func ConvertSessionIDString2Fields(s string) ([]uint32, error) {
	var f1, f2 uint32

	fields := strings.Split(s, ".")
	if len(fields) != 2 {
		return nil, errors.New("invalid format")
	}

	var e error
	var i int
	i, e = strconv.Atoi(fields[0])
	if e != nil {
		return nil, e
	}
	f1 = uint32(i)
	i, e = strconv.Atoi(fields[1])
	if e != nil {
		return nil, e
	}
	f2 = uint32(i)

	return []uint32{f1, f2}, nil
}

// 将会话id由数字转换为数组
func ConvertSessionIDNumber2Fields(d uint64) []uint32 {
	var f1, f2 uint64

	f1 = (d & 0xffffffff00000000) >> 32
	f2 = d & 0xffffffff

	return []uint32{uint32(f1), uint32(f2)}
}

//将数组转换为会话id
func ConvertFields2SessionIDNum(d []uint32) uint64 {
	if d == nil || len(d) != 2 {
		return 0
	}

	var result uint64
	result = ((uint64(d[0]) << 32) & 0xffffffff00000000) | (uint64(d[1]) & 0xffffffff)
	return result
}
