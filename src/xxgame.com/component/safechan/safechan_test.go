package safechan

import (
	"fmt"
	"testing"
	"time"
)

import (
	"github.com/stretchr/testify/assert"
	"xxgame.com/component/log"
)

func TestChannel(t *testing.T) {
	defer log.FlushAll()
	l := log.NewFileLogger("utils", ".", "test.log")
	bych := make(AnyChan, 1)

	e := bych.Write([]byte{1, 2}, l, time.Second)
	if !assert.Nil(t, e, "write failed!") {
		return
	}
	e = bych.Write([]byte{1, 2}, l, time.Second)
	if !assert.Equal(t, e, CHERR_TIMEOUT, "write should timeout!") {
		return
	}
	fmt.Println(e)
	close(bych)
	e = bych.Write([]byte{1, 2}, l, time.Second)
	if !assert.Equal(t, e, CHERR_CLOSED, "channel should be closed!") {
		return
	}
	fmt.Println(e)

	var val interface{}
	bych = make(AnyChan, 10)
	bych.Write([]byte{1, 2}, l, 0)
	val, e = bych.Read(l, time.Second)
	if !assert.Nil(t, e, "read failed!") {
		return
	}
	fmt.Println(val)
	val, e = bych.Read(l, 0)
	if !assert.Nil(t, val, "read should return nothing!") {
		return
	}
	if !assert.Equal(t, e, CHERR_EMPTY, "pipe is empy!") {
		return
	}
	bych.Write("hello", l, 0)
	bych.Write(33, l, 0)
	close(bych)
	val, e = bych.Read(l, time.Second)
	if !assert.Nil(t, e, "read failed!") {
		return
	}
	fmt.Println(val)
	val, e = bych.Read(l, time.Second)
	if !assert.Nil(t, e, "read failed!") {
		return
	}
	fmt.Println(val)
	val, e = bych.Read(l, time.Second)
	if !assert.Equal(t, e, CHERR_CLOSED, "channel should be closed!") {
		return
	}
	fmt.Println(e)
}
