// Package scheduler 提供定时任务调度功能，
// 管理游戏服务器中需要周期性执行的后台任务，
// 如怪物刷新、宠物探险结算等。
package scheduler

import (
	"sync"
	"time"

	"hero-quest/pkg/logger"
)

// Scheduler 定时任务调度器，管理所有定时任务。
// 支持动态添加任务，以及优雅启动和停止。
type Scheduler struct {
	tasks map[string]*Task // 任务名称 → 任务定义的映射表
	quit  chan struct{}     // 停止信号通道，关闭时通知所有任务退出
	wg    sync.WaitGroup   // 等待组，用于优雅停止时等待所有任务协程退出
}

// Task 定时任务定义，描述一个需要周期性执行的任务。
type Task struct {
	Name     string        // 任务名称，作为唯一标识
	Interval time.Duration // 执行间隔
	Fn       func()        // 任务函数，每次触发时执行
}

// New 创建调度器实例。
// 初始化内部的任务映射表和停止信号通道。
func New() *Scheduler {
	return &Scheduler{
		tasks: make(map[string]*Task),
		quit:  make(chan struct{}),
	}
}

// Add 添加定时任务。
// 如果同名任务已存在则会覆盖。任务在 Start 调用后才会开始执行。
func (s *Scheduler) Add(name string, interval time.Duration, fn func()) {
	s.tasks[name] = &Task{
		Name:     name,
		Interval: interval,
		Fn:       fn,
	}
	logger.Info("scheduler task added", "name", name, "interval", interval)
}

// Start 启动所有定时任务。
// 每个任务在独立的协程中运行，按照各自的间隔周期性执行。
// 首次执行会在启动后等待一个完整的间隔周期。
func (s *Scheduler) Start() {
	for _, task := range s.tasks {
		s.wg.Add(1)
		go s.runTask(task)
	}
	logger.Info("scheduler started", "task_count", len(s.tasks))
}

// runTask 运行单个定时任务。
// 使用 time.Ticker 按间隔触发，监听 quit 通道实现优雅退出。
func (s *Scheduler) runTask(task *Task) {
	defer s.wg.Done()

	ticker := time.NewTicker(task.Interval)
	defer ticker.Stop()

	logger.Info("scheduler task started", "name", task.Name, "interval", task.Interval)

	for {
		select {
		case <-ticker.C:
			// 到达执行时间，调用任务函数
			// 使用 recover 防止单个任务的 panic 影响调度器
			func() {
				defer func() {
					if r := recover(); r != nil {
						logger.Error("scheduler task panic",
							"name", task.Name,
							"error", r,
						)
					}
				}()
				task.Fn()
			}()
		case <-s.quit:
			// 收到停止信号，退出任务协程
			logger.Info("scheduler task stopped", "name", task.Name)
			return
		}
	}
}

// Stop 优雅停止所有定时任务。
// 关闭 quit 通道通知所有任务协程退出，并等待它们全部完成。
func (s *Scheduler) Stop() {
	logger.Info("scheduler stopping...")
	close(s.quit)
	s.wg.Wait()
	logger.Info("scheduler stopped")
}
