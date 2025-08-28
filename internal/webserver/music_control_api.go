package webserver

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/nantokaworks/twitch-overlay/internal/shared/logger"
	"go.uber.org/zap"
)

type MusicControlCommand struct {
	Type     string `json:"type"`     // play, pause, next, previous, volume, load_playlist
	Value    int    `json:"value,omitempty"`
	Playlist string `json:"playlist,omitempty"`
}

type MusicStatusUpdate struct {
	IsPlaying    bool    `json:"is_playing"`
	CurrentTrack *Track  `json:"current_track,omitempty"`
	Progress     float64 `json:"progress"`
	CurrentTime  float64 `json:"current_time"`
	Duration     float64 `json:"duration"`
	Volume       int     `json:"volume"`
	PlaylistName *string `json:"playlist_name,omitempty"`
}

type Track struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	Album    string `json:"album,omitempty"`
	Duration int    `json:"duration"`
	HasArtwork bool `json:"has_artwork"`
}

var (
	musicControlClients = make(map[chan MusicControlCommand]bool)
	musicControlMutex   sync.RWMutex
	
	musicStatusClients = make(map[chan MusicStatusUpdate]bool)
	musicStatusMutex   sync.RWMutex
)

// SSEクライアントを登録
func addMusicControlClient(client chan MusicControlCommand) {
	musicControlMutex.Lock()
	defer musicControlMutex.Unlock()
	musicControlClients[client] = true
}

// SSEクライアントを削除
func removeMusicControlClient(client chan MusicControlCommand) {
	musicControlMutex.Lock()
	defer musicControlMutex.Unlock()
	delete(musicControlClients, client)
	close(client)
}

// 全クライアントにコマンドを送信
func broadcastMusicCommand(cmd MusicControlCommand) {
	musicControlMutex.RLock()
	defer musicControlMutex.RUnlock()
	
	for client := range musicControlClients {
		select {
		case client <- cmd:
		default:
			// クライアントがブロックされている場合はスキップ
		}
	}
}

// SSEクライアントを登録（ステータス用）
func addMusicStatusClient(client chan MusicStatusUpdate) {
	musicStatusMutex.Lock()
	defer musicStatusMutex.Unlock()
	musicStatusClients[client] = true
}

// SSEクライアントを削除（ステータス用）
func removeMusicStatusClient(client chan MusicStatusUpdate) {
	musicStatusMutex.Lock()
	defer musicStatusMutex.Unlock()
	delete(musicStatusClients, client)
	close(client)
}

// 全クライアントにステータスを送信
func broadcastMusicStatus(status MusicStatusUpdate) {
	musicStatusMutex.RLock()
	defer musicStatusMutex.RUnlock()
	
	for client := range musicStatusClients {
		select {
		case client <- status:
		default:
			// クライアントがブロックされている場合はスキップ
		}
	}
}

