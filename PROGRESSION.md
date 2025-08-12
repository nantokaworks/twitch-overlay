# 環境変数のSQLite管理移行 - 実装手順書

## 概要

環境変数で管理していた設定をSQLiteデータベースに移行し、Webインターフェースから設定できるようにする。
特にセキュリティを重視し、プリンターのBluetoothアドレス選択を簡易化する。

## 全体のアーキテクチャ

```
環境変数 (.env) → SQLite (local.db) → Web設定画面
                     ↓
               機能の有効/無効制御
```

## Phase 1: 基盤設計とデータベース拡張

### 1.1 設定データベース設計

```sql
-- 既存のsettingsテーブルを使用
CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    setting_type TEXT NOT NULL DEFAULT 'normal', -- 'normal', 'secret'
    is_required BOOLEAN NOT NULL DEFAULT false,
    description TEXT,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### 1.2 設定管理モジュールの作成

**ファイル: `internal/settings/settings.go`**

```go
package settings

import (
    "database/sql"
    "encoding/json"
    "fmt"
    "time"
)

type SettingType string
const (
    SettingTypeNormal SettingType = "normal"
    SettingTypeSecret SettingType = "secret"
)

type Setting struct {
    Key          string      `json:"key"`
    Value        string      `json:"value"`
    Type         SettingType `json:"type"`
    Required     bool        `json:"required"`
    Description  string      `json:"description"`
    UpdatedAt    time.Time   `json:"updated_at"`
}

type SettingsManager struct {
    db *sql.DB
}

func NewSettingsManager(db *sql.DB) *SettingsManager {
    return &SettingsManager{db: db}
}

// 設定の定義
var DefaultSettings = map[string]Setting{
    // Twitch設定（機密情報）
    "CLIENT_ID": {
        Key: "CLIENT_ID", Value: "", Type: SettingTypeSecret, Required: true,
        Description: "Twitch API Client ID",
    },
    "CLIENT_SECRET": {
        Key: "CLIENT_SECRET", Value: "", Type: SettingTypeSecret, Required: true,
        Description: "Twitch API Client Secret",
    },
    "TWITCH_USER_ID": {
        Key: "TWITCH_USER_ID", Value: "", Type: SettingTypeSecret, Required: true,
        Description: "Twitch User ID for monitoring",
    },
    "TRIGGER_CUSTOM_REWORD_ID": {
        Key: "TRIGGER_CUSTOM_REWORD_ID", Value: "", Type: SettingTypeSecret, Required: true,
        Description: "Custom Reward ID for triggering FAX",
    },
    
    // プリンター設定
    "PRINTER_ADDRESS": {
        Key: "PRINTER_ADDRESS", Value: "", Type: SettingTypeNormal, Required: true,
        Description: "Bluetooth MAC address of the printer",
    },
    "DRY_RUN_MODE": {
        Key: "DRY_RUN_MODE", Value: "true", Type: SettingTypeNormal, Required: false,
        Description: "Enable dry run mode (no actual printing)",
    },
    "BEST_QUALITY": {
        Key: "BEST_QUALITY", Value: "false", Type: SettingTypeNormal, Required: false,
        Description: "Enable best quality printing",
    },
    "DITHER": {
        Key: "DITHER", Value: "false", Type: SettingTypeNormal, Required: false,
        Description: "Enable dithering",
    },
    "BLACK_POINT": {
        Key: "BLACK_POINT", Value: "100", Type: SettingTypeNormal, Required: false,
        Description: "Black point threshold (0-255)",
    },
    "AUTO_ROTATE": {
        Key: "AUTO_ROTATE", Value: "false", Type: SettingTypeNormal, Required: false,
        Description: "Auto rotate images",
    },
    "ROTATE_PRINT": {
        Key: "ROTATE_PRINT", Value: "false", Type: SettingTypeNormal, Required: false,
        Description: "Rotate print output 180 degrees",
    },
    "INITIAL_PRINT_ENABLED": {
        Key: "INITIAL_PRINT_ENABLED", Value: "false", Type: SettingTypeNormal, Required: false,
        Description: "Enable initial clock print on startup",
    },
    
    // 動作設定
    "KEEP_ALIVE_INTERVAL": {
        Key: "KEEP_ALIVE_INTERVAL", Value: "60", Type: SettingTypeNormal, Required: false,
        Description: "Keep alive interval in seconds",
    },
    "KEEP_ALIVE_ENABLED": {
        Key: "KEEP_ALIVE_ENABLED", Value: "false", Type: SettingTypeNormal, Required: false,
        Description: "Enable keep alive functionality",
    },
    "CLOCK_ENABLED": {
        Key: "CLOCK_ENABLED", Value: "false", Type: SettingTypeNormal, Required: false,
        Description: "Enable clock printing",
    },
    "DEBUG_OUTPUT": {
        Key: "DEBUG_OUTPUT", Value: "false", Type: SettingTypeNormal, Required: false,
        Description: "Enable debug output",
    },
    "TIMEZONE": {
        Key: "TIMEZONE", Value: "Asia/Tokyo", Type: SettingTypeNormal, Required: false,
        Description: "Timezone for clock display",
    },
}

