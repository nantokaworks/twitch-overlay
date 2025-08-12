#!/bin/bash

# デバッグエンドポイントのテストスクリプト

echo "Testing debug channel points endpoint..."

# バックエンドが起動しているか確認
if ! lsof -i :8080 | grep -q LISTEN; then
    echo "❌ バックエンドが起動していません。以下のコマンドで起動してください:"
    echo "   DRY_RUN_MODE=true go run ./cmd/twitch-overlay"
    exit 1
fi

echo "✅ バックエンドが起動しています"

# デバッグエンドポイントをテスト
echo "Testing /debug/channel-points endpoint..."

curl -X POST http://localhost:8080/debug/channel-points \
    -H "Content-Type: application/json" \
    -d '{
        "username": "testuser",
        "displayName": "TestUser",
        "rewardTitle": "FAX送信",
        "userInput": "これはテストメッセージです"
    }' \
    -w "\nHTTP Status: %{http_code}\n"

echo ""
echo "テスト完了"