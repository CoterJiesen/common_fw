// 提供管道读写接口，对异常处理进行了封装
package safechan

import (
	"fmt"
	"strings"
	"time"
)

import (
	"xxgame.com/component/log"
	"xxgame.com/utils/errors"
)

var CHERR_CLOSED = errors.New("channel is closed")
var CHERR_TIMEOUT = errors.New("channel ops timeout")
var CHERR_EMPTY = errors.New("channel is empty")

type AnyChan chan interface{}

// 从管道中读取数据，返回CHERR_CLOSED表示管道已经被关闭，返回CHERR_TIMEOUT表示
// 操作超时，返回CHERR_EMPTY表示管道是空的
//	l: 日志管理器
//	t: 操作超时值，为0表示在没有数据可读时立刻返回错误，否则等待特定时间直至超时
func (ch AnyChan) Read(l *log.Logger, t time.Duration) (re interface{}, err error) {
	defer func() {
		if err := recover(); err != nil {
			l.Errorf("CHANNEL", "read from channel failed: ", err)
			err = errors.New("read channel panic")
		}
	}()

	re = nil
	err = nil

	if t == 0 {
		select {
		case r, ok := <-ch:
			if !ok {
				// 管道已关闭
				err = CHERR_CLOSED
			} else {
				re = r
			}
		default:
			// 没有数据可读
			err = CHERR_EMPTY
		}
	} else {
		select {
		case r, ok := <-ch:
			if !ok {
				// 管道已关闭
				err = CHERR_CLOSED
			} else {
				re = r
			}
		case <-time.After(t):
			// 操作超时
			err = CHERR_TIMEOUT
		}
	}

	return
}

// 向管道写入数据，返回CHERR_CLOSED表示管道已经被关闭，返回CHERR_TIMEOUT表示操作超时
//	v: 向管道写入的变量
//	l: 日志管理器
//	t: 操作超时值，为0表示一直等待直至操作完成，否则等待特定时间直至超时
func (ch AnyChan) Write(v interface{}, l *log.Logger, t time.Duration) (err error) {
	defer func() {
		if e := recover(); e != nil {
			str := fmt.Sprintf("%v", e)
			if strings.Contains(str, "closed channel") {
				err = CHERR_CLOSED
			} else {
				err = errors.New("write channel panic")
			}
			l.Errorf("CHANNEL", "write into channel failed: ", e)
		}
	}()

	err = nil

	if t == 0 {
		ch <- v
		err = nil
	} else {
		select {
		case ch <- v:
			err = nil
		case <-time.After(t):
			l.Warnf("CHANNEL", "write %v to send chan time out, chan len %d", v, len(ch))
			err = CHERR_TIMEOUT
		}
	}

	return
}
