package routinepool

import (
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var (
	checksumData []byte
)

func Task() {
	time.Sleep(time.Millisecond)
	md5.Sum(checksumData)
}

const (
	TaskNum = 10000
)

func TestRoutinePool(t *testing.T) {
	pool := NewPool(100).Start()
	checksumData, _ = ioutil.ReadFile("routinepool.go")

	start := time.Now()
	for i := 0; i < TaskNum; i++ {
		Task()
	}
	fmt.Printf("LOOP consumed: %s\n", time.Now().Sub(start))

	start = time.Now()
	var taskSync sync.WaitGroup
	var idx uint32
	for i := 0; i < 100; i++ {
		go func() {
			// fmt.Printf("POOL test %d",i+1)

			for i := 0; i < TaskNum; i++ {
				taskSync.Add(1)
				pool.DispatchTask(func() {
					Task()
					taskSync.Done()
					atomic.AddUint32(&idx, 1)
				})
			}
			//taskSync.Wait()
		}()
	}
	// fmt.Printf("POOL test 0")
	for i := 0; i < TaskNum; i++ {
		taskSync.Add(1)

		pool.DispatchTask(func() {
			Task()
			taskSync.Done()
			atomic.AddUint32(&idx, 1)
		})
	}
	taskSync.Wait()
	fmt.Printf("POOL consumed: %s idx %d\n", time.Now().Sub(start), idx)
}
