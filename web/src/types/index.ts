// FAX関連の型定義

// FAXの状態
export type FaxDisplayState = 'loading' | 'waiting' | 'scrolling' | 'displaying' | 'sliding' | 'sliding-up' | 'complete';

// FAXデータ
export interface FaxData {
  id: string;
  type: 'fax';
  timestamp: number;
  username: string;
  displayName: string;
  message: string;
  imageUrl?: string;
}

// FAX状態オブジェクト
export interface FaxState {
  state: FaxDisplayState;
  progress: number;
}

// 画像タイプ
export type ImageType = 'mono' | 'color';

// FaxDisplayコンポーネントのProps
export interface FaxDisplayProps {
  faxData: FaxData;
  onComplete: () => void;
  imageType: ImageType;
  onLabelPositionUpdate: (position: number) => void;
  onAnimationStateChange: (isAnimating: boolean) => void;
  onStateChange?: (state: FaxState) => void;
}

// FaxReceiverコンポーネントのProps
export interface FaxReceiverProps {
  imageType?: ImageType;
}

// useFaxQueueフックの戻り値
export interface UseFaxQueueReturn {
  queue: FaxData[];
  currentFax: FaxData | null;
  isDisplaying: boolean;
  addToQueue: (faxData: FaxData) => void;
  onDisplayComplete: () => void;
}

// サーバーステータス
export interface ServerStatus {
  printerConnected: boolean;
}

// アニメーション用の動的スタイル
export interface DynamicStyles {
  transform?: string;
  transition?: string;
  top?: string;
  left?: string;
  width?: string;
  height?: string;
  opacity?: number;
  animation?: string;
  backgroundColor?: string;
  fontSize?: string;
  marginRight?: string;
  padding?: string;
  boxSizing?: 'content-box' | 'border-box' | 'inherit' | 'initial' | 'unset';
}

// 設定関連の型定義

// 設定タイプ
export type SettingType = 'normal' | 'secret';

// 個別設定
export interface Setting {
  key: string;
  value: string;
  type: SettingType;
  required: boolean;
  description: string;
  updated_at: string;
  has_value?: boolean;
}

// アプリケーション設定
export interface AppSettings {
  twitch: {
    CLIENT_ID: string;
    CLIENT_SECRET: string;
    TWITCH_USER_ID: string;
    TRIGGER_CUSTOM_REWORD_ID: string;
  };
  printer: {
    PRINTER_ADDRESS: string;
    DRY_RUN_MODE: boolean;
    BEST_QUALITY: boolean;
    DITHER: boolean;
    BLACK_POINT: number;
    AUTO_ROTATE: boolean;
    ROTATE_PRINT: boolean;
  };
  behavior: {
    KEEP_ALIVE_INTERVAL: number;
    KEEP_ALIVE_ENABLED: boolean;
    CLOCK_ENABLED: boolean;
    DEBUG_OUTPUT: boolean;
    TIMEZONE: string;
  };
}

// 機能ステータス
export interface FeatureStatus {
  twitch_configured: boolean;
  printer_configured: boolean;
  printer_connected: boolean;
  missing_settings: string[];
  warnings: string[];
  service_mode?: boolean;  // systemdサービスとして実行されているか
}

// Twitchユーザー情報
export interface TwitchUserInfo {
  id: string;
  login: string;
  display_name: string;
  profile_image_url?: string;
  verified: boolean;
  error?: string;
}

// Twitch認証状態
export interface AuthStatus {
  authenticated: boolean;
  authUrl: string;
  expiresAt?: number | null;
  error?: string | null;
}

// 配信状態
export interface StreamStatus {
  is_live: boolean;
  started_at?: string;
  viewer_count: number;
  last_checked: string;
  duration_seconds?: number;
}

// プリンターステータス情報
export interface PrinterStatusInfo {
  connected: boolean;
  dry_run_mode: boolean;
  printer_address: string;
  configured: boolean;
  last_print?: string | null;
  print_queue?: number;
  error?: string;
}

// Bluetoothデバイス
export interface BluetoothDevice {
  mac_address: string;
  name?: string;
  signal_strength?: number;
  last_seen: string;
}

// プリンタースキャン結果
export interface ScanResponse {
  devices: BluetoothDevice[];
  status: string;
  message?: string;
}

// プリンター接続テスト結果
export interface TestResponse {
  success: boolean;
  message: string;
}

// 設定セクション
export interface SettingsSection {
  twitch: {
    configured: boolean;
    missing_fields: string[];
  };
  printer: {
    configured: boolean;
    connected: boolean;
  };
  features: {
    keep_alive: boolean;
    clock: boolean;
    event_sub: boolean;
  };
}

// 設定API応答
export interface SettingsResponse {
  settings: Record<string, Setting>;
  status: FeatureStatus;
  font: any; // 既存のフォント情報
}

// 設定更新リクエスト
export interface UpdateSettingsRequest {
  [key: string]: string;
}

// 設定更新応答
export interface UpdateSettingsResponse {
  success: boolean;
  status: FeatureStatus;
  message: string;
}