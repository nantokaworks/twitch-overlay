package main

import (
	"log"

	"github.com/nantokaworks/twitch-fax/internal/env"
	localdb "github.com/nantokaworks/twitch-fax/internal/localdb"
	"github.com/nantokaworks/twitch-fax/internal/output"
	"github.com/nantokaworks/twitch-fax/internal/shared/logger"
	"github.com/nantokaworks/twitch-fax/internal/twitcheventsub"
	"github.com/nantokaworks/twitch-fax/internal/twitchtoken"
	"github.com/nantokaworks/twitch-fax/internal/webserver"
	"go.uber.org/zap"

	_ "github.com/nantokaworks/twitch-fax/internal/env"

	_ "github.com/mattn/go-sqlite3"
)

func main() {

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

	// check token and start OAuth callback server
	if token.AccessToken == "" {
		twitchtoken.SetupCallbackServer()

		// wait get token or ctrl+c in goroutine
		go func() {
			logger.Info("Waiting for token...")
			for {
				if token, tokenValid, _ = twitchtoken.GetLatestToken(); tokenValid {
					logger.Info("Token is valid.")
					// start twitch eventsub after getting token
					twitcheventsub.SetupEventSub(&token)
					break
				}
			}
		}()
	} else {
		// start twitch eventsub if token is already valid
		twitcheventsub.SetupEventSub(&token)
	}

	// Keep the application running
	select {}
}
