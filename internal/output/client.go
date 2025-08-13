package output

import (
	"strings"
	"time"
	
	"git.massivebox.net/massivebox/go-catprinter"
	"github.com/nantokaworks/twitch-overlay/internal/env"
	"github.com/nantokaworks/twitch-overlay/internal/shared/logger"
	"github.com/nantokaworks/twitch-overlay/internal/status"
	"go.uber.org/zap"
)

var latestPrinter *catprinter.Client
var opts *catprinter.PrinterOptions
var isConnected bool
var keepAliveStopCh chan struct{}
var keepAliveRunning bool

func SetupPrinter() (*catprinter.Client, error) {
	// 既存のクライアントがあれば再利用（BLEデバイスの再取得を避ける）
	if latestPrinter != nil {
		// 接続状態のみリセット
		if isConnected {
			logger.Info("Reusing existing printer client, disconnecting current connection")
			latestPrinter.Disconnect()
			isConnected = false
			status.SetPrinterConnected(false)
		}
		return latestPrinter, nil
	}

	// 初回のみ新規作成
	logger.Info("Creating new printer client")
	instance, err := catprinter.NewClient()
	if err != nil {
		return nil, err
	}
	latestPrinter = instance
	return instance, nil
}

func ConnectPrinter(c *catprinter.Client, address string) error {
	if c == nil {
		return nil
	}
	
	// Skip if already connected
	if isConnected {
		return nil
	}

	// DRY-RUNモードでも実際のプリンターに接続
	if env.Value.DryRunMode {
		logger.Info("Connecting to printer in DRY-RUN mode", zap.String("address", address))
	} else {
		logger.Info("Connecting to printer", zap.String("address", address))
	}
	err := c.Connect(address)
	if err != nil {
		// エラーメッセージに"already exists"が含まれる場合は、プリンタークライアント全体を再作成
		errStr := err.Error()
		if strings.Contains(errStr, "already exists") {
			logger.Info("Connection already exists, recreating printer client", zap.String("error", errStr))
			
			// 完全にクリーンアップ
			c.Disconnect()
			if latestPrinter != nil {
				latestPrinter.Stop() // BLEデバイス解放
				latestPrinter = nil
			}
			isConnected = false
			status.SetPrinterConnected(false)
			
			// 新しいクライアントを作成
			logger.Info("Creating new printer client for retry")
			newClient, err := catprinter.NewClient()
			if err != nil {
				logger.Error("Failed to create new printer client", zap.Error(err))
				return err
			}
			latestPrinter = newClient
			c = newClient // 引数も更新
			
			// 新しいクライアントで接続を試みる
			logger.Info("Attempting connection with new client")
			err = c.Connect(address)
			if err != nil {
				logger.Error("Failed to connect with new client", zap.Error(err))
				return err
			}
		} else {
			return err
		}
	}
	logger.Info("Successfully connected to printer", zap.String("address", address))
	isConnected = true
	status.SetPrinterConnected(true)

	return nil
}

func SetupPrinterOptions(bestQuality, dither, autoRotate bool, blackPoint float32) error {
	// Set up the printer options
	opts = catprinter.NewOptions().
		SetBestQuality(bestQuality).
		SetDither(dither).
		SetAutoRotate(autoRotate).
		SetBlackPoint(float32(blackPoint))

	return nil
}

// Disconnect disconnects from the printer but keeps the BLE device
func Disconnect() {
	// nilチェックとパニック対策
	defer func() {
		if r := recover(); r != nil {
			logger.Error("Recovered from panic in Disconnect()", zap.Any("panic", r))
		}
	}()
	
	if latestPrinter != nil && isConnected {
		// Disconnectが失敗してもプロセスを続行
		func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Warn("Recovered from panic during Disconnect", zap.Any("panic", r))
				}
			}()
			latestPrinter.Disconnect()
		}()
		isConnected = false
		status.SetPrinterConnected(false)
		logger.Info("Printer disconnected (BLE device kept)")
	}
}

// Stop gracefully disconnects the printer and releases BLE device
func Stop() {
	// nilチェックとパニック対策
	defer func() {
		if r := recover(); r != nil {
			logger.Error("Recovered from panic in Stop()", zap.Any("panic", r))
		}
	}()
	
	if latestPrinter != nil {
		if isConnected {
			// Disconnectが失敗してもプロセスを続行
			func() {
				defer func() {
					if r := recover(); r != nil {
						logger.Warn("Recovered from panic during Disconnect", zap.Any("panic", r))
					}
				}()
				latestPrinter.Disconnect()
			}()
			isConnected = false
			status.SetPrinterConnected(false)
		}
		// Stop()を呼ぶとBLEデバイスも解放される
		func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Warn("Recovered from panic during Stop", zap.Any("panic", r))
				}
			}()
			latestPrinter.Stop()
		}()
		latestPrinter = nil
		logger.Info("Printer client stopped and BLE device released")
	}
}

