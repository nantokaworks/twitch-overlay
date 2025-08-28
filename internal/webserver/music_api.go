package webserver

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/nantokaworks/twitch-overlay/internal/music"
	"github.com/nantokaworks/twitch-overlay/internal/shared/logger"
	"go.uber.org/zap"
)

func handleMusicUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form (50MB limit)
	err := r.ParseMultipartForm(50 << 20)
	if err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	// Get the file
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Failed to get file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Save the track
	manager := music.GetManager()
	track, err := manager.SaveTrack(header.Filename, file, header.Size)
	if err != nil {
		logger.Error("Failed to save track", zap.Error(err))
		
		switch err {
		case music.ErrFileTooLarge:
			http.Error(w, "File too large (max 50MB)", http.StatusRequestEntityTooLarge)
		case music.ErrInvalidFormat:
			http.Error(w, "Invalid audio format (only MP3/WAV/M4A/OGG supported)", http.StatusBadRequest)
		default:
			http.Error(w, "Failed to save track", http.StatusInternalServerError)
		}
		return
	}

	// プレイリストIDが指定されていれば追加
	playlistID := r.FormValue("playlist_id")
	if playlistID != "" {
		err := manager.AddTrackToPlaylist(playlistID, track.ID, 0)
		if err != nil {
			logger.Warn("Failed to add track to playlist", 
				zap.String("playlist_id", playlistID),
				zap.String("track_id", track.ID),
				zap.Error(err))
			// プレイリスト追加に失敗してもトラック自体は保存されているので続行
		} else {
			logger.Info("Track added to playlist",
				zap.String("playlist_id", playlistID),
				zap.String("track_id", track.ID))
		}
	}

	// Return track info
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(track)
}

func handleGetTracks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	manager := music.GetManager()
	tracks, err := manager.GetAllTracks()
	if err != nil {
		logger.Error("Failed to get tracks", zap.Error(err))
		http.Error(w, "Failed to get tracks", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tracks": tracks,
		"count":  len(tracks),
	})
}

