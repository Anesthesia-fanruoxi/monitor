package common

import (
	"log"
	"time"
)

// Worker Pool
//
// 关键设计：当 taskQueue 满时，不 fallback 到 inline 执行，
// 而是丢弃 + 回调计数（由 store 包注入的自监控指标），让背压可见可观测。
//
// 这样做的理由：inline 执行会让 HTTP goroutine 占着不释放，高并发时
// 反而把内存打爆；丢弃+计数让运维能在 Grafana 上看到丢任务事件，
// 进而据此扩容或限流。
const (
	workerPoolSize = 500   // 并发 worker 数量
	taskQueueSize  = 10000 // 任务队列缓冲大小
)

var taskQueue chan func()

// 队列观测回调：由使用方（store 包的自监控指标）通过 SetQueueHooks 注入，
// 避免 common 反向依赖业务存储层
var (
	dropHook  func()      // 队列满丢弃任务时回调
	levelHook func(n int) // 每秒队列长度采样回调
)

// SetQueueHooks 注入队列观测回调（drop: 丢弃计数；level: 队列长度采样）
func SetQueueHooks(drop func(), level func(int)) {
	dropHook = drop
	levelHook = level
}

func init() {
	taskQueue = make(chan func(), taskQueueSize)
	// 启动 worker pool
	for i := 0; i < workerPoolSize; i++ {
		go func() {
			for task := range taskQueue {
				safeExecute(task)
			}
		}()
	}
	// 每秒采样队列长度，写入自监控指标
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if levelHook != nil {
				levelHook(len(taskQueue))
			}
		}
	}()
}

// Submit 提交异步任务到 worker pool。
// 队列满时触发丢弃回调并返回 false（不阻塞调用方 goroutine）。
func Submit(task func()) bool {
	select {
	case taskQueue <- task:
		return true
	default:
		if dropHook != nil {
			dropHook()
		}
		return false
	}
}

// safeExecute 安全执行任务，捕获 panic 防止 worker 退出
func safeExecute(task func()) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Worker panic 恢复: %v", r)
		}
	}()
	task()
}