// IsConnected returns whether the printer is connected
func IsConnected() bool {
	return isConnected
}

// ResetConnectionStatus resets the connection status (for error recovery)
func ResetConnectionStatus() {
	isConnected = false
}

// MaintainPrinterConnection performs a single connection maintenance cycle
// This is used both at startup and in the periodic loop
func MaintainPrinterConnection() {
	// Lock printer for exclusive access
	printerMutex.Lock()
	defer printerMutex.Unlock()
	
	// Check if printer address is configured
	if env.Value.PrinterAddress == nil || *env.Value.PrinterAddress == "" {
		logger.Debug("Printer address not configured, skipping connection maintenance")
		return
	}
	
	if isConnected {
		// Already connected: disconnect and reconnect to refresh the connection
		logger.Info("Keep-alive: refreshing existing connection")
		
		// Disconnect current connection
		if latestPrinter != nil {
			func() {
				defer func() {
					if r := recover(); r != nil {
						logger.Warn("Recovered from panic during disconnect", zap.Any("panic", r))
					}
				}()
				latestPrinter.Disconnect()
			}()
			isConnected = false
			status.SetPrinterConnected(false)
		}
		
		// Wait a moment before reconnecting
		time.Sleep(500 * time.Millisecond)
		
		// Setup printer if needed
		if latestPrinter == nil {
			c, err := SetupPrinter()
			if err != nil {
				logger.Error("Keep-alive: failed to setup printer", zap.Error(err))
				return
			}
			latestPrinter = c
		}
		
		// Reconnect
		err := ConnectPrinter(latestPrinter, *env.Value.PrinterAddress)
		if err != nil {
			logger.Error("Keep-alive: failed to reconnect", zap.Error(err))
			
			// On HCI socket error, reset BLE device
			if strings.Contains(err.Error(), "hci socket") || strings.Contains(err.Error(), "broken pipe") {
				logger.Warn("Detected HCI socket error, resetting BLE device")
				if latestPrinter != nil {
					func() {
						defer func() {
							if r := recover(); r != nil {
								logger.Warn("Recovered from panic during BLE reset", zap.Any("panic", r))
							}
						}()
						latestPrinter.Stop()
					}()
					latestPrinter = nil
				}
			}
			ResetConnectionStatus()
			status.SetPrinterConnected(false)
		} else {
			logger.Info("Keep-alive: connection refreshed successfully")
		}
	} else {
		// Not connected: try to connect
		logger.Info("Keep-alive: attempting to connect")
		
		// Setup printer if needed
		if latestPrinter == nil {
			c, err := SetupPrinter()
			if err != nil {
				logger.Error("Keep-alive: failed to setup printer", zap.Error(err))
				return
			}
			latestPrinter = c
		}
		
		// Try to connect
		err := ConnectPrinter(latestPrinter, *env.Value.PrinterAddress)
		if err != nil {
			logger.Error("Keep-alive: failed to connect", zap.Error(err))
			
			// On HCI socket error, reset BLE device
			if strings.Contains(err.Error(), "hci socket") || strings.Contains(err.Error(), "broken pipe") {
				logger.Warn("Detected HCI socket error, resetting BLE device")
				if latestPrinter != nil {
					func() {
						defer func() {
							if r := recover(); r != nil {
								logger.Warn("Recovered from panic during BLE reset", zap.Any("panic", r))
							}
						}()
						latestPrinter.Stop()
					}()
					latestPrinter = nil
				}
			}
			ResetConnectionStatus()
			status.SetPrinterConnected(false)
		} else {
			logger.Info("Keep-alive: connected successfully")
		}
	}
	
	// Update last print time
	lastPrintMutex.Lock()
	lastPrintTime = time.Now()
	lastPrintMutex.Unlock()
}

// StopKeepAlive stops the keep-alive goroutine
func StopKeepAlive() {
	if keepAliveRunning && keepAliveStopCh != nil {
		logger.Info("Stopping keep-alive goroutine")
		close(keepAliveStopCh)
		keepAliveRunning = false
		// Wait a moment for goroutine to stop
		time.Sleep(100 * time.Millisecond)
	}
}

// StartKeepAlive starts the keep-alive goroutine
func StartKeepAlive() {
	// Stop existing goroutine if running
	StopKeepAlive()
	
	if !env.Value.KeepAliveEnabled {
		logger.Info("Keep-alive is disabled in configuration")
		return
	}
	
	logger.Info("Starting keep-alive goroutine")
	keepAliveStopCh = make(chan struct{})
	keepAliveRunning = true
	
	go func() {
		// Perform connection maintenance once at startup
		MaintainPrinterConnection()
		
		// Create ticker for 60 second intervals
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				// Perform periodic connection maintenance
				MaintainPrinterConnection()
			case <-keepAliveStopCh:
				logger.Info("Keep-alive goroutine stopped")
				return
			}
		}
	}()
}
