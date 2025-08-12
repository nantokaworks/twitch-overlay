package localdb

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

var DBClient *sql.DB

type Token struct {
	AccessToken  string
	RefreshToken string
	Scope        string
	ExpiresAt    int64
}

func SetupDB(dbPath string) (*sql.DB, error) {
	if DBClient != nil {
		return DBClient, nil
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	DBClient = db

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS tokens (
		id INTEGER PRIMARY KEY,
		access_token TEXT,
		refresh_token TEXT,
		scope TEXT,
		expires_at INTEGER
	)`)
	if err != nil {
		return nil, err
	}

	// settingsテーブルを追加
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		setting_type TEXT NOT NULL DEFAULT 'normal',
		is_required BOOLEAN NOT NULL DEFAULT false,
		description TEXT,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return nil, err
	}

	// 既存のsettingsテーブルに新しいカラムを追加（ALTER TABLEは既に存在する場合にはエラーになるが、それを無視）
	db.Exec(`ALTER TABLE settings ADD COLUMN setting_type TEXT NOT NULL DEFAULT 'normal'`)
	db.Exec(`ALTER TABLE settings ADD COLUMN is_required BOOLEAN NOT NULL DEFAULT false`)
	db.Exec(`ALTER TABLE settings ADD COLUMN description TEXT`)

	return db, nil
}

// GetDB は現在のデータベース接続を返します
func GetDB() *sql.DB {
	return DBClient
}
