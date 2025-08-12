package env

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/joho/godotenv"
	"github.com/nantokaworks/twitch-overlay/internal/localdb"
	"github.com/nantokaworks/twitch-overlay/internal/settings"
	"github.com/nantokaworks/twitch-overlay/internal/shared/logger"
	"go.uber.org/zap"
)

type EnvValue struct {
	ClientID              *string
	ClientSecret          *string
	TwitchUserID          *string
	TriggerCustomRewordID *string
	PrinterAddress        *string
	BestQuality           bool
	Dither                bool
	BlackPoint            float32
	AutoRotate            bool
	DebugOutput           bool
	KeepAliveInterval     int
	KeepAliveEnabled      bool
	ClockEnabled          bool
	DryRunMode            bool
	RotatePrint           bool
	InitialPrintEnabled   bool
	ServerPort            int
	TimeZone              string
}

var Value EnvValue

func init() {
	// Load environment variables from .env file
	loadDotEnv()

	// データベース優先で設定を読み込み
	if err := loadFromDatabase(); err != nil {
		// DBエラー時は環境変数フォールバック
		logger.Warn("Failed to load from database, using environment variables", zap.Error(err))
		loadFromEnvironment()
	}
}

func loadDotEnv() {
	// Get executable directory
	execPath, err := os.Executable()
	if err != nil {
		fmt.Printf("Warning: Could not get executable path: %v\n", err)
		execPath = ""
	}

	execDir := filepath.Dir(execPath)

	// Try multiple paths
	possiblePaths := []string{}

	// First, try the executable directory
	if execPath != "" {
		possiblePaths = append(possiblePaths, filepath.Join(execDir, ".env"))
	}

	// Then try other common locations
	possiblePaths = append(possiblePaths,
		".env",           // Current directory
		"../.env",        // Parent directory
		"../../.env",     // Two levels up (for cmd/twitch-overlay)
	)

	loaded := false
	for _, path := range possiblePaths {
		if err := godotenv.Load(path); err == nil {
			fmt.Printf("Loaded .env from: %s\n", path)
			loaded = true
			break
		}
	}

	if !loaded {
		// Try to load from environment without file
		fmt.Println("Warning: .env file not found, using system environment variables")
	}
}

func loadFromDatabase() error {
	// データベース接続を確立
	db, err := localdb.SetupDB("./local.db")
	if err != nil {
		return fmt.Errorf("failed to setup database: %w", err)
	}

	settingsManager := settings.NewSettingsManager(db)

	// 環境変数からの移行（初回のみ）
	if err := settingsManager.MigrateFromEnv(); err != nil {
		logger.Error("Failed to migrate from env", zap.Error(err))
		// 移行失敗時も続行
	}

	// 初期設定のセットアップ
	if err := settingsManager.InitializeDefaultSettings(); err != nil {
		logger.Error("Failed to initialize default settings", zap.Error(err))
		return fmt.Errorf("failed to initialize default settings: %w", err)
	}

	// データベースから設定を読み込み
	clientID, err := settingsManager.GetRealValue("CLIENT_ID")
	if err != nil {
		return fmt.Errorf("failed to get CLIENT_ID: %w", err)
	}

	clientSecret, err := settingsManager.GetRealValue("CLIENT_SECRET")
	if err != nil {
		return fmt.Errorf("failed to get CLIENT_SECRET: %w", err)
	}

	twitchUserID, err := settingsManager.GetRealValue("TWITCH_USER_ID")
	if err != nil {
		return fmt.Errorf("failed to get TWITCH_USER_ID: %w", err)
	}

	triggerCustomRewordID, err := settingsManager.GetRealValue("TRIGGER_CUSTOM_REWORD_ID")
	if err != nil {
		return fmt.Errorf("failed to get TRIGGER_CUSTOM_REWORD_ID: %w", err)
	}

	printerAddress, err := settingsManager.GetRealValue("PRINTER_ADDRESS")
	if err != nil {
		return fmt.Errorf("failed to get PRINTER_ADDRESS: %w", err)
	}

	bestQuality, _ := settingsManager.GetRealValue("BEST_QUALITY")
	dither, _ := settingsManager.GetRealValue("DITHER")
	blackPoint, _ := settingsManager.GetRealValue("BLACK_POINT")
	autoRotate, _ := settingsManager.GetRealValue("AUTO_ROTATE")
	debugOutput, _ := settingsManager.GetRealValue("DEBUG_OUTPUT")
	keepAliveInterval, _ := settingsManager.GetRealValue("KEEP_ALIVE_INTERVAL")
	keepAliveEnabled, _ := settingsManager.GetRealValue("KEEP_ALIVE_ENABLED")
	clockEnabled, _ := settingsManager.GetRealValue("CLOCK_ENABLED")
	dryRunMode, _ := settingsManager.GetRealValue("DRY_RUN_MODE")
	rotatePrint, _ := settingsManager.GetRealValue("ROTATE_PRINT")
	initialPrintEnabled, _ := settingsManager.GetRealValue("INITIAL_PRINT_ENABLED")
	timeZone, _ := settingsManager.GetRealValue("TIMEZONE")

	// SERVER_PORTは環境変数のまま
	serverPortStr := getEnvOrDefault("SERVER_PORT", "8080")

	// EnvValue構造体に設定
	Value = EnvValue{
		ClientID:              stringPtr(clientID),
		ClientSecret:          stringPtr(clientSecret),
		TwitchUserID:          stringPtr(twitchUserID),
		TriggerCustomRewordID: stringPtr(triggerCustomRewordID),
		PrinterAddress:        stringPtr(printerAddress),
		BestQuality:           bestQuality == "true",
		Dither:                dither == "true",
		BlackPoint:            parseFloatStr(blackPoint),
		AutoRotate:            autoRotate == "true",
		DebugOutput:           debugOutput == "true",
		KeepAliveInterval:     parseIntStr(keepAliveInterval),
		KeepAliveEnabled:      keepAliveEnabled == "true",
		ClockEnabled:          clockEnabled == "true",
		DryRunMode:            dryRunMode == "true",
		RotatePrint:           rotatePrint == "true",
		InitialPrintEnabled:   initialPrintEnabled == "true",
		ServerPort:            parseIntStr(*serverPortStr),
		TimeZone:              timeZone,
	}

	// 機能ステータスをチェックして警告を表示
	status, err := settingsManager.CheckFeatureStatus()
	if err == nil && len(status.MissingSettings) > 0 {
		logger.Warn("Some required settings are missing", 
			zap.Strings("missing", status.MissingSettings),
			zap.Strings("warnings", status.Warnings))
	}

	fmt.Printf("Loaded settings from database\n")
	return nil
}

