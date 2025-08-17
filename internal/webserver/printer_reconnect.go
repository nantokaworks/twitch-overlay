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

	// Stop keep-alive goroutine
	output.StopKeepAlive()
	
	// Completely reset printer connection and BLE device
	logger.Info("[Reconnect] Stopping printer and releasing BLE device")
	output.Stop() // This disconnects AND releases BLE device

	// Wait for Bluetooth to fully disconnect and release resources
	// 1秒待機することで、BLEデバイスが完全に解放されるのを確実にする
	time.Sleep(1 * time.Second)

	// Setup and connect to printer (create new BLE device)
	c, err := output.SetupPrinter()
	if err != nil {
		logger.Error("Failed to setup printer for reconnection", zap.Error(err))
		// Restart keep-alive even on error
		output.StartKeepAlive()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("プリンターセットアップエラー: %v", err),
		})
		return
	}

	// Connect to the printer
	err = output.ConnectPrinter(c, printerAddress)
	if err != nil {
		logger.Error("Failed to reconnect to printer", zap.String("address", printerAddress), zap.Error(err))
		// Restart keep-alive even on error
		output.StartKeepAlive()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("接続エラー: %v", err),
		})
		return
	}
	
	// Restart keep-alive goroutine after successful reconnection
	output.StartKeepAlive()

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