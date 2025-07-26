package webserver

import (
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
	"github.com/nantokaworks/twitch-fax/internal/broadcast"
	"github.com/nantokaworks/twitch-fax/internal/faxmanager"
	"github.com/nantokaworks/twitch-fax/internal/output"
	"github.com/nantokaworks/twitch-fax/internal/shared/logger"
	"github.com/nantokaworks/twitch-fax/internal/status"
	"github.com/nantokaworks/twitch-fax/internal/twitchtoken"
	"go.uber.org/zap"
)

type SSEServer struct {
	clients map[chan string]bool
	mu      sync.RWMutex
}

var sseServer = &SSEServer{
	clients: make(map[chan string]bool),
}

// corsMiddleware adds CORS headers to HTTP handlers
func corsMiddleware(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
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
	
	// Create a custom file server that handles SPA routing
	fs := http.FileServer(http.Dir(staticDir))
	
	// Handle all routes
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Check if this is an API route
		if strings.HasPrefix(r.URL.Path, "/events") || 
		   strings.HasPrefix(r.URL.Path, "/fax/") || 
		   strings.HasPrefix(r.URL.Path, "/status") || 
		   strings.HasPrefix(r.URL.Path, "/debug/") {
			// Let the API handlers handle these
			http.NotFound(w, r)
			return
		}
		
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

	// SSE endpoint
	http.HandleFunc("/events", handleSSE)

	// Fax image endpoint
	http.HandleFunc("/fax/", handleFaxImage)

	// Status endpoint
	http.HandleFunc("/status", handleStatus)

	// Debug endpoints - シンプルに直接登録
	http.HandleFunc("/debug/fax", handleDebugFax)
	http.HandleFunc("/debug/channel-points", handleDebugChannelPoints)
	http.HandleFunc("/debug/clock", handleDebugClock)
	
	// OAuth endpoints
	http.HandleFunc("/auth", handleAuth)
	http.HandleFunc("/callback", handleCallback)

	addr := fmt.Sprintf(":%d", port)
	
	// 起動メッセージを表示（logger出力の前に）
	fmt.Println("")
	fmt.Println("====================================================")
	fmt.Printf("🚀 Webサーバーが起動しました\n")
	fmt.Printf("📡 アクセスURL:\n")
	fmt.Printf("   モノクロ表示: http://localhost:%d\n", port)
	fmt.Printf("   カラー表示:   http://localhost:%d/color\n", port)
	fmt.Printf("\n")
	fmt.Printf("🐛 デバッグモード:\n")
	fmt.Printf("   モノクロ表示: http://localhost:%d?debug=true\n", port)
	fmt.Printf("   カラー表示:   http://localhost:%d/color?debug=true\n", port)
	fmt.Printf("\n")
	fmt.Printf("🔧 環境変数 SERVER_PORT で変更可能\n")
	fmt.Println("====================================================")
	fmt.Println("")
	
	logger.Info("Starting web server", zap.String("address", addr))

	go func() {
		if err := http.ListenAndServe(addr, nil); err != nil {
			logger.Fatal("Failed to start web server", zap.Error(err))
		}
	}()
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
	WithStats bool `json:"withStats"`
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
		zap.Bool("withStats", req.WithStats))

	// Call PrintClock directly (same as scheduled clock printing)
	err = output.PrintClock(timeStr)
	if err != nil {
		logger.Error("Failed to print debug clock", zap.Error(err))
		http.Error(w, "Failed to print clock", http.StatusInternalServerError)
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
