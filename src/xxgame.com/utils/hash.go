/*
常用的简单hash算法
*/

package utils

import (
	"encoding/binary"
	"hash/fnv"
)

//数值hash
type NumHash struct{}

func (h *NumHash) Hash(key []byte) uint64 {
	if len(key) != 8 {
		return 0
	}

	return binary.BigEndian.Uint64(key)
}

//字符串hash
type StrHash struct{}

func (h *StrHash) Hash(key []byte) uint64 {
	f := fnv.New64a()
	f.Write(key)
	return f.Sum64()
}

//总数取模hash算法(key是64位数值)
func TMHashNum(num int, key []byte) int {
	if num <= 0 {
		return 0
	}

	var h NumHash

	k := h.Hash(key)
	return int(k % uint64(num))
}

//总数取模hash算法(key是字符串)
func TMHashStr(num int, key []byte) int {
	if num <= 0 {
		return 0
	}

	var h StrHash

	return int(h.Hash(key) % uint64(num))
}
