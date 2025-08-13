package webserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	twitch "github.com/joeyak/go-twitch-eventsub/v3"
	"github.com/nantokaworks/twitch-overlay/internal/broadcast"
	"github.com/nantokaworks/twitch-overlay/internal/faxmanager"
	"github.com/nantokaworks/twitch-overlay/internal/fontmanager"
	"github.com/nantokaworks/twitch-overlay/internal/output"
	"github.com/nantokaworks/twitch-overlay/internal/shared/logger"
	"github.com/nantokaworks/twitch-overlay/internal/status"
	"github.com/nantokaworks/twitch-overlay/internal/twitcheventsub"
	"github.com/nantokaworks/twitch-overlay/internal/twitchtoken"
	"go.uber.org/zap"
)

type SSEServer struct {
	clients map[chan string]bool
	mu      sync.RWMutex
}

var (
	sseServer = &SSEServer{
		clients: make(map[chan string]bool),
	}
	httpServer *http.Server
)

// corsMiddleware adds CORS headers to HTTP handlers
func corsMiddleware(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		handler(w, r)
	}
}

// StartWebServer starts the HTTP server
func StartWebServer(port int) {
	// Register SSE server as the global broadcaster
	broadcast.SetBroadcaster(sseServer)

	// Initialize font manager
	if err := fontmanager.Initialize(); err != nil {
		logger.Error("Failed to initialize font manager", zap.Error(err))
	}

	// Serve static files - try multiple paths
	var staticDir string
	possiblePaths := []string{
		"./public",      // Production: same directory as executable
		"./dist/public", // Development: built files
		"./web/dist",    // Fallback: frontend build directory
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			staticDir = path
			logger.Info("Using static files directory", zap.String("path", staticDir))
			break
		}
	}

	if staticDir == "" {
		logger.Warn("No static files directory found, using default")
		staticDir = "./web/dist"
	}

	// Create a new ServeMux for better routing control
	mux := http.NewServeMux()

	// Settings API endpoints - 最初に登録してAPIが優先されるようにする
	mux.HandleFunc("/api/settings/v2", corsMiddleware(handleSettingsV2))
	mux.HandleFunc("/api/settings/status", corsMiddleware(handleSettingsStatus))
	mux.HandleFunc("/api/settings/bulk", corsMiddleware(handleBulkSettings))
	mux.HandleFunc("/api/settings/font/preview", corsMiddleware(handleFontPreview))
	mux.HandleFunc("/api/settings/font", handleFontUpload) // handleFontUploadは独自のCORS処理を持つ
	mux.HandleFunc("/api/settings/auth/status", corsMiddleware(handleAuthStatus))
	mux.HandleFunc("/api/settings", corsMiddleware(handleSettings))

	// Printer API endpoints
	mux.HandleFunc("/api/printer/scan", corsMiddleware(handlePrinterScan))
	mux.HandleFunc("/api/printer/test", corsMiddleware(handlePrinterTest))
	mux.HandleFunc("/api/printer/status", corsMiddleware(handlePrinterStatus))
	mux.HandleFunc("/api/printer/reconnect", corsMiddleware(handlePrinterReconnect))

	// Server management API endpoints
	mux.HandleFunc("/api/server/restart", corsMiddleware(handleServerRestart))
	mux.HandleFunc("/api/server/status", corsMiddleware(handleServerStatus))

	// Logs API endpoints
	mux.HandleFunc("/api/logs", corsMiddleware(handleLogs))
	mux.HandleFunc("/api/logs/download", corsMiddleware(handleLogsDownload))
	mux.HandleFunc("/api/logs/stream", handleLogsStream) // WebSocketは独自のUpgrade処理
	mux.HandleFunc("/api/logs/clear", corsMiddleware(handleLogsClear))

	// SSE endpoint
	mux.HandleFunc("/events", handleSSE)

	// Fax image endpoint
	mux.HandleFunc("/fax/", handleFaxImage)

	// Status endpoint
	mux.HandleFunc("/status", handleStatus)

	// Debug endpoints
	mux.HandleFunc("/debug/fax", handleDebugFax)
	mux.HandleFunc("/debug/channel-points", handleDebugChannelPoints)
	mux.HandleFunc("/debug/clock", handleDebugClock)
	mux.HandleFunc("/debug/follow", handleDebugFollow)
	mux.HandleFunc("/debug/cheer", handleDebugCheer)
	mux.HandleFunc("/debug/subscribe", handleDebugSubscribe)
	mux.HandleFunc("/debug/gift-sub", handleDebugGiftSub)
	mux.HandleFunc("/debug/resub", handleDebugResub)
	mux.HandleFunc("/debug/raid", handleDebugRaid)
	mux.HandleFunc("/debug/shoutout", handleDebugShoutout)
	mux.HandleFunc("/debug/stream-online", handleDebugStreamOnline)
	mux.HandleFunc("/debug/stream-offline", handleDebugStreamOffline)

	// OAuth endpoints
	mux.HandleFunc("/auth", handleAuth)
	mux.HandleFunc("/callback", handleCallback)

	// Twitch API endpoints
	mux.HandleFunc("/api/twitch/verify", corsMiddleware(handleTwitchVerify))

	// Create a custom file server that handles SPA routing
	fs := http.FileServer(http.Dir(staticDir))

	// Handle all other routes (SPA fallback) - 最後に登録
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the file
		filePath := filepath.Join(staticDir, r.URL.Path)
		if _, err := os.Stat(filePath); err == nil && !strings.HasSuffix(r.URL.Path, "/") {
			// File exists, serve it
			fs.ServeHTTP(w, r)
			return
		}

		// For all other routes, serve index.html (SPA fallback)
		http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
	})

	addr := fmt.Sprintf(":%d", port)

	// 起動メッセージを表示（logger出力の前に）
	fmt.Println("")
	fmt.Println("====================================================")
	fmt.Printf("🚀 Webサーバーが起動しました\n")
	fmt.Printf("📡 アクセスURL:\n")
	fmt.Printf("   モノクロ表示: http://localhost:%d\n", port)
	fmt.Printf("   カラー表示:   http://localhost:%d/color\n", port)
	fmt.Printf("\n")
	fmt.Printf("⚙️  設定画面:     http://localhost:%d/settings\n", port)
	fmt.Printf("\n")
	fmt.Printf("🐛 デバッグモード:\n")
	fmt.Printf("   モノクロ表示: http://localhost:%d?debug=true\n", port)
	fmt.Printf("   カラー表示:   http://localhost:%d/color?debug=true\n", port)
	fmt.Printf("\n")
	fmt.Printf("🔧 環境変数 SERVER_PORT で変更可能\n")
	fmt.Println("====================================================")
	fmt.Println("")

	logger.Info("Starting web server", zap.String("address", addr))

	// Create HTTP server instance
	httpServer = &http.Server{
		Addr:    addr,
		Handler: mux, // Use our custom ServeMux
	}

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start web server", zap.Error(err))
		}
	}()
}

