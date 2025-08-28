package music

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nantokaworks/twitch-overlay/internal/localdb"
	"github.com/nantokaworks/twitch-overlay/internal/shared/logger"
	"github.com/nantokaworks/twitch-overlay/internal/shared/paths"
	"go.uber.org/zap"
)

var (
	ErrInvalidFormat = errors.New("invalid audio format")
	ErrFileTooLarge  = errors.New("file too large")
	ErrNotFound      = errors.New("track not found")
	MaxFileSize      = int64(50 * 1024 * 1024) // 50MB
)

type Track struct {
	ID         string    `json:"id"`
	Filename   string    `json:"filename"`
	Title      string    `json:"title"`
	Artist     string    `json:"artist"`
	Album      string    `json:"album"`
	Duration   int       `json:"duration"`
	HasArtwork bool      `json:"has_artwork"`
	CreatedAt  time.Time `json:"created_at"`
}

type Manager struct {
	mu sync.RWMutex
}

var manager = &Manager{}

func GetManager() *Manager {
	return manager
}

func getMusicDir() string {
	return filepath.Join(paths.GetDataDir(), "music")
}

func getTracksDir() string {
	return filepath.Join(getMusicDir(), "tracks")
}

func getArtworkDir() string {
	return filepath.Join(getMusicDir(), "artwork")
}

func ensureDirs() error {
	dirs := []string{
		getTracksDir(),
		getArtworkDir(),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}

func (m *Manager) SaveTrack(filename string, reader io.Reader, size int64) (*Track, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if size > MaxFileSize {
		return nil, ErrFileTooLarge
	}

	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".mp3" && ext != ".wav" && ext != ".m4a" && ext != ".ogg" {
		return nil, ErrInvalidFormat
	}

	if err := ensureDirs(); err != nil {
		return nil, err
	}

	// Generate track ID
	hasher := sha256.New()
	hasher.Write([]byte(filename))
	hasher.Write([]byte(time.Now().String()))
	trackID := hex.EncodeToString(hasher.Sum(nil))[:16]

	// Save file
	trackPath := filepath.Join(getTracksDir(), trackID+ext)
	file, err := os.Create(trackPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create track file: %w", err)
	}
	defer file.Close()

	// Copy and save file
	_, err = io.Copy(file, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to write track file: %w", err)
	}
	file.Close()
	
	// Extract metadata after file is written
	metadata, err := ExtractMetadata(trackPath)
	if err != nil {
		logger.Warn("Failed to extract metadata", zap.Error(err))
		// Use filename without extension as fallback title
		baseFilename := strings.TrimSuffix(filename, filepath.Ext(filename))
		metadata = &Metadata{
			Title:  baseFilename,
			Artist: "Unknown Artist",
		}
	}

	// Save artwork if exists
	if metadata.ArtworkData != nil {
		artworkPath := filepath.Join(getArtworkDir(), trackID+".jpg")
		if err := os.WriteFile(artworkPath, metadata.ArtworkData, 0644); err != nil {
			logger.Warn("Failed to save artwork", zap.Error(err))
		}
	}

	// Create track record
	track := &Track{
		ID:         trackID,
		Filename:   filename,
		Title:      metadata.Title,
		Artist:     metadata.Artist,
		Album:      metadata.Album,
		Duration:   metadata.Duration,
		HasArtwork: metadata.ArtworkData != nil,
		CreatedAt:  time.Now(),
	}

	// Save to database
	if err := m.saveTrackToDB(track); err != nil {
		os.Remove(trackPath)
		return nil, fmt.Errorf("failed to save track to database: %w", err)
	}

	logger.Info("Track saved successfully",
		zap.String("id", trackID),
		zap.String("title", track.Title),
		zap.String("artist", track.Artist))

	return track, nil
}

func (m *Manager) GetTrack(trackID string) (*Track, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	db := localdb.GetDB()
	if db == nil {
		return nil, errors.New("database not initialized")
	}

	var track Track
	query := `SELECT id, filename, title, artist, album, duration, has_artwork, created_at 
			  FROM tracks WHERE id = ?`
	
	var createdAt string
	err := db.QueryRow(query, trackID).Scan(
		&track.ID,
		&track.Filename,
		&track.Title,
		&track.Artist,
		&track.Album,
		&track.Duration,
		&track.HasArtwork,
		&createdAt,
	)
	
	if err != nil {
		return nil, ErrNotFound
	}

	track.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &track, nil
}

func (m *Manager) GetAllTracks() ([]*Track, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	db := localdb.GetDB()
	if db == nil {
		return nil, errors.New("database not initialized")
	}

	query := `SELECT id, filename, title, artist, album, duration, has_artwork, created_at 
			  FROM tracks ORDER BY created_at DESC`
	
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []*Track
	for rows.Next() {
		var track Track
		var createdAt string
		
		err := rows.Scan(
			&track.ID,
			&track.Filename,
			&track.Title,
			&track.Artist,
			&track.Album,
			&track.Duration,
			&track.HasArtwork,
			&createdAt,
		)
		
		if err != nil {
			logger.Warn("Failed to scan track", zap.Error(err))
			continue
		}
		
		track.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		tracks = append(tracks, &track)
	}

	return tracks, nil
}

func (m *Manager) DeleteTrack(trackID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get track info first
	track, err := m.GetTrack(trackID)
	if err != nil {
		return err
	}

	// Delete files
	ext := strings.ToLower(filepath.Ext(track.Filename))
	trackPath := filepath.Join(getTracksDir(), trackID+ext)
	artworkPath := filepath.Join(getArtworkDir(), trackID+".jpg")

	os.Remove(trackPath)
	os.Remove(artworkPath)

	// Delete from database
	db := localdb.GetDB()
	if db == nil {
		return errors.New("database not initialized")
	}

	_, err = db.Exec("DELETE FROM tracks WHERE id = ?", trackID)
	if err != nil {
		return fmt.Errorf("failed to delete track from database: %w", err)
	}

	logger.Info("Track deleted", zap.String("id", trackID))
	return nil
}

func (m *Manager) DeleteAllTracks() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Delete all track files
	tracksDir := getTracksDir()
	if err := os.RemoveAll(tracksDir); err != nil {
		logger.Warn("Failed to remove tracks directory", zap.Error(err))
	}
	
	// Delete all artwork files
	artworkDir := getArtworkDir()
	if err := os.RemoveAll(artworkDir); err != nil {
		logger.Warn("Failed to remove artwork directory", zap.Error(err))
	}
	
	// Recreate directories
	if err := ensureDirs(); err != nil {
		return fmt.Errorf("failed to recreate directories: %w", err)
	}

	// Delete from database
	db := localdb.GetDB()
	if db == nil {
		return errors.New("database not initialized")
	}

	// Delete all playlist associations first
	_, err := db.Exec("DELETE FROM playlist_tracks")
	if err != nil {
		logger.Warn("Failed to delete playlist tracks", zap.Error(err))
	}

	// Delete all tracks
	_, err = db.Exec("DELETE FROM tracks")
	if err != nil {
		return fmt.Errorf("failed to delete all tracks from database: %w", err)
	}

	logger.Info("All tracks deleted")
	return nil
}

