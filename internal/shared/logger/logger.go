package logger

import (
	"encoding/json"
	"log"
	"os"
	"strings"
	"sync"
	"time"

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

		// カスタムコアを作成してログバッファに追加
		encoderConfig := config.EncoderConfig
		encoder := zapcore.NewJSONEncoder(encoderConfig)
		
		// 標準出力用のコア
		stdoutCore := zapcore.NewCore(
			encoder,
			zapcore.AddSync(os.Stdout),
			config.Level,
		)

		// バッファ用のコア
		bufferCore := zapcore.NewCore(
			encoder,
			zapcore.AddSync(&bufferWriter{}),
			config.Level,
		)

		// 両方のコアを組み合わせる
		core := zapcore.NewTee(stdoutCore, bufferCore)
		
		// ロガーを構築
		Log = zap.New(core, zap.AddStacktrace(zapcore.ErrorLevel))

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

// bufferWriter はログをバッファに書き込むためのライター
type bufferWriter struct{}

func (bw *bufferWriter) Write(p []byte) (n int, err error) {
	// JSON形式のログをパース
	var logData map[string]interface{}
	if err := json.Unmarshal(p, &logData); err == nil {
		entry := LogEntry{
			Timestamp: time.Now(),
		}
		
		if level, ok := logData["level"].(string); ok {
			entry.Level = level
			delete(logData, "level")
		}
		
		if msg, ok := logData["msg"].(string); ok {
			entry.Message = msg
			delete(logData, "msg")
		}
		
		if ts, ok := logData["ts"].(float64); ok {
			entry.Timestamp = time.Unix(int64(ts), 0)
			delete(logData, "ts")
		}
		
		// 残りのフィールドを追加
		if len(logData) > 0 {
			entry.Fields = logData
		}
		
		GetLogBuffer().Add(entry)
	}
	
	return len(p), nil
}