// Shutdown gracefully shuts down the web server
func Shutdown() {
	if httpServer == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error("Failed to shutdown web server gracefully", zap.Error(err))
	} else {
		logger.Info("Web server shutdown complete")
	}
}

// handleSSE handles Server-Sent Events connections
func handleSSE(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers first
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle OPTIONS request
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

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
func (s *SSEServer) BroadcastFax(fax *faxmanager.Fax) {
	msg := map[string]interface{}{
		"type":        "fax",
		"id":          fax.ID,
		"timestamp":   fax.Timestamp.Unix() * 1000, // JavaScriptのミリ秒に変換
		"username":    fax.UserName,
		"displayName": fax.UserName, // 表示名も同じにする
		"message":     fax.Message,
		"imageUrl":    fmt.Sprintf("/fax/%s/color", fax.ID), // カラー画像のURLを生成
	}

	jsonData, err := json.Marshal(msg)
	if err != nil {
		logger.Error("Failed to marshal fax message", zap.Error(err))
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for client := range s.clients {
		select {
		case client <- string(jsonData):
		default:
			// Client channel is full, skip
		}
	}

	logger.Info("Broadcasted fax to SSE clients",
		zap.String("id", fax.ID),
		zap.Int("clients", len(s.clients)))
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
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Message     string `json:"message"`
	ImageURL    string `json:"imageUrl,omitempty"`
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
		"status":  "ok",
		"message": "Debug fax queued successfully",
	})
}

