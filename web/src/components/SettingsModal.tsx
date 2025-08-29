import React, { useState, useEffect } from 'react';
import { X, Settings2, Bluetooth, Wifi } from 'lucide-react';
import { Tabs, TabsContent, TabsList, TabsTrigger } from './ui/tabs';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from './ui/card';
import { Button } from './ui/button';
import { Input } from './ui/input';
import { Label } from './ui/label';
import { Switch } from './ui/switch';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from './ui/select';
import { Alert, AlertDescription } from './ui/alert';
import { 
  FeatureStatus, 
  BluetoothDevice, 
  SettingsResponse,
  UpdateSettingsRequest,
  UpdateSettingsResponse,
  ScanResponse,
  TestResponse
} from '../types';
import { buildApiUrl } from '../utils/api';

interface SettingsModalProps {
  onClose: () => void;
}

export const SettingsModal: React.FC<SettingsModalProps> = ({ onClose }) => {
  const [activeTab, setActiveTab] = useState('general');
  const [settings, setSettings] = useState<Record<string, any>>({});
  const [featureStatus, setFeatureStatus] = useState<FeatureStatus | null>(null);
  const [bluetoothDevices, setBluetoothDevices] = useState<BluetoothDevice[]>([]);
  const [scanning, setScanning] = useState(false);
  const [testing, setTesting] = useState(false);
  const [saving, setSaving] = useState(false);
  const [unsavedChanges, setUnsavedChanges] = useState<UpdateSettingsRequest>({});
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');

  // 設定データの取得
  useEffect(() => {
    fetchAllSettings();
  }, []);

  const fetchAllSettings = async () => {
    try {
      const response = await fetch(buildApiUrl('/api/settings/v2'));
      if (!response.ok) throw new Error('Failed to fetch settings');
      
      const data: SettingsResponse = await response.json();
      setSettings(data.settings);
      setFeatureStatus(data.status);
    } catch (err: any) {
      setError('設定の取得に失敗しました: ' + err.message);
    }
  };

  const handleSettingChange = (key: string, value: string | boolean | number) => {
    const stringValue = typeof value === 'boolean' ? (value ? 'true' : 'false') : String(value);
    setUnsavedChanges(prev => ({
      ...prev,
      [key]: stringValue
    }));
  };

  const getSettingValue = (key: string): string => {
    if (key in unsavedChanges) {
      return unsavedChanges[key];
    }
    return settings[key]?.value || '';
  };

  const getBooleanValue = (key: string): boolean => {
    return getSettingValue(key) === 'true';
  };

  const getNumberValue = (key: string): number => {
    return parseInt(getSettingValue(key)) || 0;
  };

  const handleSave = async () => {
    if (Object.keys(unsavedChanges).length === 0) {
      setError('変更された設定がありません');
      return;
    }

    setSaving(true);
    setError('');
    try {
      const response = await fetch(buildApiUrl('/api/settings/v2'), {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(unsavedChanges),
      });

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(errorText);
      }

      const data: UpdateSettingsResponse = await response.json();
      setSuccess(data.message);
      setFeatureStatus(data.status);
      setUnsavedChanges({});
      await fetchAllSettings();
    } catch (err: any) {
      setError('設定の保存に失敗しました: ' + err.message);
    } finally {
      setSaving(false);
    }
  };

  const handleScanDevices = async () => {
    setScanning(true);
    setError('');
    try {
      const response = await fetch(buildApiUrl('/api/printer/scan'), { 
        method: 'POST' 
      });
      
      const data: ScanResponse = await response.json();
      if (data.status === 'success') {
        setBluetoothDevices(data.devices);
        setSuccess(`${data.devices.length}台のデバイスが見つかりました`);
      } else {
        throw new Error(data.message || 'Scan failed');
      }
    } catch (err: any) {
      setError('デバイススキャンに失敗しました: ' + err.message);
    } finally {
      setScanning(false);
    }
  };

  const handleTestConnection = async () => {
    const printerAddress = getSettingValue('PRINTER_ADDRESS');
    if (!printerAddress) {
      setError('プリンターアドレスが選択されていません');
      return;
    }

    setTesting(true);
    try {
      const response = await fetch(buildApiUrl('/api/printer/test'), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ mac_address: printerAddress }),
      });
      
      const data: TestResponse = await response.json();
      if (data.success) {
        setSuccess('プリンターとの接続に成功しました');
      } else {
        setError('接続テスト失敗: ' + data.message);
      }
    } catch (err: any) {
      setError('接続テストでエラーが発生しました: ' + err.message);
    } finally {
      setTesting(false);
    }
  };

  const hasUnsavedChanges = Object.keys(unsavedChanges).length > 0;

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center p-4 z-50">
      <div className="bg-white rounded-lg w-full max-w-4xl max-h-[90vh] overflow-hidden">
        {/* ヘッダー */}
        <div className="flex items-center justify-between p-6 border-b">
          <div className="flex items-center space-x-2">
            <Settings2 className="w-5 h-5 text-gray-600" />
            <h2 className="text-xl font-semibold">設定</h2>
          </div>
          <Button
            variant="ghost"
            size="icon"
            onClick={onClose}
          >
            <X className="w-4 h-4" />
          </Button>
        </div>

        {/* アラート */}
        {error && (
          <Alert variant="destructive" className="m-6 mb-0">
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        )}
        {success && (
          <Alert variant="success" className="m-6 mb-0">
            <AlertDescription>{success}</AlertDescription>
          </Alert>
        )}

        {/* ステータスバー */}
        {featureStatus && (
          <div className="px-6 py-4 bg-gray-50 border-b">
            <div className="flex items-center justify-between text-sm">
              <div className="flex items-center space-x-4">
                <div className="flex items-center space-x-2">
                  <div className={`w-2 h-2 rounded-full ${featureStatus.twitch_configured ? 'bg-green-500' : 'bg-red-500'}`} />
                  <span>Twitch連携</span>
                </div>
                <div className="flex items-center space-x-2">
                  <div className={`w-2 h-2 rounded-full ${featureStatus.printer_configured ? 'bg-green-500' : 'bg-red-500'}`} />
                  <span>プリンター</span>
                </div>
              </div>
              {featureStatus.warnings.length > 0 && (
                <span className="text-yellow-600">
                  {featureStatus.warnings.length}件の警告
                </span>
              )}
            </div>
          </div>
        )}

        {/* タブコンテンツ */}
        <div className="overflow-y-auto" style={{ maxHeight: 'calc(90vh - 200px)' }}>
          <Tabs value={activeTab} onValueChange={setActiveTab} className="p-6">
            <TabsList className="grid w-full grid-cols-3">
              <TabsTrigger value="general" className="flex items-center space-x-2">
                <Settings2 className="w-4 h-4" />
                <span>一般</span>
              </TabsTrigger>
              <TabsTrigger value="twitch" className="flex items-center space-x-2">
                <Wifi className="w-4 h-4" />
                <span>Twitch</span>
              </TabsTrigger>
              <TabsTrigger value="printer" className="flex items-center space-x-2">
                <Bluetooth className="w-4 h-4" />
                <span>プリンター</span>
              </TabsTrigger>
            </TabsList>

            {/* 一般タブ */}
            <TabsContent value="general" className="space-y-4">
              <Card>
                <CardHeader>
                  <CardTitle>基本設定</CardTitle>
                  <CardDescription>
                    アプリケーションの基本的な動作を設定します
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="space-y-2">
                    <Label htmlFor="timezone">タイムゾーン</Label>
                    <Select
                      value={getSettingValue('TIMEZONE')}
                      onValueChange={(value) => handleSettingChange('TIMEZONE', value)}
                    >
                      <SelectTrigger>
                        <SelectValue placeholder="タイムゾーンを選択" />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="Asia/Tokyo">Asia/Tokyo (JST)</SelectItem>
                        <SelectItem value="America/New_York">America/New_York (EST)</SelectItem>
                        <SelectItem value="Europe/London">Europe/London (GMT)</SelectItem>
                        <SelectItem value="UTC">UTC</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>

                  <div className="flex items-center justify-between">
                    <div className="space-y-0.5">
                      <Label>ドライランモード</Label>
                      <p className="text-sm text-gray-500">
                        実際の印刷を行わずテストします
                      </p>
                    </div>
                    <Switch
                      checked={getBooleanValue('DRY_RUN_MODE')}
                      onCheckedChange={(checked) => handleSettingChange('DRY_RUN_MODE', checked)}
                    />
                  </div>

                  <div className="flex items-center justify-between">
                    <div className="space-y-0.5">
                      <Label>デバッグ出力</Label>
                      <p className="text-sm text-gray-500">
                        詳細なログを出力します
                      </p>
                    </div>
                    <Switch
                      checked={getBooleanValue('DEBUG_OUTPUT')}
                      onCheckedChange={(checked) => handleSettingChange('DEBUG_OUTPUT', checked)}
                    />
                  </div>

                  <div className="flex items-center justify-between">
                    <div className="space-y-0.5">
                      <Label>時計機能</Label>
                      <p className="text-sm text-gray-500">
                        定期的に時計を印刷します
                      </p>
                    </div>
                    <Switch
                      checked={getBooleanValue('CLOCK_ENABLED')}
                      onCheckedChange={(checked) => handleSettingChange('CLOCK_ENABLED', checked)}
                    />
                  </div>

                  {getBooleanValue('CLOCK_ENABLED') && (
                    <div className="flex items-center justify-between">
                      <div className="space-y-0.5">
                        <Label>アイコン表示</Label>
                        <p className="text-sm text-gray-500">
                          時計にアイコンを表示します
                        </p>
                      </div>
                      <Switch
                        checked={getBooleanValue('CLOCK_SHOW_ICONS')}
                        onCheckedChange={(checked) => handleSettingChange('CLOCK_SHOW_ICONS', checked)}
                      />
                    </div>
                  )}
                </CardContent>
              </Card>
            </TabsContent>

            {/* Twitchタブ */}
            <TabsContent value="twitch" className="space-y-4">
              <Card>
                <CardHeader>
                  <CardTitle>Twitch API設定</CardTitle>
                  <CardDescription>
                    Twitch Developersで取得したAPI情報を設定してください
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="space-y-2">
                    <Label htmlFor="client_id">Client ID *</Label>
                    <Input
                      id="client_id"
                      type="text"
                      placeholder="Twitch Client ID"
                      value={getSettingValue('CLIENT_ID')}
                      onChange={(e) => handleSettingChange('CLIENT_ID', e.target.value)}
                    />
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="client_secret">Client Secret *</Label>
                    <Input
                      id="client_secret"
                      type="password"
                      placeholder="Twitch Client Secret"
                      value={getSettingValue('CLIENT_SECRET') === '●●●●●●●●' ? '' : getSettingValue('CLIENT_SECRET')}
                      onChange={(e) => handleSettingChange('CLIENT_SECRET', e.target.value)}
                    />
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="user_id">ユーザーID *</Label>
                    <Input
                      id="user_id"
                      type="text"
                      placeholder="監視対象のTwitchユーザーID"
                      value={getSettingValue('TWITCH_USER_ID')}
                      onChange={(e) => handleSettingChange('TWITCH_USER_ID', e.target.value)}
                    />
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="reward_id">カスタムリワードID *</Label>
                    <Input
                      id="reward_id"
                      type="text"
                      placeholder="FAX送信トリガーのカスタムリワードID"
                      value={getSettingValue('TRIGGER_CUSTOM_REWORD_ID')}
                      onChange={(e) => handleSettingChange('TRIGGER_CUSTOM_REWORD_ID', e.target.value)}
                    />
                  </div>
                </CardContent>
              </Card>
            </TabsContent>

            {/* プリンタータブ */}
            <TabsContent value="printer" className="space-y-4">
              <Card>
                <CardHeader>
                  <CardTitle>プリンター設定</CardTitle>
                  <CardDescription>
                    BluetoothプリンターのMACアドレスを設定してください
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="flex space-x-2">
                    <Button 
                      onClick={handleScanDevices} 
                      disabled={scanning}
                      variant="outline"
                    >
                      <Bluetooth className="w-4 h-4 mr-2" />
                      {scanning ? 'スキャン中...' : 'デバイススキャン'}
                    </Button>
                    <Button 
                      onClick={handleTestConnection} 
                      disabled={testing || !getSettingValue('PRINTER_ADDRESS')}
                      variant="outline"
                    >
                      {testing ? '接続テスト中...' : '接続テスト'}
                    </Button>
                  </div>

                  {bluetoothDevices.length > 0 && (
                    <div className="space-y-2">
                      <Label>見つかったデバイス</Label>
                      <Select
                        value={getSettingValue('PRINTER_ADDRESS')}
                        onValueChange={(value) => handleSettingChange('PRINTER_ADDRESS', value)}
                      >
                        <SelectTrigger>
                          <SelectValue placeholder="プリンターを選択してください" />
                        </SelectTrigger>
                        <SelectContent>
                          {bluetoothDevices.map((device) => (
                            <SelectItem key={device.mac_address} value={device.mac_address}>
                              {device.name || '(名前なし)'} - {device.mac_address}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                  )}

                  <div className="space-y-2">
                    <Label htmlFor="printer_address">プリンターMACアドレス *</Label>
                    <Input
                      id="printer_address"
                      type="text"
                      placeholder="00:00:00:00:00:00"
                      value={getSettingValue('PRINTER_ADDRESS')}
                      onChange={(e) => handleSettingChange('PRINTER_ADDRESS', e.target.value)}
                    />
                  </div>

                  <div className="grid grid-cols-2 gap-4">
                    <div className="flex items-center justify-between">
                      <div className="space-y-0.5">
                        <Label>高品質印刷</Label>
                        <p className="text-sm text-gray-500">印刷品質を向上</p>
                      </div>
                      <Switch
                        checked={getBooleanValue('BEST_QUALITY')}
                        onCheckedChange={(checked) => handleSettingChange('BEST_QUALITY', checked)}
                      />
                    </div>

                    <div className="flex items-center justify-between">
                      <div className="space-y-0.5">
                        <Label>ディザリング</Label>
                        <p className="text-sm text-gray-500">画像の濃淡を改善</p>
                      </div>
                      <Switch
                        checked={getBooleanValue('DITHER')}
                        onCheckedChange={(checked) => handleSettingChange('DITHER', checked)}
                      />
                    </div>

                    <div className="flex items-center justify-between">
                      <div className="space-y-0.5">
                        <Label>自動回転</Label>
                        <p className="text-sm text-gray-500">画像を自動で回転</p>
                      </div>
                      <Switch
                        checked={getBooleanValue('AUTO_ROTATE')}
                        onCheckedChange={(checked) => handleSettingChange('AUTO_ROTATE', checked)}
                      />
                    </div>

                    <div className="flex items-center justify-between">
                      <div className="space-y-0.5">
                        <Label>印刷回転</Label>
                        <p className="text-sm text-gray-500">出力を180度回転</p>
                      </div>
                      <Switch
                        checked={getBooleanValue('ROTATE_PRINT')}
                        onCheckedChange={(checked) => handleSettingChange('ROTATE_PRINT', checked)}
                      />
                    </div>
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="black_point">黒点しきい値</Label>
                    <Input
                      id="black_point"
                      type="number"
                      min="0"
                      max="255"
                      value={getNumberValue('BLACK_POINT')}
                      onChange={(e) => handleSettingChange('BLACK_POINT', parseInt(e.target.value))}
                    />
                    <p className="text-sm text-gray-500">0-255の値で黒色の判定しきい値を設定</p>
                  </div>

                  <div className="flex items-center justify-between">
                    <div className="space-y-0.5">
                      <Label>キープアライブ</Label>
                      <p className="text-sm text-gray-500">
                        プリンター接続を維持します
                      </p>
                    </div>
                    <Switch
                      checked={getBooleanValue('KEEP_ALIVE_ENABLED')}
                      onCheckedChange={(checked) => handleSettingChange('KEEP_ALIVE_ENABLED', checked)}
                    />
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="keep_alive_interval">キープアライブ間隔（秒）</Label>
                    <Input
                      id="keep_alive_interval"
                      type="number"
                      min="10"
                      max="3600"
                      value={getNumberValue('KEEP_ALIVE_INTERVAL')}
                      onChange={(e) => handleSettingChange('KEEP_ALIVE_INTERVAL', parseInt(e.target.value))}
                      disabled={!getBooleanValue('KEEP_ALIVE_ENABLED')}
                    />
                  </div>
                </CardContent>
              </Card>
            </TabsContent>

          </Tabs>
        </div>

        {/* フッター */}
        <div className="flex items-center justify-between p-6 border-t bg-gray-50">
          <div className="text-sm text-gray-500">
            {hasUnsavedChanges && (
              <span className="text-orange-600">
                未保存の変更があります
              </span>
            )}
          </div>
          <div className="flex space-x-3">
            <Button
              variant="outline"
              onClick={onClose}
            >
              キャンセル
            </Button>
            <Button
              onClick={handleSave}
              disabled={!hasUnsavedChanges || saving}
            >
              {saving ? '保存中...' : '設定を保存'}
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
};