// 機能の有効性チェック
type FeatureStatus struct {
    TwitchConfigured  bool     `json:"twitch_configured"`
    PrinterConfigured bool     `json:"printer_configured"`
    PrinterConnected  bool     `json:"printer_connected"`
    MissingSettings   []string `json:"missing_settings"`
    Warnings          []string `json:"warnings"`
}

func (sm *SettingsManager) CheckFeatureStatus() (*FeatureStatus, error) {
    status := &FeatureStatus{
        MissingSettings: []string{},
        Warnings:        []string{},
    }
    
    // Twitch設定チェック
    twitchSettings := []string{"CLIENT_ID", "CLIENT_SECRET", "TWITCH_USER_ID", "TRIGGER_CUSTOM_REWORD_ID"}
    twitchComplete := true
    for _, key := range twitchSettings {
        if val, err := sm.GetSetting(key); err != nil || val == "" {
            status.MissingSettings = append(status.MissingSettings, key)
            twitchComplete = false
        }
    }
    status.TwitchConfigured = twitchComplete
    
    // プリンター設定チェック
    if printerAddr, err := sm.GetSetting("PRINTER_ADDRESS"); err != nil || printerAddr == "" {
        status.MissingSettings = append(status.MissingSettings, "PRINTER_ADDRESS")
        status.PrinterConfigured = false
    } else {
        status.PrinterConfigured = true
        // TODO: 実際の接続テストを実装
        status.PrinterConnected = false
    }
    
    // 警告チェック
    if dryRun, _ := sm.GetSetting("DRY_RUN_MODE"); dryRun == "true" {
        status.Warnings = append(status.Warnings, "DRY_RUN_MODE is enabled - no actual printing will occur")
    }
    
    return status, nil
}

// CRUD操作
func (sm *SettingsManager) GetSetting(key string) (string, error) {
    var value string
    err := sm.db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
    if err == sql.ErrNoRows {
        // デフォルト値を返す
        if defaultSetting, exists := DefaultSettings[key]; exists {
            return defaultSetting.Value, nil
        }
        return "", fmt.Errorf("setting not found: %s", key)
    }
    return value, err
}

func (sm *SettingsManager) SetSetting(key, value string) error {
    _, err := sm.db.Exec(`
        INSERT INTO settings (key, value, setting_type, is_required, description) 
        VALUES (?, ?, ?, ?, ?) 
        ON CONFLICT(key) DO UPDATE SET 
            value = excluded.value, 
            updated_at = CURRENT_TIMESTAMP`,
        key, value,
        string(DefaultSettings[key].Type),
        DefaultSettings[key].Required,
        DefaultSettings[key].Description,
    )
    return err
}

func (sm *SettingsManager) GetAllSettings() (map[string]Setting, error) {
    rows, err := sm.db.Query(`
        SELECT key, value, setting_type, is_required, description, updated_at 
        FROM settings ORDER BY key`)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    settings := make(map[string]Setting)
    for rows.Next() {
        var s Setting
        var settingType string
        err := rows.Scan(&s.Key, &s.Value, &settingType, &s.Required, &s.Description, &s.UpdatedAt)
        if err != nil {
            return nil, err
        }
        s.Type = SettingType(settingType)
        
        // 機密情報はマスクして返す
        if s.Type == SettingTypeSecret && s.Value != "" {
            s.Value = "●●●●●●●●"
        }
        
        settings[s.Key] = s
    }
    
    // DBにない設定はデフォルト値で補完
    for key, defaultSetting := range DefaultSettings {
        if _, exists := settings[key]; !exists {
            settings[key] = defaultSetting
        }
    }
    
    return settings, nil
}

