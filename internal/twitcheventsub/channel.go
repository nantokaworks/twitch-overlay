package twitcheventsub

import (
	"fmt"
	"time"

	"github.com/joeyak/go-twitch-eventsub/v3"
	"github.com/nantokaworks/twitch-overlay/internal/env"
	"github.com/nantokaworks/twitch-overlay/internal/output"
	"github.com/nantokaworks/twitch-overlay/internal/shared/logger"
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
	title := "ビッツありがとう :)"
	userName := message.User.UserName
	details := fmt.Sprintf("%d ビッツ", message.Bits)

	output.PrintOutWithTitle(title, userName, "", details, time.Now())
}
func HandleChannelFollow(message twitch.EventChannelFollow) {
	title := "フォローありがとう :)"
	userName := message.User.UserName
	details := "" // フォローの場合は詳細なし

	output.PrintOutWithTitle(title, userName, "", details, time.Now())
}
func HandleChannelRaid(message twitch.EventChannelRaid) {
	title := "レイドありがとう :)"
	userName := message.FromBroadcasterUserName
	details := fmt.Sprintf("%d 人", message.Viewers)

	output.PrintOutWithTitle(title, userName, "", details, time.Now())
}
func HandleChannelShoutoutReceive(message twitch.EventChannelShoutoutReceive) {
	title := "応援ありがとう :)"
	userName := message.FromBroadcasterUserName
	details := "" // シャウトアウトの場合は詳細なし

	output.PrintOutWithTitle(title, userName, "", details, time.Now())
}
func HandleChannelSubscribe(message twitch.EventChannelSubscribe) {
	if !message.IsGift {
		title := "サブスクありがとう :)"
		userName := message.User.UserName
		details := fmt.Sprintf("Tier %s", message.Tier)

		output.PrintOutWithTitle(title, userName, "", details, time.Now())
	} else {
		title := "サブギフおめです :)"
		userName := message.User.UserName
		details := fmt.Sprintf("Tier %s", message.Tier)

		output.PrintOutWithTitle(title, userName, "", details, time.Now())
	}
}

func HandleChannelSubscriptionGift(message twitch.EventChannelSubscriptionGift) {
	title := "サブギフありがとう :)"

	if !message.IsAnonymous {
		userName := message.User.UserName
		details := fmt.Sprintf("Tier %s | %d個", message.Tier, message.Total)
		output.PrintOutWithTitle(title, userName, "", details, time.Now())
	} else {
		userName := "匿名さん"
		details := fmt.Sprintf("Tier %s | %d個", message.Tier, message.Total)
		output.PrintOutWithTitle(title, userName, "", details, time.Now())
	}
}

func HandleChannelSubscriptionMessage(message twitch.EventChannelSubscriptionMessage) {
	// 再サブスクメッセージの処理
	var title string
	var extra string
	var details string

	if message.CumulativeMonths > 1 {
		// 再サブスク - 4行レイアウト
		title = "サブスクありがとう :)"
		extra = fmt.Sprintf("%d ヶ月目", message.CumulativeMonths)
		details = message.Message.Text // 空メッセージの場合は空文字列
	} else {
		// 初回サブスク（メッセージ付き）
		title = "サブスクありがとう :)"
		extra = "" // 初回は月数なし
		details = message.Message.Text // 空メッセージの場合は空文字列のまま
	}

	userName := message.User.UserName
	output.PrintOutWithTitle(title, userName, extra, details, time.Now())

	logger.Info("サブスクメッセージ",
		zap.String("user", message.User.UserName),
		zap.Int("cumulative_months", message.CumulativeMonths),
		zap.Int("streak_months", message.StreakMonths),
		zap.String("tier", message.Tier),
		zap.String("message", message.Message.Text))
}
