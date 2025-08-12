package twitcheventsub

import (
	"fmt"
	"time"

	"github.com/joeyak/go-twitch-eventsub/v3"
	"github.com/nantokaworks/twitch-fax/internal/env"
	"github.com/nantokaworks/twitch-fax/internal/output"
	"github.com/nantokaworks/twitch-fax/internal/shared/logger"
	"go.uber.org/zap"
)

func HandleChannelChatMessage(message twitch.EventChannelChatMessage) {
	if message.ChannelPointsCustomRewardId != *env.Value.TriggerCustomRewordID {
		return
	}
	output.PrintOut(message.Chatter.ChatterUserName, message.Message.Fragments, time.Now())
}

func HandleChannelPointsCustomRedemptionAdd(message twitch.EventChannelChannelPointsCustomRewardRedemptionAdd) {
	if message.Reward.ID == *env.Value.TriggerCustomRewordID {
		return
	}

	// fragments := []twitch.ChatMessageFragment{
	// 	{
	// 		Type:      "text",
	// 		Text:      fmt.Sprintf("🎉チャネポ %s %s", message.Reward.Title, message.UserInput),
	// 		Cheermote: nil,
	// 		Emote:     nil,
	// 	},
	// }

	// output.PrintOut(message.User.UserName, fragments, time.Now())
	logger.Info("チャネポ", zap.String("user", message.User.UserName), zap.String("reward", message.Reward.Title), zap.String("userInput", message.UserInput))
}

func HandleChannelCheer(message twitch.EventChannelCheer) {
	fragments := []twitch.ChatMessageFragment{
		{
			Type:      "text",
			Text:      fmt.Sprintf("🎉ビッツありがとう %d", message.Bits),
			Cheermote: nil,
			Emote:     nil,
		},
	}

	output.PrintOut(message.User.UserName, fragments, time.Now())

}
func HandleChannelFollow(message twitch.EventChannelFollow) {
	fragments := []twitch.ChatMessageFragment{
		{
			Type:      "text",
			Text:      fmt.Sprintf("🎉フォローありがとう %s", message.UserLogin),
			Cheermote: nil,
			Emote:     nil,
		},
	}

	output.PrintOut(message.User.UserName, fragments, time.Now())

}
func HandleChannelRaid(message twitch.EventChannelRaid) {
	fragments := []twitch.ChatMessageFragment{
		{
			Type:      "text",
			Text:      fmt.Sprintf("🎉レイドありがとう %s", message.FromBroadcasterUserLogin),
			Cheermote: nil,
			Emote:     nil,
		},
	}

	output.PrintOut(message.FromBroadcasterUserName, fragments, time.Now())

}
func HandleChannelShoutoutReceive(message twitch.EventChannelShoutoutReceive) {
	fragments := []twitch.ChatMessageFragment{
		{
			Type:      "text",
			Text:      fmt.Sprintf("🎉応援ありがとう %s", message.FromBroadcasterUserLogin),
			Cheermote: nil,
			Emote:     nil,
		},
	}

	output.PrintOut(message.FromBroadcasterUserName, fragments, time.Now())
}
func HandleChannelSubscribe(message twitch.EventChannelSubscribe) {

	if !message.IsGift {
		fragments := []twitch.ChatMessageFragment{
			{
				Type:      "text",
				Text:      fmt.Sprintf("🎉サブスクありがとう %s", message.UserLogin),
				Cheermote: nil,
				Emote:     nil,
			},
		}

		output.PrintOut(message.User.UserName, fragments, time.Now())

	} else {
		fragments := []twitch.ChatMessageFragment{
			{
				Type:      "text",
				Text:      fmt.Sprintf("🎉サブギフおめ %s", message.UserLogin),
				Cheermote: nil,
				Emote:     nil,
			},
		}

		output.PrintOut(message.User.UserName, fragments, time.Now())
	}
}

func HandleChannelSubscriptionGift(message twitch.EventChannelSubscriptionGift) {
	if !message.IsAnonymous {
		fragments := []twitch.ChatMessageFragment{
			{
				Type:      "text",
				Text:      fmt.Sprintf("🎉サブギフありがとう %s", message.UserLogin),
				Cheermote: nil,
				Emote:     nil,
			},
		}

		output.PrintOut(message.User.UserName, fragments, time.Now())
	} else {
		fragments := []twitch.ChatMessageFragment{
			{
				Type:      "text",
				Text:      fmt.Sprintf("🎉サブギフありがとう %s", "匿名さん"),
				Cheermote: nil,
				Emote:     nil,
			},
		}

		output.PrintOut("匿名さん", fragments, time.Now())

	}

}

func HandleChannelSubscriptionMessage(message twitch.EventChannelSubscriptionMessage) {
	// 再サブスクメッセージの処理
	var text string
	if message.CumulativeMonths > 1 {
		// 再サブスク
		text = fmt.Sprintf("🎉再サブスクありがとう %s (%dヶ月目) %s", 
			message.User.UserLogin, 
			message.CumulativeMonths,
			message.Message.Text)
	} else {
		// 初回サブスク（メッセージ付き）
		text = fmt.Sprintf("🎉サブスクありがとう %s %s", 
			message.User.UserLogin,
			message.Message.Text)
	}

	fragments := []twitch.ChatMessageFragment{
		{
			Type:      "text",
			Text:      text,
			Cheermote: nil,
			Emote:     nil,
		},
	}

	output.PrintOut(message.User.UserName, fragments, time.Now())
	
	logger.Info("サブスクメッセージ", 
		zap.String("user", message.User.UserName), 
		zap.Int("cumulative_months", message.CumulativeMonths),
		zap.Int("streak_months", message.StreakMonths),
		zap.String("tier", message.Tier),
		zap.String("message", message.Message.Text))
}
