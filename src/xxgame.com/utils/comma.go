package utils

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

func GetFirstUint64(content string) (uint64, error) {
	index := strings.Index(content, ",")
	if index < 0 {
		return 0, errors.New(fmt.Sprintf("index = %d", index))
	}
	s := content[:index]
	ret, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, err
	}
	return ret, nil
}

func GetFirstUint32(content string) (uint32, error) {
	index := strings.Index(content, ",")
	if index < 0 {
		return 0, errors.New(fmt.Sprintf("index = %d", index))
	}
	s := content[:index]
	ret, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint32(ret), nil
}

func GetLastUint64(content string) (uint64, error) {
	index := strings.LastIndex(content, ",")
	if index < 0 {
		return 0, errors.New(fmt.Sprintf("index = %d", index))
	}
	s := content[index+1:]
	ret, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, err
	}
	return ret, nil
}

func GetLastUint32(content string) (uint32, error) {
	index := strings.LastIndex(content, ",")
	if index < 0 {
		return 0, errors.New(fmt.Sprintf("index = %d", index))
	}
	s := content[index+1:]
	ret, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint32(ret), nil
}

func GetUint32(content string, index int) (uint32, error) {
	s := strings.Split(content, ",")
	if index >= 0 && index < len(s) {
		ret, err := strconv.ParseUint(s[index], 10, 64)
		if err != nil {
			return 0, err
		}
		return uint32(ret), nil
	}
	return 0, errors.New(fmt.Sprintf("index = %d,len(s) = %d", index, len(s)))
}
