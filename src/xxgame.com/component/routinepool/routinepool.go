// 协程池功能如下：
//	1. 创建固定数量协程;
//	2. 向协程池派发请求;
package routinepool

import "time"
import (
	"fmt"
	"xxgame.com/utils"
)

type RoutinePool struct {
	size         uint32           //协程池大小
	idleTaskChan chan chan func() //空闲协程的任务管道队列
	started      bool             //线程池是否已启动
}

// 创建协程池
func NewPool(size uint32) *RoutinePool {
	p := new(RoutinePool)
	p.size = size
	p.idleTaskChan = make(chan chan func(), size)
	return p
}

// 启动协程池
func (p *RoutinePool) Start() *RoutinePool {
	if p.started {
		return p
	}

	p.started = true

	for i := 0; i < int(p.size); i++ {
		go utils.GoMain(func() {
			taskChan := make(chan func())
			for {
				// 协程处于空闲状态，将任务管道加入空闲队列
				p.idleTaskChan <- taskChan

				// 等待任务
				task := <-taskChan

				// 执行任务函数
				task()
			}
		}, nil)
	}

	return p
}

// 派发任务，入参函数将被传递至空闲的协程执行
func (p *RoutinePool) DispatchTask(task func()) error {
	var taskChan chan func()
	select {
	// 获取空闲协程的任务管道
	case taskChan = <-p.idleTaskChan:
	case <-time.After(time.Second * 5):
		return fmt.Errorf("dispatch task to routine pool failed!")
	}

	// 添加任务
	taskChan <- task
	return nil
}