func handleGetTrack(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract track ID from URL
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/music/track/"), "/")
	if len(pathParts) < 1 || pathParts[0] == "" {
		http.Error(w, "Track ID required", http.StatusBadRequest)
		return
	}

	trackID := pathParts[0]
	manager := music.GetManager()

	// Check if requesting audio or artwork
	if len(pathParts) >= 2 {
		switch pathParts[1] {
		case "audio":
			// Serve audio file
			trackPath, err := manager.GetTrackPath(trackID)
			if err != nil {
				http.Error(w, "Track not found", http.StatusNotFound)
				return
			}

			// Open and serve the file
			file, err := os.Open(trackPath)
			if err != nil {
				http.Error(w, "Failed to open track", http.StatusInternalServerError)
				return
			}
			defer file.Close()

			// Get file info for content length
			stat, _ := file.Stat()
			
			// Determine content type
			ext := strings.ToLower(trackPath[strings.LastIndex(trackPath, "."):])
			contentType := "audio/mpeg"
			switch ext {
			case ".wav":
				contentType = "audio/wav"
			case ".ogg":
				contentType = "audio/ogg"
			case ".m4a":
				contentType = "audio/mp4"
			}

			// Set headers for audio streaming
			w.Header().Set("Content-Type", contentType)
			w.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size()))
			w.Header().Set("Accept-Ranges", "bytes")
			w.Header().Set("Cache-Control", "public, max-age=3600")

			// Handle range requests for audio seeking
			rangeHeader := r.Header.Get("Range")
			if rangeHeader != "" {
				// Parse range header and serve partial content
				http.ServeContent(w, r, trackPath, stat.ModTime(), file)
			} else {
				// Serve full file
				io.Copy(w, file)
			}

		case "artwork":
			// Serve artwork image
			artworkPath, err := manager.GetArtworkPath(trackID)
			if err != nil {
				http.Error(w, "Artwork not found", http.StatusNotFound)
				return
			}

			w.Header().Set("Content-Type", "image/jpeg")
			w.Header().Set("Cache-Control", "public, max-age=86400")
			http.ServeFile(w, r, artworkPath)

		default:
			http.Error(w, "Invalid resource type", http.StatusBadRequest)
		}
	} else {
		// Return track metadata
		track, err := manager.GetTrack(trackID)
		if err != nil {
			http.Error(w, "Track not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(track)
	}
}

func handleDeleteTrack(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract track ID from URL
	trackID := strings.TrimPrefix(r.URL.Path, "/api/music/track/")
	if trackID == "" {
		http.Error(w, "Track ID required", http.StatusBadRequest)
		return
	}

	// Check for delete all
	if trackID == "all" {
		handleDeleteAllTracks(w, r)
		return
	}

	manager := music.GetManager()
	if err := manager.DeleteTrack(trackID); err != nil {
		if err == music.ErrNotFound {
			http.Error(w, "Track not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to delete track", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"message": "Track deleted successfully",
	})
}

func handleDeleteAllTracks(w http.ResponseWriter, r *http.Request) {
	manager := music.GetManager()
	if err := manager.DeleteAllTracks(); err != nil {
		logger.Error("Failed to delete all tracks", zap.Error(err))
		http.Error(w, "Failed to delete all tracks", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"message": "All tracks deleted successfully",
	})
}

func handleGetPlaylists(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	manager := music.GetManager()
	playlists, err := manager.GetAllPlaylists()
	if err != nil {
		logger.Error("Failed to get playlists", zap.Error(err))
		http.Error(w, "Failed to get playlists", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"playlists": playlists,
		"count":     len(playlists),
	})
}

func handleCreatePlaylist(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		TrackIDs    []string `json:"track_ids"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Playlist name is required", http.StatusBadRequest)
		return
	}

	manager := music.GetManager()
	playlist, err := manager.CreatePlaylist(req.Name, req.Description)
	if err != nil {
		logger.Error("Failed to create playlist", zap.Error(err))
		http.Error(w, "Failed to create playlist", http.StatusInternalServerError)
		return
	}

	// Add tracks if provided
	for i, trackID := range req.TrackIDs {
		if err := manager.AddTrackToPlaylist(playlist.ID, trackID, i+1); err != nil {
			logger.Warn("Failed to add track to playlist",
				zap.String("playlist_id", playlist.ID),
				zap.String("track_id", trackID),
				zap.Error(err))
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(playlist)
}

func handleGetPlaylist(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract playlist ID or name from URL
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/music/playlist/"), "/")
	if len(pathParts) < 1 || pathParts[0] == "" {
		http.Error(w, "Playlist ID or name required", http.StatusBadRequest)
		return
	}

	identifier := pathParts[0]
	manager := music.GetManager()

	// Try to get by ID first, then by name
	playlist, err := manager.GetPlaylist(identifier)
	if err != nil {
		// Try by name
		playlist, err = manager.GetPlaylistByName(identifier)
		if err != nil {
			http.Error(w, "Playlist not found", http.StatusNotFound)
			return
		}
	}

	// Check if requesting tracks
	if len(pathParts) >= 2 && pathParts[1] == "tracks" {
		tracks, err := manager.GetPlaylistTracks(playlist.ID)
		if err != nil {
			logger.Error("Failed to get playlist tracks", zap.Error(err))
			http.Error(w, "Failed to get playlist tracks", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"playlist": playlist,
			"tracks":   tracks,
		})
	} else {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(playlist)
	}
}

func handleUpdatePlaylist(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract playlist ID from URL
	playlistID := strings.TrimPrefix(r.URL.Path, "/api/music/playlist/")
	if playlistID == "" {
		http.Error(w, "Playlist ID required", http.StatusBadRequest)
		return
	}

	var req struct {
		Action   string `json:"action"`
		TrackID  string `json:"track_id"`
		Position int    `json:"position"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	manager := music.GetManager()

	switch req.Action {
	case "add_track":
		if req.TrackID == "" {
			http.Error(w, "Track ID required", http.StatusBadRequest)
			return
		}
		if err := manager.AddTrackToPlaylist(playlistID, req.TrackID, req.Position); err != nil {
			logger.Error("Failed to add track to playlist", zap.Error(err))
			http.Error(w, "Failed to add track to playlist", http.StatusInternalServerError)
			return
		}

	case "remove_track":
		if req.TrackID == "" {
			http.Error(w, "Track ID required", http.StatusBadRequest)
			return
		}
		if err := manager.RemoveTrackFromPlaylist(playlistID, req.TrackID); err != nil {
			logger.Error("Failed to remove track from playlist", zap.Error(err))
			http.Error(w, "Failed to remove track from playlist", http.StatusInternalServerError)
			return
		}

	case "reorder_track":
		if req.TrackID == "" || req.Position <= 0 {
			http.Error(w, "Track ID and position required", http.StatusBadRequest)
			return
		}
		if err := manager.UpdatePlaylistTrackOrder(playlistID, req.TrackID, req.Position); err != nil {
			logger.Error("Failed to reorder track", zap.Error(err))
			http.Error(w, "Failed to reorder track", http.StatusInternalServerError)
			return
		}

	default:
		http.Error(w, "Invalid action", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"message": fmt.Sprintf("Playlist updated successfully (action: %s)", req.Action),
	})
}

func handleDeletePlaylist(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract playlist ID from URL
	playlistID := strings.TrimPrefix(r.URL.Path, "/api/music/playlist/")
	if playlistID == "" {
		http.Error(w, "Playlist ID required", http.StatusBadRequest)
		return
	}

	manager := music.GetManager()
	if err := manager.DeletePlaylist(playlistID); err != nil {
		logger.Error("Failed to delete playlist", zap.Error(err))
		http.Error(w, "Failed to delete playlist", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"message": "Playlist deleted successfully",
	})
}

func RegisterMusicRoutes(mux *http.ServeMux) {
	// Track endpoints
	mux.HandleFunc("/api/music/upload", corsMiddleware(handleMusicUpload))
	mux.HandleFunc("/api/music/tracks", corsMiddleware(handleGetTracks))
	mux.HandleFunc("/api/music/track/", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleGetTrack(w, r)
		case http.MethodDelete:
			handleDeleteTrack(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// Playlist endpoints
	mux.HandleFunc("/api/music/playlists", corsMiddleware(handleGetPlaylists))
	mux.HandleFunc("/api/music/playlist", corsMiddleware(handleCreatePlaylist))
	mux.HandleFunc("/api/music/playlist/", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleGetPlaylist(w, r)
		case http.MethodPut:
			handleUpdatePlaylist(w, r)
		case http.MethodDelete:
			handleDeletePlaylist(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))
}