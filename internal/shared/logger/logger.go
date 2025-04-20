package logger

import (
	"log"
	"os"
	"strings"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Log *zap.Logger

var once sync.Once

func init() {

	once.Do(func() {
		// ログ設定を構築
		config := zap.NewProductionConfig()
		config.OutputPaths = []string{"stdout"}
		config.ErrorOutputPaths = []string{"stdout"}

		// 表示するログレベルを設定
		config.Level = zap.NewAtomicLevelAt(getZapLogLevel())

		// ロガーを構築
		var err error
		Log, err = config.Build()
		if err != nil {
			log.Panic("failed to build logger", err)
		}

		// Zap ロガーを標準ロガーとして設定
		zapLogger := zap.NewStdLog(Log)
		// 標準ログのタイムスタンプを無効化
		log.SetFlags(0)
		log.SetOutput(zapLogger.Writer())
	})
}

// Debug は debug レベルでのログ出力
func Debug(msg string, fields ...zap.Field) {
	Log.Debug(msg, fields...)
}

// Info は info レベルでのログ出力
func Info(msg string, fields ...zap.Field) {
	Log.Info(msg, fields...)
}

// Warn は warn レベルでのログ出力
func Warn(msg string, fields ...zap.Field) {
	Log.Warn(msg, fields...)
}

// Error は error レベルでのログ出力
func Error(msg string, fields ...zap.Field) {
	Log.Error(msg, fields...)
}

// DPanic は開発中にのみ panic するレベル (開発モードで panic し、本番では error 出力)
func DPanic(msg string, fields ...zap.Field) {
	Log.DPanic(msg, fields...)
}

// Panic は panic を発生させつつログ出力
func Panic(msg string, fields ...zap.Field) {
	Log.Panic(msg, fields...)
}

// Fatal は fatal レベルでのログ出力し、os.Exit(1) を呼び出す
func Fatal(msg string, fields ...zap.Field) {
	Log.Fatal(msg, fields...)
}

// Sync はログバッファの内容をフラッシュします。
// アプリケーション終了時に呼び出すことが推奨されます。
func Sync() error {
	return Log.Sync()
}

func getZapLogLevel() zapcore.Level {
	levelStr := strings.ToLower(os.Getenv("LOG_LEVEL"))
	switch levelStr {
	case "debug":
		return zap.DebugLevel
	case "info":
		return zap.InfoLevel
	case "warn", "warning":
		return zap.WarnLevel
	case "error":
		return zap.ErrorLevel
	case "fatal":
		return zap.FatalLevel
	default:
		return zap.InfoLevel
	}
}
