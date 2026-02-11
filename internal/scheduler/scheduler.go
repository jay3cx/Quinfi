// Package scheduler 提供定时任务调度器
// 让小基能后台自主运行，定时执行研究和监控任务
package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/jay3cx/fundmind/pkg/logger"
	"go.uber.org/zap"
)

// Task 定时任务接口
type Task interface {
	Name() string
	Run(ctx context.Context) error
}

// Schedule 调度配置
type Schedule struct {
	Task     Task
	Interval time.Duration
	RunAt    string // 可选：每日固定时间 "08:00"（暂不实现，用 Interval）
}

// Scheduler 定时任务调度器
type Scheduler struct {
	schedules []Schedule
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// New 创建调度器
func New() *Scheduler {
	return &Scheduler{}
}

// Register 注册定时任务
func (s *Scheduler) Register(task Task, interval time.Duration) {
	s.schedules = append(s.schedules, Schedule{
		Task:     task,
		Interval: interval,
	})
	logger.Info("注册定时任务",
		zap.String("task", task.Name()),
		zap.Duration("interval", interval),
	)
}

// Start 启动所有定时任务
func (s *Scheduler) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	for _, sched := range s.schedules {
		s.wg.Add(1)
		go s.runLoop(ctx, sched)
	}

	logger.Info("调度器启动", zap.Int("tasks", len(s.schedules)))
}

// Stop 停止所有定时任务
func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	logger.Info("调度器已停止")
}

// RunNow 立即执行指定任务（手动触发）
func (s *Scheduler) RunNow(ctx context.Context, taskName string) error {
	for _, sched := range s.schedules {
		if sched.Task.Name() == taskName {
			return s.executeTask(ctx, sched.Task)
		}
	}
	return nil
}

func (s *Scheduler) runLoop(ctx context.Context, sched Schedule) {
	defer s.wg.Done()

	// 启动后等一小段时间再首次执行，避免启动瞬间大量并发
	select {
	case <-time.After(10 * time.Second):
	case <-ctx.Done():
		return
	}

	// 首次执行
	s.executeTask(ctx, sched.Task)

	ticker := time.NewTicker(sched.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.executeTask(ctx, sched.Task)
		case <-ctx.Done():
			return
		}
	}
}

func (s *Scheduler) executeTask(ctx context.Context, task Task) error {
	taskCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	logger.Info("执行定时任务", zap.String("task", task.Name()))
	start := time.Now()

	err := task.Run(taskCtx)
	duration := time.Since(start)

	if err != nil {
		logger.Error("定时任务失败",
			zap.String("task", task.Name()),
			zap.Duration("duration", duration),
			zap.Error(err),
		)
	} else {
		logger.Info("定时任务完成",
			zap.String("task", task.Name()),
			zap.Duration("duration", duration),
		)
	}
	return err
}
