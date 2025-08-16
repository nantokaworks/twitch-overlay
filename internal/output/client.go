package output

import (
	"strings"
	"sync"
	"time"
	
	"git.massivebox.net/massivebox/go-catprinter"
	"github.com/nantokaworks/twitch-overlay/internal/env"
	"github.com/nantokaworks/twitch-overlay/internal/shared/logger"
	"github.com/nantokaworks/twitch-overlay/internal/status"
	"go.uber.org/zap"
)

// Printer singleton management
var (
	latestPrinter *catprinter.Client  // Single printer instance (protected by printerMutex)
	opts *catprinter.PrinterOptions
	isConnected bool
	connectionMutex sync.RWMutex  // 接続状態の読み書き用mutex
	keepAliveStopCh chan struct{}
	keepAliveRunning bool
	keepAliveMutex sync.Mutex     // KeepAlive goroutine管理用mutex
)

func SetupPrinter() (*catprinter.Client, error) {
	// 既存のクライアントがあれば再利用（BLEデバイスの再取得を避ける）
	if latestPrinter != nil {
		// 接続状態のみリセット
		connectionMutex.Lock()
		if isConnected {
			logger.Info("Reusing existing printer client, disconnecting current connection")
			latestPrinter.Disconnect()
			isConnected = false
			status.SetPrinterConnected(false)
		}
		connectionMutex.Unlock()
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
	
	// Don't rely on the isConnected flag - always try to connect
	// The printer might be disconnected even if the flag says connected
	
	// DRY-RUNモードでも実際のプリンターに接続
	if env.Value.DryRunMode {
		logger.Info("Connecting to printer in DRY-RUN mode", zap.String("address", address))
	} else {
		logger.Info("Connecting to printer", zap.String("address", address))
	}
	err := c.Connect(address)
	if err != nil {
		errStr := err.Error()
		
		// "already connected"の場合は成功として扱う
		if strings.Contains(errStr, "already connected") {
			logger.Debug("Printer is already connected", zap.String("address", address))
			connectionMutex.Lock()
			isConnected = true
			connectionMutex.Unlock()
			status.SetPrinterConnected(true)
			return nil
		}
		
		// エラーメッセージに"already exists"または"connection canceled"が含まれる場合は、プリンタークライアント全体を再作成
		if strings.Contains(errStr, "already exists") || strings.Contains(errStr, "connection canceled") || strings.Contains(errStr, "can't dial") {
			logger.Info("Connection error detected, recreating printer client", zap.String("error", errStr))
			
			// 完全にクリーンアップ
			// NOTE: printerMutex should be held by caller
			c.Disconnect()
			if latestPrinter != nil && latestPrinter != c {
				// Safety check: only stop if it's the same instance
				logger.Warn("latestPrinter and c are different instances, potential duplicate!")
			}
			if latestPrinter != nil {
				latestPrinter.Stop() // BLEデバイス解放
				latestPrinter = nil
			}
			connectionMutex.Lock()
			isConnected = false
			connectionMutex.Unlock()
			status.SetPrinterConnected(false)
			
			// 新しいクライアントを作成 (single instance)
			logger.Info("Creating new printer client for retry")
			newClient, err := catprinter.NewClient()
			if err != nil {
				logger.Error("Failed to create new printer client", zap.Error(err))
				return err
			}
			latestPrinter = newClient
			c = newClient // 引数も更新 (ensure using the same instance)
			
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
	connectionMutex.Lock()
	isConnected = true
	connectionMutex.Unlock()
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
	
	connectionMutex.RLock()
	connected := isConnected
	connectionMutex.RUnlock()
	
	if latestPrinter != nil && connected {
		// Disconnectが失敗してもプロセスを続行
		func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Warn("Recovered from panic during Disconnect", zap.Any("panic", r))
				}
			}()
			latestPrinter.Disconnect()
		}()
		connectionMutex.Lock()
		isConnected = false
		connectionMutex.Unlock()
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
		connectionMutex.RLock()
		connected := isConnected
		connectionMutex.RUnlock()
		
		if connected {
			// Disconnectが失敗してもプロセスを続行
			func() {
				defer func() {
					if r := recover(); r != nil {
						logger.Warn("Recovered from panic during Disconnect", zap.Any("panic", r))
					}
				}()
				latestPrinter.Disconnect()
			}()
			connectionMutex.Lock()
			isConnected = false
			connectionMutex.Unlock()
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
	connectionMutex.RLock()
	defer connectionMutex.RUnlock()
	return isConnected
}

// ResetConnectionStatus resets the connection status (for error recovery)
func ResetConnectionStatus() {
	connectionMutex.Lock()
	isConnected = false
	connectionMutex.Unlock()
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
	
	printerAddress := *env.Value.PrinterAddress
	
	// Get current connection status (but don't rely on it)
	connectionMutex.RLock()
	currentStatus := isConnected
	connectionMutex.RUnlock()
	
	logger.Info("[KeepAlive] Starting connection maintenance cycle", 
		zap.String("address", printerAddress), 
		zap.Bool("status_flag", currentStatus),
		zap.Time("timestamp", time.Now()))
	
	// Always try to ensure connection (don't trust the flag)
	// Setup printer if needed
	if latestPrinter == nil {
		logger.Info("[KeepAlive] Creating new printer client")
		_, err := SetupPrinter() // SetupPrinter already sets latestPrinter internally
		if err != nil {
			logger.Error("[KeepAlive] Failed to setup printer", 
				zap.Error(err),
				zap.Time("timestamp", time.Now()))
			ResetConnectionStatus()
			status.SetPrinterConnected(false)
			return
		}
		// latestPrinter is already set by SetupPrinter
	}
	
	// Try to connect (this will check if already connected internally)
	logger.Debug("[KeepAlive] Attempting to connect/verify connection")
	err := ConnectPrinter(latestPrinter, printerAddress)
	if err != nil {
		errStr := err.Error()
		
		// If already connected, connection is fine
		if strings.Contains(errStr, "already connected") {
			logger.Debug("[KeepAlive] Already connected, connection verified")
			connectionMutex.Lock()
			isConnected = true
			connectionMutex.Unlock()
			status.SetPrinterConnected(true)
			return
		}
		
		logger.Error("[KeepAlive] Connection failed", 
			zap.String("address", printerAddress), 
			zap.Error(err),
			zap.Time("timestamp", time.Now()))
		
		// Check for various Bluetooth-related errors
		if strings.Contains(errStr, "hci socket") || 
		   strings.Contains(errStr, "broken pipe") ||
		   strings.Contains(errStr, "connection reset") ||
		   strings.Contains(errStr, "device not found") ||
		   strings.Contains(errStr, "operation timed out") ||
		   strings.Contains(errStr, "input/output error") ||
		   strings.Contains(errStr, "connection canceled") ||
		   strings.Contains(errStr, "can't dial") {
			logger.Warn("[KeepAlive] Detected Bluetooth error, resetting BLE device", zap.String("error_type", errStr))
			
			// Reset BLE device
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
			
			// Wait before retry
			time.Sleep(1 * time.Second)
			
			// Try to create new client and connect again
			logger.Info("[KeepAlive] Creating new printer client after BLE reset")
			_, err := SetupPrinter() // SetupPrinter already sets latestPrinter internally
			if err != nil {
				logger.Error("[KeepAlive] Failed to setup new printer after reset", 
					zap.Error(err),
					zap.Time("timestamp", time.Now()))
				ResetConnectionStatus()
				status.SetPrinterConnected(false)
				return
			}
			// latestPrinter is already set by SetupPrinter
			
			// Final connection attempt
			logger.Debug("[KeepAlive] Final connection attempt after reset")
			err = ConnectPrinter(latestPrinter, printerAddress)
			if err != nil {
				logger.Error("[KeepAlive] Final connection attempt failed", 
					zap.String("address", printerAddress), 
					zap.Error(err),
					zap.Time("timestamp", time.Now()))
				ResetConnectionStatus()
				status.SetPrinterConnected(false)
			} else {
				logger.Info("[KeepAlive] Successfully connected after BLE reset", 
					zap.Time("timestamp", time.Now()))
			}
		} else {
			// Non-Bluetooth error
			ResetConnectionStatus()
			status.SetPrinterConnected(false)
		}
	} else {
		logger.Info("[KeepAlive] Connection established/verified successfully", 
			zap.Time("timestamp", time.Now()))
	}
}

// StopKeepAlive stops the keep-alive goroutine
func StopKeepAlive() {
	keepAliveMutex.Lock()
	defer keepAliveMutex.Unlock()
	
	if keepAliveRunning && keepAliveStopCh != nil {
		logger.Info("[KeepAlive] Stopping keep-alive goroutine", 
			zap.Time("timestamp", time.Now()))
		close(keepAliveStopCh)
		keepAliveRunning = false
		// Wait longer for goroutine to stop
		time.Sleep(500 * time.Millisecond)
	}
}

// StartKeepAlive starts the keep-alive goroutine
func StartKeepAlive() {
	logger.Info("[StartKeepAlive] Function called",
		zap.Bool("env.KeepAliveEnabled", env.Value.KeepAliveEnabled),
		zap.Int("env.KeepAliveInterval", env.Value.KeepAliveInterval),
		zap.String("env.PrinterAddress", func() string {
			if env.Value.PrinterAddress != nil {
				return *env.Value.PrinterAddress
			}
			return "<not set>"
		}()))
	
	// Stop existing goroutine if running
	StopKeepAlive()
	
	keepAliveMutex.Lock()
	defer keepAliveMutex.Unlock()
	
	if !env.Value.KeepAliveEnabled {
		logger.Warn("[StartKeepAlive] Keep-alive is DISABLED in configuration, not starting goroutine")
		return
	}
	
	// Get the interval from environment variable (default to 60 seconds if not set or invalid)
	interval := 60
	if env.Value.KeepAliveInterval > 0 {
		interval = env.Value.KeepAliveInterval
	}
	
	logger.Info("[StartKeepAlive] Starting keep-alive goroutine", 
		zap.Int("interval_seconds", interval),
		zap.Time("timestamp", time.Now()))
	keepAliveStopCh = make(chan struct{})
	keepAliveRunning = true
	
	go func() {
		// Perform connection maintenance once at startup
		logger.Info("[KeepAlive Goroutine] Started successfully, performing initial connection maintenance",
			zap.Time("startup_time", time.Now()))
		MaintainPrinterConnection()
		
		// Create ticker with the configured interval
		ticker := time.NewTicker(time.Duration(interval) * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				// Perform periodic connection maintenance
				logger.Debug("[KeepAlive] Timer triggered, performing periodic maintenance")
				MaintainPrinterConnection()
			case <-keepAliveStopCh:
				logger.Info("[KeepAlive] Goroutine stopped by stop channel", 
					zap.Time("timestamp", time.Now()))
				return
			}
		}
	}()
}
