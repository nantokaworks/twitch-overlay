package main

import (
	"log"

	"github.com/nantokaworks/twitch-fax/internal/output"
	"github.com/nantokaworks/twitch-fax/internal/shared/logger"
	"go.uber.org/zap"
)

func main() {

	// init output
	c, err := output.SetupPrinter()
	if err != nil {
		log.Fatal(err)
	}
	defer c.Stop()
	c.Debug.Log = true
	devices, err := output.FindAddress(c, "dummy")
	if err != nil {
		log.Fatal(err)
	}

	logger.Info("Found devices", zap.Int("devices", len(devices)))
	for k, v := range devices {
		logger.Info("Found device", zap.Int("device", k), zap.String("address", string(v)))
	}

}
