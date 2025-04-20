package twitchtoken

import (
	// 追加
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var scopes = []string{
	"user:read:chat",
	"user:read:email",
	"channel:read:subscriptions",
	"bits:read",
	"chat:read",
	"chat:edit",
	"moderator:read:followers",
	"channel:manage:redemptions",
	"moderator:manage:shoutouts",
}

func GetTwitchToken(code string) (map[string]interface{}, error) {
	clientID := os.Getenv("CLIENT_ID")
	clientSecret := os.Getenv("CLIENT_SECRET")
	redirectURI := os.Getenv("REDIRECT_URI")

	resp, err := http.PostForm("https://id.twitch.tv/oauth2/token", url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {redirectURI},
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if _, ok := result["access_token"]; !ok {
		return nil, errors.New("access_token not found in response")
	}
	// スコープの設定（必要に応じて加工）
	result["scope"] = strings.Join(scopes, " ")
	return result, nil
}

func (t *Token) RefreshTwitchToken() error {
	clientID := os.Getenv("CLIENT_ID")
	clientSecret := os.Getenv("CLIENT_SECRET")

	resp, err := http.PostForm("https://id.twitch.tv/oauth2/token", url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"refresh_token": {t.RefreshToken},
		"grant_type":    {"refresh_token"},
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	var accessToken string
	if v, ok := result["access_token"]; !ok {
		return errors.New("access_token not found in response")
	} else {
		accessToken = v.(string)
	}

	var refreshToken string
	if v, ok := result["refresh_token"]; !ok {
		return errors.New("refresh_token not found in response")
	} else {
		refreshToken = v.(string)
	}

	var scope string
	if v, ok := result["scope"].([]interface{}); !ok {
		return errors.New("scope not found in response")
	} else {
		scopes := make([]string, 0)
		for _, s := range v {
			if str, ok := s.(string); ok {
				scopes = append(scopes, str)
			}
		}
		scope = strings.Join(scopes, " ")
	}
	if _, ok := result["expires_in"]; !ok {
		return errors.New("expires_in not found in response")
	}

	// save token
	t.AccessToken = accessToken
	t.RefreshToken = refreshToken
	t.Scope = scope
	t.ExpiresAt = time.Now().Unix() + int64(result["expires_in"].(float64))
	return t.SaveToken()
}

// 変更: 引数なしで環境変数から認証情報を取得し、定数 scopes を使用
func GetAuthURL() string {
	clientID := os.Getenv("CLIENT_ID")
	redirectURI := os.Getenv("REDIRECT_URI")
	return fmt.Sprintf(
		"https://id.twitch.tv/oauth2/authorize?response_type=code&client_id=%s&redirect_uri=%s&scope=%s",
		url.QueryEscape(clientID),
		url.QueryEscape(redirectURI),
		url.QueryEscape(strings.Join(scopes, " ")),
	)
}
