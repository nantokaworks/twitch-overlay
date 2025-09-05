package output

import (
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
var isReconnecting bool
var hasInitialPrintBeenDone bool

func SetupPrinter() (*catprinter.Client, error) {
	// 既存のクライアントがある場合は完全リセット（真のKeepAliveのため）
	if latestPrinter != nil {
		logger.Info("Resetting printer client for proper keep-alive")
		
		// 再接続中フラグを立てる
		isReconnecting = true
		
		// 既存の接続を切断
		if isConnected {
			logger.Info("Disconnecting existing connection")
			latestPrinter.Disconnect()
			isConnected = false
			// 再接続中はステータスを変更しない（接続中を維持）
			// status.SetPrinterConnected(false) を呼ばない
		}
		
		// BLEデバイスを完全に解放
		logger.Info("Releasing BLE device")
		latestPrinter.Stop()
		latestPrinter = nil
		
		// Bluetoothリソースの解放を待つ
		// Note: この待機時間により、BLEデバイスが完全に解放される
		time.Sleep(500 * time.Millisecond)
	} else {
		// 新規接続の場合は再接続フラグをクリア
		isReconnecting = false
	}

	// 新規クライアント作成
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
	
	// Skip if already connected (and not reconnecting)
	if isConnected && !isReconnecting {
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
		// 接続失敗時、再接続中でなければステータスを更新
		if !isReconnecting {
			status.SetPrinterConnected(false)
		}
		return err
	}
	
	logger.Info("Successfully connected to printer", zap.String("address", address))
	isConnected = true
	
	// 再接続が完了したらフラグをクリア
	isReconnecting = false
	
	// 常にステータスを更新（再接続完了時も含む）
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
		isReconnecting = false  // 再接続フラグもクリア
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

// HasInitialPrintBeenDone returns whether the initial print has been done
func HasInitialPrintBeenDone() bool {
	return hasInitialPrintBeenDone
}