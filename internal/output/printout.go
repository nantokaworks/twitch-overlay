package output

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/joeyak/go-twitch-eventsub/v3"
	"github.com/nantokaworks/twitch-overlay/internal/env"
	"github.com/nantokaworks/twitch-overlay/internal/faxmanager"
	"github.com/nantokaworks/twitch-overlay/internal/shared/logger"
	"github.com/nantokaworks/twitch-overlay/internal/broadcast"
	"github.com/nantokaworks/twitch-overlay/internal/status"
	"go.uber.org/zap"
)

var printQueue chan image.Image
var lastPrintTime time.Time
var lastPrintMutex sync.Mutex
var printerMutex sync.Mutex

// shouldUseDryRun determines if dry-run mode should be active
func shouldUseDryRun() bool {
	// If DryRunMode is explicitly set, always use it
	if env.Value.DryRunMode {
		return true
	}
	
	// If AutoDryRunWhenOffline is enabled and stream is offline, use dry-run
	if env.Value.AutoDryRunWhenOffline && !status.IsStreamLive() {
		return true
	}
	
	return false
}

// InitializePrinter initializes the printer subsystem (including keep-alive and clock)
// This should be called from main() after env.Value is properly initialized
func InitializePrinter() {
	logger.Info("[InitializePrinter] Starting printer subsystem initialization",
		zap.Bool("keep_alive_enabled", env.Value.KeepAliveEnabled),
		zap.Int("keep_alive_interval", env.Value.KeepAliveInterval),
		zap.Bool("clock_enabled", env.Value.ClockEnabled),
		zap.String("printer_address", func() string {
			if env.Value.PrinterAddress != nil {
				return *env.Value.PrinterAddress
			}
			return "<not set>"
		}()))
	
	// Start keep-alive goroutine if enabled
	if env.Value.KeepAliveEnabled {
		logger.Info("[InitializePrinter] Starting keep-alive routine")
		go keepAliveRoutine()
	} else {
		logger.Info("[InitializePrinter] Keep-alive routine disabled")
	}
	
	// Start clock routine
	if env.Value.ClockEnabled {
		logger.Info("[InitializePrinter] Starting clock routine")
		go clockRoutine()
	} else {
		logger.Info("[InitializePrinter] Clock routine disabled")
	}
	
	logger.Info("[InitializePrinter] Printer subsystem initialization complete", 
		zap.Bool("keep_alive_enabled", env.Value.KeepAliveEnabled),
		zap.Int("keep_alive_interval", env.Value.KeepAliveInterval),
		zap.Bool("clock_enabled", env.Value.ClockEnabled))
}

func init() {
	printQueue = make(chan image.Image, 100)
	
	// Initialize last print time to now
	lastPrintTime = time.Now()
	
	// Note: clockRoutine() is now called from InitializePrinter()
	// after env.Value is properly initialized
	
	go func() {
		for img := range printQueue {
			// Lock printer for exclusive access
			printerMutex.Lock()
			
			// Setup printer if needed
			c, err := SetupPrinter()
			if err != nil {
				logger.Error("failed to setup printer", zap.Error(err))
				printerMutex.Unlock()
				continue
			}
			
			// Try to connect if not connected
			err = ConnectPrinter(c, *env.Value.PrinterAddress)
			if err != nil {
				logger.Error("failed to connect printer", zap.Error(err))
				printerMutex.Unlock()
				continue
			}
			
			// Check for dry-run mode (including auto dry-run when offline)
			if shouldUseDryRun() {
				if env.Value.AutoDryRunWhenOffline && !status.IsStreamLive() {
					logger.Info("Auto dry-run mode (stream offline): skipping actual printing")
				} else {
					logger.Info("Dry-run mode: skipping actual printing")
				}
				// Update last print time even in dry-run mode
				lastPrintMutex.Lock()
				lastPrintTime = time.Now()
				lastPrintMutex.Unlock()
			} else {
				// Rotate image 180 degrees if ROTATE_PRINT is enabled
				finalImg := img
				if env.Value.RotatePrint {
					finalImg = rotateImage180(img)
				}
				
				if err := c.Print(finalImg, opts, false); err != nil {
					logger.Error("failed to print", zap.Error(err))
				} else {
					// Update last print time on successful print
					lastPrintMutex.Lock()
					lastPrintTime = time.Now()
					lastPrintMutex.Unlock()
				}
			}
			
			// Release printer lock
			printerMutex.Unlock()
		}
	}()
}

// PrintClock sends clock output to printer and frontend
func PrintClock(timeStr string) error {
	return PrintClockWithOptions(timeStr, false)
}

