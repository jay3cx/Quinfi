// Package logger 提供基于 Zap 的结构化日志功能
package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Log 全局日志实例（默认 nop，防止未初始化时 nil panic）
var Log *zap.Logger = zap.NewNop()

// Init 初始化日志系统
// env: "development" 使用人类可读格式，其他使用 JSON 格式
// level: 日志级别（debug/info/warn/error），为空则使用环境默认级别
func Init(env string, level string) error {
	var config zap.Config

	if env == "development" {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		config = zap.NewProductionConfig()
	}

	// 应用指定的日志级别
	if level != "" {
		var lvl zapcore.Level
		if err := lvl.UnmarshalText([]byte(level)); err == nil {
			config.Level = zap.NewAtomicLevelAt(lvl)
		}
	}

	var err error
	Log, err = config.Build()
	if err != nil {
		return err
	}

	return nil
}

// Sync 刷新日志缓冲区
func Sync() {
	if Log != nil {
		_ = Log.Sync()
	}
}

// Info 记录信息级别日志
func Info(msg string, fields ...zap.Field) {
	Log.Info(msg, fields...)
}

// Error 记录错误级别日志
func Error(msg string, fields ...zap.Field) {
	Log.Error(msg, fields...)
}

// Debug 记录调试级别日志
func Debug(msg string, fields ...zap.Field) {
	Log.Debug(msg, fields...)
}

// Warn 记录警告级别日志
func Warn(msg string, fields ...zap.Field) {
	Log.Warn(msg, fields...)
}

// GetEnv 获取环境变量，如果不存在则返回默认值
func GetEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
