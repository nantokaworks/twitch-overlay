package output

import (
	"strings"
	
	"git.massivebox.net/massivebox/go-catprinter"
	"github.com/nantokaworks/twitch-fax/internal/env"
	"github.com/nantokaworks/twitch-fax/internal/shared/logger"
	"github.com/nantokaworks/twitch-fax/internal/status"
	"go.uber.org/zap"
)

var latestPrinter *catprinter.Client
var opts *catprinter.PrinterOptions
var isConnected bool
var hasInitialPrintBeenDone bool

// formatAddress formats the address to match the error message format
func formatAddress(address string) string {
	// アドレスにハイフンが含まれていない場合、追加する
	// 58c122f9faa1484624a37d6858cbbc5b -> 58c122f9-faa1-4846-24a3-7d6858cbbc5b
	if !strings.Contains(address, "-") && len(address) == 32 {
		return address[:8] + "-" + address[8:12] + "-" + address[12:16] + "-" + address[16:20] + "-" + address[20:]
	}
	return address
}

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
	}

	logger.Info("Connecting to printer", zap.String("address", address))
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

// Stop gracefully disconnects the printer and releases BLE device
func Stop() {
	if latestPrinter != nil {
		if isConnected {
			latestPrinter.Disconnect()
			isConnected = false
			status.SetPrinterConnected(false)
		}
		// Stop()を呼ぶとBLEデバイスも解放される
		latestPrinter.Stop()
		latestPrinter = nil
		logger.Info("Printer client stopped and BLE device released")
	}
}

// MarkInitialPrintDone marks that the initial print has been completed
func MarkInitialPrintDone() {
	hasInitialPrintBeenDone = true
}

// IsConnected returns whether the printer is connected
func IsConnected() bool {
	return isConnected
}

// ResetConnectionStatus resets the connection status (for error recovery)
func ResetConnectionStatus() {
	isConnected = false
}

// HasInitialPrintBeenDone returns whether the initial print has been done
func HasInitialPrintBeenDone() bool {
	return hasInitialPrintBeenDone
}
