package webserver

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/nantokaworks/twitch-fax/internal/faxmanager"
	"github.com/nantokaworks/twitch-fax/internal/output"
	"github.com/nantokaworks/twitch-fax/internal/shared/logger"
	"github.com/nantokaworks/twitch-fax/internal/status"
	"github.com/twitch/twitch-api-go"
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

	// Debug endpoints
	http.HandleFunc("/debug/fax", handleDebugFax) // Legacy endpoint
	http.HandleFunc("/debug/channel-points", handleDebugChannelPoints)

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

// DebugFaxRequest represents a debug fax request
type DebugFaxRequest struct {
	Username string `json:"username"`
	DisplayName string `json:"displayName"`
	Message string `json:"message"`
	ImageURL string `json:"imageUrl,omitempty"`
}

// handleDebugFax handles debug fax submissions
func handleDebugFax(w http.ResponseWriter, r *http.Request) {
	// Note: This endpoint is kept for backwards compatibility
	// but the frontend now uses local mode by default
	// Only allow in debug mode
	if os.Getenv("DEBUG_MODE") != "true" {
		http.Error(w, "Debug mode not enabled", http.StatusForbidden)
		return
	}

	// Only accept POST
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req DebugFaxRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Username == "" || req.Message == "" {
		http.Error(w, "Username and message are required", http.StatusBadRequest)
		return
	}

	// If displayName is empty, use username
	if req.DisplayName == "" {
		req.DisplayName = req.Username
	}

	// Create message fragments
	fragments := []twitch.ChatMessageFragment{
		{
			Type: "text",
			Text: req.Message,
		},
	}

	// Process the fax
	logger.Info("Processing debug fax", 
		zap.String("username", req.Username),
		zap.String("message", req.Message),
		zap.String("imageUrl", req.ImageURL))

	// Call PrintOut directly (same as custom reward handling)
	err = output.PrintOut(req.Username, fragments, time.Now())
	if err != nil {
		logger.Error("Failed to process debug fax", zap.Error(err))
		http.Error(w, "Failed to process fax", http.StatusInternalServerError)
		return
	}

	// Return success
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"message": "Debug fax queued successfully",
	})
}

// DebugChannelPointsRequest represents a debug channel points request
type DebugChannelPointsRequest struct {
	Username string `json:"username"`
	DisplayName string `json:"displayName"`
	RewardTitle string `json:"rewardTitle"`
	UserInput string `json:"userInput"`
}

// handleDebugChannelPoints handles debug channel points redemption
func handleDebugChannelPoints(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	
	// Handle preflight requests
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Only accept POST
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req DebugChannelPointsRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Username == "" || req.RewardTitle == "" || req.UserInput == "" {
		http.Error(w, "Username, rewardTitle, and userInput are required", http.StatusBadRequest)
		return
	}

	// If displayName is empty, use username
	if req.DisplayName == "" {
		req.DisplayName = req.Username
	}

	// Create message fragments - exactly like HandleChannelPointsCustomRedemptionAdd
	fragments := []twitch.ChatMessageFragment{
		{
			Type: "text",
			Text: fmt.Sprintf("🎉チャネポ %s %s", req.RewardTitle, req.UserInput),
		},
	}

	// Process the fax - exactly like HandleChannelPointsCustomRedemptionAdd
	logger.Info("Processing debug channel points redemption", 
		zap.String("username", req.Username),
		zap.String("rewardTitle", req.RewardTitle),
		zap.String("userInput", req.UserInput))

	// Call PrintOut directly (same as channel points handling)
	err = output.PrintOut(req.Username, fragments, time.Now())
	if err != nil {
		logger.Error("Failed to process debug channel points", zap.Error(err))
		http.Error(w, "Failed to process channel points redemption", http.StatusInternalServerError)
		return
	}

	// Return success
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"message": "Debug channel points redemption processed successfully",
	})
}