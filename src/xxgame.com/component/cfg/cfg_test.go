/*
配置管理组件测试
*/

package cfg

import (
	"github.com/golang/protobuf/proto"
	"testing"
	"xxgame.com/component/log"
	"xxgame.com/types/proto/config"
)

var (
	l *log.Logger
)

func init() {
	l = log.NewFileLogger("filelog_test", ".", "cfg.log")
	l.AddTagFilter("CFG")
	l.SetLevel(log.DEBUG)

	c := Instance()
	c.Initialize("testdata", l)

	_, e := c.Register(
		"config0",
		"config0.cfg",
		true,
		func() proto.Message { return new(config.Config0) },
		nil,
		nil)
	if e != nil {
		panic(e.Error())
	}

	_, e = c.Register(
		"config1",
		"config1.cfg",
		false,
		func() proto.Message { return new(config.Config1) },
		nil,
		nil)
	if e != nil {
		panic(e.Error())
	}

	_, e = c.Register(
		"config2",
		"config2.cfg",
		true,
		func() proto.Message { return new(config.Config2) },
		nil,
		nil)
	if e != nil {
		panic(e.Error())
	}
}

func doTest(t testing.TB, dualInitLoad bool) {
	c := Instance()
	if c == nil {
		t.Fatalf("get instance failed")
	}

	//测试初始加载
	_, e := c.InitLoadAll()
	if e != nil && dualInitLoad {
		t.Fatalf("%s", e.Error())
	}

	//测试打印日志
	c.PrintAll()

	//测试重复初始化加载
	_, e = c.InitLoadAll()
	if e == nil && dualInitLoad {
		t.Fatalf("dual InitLoadAll ought to failed")
	}

	//测试全部重载
	_, e = c.ReLoadAll()
	if e != nil {
		t.Fatalf("%s", e.Error())
	}
	c.PrintAll()

	//测试单独加载一个不存在的配置
	_, e = c.ReLoad("Config2")
	if e == nil {
		t.Fatalf("should failed due to unkown cfg")
	}

	//测试重载一个不能重载的配置
	_, e = c.ReLoad("config1")
	if e == nil {
		t.Fatalf("should failed due to not reloadable cfg")
	}

	//测试单独加载一个的配置
	_, e = c.ReLoad("config2")
	if e != nil {
		t.Fatalf("%s", e.Error())
	}

	//测试获取单个配置并
	i := c.Get("config2")
	if i == nil {
		t.Fatalf("can't get config2")
	}
	l.Infof("CFG", "%s\n", i.String())

	log.FlushAll()
}

func TestAll(t *testing.T) {
	doTest(t, true)
}

func BenchmarkAll(t *testing.B) {
	for i := 0; i < t.N; i++ {
		doTest(t, false)
	}
}
