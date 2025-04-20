package output

import (
	"errors"
	"fmt"
	"time"

	"git.massivebox.net/massivebox/go-catprinter"
	"github.com/nantokaworks/twitch-fax/internal/shared/logger"
	"go.uber.org/zap"
)

func FindAddress(c *catprinter.Client, name string) (string, error) {
	fmt.Printf("Finding MAC by name (will take %d seconds)...", c.Timeout/time.Second)

	devices, err := c.ScanDevices(name)
	if err != nil {
		return "", err
	}
	switch len(devices) {
	case 0:
		return "", errors.New("no devices found with name " + name)
	case 1:
		for k, _ := range devices {
			return k, nil
		}
	default:
		logger.Info("Found multiple devices", zap.Int("devices", len(devices)))
		for m, n := range devices {
			logger.Info("Found device", zap.String("name", m), zap.String("address", string(n)))
		}
		return "", errors.New("multiple devices found with name " + name + ", please specify MAC directly")
	}
	return "", nil
}