// 環境変数からの移行
func (sm *SettingsManager) MigrateFromEnv() error {
    for key := range DefaultSettings {
        // 既にDB設定が存在する場合はスキップ
        if _, err := sm.GetSetting(key); err == nil {
            continue
        }
        
        // 環境変数から取得
        if envValue := os.Getenv(key); envValue != "" {
            if err := sm.SetSetting(key, envValue); err != nil {
                return fmt.Errorf("failed to migrate %s: %w", key, err)
            }
        }
    }
    return nil
}
```

## Phase 2: プリンタースキャン機能の実装

### 2.1 プリンタースキャンAPI

**ファイル: `internal/webserver/printer_api.go`**

```go
package webserver

import (
    "encoding/json"
    "net/http"
    "time"
    
    "github.com/nantokaworks/twitch-fax/internal/output"
)

type BluetoothDevice struct {
    MACAddress     string    `json:"mac_address"`
    Name           string    `json:"name,omitempty"`
    SignalStrength int       `json:"signal_strength,omitempty"`
    LastSeen       time.Time `json:"last_seen"`
}

type ScanResponse struct {
    Devices []BluetoothDevice `json:"devices"`
    Status  string            `json:"status"`
    Message string            `json:"message,omitempty"`
}

func handlePrinterScan(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    // プリンタースキャンを実行
    c, err := output.SetupPrinter()
    if err != nil {
        http.Error(w, "Failed to setup scanner", http.StatusInternalServerError)
        return
    }
    defer c.Stop()
    
    // 10秒間スキャン
    c.Timeout = 10 * time.Second
    devices, err := c.ScanDevices("")
    
    response := ScanResponse{
        Devices: []BluetoothDevice{},
        Status:  "success",
    }
    
    if err != nil {
        response.Status = "error"
        response.Message = err.Error()
    } else {
        for mac, name := range devices {
            device := BluetoothDevice{
                MACAddress: mac,
                Name:       string(name),
                LastSeen:   time.Now(),
            }
            response.Devices = append(response.Devices, device)
        }
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

func handlePrinterTest(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    var req struct {
        MACAddress string `json:"mac_address"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }
    
    // プリンター接続テスト
    c, err := output.SetupPrinter()
    if err != nil {
        http.Error(w, "Failed to setup printer", http.StatusInternalServerError)
        return
    }
    defer c.Stop()
    
    err = output.ConnectPrinter(c, req.MACAddress)
    
    response := map[string]interface{}{
        "success": err == nil,
        "message": "",
    }
    
    if err != nil {
        response["message"] = err.Error()
    } else {
        response["message"] = "Connection successful"
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}
```

## Phase 3: 設定管理APIの実装

### 3.1 設定API

**ファイル: `internal/webserver/settings_api.go`** (handleSettings関数を拡張)

```go
func handleSettingsV2(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        handleGetSettings(w, r)
    case http.MethodPut:
        handleUpdateSettings(w, r)
    default:
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
}

func handleGetSettings(w http.ResponseWriter, r *http.Request) {
    settingsManager := settings.NewSettingsManager(localdb.GetDB())
    
    allSettings, err := settingsManager.GetAllSettings()
    if err != nil {
        http.Error(w, "Failed to get settings", http.StatusInternalServerError)
        return
    }
    
    featureStatus, err := settingsManager.CheckFeatureStatus()
    if err != nil {
        http.Error(w, "Failed to check feature status", http.StatusInternalServerError)
        return
    }
    
    response := map[string]interface{}{
        "settings": allSettings,
        "status":   featureStatus,
        "font":     fontmanager.GetCurrentFontInfo(), // 既存のフォント情報
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

func handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
    var req map[string]string
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }
    
    settingsManager := settings.NewSettingsManager(localdb.GetDB())
    
    // バリデーションと更新
    for key, value := range req {
        if err := validateSetting(key, value); err != nil {
            http.Error(w, fmt.Sprintf("Invalid value for %s: %v", key, err), http.StatusBadRequest)
            return
        }
        
        if err := settingsManager.SetSetting(key, value); err != nil {
            http.Error(w, fmt.Sprintf("Failed to update %s: %v", key, err), http.StatusInternalServerError)
            return
        }
    }
    
    // 更新後の設定状態を返す
    featureStatus, _ := settingsManager.CheckFeatureStatus()
    
    response := map[string]interface{}{
        "success": true,
        "status":  featureStatus,
        "message": "Settings updated successfully",
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

func validateSetting(key, value string) error {
    switch key {
    case "BLACK_POINT":
        if val, err := strconv.Atoi(value); err != nil || val < 0 || val > 255 {
            return fmt.Errorf("must be integer between 0 and 255")
        }
    case "KEEP_ALIVE_INTERVAL":
        if val, err := strconv.Atoi(value); err != nil || val < 10 || val > 3600 {
            return fmt.Errorf("must be integer between 10 and 3600 seconds")
        }
    case "PRINTER_ADDRESS":
        // MACアドレスの形式チェック
        if matched, _ := regexp.MatchString(`^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$`, value); !matched && value != "" {
            return fmt.Errorf("invalid MAC address format")
        }
    }
    return nil
}
```

## Phase 4: env.goの改修

### 4.1 データベース優先の設定読み込み

**ファイル: `internal/env/env.go`** の改修

```go
func init() {
    // 最初に.envファイルを読み込み（互換性のため）
    loadDotEnv()
    
    // データベースから設定を読み込み
    if err := loadFromDatabase(); err != nil {
        // DBエラー時は環境変数フォールバック
        logger.Warn("Failed to load from database, using environment variables", zap.Error(err))
        loadFromEnvironment()
    }
}

func loadFromDatabase() error {
    // まずデータベース接続を確立
    db, err := localdb.SetupDB("./local.db")
    if err != nil {
        return err
    }
    
    settingsManager := settings.NewSettingsManager(db)
    
    // 環境変数からの移行（初回のみ）
    if err := settingsManager.MigrateFromEnv(); err != nil {
        logger.Error("Failed to migrate from env", zap.Error(err))
    }
    
    // データベースから設定を読み込み
    clientID, err := settingsManager.GetSetting("CLIENT_ID")
    if err != nil {
        return err
    }
    
    clientSecret, err := settingsManager.GetSetting("CLIENT_SECRET")
    if err != nil {
        return err
    }
    
    // ... 他の設定も同様に読み込み
    
    // EnvValue構造体に設定
    Value = EnvValue{
        ClientID:              &clientID,
        ClientSecret:          &clientSecret,
        // ... 他のフィールド
    }
    
    // SERVER_PORTは環境変数のまま
    serverPort := getEnvOrDefault("SERVER_PORT", "8080")
    Value.ServerPort = parseInt(serverPort)
    
    return nil
}
```

## Phase 5: フロントエンド設定画面の実装

### 5.1 設定画面の拡張

**ファイル: `web/src/components/Settings.tsx`** を大幅拡張

```typescript
interface AppSettings {
  twitch: {
    client_id: string;
    client_secret: string;
    user_id: string;
    custom_reward_id: string;
  };
  printer: {
    address: string;
    dry_run_mode: boolean;
    best_quality: boolean;
    dither: boolean;
    black_point: number;
    auto_rotate: boolean;
    rotate_print: boolean;
    initial_print_enabled: boolean;
  };
  behavior: {
    keep_alive_interval: number;
    keep_alive_enabled: boolean;
    clock_enabled: boolean;
    debug_output: boolean;
    timezone: string;
  };
}

interface FeatureStatus {
  twitch_configured: boolean;
  printer_configured: boolean;
  printer_connected: boolean;
  missing_settings: string[];
  warnings: string[];
}

// プリンタースキャン関連
interface BluetoothDevice {
  mac_address: string;
  name?: string;
  signal_strength?: number;
  last_seen: string;
}
```

### 5.2 セットアップウィザードの実装

```typescript
const SetupWizard: React.FC = () => {
  const [step, setStep] = useState(1);
  const [settings, setSettings] = useState<Partial<AppSettings>>({});
  
  const steps = [
    { id: 1, title: 'Twitch API設定', component: TwitchSetup },
    { id: 2, title: 'プリンター設定', component: PrinterSetup },
    { id: 3, title: '動作設定', component: BehaviorSetup },
    { id: 4, title: '設定確認', component: ConfirmSetup },
  ];
  
  return (
    <div className="setup-wizard">
      <div className="progress-bar">
        {/* ステップインジケーター */}
      </div>
      <div className="step-content">
        {/* 各ステップのコンテンツ */}
      </div>
    </div>
  );
};
```

### 5.3 プリンタースキャン機能

```typescript
const PrinterSetup: React.FC = ({ settings, onUpdate }) => {
  const [devices, setDevices] = useState<BluetoothDevice[]>([]);
  const [scanning, setScanning] = useState(false);
  const [testing, setTesting] = useState(false);
  const [selectedDevice, setSelectedDevice] = useState('');
  
  const handleScan = async () => {
    setScanning(true);
    try {
      const response = await fetch('/api/printer/scan', { method: 'POST' });
      const data = await response.json();
      setDevices(data.devices || []);
    } catch (err) {
      console.error('Scan failed:', err);
    } finally {
      setScanning(false);
    }
  };
  
  const handleTest = async () => {
    setTesting(true);
    try {
      const response = await fetch('/api/printer/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ mac_address: selectedDevice }),
      });
      const data = await response.json();
      // テスト結果を表示
    } catch (err) {
      console.error('Test failed:', err);
    } finally {
      setTesting(false);
    }
  };
  
  return (
    <div className="printer-setup">
      <h3>プリンター設定</h3>
      
      <div className="scan-section">
        <button onClick={handleScan} disabled={scanning}>
          {scanning ? 'スキャン中...' : 'デバイスをスキャン'}
        </button>
        
        {devices.length > 0 && (
          <div className="device-list">
            <h4>見つかったデバイス</h4>
            {devices.map(device => (
              <div key={device.mac_address} className="device-item">
                <input
                  type="radio"
                  value={device.mac_address}
                  checked={selectedDevice === device.mac_address}
                  onChange={(e) => setSelectedDevice(e.target.value)}
                />
                <span>
                  {device.name || '(名前なし)'} - {device.mac_address}
                </span>
              </div>
            ))}
          </div>
        )}
      </div>
      
      <div className="manual-input">
        <label>または手動でMACアドレスを入力:</label>
        <input
          type="text"
          value={selectedDevice}
          onChange={(e) => setSelectedDevice(e.target.value)}
          placeholder="00:00:00:00:00:00"
          pattern="^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$"
        />
      </div>
      
      {selectedDevice && (
        <button onClick={handleTest} disabled={testing}>
          {testing ? '接続テスト中...' : '接続テスト'}
        </button>
      )}
    </div>
  );
};
```

## Phase 6: 移行処理とテスト

### 6.1 移行チェックリスト

- [ ] 既存の環境変数が正常にDBに移行されること
- [ ] DB優先、環境変数フォールバックが動作すること
- [ ] 設定変更がリアルタイムに反映されること
- [ ] プリンタースキャン機能が正常に動作すること
- [ ] セキュリティ情報が適切にマスクされること
- [ ] 必須設定不足時に機能が停止すること

### 6.2 テストシナリオ

1. **初回起動テスト**
   - 新しいDBでの起動
   - 環境変数からの移行
   - デフォルト値の適用

2. **設定変更テスト**
   - Web画面からの設定変更
   - バリデーション機能
   - 設定反映の確認

3. **プリンター機能テスト**
   - デバイススキャン
   - 接続テスト
   - 印刷機能の有効/無効制御

4. **セキュリティテスト**
   - 機密情報のマスキング
   - API認証（ローカルのみ）
   - ファイルアクセス制限

## 実装時の注意点

### セキュリティ考慮事項
- CLIENT_SECRETなどは.envから削除を促す警告を表示
- local.dbファイルの権限設定を確認
- API呼び出しは127.0.0.1からのみ許可

### パフォーマンス考慮事項
- 設定変更時のDB書き込みを最小化
- プリンタースキャンはオンデマンドのみ実行
- 設定キャッシュの実装を検討

### ユーザビリティ
- 初回セットアップウィザードの実装
- 設定変更時の明確なフィードバック
- エラーメッセージの日本語化

## 今後の拡張予定

1. **設定バックアップ/復元機能**
2. **設定変更履歴の記録**
3. **複数プリンター対応**
4. **設定のインポート/エクスポート**
5. **リモート設定管理（将来的）**

---

このPROGRESSION.mdファイルは実装の進捗に応じて更新していきます。
各Phaseの完了時にはチェックマークを付けて進捗を管理してください。