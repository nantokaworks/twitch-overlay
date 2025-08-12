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

print_info "Twitch FAXサービスをユーザー '$USERNAME' でインストールします"

# ユーザーが存在するか確認
if ! id "$USERNAME" &>/dev/null; then
    print_error "ユーザー '$USERNAME' が存在しません"
    exit 1
fi

# twitch-faxディレクトリが存在するか確認
TWITCH_FAX_DIR="/home/$USERNAME/twitch-fax"
if [ ! -d "$TWITCH_FAX_DIR" ]; then
    print_error "ディレクトリ '$TWITCH_FAX_DIR' が存在しません"
    print_info "先にTwitch FAXをインストールしてください"
    exit 1
fi

# distディレクトリが存在するか確認
if [ ! -d "$TWITCH_FAX_DIR/dist" ]; then
    print_error "ディレクトリ '$TWITCH_FAX_DIR/dist' が存在しません"
    print_info "先に 'task build:all' でビルドしてください"
    exit 1
fi

# bluetoothグループにユーザーを追加するか確認
print_info "Bluetooth権限の設定方法を選択してください:"
echo "1) bluetoothグループにユーザーを追加（推奨）"
echo "2) systemdのCapabilitiesのみを使用"
echo -n "選択 [1/2]: "
read -r choice

if [ "$choice" = "1" ] || [ -z "$choice" ]; then
    print_info "ユーザー '$USERNAME' をbluetoothグループに追加します"
    if sudo usermod -a -G bluetooth "$USERNAME"; then
        print_success "bluetoothグループに追加しました"
        print_warning "変更を反映するには再ログインが必要です"
    else
        print_error "bluetoothグループへの追加に失敗しました"
    fi
fi

# systemdサービスファイルをコピー
print_info "systemdサービスファイルをインストールします"
SERVICE_FILE="$TWITCH_FAX_DIR/systemd/twitch-fax.service"
if [ ! -f "$SERVICE_FILE" ]; then
    print_error "サービスファイル '$SERVICE_FILE' が見つかりません"
    exit 1
fi

# サービスファイルをsystemdディレクトリにコピー
sudo cp "$SERVICE_FILE" "/etc/systemd/system/twitch-fax@$USERNAME.service"
print_success "サービスファイルをインストールしました"

# systemdをリロード
print_info "systemdデーモンをリロードします"
sudo systemctl daemon-reload
print_success "systemdデーモンをリロードしました"

# サービスを有効化するか確認
echo -n "サービスを自動起動に登録しますか？ [Y/n]: "
read -r enable_service
if [ "$enable_service" != "n" ] && [ "$enable_service" != "N" ]; then
    sudo systemctl enable "twitch-fax@$USERNAME.service"
    print_success "サービスを自動起動に登録しました"
fi

# サービスを今すぐ起動するか確認
echo -n "サービスを今すぐ起動しますか？ [Y/n]: "
read -r start_service
if [ "$start_service" != "n" ] && [ "$start_service" != "N" ]; then
    sudo systemctl start "twitch-fax@$USERNAME.service"
    print_success "サービスを起動しました"
    
    # ステータスを表示
    print_info "サービスの状態:"
    sudo systemctl status "twitch-fax@$USERNAME.service" --no-pager
fi

print_success "インストールが完了しました！"
echo ""
print_info "以下のコマンドでサービスを管理できます:"
echo "  起動: sudo systemctl start twitch-fax@$USERNAME.service"
echo "  停止: sudo systemctl stop twitch-fax@$USERNAME.service"
echo "  再起動: sudo systemctl restart twitch-fax@$USERNAME.service"
echo "  状態確認: sudo systemctl status twitch-fax@$USERNAME.service"
echo "  ログ確認: sudo journalctl -u twitch-fax@$USERNAME.service -f"