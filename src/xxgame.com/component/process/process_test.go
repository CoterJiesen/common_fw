/*
测试进程管理组件
*/

package process

import (
	"testing"
	"xxgame.com/component/log"
)

var (
	l *log.Logger
)

type TestCmd struct {
	httpChan chan *HttpContext
}

func (c *TestCmd) Init() {
	c.httpChan = make(chan *HttpContext)
}

func (c *TestCmd) SignalReload() {
	l.Infof("PROCESS", "reload\n")
}

func (c *TestCmd) SignalQuit() {
	l.Infof("PROCESS", "quit\n")
	p := Instance()
	p.ExitDone()
}

func (c *TestCmd) GetHttpChan() chan *HttpContext {
	return c.httpChan
}

func TestAll(t *testing.T) {
	p := Instance()
	c := new(TestCmd)

	l = log.NewFileLogger("filelog_test", ".", "process.log")
	l.AddTagFilter("PROCESS")
	l.SetLevel(log.DEBUG)

	c.Init()
	p.Initialize("", "", "", "", c)

	_, e := p.Daemonize()
	if e != nil {
		t.Fatalf(e.Error())
	}

	for {
		h := <-c.GetHttpChan()
		_ = h
		l.Infof("PROCESS", "get http cmd\n")
		c.GetHttpChan() <- nil
	}
}
