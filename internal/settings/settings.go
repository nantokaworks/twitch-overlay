package settings

import (
	"database/sql"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/nantokaworks/twitch-overlay/internal/shared/logger"
	"go.uber.org/zap"
)

type SettingType string

const (
	SettingTypeNormal SettingType = "normal"
	SettingTypeSecret SettingType = "secret"
)

type Setting struct {
	Key         string      `json:"key"`
	Value       string      `json:"value"`
	Type        SettingType `json:"type"`
	Required    bool        `json:"required"`
	Description string      `json:"description"`
	UpdatedAt   time.Time   `json:"updated_at"`
	HasValue    bool        `json:"has_value"` // シークレット値が設定されているかどうか
}

type SettingsManager struct {
	db *sql.DB
}

func NewSettingsManager(db *sql.DB) *SettingsManager {
	return &SettingsManager{db: db}
}

// 設定の定義
var DefaultSettings = map[string]Setting{
	// Twitch設定（機密情報）
	"CLIENT_ID": {
		Key: "CLIENT_ID", Value: "", Type: SettingTypeSecret, Required: true,
		Description: "Twitch API Client ID",
	},
	"CLIENT_SECRET": {
		Key: "CLIENT_SECRET", Value: "", Type: SettingTypeSecret, Required: true,
		Description: "Twitch API Client Secret",
	},
	"TWITCH_USER_ID": {
		Key: "TWITCH_USER_ID", Value: "", Type: SettingTypeSecret, Required: true,
		Description: "Twitch User ID for monitoring",
	},
	"TRIGGER_CUSTOM_REWORD_ID": {
		Key: "TRIGGER_CUSTOM_REWORD_ID", Value: "", Type: SettingTypeSecret, Required: true,
		Description: "Custom Reward ID for triggering FAX",
	},

	// プリンター設定
	"PRINTER_ADDRESS": {
		Key: "PRINTER_ADDRESS", Value: "", Type: SettingTypeNormal, Required: true,
		Description: "Bluetooth MAC address of the printer",
	},
	"DRY_RUN_MODE": {
		Key: "DRY_RUN_MODE", Value: "true", Type: SettingTypeNormal, Required: false,
		Description: "Enable dry run mode (no actual printing)",
	},
	"BEST_QUALITY": {
		Key: "BEST_QUALITY", Value: "true", Type: SettingTypeNormal, Required: false,
		Description: "Enable best quality printing",
	},
	"DITHER": {
		Key: "DITHER", Value: "true", Type: SettingTypeNormal, Required: false,
		Description: "Enable dithering",
	},
	"BLACK_POINT": {
		Key: "BLACK_POINT", Value: "0", Type: SettingTypeNormal, Required: false,
		Description: "Black point threshold (0-255)",
	},
	"AUTO_ROTATE": {
		Key: "AUTO_ROTATE", Value: "false", Type: SettingTypeNormal, Required: false,
		Description: "Auto rotate images",
	},
	"ROTATE_PRINT": {
		Key: "ROTATE_PRINT", Value: "true", Type: SettingTypeNormal, Required: false,
		Description: "Rotate print output 180 degrees",
	},
	"INITIAL_PRINT_ENABLED": {
		Key: "INITIAL_PRINT_ENABLED", Value: "false", Type: SettingTypeNormal, Required: false,
		Description: "Enable initial clock print on startup",
	},

	// 動作設定
	"KEEP_ALIVE_INTERVAL": {
		Key: "KEEP_ALIVE_INTERVAL", Value: "60", Type: SettingTypeNormal, Required: false,
		Description: "Keep alive interval in seconds",
	},
	"KEEP_ALIVE_ENABLED": {
		Key: "KEEP_ALIVE_ENABLED", Value: "false", Type: SettingTypeNormal, Required: false,
		Description: "Enable keep alive functionality",
	},
	"CLOCK_ENABLED": {
		Key: "CLOCK_ENABLED", Value: "false", Type: SettingTypeNormal, Required: false,
		Description: "Enable clock printing",
	},
	"DEBUG_OUTPUT": {
		Key: "DEBUG_OUTPUT", Value: "false", Type: SettingTypeNormal, Required: false,
		Description: "Enable debug output",
	},
	"TIMEZONE": {
		Key: "TIMEZONE", Value: "Asia/Tokyo", Type: SettingTypeNormal, Required: false,
		Description: "Timezone for clock display",
	},
	
	// フォント設定
	"FONT_FILENAME": {
		Key: "FONT_FILENAME", Value: "", Type: SettingTypeNormal, Required: false,
		Description: "Uploaded font file name",
	},
}

