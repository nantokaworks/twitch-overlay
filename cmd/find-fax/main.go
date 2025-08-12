package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nantokaworks/twitch-fax/internal/output"
	"github.com/nantokaworks/twitch-fax/internal/shared/logger"
	"go.uber.org/zap"
)

func main() {
	var outputFile string
	var rawMode bool
	flag.StringVar(&outputFile, "o", "", "Output file path to save the device list")
	flag.BoolVar(&rawMode, "raw", false, "Output raw scan data (includes all BLE scan logs)")
	flag.Parse()

	// init output
	c, err := output.SetupPrinter()
	if err != nil {
		log.Fatal(err)
	}
	defer c.Stop()
	c.Debug.Log = true
	
	// タイムアウトを設定
	c.Timeout = 10 * time.Second
	
	// テキスト出力用の文字列を作成
	var outputText string
	outputText += fmt.Sprintf("FAX Device Scanner - %s\n", time.Now().Format("2006-01-02 15:04:05"))
	outputText += fmt.Sprintf("==========================================\n\n")
	
	if rawMode {
		// rawモードの場合は、スキャンログをそのまま記録
		outputText += fmt.Sprintf("Mode: RAW SCAN DATA\n")
		outputText += fmt.Sprintf("Note: Run with stderr redirect to capture all logs\n")
		outputText += fmt.Sprintf("Example: ./find-fax --raw -o scan_logs.txt 2>&1\n\n")
		outputText += fmt.Sprintf("==========================================\n")
		outputText += fmt.Sprintf("Scanning for %d seconds...\n\n", c.Timeout/time.Second)
		
		// スキャン開始（rawモードでも実行）
		fmt.Printf("Starting BLE scan in RAW mode...\n")
		devices, err := c.ScanDevices("")
		
		if err != nil {
			outputText += fmt.Sprintf("Error during scan: %v\n", err)
		} else {
			outputText += fmt.Sprintf("\n==========================================\n")
			outputText += fmt.Sprintf("SCAN SUMMARY\n")
			outputText += fmt.Sprintf("==========================================\n\n")
			outputText += fmt.Sprintf("Total devices found: %d\n\n", len(devices))
			
			if len(devices) > 0 {
				outputText += fmt.Sprintf("Device List:\n")
				outputText += fmt.Sprintf("-----------\n")
				i := 1
				for mac, name := range devices {
					outputText += fmt.Sprintf("%3d. MAC: %s", i, mac)
					if string(name) != "" {
						outputText += fmt.Sprintf(" | Name: %s", string(name))
					}
					outputText += fmt.Sprintf("\n")
					i++
				}
			}
			
			outputText += fmt.Sprintf("\n==========================================\n")
			outputText += fmt.Sprintf("Note: Check stderr output for detailed scan logs including:\n")
			outputText += fmt.Sprintf("  - Service UUIDs (ae3a, etc.)\n")
			outputText += fmt.Sprintf("  - Characteristics (ae01, ae3b, ae3c, etc.)\n")
			outputText += fmt.Sprintf("  - Connection attempts and results\n")
		}
	} else {
		// デバイススキャン（空文字列を渡してすべてのデバイスを取得）
		fmt.Printf("Scanning for all BLE devices (will take %d seconds)...\n", c.Timeout/time.Second)
		
		// スキャン開始
		devices, err := c.ScanDevices("")
		
		if err != nil {
			logger.Error("Error during device scan", zap.Error(err))
			outputText += fmt.Sprintf("Error during scan: %v\n\n", err)
		} else {
			outputText += fmt.Sprintf("Found %d device(s)\n\n", len(devices))
			
			if len(devices) == 0 {
				outputText += "No devices found\n"
			} else {
				i := 1
				for mac, name := range devices {
					logger.Info("Found device", zap.Int("device", i), zap.String("mac", mac), zap.String("name", string(name)))
					outputText += fmt.Sprintf("Device #%d:\n", i)
					outputText += fmt.Sprintf("  MAC Address: %s\n", mac)
					if string(name) != "" {
						outputText += fmt.Sprintf("  Name: %s\n", string(name))
					} else {
						outputText += fmt.Sprintf("  Name: (unnamed)\n")
					}
					i++
				}
			}
		}
	}
	
	// ファイル出力が指定されている場合
	if outputFile != "" {
		outputText += fmt.Sprintf("\n==========================================\n")
		outputText += fmt.Sprintf("Scan completed at %s\n", time.Now().Format("15:04:05"))
		
		err = os.WriteFile(outputFile, []byte(outputText), 0644)
		if err != nil {
			logger.Error("Failed to write output file", zap.Error(err))
			log.Fatal(err)
		}
		logger.Info("Device list saved to file", zap.String("file", outputFile))
		fmt.Printf("Device list saved to: %s\n", outputFile)
	} else {
		// 標準出力に表示
		fmt.Print(outputText)
	}
}
