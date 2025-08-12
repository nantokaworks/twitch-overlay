package twitchtoken

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/nantokaworks/twitch-overlay/internal/shared/logger"
	"go.uber.org/zap"
)

func SetupCallbackServer() {
	// 独自のServeMuxを作成
	mux := http.NewServeMux()

	// 認証ページまたはトークン情報の返却
	authURL := GetAuthURL()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, authURL, http.StatusFound)
	})

	// コールバックハンドラ
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "code not found", http.StatusBadRequest)
			return
		}
		// Twitchからトークン取得
		result, err := GetTwitchToken(code)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// expires_inの処理
		expiresInFloat, ok := result["expires_in"].(float64)
		if !ok {
			http.Error(w, "invalid expires_in", http.StatusInternalServerError)
			return
		}
		expiresAtNew := time.Now().Unix() + int64(expiresInFloat)
		newToken := Token{
			AccessToken:  result["access_token"].(string),
			RefreshToken: result["refresh_token"].(string),
			Scope:        result["scope"].(string),
			ExpiresAt:    expiresAtNew,
		}
		if err := newToken.SaveToken(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	logger.Info("Starting OAuth callback server on port 30303")

	go func() {
		if err := http.ListenAndServe(":30303", mux); err != nil {
			logger.Error("Failed to start OAuth callback server", zap.Error(err))
			return
		}
	}()

	time.Sleep(1 * time.Second) // Wait for the server to start
}
