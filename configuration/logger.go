package configuration

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
	"sync"
	"time"
)

var globalLogger *zap.Logger

//func InitLogger(logPath string, logLevel zapcore.Level) {
//	// 1. 配置日志切割器 (Lumberjack)
//	lumberjackLogger := &lumberjack.Logger{
//		Filename:   logPath, // 日志文件路径
//		MaxSize:    1,       // 单文件最大100MB
//		MaxBackups: 3,       // 保留3个备份
//		MaxAge:     30,      // 日志保留30天
//		Compress:   true,    // 压缩旧日志
//	}
//
//	// 2. 创建双输出目标
//	consoleSyncer := zapcore.AddSync(os.Stdout)     // 控制台输出
//	fileSyncer := zapcore.AddSync(lumberjackLogger) // 文件输出
//	zapcore.NewMultiWriteSyncer(consoleSyncer, fileSyncer)
//
//	// 3. 配置编码器（控制台彩色输出 + 文件JSON格式）
//	consoleEncoder := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()) // 控制台友好格式
//	fileEncoder := zapcore.NewJSONEncoder(zapcore.EncoderConfig{                   // 文件结构化JSON
//		TimeKey:     "time",
//		LevelKey:    "level",
//		MessageKey:  "msg",
//		EncodeTime:  zapcore.ISO8601TimeEncoder,  // ISO8601时间格式 [1,5](@ref)
//		EncodeLevel: zapcore.CapitalLevelEncoder, // 级别大写（INFO/ERROR）
//	})
//
//	// 4. 分离核心（Core）
//	consoleCore := zapcore.NewCore(consoleEncoder, consoleSyncer, logLevel)
//	fileCore := zapcore.NewCore(fileEncoder, fileSyncer, logLevel)
//	combinedCore := zapcore.NewTee(consoleCore, fileCore) // 双输出核心组合 [8](@ref)
//
//	// 5. 创建全局 Logger
//	globalLogger = zap.New(combinedCore, zap.AddCaller()) // 添加调用位置信息 [4](@ref)
//	zap.ReplaceGlobals(globalLogger)                      // 设为全局默认 Logger [2](@ref)
//}

var (
	loggers = make(map[string]*zap.Logger) // 按命令名存储Logger
	mu      sync.Mutex
)

const (
	GlobalLogName = "global"
	WebLogName    = "web-monitor"
	MysqlLogName  = "mysql-monitor"
)

// InitLogger 为不同命令创建独立Logger
func InitLogger(cmdName, logPath string) *zap.Logger {
	mu.Lock()
	defer mu.Unlock()

	if logger, exists := loggers[cmdName]; exists {
		return logger // 避免重复创建
	}

	// 1. 配置日志切割器 (Lumberjack)
	lumberjackLogger := &lumberjack.Logger{
		Filename:   logPath, // 如: "./logs/commandA.log"
		MaxSize:    10,      // 单文件最大100MB
		MaxBackups: 3,       // 保留3个备份
		MaxAge:     30,      // 日志保留30天
		Compress:   true,    // 压缩旧日志
	}

	// 2. 双输出目标（控制台+文件）
	consoleSyncer := zapcore.AddSync(os.Stdout)
	fileSyncer := zapcore.AddSync(lumberjackLogger)
	zapcore.NewMultiWriteSyncer(consoleSyncer, fileSyncer)

	// 3. 编码器配置（控制台彩色+文件JSON）
	consoleEncoder := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
	fileEncoder := zapcore.NewJSONEncoder(zapcore.EncoderConfig{
		TimeKey:      "datetime",
		LevelKey:     "level",
		MessageKey:   "msg",
		CallerKey:    "caller",
		EncodeTime:   customTimeEncoder,           // 关键：自定义时间格式
		EncodeLevel:  zapcore.CapitalLevelEncoder, // 控制台彩色级别
		EncodeCaller: zapcore.ShortCallerEncoder,
	})

	// 4. 分离核心（Core）
	consoleCore := zapcore.NewCore(consoleEncoder, consoleSyncer, zapcore.DebugLevel)
	fileCore := zapcore.NewCore(fileEncoder, fileSyncer, zapcore.DebugLevel)
	combinedCore := zapcore.NewTee(consoleCore, fileCore)

	// 5. 创建命令专属Logger（添加命令名作为全局字段）
	logger := zap.New(combinedCore, zap.AddCaller(), zap.Fields(
		zap.String("command", cmdName),
	))
	loggers[cmdName] = logger
	return logger
}

func GetLogger(cmdName string) *zap.Logger {
	return loggers[cmdName]
}

// 自定义时间格式 (精确到毫秒+时区)
func customTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	// 格式：2006-01-02 15:04:05.00000 -0700 MST m=+0.000000001
	enc.AppendString(t.Format("2006-01-02 15:04:05.000000 -0700 MST m=+0.000000001"))
}
