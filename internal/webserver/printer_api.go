package webserver

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/nantokaworks/twitch-fax/internal/output"
	"github.com/nantokaworks/twitch-fax/internal/shared/logger"
	"go.uber.org/zap"
)

type BluetoothDevice struct {
	MACAddress     string    `json:"mac_address"`
	Name           string    `json:"name,omitempty"`
	SignalStrength int       `json:"signal_strength,omitempty"`
	LastSeen       time.Time `json:"last_seen"`
}

type ScanResponse struct {
	Devices []BluetoothDevice `json:"devices"`
	Status  string            `json:"status"`
	Message string            `json:"message,omitempty"`
}

type TestResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// handlePrinterScan プリンターデバイスのスキャンを実行
func handlePrinterScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	logger.Info("Starting printer scan")

	// プリンタースキャンを実行
	c, err := output.SetupPrinter()
	if err != nil {
		logger.Error("Failed to setup scanner", zap.Error(err))
		http.Error(w, "Failed to setup scanner", http.StatusInternalServerError)
		return
	}
	defer c.Stop()

	// デバッグログを有効にする（find-faxと同じ設定）
	c.Debug.Log = true

	// 10秒間スキャン
	c.Timeout = 10 * time.Second
	devices, err := c.ScanDevices("")

	response := ScanResponse{
		Devices: []BluetoothDevice{},
		Status:  "success",
	}

	if err != nil {
		logger.Error("Device scan failed", zap.Error(err))
		response.Status = "error"
		response.Message = err.Error()
	} else {
		logger.Info("Device scan completed", zap.Int("device_count", len(devices)))
		for mac, name := range devices {
			device := BluetoothDevice{
				MACAddress: mac,
				Name:       string(name),
				LastSeen:   time.Now(),
			}
			response.Devices = append(response.Devices, device)
			logger.Debug("Found device", zap.String("mac", mac), zap.String("name", string(name)))
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handlePrinterTest 指定されたプリンターの接続テスト
func handlePrinterTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		MACAddress string `json:"mac_address"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.MACAddress == "" {
		http.Error(w, "MAC address is required", http.StatusBadRequest)
		return
	}

	logger.Info("Testing printer connection", zap.String("mac_address", req.MACAddress))

	// プリンター接続テスト
	c, err := output.SetupPrinter()
	if err != nil {
		logger.Error("Failed to setup printer", zap.Error(err))
		http.Error(w, "Failed to setup printer", http.StatusInternalServerError)
		return
	}
	defer c.Stop()

	err = output.ConnectPrinter(c, req.MACAddress)

	response := TestResponse{
		Success: err == nil,
		Message: "",
	}

	if err != nil {
		logger.Error("Printer connection test failed", zap.String("mac_address", req.MACAddress), zap.Error(err))
		response.Message = err.Error()
	} else {
		logger.Info("Printer connection test successful", zap.String("mac_address", req.MACAddress))
		response.Message = "Connection successful"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handlePrinterStatus プリンターの現在の状態を取得
func handlePrinterStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// TODO: プリンターの状態を取得する実装
	// 現在は基本的な情報のみ返す
	response := map[string]interface{}{
		"connected":    false, // TODO: 実際の接続状態を確認
		"last_print":   nil,   // TODO: 最後の印刷時刻
		"print_queue":  0,     // TODO: 印刷キューの長さ
		"dry_run_mode": true,  // TODO: 設定から取得
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}