// POST /api/music/control/play
func handleMusicPlay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cmd := MusicControlCommand{Type: "play"}
	broadcastMusicCommand(cmd)
	logger.Info("Music play command sent")
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// POST /api/music/control/pause
func handleMusicPause(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cmd := MusicControlCommand{Type: "pause"}
	broadcastMusicCommand(cmd)
	logger.Info("Music pause command sent")
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// POST /api/music/control/next
func handleMusicNext(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cmd := MusicControlCommand{Type: "next"}
	broadcastMusicCommand(cmd)
	logger.Info("Music next command sent")
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// POST /api/music/control/previous
func handleMusicPrevious(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cmd := MusicControlCommand{Type: "previous"}
	broadcastMusicCommand(cmd)
	logger.Info("Music previous command sent")
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// POST /api/music/control/volume
func handleMusicVolume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Volume int `json:"volume"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	if req.Volume < 0 || req.Volume > 100 {
		http.Error(w, "Volume must be between 0 and 100", http.StatusBadRequest)
		return
	}

	cmd := MusicControlCommand{
		Type:  "volume",
		Value: req.Volume,
	}
	broadcastMusicCommand(cmd)
	logger.Info("Music volume command sent", zap.Int("volume", req.Volume))
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// POST /api/music/control/load
func handleMusicLoad(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Playlist string `json:"playlist,omitempty"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	cmd := MusicControlCommand{
		Type:     "load_playlist",
		Playlist: req.Playlist,
	}
	broadcastMusicCommand(cmd)
	logger.Info("Music load playlist command sent", zap.String("playlist", req.Playlist))
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// SSE: /api/music/control/events
func handleMusicControlEvents(w http.ResponseWriter, r *http.Request) {
	// SSEヘッダー設定
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// クライアントチャンネル作成
	client := make(chan MusicControlCommand)
	addMusicControlClient(client)
	defer removeMusicControlClient(client)

	// クライアント切断検知
	ctx := r.Context()

	for {
		select {
		case cmd := <-client:
			data, err := json.Marshal(cmd)
			if err != nil {
				logger.Error("Failed to marshal music command", zap.Error(err))
				continue
			}
			
			// SSEフォーマットで送信
			_, err = w.Write([]byte("data: " + string(data) + "\n\n"))
			if err != nil {
				logger.Debug("Client disconnected from music control SSE")
				return
			}
			
			// フラッシュしてリアルタイム送信
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			
		case <-ctx.Done():
			logger.Debug("Music control SSE connection closed")
			return
		}
	}
}

// POST /api/music/status/update
func handleMusicStatusUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var status MusicStatusUpdate
	if err := json.NewDecoder(r.Body).Decode(&status); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 全クライアントに状態を配信
	broadcastMusicStatus(status)
	logger.Debug("Music status broadcasted", zap.Bool("is_playing", status.IsPlaying))
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// SSE: /api/music/status/events
func handleMusicStatusEvents(w http.ResponseWriter, r *http.Request) {
	// SSEヘッダー設定
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// クライアントチャンネル作成
	client := make(chan MusicStatusUpdate)
	addMusicStatusClient(client)
	defer removeMusicStatusClient(client)

	// クライアント切断検知
	ctx := r.Context()

	for {
		select {
		case status := <-client:
			data, err := json.Marshal(status)
			if err != nil {
				logger.Error("Failed to marshal music status", zap.Error(err))
				continue
			}
			
			// SSEフォーマットで送信
			_, err = w.Write([]byte("data: " + string(data) + "\n\n"))
			if err != nil {
				logger.Debug("Client disconnected from music status SSE")
				return
			}
			
			// フラッシュしてリアルタイム送信
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			
		case <-ctx.Done():
			logger.Debug("Music status SSE connection closed")
			return
		}
	}
}

// RegisterMusicControlRoutes 音楽制御用のルートを登録
func RegisterMusicControlRoutes(mux *http.ServeMux) {
	// 制御エンドポイント
	mux.HandleFunc("/api/music/control/play", corsMiddleware(handleMusicPlay))
	mux.HandleFunc("/api/music/control/pause", corsMiddleware(handleMusicPause))
	mux.HandleFunc("/api/music/control/next", corsMiddleware(handleMusicNext))
	mux.HandleFunc("/api/music/control/previous", corsMiddleware(handleMusicPrevious))
	mux.HandleFunc("/api/music/control/volume", corsMiddleware(handleMusicVolume))
	mux.HandleFunc("/api/music/control/load", corsMiddleware(handleMusicLoad))
	
	// SSEエンドポイント
	mux.HandleFunc("/api/music/control/events", corsMiddleware(handleMusicControlEvents))
	
	// 状態同期エンドポイント
	mux.HandleFunc("/api/music/status/update", corsMiddleware(handleMusicStatusUpdate))
	mux.HandleFunc("/api/music/status/events", corsMiddleware(handleMusicStatusEvents))
}