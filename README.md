# twitch-overlay

Twitchカスタムリワードと連携した配信用オーバーレイシステム

> **📝 利用について**  
> このプロジェクトは個人の配信環境向けにカスタマイズされています。  
> 時計の表示内容等に個人設定が含まれているため、技術実装の参考として、または改造ベースとしてご活用ください。

**主要機能**: Twitchリワード連携 | FAX風画像表示 | カスタマイズ可能時計 | サーマルプリンター印刷

## 機能

- Twitchチャンネルポイント報酬の自動印刷
- Webインターフェースでの設定管理
- カスタムフォントサポート
- 時計印刷機能
- プリンター設定のカスタマイズ

## セットアップ

### 必要要件

- Go 1.21以上
- Node.js 20以上 / Bun
- Bluetooth対応サーマルプリンター（Cat Printer）
- Linux環境（systemdサービス利用時）

### インストール

1. リポジトリをクローン
```bash
git clone https://github.com/nantokaworks/twitch-overlay.git
cd twitch-overlay
```

2. 依存関係をインストール
```bash
# フロントエンド
cd web && bun install && cd ..

# バックエンド（自動的にインストールされます）
```

3. 環境変数を設定
```bash
cp .env.template .env
# .envファイルを編集して必要な値を設定
```

4. ビルド
```bash
task build:all
```

### 実行方法

#### 方法1: 直接実行（setcap権限が必要）

```bash
# Bluetooth権限を付与
sudo setcap 'cap_net_admin,cap_net_raw+eip' ./dist/twitch-overlay

# 実行
./dist/twitch-overlay
```

#### 方法2: systemdサービスとして実行（推奨）

systemdサービスとして実行することで、setcapを毎回実行する必要がなくなります。

```bash
# サービスをインストール
task install:service

# または手動でインストール
bash scripts/install-service.sh [username]
```

インストール時のオプション:
- bluetoothグループへの追加（推奨）
- 自動起動の設定
- 即座にサービスを開始

サービス管理コマンド:
```bash
# サービスの状態確認
task service:status

# サービスのログ確認
task service:logs

# サービスの手動起動/停止/再起動
sudo systemctl start twitch-overlay@$USER.service
sudo systemctl stop twitch-overlay@$USER.service
sudo systemctl restart twitch-overlay@$USER.service

# サービスのアンインストール
task uninstall:service
```

### Twitch認証

1. アプリケーションを起動すると、認証URLが表示されます
2. ブラウザで `http://localhost:8080/auth` にアクセス
3. Twitchアカウントでログインして認証を完了

### Web設定ページ

ブラウザで `http://localhost:8080/settings` にアクセスして設定を変更できます。

## 開発

### 開発サーバーの起動

```bash
# フロントエンド
task dev:frontend

# バックエンド
task dev:backend
```

### テストの実行

```bash
# DRY_RUN_MODE=trueで実行（実際の印刷を防ぐ）
task test
```

## 環境変数

| 変数名 | 説明 | デフォルト値 |
|--------|------|------------|
| `PRINTER_ADDRESS` | プリンターのMACアドレス | 必須 |
| `DRY_RUN_MODE` | 実際の印刷を行わないモード | false |
| `ROTATE_PRINT` | 印刷を180度回転 | false |
| `CLOCK_ENABLED` | 時計印刷機能の有効化 | true |

詳細は `.env.template` を参照してください。

## トラブルシューティング

### Bluetooth権限エラー

以下のいずれかの方法で解決できます：

1. **bluetoothグループに追加（推奨）**
```bash
sudo usermod -a -G bluetooth $USER
# 再ログインが必要
```

2. **systemdサービスを使用**
```bash
task install:service
```

3. **setcapを使用（毎回必要）**
```bash
sudo setcap 'cap_net_admin,cap_net_raw+eip' ./dist/twitch-overlay
```

### プリンターが見つからない

1. プリンターの電源が入っているか確認
2. プリンターのMACアドレスが正しいか確認
3. Web設定画面（`http://localhost:8080/settings`）の「プリンター」タブから「デバイススキャン」を実行

## ライセンス

MIT