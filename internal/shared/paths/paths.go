package paths

import (
	"os"
	"path/filepath"
	"strings"
)

// GetDataDir returns the data directory path for twitch-overlay
// Priority: TWITCH_OVERLAY_DATA_DIR env var > ~/.twitch-overlay
func GetDataDir() string {
	if dir := os.Getenv("TWITCH_OVERLAY_DATA_DIR"); dir != "" {
		// Expand environment variables and home directory
		dir = os.ExpandEnv(dir)
		if strings.HasPrefix(dir, "~") {
			home, _ := os.UserHomeDir()
			dir = filepath.Join(home, dir[2:])
		}
		return dir
	}
	
	// Default to ~/.twitch-overlay
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".twitch-overlay")
}

// GetDBPath returns the path to the local database file
func GetDBPath() string {
	return filepath.Join(GetDataDir(), "local.db")
}

// GetFontsDir returns the path to the fonts directory
func GetFontsDir() string {
	return filepath.Join(GetDataDir(), "fonts")
}

// GetUploadsDir returns the path to the uploads directory
func GetUploadsDir() string {
	return filepath.Join(GetDataDir(), "uploads")
}

// EnsureDataDirs creates all necessary data directories
func EnsureDataDirs() error {
	dirs := []string{
		GetDataDir(),
		GetFontsDir(),
		GetUploadsDir(),
	}
	
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	
	return nil
}