// DebugChannelPointsRequest represents a debug channel points request
type DebugChannelPointsRequest struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	RewardTitle string `json:"rewardTitle"`
	UserInput   string `json:"userInput"`
}

// handleDebugChannelPoints handles debug channel points redemption
func handleDebugChannelPoints(w http.ResponseWriter, r *http.Request) {
	// CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle OPTIONS
	if r.Method == http.MethodOptions {
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
	if req.Username == "" || req.UserInput == "" {
		http.Error(w, "Username and userInput are required", http.StatusBadRequest)
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
			Text: req.UserInput,
		},
	}

	// Process the fax - exactly like HandleChannelPointsCustomRedemptionAdd
	logger.Info("Processing debug channel points redemption",
		zap.String("username", req.Username),
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
		"status":  "ok",
		"message": "Debug channel points redemption processed successfully",
	})
}

// DebugClockRequest represents a debug clock print request
type DebugClockRequest struct {
	WithStats        bool `json:"withStats"`
	EmptyLeaderboard bool `json:"emptyLeaderboard"`
}

// handleDebugClock handles debug clock print requests
func handleDebugClock(w http.ResponseWriter, r *http.Request) {
	// CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle OPTIONS
	if r.Method == http.MethodOptions {
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

	var req DebugClockRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Get current time
	now := time.Now()
	timeStr := now.Format("15:04")

	logger.Info("Processing debug clock print",
		zap.String("time", timeStr),
		zap.Bool("withStats", req.WithStats),
		zap.Bool("emptyLeaderboard", req.EmptyLeaderboard))

	// Call PrintClock with options based on request
	err = output.PrintClockWithOptions(timeStr, req.EmptyLeaderboard)
	if err != nil {
		logger.Error("Failed to print debug clock", 
			zap.Error(err),
			zap.String("time", timeStr),
			zap.Bool("emptyLeaderboard", req.EmptyLeaderboard))
		// Return more detailed error message
		errorMsg := fmt.Sprintf("Failed to print clock: %v", err)
		http.Error(w, errorMsg, http.StatusInternalServerError)
		return
	}

	// Return success
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": fmt.Sprintf("Clock printed at %s with leaderboard stats", timeStr),
		"time":    timeStr,
	})
}

