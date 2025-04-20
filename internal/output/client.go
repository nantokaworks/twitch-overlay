package output

import (
	"git.massivebox.net/massivebox/go-catprinter"
)

var latestPrinter *catprinter.Client
var opts *catprinter.PrinterOptions

func SetupPrinter() (*catprinter.Client, error) {
	if latestPrinter != nil {
		latestPrinter.Disconnect()
		latestPrinter = nil
	}

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

	err := c.Connect(address)
	if err != nil {
		return err
	}

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
