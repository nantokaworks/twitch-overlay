package output

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/joeyak/go-twitch-eventsub/v3"
	"github.com/nantokaworks/twitch-fax/internal/env"
)

var printQueue chan image.Image

func init() {
	printQueue = make(chan image.Image, 100)
	go func() {
		for img := range printQueue {
			c, err := SetupPrinter()
			if err != nil {
				fmt.Printf("failed to setup printer: %v\n", err)
				continue
			}
			err = ConnectPrinter(c, *env.Value.PrinterAddress)
			if err != nil {
				fmt.Printf("failed to connect printer: %v\n", err)
				continue
			}

			if err := c.Print(img, opts, false); err != nil {
				fmt.Printf("failed to print: %v\n", err)
			}
		}
	}()
}

func PrintOut(userName string, message []twitch.ChatMessageFragment, timestamp time.Time) error {

	img, err := MessageToImage(userName, message)
	if err != nil {
		return fmt.Errorf("failed to create image: %w", err)
	}

	if env.Value.DebugOutput {
		outputDir := ".output"
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}

		filepath := filepath.Join(outputDir, fmt.Sprintf("%s_%s.png", timestamp.Format("20060102_150405_000"), userName))

		file, err := os.Create(filepath)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer file.Close()
		err = png.Encode(file, img)
		if err != nil {
			return fmt.Errorf("failed to encode image: %w", err)
		}
		fmt.Printf("output file: %s\n", filepath)
		return nil
	}

	printQueue <- img
	return nil
}