// handleDebugFollow handles debug follow event
func handleDebugFollow(w http.ResponseWriter, r *http.Request) {
	// CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username string `json:"username"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Username == "" {
		req.Username = "DebugUser"
	}

	// Call the same handler as real follow events
	twitcheventsub.HandleChannelFollow(twitch.EventChannelFollow{
		User: twitch.User{
			UserID:    "debug-" + req.Username,
			UserLogin: strings.ToLower(req.Username),
			UserName:  req.Username,
		},
		FollowedAt: time.Now(),
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleDebugCheer handles debug cheer event
func handleDebugCheer(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username string `json:"username"`
		Bits     int    `json:"bits"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Username == "" {
		req.Username = "DebugUser"
	}
	if req.Bits == 0 {
		req.Bits = 100
	}

	twitcheventsub.HandleChannelCheer(twitch.EventChannelCheer{
		User: twitch.User{
			UserID:    "debug-" + req.Username,
			UserLogin: strings.ToLower(req.Username),
			UserName:  req.Username,
		},
		Bits: req.Bits,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleDebugSubscribe handles debug subscribe event
func handleDebugSubscribe(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username string `json:"username"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Username == "" {
		req.Username = "DebugUser"
	}

	twitcheventsub.HandleChannelSubscribe(twitch.EventChannelSubscribe{
		User: twitch.User{
			UserID:    "debug-" + req.Username,
			UserLogin: strings.ToLower(req.Username),
			UserName:  req.Username,
		},
		Tier:   "1000",
		IsGift: false,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleDebugGiftSub handles debug gift sub event
func handleDebugGiftSub(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username    string `json:"username"`
		IsAnonymous bool   `json:"isAnonymous"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Username == "" {
		req.Username = "DebugUser"
	}

	twitcheventsub.HandleChannelSubscriptionGift(twitch.EventChannelSubscriptionGift{
		User: twitch.User{
			UserID:    "debug-" + req.Username,
			UserLogin: strings.ToLower(req.Username),
			UserName:  req.Username,
		},
		Total:       1,
		Tier:        "1000",
		IsAnonymous: req.IsAnonymous,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleDebugResub handles debug resub event
func handleDebugResub(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username         string `json:"username"`
		CumulativeMonths int    `json:"cumulativeMonths"`
		Message          string `json:"message"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Username == "" {
		req.Username = "DebugUser"
	}
	if req.CumulativeMonths == 0 {
		req.CumulativeMonths = 3
	}
	if req.Message == "" {
		req.Message = "デバッグ再サブスクメッセージ"
	}

	twitcheventsub.HandleChannelSubscriptionMessage(twitch.EventChannelSubscriptionMessage{
		User: twitch.User{
			UserID:    "debug-" + req.Username,
			UserLogin: strings.ToLower(req.Username),
			UserName:  req.Username,
		},
		Tier:             "1000",
		Message:          twitch.Message{Text: req.Message},
		CumulativeMonths: req.CumulativeMonths,
		StreakMonths:     req.CumulativeMonths,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleDebugRaid handles debug raid event
func handleDebugRaid(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		FromBroadcaster string `json:"fromBroadcaster"`
		Viewers         int    `json:"viewers"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.FromBroadcaster == "" {
		req.FromBroadcaster = "DebugRaider"
	}
	if req.Viewers == 0 {
		req.Viewers = 10
	}

	twitcheventsub.HandleChannelRaid(twitch.EventChannelRaid{
		FromBroadcaster: twitch.FromBroadcaster{
			FromBroadcasterUserId:    "debug-" + req.FromBroadcaster,
			FromBroadcasterUserLogin: strings.ToLower(req.FromBroadcaster),
			FromBroadcasterUserName:  req.FromBroadcaster,
		},
		Viewers: req.Viewers,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleDebugShoutout handles debug shoutout event
func handleDebugShoutout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		FromBroadcaster string `json:"fromBroadcaster"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.FromBroadcaster == "" {
		req.FromBroadcaster = "DebugShouter"
	}

	twitcheventsub.HandleChannelShoutoutReceive(twitch.EventChannelShoutoutReceive{
		FromBroadcaster: twitch.FromBroadcaster{
			FromBroadcasterUserId:    "debug-" + req.FromBroadcaster,
			FromBroadcasterUserLogin: strings.ToLower(req.FromBroadcaster),
			FromBroadcasterUserName:  req.FromBroadcaster,
		},
		ViewerCount: 100,
		StartedAt:   time.Now(),
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleDebugStreamOnline handles debug stream online event
func handleDebugStreamOnline(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	twitcheventsub.HandleStreamOnline(twitch.EventStreamOnline{
		Broadcaster: twitch.Broadcaster{
			BroadcasterUserId:    "debug-broadcaster",
			BroadcasterUserLogin: "debugbroadcaster",
			BroadcasterUserName:  "DebugBroadcaster",
		},
		Id:        "debug-stream-" + time.Now().Format("20060102150405"),
		Type:      "live",
		StartedAt: time.Now(),
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleDebugStreamOffline handles debug stream offline event
func handleDebugStreamOffline(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	twitcheventsub.HandleStreamOffline(twitch.EventStreamOffline{
		BroadcasterUserId:    "debug-broadcaster",
		BroadcasterUserLogin: "debugbroadcaster",
		BroadcasterUserName:  "DebugBroadcaster",
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleAuth handles OAuth authentication redirect
func handleAuth(w http.ResponseWriter, r *http.Request) {
	authURL := twitchtoken.GetAuthURL()
	http.Redirect(w, r, authURL, http.StatusFound)
}

// handleCallback handles OAuth callback
func handleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "code not found", http.StatusBadRequest)
		return
	}

	// Get token from Twitch
	result, err := twitchtoken.GetTwitchToken(code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Process expires_in
	expiresInFloat, ok := result["expires_in"].(float64)
	if !ok {
		http.Error(w, "invalid expires_in", http.StatusInternalServerError)
		return
	}
	expiresAtNew := time.Now().Unix() + int64(expiresInFloat)
	newToken := twitchtoken.Token{
		AccessToken:  result["access_token"].(string),
		RefreshToken: result["refresh_token"].(string),
		Scope:        result["scope"].(string),
		ExpiresAt:    expiresAtNew,
	}
	if err := newToken.SaveToken(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Success message
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <title>認証成功 - Twitch FAX</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            height: 100vh;
            margin: 0;
            background-color: #0e0e10;
            color: #efeff1;
        }
        .container {
            text-align: center;
            padding: 2rem;
            background-color: #18181b;
            border-radius: 8px;
            box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
        }
        h1 {
            color: #9147ff;
            margin-bottom: 1rem;
        }
        p {
            margin-bottom: 1.5rem;
        }
        a {
            color: #9147ff;
            text-decoration: none;
            padding: 0.5rem 1rem;
            border: 2px solid #9147ff;
            border-radius: 4px;
            transition: all 0.2s;
        }
        a:hover {
            background-color: #9147ff;
            color: #ffffff;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>🎉 認証成功！</h1>
        <p>Twitchアカウントの連携が完了しました。</p>
        <p>このウィンドウを閉じて、アプリケーションに戻ってください。</p>
        <a href="/">ホームに戻る</a>
    </div>
</body>
</html>
`)
}

// handleSettings returns current settings
func handleSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	settings := map[string]interface{}{
		"font": fontmanager.GetCurrentFontInfo(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}

// handleFontUpload handles font file upload
func handleFontUpload(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers first
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle OPTIONS request
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	switch r.Method {
	case http.MethodPost:
		// Parse multipart form
		err := r.ParseMultipartForm(fontmanager.MaxFileSize)
		if err != nil {
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			return
		}

		// Get the file
		file, header, err := r.FormFile("font")
		if err != nil {
			http.Error(w, "Failed to get file", http.StatusBadRequest)
			return
		}
		defer file.Close()

		// Save the font
		err = fontmanager.SaveCustomFont(header.Filename, file, header.Size)
		if err != nil {
			logger.Error("Failed to save font", zap.Error(err))

			// Return appropriate error message
			switch err {
			case fontmanager.ErrFileTooLarge:
				http.Error(w, "File too large (max 50MB)", http.StatusRequestEntityTooLarge)
			case fontmanager.ErrInvalidFormat:
				http.Error(w, "Invalid font format (only TTF/OTF supported)", http.StatusBadRequest)
			default:
				http.Error(w, "Failed to save font", http.StatusInternalServerError)
			}
			return
		}

		// Return success with updated font info
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"font":    fontmanager.GetCurrentFontInfo(),
		})

	case http.MethodDelete:
		// Delete custom font
		err := fontmanager.DeleteCustomFont()
		if err != nil {
			if err == fontmanager.ErrNoCustomFont {
				http.Error(w, "No custom font configured", http.StatusNotFound)
			} else {
				http.Error(w, "Failed to delete font", http.StatusInternalServerError)
			}
			return
		}

		// Return success
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Custom font deleted successfully",
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleFontPreview generates a preview image with the current font
func handleFontPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse JSON body
	var req struct {
		Text string `json:"text"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Text == "" {
		req.Text = "サンプルテキスト Sample Text 123"
	}

	// Generate preview image
	fragments := []twitch.ChatMessageFragment{
		{Type: "text", Text: req.Text},
	}

	// Use output package to generate image
	img, err := output.GeneratePreviewImage("プレビュー", fragments)
	if err != nil {
		logger.Error("Failed to generate preview", zap.Error(err))
		http.Error(w, "Failed to generate preview", http.StatusInternalServerError)
		return
	}

	// Return base64 encoded image
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"image": img,
	})
}

// handleAuthStatus returns current Twitch authentication status
func handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get current token status
	token, isValid, err := twitchtoken.GetLatestToken()

	response := map[string]interface{}{
		"authUrl":       twitchtoken.GetAuthURL(),
		"authenticated": false,
		"expiresAt":     nil,
		"error":         nil,
	}

	if err != nil {
		// No token found
		response["error"] = "No token found"
	} else {
		response["authenticated"] = isValid
		response["expiresAt"] = token.ExpiresAt
		if !isValid {
			response["error"] = "Token expired"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
