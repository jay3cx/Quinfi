// Quinfi - AI 智能基金投研助手
// 主入口文件
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jay3cx/Quinfi/internal/api"
	"github.com/jay3cx/Quinfi/internal/config"
	"github.com/jay3cx/Quinfi/internal/db"
	"github.com/jay3cx/Quinfi/pkg/logger"
	"go.uber.org/zap"
)

func main() {
	// 命令行参数
	configPath := flag.String("config", "", "配置文件路径")
	flag.Parse()

	// 加载配置（含 Validate）
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志（env + level）
	if err := logger.Init(cfg.Log.Env, cfg.Log.Level); err != nil {
		fmt.Printf("初始化日志失败: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("Quinfi 服务启动中...",
		zap.String("port", cfg.Server.Port),
		zap.String("mode", cfg.Server.Mode),
		zap.String("log_level", cfg.Log.Level),
	)

	// 初始化数据库
	sqlDB := initDB(cfg)
	if sqlDB != nil {
		defer sqlDB.Close()
	}

	// 初始化路由和所有服务
	result := api.SetupRouter(cfg, sqlDB)

	// 创建 HTTP 服务器（支持优雅关闭）
	addr := ":" + cfg.Server.Port
	srv := &http.Server{
		Addr:         addr,
		Handler:      result.Engine,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 10 * time.Minute, // SSE 辩论流程可达 5 分钟，需足够长的写超时
		IdleTimeout:  120 * time.Second,
	}

	// 启动 HTTP 服务（非阻塞）
	go func() {
		logger.Info("HTTP 服务监听", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP 服务异常退出", zap.Error(err))
			os.Exit(1)
		}
	}()

	// 等待退出信号
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	// ====== 优雅关闭 ======
	logger.Info("收到退出信号，开始优雅关闭...")

	// 1. 停止接收新请求，等待已有请求完成（最多 30 秒）
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP 服务关闭异常", zap.Error(err))
	} else {
		logger.Info("HTTP 服务已关闭")
	}

	// 2. 停止定时任务调度器
	result.Scheduler.Stop()

	// 3. 停止 RSS 抓取调度器
	if result.RSSScheduler != nil {
		result.RSSScheduler.Stop()
	}

	logger.Info("Quinfi 服务已完全关闭")
}

// initDB 初始化数据库，返回 nil 表示降级为纯内存模式
func initDB(cfg *config.Config) *sql.DB {
	if cfg.DB.Path == "" {
		return nil
	}

	sqlDB, err := db.Open(cfg.DB.Path)
	if err != nil {
		logger.Error("数据库初始化失败，降级为内存模式", zap.Error(err))
		return nil
	}

	logger.Info("数据库初始化成功", zap.String("path", cfg.DB.Path))
	return sqlDB
}
