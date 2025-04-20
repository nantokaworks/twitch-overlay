package twitcheventsub

import (
	"fmt"

	"github.com/joeyak/go-twitch-eventsub/v3"
)

func HandleStreamOnline(message twitch.EventStreamOnline) {

	fmt.Printf("STREAM ONLINE: %+v\n", message)

}

func HandleStreamOffline(message twitch.EventStreamOffline) {

	fmt.Printf("STREAM OFFLINE: %+v\n", message)

}
