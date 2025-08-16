package webserver

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"time"

	"github.com/nantokaworks/twitch-overlay/internal/shared/logger"
	"go.uber.org/zap"
)

// handleBluetoothRestart handles Bluetooth service restart requests
func handleBluetoothRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	logger.Info("[ServiceControl] Starting Bluetooth service restart")

	// Execute systemctl restart bluetooth.service
	cmd := exec.Command("sudo", "systemctl", "restart", "bluetooth.service")
	output, err := cmd.CombinedOutput()

	if err != nil {
		logger.Error("[ServiceControl] Failed to restart Bluetooth service",
			zap.Error(err),
			zap.String("output", string(output)))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Bluetoothサービスの再起動に失敗しました",
			"error":   err.Error(),
		})
		return
	}

	logger.Info("[ServiceControl] Bluetooth service restart command executed successfully")

	// Wait a bit for the service to stabilize
	time.Sleep(2 * time.Second)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Bluetoothサービスを再起動しました",
	})
}

// handleServiceRestart handles application service restart requests
func handleServiceRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	logger.Info("[ServiceControl] Starting application service restart")

	// Execute systemctl restart twitch-overlay.service
	cmd := exec.Command("sudo", "systemctl", "restart", "twitch-overlay.service")
	output, err := cmd.CombinedOutput()

	if err != nil {
		logger.Error("[ServiceControl] Failed to restart application service",
			zap.Error(err),
			zap.String("output", string(output)))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "アプリケーションサービスの再起動に失敗しました",
			"error":   err.Error(),
		})
		return
	}

	logger.Info("[ServiceControl] Application service restart command executed successfully")

	// The response might not reach the client as the service is restarting
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "アプリケーションサービスを再起動しています",
	})
}