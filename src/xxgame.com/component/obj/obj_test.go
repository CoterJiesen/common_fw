/*
测试对象管理组件
*/

package obj

import (
	"testing"
	"xxgame.com/component/uuid"
)

type XObject struct {
	BaseObject
}

type XObMng struct {
	BaseObMng
}

func TestAll(t *testing.T) {
	g := new(uuid.UuidGen)
	m := new(XObMng)

	//测试初始化
	e := m.Init(-1, g, func() Object { return new(XObject) })
	if e != nil {
		t.Fatalf(e.Error())
	}

	//测试反复初始化
	e = m.Init(-1, g, func() Object { return new(XObject) })
	if e == nil {
		t.Fatalf(e.Error())
	}

	var o Object
	//测试创建成功
	o, e = m.Create()
	if e != nil {
		t.Fatalf(e.Error())
	}

	//测试创建失败
	m.cap = 1
	_, e = m.Create()
	if e == nil {
		t.Fatalf(e.Error())
	}

	//测试是否满
	if !m.IsFull() {
		t.Fatalf("IsFull test failed")
	}

	if m.Cap() != 1 {
		t.Fatalf("Cap test failed")
	}

	if m.Len() != 1 {
		t.Fatalf("Len test failed")
	}

	id := o.GetID()
	o, e = m.Get(id)
	if e != nil {
		t.Fatalf(e.Error())
	}

	o, e = m.Get(id + 1)
	if e == nil {
		t.Fatalf(e.Error())
	}

	e = m.Del(id)
	if e != nil {
		t.Fatalf(e.Error())
	}

	e = m.Del(id)
	if e == nil {
		t.Fatalf(e.Error())
	}

	m.Clear()
	if m.Len() != 0 {
		t.Fatalf("Clear test failed")
	}
}
