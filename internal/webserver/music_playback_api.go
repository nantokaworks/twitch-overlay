package webserver

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/nantokaworks/twitch-overlay/internal/localdb"
	"github.com/nantokaworks/twitch-overlay/internal/shared/logger"
	"go.uber.org/zap"
)

// PlaybackState represents the current playback state
type PlaybackState struct {
	TrackID        string    `json:"track_id"`
	Position       float64   `json:"position"`      // 秒
	Duration       float64   `json:"duration"`      
	PlaybackStatus string    `json:"playback_status,omitempty"` // playing, paused, stopped
	IsPlaying      bool      `json:"is_playing"`    // 互換性のため残す
	Volume         int       `json:"volume"`
	PlaylistName   *string   `json:"playlist_name"`
	UpdatedAt      time.Time `json:"updated_at"`
}

var (
	currentPlaybackState *PlaybackState
	playbackStateMutex   sync.RWMutex
	playbackStateFile    = "data/playback_state.json" // マイグレーション用に残す
)

// savePlaybackStateDB saves playback state to database
func savePlaybackStateDB(state *PlaybackState) error {
	db := localdb.GetDB()
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	// SQLiteでは常に単一レコードを保持（id=1を固定使用）
	_, err := db.Exec(`
		INSERT OR REPLACE INTO playback_state 
		(id, track_id, position, duration, playback_status, is_playing, volume, playlist_name, updated_at)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?)
	`, state.TrackID, state.Position, state.Duration, state.PlaybackStatus, 
	   state.IsPlaying, state.Volume, state.PlaylistName, state.UpdatedAt)

	if err != nil {
		logger.Error("Failed to save playback state to DB", zap.Error(err))
		return err
	}

	logger.Debug("Saved playback state to DB", 
		zap.String("track_id", state.TrackID),
		zap.Float64("position", state.Position))
	return nil
}

// loadPlaybackStateDB loads playback state from database
func loadPlaybackStateDB() (*PlaybackState, error) {
	db := localdb.GetDB()
	if db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	row := db.QueryRow(`
		SELECT track_id, position, duration, playback_status, is_playing, volume, playlist_name, updated_at
		FROM playback_state WHERE id = 1
	`)

	var state PlaybackState
	var playlistName sql.NullString
	err := row.Scan(&state.TrackID, &state.Position, &state.Duration, 
		&state.PlaybackStatus, &state.IsPlaying, &state.Volume, &playlistName, &state.UpdatedAt)
	
	if err != nil {
		return nil, err
	}

	if playlistName.Valid {
		state.PlaylistName = &playlistName.String
	}
	return &state, nil
}

// InitPlaybackState initializes the playback state from database (with JSON migration)
func InitPlaybackState() {
	// まずDBから状態を読み込み
	if state, err := loadPlaybackStateDB(); err == nil {
		playbackStateMutex.Lock()
		currentPlaybackState = state
		playbackStateMutex.Unlock()
		
		logger.Info("Restored playback state from DB",
			zap.String("track_id", state.TrackID),
			zap.Float64("position", state.Position),
			zap.Time("updated_at", state.UpdatedAt))
		return
	}
	
	// DBに状態がない場合、JSONファイルからマイグレーション
	logger.Info("No playback state in DB, attempting JSON migration...")
	os.MkdirAll("data", 0755)
	
	if data, err := os.ReadFile(playbackStateFile); err == nil {
		var state PlaybackState
		if err := json.Unmarshal(data, &state); err == nil {
			// DBに保存
			if err := savePlaybackStateDB(&state); err == nil {
				playbackStateMutex.Lock()
				currentPlaybackState = &state
				playbackStateMutex.Unlock()
				
				logger.Info("Successfully migrated playback state from JSON to DB",
					zap.String("track_id", state.TrackID),
					zap.Float64("position", state.Position))
				
				// マイグレーション成功後、JSONファイルをバックアップとしてリネーム
				backupFile := playbackStateFile + ".migrated"
				os.Rename(playbackStateFile, backupFile)
				logger.Info("JSON file backed up", zap.String("backup_file", backupFile))
			} else {
				logger.Error("Failed to migrate playback state to DB", zap.Error(err))
			}
		}
	} else {
		logger.Info("No existing JSON playback state found")
	}
}

// handlePlaybackStateUpdate handles POST /api/music/state/update
func handlePlaybackStateUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var state PlaybackState
	if err := json.NewDecoder(r.Body).Decode(&state); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	state.UpdatedAt = time.Now()

	// Update in-memory state
	playbackStateMutex.Lock()
	currentPlaybackState = &state
	playbackStateMutex.Unlock()

	// Persist to database
	if err := savePlaybackStateDB(&state); err != nil {
		logger.Error("Failed to save playback state to DB", zap.Error(err))
		http.Error(w, "Failed to save state", http.StatusInternalServerError)
		return
	}

	logger.Debug("Updated playback state",
		zap.String("track_id", state.TrackID),
		zap.Float64("position", state.Position),
		zap.String("playback_status", state.PlaybackStatus),
		zap.Bool("is_playing", state.IsPlaying))

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// RegisterPlaybackRoutes registers playback-related routes
func RegisterPlaybackRoutes(mux *http.ServeMux) {
	// Initialize playback state on startup
	InitPlaybackState()
	
	// Register routes
	mux.HandleFunc("/api/music/state/update", corsMiddleware(handlePlaybackStateUpdate))
	mux.HandleFunc("/api/music/state/get", corsMiddleware(handlePlaybackStateGet))
}

// handlePlaybackStateGet handles GET /api/music/state/get
func handlePlaybackStateGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	playbackStateMutex.RLock()
	state := currentPlaybackState
	playbackStateMutex.RUnlock()

	if state == nil {
		// Try to load from database
		if dbState, err := loadPlaybackStateDB(); err == nil {
			state = dbState
		}
	}

	if state == nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "no saved state"})
		return
	}

	// Check if state is too old (24 hours)
	if time.Since(state.UpdatedAt) > 24*time.Hour {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "state too old"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(state)
}