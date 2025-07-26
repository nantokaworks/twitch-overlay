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

## ビルド時の注意事項
- ビルドテストが完了したら、生成されたバイナリファイルは削除する
- 例: `go build ./cmd/twitch-fax && rm twitch-fax`
- リポジトリにバイナリファイルをコミットしない

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