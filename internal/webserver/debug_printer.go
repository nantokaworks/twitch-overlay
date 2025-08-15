package webserver

import (
	"encoding/json"
	"net/http"
	"github.com/nantokaworks/twitch-overlay/internal/status"
)

// handleDebugPrinterStatus はデバッグ用にプリンター接続状態を手動で変更する
func handleDebugPrinterStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Connected bool `json:"connected"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// プリンター接続状態を手動で設定（これによりSSEイベントが発火する）
	status.SetPrinterConnected(req.Connected)

	response := map[string]interface{}{
		"success": true,
		"connected": req.Connected,
		"message": "Printer status updated (debug)",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}