package twitcheventsub

import (
	"fmt"
	"time"

	"github.com/joeyak/go-twitch-eventsub/v3"
	"github.com/nantokaworks/twitch-overlay/internal/broadcast"
	"github.com/nantokaworks/twitch-overlay/internal/shared/logger"
	"github.com/nantokaworks/twitch-overlay/internal/status"
	"go.uber.org/zap"
)

func HandleStreamOnline(message twitch.EventStreamOnline) {
	logger.Info("Stream went online", 
		zap.String("broadcaster_id", message.Broadcaster.BroadcasterUserId),
		zap.String("broadcaster_name", message.Broadcaster.BroadcasterUserName),
		zap.Time("started_at", message.StartedAt))

	// 配信状態を更新
	startedAt := message.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now()
	}
	status.SetStreamOnline(startedAt, 0) // 視聴者数は後でAPIから取得

	// WebSocketで通知（broadcastパッケージ経由）
	broadcast.Send(map[string]interface{}{
		"type": "stream_online",
		"data": map[string]interface{}{
			"broadcaster_id":   message.Broadcaster.BroadcasterUserId,
			"broadcaster_name": message.Broadcaster.BroadcasterUserName,
			"started_at":       startedAt,
			"is_live":          true,
		},
	})

	fmt.Printf("🟢 配信開始: %s\n", message.Broadcaster.BroadcasterUserName)
}

func HandleStreamOffline(message twitch.EventStreamOffline) {
	logger.Info("Stream went offline",
		zap.String("broadcaster_id", message.BroadcasterUserId),
		zap.String("broadcaster_name", message.BroadcasterUserName))

	// 配信状態を更新
	status.SetStreamOffline()

	// WebSocketで通知（broadcastパッケージ経由）
	broadcast.Send(map[string]interface{}{
		"type": "stream_offline",
		"data": map[string]interface{}{
			"broadcaster_id":   message.BroadcasterUserId,
			"broadcaster_name": message.BroadcasterUserName,
			"is_live":          false,
		},
	})

	fmt.Printf("🔴 配信終了: %s\n", message.BroadcasterUserName)
}
