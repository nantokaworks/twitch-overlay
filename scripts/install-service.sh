#!/bin/bash

set -e

# 色付き出力用の関数
print_info() {
    echo -e "\033[1;34m[INFO]\033[0m $1"
}

print_success() {
    echo -e "\033[1;32m[SUCCESS]\033[0m $1"
}

print_error() {
    echo -e "\033[1;31m[ERROR]\033[0m $1"
}

print_warning() {
    echo -e "\033[1;33m[WARNING]\033[0m $1"
}

# ユーザー名を取得
USERNAME=${1:-$USER}
if [ -z "$USERNAME" ]; then
    print_error "ユーザー名を指定してください"
    echo "使用方法: $0 [username]"
    exit 1
fi

print_info "Twitch Overlayサービスをユーザー '$USERNAME' でインストールします"

# ユーザーが存在するか確認
if ! id "$USERNAME" &>/dev/null; then
    print_error "ユーザー '$USERNAME' が存在しません"
    exit 1
fi

# twitch-overlayディレクトリが存在するか確認
TWITCH_OVERLAY_DIR="/home/$USERNAME/twitch-overlay"
if [ ! -d "$TWITCH_OVERLAY_DIR" ]; then
    print_error "ディレクトリ '$TWITCH_OVERLAY_DIR' が存在しません"
    print_info "先にTwitch Overlayをインストールしてください"
    exit 1
fi

# distディレクトリが存在するか確認
if [ ! -d "$TWITCH_OVERLAY_DIR/dist" ]; then
    print_error "ディレクトリ '$TWITCH_OVERLAY_DIR/dist' が存在しません"
    print_info "先に 'task build:all' でビルドしてください"
    exit 1
fi

# bluetoothグループの確認と作成
if ! getent group bluetooth >/dev/null 2>&1; then
    print_info "bluetoothグループが存在しません。作成します..."
    if sudo groupadd bluetooth; then
        print_success "bluetoothグループを作成しました"
    else
        print_error "bluetoothグループの作成に失敗しました"
        print_info "systemdのCapabilitiesのみを使用します"
    fi
fi

# bluetoothグループにユーザーを追加
if getent group bluetooth >/dev/null 2>&1; then
    # ユーザーが既にbluetoothグループに所属しているか確認
    if ! groups "$USERNAME" | grep -q bluetooth; then
        print_info "ユーザー '$USERNAME' をbluetoothグループに追加します"
        if sudo usermod -a -G bluetooth "$USERNAME"; then
            print_success "bluetoothグループに追加しました"
            print_warning "変更を反映するには再ログインが必要です"
        else
            print_error "bluetoothグループへの追加に失敗しました"
        fi
    else
        print_info "ユーザー '$USERNAME' は既にbluetoothグループのメンバーです"
    fi
else
    print_info "bluetoothグループが存在しないため、systemdのCapabilitiesのみを使用します"
fi

# systemdサービスファイルをコピー
print_info "systemdサービスファイルをインストールします"
SERVICE_FILE="$TWITCH_OVERLAY_DIR/systemd/twitch-overlay.service"
if [ ! -f "$SERVICE_FILE" ]; then
    print_error "サービスファイル '$SERVICE_FILE' が見つかりません"
    exit 1
fi

# サービスファイルをsystemdディレクトリにコピー
sudo cp "$SERVICE_FILE" "/etc/systemd/system/twitch-overlay@$USERNAME.service"
print_success "サービスファイルをインストールしました"

# systemdをリロード
print_info "systemdデーモンをリロードします"
sudo systemctl daemon-reload
print_success "systemdデーモンをリロードしました"

# サービスを有効化（自動起動）
print_info "サービスを自動起動に登録します"
sudo systemctl enable "twitch-overlay@$USERNAME.service"
print_success "サービスを自動起動に登録しました"

# サービスを起動
print_info "サービスを起動します"
sudo systemctl start "twitch-overlay@$USERNAME.service"
print_success "サービスを起動しました"

# ステータスを表示
print_info "サービスの状態:"
sudo systemctl status "twitch-overlay@$USERNAME.service" --no-pager

print_success "インストールが完了しました！"
echo ""
print_info "以下のコマンドでサービスを管理できます:"
echo "  起動: sudo systemctl start twitch-overlay@$USERNAME.service"
echo "  停止: sudo systemctl stop twitch-overlay@$USERNAME.service"
echo "  再起動: sudo systemctl restart twitch-overlay@$USERNAME.service"
echo "  状態確認: sudo systemctl status twitch-overlay@$USERNAME.service"
echo "  ログ確認: sudo journalctl -u twitch-overlay@$USERNAME.service -f"