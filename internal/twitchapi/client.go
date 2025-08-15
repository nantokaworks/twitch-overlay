package twitchapi

import (
	"fmt"
	"io"
	"net/http"

	"github.com/nantokaworks/twitch-overlay/internal/env"
	"github.com/nantokaworks/twitch-overlay/internal/shared/logger"
	"github.com/nantokaworks/twitch-overlay/internal/twitchtoken"
	"go.uber.org/zap"
)

// makeAuthenticatedRequest は認証付きのHTTPリクエストを実行し、401エラー時は自動的にトークンをリフレッシュしてリトライします
func makeAuthenticatedRequest(method, url string, body io.Reader) (*http.Response, error) {
	// 最初にトークンを取得
	token, valid, err := twitchtoken.GetLatestToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}
	
	// トークンが無効な場合は先にリフレッシュを試みる
	if !valid && token.RefreshToken != "" {
		logger.Info("Token is invalid, attempting to refresh before API call")
		if err := token.RefreshTwitchToken(); err != nil {
			logger.Error("Failed to refresh token", zap.Error(err))
			return nil, fmt.Errorf("token is invalid and refresh failed: %w", err)
		}
		// リフレッシュ後に新しいトークンを取得
		token, valid, err = twitchtoken.GetLatestToken()
		if err != nil || !valid {
			return nil, fmt.Errorf("failed to get token after refresh: %w", err)
		}
	} else if !valid {
		return nil, fmt.Errorf("token is invalid and no refresh token available")
	}

	// リクエストを実行する関数
	doRequest := func(accessToken string) (*http.Response, error) {
		req, err := http.NewRequest(method, url, body)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// 必須ヘッダーを設定
		req.Header.Set("Client-ID", *env.Value.ClientID)
		req.Header.Set("Authorization", "Bearer "+accessToken)

		client := &http.Client{}
		return client.Do(req)
	}

	// 最初のリクエストを実行
	resp, err := doRequest(token.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// 401 Unauthorizedの場合はトークンをリフレッシュして再試行
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close() // 最初のレスポンスをクローズ
		
		logger.Info("Received 401 Unauthorized, attempting to refresh token")
		
		// トークンをリフレッシュ
		if err := token.RefreshTwitchToken(); err != nil {
			logger.Error("Failed to refresh token after 401", zap.Error(err))
			return nil, fmt.Errorf("failed to refresh token after 401: %w", err)
		}

		// 新しいトークンを取得
		newToken, valid, err := twitchtoken.GetLatestToken()
		if err != nil || !valid {
			return nil, fmt.Errorf("failed to get new token after refresh: %w", err)
		}

		logger.Info("Token refreshed successfully, retrying request")
		
		// 新しいトークンで再試行
		resp, err = doRequest(newToken.AccessToken)
		if err != nil {
			return nil, fmt.Errorf("request failed after token refresh: %w", err)
		}
	}

	return resp, nil
}

// makeAuthenticatedGetRequest は認証付きのGETリクエストを実行します
func makeAuthenticatedGetRequest(url string) (*http.Response, error) {
	return makeAuthenticatedRequest("GET", url, nil)
}