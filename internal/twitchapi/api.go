package twitchapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/nantokaworks/twitch-fax/internal/env"
	"github.com/nantokaworks/twitch-fax/internal/shared/logger"
	"github.com/nantokaworks/twitch-fax/internal/twitchtoken"
	"go.uber.org/zap"
)

// StreamInfo contains stream information
type StreamInfo struct {
	ViewerCount int
	IsLive      bool
}

// ChannelInfo contains channel information
type ChannelInfo struct {
	FollowerCount int
}

// GetStreamInfo retrieves current stream information
func GetStreamInfo() (*StreamInfo, error) {
	token, valid, err := twitchtoken.GetLatestToken()
	if !valid || err != nil {
		return nil, fmt.Errorf("failed to get valid token: %w", err)
	}

	reqURL := fmt.Sprintf("https://api.twitch.tv/helix/streams?user_id=%s", url.QueryEscape(*env.Value.TwitchUserID))
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Client-ID", *env.Value.ClientID)
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	var result struct {
		Data []struct {
			ViewerCount int `json:"viewer_count"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	info := &StreamInfo{
		ViewerCount: 0,
		IsLive:      false,
	}

	if len(result.Data) > 0 {
		info.ViewerCount = result.Data[0].ViewerCount
		info.IsLive = true
	}

	return info, nil
}

// GetChannelInfo retrieves channel information including follower count
func GetChannelInfo() (*ChannelInfo, error) {
	token, valid, err := twitchtoken.GetLatestToken()
	if !valid || err != nil {
		return nil, fmt.Errorf("failed to get valid token: %w", err)
	}

	reqURL := fmt.Sprintf("https://api.twitch.tv/helix/channels/followers?broadcaster_id=%s", url.QueryEscape(*env.Value.TwitchUserID))
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Client-ID", *env.Value.ClientID)
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	var result struct {
		Total int `json:"total"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &ChannelInfo{
		FollowerCount: result.Total,
	}, nil
}

// GetChannelStats retrieves both stream and channel information
func GetChannelStats() (viewers int, followers int, isLive bool, err error) {
	streamInfo, err := GetStreamInfo()
	if err != nil {
		logger.Error("Failed to get stream info", zap.Error(err))
		// Continue even if stream info fails
	} else {
		viewers = streamInfo.ViewerCount
		isLive = streamInfo.IsLive
	}

	channelInfo, err := GetChannelInfo()
	if err != nil {
		logger.Error("Failed to get channel info", zap.Error(err))
		return viewers, 0, isLive, err
	}

	return viewers, channelInfo.FollowerCount, isLive, nil
}