package music

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/nantokaworks/twitch-overlay/internal/localdb"
	"github.com/nantokaworks/twitch-overlay/internal/shared/logger"
	"go.uber.org/zap"
)

type Playlist struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	TrackCount  int       `json:"track_count"`
}

type PlaylistTrack struct {
	*Track
	Position int `json:"position"`
}

func (m *Manager) CreatePlaylist(name, description string) (*Playlist, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	db := localdb.GetDB()
	if db == nil {
		return nil, errors.New("database not initialized")
	}

	// Generate playlist ID
	hasher := sha256.New()
	hasher.Write([]byte(name))
	hasher.Write([]byte(time.Now().String()))
	playlistID := hex.EncodeToString(hasher.Sum(nil))[:16]

	playlist := &Playlist{
		ID:          playlistID,
		Name:        name,
		Description: description,
		CreatedAt:   time.Now(),
	}

	query := `INSERT INTO playlists (id, name, description, created_at)
			  VALUES (?, ?, ?, ?)`
	
	_, err := db.Exec(query,
		playlist.ID,
		playlist.Name,
		playlist.Description,
		playlist.CreatedAt.Format(time.RFC3339),
	)
	
	if err != nil {
		return nil, fmt.Errorf("failed to create playlist: %w", err)
	}

	logger.Info("Playlist created",
		zap.String("id", playlistID),
		zap.String("name", name))

	return playlist, nil
}

func (m *Manager) GetPlaylist(playlistID string) (*Playlist, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	db := localdb.GetDB()
	if db == nil {
		return nil, errors.New("database not initialized")
	}

	var playlist Playlist
	var createdAt string
	
	query := `SELECT id, name, description, created_at,
			  (SELECT COUNT(*) FROM playlist_tracks WHERE playlist_id = playlists.id) as track_count
			  FROM playlists WHERE id = ?`
	
	err := db.QueryRow(query, playlistID).Scan(
		&playlist.ID,
		&playlist.Name,
		&playlist.Description,
		&createdAt,
		&playlist.TrackCount,
	)
	
	if err != nil {
		return nil, fmt.Errorf("playlist not found: %w", err)
	}

	playlist.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &playlist, nil
}

func (m *Manager) GetPlaylistByName(name string) (*Playlist, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	db := localdb.GetDB()
	if db == nil {
		return nil, errors.New("database not initialized")
	}

	var playlist Playlist
	var createdAt string
	
	query := `SELECT id, name, description, created_at,
			  (SELECT COUNT(*) FROM playlist_tracks WHERE playlist_id = playlists.id) as track_count
			  FROM playlists WHERE name = ?`
	
	err := db.QueryRow(query, name).Scan(
		&playlist.ID,
		&playlist.Name,
		&playlist.Description,
		&createdAt,
		&playlist.TrackCount,
	)
	
	if err != nil {
		return nil, fmt.Errorf("playlist not found: %w", err)
	}

	playlist.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &playlist, nil
}

func (m *Manager) GetAllPlaylists() ([]*Playlist, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	db := localdb.GetDB()
	if db == nil {
		return nil, errors.New("database not initialized")
	}

	query := `SELECT id, name, description, created_at,
			  (SELECT COUNT(*) FROM playlist_tracks WHERE playlist_id = playlists.id) as track_count
			  FROM playlists ORDER BY created_at DESC`
	
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var playlists []*Playlist
	for rows.Next() {
		var playlist Playlist
		var createdAt string
		
		err := rows.Scan(
			&playlist.ID,
			&playlist.Name,
			&playlist.Description,
			&createdAt,
			&playlist.TrackCount,
		)
		
		if err != nil {
			logger.Warn("Failed to scan playlist", zap.Error(err))
			continue
		}
		
		playlist.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		playlists = append(playlists, &playlist)
	}

	return playlists, nil
}

func (m *Manager) AddTrackToPlaylist(playlistID, trackID string, position int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	db := localdb.GetDB()
	if db == nil {
		return errors.New("database not initialized")
	}

	// 位置が指定されていない場合は最後に追加
	if position <= 0 {
		var maxPos int
		err := db.QueryRow(
			"SELECT COALESCE(MAX(position), 0) FROM playlist_tracks WHERE playlist_id = ?",
			playlistID,
		).Scan(&maxPos)
		
		if err != nil {
			return fmt.Errorf("failed to get max position: %w", err)
		}
		position = maxPos + 1
	}

	query := `INSERT OR REPLACE INTO playlist_tracks (playlist_id, track_id, position)
			  VALUES (?, ?, ?)`
	
	_, err := db.Exec(query, playlistID, trackID, position)
	if err != nil {
		return fmt.Errorf("failed to add track to playlist: %w", err)
	}

	logger.Info("Track added to playlist",
		zap.String("playlist_id", playlistID),
		zap.String("track_id", trackID),
		zap.Int("position", position))

	return nil
}

