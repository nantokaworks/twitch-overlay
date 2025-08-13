package webserver

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/nantokaworks/twitch-overlay/internal/env"
	"github.com/nantokaworks/twitch-overlay/internal/shared/logger"
	"github.com/nantokaworks/twitch-overlay/internal/twitchtoken"
	"go.uber.org/zap"
)

// TwitchUserInfo represents Twitch user information
type TwitchUserInfo struct {
	ID              string `json:"id"`
	Login           string `json:"login"`
	DisplayName     string `json:"display_name"`
	ProfileImageURL string `json:"profile_image_url,omitempty"`
	Verified        bool   `json:"verified"`
	Error           string `json:"error,omitempty"`
}

// TwitchUsersResponse represents the response from Twitch Users API
type TwitchUsersResponse struct {
	Data []struct {
		ID              string `json:"id"`
		Login           string `json:"login"`
		DisplayName     string `json:"display_name"`
		ProfileImageURL string `json:"profile_image_url"`
	} `json:"data"`
}

// handleTwitchVerify verifies Twitch configuration by fetching user information
func handleTwitchVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	logger.Info("Verifying Twitch configuration")

	// Get current token
	token, valid, err := twitchtoken.GetLatestToken()
	if err != nil || !valid {
		logger.Error("Failed to get valid token", zap.Error(err))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TwitchUserInfo{
			Verified: false,
			Error:    "Twitch認証が必要です",
		})
		return
	}

	// Get user ID from environment
	userID := env.Value.TwitchUserID
	if userID == nil || *userID == "" {
		logger.Error("TWITCH_USER_ID not configured")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TwitchUserInfo{
			Verified: false,
			Error:    "TWITCH_USER_IDが設定されていません",
		})
		return
	}

	// Call Twitch API to get user information
	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.twitch.tv/helix/users?id=%s", *userID), nil)
	if err != nil {
		logger.Error("Failed to create request", zap.Error(err))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TwitchUserInfo{
			Verified: false,
			Error:    "リクエストの作成に失敗しました",
		})
		return
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Client-Id", *env.Value.ClientID)

	// Make request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Failed to fetch user info", zap.Error(err))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TwitchUserInfo{
			Verified: false,
			Error:    "Twitch APIへの接続に失敗しました",
		})
		return
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		logger.Error("Twitch API returned error", zap.Int("status", resp.StatusCode))
		w.Header().Set("Content-Type", "application/json")
		
		errorMessage := "Twitch APIエラー"
		if resp.StatusCode == http.StatusUnauthorized {
			errorMessage = "認証エラー: トークンが無効です"
		} else if resp.StatusCode == http.StatusForbidden {
			errorMessage = "アクセス権限がありません"
		}
		
		json.NewEncoder(w).Encode(TwitchUserInfo{
			Verified: false,
			Error:    errorMessage,
		})
		return
	}

	// Parse response
	var twitchResp TwitchUsersResponse
	if err := json.NewDecoder(resp.Body).Decode(&twitchResp); err != nil {
		logger.Error("Failed to parse response", zap.Error(err))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TwitchUserInfo{
			Verified: false,
			Error:    "レスポンスの解析に失敗しました",
		})
		return
	}

	// Check if user data exists
	if len(twitchResp.Data) == 0 {
		logger.Error("User not found", zap.String("user_id", *userID))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TwitchUserInfo{
			Verified: false,
			Error:    "ユーザーが見つかりません",
		})
		return
	}

	// Return user information
	userData := twitchResp.Data[0]
	logger.Info("Twitch configuration verified successfully", 
		zap.String("login", userData.Login),
		zap.String("display_name", userData.DisplayName))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(TwitchUserInfo{
		ID:              userData.ID,
		Login:           userData.Login,
		DisplayName:     userData.DisplayName,
		ProfileImageURL: userData.ProfileImageURL,
		Verified:        true,
	})
}