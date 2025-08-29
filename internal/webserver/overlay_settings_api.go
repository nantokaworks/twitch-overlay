package webserver

import (
	"encoding/json"
	"net/http"
	"os"
	"sync"
	"time"

	"fmt"
	
	"github.com/nantokaworks/twitch-overlay/internal/shared/logger"
	"go.uber.org/zap"
)

// OverlaySettings represents the overlay display settings
type OverlaySettings struct {
	// 音楽プレイヤー設定
	MusicEnabled  bool    `json:"music_enabled"`
	MusicPlaylist *string `json:"music_playlist"`
	MusicVolume   int     `json:"music_volume"`
	MusicAutoPlay bool    `json:"music_auto_play"`

	// FAX表示設定
	FaxEnabled        bool    `json:"fax_enabled"`
	FaxAnimationSpeed float64 `json:"fax_animation_speed"`

	// 時計表示設定
	ClockEnabled    bool   `json:"clock_enabled"`
	ClockFormat     string `json:"clock_format"` // "12h" or "24h"
	LocationEnabled bool   `json:"location_enabled"`
	DateEnabled     bool   `json:"date_enabled"`
	TimeEnabled     bool   `json:"time_enabled"`
	StatsEnabled    bool   `json:"stats_enabled"`

	// その他の表示設定
	ShowDebugInfo bool `json:"show_debug_info"`

	UpdatedAt time.Time `json:"updated_at"`
}

var (
	currentOverlaySettings *OverlaySettings
	overlaySettingsMutex   sync.RWMutex
	overlaySettingsFile    = "data/overlay_settings.json"

	// SSE clients for settings updates
	settingsEventClients   = make(map[chan string]bool)
	settingsEventClientsMu sync.RWMutex
)

// InitOverlaySettings initializes the overlay settings from saved file
func InitOverlaySettings() {
	// Create data directory if it doesn't exist
	os.MkdirAll("data", 0755)

	// デフォルト設定
	defaultSettings := &OverlaySettings{
		MusicEnabled:      true,
		MusicPlaylist:     nil, // nil = all tracks
		MusicVolume:       70,
		MusicAutoPlay:     false,
		FaxEnabled:        true,
		FaxAnimationSpeed: 1.0,
		ClockEnabled:      true,
		ClockFormat:       "24h",
		LocationEnabled:   true,
		DateEnabled:       true,
		TimeEnabled:       true,
		StatsEnabled:      true,
		ShowDebugInfo:     false,
		UpdatedAt:         time.Now(),
	}

	// Try to load existing settings
	if data, err := os.ReadFile(overlaySettingsFile); err == nil {
		var settings OverlaySettings
		if err := json.Unmarshal(data, &settings); err == nil {
			overlaySettingsMutex.Lock()
			currentOverlaySettings = &settings
			overlaySettingsMutex.Unlock()

			logger.Info("Restored overlay settings",
				zap.Bool("music_enabled", settings.MusicEnabled),
				zap.Bool("fax_enabled", settings.FaxEnabled),
				zap.Bool("clock_enabled", settings.ClockEnabled))
			return
		}
	}

	// Use default settings if file doesn't exist or is invalid
	overlaySettingsMutex.Lock()
	currentOverlaySettings = defaultSettings
	overlaySettingsMutex.Unlock()

	// Save default settings
	saveOverlaySettings(defaultSettings)
}

// saveOverlaySettings saves settings to file
func saveOverlaySettings(settings *OverlaySettings) error {
	settings.UpdatedAt = time.Now()

	if data, err := json.MarshalIndent(settings, "", "  "); err == nil {
		if err := os.WriteFile(overlaySettingsFile, data, 0644); err != nil {
			logger.Error("Failed to save overlay settings", zap.Error(err))
			return err
		}
	}
	return nil
}

// broadcastSettingsUpdate sends settings update to all SSE clients
func broadcastSettingsUpdate(settings *OverlaySettings) {
	settingsEventClientsMu.RLock()
	defer settingsEventClientsMu.RUnlock()

	data, err := json.Marshal(settings)
	if err != nil {
		logger.Error("Failed to marshal settings for SSE", zap.Error(err))
		return
	}

	message := "data: " + string(data) + "\n\n"
	for client := range settingsEventClients {
		select {
		case client <- message:
			// Sent successfully
		default:
			// Client is not ready, skip
		}
	}
}

// handleOverlaySettingsUpdate handles POST /api/settings/overlay
func handleOverlaySettingsUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var settings OverlaySettings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Update in-memory settings
	overlaySettingsMutex.Lock()
	currentOverlaySettings = &settings
	overlaySettingsMutex.Unlock()

	// Save to file
	if err := saveOverlaySettings(&settings); err != nil {
		http.Error(w, "Failed to save settings", http.StatusInternalServerError)
		return
	}

	// Broadcast to SSE clients
	broadcastSettingsUpdate(&settings)

	logger.Debug("Updated overlay settings",
		zap.Bool("music_enabled", settings.MusicEnabled),
		zap.Bool("fax_enabled", settings.FaxEnabled))

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleOverlaySettingsGet handles GET /api/settings/overlay
func handleOverlaySettingsGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	overlaySettingsMutex.RLock()
	settings := currentOverlaySettings
	overlaySettingsMutex.RUnlock()

	if settings == nil {
		// Return default settings if not initialized
		settings = &OverlaySettings{
			MusicEnabled:      true,
			MusicVolume:       70,
			FaxEnabled:        true,
			FaxAnimationSpeed: 1.0,
			ClockEnabled:      true,
			ClockFormat:       "24h",
			LocationEnabled:   true,
			DateEnabled:       true,
			TimeEnabled:       true,
			StatsEnabled:      true,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}

// handleOverlaySettingsEvents handles SSE for settings updates
func handleOverlaySettingsEvents(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create client channel
	clientChan := make(chan string, 10)

	// Register client
	settingsEventClientsMu.Lock()
	settingsEventClients[clientChan] = true
	settingsEventClientsMu.Unlock()

	// Remove client on disconnect
	defer func() {
		settingsEventClientsMu.Lock()
		delete(settingsEventClients, clientChan)
		close(clientChan)
		settingsEventClientsMu.Unlock()
	}()

	// Send initial settings
	overlaySettingsMutex.RLock()
	if currentOverlaySettings != nil {
		if data, err := json.Marshal(currentOverlaySettings); err == nil {
			fmt.Fprintf(w, "data: %s\n\n", string(data))
			w.(http.Flusher).Flush()
		}
	}
	overlaySettingsMutex.RUnlock()

	// Keep connection alive
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case msg := <-clientChan:
			fmt.Fprint(w, msg)
			w.(http.Flusher).Flush()
		case <-ticker.C:
			fmt.Fprintf(w, ": keepalive\n\n")
			w.(http.Flusher).Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// RegisterOverlaySettingsRoutes registers overlay settings routes
func RegisterOverlaySettingsRoutes(mux *http.ServeMux) {
	// Initialize settings on startup
	InitOverlaySettings()

	// Register routes
	mux.HandleFunc("/api/settings/overlay", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleOverlaySettingsGet(w, r)
		case http.MethodPost:
			handleOverlaySettingsUpdate(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))
	mux.HandleFunc("/api/settings/overlay/events", corsMiddleware(handleOverlaySettingsEvents))
}