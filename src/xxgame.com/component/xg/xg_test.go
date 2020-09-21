package xg

import (
	_ "fmt"
	"testing"
)

import (
	"github.com/stretchr/testify/assert"
	"xxgame.com/component/log"
)

func TestXG(t *testing.T) {
	logger := log.NewTermLogger("xg_test")
	xg := NewXG(logger)
	e := xg.SendMsg(uint64(1011987), "Test from jack!", 4)
	assert.Nil(t, e, "send msg failed!")
}
