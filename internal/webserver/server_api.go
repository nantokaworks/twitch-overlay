package webserver

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/nantokaworks/twitch-overlay/internal/output"
	"github.com/nantokaworks/twitch-overlay/internal/shared/logger"
	"go.uber.org/zap"
)

// RestartExitCode は systemd で再起動をトリガーする終了コード
const RestartExitCode = 75

type RestartRequest struct {
	Force bool `json:"force,omitempty"` // 強制再起動フラグ
}

type RestartResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Warning string `json:"warning,omitempty"`
}

// isRunningAsService はサービスとして実行されているかを判定
func isRunningAsService() bool {
	// 環境変数で明示的に指定されている場合
	if os.Getenv("RUNNING_AS_SERVICE") == "true" {
		return true
	}

	// systemd環境の検出（Linux）
	if runtime.GOOS == "linux" {
		// systemdの環境変数をチェック
		if os.Getenv("INVOCATION_ID") != "" || os.Getenv("JOURNAL_STREAM") != "" {
			return true
		}

		// /proc/1/comm でinitシステムを確認
		if data, err := os.ReadFile("/proc/1/comm"); err == nil {
			initSystem := strings.TrimSpace(string(data))
			if initSystem == "systemd" {
				// systemdが動作している環境
				// 追加で親プロセスがsystemdかチェック
				if ppid := os.Getppid(); ppid == 1 {
					return true
				}
			}
		}
	}

	return false
}

// handleServerRestart はサーバーの再起動を処理
func handleServerRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RestartRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			logger.Error("Failed to decode restart request", zap.Error(err))
			// エラーを無視してデフォルト値で続行
			req = RestartRequest{}
		}
	}

	logger.Info("Server restart requested", 
		zap.Bool("force", req.Force),
		zap.Bool("running_as_service", isRunningAsService()))

	response := RestartResponse{
		Success: true,
		Message: "サーバーを再起動します",
	}

	// 印刷キューをチェック（強制再起動でない場合）
	if !req.Force {
		queueSize := output.GetPrintQueueSize()
		if queueSize > 0 {
			logger.Warn("Restart blocked: print queue not empty", zap.Int("queue_size", queueSize))
			response.Success = false
			response.Message = "印刷キューが空でないため再起動できません"
			response.Warning = "処理中の印刷ジョブがあります。完了を待つか、強制再起動してください"
			
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(response)
			return
		}
	}

	// 成功レスポンスを先に送信
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

	// クライアントにレスポンスを送信するため少し待機
	time.Sleep(100 * time.Millisecond)

	// 再起動処理を非同期で実行
	go func() {
		// シャットダウン前に少し待機（クライアントがレスポンスを受信するため）
		time.Sleep(500 * time.Millisecond)

		if isRunningAsService() {
			// サービスモード: 特定の終了コードで終了してsystemdに再起動を任せる
			logger.Info("Exiting with restart code for systemd", zap.Int("exit_code", RestartExitCode))
			
			// グレースフルシャットダウン
			Shutdown()
			
			// systemd用の再起動コードで終了
			os.Exit(RestartExitCode)
		} else {
			// 通常モード: 新しいプロセスを起動してから終了
			logger.Info("Restarting in standalone mode")
			
			// 実行ファイルのパスを取得
			executable, err := os.Executable()
			if err != nil {
				logger.Error("Failed to get executable path", zap.Error(err))
				return
			}

			// シンボリックリンクを解決
			executable, err = filepath.EvalSymlinks(executable)
			if err != nil {
				logger.Error("Failed to resolve symlinks", zap.Error(err))
				return
			}

			// 新しいプロセスを起動
			cmd := exec.Command(executable, os.Args[1:]...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Stdin = os.Stdin
			cmd.Env = os.Environ()

			if err := cmd.Start(); err != nil {
				logger.Error("Failed to start new process", zap.Error(err))
				return
			}

			logger.Info("New process started", zap.Int("pid", cmd.Process.Pid))
			
			// グレースフルシャットダウン
			Shutdown()
			
			// 現在のプロセスを終了
			os.Exit(0)
		}
	}()
}

// handleServerStatus はサーバーのステータスを返す
func handleServerStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := map[string]interface{}{
		"running":          true,
		"running_as_service": isRunningAsService(),
		"print_queue_size": output.GetPrintQueueSize(),
		"uptime":           time.Since(startTime).Seconds(),
		"version":          "1.0.0", // TODO: バージョン情報を取得
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// startTime はサーバーの起動時刻
var startTime = time.Now()