func loadFromEnvironment() {
	// 従来の環境変数読み込み（フォールバック用）
	clientID, err := getEnv("CLIENT_ID")
	if err != nil {
		logger.Warn("CLIENT_ID not found in environment, using empty value")
		clientID = stringPtr("")
	}
	clientSecret, err := getEnv("CLIENT_SECRET")
	if err != nil {
		logger.Warn("CLIENT_SECRET not found in environment, using empty value")
		clientSecret = stringPtr("")
	}
	twitchUserID, err := getEnv("TWITCH_USER_ID")
	if err != nil {
		logger.Warn("TWITCH_USER_ID not found in environment, using empty value")
		twitchUserID = stringPtr("")
	}
	triggerCustomRewordID, err := getEnv("TRIGGER_CUSTOM_REWORD_ID")
	if err != nil {
		logger.Warn("TRIGGER_CUSTOM_REWORD_ID not found in environment, using empty value")
		triggerCustomRewordID = stringPtr("")
	}
	printerAddress, err := getEnv("PRINTER_ADDRESS")
	if err != nil {
		logger.Warn("PRINTER_ADDRESS not found in environment, using empty value")
		printerAddress = stringPtr("")
	}
	bestQuality := getEnvOrDefault("BEST_QUALITY", "false")
	dither := getEnvOrDefault("DITHER", "false")
	blackPoint := getEnvOrDefault("BLACK_POINT", "100")
	autoRotate := getEnvOrDefault("AUTO_ROTATE", "false")
	debugOutput := getEnvOrDefault("DEBUG_OUTPUT", "false")

	// Optional environment variables
	keepAliveInterval := getEnvOrDefault("KEEP_ALIVE_INTERVAL", "60")
	keepAliveEnabled := getEnvOrDefault("KEEP_ALIVE_ENABLED", "false")
	clockEnabled := getEnvOrDefault("CLOCK_ENABLED", "false")
	dryRunMode := getEnvOrDefault("DRY_RUN_MODE", "true") // セキュリティ上trueをデフォルトに
	rotatePrint := getEnvOrDefault("ROTATE_PRINT", "false")
	initialPrintEnabled := getEnvOrDefault("INITIAL_PRINT_ENABLED", "false")
	serverPort := getEnvOrDefault("SERVER_PORT", "8080")
	timeZone := getEnvOrDefault("TIMEZONE", "Asia/Tokyo")

	// Initialize the Env struct with environment variables
	Value = EnvValue{
		ClientID:              clientID,
		ClientSecret:          clientSecret,
		TwitchUserID:          twitchUserID,
		TriggerCustomRewordID: triggerCustomRewordID,
		PrinterAddress:        printerAddress,
		BestQuality:           *bestQuality == "true",
		Dither:                *dither == "true",
		BlackPoint:            parseFloat(blackPoint),
		AutoRotate:            *autoRotate == "true",
		DebugOutput:           *debugOutput == "true",
		KeepAliveInterval:     parseInt(keepAliveInterval),
		KeepAliveEnabled:      *keepAliveEnabled == "true",
		ClockEnabled:          *clockEnabled == "true",
		DryRunMode:            *dryRunMode == "true",
		RotatePrint:           *rotatePrint == "true",
		InitialPrintEnabled:   *initialPrintEnabled == "true",
		ServerPort:            parseInt(serverPort),
		TimeZone:              *timeZone,
	}

	fmt.Printf("Loaded environment variables (fallback mode)\n")
}

func getEnv(key string) (*string, error) {
	value, exists := os.LookupEnv(key)
	if !exists {
		return nil, fmt.Errorf("environment variable %s not set", key)
	}
	return &value, nil
}

func getEnvOrDefault(key, defaultValue string) *string {
	value, exists := os.LookupEnv(key)
	if !exists {
		return &defaultValue
	}
	return &value
}

func stringPtr(s string) *string {
	return &s
}

func parseFloat(s *string) float32 {
	f, err := strconv.ParseFloat(*s, 32)
	if err != nil {
		log.Fatalf("floatへの変換エラー: %v", err)
	}
	return float32(f)
}

func parseFloatStr(s string) float32 {
	if s == "" {
		return 100.0 // デフォルト値
	}
	f, err := strconv.ParseFloat(s, 32)
	if err != nil {
		logger.Warn("Float conversion error, using default", zap.String("value", s), zap.Error(err))
		return 100.0
	}
	return float32(f)
}

func parseInt(s *string) int {
	i, err := strconv.Atoi(*s)
	if err != nil {
		log.Fatalf("intへの変換エラー: %v", err)
	}
	return i
}

func parseIntStr(s string) int {
	if s == "" {
		return 0
	}
	i, err := strconv.Atoi(s)
	if err != nil {
		logger.Warn("Int conversion error, using default", zap.String("value", s), zap.Error(err))
		return 0
	}
	return i
}