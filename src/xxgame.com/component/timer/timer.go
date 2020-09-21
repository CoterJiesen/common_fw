// 定时器管理器实现
package timer

import (
	"sync"
	"time"
)

import (
	"xxgame.com/component/log"
	"xxgame.com/component/uuid"
)

type timer struct {
	id        uint32
	t         *time.Timer
	privData  interface{}
	closeChan chan int
}

type ticker struct {
	id        uint32
	t         *time.Ticker
	privData  interface{}
	closeChan chan int
}

// 定时器超时通知
type TimeoutNotify struct {
	Id       uint32      // 定时器id
	PrivData interface{} // 私有数据
}

// 定时器管理器
type TimerManager struct {
	inited     bool
	timers     map[uint32]*timer
	tickers    map[uint32]*ticker
	tickerMu   sync.Mutex
	timerMu    sync.Mutex
	uuidGen    uuid.UuidGen
	notifyChan chan *TimeoutNotify
	logger     *log.Logger
}

var (
	tm *TimerManager
)

// 返回定时器管理器单件
func Instance() *TimerManager {
	if tm == nil {
		tm = new(TimerManager)
		tm.timers = make(map[uint32]*timer, 10000)
		tm.tickers = make(map[uint32]*ticker, 10000)
	}
	return tm
}

// 初始化定时器管理器
func (tm *TimerManager) Initialize(notifyChan chan *TimeoutNotify, logger *log.Logger) bool {
	if tm.inited {
		panic("timer manager already initialized!")
	}

	if notifyChan != nil && logger != nil {
		tm.notifyChan = notifyChan
		tm.logger = logger
		tm.inited = true
		return true
	}

	return false
}

// 申请新的定时器，返回定时器id
//	milliSeconds: 定时器超时时间
//	isLoop: 标明该定时器是否自动重置
//	privData: 私有数据
func (tm *TimerManager) NewTimer(milliSeconds uint64, isLoop bool, privData interface{}) uint32 {
	if !tm.inited {
		panic("timer manager not initialized yet!")
	}

	id := tm.uuidGen.GenID()
	if isLoop {
		tm.tickerMu.Lock()
		defer tm.tickerMu.Unlock()
		t := ticker{
			id:        id,
			t:         time.NewTicker(time.Millisecond * time.Duration(milliSeconds)),
			privData:  privData,
			closeChan: make(chan int, 1),
		}
		tm.tickers[id] = &t
		go t.job()
		tm.logger.Debugf("tm", "new ticker allocated, timeout in %d(ms), id:%d", milliSeconds, id)
	} else {
		tm.timerMu.Lock()
		defer tm.timerMu.Unlock()
		t := timer{
			id:        id,
			t:         time.NewTimer(time.Millisecond * time.Duration(milliSeconds)),
			privData:  privData,
			closeChan: make(chan int, 1),
		}
		tm.timers[id] = &t
		go t.job()
		tm.logger.Debugf("tm", "new timer allocated, timeout in %d(ms), id:%d", milliSeconds, id)
	}

	return id
}

// 销毁定时器
func (tm *TimerManager) DelTimer(id uint32) {
	if !tm.inited {
		panic("timer manager not initialized yet!")
	}

	tm.timerMu.Lock()
	defer tm.timerMu.Unlock()
	timer, timerExist := tm.timers[id]
	if timerExist {
		timer.t.Stop()
		timer.closeChan <- 0
		delete(tm.timers, id)
		tm.logger.Debugf("tm", "timer %d is removed", id)
		return
	}

	tm.tickerMu.Lock()
	defer tm.tickerMu.Unlock()
	ticker, tickerExist := tm.tickers[id]
	if tickerExist {
		ticker.t.Stop()
		ticker.closeChan <- 0
		delete(tm.tickers, id)
		tm.logger.Debugf("tm", "ticker %d is removed", id)
		return
	}
}

// 等待定时器超时
func (t *timer) job() {
	defer tm.DelTimer(t.id)

	select {
	case <-t.t.C:
		tm.notifyChan <- &TimeoutNotify{
			Id:       t.id,
			PrivData: t.privData,
		}
		tm.logger.Debugf("tm", "timer %d is timeout", t.id)
		return
	case <-t.closeChan:
		return // 定时器被关闭
	}
}

// 等待循环定时器超时
func (t *ticker) job() {
	defer tm.DelTimer(t.id)

	for {
		select {
		case <-t.t.C:
			tm.notifyChan <- &TimeoutNotify{
				Id:       t.id,
				PrivData: t.privData,
			}
			tm.logger.Debugf("tm", "ticker %d is timeout", t.id)
			continue
		case <-t.closeChan:
			return // 定时器被关闭
		}
	}
}
