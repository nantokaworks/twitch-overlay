package main

import (
	"fmt"
	"log"
	"time"

	"github.com/nantokaworks/twitch-fax/internal/env"
	localdb "github.com/nantokaworks/twitch-fax/internal/localdb"
	"github.com/nantokaworks/twitch-fax/internal/output"
	"github.com/nantokaworks/twitch-fax/internal/shared/logger"
	"github.com/nantokaworks/twitch-fax/internal/twitcheventsub"
	"github.com/nantokaworks/twitch-fax/internal/twitchtoken"
	"github.com/nantokaworks/twitch-fax/internal/version"
	"github.com/nantokaworks/twitch-fax/internal/webserver"
	"go.uber.org/zap"

	_ "github.com/nantokaworks/twitch-fax/internal/env"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// Display version
	fmt.Println("🖨️  Twitch FAX " + version.String())
	fmt.Println()

	// init db
	db, err := localdb.SetupDB("./local.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

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
				if token, tokenValid, _ = twitchtoken.GetLatestToken(); tokenValid {
					logger.Info("Token is valid.")
					fmt.Println("")
					fmt.Println("✅ Twitch認証が完了しました！")
					fmt.Println("")
					// start twitch eventsub after getting token
					twitcheventsub.SetupEventSub(&token)
					break
				}
				time.Sleep(1 * time.Second)
			}
		}()
	} else {
		// start twitch eventsub if token is already valid
		twitcheventsub.SetupEventSub(&token)
	}

	// Keep the application running
	select {}
}
