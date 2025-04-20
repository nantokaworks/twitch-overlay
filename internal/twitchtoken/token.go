package twitchtoken

import (
	"time"

	"github.com/nantokaworks/twitch-fax/internal/localdb"
)

type Token struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	ExpiresAt    int64  `json:"expires_at"`
}

func GetLatestToken() (Token, bool, error) {
	var t Token
	row := localdb.DBClient.QueryRow(`SELECT access_token, refresh_token, scope, expires_at FROM tokens ORDER BY id DESC LIMIT 1`)
	if err := row.Scan(&t.AccessToken, &t.RefreshToken, &t.Scope, &t.ExpiresAt); err != nil {
		return t, false, err
	}
	if time.Now().Unix() < t.ExpiresAt {
		return t, true, nil
	}
	return t, false, nil
}

func (t *Token) SaveToken() error {
	_, err := localdb.DBClient.Exec(`INSERT INTO tokens (access_token, refresh_token, scope, expires_at) VALUES (?, ?, ?, ?)`,
		t.AccessToken, t.RefreshToken, t.Scope, t.ExpiresAt)
	return err
}
