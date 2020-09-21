package utils

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"os"
	"runtime/debug"
	"time"
)

const (
	TIME_FORMAT = "2006-01-02 15:04:05"
	DATE_FORMAT = "2006-01-02"
)

//压缩数据
func Compress(data []byte) []byte {
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	w.Write(data)
	w.Close()
	return buf.Bytes()
}

func GetStringFromMap(m map[string]interface{}, key string) (string, error) {
	ret := m[key]
	if ret == nil {
		return "", fmt.Errorf("can't find key %v in map", key)
	}

	s, ok := ret.(string)
	if !ok {
		return "", fmt.Errorf("key %v in map is not string", key)
	}

	return s, nil
}

func GetInt32FromMap(m map[string]interface{}, key string) (int32, error) {
	ret := m[key]
	if ret == nil {
		return int32(0), fmt.Errorf("can't find key %v in map", key)
	}

	n, ok := ret.(float64)
	if !ok {
		return int32(0), fmt.Errorf("key %v in map is not decimal", key)
	}

	return int32(n), nil
}

func Try(fun func(), handler func(interface{})) {
	defer func() {
		var err interface{}
		if err = recover(); err != nil {
			//release模式，恢复
			fmt.Fprintln(os.Stderr, fmt.Sprintf("======catch error begin [%v]======", time.Now().Format("2006-01-02 15:04:05")))
			fmt.Fprintln(os.Stderr, err)
			//打印堆栈
			debug.PrintStack()
			fmt.Fprintln(os.Stderr, fmt.Sprintf("======catch error end   [%v]======", time.Now().Format("2006-01-02 15:04:05")))
			os.Stderr.Sync()
		}
		if handler != nil {
			handler(err)
		}
	}()
	fun()
}
