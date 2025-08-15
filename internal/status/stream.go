package status

import (
	"sync"
	"time"
)

// StreamStatus represents the current stream status
type StreamStatus struct {
	IsLive      bool       `json:"is_live"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	ViewerCount int        `json:"viewer_count"`
	LastChecked time.Time  `json:"last_checked"`
}

var (
	streamMu     sync.RWMutex
	streamStatus StreamStatus
	// コールバック関数のリスト
	statusChangeCallbacks []func(StreamStatus)
	callbackMu           sync.RWMutex
)

// SetStreamOnline sets the stream status to online
func SetStreamOnline(startedAt time.Time, viewerCount int) {
	streamMu.Lock()
	previousStatus := streamStatus.IsLive
	streamStatus.IsLive = true
	streamStatus.StartedAt = &startedAt
	streamStatus.ViewerCount = viewerCount
	streamStatus.LastChecked = time.Now()
	currentStatus := streamStatus
	streamMu.Unlock()

	// 状態が変更された場合はコールバックを実行
	if !previousStatus {
		notifyCallbacks(currentStatus)
	}
}

// SetStreamOffline sets the stream status to offline
func SetStreamOffline() {
	streamMu.Lock()
	previousStatus := streamStatus.IsLive
	streamStatus.IsLive = false
	streamStatus.StartedAt = nil
	streamStatus.ViewerCount = 0
	streamStatus.LastChecked = time.Now()
	currentStatus := streamStatus
	streamMu.Unlock()

	// 状態が変更された場合はコールバックを実行
	if previousStatus {
		notifyCallbacks(currentStatus)
	}
}

// UpdateViewerCount updates the viewer count
func UpdateViewerCount(count int) {
	streamMu.Lock()
	streamStatus.ViewerCount = count
	streamStatus.LastChecked = time.Now()
	streamMu.Unlock()
}

// GetStreamStatus returns the current stream status
func GetStreamStatus() StreamStatus {
	streamMu.RLock()
	defer streamMu.RUnlock()
	return streamStatus
}

// IsStreamLive returns whether the stream is currently live
func IsStreamLive() bool {
	streamMu.RLock()
	defer streamMu.RUnlock()
	return streamStatus.IsLive
}

// GetStreamStartTime returns the stream start time if live
func GetStreamStartTime() *time.Time {
	streamMu.RLock()
	defer streamMu.RUnlock()
	if streamStatus.StartedAt != nil {
		t := *streamStatus.StartedAt
		return &t
	}
	return nil
}

// GetStreamDuration returns the duration of the current stream
func GetStreamDuration() time.Duration {
	streamMu.RLock()
	defer streamMu.RUnlock()
	if streamStatus.IsLive && streamStatus.StartedAt != nil {
		return time.Since(*streamStatus.StartedAt)
	}
	return 0
}

// RegisterStatusChangeCallback registers a callback function to be called when stream status changes
func RegisterStatusChangeCallback(callback func(StreamStatus)) {
	callbackMu.Lock()
	defer callbackMu.Unlock()
	statusChangeCallbacks = append(statusChangeCallbacks, callback)
}

// notifyCallbacks notifies all registered callbacks of a status change
func notifyCallbacks(status StreamStatus) {
	callbackMu.RLock()
	callbacks := make([]func(StreamStatus), len(statusChangeCallbacks))
	copy(callbacks, statusChangeCallbacks)
	callbackMu.RUnlock()

	// コールバックを非同期で実行
	for _, callback := range callbacks {
		go callback(status)
	}
}

// UpdateStreamStatus updates the stream status from API data
func UpdateStreamStatus(isLive bool, startedAt *time.Time, viewerCount int) {
	streamMu.Lock()
	previousStatus := streamStatus.IsLive
	streamStatus.IsLive = isLive
	streamStatus.StartedAt = startedAt
	streamStatus.ViewerCount = viewerCount
	streamStatus.LastChecked = time.Now()
	currentStatus := streamStatus
	streamMu.Unlock()

	// 状態が変更された場合はコールバックを実行
	if previousStatus != isLive {
		notifyCallbacks(currentStatus)
	}
}