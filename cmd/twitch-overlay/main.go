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
	"github.com/nantokaworks/twitch-overlay/internal/shared/paths"
	"github.com/nantokaworks/twitch-overlay/internal/status"
	"github.com/nantokaworks/twitch-overlay/internal/twitchapi"
	"github.com/nantokaworks/twitch-overlay/internal/twitcheventsub"
	"github.com/nantokaworks/twitch-overlay/internal/twitchtoken"
	"github.com/nantokaworks/twitch-overlay/internal/version"
	"github.com/nantokaworks/twitch-overlay/internal/webserver"
	"go.uber.org/zap"

	_ "github.com/nantokaworks/twitch-overlay/internal/env"

	_ "github.com/mattn/go-sqlite3"
)

// refreshTokenPeriodically はトークンの有効期限を監視し、期限の30分前に自動的にリフレッシュを行います
func refreshTokenPeriodically(done <-chan struct{}) {
	logger.Info("Starting token refresh goroutine")
	
	for {
		select {
		case <-done:
			logger.Info("Stopping token refresh goroutine")
			return
		default:
			token, _, err := twitchtoken.GetLatestToken()
			if err != nil {
				// トークンが見つからない場合は1分後に再チェック
				time.Sleep(1 * time.Minute)
				continue
			}
			
			// 現在時刻とトークンの有効期限を比較
			now := time.Now().Unix()
			timeUntilExpiry := token.ExpiresAt - now
			
			if timeUntilExpiry <= 0 {
				// トークンがすでに期限切れの場合、即座にリフレッシュ
				logger.Info("Token has expired, refreshing immediately")
				if err := token.RefreshTwitchToken(); err != nil {
					logger.Error("Failed to refresh expired token", zap.Error(err))
					// リフレッシュに失敗した場合は5分後に再試行
					time.Sleep(5 * time.Minute)
				} else {
					logger.Info("Token refreshed successfully")
				}
			} else if timeUntilExpiry <= 30*60 { // 30分 = 1800秒
				// 期限の30分前になったらリフレッシュ
				logger.Info("Token expires in less than 30 minutes, refreshing now", 
					zap.Int64("seconds_until_expiry", timeUntilExpiry))
				if err := token.RefreshTwitchToken(); err != nil {
					logger.Error("Failed to refresh token", zap.Error(err))
					// リフレッシュに失敗した場合は5分後に再試行
					time.Sleep(5 * time.Minute)
				} else {
					logger.Info("Token refreshed successfully")
				}
			} else {
				// 次のチェックまでの時間を計算（期限の30分前になるまで待つ）
				sleepDuration := time.Duration(timeUntilExpiry-30*60) * time.Second
				// ただし、最大1時間までとする（長時間スリープを避ける）
				if sleepDuration > time.Hour {
					sleepDuration = time.Hour
				}
				logger.Debug("Next token refresh check", 
					zap.Duration("sleep_duration", sleepDuration),
					zap.Int64("seconds_until_expiry", timeUntilExpiry))
				time.Sleep(sleepDuration)
			}
		}
	}
}

// checkStreamStatus は配信状態をAPIから取得して更新します
func checkStreamStatus() {
	// TwitchユーザーIDが設定されていない場合はスキップ
	if env.Value.TwitchUserID == nil || *env.Value.TwitchUserID == "" {
		return
	}

	streamInfo, err := twitchapi.GetStreamInfo()
	if err != nil {
		logger.Debug("Failed to get stream info", zap.Error(err))
		return
	}

	if streamInfo.IsLive {
		// 配信中
		startTime := time.Now() // 本来はAPIから取得すべきだが、現在のAPIでは開始時刻が取れない
		status.UpdateStreamStatus(true, &startTime, streamInfo.ViewerCount)
		logger.Debug("Stream is live", zap.Int("viewers", streamInfo.ViewerCount))
	} else {
		// オフライン
		status.UpdateStreamStatus(false, nil, 0)
		logger.Debug("Stream is offline")
	}
}

// startStreamMonitoring は定期的に配信状態をチェックします
func startStreamMonitoring(done <-chan struct{}) {
	logger.Info("Starting stream status monitoring")
	
	// 初回チェック
	checkStreamStatus()
	
	// 1分ごとにチェック
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			checkStreamStatus()
		case <-done:
			logger.Info("Stopping stream status monitoring")
			return
		}
	}
}

func main() {
	// Display version
	fmt.Println("🖨️  Twitch Overlay " + version.String())
	fmt.Println()

	// Ensure data directories exist
	if err := paths.EnsureDataDirs(); err != nil {
		log.Fatal("Failed to create data directories: ", err)
	}
	logger.Info("Data directory", zap.String("path", paths.GetDataDir()))

	// init db
	db, err := localdb.SetupDB(paths.GetDBPath())
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

	// init printer options (printer setup is handled by keep-alive goroutine)
	defer output.Stop()
	err = output.SetupPrinterOptions(env.Value.BestQuality, env.Value.Dither, env.Value.AutoRotate, env.Value.BlackPoint)
	if err != nil {
		logger.Error("Failed to setup printer options", zap.Error(err))
	}
	
	// Initialize printer subsystem (including keep-alive and clock)
	// This must be called after env.Value is initialized
	output.InitializePrinter()

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
						// Start token refresh goroutine after successful authentication
						go refreshTokenPeriodically(done)
						// Start stream monitoring
						go startStreamMonitoring(done)
						return
					}
					time.Sleep(1 * time.Second)
				}
			}
		}()
	} else {
		// start twitch eventsub if token is already valid
		twitcheventsub.SetupEventSub(&token)
		// Start token refresh goroutine
		go refreshTokenPeriodically(done)
		// Start stream monitoring
		go startStreamMonitoring(done)
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
