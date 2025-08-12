package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nantokaworks/twitch-overlay/internal/env"
	"github.com/nantokaworks/twitch-overlay/internal/fontmanager"
	localdb "github.com/nantokaworks/twitch-overlay/internal/localdb"
	"github.com/nantokaworks/twitch-overlay/internal/output"
	"github.com/nantokaworks/twitch-overlay/internal/shared/logger"
	"github.com/nantokaworks/twitch-overlay/internal/twitcheventsub"
	"github.com/nantokaworks/twitch-overlay/internal/twitchtoken"
	"github.com/nantokaworks/twitch-overlay/internal/version"
	"github.com/nantokaworks/twitch-overlay/internal/webserver"
	"go.uber.org/zap"

	_ "github.com/nantokaworks/twitch-overlay/internal/env"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// Display version
	fmt.Println("🖨️  Twitch Overlay " + version.String())
	fmt.Println()

	// init db
	db, err := localdb.SetupDB("./local.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// init font manager
	if err := fontmanager.Initialize(); err != nil {
		logger.Error("Failed to initialize font manager", zap.Error(err))
		log.Fatal("フォントマネージャーの初期化に失敗しました")
	}

	// フォントが設定されているか確認（必須）
	if info := fontmanager.GetCurrentFontInfo(); info["path"] == nil || info["path"] == "" {
		fmt.Println("")
		fmt.Println("========================================")
		fmt.Println("❌ エラー: フォントがアップロードされていません")
		fmt.Println("")
		fmt.Println("FAXと時計機能を使用するためには、フォントファイル（.ttf/.otf）のアップロードが必須です。")
		fmt.Println("")
		fmt.Printf("1. Webサーバーを起動します（ポート %d）\n", env.Value.ServerPort)
		fmt.Printf("2. ブラウザで http://localhost:%d/settings にアクセスしてください\n", env.Value.ServerPort)
		fmt.Println("3. 「フォント」タブから .ttf または .otf ファイルをアップロードしてください")
		fmt.Println("========================================")
		fmt.Println("")
		
		// Webサーバーだけは起動する（フォント設定のため）
		webserver.StartWebServer(env.Value.ServerPort)
		
		// フォントがアップロードされるまで待機
		fmt.Println("フォントがアップロードされるのを待っています...")
		fmt.Println("Ctrl+C で終了できます")
		
		// シグナル待機
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		
		fmt.Println("\n終了します...")
		webserver.Shutdown()
		os.Exit(0)
	}

	// init output
	c, err := output.SetupPrinter()
	if err != nil {
		log.Fatal(err)
	}
	defer output.Stop()
	err = output.SetupPrinterOptions(env.Value.BestQuality, env.Value.Dither, env.Value.AutoRotate, env.Value.BlackPoint)
	if err != nil {
		log.Fatal(err)
	}
	err = output.ConnectPrinter(c, *env.Value.PrinterAddress)
	if err != nil {
		logger.Error("Failed to connect to printer at startup", zap.Error(err))
		logger.Info("Will retry connection when printing")
	} else {
		// Print initial clock on successful connection
		if env.Value.InitialPrintEnabled && env.Value.ClockEnabled {
			if env.Value.DryRunMode {
				logger.Info("Printing initial clock (DRY-RUN MODE)")
			} else {
				logger.Info("Printing initial clock")
			}
			err = output.PrintInitialClock()
			if err != nil {
				logger.Error("Failed to print initial clock", zap.Error(err))
			} else {
				output.MarkInitialPrintDone()
			}
		} else {
			logger.Info("Skipping initial print (InitialPrintEnabled=false or ClockEnabled=false)")
			output.MarkInitialPrintDone()
		}
	}

	// load token from db
	var tokenValid bool
	var token twitchtoken.Token
	if token, tokenValid, _ = twitchtoken.GetLatestToken(); !tokenValid {
		// refresh token
		err := token.RefreshTwitchToken()
		if err != nil {
			logger.Error("Token is not valid, please authorize the app.", zap.Error(err))
			token = twitchtoken.Token{}
		}
	}

	// start web server (always start, even without token)
	webserver.StartWebServer(env.Value.ServerPort)

	// Create a done channel for goroutines
	done := make(chan struct{})

	// check token and start monitoring
	if token.AccessToken == "" {
		// Display authentication URL
		fmt.Println("")
		fmt.Println("====================================================")
		fmt.Println("⚠️  Twitch認証が必要です")
		fmt.Printf("🔗 以下のURLにアクセスして認証してください:\n")
		fmt.Printf("   http://localhost:%d/auth\n", env.Value.ServerPort)
		fmt.Printf("\n")
		fmt.Printf("📍 Twitchアプリ設定のリダイレクトURLに以下を追加してください:\n")
		fmt.Printf("   http://localhost:%d/callback\n", env.Value.ServerPort)
		fmt.Println("====================================================")
		fmt.Println("")
		
		logger.Info("Waiting for Twitch authentication")

		// wait get token or ctrl+c in goroutine
		go func() {
			for {
				select {
				case <-done:
					return
				default:
					if token, tokenValid, _ = twitchtoken.GetLatestToken(); tokenValid {
						logger.Info("Token is valid.")
						fmt.Println("")
						fmt.Println("✅ Twitch認証が完了しました！")
						fmt.Println("")
						// start twitch eventsub after getting token
						twitcheventsub.SetupEventSub(&token)
						return
					}
					time.Sleep(1 * time.Second)
				}
			}
		}()
	} else {
		// start twitch eventsub if token is already valid
		twitcheventsub.SetupEventSub(&token)
	}

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Wait for signal
	sig := <-sigChan
	logger.Info("Received signal, shutting down...", zap.String("signal", sig.String()))

	// Signal all goroutines to stop
	close(done)

	// Shutdown all services concurrently for faster shutdown
	go twitcheventsub.Shutdown()
	go webserver.Shutdown()

	// Give services a moment to shutdown gracefully
	time.Sleep(200 * time.Millisecond)

	// Clean up resources (already handled by defer statements)
	logger.Info("Shutdown complete")
}
