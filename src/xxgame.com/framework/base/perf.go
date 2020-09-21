package base

import (
	"fmt"
	"os"
	"runtime"
	"runtime/trace"
	"time"
)

type PerfData struct {
	MsgProcTime  time.Duration //消息处理时间
	TaskWaitTime time.Duration //协程池等待时间
}

//性能监控
type PerfMon struct {
	statsPath, tracePath string         //日志路径
	statsOut             *os.File       //统计输出文件
	traceOut             *os.File       //trace输出
	PerfChan             chan *PerfData //性能数据管道
	inmsg                int            //接收到的消息数目
	avgMsgProcTime       time.Duration  //平均消息处理时间
	maxMsgProcTime       time.Duration  //最大消息处理时间
	minMsgProcTime       time.Duration  //最小消息处理时间
	avgTaskWaitTime      time.Duration  //平均协程池等待时间
	maxTaskWaitTime      time.Duration  //最大协程池等待时间
	minTaskWaitTime      time.Duration  //最小协程池等待时间
}

const (
	MaxPerfLogSize = 64 * 1024 * 1024 //64MB
)

func NewPerfMon(s *BaseService) *PerfMon {
	p := new(PerfMon)
	p.PerfChan = make(chan *PerfData, 1000)
	p.statsPath = s.Proc.DefaultLogDir + "/perf_stats.log"
	p.tracePath = s.Proc.DefaultLogDir + "/trace.out"
	p.statsOut, _ = os.OpenFile(p.statsPath, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	tempData := make([]byte, MaxPerfLogSize/2)

	go func() {
		for {
			stat, err := os.Lstat(p.statsPath)
			if err != nil {
				// 日志文件不存在则创建
				p.statsOut.Close()
				p.statsOut, _ = os.OpenFile(p.statsPath,
					os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
			} else if stat.Size() >= MaxPerfLogSize {
				// 日志文件过大则保留最新的统计数据并截断
				p.statsOut.Seek(MaxPerfLogSize/2, 2)
				p.statsOut.Read(tempData)
				p.statsOut.Seek(0, 0)
				p.statsOut.Write(tempData)
				p.statsOut.Truncate(MaxPerfLogSize / 2)
				p.statsOut.Seek(0, 2)
			}

			//输出统计日志
			p.Output()
			p.Clear()

			time.Sleep(time.Second * 15)
		}
	}()

	go func() {
		for {
			data := <-p.PerfChan
			p.inmsg++
			if p.avgMsgProcTime == 0 {
				p.avgMsgProcTime = data.MsgProcTime
				p.maxMsgProcTime = data.MsgProcTime
				p.minMsgProcTime = data.MsgProcTime
				p.avgTaskWaitTime = data.TaskWaitTime
				p.maxTaskWaitTime = data.TaskWaitTime
				p.minTaskWaitTime = data.TaskWaitTime
			} else {
				if data.MsgProcTime > p.maxMsgProcTime {
					p.maxMsgProcTime = data.MsgProcTime
				} else if data.MsgProcTime < p.minMsgProcTime {
					p.minMsgProcTime = data.MsgProcTime
				}
				p.avgMsgProcTime += data.MsgProcTime
				p.avgMsgProcTime /= 2

				if data.TaskWaitTime > p.maxTaskWaitTime {
					p.maxTaskWaitTime = data.TaskWaitTime
				} else if data.TaskWaitTime < p.minTaskWaitTime {
					p.minTaskWaitTime = data.TaskWaitTime
				}
				p.avgTaskWaitTime += data.TaskWaitTime
				p.avgTaskWaitTime /= 2
			}
		}
	}()

	return p
}

func (p *PerfMon) Clear() {
	p.inmsg = 0
	p.avgMsgProcTime = 0
	p.maxMsgProcTime = 0
	p.minMsgProcTime = 0
	p.avgTaskWaitTime = 0
	p.maxTaskWaitTime = 0
	p.minTaskWaitTime = 0
}

func (p *PerfMon) Output() {
	fmt.Fprintf(p.statsOut, ">>>%s========================================\n", time.Now())
	fmt.Fprintf(p.statsOut, ">Msg Processing statistics\n")
	fmt.Fprintf(p.statsOut, "RecvMsg:%d", p.inmsg)
	fmt.Fprintf(p.statsOut, "\tAvgMsgProcTime:%s", p.avgMsgProcTime)
	fmt.Fprintf(p.statsOut, "\tMaxMsgProcTime:%s", p.maxMsgProcTime)
	fmt.Fprintf(p.statsOut, "\tMinMsgProcTime:%s\n", p.minMsgProcTime)
	fmt.Fprintf(p.statsOut, "AvgTaskWaitTime:%s", p.avgTaskWaitTime)
	fmt.Fprintf(p.statsOut, "\tMaxTaskWaitTime:%s", p.maxTaskWaitTime)
	fmt.Fprintf(p.statsOut, "\tMinTaskWaitTime:%s\n", p.minTaskWaitTime)

	memstats := new(runtime.MemStats)
	runtime.ReadMemStats(memstats)
	fmt.Fprintf(p.statsOut, "\n>Memory: General statistics\n")
	fmt.Fprintf(p.statsOut, "Alloc:%d - bytes allocated and not yet freed\n", memstats.Alloc)
	fmt.Fprintf(p.statsOut, "TotalAlloc:%d - bytes allocated (even if freed)\n", memstats.TotalAlloc)
	fmt.Fprintf(p.statsOut, "Sys:%d - bytes obtained from system (sum of XxxSys below)\n", memstats.Sys)
	fmt.Fprintf(p.statsOut, "Lookups:%d - number of pointer lookups\n", memstats.Lookups)
	fmt.Fprintf(p.statsOut, "Mallocs:%d - number of mallocs\n", memstats.Mallocs)
	fmt.Fprintf(p.statsOut, "Frees:%d - number of frees\n", memstats.Frees)

	fmt.Fprintf(p.statsOut, "\n>Memory: Main allocation heap statistics\n")
	fmt.Fprintf(p.statsOut, "HeapAlloc:%d - bytes allocated and not yet freed (same as Alloc above)\n", memstats.HeapAlloc)
	fmt.Fprintf(p.statsOut, "HeapSys:%d - bytes obtained from system\n", memstats.HeapSys)
	fmt.Fprintf(p.statsOut, "HeapIdle:%d - bytes in idle spans\n", memstats.HeapIdle)
	fmt.Fprintf(p.statsOut, "HeapInuse:%d - bytes in non-idle span\n", memstats.HeapInuse)
	fmt.Fprintf(p.statsOut, "HeapReleased:%d - bytes released to the OS\n", memstats.HeapReleased)
	fmt.Fprintf(p.statsOut, "HeapObjects:%d - total number of allocated objects\n", memstats.HeapObjects)

	fmt.Fprintf(p.statsOut, "\n>Memory: Garbage collector statistics\n")
	fmt.Fprintf(p.statsOut, "NextGC:%d - next collection will happen when HeapAlloc ≥ this amount\n", memstats.NextGC)
	fmt.Fprintf(p.statsOut, "LastGC:%s - end time of last collection\n", time.Unix(0, int64(memstats.LastGC)))
	fmt.Fprintf(p.statsOut, "PauseTotalNs:%d\n", memstats.PauseTotalNs)
	fmt.Fprintf(p.statsOut, "NumGC:%d\n", memstats.NumGC)
	fmt.Fprintf(p.statsOut, "GCCPUFraction:%f - fraction of CPU time used by GC\n", memstats.GCCPUFraction)

	fmt.Fprintf(p.statsOut, "===============================================================================\n\n")
}

func (p *PerfMon) StartTrace() error {
	p.traceOut, _ = os.OpenFile(p.tracePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0444)
	return trace.Start(p.traceOut)
}

func (p *PerfMon) StopTrace() {
	p.traceOut.Close()
	trace.Stop()
}
