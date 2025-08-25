package webserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

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

	// First, try simple reconnection (disconnect and reconnect)
	logger.Info("[Reconnect] Attempting simple reconnection")
	
	// Setup printer (will disconnect if connected, reuse existing client)
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

	// Try to connect
	err = output.ConnectPrinter(c, printerAddress)
	if err == nil {
		// Simple reconnection succeeded
		logger.Info("Simple reconnection successful", zap.String("address", printerAddress))
		response := map[string]interface{}{
			"success":         true,
			"connected":       output.IsConnected(),
			"printer_address": printerAddress,
			"message":         "プリンターに再接続しました",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Simple reconnection failed, try complete reset
	logger.Warn("Simple reconnection failed, attempting complete reset", zap.Error(err))
	
	// Completely reset printer connection and BLE device
	logger.Info("[Reconnect] Stopping printer and releasing BLE device")
	output.Stop() // This disconnects AND releases BLE device

	// Wait for Bluetooth to fully disconnect and release resources
	time.Sleep(500 * time.Millisecond)

	// Setup and connect to printer (create new BLE device)
	c, err = output.SetupPrinter()
	if err != nil {
		logger.Error("Failed to setup new printer after reset", zap.Error(err))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("プリンターセットアップエラー: %v", err),
		})
		return
	}

	// Final connection attempt
	err = output.ConnectPrinter(c, printerAddress)
	if err != nil {
		logger.Error("Failed to reconnect after complete reset", zap.String("address", printerAddress), zap.Error(err))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("接続エラー: %v", err),
		})
		return
	}

	logger.Info("Printer reconnected successfully after reset", zap.String("address", printerAddress))
	
	// Return success with current status
	response := map[string]interface{}{
		"success":         true,
		"connected":       output.IsConnected(),
		"printer_address": printerAddress,
		"message":         "プリンターに再接続しました（完全リセット後）",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}