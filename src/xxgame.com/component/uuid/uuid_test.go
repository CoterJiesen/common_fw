/*
测试uuid组件
*/
package uuid

import (
	"sync"
	"testing"
)

func TestAll(t *testing.T) {
	var g UuidGen
	var wg sync.WaitGroup
	m := make(map[uint64]uint64)
	g.id = 0xffffffffffffffff

	for i := 0; i < 10000000; i++ {
		go func() {
			wg.Add(1)
			v := g.GenID()
			if v == 0 {
				t.Fatalf("uuid == 0")
			}

			i, e := m[v]
			if e {
				t.Fatalf("uuid duplicated %d", i)
			}
			wg.Done()
		}()
	}
	wg.Wait()
}
