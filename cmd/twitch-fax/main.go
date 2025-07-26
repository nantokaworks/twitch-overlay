package main

import (
	"log"

	"github.com/nantokaworks/twitch-fax/internal/env"
	localdb "github.com/nantokaworks/twitch-fax/internal/localdb"
	"github.com/nantokaworks/twitch-fax/internal/output"
	"github.com/nantokaworks/twitch-fax/internal/shared/logger"
	"github.com/nantokaworks/twitch-fax/internal/twitcheventsub"
	"github.com/nantokaworks/twitch-fax/internal/twitchtoken"
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
	defer c.Stop()
	err = output.SetupPrinterOptions(env.Value.BestQuality, env.Value.Dither, env.Value.AutoRotate, env.Value.BlackPoint)
	if err != nil {
		log.Fatal(err)
	}
	err = output.ConnectPrinter(c, *env.Value.PrinterAddress)
	if err != nil {
		log.Fatal(err)
	}
	
	// Print initial clock and stats on successful connection
	if env.Value.ClockEnabled {
		logger.Info("Printing initial clock and stats")
		err = output.PrintInitialClockAndStats()
		if err != nil {
			logger.Error("Failed to print initial clock and stats", zap.Error(err))
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

	// check token and start OAuth callback server
	if token.AccessToken == "" {
		twitchtoken.SetupCallbackServer()

		// wait get token or ctrl+c
		logger.Info("Waiting for token...")
		for {
			if token, tokenValid, _ = twitchtoken.GetLatestToken(); tokenValid {
				break
			}
		}
		logger.Info("Token is valid.")
	}

	// start twitch eventsub
	twitcheventsub.SetupEventSub(&token)

}
