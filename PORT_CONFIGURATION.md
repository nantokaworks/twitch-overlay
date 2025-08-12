# ポート設定ガイド

## 開発環境

### バックエンドサーバー

```bash
# デフォルト: 8080
SERVER_PORT=3000 go run cmd/twitch-overlay/main.go
```

### フロントエンド開発サーバー

```bash
# デフォルト: フロントエンド5173、バックエンド8080
cd web
VITE_FRONTEND_PORT=5000 VITE_BACKEND_PORT=3000 bun run dev
```

### 統合起動例

```bash
# ターミナル1: バックエンドを3000番ポートで起動
SERVER_PORT=3000 go run cmd/twitch-overlay/main.go

# ターミナル2: フロントエンドを起動（バックエンドの3000番ポートにプロキシ）
cd web
VITE_BACKEND_PORT=3000 bun run dev
```

## ビルド後（本番環境）

ビルド後は、フロントエンドは静的ファイルとしてバックエンドから配信されます。
そのため、**バックエンドのポート設定のみ**が必要です。

```bash
# ビルド
task build:all

# デフォルトポート（8080）で起動
./dist/twitch-overlay

# カスタムポート（3000）で起動
SERVER_PORT=3000 ./dist/twitch-overlay

# 80番ポートで起動（root権限が必要）
sudo SERVER_PORT=80 ./dist/twitch-overlay
```

### ビルド後の動作

1. バックエンドが指定されたポートで起動
2. 同じポートでフロントエンドの静的ファイルを配信
3. フロントエンドは相対パスでAPIにアクセス（同一ポート）

例：`SERVER_PORT=3000`で起動した場合
- Webインターフェース: http://localhost:3000
- API: http://localhost:3000/events, /status, /fax/* など

## 異なるホストでAPIを使用する場合

フロントエンドとバックエンドを別のホストで運用する場合は、ビルド時に`VITE_API_BASE_URL`を設定します：

```bash
cd web
VITE_API_BASE_URL=https://api.example.com bun run build
```