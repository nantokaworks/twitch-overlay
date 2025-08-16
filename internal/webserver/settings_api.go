package webserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/nantokaworks/twitch-overlay/internal/env"
	"github.com/nantokaworks/twitch-overlay/internal/fontmanager"
	"github.com/nantokaworks/twitch-overlay/internal/localdb"
	"github.com/nantokaworks/twitch-overlay/internal/output"
	"github.com/nantokaworks/twitch-overlay/internal/settings"
	"github.com/nantokaworks/twitch-overlay/internal/shared/logger"
	"go.uber.org/zap"
)

// handleSettingsV2 設定の取得・更新を処理
func handleSettingsV2(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		handleGetSettings(w, r)
	case http.MethodPut:
		handleUpdateSettings(w, r)
	case http.MethodPost:
		if r.URL.Path == "/api/settings/reset" {
			handleResetSettings(w, r)
		} else {
			http.Error(w, "Not found", http.StatusNotFound)
		}
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGetSettings すべての設定を取得
func handleGetSettings(w http.ResponseWriter, r *http.Request) {
	settingsManager := settings.NewSettingsManager(localdb.GetDB())

	allSettings, err := settingsManager.GetAllSettings()
	if err != nil {
		logger.Error("Failed to get settings", zap.Error(err))
		http.Error(w, "Failed to get settings", http.StatusInternalServerError)
		return
	}

	featureStatus, err := settingsManager.CheckFeatureStatus()
	if err != nil {
		logger.Error("Failed to check feature status", zap.Error(err))
		http.Error(w, "Failed to check feature status", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"settings": allSettings,
		"status":   featureStatus,
		"font":     fontmanager.GetCurrentFontInfo(), // 既存のフォント情報
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleUpdateSettings 設定を更新
func handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req map[string]string
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	settingsManager := settings.NewSettingsManager(localdb.GetDB())

	// バリデーションと更新
	for key, value := range req {
		// バリデーション
		if err := settings.ValidateSetting(key, value); err != nil {
			logger.Warn("Setting validation failed", zap.String("key", key), zap.String("value", value), zap.Error(err))
			http.Error(w, fmt.Sprintf("Invalid value for %s: %v", key, err), http.StatusBadRequest)
			return
		}

		// 設定更新
		if err := settingsManager.SetSetting(key, value); err != nil {
			logger.Error("Failed to update setting", zap.String("key", key), zap.Error(err))
			http.Error(w, fmt.Sprintf("Failed to update %s: %v", key, err), http.StatusInternalServerError)
			return
		}

		// 機密情報以外はログに記録
		if defaultSetting, exists := settings.DefaultSettings[key]; exists && defaultSetting.Type != settings.SettingTypeSecret {
			logger.Info("Setting updated", zap.String("key", key), zap.String("value", value))
		} else {
			logger.Info("Secret setting updated", zap.String("key", key))
		}
	}

	// 設定変更後にenv.Valueを再読み込み
	if err := env.ReloadFromDatabase(); err != nil {
		logger.Warn("Failed to reload env values from database", zap.Error(err))
	}

	// PRINTER_ADDRESSが変更された場合は再接続を試みる
	if newAddress, hasPrinterAddress := req["PRINTER_ADDRESS"]; hasPrinterAddress && newAddress != "" {
		logger.Info("Printer address changed, attempting reconnection", zap.String("new_address", newAddress))
		
		// 新しいアドレスで再接続（goroutineで非同期実行）
		go func() {
			// パニックからの回復処理
			defer func() {
				if r := recover(); r != nil {
					logger.Error("Panic during printer reconnection", 
						zap.Any("panic", r),
						zap.String("address", newAddress))
				}
			}()
			
			// Stop keep-alive goroutine
			output.StopKeepAlive()
			
			// 既存の接続を安全に切断
			func() {
				defer func() {
					if r := recover(); r != nil {
						logger.Warn("Recovered from panic during disconnect", zap.Any("panic", r))
					}
				}()
				output.Disconnect()
			}()
			
			time.Sleep(500 * time.Millisecond) // 少し待機
			
			c, err := output.SetupPrinter()
			if err != nil {
				logger.Error("Failed to setup printer after settings change", zap.Error(err))
				// Restart keep-alive even on error
				output.StartKeepAlive()
				return
			}
			
			err = output.ConnectPrinter(c, newAddress)
			if err != nil {
				logger.Error("Failed to reconnect to printer with new address", zap.String("address", newAddress), zap.Error(err))
			} else {
				logger.Info("Successfully reconnected to printer", zap.String("address", newAddress))
			}
			
			// Restart keep-alive goroutine
			output.StartKeepAlive()
		}()
	}
	
	// KEEP_ALIVE関連の設定が変更された場合はKeepAliveを再起動
	if _, hasKeepAliveEnabled := req["KEEP_ALIVE_ENABLED"]; hasKeepAliveEnabled {
		logger.Info("Keep-alive enabled setting changed, restarting keep-alive")
		go func() {
			// パニックからの回復処理
			defer func() {
				if r := recover(); r != nil {
					logger.Error("Panic during keep-alive restart", zap.Any("panic", r))
				}
			}()
			
			// Stop and restart keep-alive
			output.StopKeepAlive()
			time.Sleep(500 * time.Millisecond)
			output.StartKeepAlive()
		}()
	} else if _, hasKeepAliveInterval := req["KEEP_ALIVE_INTERVAL"]; hasKeepAliveInterval {
		logger.Info("Keep-alive interval setting changed, restarting keep-alive")
		go func() {
			// パニックからの回復処理
			defer func() {
				if r := recover(); r != nil {
					logger.Error("Panic during keep-alive restart", zap.Any("panic", r))
				}
			}()
			
			// Stop and restart keep-alive with new interval
			output.StopKeepAlive()
			time.Sleep(500 * time.Millisecond)
			output.StartKeepAlive()
		}()
	}

	// 更新後の設定状態を返す
	featureStatus, err := settingsManager.CheckFeatureStatus()
	if err != nil {
		logger.Error("Failed to check feature status after update", zap.Error(err))
		featureStatus = &settings.FeatureStatus{} // 空の状態を返す
	}

	// 更新された設定を取得（シークレット値も含めて返す）
	updatedSettings := make(map[string]settings.Setting)
	for key, value := range req {
		if defaultSetting, exists := settings.DefaultSettings[key]; exists {
			updatedSettings[key] = settings.Setting{
				Key:         key,
				Value:       value, // 更新直後は実際の値を返す
				Type:        defaultSetting.Type,
				Required:    defaultSetting.Required,
				Description: defaultSetting.Description,
				HasValue:    value != "",
			}
		}
	}

	response := map[string]interface{}{
		"success":  true,
		"status":   featureStatus,
		"message":  fmt.Sprintf("Updated %d setting(s) successfully", len(req)),
		"settings": updatedSettings,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleResetSettings 設定をデフォルト値にリセット
func handleResetSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Keys []string `json:"keys"` // リセットする設定キーのリスト、空の場合は全てリセット
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	settingsManager := settings.NewSettingsManager(localdb.GetDB())

	// リセット対象のキーを決定
	keysToReset := req.Keys
	if len(keysToReset) == 0 {
		// 全ての設定をリセット
		for key := range settings.DefaultSettings {
			keysToReset = append(keysToReset, key)
		}
	}

	// 設定をデフォルト値にリセット
	resetCount := 0
	for _, key := range keysToReset {
		if defaultSetting, exists := settings.DefaultSettings[key]; exists {
			if err := settingsManager.SetSetting(key, defaultSetting.Value); err != nil {
				logger.Error("Failed to reset setting", zap.String("key", key), zap.Error(err))
				http.Error(w, fmt.Sprintf("Failed to reset %s: %v", key, err), http.StatusInternalServerError)
				return
			}
			resetCount++
			logger.Info("Setting reset to default", zap.String("key", key))
		}
	}

	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Reset %d setting(s) to default values", resetCount),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleSettingsStatus 設定状態のみを取得（軽量）
func handleSettingsStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	settingsManager := settings.NewSettingsManager(localdb.GetDB())

	featureStatus, err := settingsManager.CheckFeatureStatus()
	if err != nil {
		logger.Error("Failed to check feature status", zap.Error(err))
		http.Error(w, "Failed to check feature status", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(featureStatus)
}

// handleBulkSettings 複数の設定値を一括で取得（機密情報も実値で返す - 内部用）
func handleBulkSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Keys []string `json:"keys"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	settingsManager := settings.NewSettingsManager(localdb.GetDB())
	result := make(map[string]string)

	for _, key := range req.Keys {
		if value, err := settingsManager.GetRealValue(key); err == nil {
			result[key] = value
		} else {
			logger.Warn("Failed to get setting", zap.String("key", key), zap.Error(err))
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}