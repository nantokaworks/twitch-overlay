package webserver

import (
	"encoding/json"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/nantokaworks/twitch-overlay/internal/shared/logger"
	"go.uber.org/zap"
)

// PlaybackState represents the current playback state
type PlaybackState struct {
	TrackID      string    `json:"track_id"`
	Position     float64   `json:"position"`      // 秒
	Duration     float64   `json:"duration"`      
	IsPlaying    bool      `json:"is_playing"`
	Volume       int       `json:"volume"`
	PlaylistName *string   `json:"playlist_name"`
	UpdatedAt    time.Time `json:"updated_at"`
}

var (
	currentPlaybackState *PlaybackState
	playbackStateMutex   sync.RWMutex
	playbackStateFile    = "data/playback_state.json"
)

// InitPlaybackState initializes the playback state from saved file
func InitPlaybackState() {
	// Create data directory if it doesn't exist
	os.MkdirAll("data", 0755)
	
	// Try to load existing state
	if data, err := os.ReadFile(playbackStateFile); err == nil {
		var state PlaybackState
		if err := json.Unmarshal(data, &state); err == nil {
			playbackStateMutex.Lock()
			currentPlaybackState = &state
			playbackStateMutex.Unlock()
			
			logger.Info("Restored playback state",
				zap.String("track_id", state.TrackID),
				zap.Float64("position", state.Position),
				zap.Time("updated_at", state.UpdatedAt))
		}
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

	// Persist to file
	if data, err := json.MarshalIndent(state, "", "  "); err == nil {
		if err := os.WriteFile(playbackStateFile, data, 0644); err != nil {
			logger.Error("Failed to save playback state", zap.Error(err))
		}
	}

	logger.Debug("Updated playback state",
		zap.String("track_id", state.TrackID),
		zap.Float64("position", state.Position),
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
		// Try to load from file
		if data, err := os.ReadFile(playbackStateFile); err == nil {
			var fileState PlaybackState
			if err := json.Unmarshal(data, &fileState); err == nil {
				state = &fileState
			}
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