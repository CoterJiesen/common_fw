/*
自定义的errors类型，支持格式化输出错误字符串
*/

package errors

import (
	"fmt"
)

func Code(errorCode int32) error {
	return &Error{fmt.Sprintf("%d", errorCode), errorCode}
}

func NewEx(errorCode int32, format string, arg ...interface{}) error {
	s := fmt.Sprintf(format, arg...)
	return &Error{s, errorCode}
}

func New(format string, arg ...interface{}) error {
	s := fmt.Sprintf(format, arg...)
	return &Error{s, -1}
}

type Error struct {
	s string
	c int32
}

func (e *Error) ErrorCode() int32 {
	return e.c
}

func (e *Error) Error() string {
	return e.s
}
