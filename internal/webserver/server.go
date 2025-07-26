package webserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/nantokaworks/twitch-fax/internal/faxmanager"
	"github.com/nantokaworks/twitch-fax/internal/shared/logger"
	"github.com/nantokaworks/twitch-fax/internal/status"
	"go.uber.org/zap"
)

type SSEServer struct {
	clients map[chan string]bool
	mu      sync.RWMutex
}

var sseServer = &SSEServer{
	clients: make(map[chan string]bool),
}

// StartWebServer starts the HTTP server
func StartWebServer(port int) {
	// Serve static files from web/dist
	fs := http.FileServer(http.Dir("./web/dist"))
	http.Handle("/", fs)

	// SSE endpoint
	http.HandleFunc("/events", handleSSE)

	// Fax image endpoint
	http.HandleFunc("/fax/", handleFaxImage)

	// Status endpoint
	http.HandleFunc("/status", handleStatus)

	addr := fmt.Sprintf(":%d", port)
	logger.Info("Starting web server", zap.String("address", addr))
	
	go func() {
		if err := http.ListenAndServe(addr, nil); err != nil {
			logger.Fatal("Failed to start web server", zap.Error(err))
		}
	}()
}

// handleSSE handles Server-Sent Events connections
func handleSSE(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create client channel
	clientChan := make(chan string)
	
	// Register client
	sseServer.mu.Lock()
	sseServer.clients[clientChan] = true
	sseServer.mu.Unlock()

	// Remove client on disconnect
	defer func() {
		sseServer.mu.Lock()
		delete(sseServer.clients, clientChan)
		close(clientChan)
		sseServer.mu.Unlock()
	}()

	logger.Info("SSE client connected", zap.String("remote", r.RemoteAddr))

	// Send initial connection message
	fmt.Fprintf(w, "data: {\"type\":\"connected\"}\n\n")
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	// Create heartbeat ticker
	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	// Send messages to client
	for {
		select {
		case msg := <-clientChan:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		case <-heartbeat.C:
			// Send heartbeat
			fmt.Fprintf(w, ": heartbeat\n\n")
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		case <-r.Context().Done():
			logger.Info("SSE client disconnected", zap.String("remote", r.RemoteAddr))
			return
		}
	}
}

// handleFaxImage serves fax images
func handleFaxImage(w http.ResponseWriter, r *http.Request) {
	// Parse URL: /fax/{id}/{type}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/fax/"), "/")
	if len(parts) != 2 {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	id := parts[0]
	imageType := parts[1]

	// Get image path from fax manager
	imagePath, err := faxmanager.GetImagePath(id, imageType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Check if file exists
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		http.Error(w, "Image not found", http.StatusNotFound)
		return
	}

	// Set content type
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=600") // Cache for 10 minutes
	
	// Serve the file
	http.ServeFile(w, r, imagePath)
}

// BroadcastFax sends a fax notification to all connected SSE clients
func BroadcastFax(fax *faxmanager.Fax) {
	msg := map[string]interface{}{
		"type":      "fax",
		"id":        fax.ID,
		"timestamp": fax.Timestamp.Format("2006-01-02T15:04:05Z"),
		"userName":  fax.UserName,
	}

	jsonData, err := json.Marshal(msg)
	if err != nil {
		logger.Error("Failed to marshal fax message", zap.Error(err))
		return
	}

	sseServer.mu.RLock()
	defer sseServer.mu.RUnlock()

	for client := range sseServer.clients {
		select {
		case client <- string(jsonData):
		default:
			// Client channel is full, skip
		}
	}

	logger.Info("Broadcasted fax to SSE clients", 
		zap.String("id", fax.ID),
		zap.Int("clients", len(sseServer.clients)))
}

// handleStatus returns the current system status
func handleStatus(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	statusData := map[string]interface{}{
		"printerConnected": status.IsPrinterConnected(),
		"timestamp":        time.Now().Format("2006-01-02T15:04:05Z"),
	}

	jsonData, err := json.Marshal(statusData)
	if err != nil {
		http.Error(w, "Failed to marshal status", http.StatusInternalServerError)
		return
	}

	w.Write(jsonData)
}