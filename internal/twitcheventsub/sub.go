package twitcheventsub

import (
	"encoding/json"
	"fmt"

	"github.com/joeyak/go-twitch-eventsub/v3"
	"github.com/nantokaworks/twitch-overlay/internal/env"
	"github.com/nantokaworks/twitch-overlay/internal/shared/logger"
	"github.com/nantokaworks/twitch-overlay/internal/twitchtoken"
	"go.uber.org/zap"
)

var (
	client *twitch.Client
	shutdownChan = make(chan struct{})
)

func SetupEventSub(token *twitchtoken.Token) {
	client = twitch.NewClient()

	client.OnError(func(err error) {
		logger.Error("ERROR: %v\n", zap.Error(err))
	})
	client.OnWelcome(func(message twitch.WelcomeMessage) {
		events := []twitch.EventSubscription{
			twitch.SubChannelChannelPointsCustomRewardRedemptionAdd,
			twitch.SubChannelCheer,
			twitch.SubChannelFollow,
			twitch.SubChannelRaid,
			twitch.SubChannelChatMessage,
			twitch.SubChannelShoutoutReceive,
			twitch.SubChannelSubscribe,
			twitch.SubChannelSubscriptionGift,
			twitch.SubChannelSubscriptionMessage,
			twitch.SubStreamOffline,
			twitch.SubStreamOnline,
		}

		for _, event := range events {
			logger.Info("subscribing", zap.String("event", string(event)))

			_, err := twitch.SubscribeEvent(twitch.SubscribeRequest{
				SessionID:   message.Payload.Session.ID,
				ClientID:    *env.Value.ClientID,
				AccessToken: token.AccessToken,
				Event:       event,
				Condition: map[string]string{
					"broadcaster_user_id":    *env.Value.TwitchUserID,
					"to_broadcaster_user_id": *env.Value.TwitchUserID,
					"moderator_user_id":      *env.Value.TwitchUserID,
					"user_id":                *env.Value.TwitchUserID,
				},
			})
			if err != nil {
				fmt.Printf("ERROR: %v\n", err)
				return
			}
		}
	})
	client.OnNotification(func(message twitch.NotificationMessage) {

		rawJson := string(*message.Payload.Event)
		fmt.Printf("NOTIFICATION: %s: %s\n", message.Payload.Subscription.Type, string(rawJson))

		switch message.Payload.Subscription.Type {

		// use channel chat message
		case twitch.SubChannelChatMessage:
			var evt twitch.EventChannelChatMessage
			if err := json.Unmarshal(*message.Payload.Event, &evt); err != nil {
				fmt.Printf("Error parsing CHANNEL CHAT MESSAGE event: %v\n", err)
			} else {
				HandleChannelChatMessage(evt)
			}

		// use channel point
		case twitch.SubChannelChannelPointsCustomRewardRedemptionAdd:
			var evt twitch.EventChannelChannelPointsCustomRewardRedemptionAdd
			if err := json.Unmarshal(*message.Payload.Event, &evt); err != nil {
				fmt.Printf("Error parsing CHANNEL POINTS CUSTOM REWARD event: %v\n", err)
			} else {
				HandleChannelPointsCustomRedemptionAdd(evt)
			}

		// use cheer
		case twitch.SubChannelCheer:
			var evt twitch.EventChannelCheer
			if err := json.Unmarshal(*message.Payload.Event, &evt); err != nil {
				fmt.Printf("Error parsing CHEER event: %v\n", err)
			} else {
				HandleChannelCheer(evt)
			}

		// use follow
		case twitch.SubChannelFollow:
			var evt twitch.EventChannelFollow
			if err := json.Unmarshal(*message.Payload.Event, &evt); err != nil {
				fmt.Printf("Error parsing FOLLOW event: %v\n", err)
			} else {
				HandleChannelFollow(evt)
			}

		// use raid
		case twitch.SubChannelRaid:
			var evt twitch.EventChannelRaid
			if err := json.Unmarshal(*message.Payload.Event, &evt); err != nil {
				fmt.Printf("Error parsing RAID event: %v\n", err)
			} else {
				HandleChannelRaid(evt)
			}

		// use shoutout
		case twitch.SubChannelShoutoutReceive:
			var evt twitch.EventChannelShoutoutReceive
			if err := json.Unmarshal(*message.Payload.Event, &evt); err != nil {
				fmt.Printf("Error parsing SHOUTOUT event: %v\n", err)
			} else {
				HandleChannelShoutoutReceive(evt)
			}

		// use subscribe
		case twitch.SubChannelSubscribe:
			var evt twitch.EventChannelSubscribe
			if err := json.Unmarshal(*message.Payload.Event, &evt); err != nil {
				fmt.Printf("Error parsing SUBSCRIBE event: %v\n", err)
			} else {
				HandleChannelSubscribe(evt)
			}

		// use subscribe gift
		case twitch.SubChannelSubscriptionGift:
			var evt twitch.EventChannelSubscriptionGift
			if err := json.Unmarshal(*message.Payload.Event, &evt); err != nil {
				fmt.Printf("Error parsing SUBSCRIBE event: %v\n", err)
			} else {
				HandleChannelSubscriptionGift(evt)
			}

		// use subscription message (for resubs)
		case twitch.SubChannelSubscriptionMessage:
			var evt twitch.EventChannelSubscriptionMessage
			if err := json.Unmarshal(*message.Payload.Event, &evt); err != nil {
				fmt.Printf("Error parsing SUBSCRIPTION MESSAGE event: %v\n", err)
			} else {
				HandleChannelSubscriptionMessage(evt)
			}

		// use stream offline
		case twitch.SubStreamOffline:
			var evt twitch.EventStreamOffline
			if err := json.Unmarshal(*message.Payload.Event, &evt); err != nil {
				fmt.Printf("Error parsing STREAM OFFLINE event: %v\n", err)
			} else {
				HandleStreamOffline(evt)
			}

		// use stream online
		case twitch.SubStreamOnline:
			var evt twitch.EventStreamOnline
			if err := json.Unmarshal(*message.Payload.Event, &evt); err != nil {
				fmt.Printf("Error parsing STREAM ONLINE event: %v\n", err)
			} else {
				HandleStreamOnline(evt)
			}

		default:
			fmt.Printf("NOTIFICATION: %s: %s\n", message.Payload.Subscription.Type, string(*message.Payload.Event))
		}
	})
	client.OnKeepAlive(func(message twitch.KeepAliveMessage) {
		// Suppress keepalive logs
	})
	client.OnRevoke(func(message twitch.RevokeMessage) {
		fmt.Printf("REVOKE: %v\n", message)
	})
	client.OnRawEvent(func(event string, metadata twitch.MessageMetadata, subscription twitch.PayloadSubscription) {
		fmt.Printf("RAW EVENT: %s\n", subscription.Type)
	})

	go func() {
		err := client.Connect()
		if err != nil {
			fmt.Printf("Could not connect client: %v\n", err)
		}
	}()
}

// Shutdown closes the EventSub client connection
func Shutdown() {
	if client != nil {
		client.Close()
	}
}
