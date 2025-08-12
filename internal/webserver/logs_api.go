package webserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nantokaworks/twitch-fax/internal/shared/logger"
	"go.uber.org/zap"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// 開発環境では全てのオリジンを許可
		// TODO: 本番環境では適切なオリジンチェックを実装
		return true
	},
}

// WebSocket接続を管理
type LogStreamer struct {
	clients map[*websocket.Conn]bool
	broadcast chan logger.LogEntry
	register chan *websocket.Conn
	unregister chan *websocket.Conn
}

var logStreamer = &LogStreamer{
	clients:    make(map[*websocket.Conn]bool),
	broadcast:  make(chan logger.LogEntry),
	register:   make(chan *websocket.Conn),
	unregister: make(chan *websocket.Conn),
}

func init() {
	go logStreamer.run()
}

func (ls *LogStreamer) run() {
	for {
		select {
		case client := <-ls.register:
			ls.clients[client] = true
			logger.Info("WebSocket client connected for logs")

		case client := <-ls.unregister:
			if _, ok := ls.clients[client]; ok {
				delete(ls.clients, client)
				client.Close()
				logger.Info("WebSocket client disconnected from logs")
			}

		case entry := <-ls.broadcast:
			for client := range ls.clients {
				err := client.WriteJSON(entry)
				if err != nil {
					client.Close()
					delete(ls.clients, client)
				}
			}
		}
	}
}

// BroadcastLog sends a log entry to all connected WebSocket clients
func BroadcastLog(entry logger.LogEntry) {
	select {
	case logStreamer.broadcast <- entry:
	default:
		// チャネルがブロックされている場合はスキップ
	}
}

// handleLogs returns recent logs
func handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// クエリパラメータから件数を取得
	limitStr := r.URL.Query().Get("limit")
	limit := 100 // デフォルト100件
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	// ログバッファから取得
	buffer := logger.GetLogBuffer()
	logs := buffer.GetRecent(limit)

	// レスポンス
	response := map[string]interface{}{
		"logs":      logs,
		"count":     len(logs),
		"timestamp": time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleLogsDownload downloads logs as a file
func handleLogsDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	buffer := logger.GetLogBuffer()
	
	switch format {
	case "json":
		data, err := buffer.ToJSON()
		if err != nil {
			http.Error(w, "Failed to generate JSON", http.StatusInternalServerError)
			return
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=twitch-fax-logs-%s.json", time.Now().Format("20060102-150405")))
		w.Write(data)
		
	case "text":
		data := buffer.ToText()
		
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=twitch-fax-logs-%s.txt", time.Now().Format("20060102-150405")))
		w.Write([]byte(data))
		
	default:
		http.Error(w, "Invalid format. Use 'json' or 'text'", http.StatusBadRequest)
	}
}

// handleLogsStream provides real-time log streaming via WebSocket
func handleLogsStream(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("Failed to upgrade to WebSocket", zap.Error(err))
		return
	}

	// クライアントを登録
	logStreamer.register <- conn

	// 最近のログを送信
	buffer := logger.GetLogBuffer()
	recentLogs := buffer.GetRecent(50)
	for _, log := range recentLogs {
		if err := conn.WriteJSON(log); err != nil {
			break
		}
	}

	// 接続を維持
	defer func() {
		logStreamer.unregister <- conn
	}()

	// クライアントからのメッセージを読み続ける（接続維持のため）
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

// handleLogsClear clears the log buffer
func handleLogsClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	buffer := logger.GetLogBuffer()
	buffer.Clear()

	logger.Info("Log buffer cleared")

	response := map[string]interface{}{
		"success": true,
		"message": "Log buffer cleared",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}