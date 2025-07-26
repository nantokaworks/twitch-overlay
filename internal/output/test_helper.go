package output

import (
	"os"
	"testing"
)

// SetupTestEnvironment sets up the test environment
func SetupTestEnvironment(t *testing.T) func() {
	t.Helper()
	
	// Save original environment
	originalEnv := make(map[string]string)
	envVars := []string{
		"CLIENT_ID",
		"CLIENT_SECRET",
		"REDIRECT_URI",
		"TWITCH_USER_ID",
		"TRIGGER_CUSTOM_REWORD_ID",
		"PRINTER_ADDRESS",
		"DEBUG_OUTPUT",
	}
	
	for _, key := range envVars {
		originalEnv[key] = os.Getenv(key)
	}
	
	// Set test environment
	os.Setenv("CLIENT_ID", "test_client_id")
	os.Setenv("CLIENT_SECRET", "test_client_secret")
	os.Setenv("REDIRECT_URI", "http://localhost:3000/callback")
	os.Setenv("TWITCH_USER_ID", "123456789")
	os.Setenv("TRIGGER_CUSTOM_REWORD_ID", "test_reward_id")
	os.Setenv("PRINTER_ADDRESS", "test-printer-address")
	os.Setenv("DEBUG_OUTPUT", "true")
	os.Setenv("DRY_RUN_MODE", "true")
	
	// Return cleanup function
	return func() {
		for key, value := range originalEnv {
			if value == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, value)
			}
		}
	}
}