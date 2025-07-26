package env

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/joho/godotenv"
)

type EnvValue struct {
	ClientID              *string
	ClientSecret          *string
	RedirectURI           *string
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
}

var Value EnvValue

func init() {

	// Load environment variables from .env file
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
		"../../.env",     // Two levels up (for cmd/twitch-fax)
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

	clientID, err := getEnv("CLIENT_ID")
	if err != nil {
		log.Fatalf("Error getting CLIENT_ID: %v", err)
	}
	clientSecret, err := getEnv("CLIENT_SECRET")
	if err != nil {
		log.Fatalf("Error getting CLIENT_SECRET: %v", err)
	}
	redirectURI, err := getEnv("REDIRECT_URI")
	if err != nil {
		log.Fatalf("Error getting REDIRECT_URI: %v", err)
	}
	twitchUserID, err := getEnv("TWITCH_USER_ID")
	if err != nil {
		log.Fatalf("Error getting TWITCH_USER_ID: %v", err)
	}
	triggerCustomRewordID, err := getEnv("TRIGGER_CUSTOM_REWORD_ID")
	if err != nil {
		log.Fatalf("Error getting TRIGGER_CUSTOM_REWORD_ID: %v", err)
	}
	printerAddress, err := getEnv("PRINTER_ADDRESS")
	if err != nil {
		log.Fatalf("Error getting PRINTER_ADDRESS: %v", err)
	}
	bestQuality, err := getEnv("BEST_QUALITY")
	if err != nil {
		log.Fatalf("Error getting BEST_QUALITY: %v", err)
	}
	dither, err := getEnv("DITHER")
	if err != nil {
		log.Fatalf("Error getting DITHER: %v", err)
	}
	blackPoint, err := getEnv("BLACK_POINT")
	if err != nil {
		log.Fatalf("Error getting BLACK_POINT: %v", err)
	}
	autoRotate, err := getEnv("AUTO_ROTATE")
	if err != nil {
		log.Fatalf("Error getting AUTO_ROTATE: %v", err)
	}
	debugOutput, err := getEnv("DEBUG_OUTPUT")
	if err != nil {
		log.Fatalf("Error getting DEBUG_OUTPUT: %v", err)
	}
	
	// Optional environment variables
	keepAliveInterval := getEnvOrDefault("KEEP_ALIVE_INTERVAL", "60")
	keepAliveEnabled := getEnvOrDefault("KEEP_ALIVE_ENABLED", "true")
	clockEnabled := getEnvOrDefault("CLOCK_ENABLED", "true")
	dryRunMode := getEnvOrDefault("DRY_RUN_MODE", "false")
	rotatePrint := getEnvOrDefault("ROTATE_PRINT", "false")
	initialPrintEnabled := getEnvOrDefault("INITIAL_PRINT_ENABLED", "false")
	serverPort := getEnvOrDefault("SERVER_PORT", "8080")

	// Initialize the Env struct with environment variables
	Value = EnvValue{
		ClientID:              clientID,
		ClientSecret:          clientSecret,
		RedirectURI:           redirectURI,
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
	}

	fmt.Printf("Loaded environment variables: %+v\n", Value)
}

func getEnv(key string) (*string, error) {
	value, exists := os.LookupEnv(key)
	if !exists {
		return nil, fmt.Errorf("environment variable %s not set", key)
	}
	return &value, nil
}

func parseFloat(s *string) float32 {
	f, err := strconv.ParseFloat(*s, 32)
	if err != nil {
		log.Fatalf("floatへの変換エラー: %v", err)
	}
	return float32(f)
}

func getEnvOrDefault(key, defaultValue string) *string {
	value, exists := os.LookupEnv(key)
	if !exists {
		return &defaultValue
	}
	return &value
}

func parseInt(s *string) int {
	i, err := strconv.Atoi(*s)
	if err != nil {
		log.Fatalf("intへの変換エラー: %v", err)
	}
	return i
}