// 機能の有効性チェック
type FeatureStatus struct {
	TwitchConfigured  bool     `json:"twitch_configured"`
	PrinterConfigured bool     `json:"printer_configured"`
	PrinterConnected  bool     `json:"printer_connected"`
	MissingSettings   []string `json:"missing_settings"`
	Warnings          []string `json:"warnings"`
}

func (sm *SettingsManager) CheckFeatureStatus() (*FeatureStatus, error) {
	status := &FeatureStatus{
		MissingSettings: []string{},
		Warnings:        []string{},
	}

	// Twitch設定チェック
	twitchSettings := []string{"CLIENT_ID", "CLIENT_SECRET", "TWITCH_USER_ID", "TRIGGER_CUSTOM_REWORD_ID"}
	twitchComplete := true
	for _, key := range twitchSettings {
		if val, err := sm.GetSetting(key); err != nil || val == "" {
			status.MissingSettings = append(status.MissingSettings, key)
			twitchComplete = false
		}
	}
	status.TwitchConfigured = twitchComplete

	// プリンター設定チェック
	if printerAddr, err := sm.GetSetting("PRINTER_ADDRESS"); err != nil || printerAddr == "" {
		status.MissingSettings = append(status.MissingSettings, "PRINTER_ADDRESS")
		status.PrinterConfigured = false
	} else {
		status.PrinterConfigured = true
		// TODO: 実際の接続テストを実装
		status.PrinterConnected = false
	}

	// 警告チェック
	if dryRun, _ := sm.GetSetting("DRY_RUN_MODE"); dryRun == "true" {
		status.Warnings = append(status.Warnings, "DRY_RUN_MODE is enabled - no actual printing will occur")
	}

	return status, nil
}

// CRUD操作
func (sm *SettingsManager) GetSetting(key string) (string, error) {
	var value string
	err := sm.db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		// デフォルト値を返す
		if defaultSetting, exists := DefaultSettings[key]; exists {
			return defaultSetting.Value, nil
		}
		return "", fmt.Errorf("setting not found: %s", key)
	}
	return value, err
}

func (sm *SettingsManager) SetSetting(key, value string) error {
	// デフォルト設定が存在するかチェック
	defaultSetting, exists := DefaultSettings[key]
	if !exists {
		return fmt.Errorf("unknown setting key: %s", key)
	}

	_, err := sm.db.Exec(`
		INSERT INTO settings (key, value, setting_type, is_required, description) 
		VALUES (?, ?, ?, ?, ?) 
		ON CONFLICT(key) DO UPDATE SET 
			value = excluded.value, 
			updated_at = CURRENT_TIMESTAMP`,
		key, value,
		string(defaultSetting.Type),
		defaultSetting.Required,
		defaultSetting.Description,
	)
	return err
}

