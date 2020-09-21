package utils

import (
	"fmt"
	"strconv"
	"strings"
)

import (
	"xxgame.com/utils/errors"
)

// 将服务id由点分十进制格式转换为数字
func ConvertServerIDString2Number(s string) (uint32, error) {
	var f1, f2, id uint32

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

	id = f1<<16 | f2
	return id, nil
}

// 将服务id由数字转换为点分十进制
func ConvertServerIDNumber2String(d uint32) string {
	var f1, f2 uint32

	f1 = (d & 0xffff0000) >> 16
	f2 = d & 0xffff

	return fmt.Sprintf("%d.%d", f1, f2)
}

// 将服务id由点分十进制格式转换为数组
func ConvertServerIDString2Fields(s string) ([]uint32, error) {
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

// 将服务id由数字转换为数组
func ConvertServerIDNumber2Fields(d uint32) []uint32 {
	var f1, f2 uint32

	f1 = (d & 0xffff0000) >> 16
	f2 = d & 0xffff

	return []uint32{f1, f2}
}

//将数组转换为服务id
func ConvertFields2ServerIDNum(d []uint32) uint32 {
	if d == nil || len(d) != 2 {
		return 0
	}

	var result uint32
	result = ((d[0] << 16) & 0xffff0000) | (d[1] & 0xffff)
	return result
}
