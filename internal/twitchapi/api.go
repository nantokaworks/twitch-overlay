package twitchapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/nantokaworks/twitch-overlay/internal/env"
	"github.com/nantokaworks/twitch-overlay/internal/shared/logger"
	"github.com/nantokaworks/twitch-overlay/internal/twitchtoken"
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

// BitsLeaderboardEntry represents a single entry in the bits leaderboard
type BitsLeaderboardEntry struct {
	UserID    string `json:"user_id"`
	UserLogin string `json:"user_login"`
	UserName  string `json:"user_name"`
	Rank      int    `json:"rank"`
	Score     int    `json:"score"`
	AvatarURL string // Will be populated separately
}

// BitsLeaderboardResponse represents the full response from the bits leaderboard API
type BitsLeaderboardResponse struct {
	Data      []BitsLeaderboardEntry `json:"data"`
	DateRange struct {
		StartedAt string `json:"started_at"`
		EndedAt   string `json:"ended_at"`
	} `json:"date_range"`
	Total int `json:"total"`
}

// GetBitsLeaderboard retrieves the bits leaderboard for a specific period
func GetBitsLeaderboard(period string) ([]*BitsLeaderboardEntry, *BitsLeaderboardResponse, error) {
	logger.Info("Getting bits leaderboard", zap.String("period", period))
	token, valid, err := twitchtoken.GetLatestToken()
	if !valid || err != nil {
		logger.Warn("No valid token available, returning empty leaderboard", zap.Error(err))
		return nil, nil, nil // Return empty result instead of error
	}

	// For "month" period, we need to specify started_at parameter
	var reqURL string
	if period == "month" {
		// Get first day of current month
		// Twitch API uses PST timezone, so we need to add 8 hours to UTC to ensure we get the correct month
		// UTC 08:00:00 = PST 00:00:00
		now := time.Now()
		firstOfMonth := time.Date(now.Year(), now.Month(), 1, 8, 0, 0, 0, time.UTC)
		startedAt := firstOfMonth.Format(time.RFC3339)
		reqURL = fmt.Sprintf("https://api.twitch.tv/helix/bits/leaderboard?count=5&period=%s&started_at=%s&broadcaster_id=%s", 
			url.QueryEscape(period), url.QueryEscape(startedAt), url.QueryEscape(*env.Value.TwitchUserID))
	} else {
		reqURL = fmt.Sprintf("https://api.twitch.tv/helix/bits/leaderboard?count=5&period=%s&broadcaster_id=%s", 
			url.QueryEscape(period), url.QueryEscape(*env.Value.TwitchUserID))
	}
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("Client-ID", *env.Value.ClientID)
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	var result BitsLeaderboardResponse

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, nil, err
	}

	if len(result.Data) == 0 {
		return nil, &result, nil // No leaders found but return the response for date_range
	}

	// Get avatar only for the first place
	if len(result.Data) > 0 {
		avatarURL, err := GetUserAvatar(result.Data[0].UserID)
		if err != nil {
			logger.Warn("Failed to get user avatar", zap.Error(err))
			// Continue without avatar
		} else {
			result.Data[0].AvatarURL = avatarURL
		}
	}

	// Return slice of leaders
	leaders := make([]*BitsLeaderboardEntry, len(result.Data))
	for i := range result.Data {
		leaders[i] = &result.Data[i]
	}

	return leaders, &result, nil
}

// GetUserAvatar retrieves the profile image URL for a user
func GetUserAvatar(userID string) (string, error) {
	token, valid, err := twitchtoken.GetLatestToken()
	if !valid || err != nil {
		return "", fmt.Errorf("failed to get valid token: %w", err)
	}

	reqURL := fmt.Sprintf("https://api.twitch.tv/helix/users?id=%s", url.QueryEscape(userID))
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Client-ID", *env.Value.ClientID)
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	var result struct {
		Data []struct {
			ProfileImageURL string `json:"profile_image_url"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Data) == 0 {
		return "", fmt.Errorf("user not found")
	}

	return result.Data[0].ProfileImageURL, nil
}