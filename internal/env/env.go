package env

import (
	"fmt"
	"log"
	"os"
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
}

var Value EnvValue

func init() {

	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
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
