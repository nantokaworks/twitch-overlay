# プロジェクトガイドライン

## コミュニケーション
- すべてのチャットは日本語で行う
- ドキュメントも日本語で記載する

## テスト実行時の注意事項
- テストを実行する際は必ず `DRY_RUN_MODE=true` 環境変数を設定する
- これにより実際のプリンターへの印刷を防ぐ
- 例: `DRY_RUN_MODE=true go test ./...`

## 環境変数
### プリンター関連
- `ROTATE_PRINT`: プリンターに印刷する際に画像を180度回転させる（デフォルト: false）
  - プリンターの設置向きに合わせて使用する

### デバッグ関連
- フロントエンドで`?debug=true`パラメータを使用してデバッグパネルを表示可能
  - デバッグパネルではローカルモードで動作し、実際の印刷は行われない
  - `DEBUG_MODE=true`環境変数はバックエンドAPIモード用（通常は不要）

## プリンター接続管理

### KeepAlive機能の仕様
- go-catprinterモジュールには組み込みのKeepAlive機能が存在しない
- 長時間接続を維持するため、定期的にDisconnect→Reconnectを実行する必要がある
- この処理により、Bluetooth接続の安定性を保つ

### 実装上の注意事項
- **重要**: KeepAlive処理では必ず以下の手順を実行すること
  1. 既存接続をDisconnect
  2. 500ms程度の待機
  3. 再度Connectを実行
- **最も確実な方法**: Disconnectの後、catprinterインスタンス自体を再生成してから接続
  - これによりBLEデバイスレベルでのリセットが可能
  - 接続エラーが頻発する場合はこの方法を採用すること

### KeepAlive関連の環境変数
- `KEEP_ALIVE_ENABLED`: KeepAlive機能の有効/無効（デフォルト: false）
- `KEEP_ALIVE_INTERVAL`: KeepAliveの実行間隔（秒）（デフォルト: 60）

## Bluetooth権限設定（Linux環境）

### 権限が必要な理由
- go-catprinterはBluetoothデバイスにアクセスするためHCIソケットを使用
- 通常のユーザー権限ではHCIソケットにアクセスできない
- `cap_net_raw`と`cap_net_admin`のケーパビリティが必要

### 権限設定方法

#### 1. 自動設定（推奨）
```bash
# task build:all実行時に自動的に権限設定される
task build:all
```

#### 2. 手動設定
```bash
# ビルド済みバイナリに権限を付与
sudo setcap 'cap_net_raw,cap_net_admin+eip' dist/twitch-overlay

# 権限確認
getcap dist/twitch-overlay
```

#### 3. systemdサービスとして実行
```bash
# サービスインストール時に適切な権限設定が行われる
task service:install
```

### トラブルシューティング
- `can't init hci: no devices available`エラーが出る場合は権限設定を確認
- `bluetoothctl power on`でBluetoothアダプタの電源を確認
- `sudo usermod -a -G bluetooth $USER`でbluetoothグループに追加

## ビルド時の注意事項
- ビルドテストが完了したら、生成されたバイナリファイルは削除する
- 例: `go build ./cmd/twitch-overlay && rm twitch-overlay`
- リポジトリにバイナリファイルをコミットしない

## フロントエンド開発ガイドライン

### TypeScript化の注意事項
- フロントエンドはTypeScriptで実装されている
- アニメーション処理は`requestAnimationFrame`を使用して実装されており、型安全性を保ちながら動作する
- 定数ファイル（`layout.ts`）には動的なgetterメソッドが含まれている
- すべての型定義は`src/types/index.ts`に集約されている

### 開発コマンド
- `bun run dev`: 開発サーバーの起動
- `bun run build`: プロダクションビルド
- `bun run tsc --noEmit`: TypeScriptの型チェック
- `bun run lint`: ESLintの実行

## Goテストガイドライン

### テストフレームワーク
- Go標準ライブラリの `testing` パッケージを使用する
- 外部のテストフレームワークは明示的に要求されない限り使用しない

### ファイル構成
- テストファイルは必ず `_test.go` で終わる
- テストファイルはテスト対象のコードと同じパッケージ/ディレクトリに配置する
- 命名規則: `filename.go` → `filename_test.go`

### テスト関数の命名
- テスト関数名は `Test` で始まり、その後に関数名/メソッド名を続ける
- わかりやすい名前を使用: `TestFunctionName` または `TestTypeName_MethodName`
- サブテストには `t.Run()` を使用し、わかりやすい名前を付ける

### テストの構成
```go
// 基本的なテスト構造
func TestFunctionName(t *testing.T) {
    // 準備 (Arrange)
    // 実行 (Act)
    // 検証 (Assert)
}

// テーブル駆動テスト
func TestFunctionName(t *testing.T) {
    tests := []struct {
        name     string
        input    type
        expected type
    }{
        // テストケース
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // テストロジック
        })
    }
}
```

### ベストプラクティス
- 複数のテストケースにはテーブル駆動テストを使用
- テストは独立して実行できるようにする
- テストヘルパー関数には `t.Helper()` を使用
- テストフィクスチャは `testdata/` ディレクトリに配置
- 外部依存関係は必要に応じてモックする
- 並行実行可能なテストには `t.Parallel()` を使用
- AAA パターン（準備・実行・検証）に従う

## Git コミットガイドライン

### 重要：コミットルール
- **勝手にコミットしない** - ユーザーから明示的にコミットの指示があった場合のみコミットを実行する
- コード変更後は、変更内容の説明のみ行い、コミットは行わない

### コミットメッセージ絵文字ガイド

- 🐛 :bug: バグ修正
- 🎈 :balloon: 文字列変更や軽微な修正
- 👍 :+1: 機能改善
- ✨ :sparkles: 部分的な機能追加
- 🎉 :tada: 盛大に祝うべき大きな機能追加
- ♻️ :recycle: リファクタリング
- 🚿 :shower: 不要な機能・使われなくなった機能の削除
- 💚 :green_heart: テストやCIの修正・改善
- 👕 :shirt: Lintエラーの修正やコードスタイルの修正
- 🚀 :rocket: パフォーマンス改善
- 🆙 :up: 依存パッケージなどのアップデート
- 🔒 :lock: 新機能の公開範囲の制限
- 👮 :cop: セキュリティ関連の改善
- 🔧 :wrench: 設定関連変更
- 📝 :memo: ドキュメントの整理
- 🚧 :construction: 作業中

### コミットメッセージフォーマット

```
:emoji: Subject

Commit body...
```