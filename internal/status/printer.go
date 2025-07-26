package status

import "sync"

var (
	mu                sync.RWMutex
	printerConnected  bool
)

// SetPrinterConnected sets the printer connection status
func SetPrinterConnected(connected bool) {
	mu.Lock()
	defer mu.Unlock()
	printerConnected = connected
}

// IsPrinterConnected returns the printer connection status
func IsPrinterConnected() bool {
	mu.RLock()
	defer mu.RUnlock()
	return printerConnected
}