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
	"github.com/nantokaworks/twitch-fax/internal/faxmanager"
	"github.com/nantokaworks/twitch-fax/internal/shared/logger"
	"github.com/nantokaworks/twitch-fax/internal/webserver"
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
			
			// Check if initial print hasn't been done yet and we have a connection
			if !hasInitialPrintBeenDone && isConnected && env.Value.ClockEnabled {
				logger.Info("Performing initial clock print on first successful connection")
				go func() {
					if env.Value.DryRunMode {
						logger.Info("Printing initial clock and stats (DRY-RUN MODE)")
					} else {
						logger.Info("Printing initial clock and stats")
					}
					err := PrintInitialClockAndStats()
					if err != nil {
						logger.Error("Failed to print initial clock and stats", zap.Error(err))
					} else {
						hasInitialPrintBeenDone = true
					}
				}()
			}

			// Check for dry-run mode
			if env.Value.DryRunMode {
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

	// Save fax with faxmanager
	fax, err := faxmanager.SaveFax(userName, colorImg, monoImg)
	if err != nil {
		return fmt.Errorf("failed to save fax: %w", err)
	}

	// Save images to disk
	if err := saveFaxImages(fax, colorImg, monoImg); err != nil {
		return fmt.Errorf("failed to save fax images: %w", err)
	}

	// Broadcast to SSE clients
	webserver.BroadcastFax(fax)

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

	if env.Value.DryRunMode {
		logger.Info("Fax images saved (DRY-RUN MODE)",
			zap.String("id", fax.ID),
			zap.String("colorPath", fax.ColorPath),
			zap.String("monoPath", fax.MonoPath))
	} else {
		logger.Info("Fax images saved",
			zap.String("id", fax.ID),
			zap.String("colorPath", fax.ColorPath),
			zap.String("monoPath", fax.MonoPath))
	}

	return nil
}

func keepAliveRoutine() {
	ticker := time.NewTicker(1 * time.Second) // Check every second
	defer ticker.Stop()
	
	for range ticker.C {
		// First check if we need to do initial connection
		if !isConnected && !hasInitialPrintBeenDone {
			logger.Info("Keep-alive: attempting initial printer connection")
			
			// Lock printer for exclusive access
			printerMutex.Lock()
			
			// Setup printer if needed
			if latestPrinter == nil {
				_, err := SetupPrinter()
				if err != nil {
					logger.Error("Keep-alive: failed to setup printer for initial connection", zap.Error(err))
					printerMutex.Unlock()
					continue
				}
			}
			
			// Try to connect
			err := ConnectPrinter(latestPrinter, *env.Value.PrinterAddress)
			if err != nil {
				logger.Error("Keep-alive: failed initial connection to printer", zap.Error(err))
				printerMutex.Unlock()
				continue
			}
			
			logger.Info("Keep-alive: initial connection established")
			
			// Perform initial print if clock is enabled
			if env.Value.ClockEnabled {
				logger.Info("Keep-alive: performing initial clock print")
				if env.Value.DryRunMode {
					logger.Info("Printing initial clock and stats (DRY-RUN MODE)")
				} else {
					logger.Info("Printing initial clock and stats")
				}
				err := PrintInitialClockAndStats()
				if err != nil {
					logger.Error("Keep-alive: failed to print initial clock and stats", zap.Error(err))
				} else {
					hasInitialPrintBeenDone = true
				}
			}
			
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
			
			// Disconnect existing client if any
			if latestPrinter != nil {
				// Disconnect the printer
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
			
			// Check if initial print hasn't been done yet after successful reconnection
			if !hasInitialPrintBeenDone && isConnected && env.Value.ClockEnabled {
				logger.Info("Keep-alive: performing initial clock print on first successful connection")
				if env.Value.DryRunMode {
					logger.Info("Printing initial clock and stats (DRY-RUN MODE)")
				} else {
					logger.Info("Printing initial clock and stats")
				}
				err := PrintInitialClockAndStats()
				if err != nil {
					logger.Error("Keep-alive: failed to print initial clock and stats", zap.Error(err))
				} else {
					hasInitialPrintBeenDone = true
				}
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
