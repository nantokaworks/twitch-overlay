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
	"github.com/nantokaworks/twitch-fax/internal/env"
	"github.com/nantokaworks/twitch-fax/internal/shared/logger"
	"go.uber.org/zap"
)

var printQueue chan image.Image
var lastPrintTime time.Time
var lastPrintMutex sync.Mutex
var printerMutex sync.Mutex

func init() {
	printQueue = make(chan image.Image, 100)
	
	// Initialize last print time to now
	lastPrintTime = time.Now()
	
	// Start keep-alive goroutine if enabled
	if env.Value.KeepAliveEnabled {
		go keepAliveRoutine()
	}
	
	// Start clock routine
	if env.Value.ClockEnabled {
		go clockRoutine()
	}
	
	go func() {
		for img := range printQueue {
			// Lock printer for exclusive access
			printerMutex.Lock()
			
			// Use existing client or create new one
			if latestPrinter == nil {
				_, err := SetupPrinter()
				if err != nil {
					logger.Error("failed to setup printer", zap.Error(err))
					printerMutex.Unlock()
					continue
				}
			}
			
			// Try to connect if not connected
			err := ConnectPrinter(latestPrinter, *env.Value.PrinterAddress)
			if err != nil {
				logger.Error("failed to connect printer", zap.Error(err))
				// Try to create new client
				_, err := SetupPrinter()
				if err != nil {
					logger.Error("failed to setup new printer", zap.Error(err))
					printerMutex.Unlock()
					continue
				}
				err = ConnectPrinter(latestPrinter, *env.Value.PrinterAddress)
				if err != nil {
					logger.Error("failed to connect new printer", zap.Error(err))
					printerMutex.Unlock()
					continue
				}
			}

			if err := latestPrinter.Print(img, opts, false); err != nil {
				logger.Error("failed to print", zap.Error(err))
			} else {
				// Update last print time on successful print
				lastPrintMutex.Lock()
				lastPrintTime = time.Now()
				lastPrintMutex.Unlock()
			}
			
			// Release printer lock
			printerMutex.Unlock()
		}
	}()
}

func PrintOut(userName string, message []twitch.ChatMessageFragment, timestamp time.Time) error {

	img, err := MessageToImage(userName, message)
	if err != nil {
		return fmt.Errorf("failed to create image: %w", err)
	}

	if env.Value.DebugOutput {
		outputDir := ".output"
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}

		filepath := filepath.Join(outputDir, fmt.Sprintf("%s_%s.png", timestamp.Format("20060102_150405_000"), userName))

		file, err := os.Create(filepath)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer file.Close()
		err = png.Encode(file, img)
		if err != nil {
			return fmt.Errorf("failed to encode image: %w", err)
		}
		logger.Info("output file saved", zap.String("path", filepath))
		return nil
	}

	printQueue <- img
	return nil
}

func keepAliveRoutine() {
	ticker := time.NewTicker(1 * time.Second) // Check every second
	defer ticker.Stop()
	
	for range ticker.C {
		lastPrintMutex.Lock()
		timeSinceLastPrint := time.Since(lastPrintTime)
		lastPrintMutex.Unlock()
		
		// If more than KeepAliveInterval seconds have passed since last print
		if timeSinceLastPrint > time.Duration(env.Value.KeepAliveInterval)*time.Second {
			logger.Info("Keep-alive: waiting for printer access", zap.Int("seconds_since_last_print", int(timeSinceLastPrint.Seconds())))
			
			// Lock printer for exclusive access
			printerMutex.Lock()
			
			logger.Info("Keep-alive: creating new connection")
			
			// Disconnect existing client if any
			if latestPrinter != nil {
				latestPrinter.Disconnect()
				isConnected = false
			}
			
			// Create new client and connect
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
			
			// Update last print time
			lastPrintMutex.Lock()
			lastPrintTime = time.Now()
			lastPrintMutex.Unlock()
			
			// Release printer lock
			printerMutex.Unlock()
		}
	}
}

func clockRoutine() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	lastPrintedTime := ""
	
	for range ticker.C {
		now := time.Now()
		minute := now.Minute()
		
		// Check if it's 0 or 30 minutes
		if minute == 0 || minute == 30 {
			currentTimeStr := now.Format("15:04")
			
			// Avoid printing the same time multiple times
			if currentTimeStr != lastPrintedTime {
				lastPrintedTime = currentTimeStr
				
				logger.Info("Clock: printing time", zap.String("time", currentTimeStr))
				
				// Generate time image with channel stats
				img, err := generateTimeImageWithStats(currentTimeStr)
				if err != nil {
					logger.Error("Clock: failed to generate time image", zap.Error(err))
					continue
				}
				
				// Add to print queue
				select {
				case printQueue <- img:
					logger.Info("Clock: time added to print queue")
				default:
					logger.Warn("Clock: print queue is full, skipping time print")
				}
			}
		}
	}
}

// PrintInitialClockAndStats prints current time and stats on startup
func PrintInitialClockAndStats() error {
	currentTime := time.Now().Format("15:04")
	logger.Info("Printing initial clock and stats", zap.String("time", currentTime))
	
	// Generate image with stats
	img, err := generateTimeImageWithStats(currentTime)
	if err != nil {
		return fmt.Errorf("failed to generate initial clock image: %w", err)
	}
	
	// Add to print queue
	select {
	case printQueue <- img:
		logger.Info("Initial clock and stats added to print queue")
	default:
		return fmt.Errorf("print queue is full")
	}
	
	return nil
}
