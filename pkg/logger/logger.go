package logger

import (
	"os"
	"path/filepath"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	Log  *zap.Logger
	Once sync.Once
)

// InitLogger 初始化全局日志记录器
func InitLogger(logDir string, debug bool) {
	Once.Do(func() {
		if logDir == "" {
			logDir = "logs"
		}
		if err := os.MkdirAll(logDir, 0755); err != nil {
			panic(err)
		}

		// 1. 配置控制台输出 (Human Readable + Color)
		consoleEncoderConfig := zap.NewDevelopmentEncoderConfig()
		consoleEncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		consoleEncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05")
		consoleEncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
		consoleCore := zapcore.NewCore(
			zapcore.NewConsoleEncoder(consoleEncoderConfig),
			zapcore.AddSync(os.Stdout),
			getLogLevel(debug),
		)

		// 2. 配置文件输出 (JSON + Rotation)
		fileEncoderConfig := zap.NewProductionEncoderConfig()
		fileEncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		fileWriteSyncer := zapcore.AddSync(&lumberjack.Logger{
			Filename:   filepath.Join(logDir, "app.json"),
			MaxSize:    10,   // 每个日志文件最大 10MB
			MaxBackups: 30,   // 保留最近 30 个文件
			MaxAge:     30,   // 保留最近 30 天
			Compress:   true, // 压缩旧日志
		})
		fileCore := zapcore.NewCore(
			zapcore.NewJSONEncoder(fileEncoderConfig),
			fileWriteSyncer,
			zapcore.InfoLevel, // 文件始终记录 INFO 及以上
		)

		// 3. 组合 Core
		core := zapcore.NewTee(consoleCore, fileCore)

		// 4. 构建 Logger (添加 Caller 和 Stacktrace)
		Log = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
		
		// 替换全局标准库 log (可选，拦截部分第三方库的 log)
		zap.ReplaceGlobals(Log)
	})
}

func getLogLevel(debug bool) zapcore.LevelEnabler {
	if debug {
		return zapcore.DebugLevel
	}
	return zapcore.InfoLevel
}

// 辅助函数：快速获取带 module 字段的 logger
func NewModuleLogger(moduleName string) *zap.Logger {
	if Log == nil {
		InitLogger("logs", true) // 防止未初始化调用 panic
	}
	return Log.With(zap.String("module", moduleName))
}

// 辅助函数：通用日志方法
func Info(msg string, fields ...zap.Field) {
	if Log != nil {
		Log.Info(msg, fields...)
	}
}

func Error(msg string, fields ...zap.Field) {
	if Log != nil {
		Log.Error(msg, fields...)
	}
}

func Warn(msg string, fields ...zap.Field) {
	if Log != nil {
		Log.Warn(msg, fields...)
	}
}

func Debug(msg string, fields ...zap.Field) {
	if Log != nil {
		Log.Debug(msg, fields...)
	}
}