func (m *Manager) GetTrackPath(trackID string) (string, error) {
	track, err := m.GetTrack(trackID)
	if err != nil {
		return "", err
	}

	ext := strings.ToLower(filepath.Ext(track.Filename))
	trackPath := filepath.Join(getTracksDir(), trackID+ext)
	
	if _, err := os.Stat(trackPath); os.IsNotExist(err) {
		return "", ErrNotFound
	}

	return trackPath, nil
}

func (m *Manager) GetArtworkPath(trackID string) (string, error) {
	track, err := m.GetTrack(trackID)
	if err != nil {
		return "", err
	}

	if !track.HasArtwork {
		return "", ErrNotFound
	}

	artworkPath := filepath.Join(getArtworkDir(), trackID+".jpg")
	if _, err := os.Stat(artworkPath); os.IsNotExist(err) {
		return "", ErrNotFound
	}

	return artworkPath, nil
}

func (m *Manager) saveTrackToDB(track *Track) error {
	db := localdb.GetDB()
	if db == nil {
		return errors.New("database not initialized")
	}

	query := `INSERT INTO tracks (id, filename, title, artist, album, duration, has_artwork, created_at)
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	
	_, err := db.Exec(query,
		track.ID,
		track.Filename,
		track.Title,
		track.Artist,
		track.Album,
		track.Duration,
		track.HasArtwork,
		track.CreatedAt.Format(time.RFC3339),
	)
	
	return err
}

func InitMusicDB() error {
	db := localdb.GetDB()
	if db == nil {
		return errors.New("database not initialized")
	}

	// Create tracks table
	tracksTable := `
	CREATE TABLE IF NOT EXISTS tracks (
		id TEXT PRIMARY KEY,
		filename TEXT NOT NULL,
		title TEXT NOT NULL,
		artist TEXT NOT NULL,
		album TEXT,
		duration INTEGER DEFAULT 0,
		has_artwork BOOLEAN DEFAULT 0,
		created_at TEXT NOT NULL
	)`

	if _, err := db.Exec(tracksTable); err != nil {
		return fmt.Errorf("failed to create tracks table: %w", err)
	}

	// Create playlists table
	playlistsTable := `
	CREATE TABLE IF NOT EXISTS playlists (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		description TEXT,
		created_at TEXT NOT NULL
	)`

	if _, err := db.Exec(playlistsTable); err != nil {
		return fmt.Errorf("failed to create playlists table: %w", err)
	}

	// Create playlist_tracks table
	playlistTracksTable := `
	CREATE TABLE IF NOT EXISTS playlist_tracks (
		playlist_id TEXT NOT NULL,
		track_id TEXT NOT NULL,
		position INTEGER NOT NULL,
		PRIMARY KEY (playlist_id, track_id),
		FOREIGN KEY (playlist_id) REFERENCES playlists(id) ON DELETE CASCADE,
		FOREIGN KEY (track_id) REFERENCES tracks(id) ON DELETE CASCADE
	)`

	if _, err := db.Exec(playlistTracksTable); err != nil {
		return fmt.Errorf("failed to create playlist_tracks table: %w", err)
	}

	logger.Info("Music database initialized")
	return nil
}