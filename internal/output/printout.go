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

			// Check for dry-run mode
			if *env.Value.PrinterAddress == "dry-run-mode" {
				logger.Info("Dry-run mode: skipping actual printing")
				// Update last print time even in dry-run mode
				lastPrintMutex.Lock()
				lastPrintTime = time.Now()
				lastPrintMutex.Unlock()
			} else {
				if err := latestPrinter.Print(img, opts, false); err != nil {
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

func PrintOut(userName string, message []twitch.ChatMessageFragment, timestamp time.Time) error {
	// Generate color version if debug output is enabled
	var colorImg image.Image
	if env.Value.DebugOutput {
		var err error
		colorImg, err = MessageToImage(userName, message, true)
		if err != nil {
			logger.Error("Failed to create color image", zap.Error(err))
			colorImg = nil
		}
	}

	// Generate monochrome version for printing
	img, err := MessageToImage(userName, message, false)
	if err != nil {
		return fmt.Errorf("failed to create image: %w", err)
	}

	if env.Value.DebugOutput {
		outputDir := ".output"
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}

		// Save color version if available
		if colorImg != nil {
			colorPath := filepath.Join(outputDir, fmt.Sprintf("%s_%s_color.png", timestamp.Format("20060102_150405_000"), userName))
			file, err := os.Create(colorPath)
			if err != nil {
				logger.Error("Failed to create color output file", zap.Error(err))
			} else {
				if err := png.Encode(file, colorImg); err != nil {
					logger.Error("Failed to encode color image", zap.Error(err))
				} else {
					logger.Info("Color output file saved", zap.String("path", colorPath))
				}
				file.Close()
			}
		}

		// Save monochrome version
		monoPath := filepath.Join(outputDir, fmt.Sprintf("%s_%s.png", timestamp.Format("20060102_150405_000"), userName))
		file, err := os.Create(monoPath)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer file.Close()
		err = png.Encode(file, img)
		if err != nil {
			return fmt.Errorf("failed to encode image: %w", err)
		}
		if *env.Value.PrinterAddress == "dry-run-mode" {
			logger.Info("output file saved (DRY-RUN MODE)", zap.String("path", monoPath))
		} else {
			logger.Info("output file saved", zap.String("path", monoPath))
		}
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
				// Only disconnect if we have a real connection (not in dry-run mode)
				if *env.Value.PrinterAddress != "dry-run-mode" {
					latestPrinter.Disconnect()
				}
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
				
				logger.Info("Clock: printing time with latest leaderboard data", zap.String("time", currentTimeStr))
				
				// Generate time image with channel stats
				// Note: リーダーボードデータは GenerateTimeImageWithStats 内で毎回最新を取得
				// First generate color version if debug output is enabled
				var colorImg image.Image
				if env.Value.DebugOutput {
					var err error
					colorImg, err = GenerateTimeImageWithStatsColor(currentTimeStr)
					if err != nil {
						logger.Error("Clock: failed to generate color time image", zap.Error(err))
						colorImg = nil
					}
				}
				
				// Generate monochrome version for printing
				img, err := GenerateTimeImageWithStats(currentTimeStr)
				if err != nil {
					logger.Error("Clock: failed to generate time image", zap.Error(err))
					continue
				}
				
				// Save images if debug output is enabled
				if env.Value.DebugOutput {
					outputDir := ".output"
					if err := os.MkdirAll(outputDir, 0755); err != nil {
						logger.Error("Clock: failed to create output directory", zap.Error(err))
					} else {
						// Save color version if available
						if colorImg != nil {
							colorPath := filepath.Join(outputDir, fmt.Sprintf("%s_clock_color.png", now.Format("20060102_150405")))
							file, err := os.Create(colorPath)
							if err != nil {
								logger.Error("Clock: failed to create color output file", zap.Error(err))
							} else {
								if err := png.Encode(file, colorImg); err != nil {
									logger.Error("Clock: failed to encode color image", zap.Error(err))
								} else {
									logger.Info("Clock: color output file saved", zap.String("path", colorPath))
								}
								file.Close()
							}
						}
						
						// Save monochrome version
						monoPath := filepath.Join(outputDir, fmt.Sprintf("%s_clock.png", now.Format("20060102_150405")))
						file, err := os.Create(monoPath)
						if err != nil {
							logger.Error("Clock: failed to create output file", zap.Error(err))
						} else {
							if err := png.Encode(file, img); err != nil {
								logger.Error("Clock: failed to encode image", zap.Error(err))
							} else {
								logger.Info("Clock: output file saved", zap.String("path", monoPath))
							}
							file.Close()
						}
					}
					// Skip adding to print queue when debug output is enabled
					continue
				}
				
				// Add to print queue only when debug output is disabled
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
	now := time.Now()
	currentTime := now.Format("15:04")
	logger.Info("Printing initial clock and stats", zap.String("time", currentTime))
	
	// Generate color version if debug output is enabled
	var colorImg image.Image
	if env.Value.DebugOutput {
		var err error
		colorImg, err = GenerateTimeImageWithStatsColor(currentTime)
		if err != nil {
			logger.Error("Initial clock: failed to generate color image", zap.Error(err))
			colorImg = nil
		}
	}
	
	// Generate monochrome version for printing
	img, err := GenerateTimeImageWithStats(currentTime)
	if err != nil {
		return fmt.Errorf("failed to generate initial clock image: %w", err)
	}
	
	// Save images if debug output is enabled
	if env.Value.DebugOutput {
		outputDir := ".output"
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
		
		// Save color version if available
		if colorImg != nil {
			colorPath := filepath.Join(outputDir, fmt.Sprintf("%s_initial_clock_color.png", now.Format("20060102_150405")))
			file, err := os.Create(colorPath)
			if err != nil {
				logger.Error("Initial clock: failed to create color output file", zap.Error(err))
			} else {
				if err := png.Encode(file, colorImg); err != nil {
					logger.Error("Initial clock: failed to encode color image", zap.Error(err))
				} else {
					logger.Info("Initial clock: color output file saved", zap.String("path", colorPath))
				}
				file.Close()
			}
		}
		
		// Save monochrome version
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
	
	// Add to print queue only when debug output is disabled
	select {
	case printQueue <- img:
		logger.Info("Initial clock and stats added to print queue")
	default:
		return fmt.Errorf("print queue is full")
	}
	
	return nil
}