// PrintClockWithOptions sends clock output to printer and frontend with options
func PrintClockWithOptions(timeStr string, forceEmptyLeaderboard bool) error {
	// Generate color version
	colorImg, err := GenerateTimeImageWithStatsColorOptions(timeStr, forceEmptyLeaderboard)
	if err != nil {
		return fmt.Errorf("failed to create color clock image: %w", err)
	}

	// Generate monochrome version for printing
	monoImg, err := GenerateTimeImageWithStatsOptions(timeStr, forceEmptyLeaderboard)
	if err != nil {
		return fmt.Errorf("failed to create monochrome clock image: %w", err)
	}

	// Save fax with faxmanager (use "System" as username for clock)
	fax, err := faxmanager.SaveFax("🕐 Clock", timeStr, "", colorImg, monoImg)
	if err != nil {
		return fmt.Errorf("failed to save clock fax: %w", err)
	}

	// Save images to disk
	if err := saveFaxImages(fax, colorImg, monoImg); err != nil {
		return fmt.Errorf("failed to save clock fax images: %w", err)
	}

	// Broadcast to SSE clients
	broadcast.BroadcastFax(fax)

	// Add to print queue
	printQueue <- monoImg
	return nil
}

func PrintOut(userName string, message []twitch.ChatMessageFragment, timestamp time.Time) error {
	// Generate color version
	colorImg, err := MessageToImage(userName, message, true)
	if err != nil {
		return fmt.Errorf("failed to create color image: %w", err)
	}

	// Generate monochrome version for printing
	monoImg, err := MessageToImage(userName, message, false)
	if err != nil {
		return fmt.Errorf("failed to create monochrome image: %w", err)
	}

	// Extract message text from fragments
	messageText := ""
	for _, fragment := range message {
		if fragment.Type == "text" {
			messageText += fragment.Text
		}
	}

	// Save fax with faxmanager
	fax, err := faxmanager.SaveFax(userName, messageText, "", colorImg, monoImg)
	if err != nil {
		return fmt.Errorf("failed to save fax: %w", err)
	}

	// Save images to disk
	if err := saveFaxImages(fax, colorImg, monoImg); err != nil {
		return fmt.Errorf("failed to save fax images: %w", err)
	}

	// Broadcast to SSE clients
	broadcast.BroadcastFax(fax)

	// Add to print queue
	printQueue <- monoImg
	return nil
}

// PrintOutWithTitle sends fax output with separate title and details to printer and frontend
func PrintOutWithTitle(title, userName, extra, details string, timestamp time.Time) error {
	// Generate color version
	colorImg, err := MessageToImageWithTitle(title, userName, extra, details, true)
	if err != nil {
		return fmt.Errorf("failed to create color image: %w", err)
	}

	// Generate monochrome version for printing
	monoImg, err := MessageToImageWithTitle(title, userName, extra, details, false)
	if err != nil {
		return fmt.Errorf("failed to create monochrome image: %w", err)
	}

	// Create display text for fax manager
	messageText := title
	if extra != "" {
		messageText += "\n" + extra
	}
	if details != "" {
		messageText += "\n" + details
	}

	// Save fax with faxmanager
	fax, err := faxmanager.SaveFax(userName, messageText, "", colorImg, monoImg)
	if err != nil {
		return fmt.Errorf("failed to save fax: %w", err)
	}

	// Save images to disk
	if err := saveFaxImages(fax, colorImg, monoImg); err != nil {
		return fmt.Errorf("failed to save fax images: %w", err)
	}

	// Broadcast to SSE clients
	broadcast.BroadcastFax(fax)

	// Add to print queue
	printQueue <- monoImg
	return nil
}

// saveFaxImages saves the fax images to disk
func saveFaxImages(fax *faxmanager.Fax, colorImg, monoImg image.Image) error {
	// Save color image
	colorFile, err := os.Create(fax.ColorPath)
	if err != nil {
		return fmt.Errorf("failed to create color file: %w", err)
	}
	defer colorFile.Close()

	if err := png.Encode(colorFile, colorImg); err != nil {
		return fmt.Errorf("failed to encode color image: %w", err)
	}

	// Save mono image
	monoFile, err := os.Create(fax.MonoPath)
	if err != nil {
		return fmt.Errorf("failed to create mono file: %w", err)
	}
	defer monoFile.Close()

	if err := png.Encode(monoFile, monoImg); err != nil {
		return fmt.Errorf("failed to encode mono image: %w", err)
	}

	if shouldUseDryRun() {
		if env.Value.AutoDryRunWhenOffline && !status.IsStreamLive() {
			logger.Info("Fax images saved (AUTO DRY-RUN: STREAM OFFLINE)",
				zap.String("id", fax.ID),
				zap.String("colorPath", fax.ColorPath),
				zap.String("monoPath", fax.MonoPath))
		} else {
			logger.Info("Fax images saved (DRY-RUN MODE)",
				zap.String("id", fax.ID),
				zap.String("colorPath", fax.ColorPath),
				zap.String("monoPath", fax.MonoPath))
		}
	} else {
		logger.Info("Fax images saved",
			zap.String("id", fax.ID),
			zap.String("colorPath", fax.ColorPath),
			zap.String("monoPath", fax.MonoPath))
	}

	return nil
}