func (m *Manager) RemoveTrackFromPlaylist(playlistID, trackID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	db := localdb.GetDB()
	if db == nil {
		return errors.New("database not initialized")
	}

	_, err := db.Exec(
		"DELETE FROM playlist_tracks WHERE playlist_id = ? AND track_id = ?",
		playlistID, trackID,
	)
	
	if err != nil {
		return fmt.Errorf("failed to remove track from playlist: %w", err)
	}

	// 位置を再調整
	_, err = db.Exec(`
		UPDATE playlist_tracks 
		SET position = position - 1 
		WHERE playlist_id = ? AND position > (
			SELECT position FROM playlist_tracks 
			WHERE playlist_id = ? AND track_id = ?
		)`,
		playlistID, playlistID, trackID,
	)

	logger.Info("Track removed from playlist",
		zap.String("playlist_id", playlistID),
		zap.String("track_id", trackID))

	return nil
}

func (m *Manager) GetPlaylistTracks(playlistID string) ([]*PlaylistTrack, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	db := localdb.GetDB()
	if db == nil {
		return nil, errors.New("database not initialized")
	}

	query := `SELECT t.id, t.filename, t.title, t.artist, t.album, t.duration, t.has_artwork, t.created_at, pt.position
			  FROM tracks t
			  JOIN playlist_tracks pt ON t.id = pt.track_id
			  WHERE pt.playlist_id = ?
			  ORDER BY pt.position`
	
	rows, err := db.Query(query, playlistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []*PlaylistTrack
	for rows.Next() {
		var track PlaylistTrack
		var createdAt string
		
		track.Track = &Track{}
		err := rows.Scan(
			&track.ID,
			&track.Filename,
			&track.Title,
			&track.Artist,
			&track.Album,
			&track.Duration,
			&track.HasArtwork,
			&createdAt,
			&track.Position,
		)
		
		if err != nil {
			logger.Warn("Failed to scan playlist track", zap.Error(err))
			continue
		}
		
		track.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		tracks = append(tracks, &track)
	}

	return tracks, nil
}

func (m *Manager) UpdatePlaylistTrackOrder(playlistID string, trackID string, newPosition int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	db := localdb.GetDB()
	if db == nil {
		return errors.New("database not initialized")
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 現在の位置を取得
	var currentPosition int
	err = tx.QueryRow(
		"SELECT position FROM playlist_tracks WHERE playlist_id = ? AND track_id = ?",
		playlistID, trackID,
	).Scan(&currentPosition)
	
	if err != nil {
		return fmt.Errorf("track not found in playlist: %w", err)
	}

	if currentPosition == newPosition {
		return nil // 位置変更なし
	}

	// 他のトラックの位置を調整
	if newPosition < currentPosition {
		// 上に移動
		_, err = tx.Exec(`
			UPDATE playlist_tracks 
			SET position = position + 1 
			WHERE playlist_id = ? AND position >= ? AND position < ?`,
			playlistID, newPosition, currentPosition,
		)
	} else {
		// 下に移動
		_, err = tx.Exec(`
			UPDATE playlist_tracks 
			SET position = position - 1 
			WHERE playlist_id = ? AND position > ? AND position <= ?`,
			playlistID, currentPosition, newPosition,
		)
	}
	
	if err != nil {
		return fmt.Errorf("failed to update positions: %w", err)
	}

	// 対象トラックの位置を更新
	_, err = tx.Exec(
		"UPDATE playlist_tracks SET position = ? WHERE playlist_id = ? AND track_id = ?",
		newPosition, playlistID, trackID,
	)
	
	if err != nil {
		return fmt.Errorf("failed to update track position: %w", err)
	}

	return tx.Commit()
}

func (m *Manager) DeletePlaylist(playlistID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	db := localdb.GetDB()
	if db == nil {
		return errors.New("database not initialized")
	}

	// プレイリストとその関連を削除（CASCADE削除）
	_, err := db.Exec("DELETE FROM playlists WHERE id = ?", playlistID)
	if err != nil {
		return fmt.Errorf("failed to delete playlist: %w", err)
	}

	logger.Info("Playlist deleted", zap.String("id", playlistID))
	return nil
}