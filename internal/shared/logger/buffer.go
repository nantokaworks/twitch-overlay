package logger

import (
	"encoding/json"
	"sync"
	"time"
)

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// LogBuffer holds recent log entries in memory
type LogBuffer struct {
	mu      sync.RWMutex
	entries []LogEntry
	maxSize int
}

var (
	globalBuffer *LogBuffer
	bufferOnce   sync.Once
	broadcastCallback func(LogEntry)
	callbackMu sync.RWMutex
)

// GetLogBuffer returns the global log buffer instance
func GetLogBuffer() *LogBuffer {
	bufferOnce.Do(func() {
		globalBuffer = &LogBuffer{
			entries: make([]LogEntry, 0, 1000),
			maxSize: 1000,
		}
	})
	return globalBuffer
}

// SetBroadcastCallback sets the callback function for broadcasting log entries
func SetBroadcastCallback(callback func(LogEntry)) {
	callbackMu.Lock()
	defer callbackMu.Unlock()
	broadcastCallback = callback
}

// Add adds a new log entry to the buffer
func (lb *LogBuffer) Add(entry LogEntry) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.entries = append(lb.entries, entry)
	
	// Remove old entries if buffer exceeds max size
	if len(lb.entries) > lb.maxSize {
		lb.entries = lb.entries[len(lb.entries)-lb.maxSize:]
	}
	
	// Broadcast to WebSocket clients if callback is set
	callbackMu.RLock()
	callback := broadcastCallback
	callbackMu.RUnlock()
	
	if callback != nil {
		// Call the callback in a goroutine to avoid blocking
		go callback(entry)
	}
}

// GetAll returns all log entries
func (lb *LogBuffer) GetAll() []LogEntry {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	
	// Return a copy to avoid race conditions
	result := make([]LogEntry, len(lb.entries))
	copy(result, lb.entries)
	return result
}

// GetRecent returns the most recent n log entries
func (lb *LogBuffer) GetRecent(n int) []LogEntry {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	
	if n > len(lb.entries) {
		n = len(lb.entries)
	}
	
	result := make([]LogEntry, n)
	copy(result, lb.entries[len(lb.entries)-n:])
	return result
}

// Clear clears all log entries
func (lb *LogBuffer) Clear() {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	
	lb.entries = lb.entries[:0]
}

// ToJSON converts log entries to JSON
func (lb *LogBuffer) ToJSON() ([]byte, error) {
	entries := lb.GetAll()
	return json.Marshal(entries)
}

// ToText converts log entries to plain text
func (lb *LogBuffer) ToText() string {
	entries := lb.GetAll()
	var result string
	
	for _, entry := range entries {
		result += entry.Timestamp.Format("2006-01-02 15:04:05") + " "
		result += "[" + entry.Level + "] "
		result += entry.Message
		
		if len(entry.Fields) > 0 {
			result += " "
			fieldsJSON, _ := json.Marshal(entry.Fields)
			result += string(fieldsJSON)
		}
		result += "\n"
	}
	
	return result
}