func clockRoutine() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	lastPrintedTime := ""
	lastMonth := time.Now().Format("2006-01")
	
	for range ticker.C {
		now := time.Now()
		minute := now.Minute()
		currentMonth := now.Format("2006-01")
		
		// Check if month has changed
		if currentMonth != lastMonth {
			logger.Info("Month changed", 
				zap.String("from", lastMonth), 
				zap.String("to", currentMonth))
			lastMonth = currentMonth
		}
		
		// Check if it's 0 minutes (on the hour)
		if minute == 0 {
			currentTimeStr := now.Format("15:04")
			
			// Avoid printing the same time multiple times
			if currentTimeStr != lastPrintedTime {
				lastPrintedTime = currentTimeStr
				
				logger.Info("Clock: printing time with latest leaderboard data", zap.String("time", currentTimeStr))
				
				// Use PrintClock to handle everything (generation, saving, broadcasting, and printing)
				if err := PrintClock(currentTimeStr); err != nil {
					logger.Error("Clock: failed to print clock", zap.Error(err))
				} else {
					logger.Info("Clock: successfully printed and broadcasted")
				}
			}
		}
	}
}




// keepAliveRoutine maintains printer connection
func keepAliveRoutine() {
	ticker := time.NewTicker(1 * time.Second) // Check every second
	defer ticker.Stop()
	
	for range ticker.C {
		// First check if we need to do initial connection
		if !IsConnected() && !HasInitialPrintBeenDone() {
			logger.Info("Keep-alive: attempting initial printer connection")
			
			// Lock printer for exclusive access
			printerMutex.Lock()
			
			// Setup printer if needed
			c, err := SetupPrinter()
			if err != nil {
				logger.Error("Keep-alive: failed to setup printer for initial connection", zap.Error(err))
				printerMutex.Unlock()
				continue
			}
			
			// Try to connect
			err = ConnectPrinter(c, *env.Value.PrinterAddress)
			if err != nil {
				logger.Error("Keep-alive: failed initial connection to printer", zap.Error(err))
				printerMutex.Unlock()
				continue
			}
			
			logger.Info("Keep-alive: initial connection established")
			
			// Mark initial print as done
			logger.Info("Keep-alive: marking initial print as done")
			MarkInitialPrintDone()
			
			// Update last print time
			lastPrintMutex.Lock()
			lastPrintTime = time.Now()
			lastPrintMutex.Unlock()
			
			printerMutex.Unlock()
			continue
		}
		
		lastPrintMutex.Lock()
		timeSinceLastPrint := time.Since(lastPrintTime)
		lastPrintMutex.Unlock()
		
		// If more than KeepAliveInterval seconds have passed since last print
		if timeSinceLastPrint > time.Duration(env.Value.KeepAliveInterval)*time.Second {
			logger.Info("Keep-alive: waiting for printer access", zap.Int("seconds_since_last_print", int(timeSinceLastPrint.Seconds())))
			
			// Lock printer for exclusive access
			printerMutex.Lock()
			
			logger.Info("Keep-alive: creating new connection")
			
			// Setup printer (will disconnect if connected)
			c, err := SetupPrinter()
			if err != nil {
				logger.Error("Keep-alive: failed to setup printer", zap.Error(err))
				printerMutex.Unlock()
				continue
			}
			
			err = ConnectPrinter(c, *env.Value.PrinterAddress)
			if err != nil {
				logger.Error("Keep-alive: failed to connect printer", zap.Error(err))
				printerMutex.Unlock()
				continue
			}
			
			logger.Info("Keep-alive: new connection established")
			
			// Mark initial print as done if not already done
			if !HasInitialPrintBeenDone() {
				logger.Info("Keep-alive: marking initial print as done after reconnection")
				MarkInitialPrintDone()
			}
			
			// Update last print time
			lastPrintMutex.Lock()
			lastPrintTime = time.Now()
			lastPrintMutex.Unlock()
			
			// Release printer lock
			printerMutex.Unlock()
		}
	}
}

// PrintInitialClock prints initial clock on startup
func PrintInitialClock() error {
	now := time.Now()
	currentTime := now.Format("15:04")
	logger.Info("Printing initial clock (simple)", zap.String("time", currentTime))
	
	// Generate simple time-only image
	img, err := GenerateTimeImageSimple(currentTime)
	if err != nil {
		return fmt.Errorf("failed to generate initial clock image: %w", err)
	}
	
	// Save image if debug output is enabled
	if env.Value.DebugOutput {
		outputDir := ".output"
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
		
		// Save time-only image
		monoPath := filepath.Join(outputDir, fmt.Sprintf("%s_initial_clock.png", now.Format("20060102_150405")))
		file, err := os.Create(monoPath)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer file.Close()
		if err := png.Encode(file, img); err != nil {
			return fmt.Errorf("failed to encode image: %w", err)
		}
		logger.Info("Initial clock: output file saved", zap.String("path", monoPath))
		
		// Return early when debug output is enabled (skip print queue)
		return nil
	}
	
	// Directly add to print queue without frontend notification
	// This is the only output that doesn't notify the frontend
	select {
	case printQueue <- img:
		logger.Info("Initial clock added to print queue (no frontend notification)")
	default:
		return fmt.Errorf("print queue is full")
	}
	
	return nil
}

// GetPrintQueueSize returns the current number of items in the print queue
func GetPrintQueueSize() int {
	return len(printQueue)
}
