package status

import (
	"sync"
	"github.com/nantokaworks/twitch-overlay/internal/broadcast"
)

var (
	mu                sync.RWMutex
	printerConnected  bool
)

// SetPrinterConnected sets the printer connection status
func SetPrinterConnected(connected bool) {
	mu.Lock()
	previousStatus := printerConnected
	printerConnected = connected
	mu.Unlock()
	
	// 状態が変更された場合はSSEで通知
	if previousStatus != connected {
		eventType := "printer_disconnected"
		if connected {
			eventType = "printer_connected"
		}
		
		broadcast.Send(map[string]interface{}{
			"type": eventType,
			"data": map[string]interface{}{
				"connected": connected,
			},
		})
	}
}

// IsPrinterConnected returns the printer connection status
func IsPrinterConnected() bool {
	mu.RLock()
	defer mu.RUnlock()
	return printerConnected
}