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
// 日志同时输出到终端和 ./quinfi.log 文件
func Init(env string, level string) error {
	// 解析日志级别
	atomicLevel := zap.NewAtomicLevel()
	if level != "" {
		var lvl zapcore.Level
		if err := lvl.UnmarshalText([]byte(level)); err == nil {
			atomicLevel.SetLevel(lvl)
		}
	} else if env == "development" {
		atomicLevel.SetLevel(zapcore.DebugLevel)
	}

	// 终端 encoder
	var consoleEncoder zapcore.Encoder
	if env == "development" {
		encCfg := zap.NewDevelopmentEncoderConfig()
		encCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
		consoleEncoder = zapcore.NewConsoleEncoder(encCfg)
	} else {
		consoleEncoder = zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
	}

	// 文件 encoder（始终 JSON，方便分析）
	fileEncCfg := zap.NewProductionEncoderConfig()
	fileEncCfg.TimeKey = "ts"
	fileEncCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	fileEncoder := zapcore.NewJSONEncoder(fileEncCfg)

	// 打开日志文件（追加模式）
	logFile, err := os.OpenFile("quinfi.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		// 文件打不开则退化为纯终端
		Log, err = zap.Config{
			Level:            atomicLevel,
			OutputPaths:      []string{"stderr"},
			ErrorOutputPaths: []string{"stderr"},
		}.Build()
		return err
	}

	// 双写 core：终端 + 文件
	core := zapcore.NewTee(
		zapcore.NewCore(consoleEncoder, zapcore.Lock(os.Stderr), atomicLevel),
		zapcore.NewCore(fileEncoder, zapcore.AddSync(logFile), atomicLevel),
	)

	Log = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
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
