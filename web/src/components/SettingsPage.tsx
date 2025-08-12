import React, { useState, useEffect, useRef } from 'react';
import { ArrowLeft, Settings2, Bluetooth, Wifi, Zap, Eye, EyeOff, FileText, Upload, X } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { Tabs, TabsContent, TabsList, TabsTrigger } from './ui/tabs';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from './ui/card';
import { Button } from './ui/button';
import { Input } from './ui/input';
import { Label } from './ui/label';
import { Switch } from './ui/switch';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from './ui/select';
import { Alert, AlertDescription } from './ui/alert';
import { LogViewer } from './LogViewer';
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
import { toast } from 'sonner';

export const SettingsPage: React.FC = () => {
  const navigate = useNavigate();
  const [activeTab, setActiveTab] = useState('general');
  const [settings, setSettings] = useState<Record<string, any>>({});
  const [featureStatus, setFeatureStatus] = useState<FeatureStatus | null>(null);
  const [bluetoothDevices, setBluetoothDevices] = useState<BluetoothDevice[]>([]);
  const [scanning, setScanning] = useState(false);
  const [testing, setTesting] = useState(false);
  const [unsavedChanges, setUnsavedChanges] = useState<UpdateSettingsRequest>({});
  const [showSecrets, setShowSecrets] = useState<Record<string, boolean>>({});
  const [uploadingFont, setUploadingFont] = useState(false);
  const [previewImage, setPreviewImage] = useState<string>('');
  const [previewText, setPreviewText] = useState<string>('サンプルテキスト Sample Text 123\nフォントプレビュー 🎨');
  const fileInputRef = useRef<HTMLInputElement>(null);
  const saveTimeoutRef = useRef<NodeJS.Timeout | undefined>(undefined);

  // 設定データの取得
  useEffect(() => {
    fetchAllSettings();
  }, []);
  
  // 設定読み込み時に現在のプリンターアドレスをデバイスリストに追加
  useEffect(() => {
    const currentAddress = getSettingValue('PRINTER_ADDRESS');
    if (currentAddress && bluetoothDevices.length === 0) {
      setBluetoothDevices([{
        mac_address: currentAddress,
        name: '(現在の設定)',
        last_seen: new Date().toISOString()
      }]);
    }
  }, [settings]);

  const fetchAllSettings = async () => {
    try {
      const response = await fetch(buildApiUrl('/api/settings/v2'));
      if (!response.ok) throw new Error('Failed to fetch settings');
      
      const data: SettingsResponse = await response.json();
      setSettings(data.settings);
      setFeatureStatus(data.status);
    } catch (err: any) {
      toast.error('設定の取得に失敗しました: ' + err.message);
    }
  };

  const handleSettingChange = (key: string, value: string | boolean | number) => {
    const stringValue = typeof value === 'boolean' ? (value ? 'true' : 'false') : String(value);
    setUnsavedChanges(prev => ({
      ...prev,
      [key]: stringValue
    }));
    
    // 自動保存のタイマーをリセット
    if (saveTimeoutRef.current) {
      clearTimeout(saveTimeoutRef.current);
    }
    
    // 1.5秒後に自動保存
    saveTimeoutRef.current = setTimeout(() => {
      handleAutoSave(key, stringValue);
    }, 1500);
  };
  
  const handleAutoSave = async (key: string, value: string) => {
    try {
      const response = await fetch(buildApiUrl('/api/settings/v2'), {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ [key]: value }),
      });

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(errorText);
      }

      const data: any = await response.json();
      toast.success(`設定を保存しました: ${key}`);
      setFeatureStatus(data.status);
      
      // 更新された設定をローカル状態に反映
      if (data.settings && data.settings[key]) {
        setSettings(prev => ({
          ...prev,
          [key]: data.settings[key]
        }));
      }
      
      setUnsavedChanges(prev => {
        const updated = { ...prev };
        delete updated[key];
        return updated;
      });
    } catch (err: any) {
      toast.error('設定の保存に失敗しました: ' + err.message);
    }
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


  const handleScanDevices = async () => {
    setScanning(true);
    try {
      const response = await fetch(buildApiUrl('/api/printer/scan'), { 
        method: 'POST' 
      });
      
      const data: ScanResponse = await response.json();
      if (data.status === 'success') {
        // 現在の設定値を保持
        const currentAddress = getSettingValue('PRINTER_ADDRESS');
        let updatedDevices = [...data.devices];
        
        // 現在の設定値がスキャン結果に含まれていない場合、追加
        if (currentAddress && !data.devices.find(d => d.mac_address === currentAddress)) {
          updatedDevices.unshift({
            mac_address: currentAddress,
            name: '(現在の設定)',
            last_seen: new Date().toISOString()
          });
        }
        
        setBluetoothDevices(updatedDevices);
        toast.success(`${data.devices.length}台のデバイスが見つかりました`);
      } else {
        throw new Error(data.message || 'Scan failed');
      }
    } catch (err: any) {
      toast.error('デバイススキャンに失敗しました: ' + err.message);
    } finally {
      setScanning(false);
    }
  };

  const handleTestConnection = async () => {
    const printerAddress = getSettingValue('PRINTER_ADDRESS');
    if (!printerAddress) {
      toast.error('プリンターアドレスが選択されていません');
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
        toast.success('プリンターとの接続に成功しました');
      } else {
        toast.error('接続テスト失敗: ' + data.message);
      }
    } catch (err: any) {
      toast.error('接続テストでエラーが発生しました: ' + err.message);
    } finally {
      setTesting(false);
    }
  };

  const handleFontUpload = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) return;

    // ファイル形式の確認
    if (!file.name.endsWith('.ttf') && !file.name.endsWith('.otf')) {
      toast.error('フォントファイルは.ttfまたは.otf形式である必要があります');
      return;
    }

    setUploadingFont(true);
    const formData = new FormData();
    formData.append('font', file);

    try {
      const response = await fetch(buildApiUrl('/api/settings/font'), {
        method: 'POST',
        body: formData,
      });

      if (!response.ok) {
        const error = await response.text();
        throw new Error(error);
      }

      const result = await response.json();
      
      // フォント情報を取得
      const fontInfo = result.font;
      const fontName = fontInfo?.filename || file.name;
      
      toast.success(`フォント「${fontName}」をアップロードしました`);

      // 設定を更新
      if (fontName) {
        handleSettingChange('FONT_FILENAME', fontName);
      }
      
      // 設定を再取得して画面を更新
      await fetchAllSettings();

      // ファイル入力をリセット
      if (fileInputRef.current) {
        fileInputRef.current.value = '';
      }
    } catch (err: any) {
      toast.error('フォントのアップロードに失敗しました: ' + err.message);
    } finally {
      setUploadingFont(false);
    }
  };

  const handleDeleteFont = async () => {
    try {
      const response = await fetch(buildApiUrl('/api/settings/font'), {
        method: 'DELETE',
      });

      if (!response.ok) {
        throw new Error('Failed to delete font');
      }

      toast.success('フォントを削除しました');
      
      // フォント設定をクリア
      handleSettingChange('FONT_FILENAME', '');
      
      // 設定を再取得
      await fetchAllSettings();
    } catch (err: any) {
      toast.error('フォントの削除に失敗しました: ' + err.message);
    }
  };

  const handleFontPreview = async () => {
    try {
      const response = await fetch(buildApiUrl('/api/settings/font/preview'), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ 
          text: previewText 
        }),
      });

      if (!response.ok) {
        throw new Error('Failed to generate preview');
      }

      const result = await response.json();
      if (result.image) {
        setPreviewImage(result.image);
        toast.success('プレビューを生成しました');
      } else {
        throw new Error('No image data received');
      }
    } catch (err: any) {
      toast.error('プレビューの生成に失敗しました: ' + err.message);
    }
  };

  return (
    <div className="min-h-screen bg-gray-50" style={{ fontFamily: 'system-ui, -apple-system, sans-serif' }}>
      {/* ヘッダー */}
      <div className="bg-white shadow-sm border-b">
        <div className="max-w-6xl mx-auto px-4 py-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center space-x-4">
              <Button
                variant="ghost"
                size="sm"
                onClick={() => navigate('/')}
                className="flex items-center space-x-2"
              >
                <ArrowLeft className="w-4 h-4" />
                <span>FAX画面に戻る</span>
              </Button>
              <div className="flex items-center space-x-2">
                <Settings2 className="w-6 h-6 text-gray-600" />
                <h1 className="text-2xl font-bold">設定</h1>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div className="max-w-6xl mx-auto px-4 py-6">
        {/* ステータスカード */}
        {featureStatus && (
          <Card className="mb-6">
            <CardHeader>
              <CardTitle className="text-lg">システム状態</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                <div className="flex items-center space-x-2">
                  <div className={`w-3 h-3 rounded-full ${featureStatus.twitch_configured ? 'bg-green-500' : 'bg-red-500'}`} />
                  <span className="font-medium">Twitch連携</span>
                  <span className="text-sm text-gray-500">
                    {featureStatus.twitch_configured ? '設定済み' : '未設定'}
                  </span>
                </div>
                <div className="flex items-center space-x-2">
                  <div className={`w-3 h-3 rounded-full ${featureStatus.printer_configured ? 'bg-green-500' : 'bg-red-500'}`} />
                  <span className="font-medium">プリンター</span>
                  <span className="text-sm text-gray-500">
                    {featureStatus.printer_configured ? '設定済み' : '未設定'}
                  </span>
                </div>
                <div className="flex items-center space-x-2">
                  <div className={`w-3 h-3 rounded-full ${featureStatus.warnings.length === 0 ? 'bg-green-500' : 'bg-yellow-500'}`} />
                  <span className="font-medium">警告</span>
                  <span className="text-sm text-gray-500">
                    {featureStatus.warnings.length}件
                  </span>
                </div>
              </div>
              {featureStatus.missing_settings.length > 0 && (
                <div className="mt-4 p-3 bg-yellow-50 rounded-lg">
                  <p className="text-sm text-yellow-800">
                    <strong>未設定項目:</strong> {featureStatus.missing_settings.join(', ')}
                  </p>
                </div>
              )}
            </CardContent>
          </Card>
        )}

        {/* タブコンテンツ */}
        <Tabs value={activeTab} onValueChange={setActiveTab}>
          <TabsList className="grid w-full grid-cols-5 mb-6">
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
            <TabsTrigger value="behavior" className="flex items-center space-x-2">
              <Zap className="w-4 h-4" />
              <span>動作</span>
            </TabsTrigger>
            <TabsTrigger value="logs" className="flex items-center space-x-2">
              <FileText className="w-4 h-4" />
              <span>ログ</span>
            </TabsTrigger>
          </TabsList>

          {/* 一般タブ */}
          <TabsContent value="general" className="space-y-6">
            <Card>
              <CardHeader>
                <CardTitle>基本設定</CardTitle>
                <CardDescription>
                  アプリケーションの基本的な動作を設定します
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-6">
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

                <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
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
                </div>
              </CardContent>
            </Card>

            {/* 時計表示設定カード */}
            <Card>
              <CardHeader>
                <CardTitle>時計表示設定</CardTitle>
                <CardDescription>
                  時計に表示される数値を設定します（配信ネタ用）
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-6">
                <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                  <div className="space-y-2">
                    <Label htmlFor="clock_weight">体重（kg）</Label>
                    <Input
                      id="clock_weight"
                      type="number"
                      step="0.1"
                      min="0"
                      max="999.9"
                      placeholder="75.4"
                      value={getSettingValue('CLOCK_WEIGHT') || '75.4'}
                      onChange={(e) => handleSettingChange('CLOCK_WEIGHT', e.target.value)}
                      className="max-w-xs"
                    />
                    <p className="text-sm text-gray-500">
                      時計に表示する体重（小数点1桁）
                    </p>
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="clock_wallet">さいふ（円）</Label>
                    <Input
                      id="clock_wallet"
                      type="number"
                      min="0"
                      max="9999999"
                      placeholder="10387"
                      value={getSettingValue('CLOCK_WALLET') || '10387'}
                      onChange={(e) => handleSettingChange('CLOCK_WALLET', e.target.value)}
                      className="max-w-xs"
                    />
                    <p className="text-sm text-gray-500">
                      時計に表示する財布の金額（整数値）
                    </p>
                  </div>
                </div>
              </CardContent>
            </Card>

            {/* フォント設定カード */}
            <Card>
              <CardHeader>
                <CardTitle>フォント設定（必須）</CardTitle>
                <CardDescription>
                  FAXと時計機能を使用するためにフォントのアップロードが必要です
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-6">
                {!getSettingValue('FONT_FILENAME') && (
                  <Alert>
                    <AlertDescription className="text-yellow-700">
                      ⚠️ フォントがアップロードされていません。FAXと時計機能を使用するには、フォントファイル（.ttf/.otf）をアップロードしてください。
                    </AlertDescription>
                  </Alert>
                )}

                <div className="space-y-4">
                    <div className="space-y-2">
                      <Label>フォントファイルをアップロード</Label>
                      <div className="flex items-center gap-2">
                        <input
                          ref={fileInputRef}
                          type="file"
                          accept=".ttf,.otf"
                          onChange={handleFontUpload}
                          className="hidden"
                        />
                        <Button
                          onClick={() => fileInputRef.current?.click()}
                          disabled={uploadingFont}
                          variant="outline"
                          className="flex items-center gap-2"
                        >
                          <Upload className="h-4 w-4" />
                          {uploadingFont ? 'アップロード中...' : 'フォントをアップロード'}
                        </Button>
                        <span className="text-sm text-gray-500">
                          .ttf または .otf ファイル
                        </span>
                      </div>
                    </div>

                    {getSettingValue('FONT_FILENAME') && (
                      <>
                        <div className="space-y-2">
                          <Label>現在のフォント</Label>
                          <div className="flex items-center gap-2">
                            <Input
                              value={getSettingValue('FONT_FILENAME')}
                              disabled
                              className="max-w-xs"
                            />
                            <Button
                              onClick={handleDeleteFont}
                              variant="outline"
                              size="sm"
                              className="text-red-600 hover:text-red-700"
                            >
                              <X className="h-4 w-4" />
                              削除
                            </Button>
                          </div>
                        </div>
                        
                        {/* フォントプレビュー */}
                        <div className="space-y-2">
                          <Label>フォントプレビュー</Label>
                          <div className="space-y-2">
                            <textarea
                              value={previewText}
                              onChange={(e) => setPreviewText(e.target.value)}
                              className="w-full p-2 border rounded-md min-h-[80px] font-mono text-sm"
                              placeholder="プレビューテキストを入力..."
                            />
                            <Button
                              onClick={handleFontPreview}
                              variant="outline"
                              disabled={!getSettingValue('FONT_FILENAME')}
                            >
                              プレビューを生成
                            </Button>
                          </div>
                          {previewImage && (
                            <div className="mt-2 p-4 bg-gray-100 rounded">
                              <img 
                                src={previewImage} 
                                alt="Font Preview" 
                                className="max-w-full h-auto border border-gray-300"
                                style={{ imageRendering: 'pixelated' }}
                              />
                            </div>
                          )}
                        </div>
                      </>
                    )}
                </div>
              </CardContent>
            </Card>

          </TabsContent>

          {/* Twitchタブ */}
          <TabsContent value="twitch" className="space-y-6">
            <Card>
              <CardHeader>
                <CardTitle>Twitch API設定</CardTitle>
                <CardDescription>
                  Twitch Developersで取得したAPI情報を設定してください
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-6">
                <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                  <div className="space-y-2">
                    <Label htmlFor="client_id">Client ID *</Label>
                    <div className="relative">
                      <Input
                        id="client_id"
                        type={showSecrets['CLIENT_ID'] ? "text" : "password"}
                        placeholder={settings['CLIENT_ID']?.has_value ? "（設定済み）" : "Twitch Client ID"}
                        value={unsavedChanges['CLIENT_ID'] !== undefined ? unsavedChanges['CLIENT_ID'] : getSettingValue('CLIENT_ID')}
                        onChange={(e) => handleSettingChange('CLIENT_ID', e.target.value)}
                        onKeyDown={(e) => {
                          if (e.key === 'Escape' && unsavedChanges['CLIENT_ID'] !== undefined) {
                            setUnsavedChanges(prev => {
                              const updated = { ...prev };
                              delete updated['CLIENT_ID'];
                              return updated;
                            });
                          }
                        }}
                        className="pr-10"
                      />
                      <button
                        type="button"
                        onClick={() => setShowSecrets(prev => ({ ...prev, CLIENT_ID: !prev.CLIENT_ID }))}
                        className="absolute right-2 top-1/2 -translate-y-1/2 text-gray-500 hover:text-gray-700"
                      >
                        {showSecrets['CLIENT_ID'] ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                      </button>
                    </div>
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="client_secret">Client Secret *</Label>
                    <div className="relative">
                      <Input
                        id="client_secret"
                        type={showSecrets['CLIENT_SECRET'] ? "text" : "password"}
                        placeholder={settings['CLIENT_SECRET']?.has_value ? "（設定済み）" : "Twitch Client Secret"}
                        value={unsavedChanges['CLIENT_SECRET'] !== undefined ? unsavedChanges['CLIENT_SECRET'] : getSettingValue('CLIENT_SECRET')}
                        onChange={(e) => handleSettingChange('CLIENT_SECRET', e.target.value)}
                        className="pr-10"
                      />
                      <button
                        type="button"
                        onClick={() => setShowSecrets(prev => ({ ...prev, CLIENT_SECRET: !prev.CLIENT_SECRET }))}
                        className="absolute right-2 top-1/2 -translate-y-1/2 text-gray-500 hover:text-gray-700"
                      >
                        {showSecrets['CLIENT_SECRET'] ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                      </button>
                    </div>
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="user_id">ユーザーID *</Label>
                    <div className="relative">
                      <Input
                        id="user_id"
                        type={showSecrets['TWITCH_USER_ID'] ? "text" : "password"}
                        placeholder={settings['TWITCH_USER_ID']?.has_value ? "（設定済み）" : "監視対象のTwitchユーザーID"}
                        value={unsavedChanges['TWITCH_USER_ID'] !== undefined ? unsavedChanges['TWITCH_USER_ID'] : getSettingValue('TWITCH_USER_ID')}
                        onChange={(e) => handleSettingChange('TWITCH_USER_ID', e.target.value)}
                        className="pr-10"
                      />
                      <button
                        type="button"
                        onClick={() => setShowSecrets(prev => ({ ...prev, TWITCH_USER_ID: !prev.TWITCH_USER_ID }))}
                        className="absolute right-2 top-1/2 -translate-y-1/2 text-gray-500 hover:text-gray-700"
                      >
                        {showSecrets['TWITCH_USER_ID'] ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                      </button>
                    </div>
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="reward_id">カスタムリワードID *</Label>
                    <div className="relative">
                      <Input
                        id="reward_id"
                        type={showSecrets['TRIGGER_CUSTOM_REWORD_ID'] ? "text" : "password"}
                        placeholder={settings['TRIGGER_CUSTOM_REWORD_ID']?.has_value ? "（設定済み）" : "FAX送信トリガーのカスタムリワードID"}
                        value={unsavedChanges['TRIGGER_CUSTOM_REWORD_ID'] !== undefined ? unsavedChanges['TRIGGER_CUSTOM_REWORD_ID'] : getSettingValue('TRIGGER_CUSTOM_REWORD_ID')}
                        onChange={(e) => handleSettingChange('TRIGGER_CUSTOM_REWORD_ID', e.target.value)}
                        className="pr-10"
                      />
                      <button
                        type="button"
                        onClick={() => setShowSecrets(prev => ({ ...prev, TRIGGER_CUSTOM_REWORD_ID: !prev.TRIGGER_CUSTOM_REWORD_ID }))}
                        className="absolute right-2 top-1/2 -translate-y-1/2 text-gray-500 hover:text-gray-700"
                      >
                        {showSecrets['TRIGGER_CUSTOM_REWORD_ID'] ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                      </button>
                    </div>
                  </div>
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          {/* プリンタータブ */}
          <TabsContent value="printer" className="space-y-6">
            <Card>
              <CardHeader>
                <CardTitle>プリンター設定</CardTitle>
                <CardDescription>
                  BluetoothプリンターのMACアドレスを設定してください
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-6">
                <div className="flex space-x-3">
                  <Button 
                    onClick={handleScanDevices} 
                    disabled={scanning}
                    variant="outline"
                    className="flex items-center space-x-2"
                  >
                    <Bluetooth className="w-4 h-4" />
                    <span>{scanning ? 'スキャン中...' : 'デバイススキャン'}</span>
                  </Button>
                  <Button 
                    onClick={handleTestConnection} 
                    disabled={testing || !getSettingValue('PRINTER_ADDRESS')}
                    variant="outline"
                    className="flex items-center space-x-2"
                  >
                    <span>{testing ? '接続テスト中...' : '接続テスト'}</span>
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

                <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
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
              </CardContent>
            </Card>
          </TabsContent>

          {/* 動作タブ */}
          <TabsContent value="behavior" className="space-y-6">
            <Card>
              <CardHeader>
                <CardTitle>動作設定</CardTitle>
                <CardDescription>
                  アプリケーションの動作に関する設定を行います
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-6">
                <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
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

                  <div className="flex items-center justify-between">
                    <div className="space-y-0.5">
                      <Label>初回印刷</Label>
                      <p className="text-sm text-gray-500">
                        起動時に時計を印刷します
                      </p>
                    </div>
                    <Switch
                      checked={getBooleanValue('INITIAL_PRINT_ENABLED')}
                      onCheckedChange={(checked) => handleSettingChange('INITIAL_PRINT_ENABLED', checked)}
                    />
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
                    className="max-w-xs"
                  />
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          {/* ログタブ */}
          <TabsContent value="logs" className="space-y-6">
            <LogViewer embedded={true} />
          </TabsContent>
        </Tabs>
      </div>
    </div>
  );
};