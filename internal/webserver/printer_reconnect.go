package webserver

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/nantokaworks/twitch-overlay/internal/env"
	"github.com/nantokaworks/twitch-overlay/internal/output"
	"github.com/nantokaworks/twitch-overlay/internal/shared/logger"
	"go.uber.org/zap"
)

// handlePrinterReconnect プリンターへの再接続を強制的に実行
func handlePrinterReconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	logger.Info("Starting printer reconnection")

	// Get printer address from environment
	printerAddress := ""
	if env.Value.PrinterAddress != nil {
		printerAddress = *env.Value.PrinterAddress
	}

	if printerAddress == "" {
		logger.Error("Printer address not configured")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "プリンターアドレスが設定されていません",
		})
		return
	}

	// 常に完全リセットを実行（SetupPrinterが内部で完全リセットするように修正済み）
	logger.Info("[Reconnect] Performing complete printer reset")
	
	// Setup printer (内部で完全リセットを実行)
	c, err := output.SetupPrinter()
	if err != nil {
		logger.Error("Failed to setup printer", zap.Error(err))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("プリンターセットアップエラー: %v", err),
		})
		return
	}

	// Connect to printer
	err = output.ConnectPrinter(c, printerAddress)
	if err != nil {
		logger.Error("Failed to reconnect", zap.String("address", printerAddress), zap.Error(err))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("接続エラー: %v", err),
		})
		return
	}

	logger.Info("Printer reconnected successfully", zap.String("address", printerAddress))
	
	// Return success with current status
	response := map[string]interface{}{
		"success":         true,
		"connected":       output.IsConnected(),
		"printer_address": printerAddress,
		"message":         "プリンターに再接続しました",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}