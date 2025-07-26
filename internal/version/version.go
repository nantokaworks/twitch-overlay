package version

import "fmt"

var (
	// Version is the application version (set at build time)
	Version = "dev"
	
	// Commit is the git commit hash (set at build time)
	Commit = "unknown"
	
	// BuildTime is the build timestamp (set at build time)
	BuildTime = "unknown"
)

// String returns a formatted version string
func String() string {
	return fmt.Sprintf("v%s (commit: %s, built: %s)", Version, Commit, BuildTime)
}