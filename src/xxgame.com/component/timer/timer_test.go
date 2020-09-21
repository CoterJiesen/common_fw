package timer

import (
	"fmt"
	"testing"
	"time"
)

import (
	"xxgame.com/component/log"
)

/*
func TestTimer(t *testing.T) {
	l := log.NewFileLogger("timer_test", ".", "testlog.log")
	l.SetLevel(log.DEBUG)
	nch := make(chan *TimeoutNotify, 100)
	Instance().Initialize(nch, l)

	id := Instance().NewTimer(1000, false, "1st timeout!!!")
	fmt.Printf("[%s] allocated new timer %d\n", time.Now(), id)
	select {
	case notify := <-nch:
		fmt.Printf("[%s] got time notification: %#v\n", time.Now(), notify)
	case <-time.After(time.Second * 2):
		t.Fatalf("no timeout happend!")
	}

	id = Instance().NewTimer(1000, true, "2nd timeout!!!")
	fmt.Printf("[%s] allocated new timer %d\n", time.Now(), id)
	stopTimer := time.After(time.Second * 5)
	quitTimer := time.After(time.Second * 11)
LOOP1:
	for {
		select {
		case notify := <-nch:
			fmt.Printf("[%s] got time notification: %#v\n", time.Now(), notify)
		case <-stopTimer:
			Instance().DelTimer(id)
		case <-quitTimer:
			break LOOP1
		}
	}
}
*/

func TestManyTimers(t *testing.T) {
	l := log.NewFileLogger("timer_benchmark", ".", "benchmark.log")
	l.SetLevel(log.DEBUG)
	nch := make(chan *TimeoutNotify, 10000)
	Instance().Initialize(nch, l)

	N := 1000
	startTimes := make(map[uint32]time.Time, N)
	for i := 0; i < N; i++ {
		id := Instance().NewTimer(5000, false, "benchmark timeout!!!")
		startTimes[id] = time.Now()
	}

	count := 0
LOOP:
	for {
		select {
		case notify := <-nch:
			fmt.Printf("[%s] got time notification: %#v\n", time.Now().Sub(startTimes[notify.Id]), notify)
			count++
			if count >= N {
				break LOOP
			}
		}
	}
}