func (sm *SettingsManager) GetAllSettings() (map[string]Setting, error) {
	rows, err := sm.db.Query(`
		SELECT key, value, setting_type, is_required, description, updated_at 
		FROM settings ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings := make(map[string]Setting)
	for rows.Next() {
		var s Setting
		var settingType string
		var description sql.NullString
		err := rows.Scan(&s.Key, &s.Value, &settingType, &s.Required, &description, &s.UpdatedAt)
		if err != nil {
			return nil, err
		}
		s.Type = SettingType(settingType)
		s.Description = description.String // NullStringから通常のstringへ変換

		// 機密情報も実際の値を返す（フロントエンドでマスク処理）
		s.HasValue = s.Value != ""

		settings[s.Key] = s
	}

	// DBにない設定はデフォルト値で補完
	for key, defaultSetting := range DefaultSettings {
		if _, exists := settings[key]; !exists {
			settings[key] = defaultSetting
		}
	}

	return settings, nil
}

// 実際の値を取得（マスクなし）- 内部処理用
func (sm *SettingsManager) GetRealValue(key string) (string, error) {
	var value string
	err := sm.db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		// デフォルト値を返す
		if defaultSetting, exists := DefaultSettings[key]; exists {
			return defaultSetting.Value, nil
		}
		return "", fmt.Errorf("setting not found: %s", key)
	}
	return value, err
}

// 環境変数からの移行
func (sm *SettingsManager) MigrateFromEnv() error {
	logger.Info("Starting migration from environment variables")
	migrated := 0

	for key := range DefaultSettings {
		// 既にDB設定が存在する場合はスキップ
		var existingKey string
		if err := sm.db.QueryRow("SELECT key FROM settings WHERE key = ?", key).Scan(&existingKey); err == nil {
			continue
		}

		// 環境変数から取得
		if envValue := os.Getenv(key); envValue != "" {
			if err := sm.SetSetting(key, envValue); err != nil {
				logger.Error("Failed to migrate setting", zap.String("key", key), zap.Error(err))
				return fmt.Errorf("failed to migrate %s: %w", key, err)
			}
			logger.Info("Migrated setting from environment", zap.String("key", key))
			migrated++
		}
	}

	if migrated > 0 {
		logger.Info("Migration completed", zap.Int("migrated_count", migrated))
		
		// セキュリティ警告を表示
		if hasSecretInEnv() {
			logger.Warn("SECURITY WARNING: Sensitive data found in environment variables.")
			logger.Warn("Please remove CLIENT_SECRET and other sensitive values from .env file after confirming the migration is successful.")
		}
	}

	return nil
}

func hasSecretInEnv() bool {
	secretKeys := []string{"CLIENT_SECRET", "CLIENT_ID", "TWITCH_USER_ID", "TRIGGER_CUSTOM_REWORD_ID"}
	for _, key := range secretKeys {
		if os.Getenv(key) != "" {
			return true
		}
	}
	return false
}

// バリデーション
func ValidateSetting(key, value string) error {
	switch key {
	case "BLACK_POINT":
		if val, err := strconv.Atoi(value); err != nil || val < 0 || val > 255 {
			return fmt.Errorf("must be integer between 0 and 255")
		}
	case "KEEP_ALIVE_INTERVAL":
		if val, err := strconv.Atoi(value); err != nil || val < 10 || val > 3600 {
			return fmt.Errorf("must be integer between 10 and 3600 seconds")
		}
	case "PRINTER_ADDRESS":
		// MACアドレスの形式チェック
		if value != "" {
			matched, _ := regexp.MatchString(`^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$`, value)
			if !matched {
				return fmt.Errorf("invalid MAC address format")
			}
		}
	case "TIMEZONE":
		// 基本的なタイムゾーンのバリデーション
		if value != "" {
			if _, err := time.LoadLocation(value); err != nil {
				return fmt.Errorf("invalid timezone: %v", err)
			}
		}
	case "DRY_RUN_MODE", "BEST_QUALITY", "DITHER", "AUTO_ROTATE", "ROTATE_PRINT", "INITIAL_PRINT_ENABLED", "KEEP_ALIVE_ENABLED", "CLOCK_ENABLED", "DEBUG_OUTPUT":
		// boolean値のチェック
		if value != "true" && value != "false" {
			return fmt.Errorf("must be 'true' or 'false'")
		}
	}
	return nil
}

// 初期設定のセットアップ
func (sm *SettingsManager) InitializeDefaultSettings() error {
	for key, setting := range DefaultSettings {
		// 既に設定が存在する場合はスキップ
		var existingKey string
		if err := sm.db.QueryRow("SELECT key FROM settings WHERE key = ?", key).Scan(&existingKey); err == nil {
			continue
		}

		// デフォルト値で初期化
		if err := sm.SetSetting(key, setting.Value); err != nil {
			return fmt.Errorf("failed to initialize setting %s: %w", key, err)
		}
	}
	return nil
}