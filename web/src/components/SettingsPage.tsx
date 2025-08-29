import React, { useState, useEffect, useRef } from 'react';
import { Settings2, Bluetooth, Wifi, Eye, EyeOff, FileText, Upload, X, RefreshCw, Server, Monitor, Bug, Radio, Sun, Moon, Music, Layers, Play, Pause, SkipForward, SkipBack, Volume2 } from 'lucide-react';
import { Tabs, TabsContent, TabsList, TabsTrigger } from './ui/tabs';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from './ui/card';
import { Button } from './ui/button';
import { Input } from './ui/input';
import { Label } from './ui/label';
import { Switch } from './ui/switch';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from './ui/select';
import { Alert, AlertDescription } from './ui/alert';
import { LogViewer } from './LogViewer';
import MusicManagerEmbed from './music/MusicManagerEmbed';
import { Tooltip } from './ui/tooltip';
import { useSettings } from '../contexts/SettingsContext';
import { 
  FeatureStatus, 
  BluetoothDevice, 
  SettingsResponse,
  UpdateSettingsRequest,
  ScanResponse,
  TestResponse,
  TwitchUserInfo,
  PrinterStatusInfo,
  AuthStatus,
  StreamStatus
} from '../types';
import type { Playlist, Track } from '../types/music';
import { buildApiUrl, buildEventSourceUrl } from '../utils/api';
import { toast } from 'sonner';
import { useTheme } from '../hooks/useTheme';

// タブ状態を保存するキー
const SETTINGS_TAB_KEY = 'settingsPage.activeTab';

export const SettingsPage: React.FC = () => {
  const { theme, toggleTheme } = useTheme();
  const { settings: overlaySettings, updateSettings: updateOverlaySettings } = useSettings();
  
  // localStorageから保存されたタブを取得、なければ'general'
  const [activeTab, setActiveTab] = useState(() => {
    try {
      return localStorage.getItem(SETTINGS_TAB_KEY) || 'general';
    } catch {
      return 'general';
    }
  });
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
  const [restarting, setRestarting] = useState(false);
  const [restartCountdown, setRestartCountdown] = useState(0);
  const [twitchUserInfo, setTwitchUserInfo] = useState<TwitchUserInfo | null>(null);
  const [verifyingTwitch, setVerifyingTwitch] = useState(false);
  const [printerStatusInfo, setPrinterStatusInfo] = useState<PrinterStatusInfo | null>(null);
  const [reconnectingPrinter, setReconnectingPrinter] = useState(false);
  const [authStatus, setAuthStatus] = useState<AuthStatus | null>(null);
  const [streamStatus, setStreamStatus] = useState<StreamStatus | null>(null);
  
  // 音楽プレイヤー制御用の状態
  const [playlists, setPlaylists] = useState<Playlist[]>([]);
  const [musicStatus, setMusicStatus] = useState<{
    is_playing: boolean;
    current_track?: Track;
    progress: number;
    current_time: number;
    duration: number;
    volume: number;
    playlist_name?: string;
  }>({
    is_playing: false,
    progress: 0,
    current_time: 0,
    duration: 0,
    volume: 70
  });

  // デバイスのソート関数
  const sortBluetoothDevices = (devices: BluetoothDevice[]): BluetoothDevice[] => {
    return [...devices].sort((a, b) => {
      // 両方名前がある場合: 名前でソート
      if (a.name && b.name) {
        // (現在の設定) を最上位に
        if (a.name === '(現在の設定)') return -1;
        if (b.name === '(現在の設定)') return 1;
        // それ以外は名前順
        return a.name.localeCompare(b.name);
      }
      // 片方だけ名前がある場合: 名前があるものを上に
      if (a.name && !b.name) return -1;
      if (!a.name && b.name) return 1;
      // 両方名前がない場合: MACアドレスでソート
      return a.mac_address.localeCompare(b.mac_address);
    });
  };

  // activeTabが変更されたときにlocalStorageに保存
  useEffect(() => {
    try {
      localStorage.setItem(SETTINGS_TAB_KEY, activeTab);
    } catch (error) {
      console.error('Failed to save active tab:', error);
    }
  }, [activeTab]);

  // 設定データの取得
  useEffect(() => {
    fetchAllSettings();
    fetchAuthStatus();
    fetchStreamStatus();
  }, []);

  // 配信状態の定期更新
  useEffect(() => {
    const interval = setInterval(() => {
      fetchStreamStatus();
    }, 30000); // 30秒ごとに更新

    return () => clearInterval(interval);
  }, []);

  // Twitch連携が設定済みの場合、ユーザー情報を検証
  useEffect(() => {
    if (featureStatus?.twitch_configured && authStatus?.authenticated) {
      verifyTwitchConfig();
    }
  }, [featureStatus?.twitch_configured, authStatus?.authenticated]);

  // プリンター設定済みの場合、プリンター状態を取得
  useEffect(() => {
    if (featureStatus?.printer_configured) {
      fetchPrinterStatus();
    }
  }, [featureStatus?.printer_configured]);
  
  // プレイリスト一覧を取得
  useEffect(() => {
    fetch(buildApiUrl('/api/music/playlists'))
      .then(res => res.json())
      .then(data => {
        setPlaylists(data.playlists || []);
      })
      .catch(console.error);
  }, []);
  
  // オーバーレイからの音楽状態を受信
  useEffect(() => {
    const eventSource = new EventSource(buildEventSourceUrl('/api/music/status/events'));
    
    eventSource.onmessage = (event) => {
      try {
        const status = JSON.parse(event.data);
        setMusicStatus(status);
      } catch (error) {
        console.error('Failed to parse music status:', error);
      }
    };
    
    eventSource.onerror = (error) => {
      console.error('Music status SSE error:', error);
    };
    
    return () => {
      eventSource.close();
    };
  }, []);
  
  // SSE接続でプリンター状態の変更をリアルタイムで受信
  useEffect(() => {
    let eventSource: EventSource | null = null;
    let reconnectTimeout: NodeJS.Timeout | null = null;
    
    const connect = () => {
      eventSource = new EventSource(buildEventSourceUrl('/events'));
      
      eventSource.onopen = () => {
        console.log('Settings SSE connection opened');
        if (reconnectTimeout) {
          clearTimeout(reconnectTimeout);
          reconnectTimeout = null;
        }
      };
      
      eventSource.onmessage = (event: MessageEvent) => {
        try {
          const data = JSON.parse(event.data);
          
          // プリンター接続/切断イベントを処理
          if (data.type === 'printer_connected' || data.type === 'printer_disconnected') {
            console.log('Printer status changed:', data.type);
            // プリンター状態を再取得
            fetchPrinterStatus();
          }
          
          // 配信状態変更イベントも処理
          if (data.type === 'stream_online' || data.type === 'stream_offline') {
            console.log('Stream status changed:', data.type);
            fetchStreamStatus();
          }
        } catch (error) {
          console.error('Failed to parse SSE message:', error);
        }
      };
      
      eventSource.onerror = (error: Event) => {
        console.error('Settings SSE connection error:', error);
        eventSource?.close();
        
        // 3秒後に再接続を試みる
        reconnectTimeout = setTimeout(() => {
          console.log('Attempting to reconnect SSE...');
          connect();
        }, 3000);
      };
    };
    
    connect();
    
    return () => {
      if (reconnectTimeout) {
        clearTimeout(reconnectTimeout);
      }
      eventSource?.close();
    };
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

  // 音楽リモートコントロール関数
  const sendMusicControlCommand = async (endpoint: string, body?: any) => {
    try {
      const options: RequestInit = {
        method: 'POST'
      };
      
      // Only add headers and body if needed
      if (body) {
        options.headers = { 'Content-Type': 'application/json' };
        options.body = JSON.stringify(body);
      }
      
      await fetch(buildApiUrl(`/api/music/control/${endpoint}`), options);
    } catch (error) {
      console.error(`Failed to send ${endpoint} command:`, error);
    }
  };
  
  const handleSeek = (time: number) => {
    sendMusicControlCommand('seek', { time });
  };

  // 時間フォーマット
  const formatTime = (seconds: number) => {
    const mins = Math.floor(seconds / 60);
    const secs = Math.floor(seconds % 60);
    return `${mins}:${secs.toString().padStart(2, '0')}`;
  };

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


  const verifyTwitchConfig = async () => {
    setVerifyingTwitch(true);
    try {
      const response = await fetch(buildApiUrl('/api/twitch/verify'));
      const data: TwitchUserInfo = await response.json();
      
      setTwitchUserInfo(data);
      
      if (data.verified) {
        toast.success(`Twitch連携確認: ${data.display_name} (${data.login})`);
      } else if (data.error) {
        toast.error(`Twitch連携エラー: ${data.error}`);
      }
    } catch (err: any) {
      toast.error('Twitch連携の検証に失敗しました');
      setTwitchUserInfo({
        id: '',
        login: '',
        display_name: '',
        verified: false,
        error: '検証に失敗しました'
      });
    } finally {
      setVerifyingTwitch(false);
    }
  };

  const fetchPrinterStatus = async () => {
    try {
      const response = await fetch(buildApiUrl('/api/printer/status'));
      const data: PrinterStatusInfo = await response.json();
      setPrinterStatusInfo(data);
    } catch (err: any) {
      console.error('Failed to fetch printer status:', err);
    }
  };

  const fetchAuthStatus = async () => {
    try {
      const response = await fetch(buildApiUrl('/api/settings/auth/status'));
      const data: AuthStatus = await response.json();
      setAuthStatus(data);
    } catch (err: any) {
      console.error('Failed to fetch auth status:', err);
    }
  };

  const fetchStreamStatus = async () => {
    try {
      const response = await fetch(buildApiUrl('/api/stream/status'));
      const data: StreamStatus = await response.json();
      setStreamStatus(data);
    } catch (err: any) {
      console.error('Failed to fetch stream status:', err);
    }
  };

  const handleTwitchAuth = () => {
    // Redirect to Twitch OAuth
    window.location.href = buildApiUrl('/auth');
  };

  const handlePrinterReconnect = async () => {
    setReconnectingPrinter(true);
    try {
      const response = await fetch(buildApiUrl('/api/printer/reconnect'), {
        method: 'POST',
      });
      
      const data = await response.json();
      
      if (data.success) {
        toast.success('プリンターに再接続しました');
        // 状態を更新
        await fetchPrinterStatus();
      } else {
        toast.error(`再接続エラー: ${data.error || '接続に失敗しました'}`);
      }
    } catch (err: any) {
      toast.error('プリンター再接続に失敗しました');
    } finally {
      setReconnectingPrinter(false);
    }
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
        
        // デバイスをソート
        const sortedDevices = sortBluetoothDevices(updatedDevices);
        
        setBluetoothDevices(sortedDevices);
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

  const handleTokenRefresh = async () => {
    try {
      const response = await fetch(buildApiUrl('/api/twitch/refresh-token'), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
      });
      
      const result = await response.json();
      
      if (!response.ok) {
        throw new Error(result.error || 'Token refresh failed');
      }
      
      if (result.success) {
        toast.success('トークンを更新しました');
        // 認証状態を再取得
        await fetchAuthStatus();
      } else {
        throw new Error(result.error || 'トークンの更新に失敗しました');
      }
    } catch (err: any) {
      toast.error(`トークンの更新に失敗しました: ${err.message}`);
    }
  };

  const handleBluetoothRestart = async () => {
    if (!confirm('Bluetoothサービスを再起動しますか？\nプリンター接続が一時的に切断されます。')) {
      return;
    }

    try {
      const response = await fetch(buildApiUrl('/api/bluetooth/restart'), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
      });
      
      const result = await response.json();
      
      if (response.ok) {
        toast.success(result.message || 'Bluetoothサービスを再起動しました');
      } else {
        toast.error(result.message || 'Bluetoothサービスの再起動に失敗しました');
      }
    } catch (err) {
      toast.error('Bluetoothサービスの再起動に失敗しました');
    }
  };

  const handleServerRestart = async (force: boolean = false) => {
    const confirmMessage = force 
      ? 'サーバーを強制的に再起動しますか？\n処理中のタスクがある場合は中断されます。'
      : 'サーバーを再起動しますか？';
    
    if (!confirm(confirmMessage)) {
      return;
    }
    
    setRestarting(true);
    setRestartCountdown(10); // 10秒のカウントダウン
    
    try {
      const response = await fetch(buildApiUrl('/api/server/restart'), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ force }),
      });
      
      const result = await response.json();
      
      if (!response.ok) {
        if (response.status === 409 && result.warning) {
          // 印刷キューが空でない場合
          toast.warning(result.warning);
          if (confirm('強制的に再起動しますか？')) {
            await handleServerRestart(true); // 強制再起動を再実行
          }
          setRestarting(false);
          return;
        }
        throw new Error(result.message || 'Restart failed');
      }
      
      toast.success(result.message);
      
      // カウントダウンを開始
      let countdown = 10;
      const countdownInterval = setInterval(() => {
        countdown--;
        setRestartCountdown(countdown);
        
        if (countdown <= 0) {
          clearInterval(countdownInterval);
          // 自動リロード
          window.location.reload();
        }
      }, 1000);
      
      // 5秒後から接続確認を開始
      setTimeout(() => {
        const checkInterval = setInterval(async () => {
          try {
            const statusResponse = await fetch(buildApiUrl('/api/server/status'));
            if (statusResponse.ok) {
              clearInterval(checkInterval);
              clearInterval(countdownInterval);
              window.location.reload();
            }
          } catch (err) {
            // まだ接続できない
          }
        }, 1000);
      }, 5000);
      
    } catch (err: any) {
      toast.error('再起動に失敗しました: ' + err.message);
      setRestarting(false);
      setRestartCountdown(0);
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
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900 transition-colors" style={{ fontFamily: 'system-ui, -apple-system, sans-serif' }}>
      {/* ヘッダー */}
      <div className="bg-white dark:bg-gray-800 shadow-sm border-b dark:border-gray-700 transition-colors">
        <div className="max-w-6xl mx-auto px-4 py-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center space-x-4">
              <div className="flex items-center space-x-2">
                <Settings2 className="w-6 h-6 text-gray-600 dark:text-gray-400" />
                <h1 className="text-2xl font-bold dark:text-white">設定</h1>
              </div>
              <div className="flex flex-wrap gap-2">
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => window.open('/', '_blank')}
                  className="flex items-center space-x-1"
                >
                  <Monitor className="w-3 h-3" />
                  <span>モノクロ表示</span>
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => window.open('/?debug=true', '_blank')}
                  className="flex items-center space-x-1"
                >
                  <Monitor className="w-3 h-3" />
                  <Bug className="w-3 h-3" />
                  <span>モノクロ＋デバッグ</span>
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => window.open('/color', '_blank')}
                  className="flex items-center space-x-1"
                >
                  <Monitor className="w-3 h-3" />
                  <span>カラー表示</span>
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => window.open('/color?debug=true', '_blank')}
                  className="flex items-center space-x-1"
                >
                  <Monitor className="w-3 h-3" />
                  <Bug className="w-3 h-3" />
                  <span>カラー＋デバッグ</span>
                </Button>
              </div>
            </div>
            <Button
              size="sm"
              variant="outline"
              onClick={toggleTheme}
              className="flex items-center space-x-1"
              title="テーマを切り替え"
            >
              {theme === 'light' ? (
                <Moon className="w-4 h-4" />
              ) : (
                <Sun className="w-4 h-4" />
              )}
            </Button>
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
                <div className="space-y-1">
                  <div className="flex items-center space-x-2">
                    <div className={`w-3 h-3 rounded-full ${featureStatus.twitch_configured ? 'bg-green-500' : 'bg-red-500'}`} />
                    <span className="font-medium dark:text-gray-200">Twitch連携</span>
                    <span className="text-sm text-gray-500 dark:text-gray-400">
                      {featureStatus.twitch_configured ? '設定済み' : '未設定'}
                    </span>
                  </div>
                  {featureStatus.twitch_configured && authStatus && !authStatus.authenticated && (
                    <div className="ml-5 text-sm">
                      <div className="flex items-center space-x-2">
                        <span className="text-orange-600">
                          ⚠️ Twitch認証が必要です
                        </span>
                        <Button
                          size="sm"
                          variant="default"
                          onClick={handleTwitchAuth}
                          className="h-6 px-2 text-xs"
                        >
                          Twitchで認証
                        </Button>
                      </div>
                    </div>
                  )}
                  {featureStatus.twitch_configured && authStatus?.authenticated && twitchUserInfo && (
                    <div className="ml-5 text-sm">
                      {twitchUserInfo.verified ? (
                        <div className="space-y-1">
                          <div className="flex items-center space-x-2">
                            <span className="text-gray-600 dark:text-gray-300">
                              ユーザー: {twitchUserInfo.login} ({twitchUserInfo.display_name})
                            </span>
                            <Button
                              size="sm"
                              variant="ghost"
                              onClick={verifyTwitchConfig}
                              disabled={verifyingTwitch}
                              className="h-6 px-2 text-xs"
                            >
                              {verifyingTwitch ? '検証中...' : '検証'}
                            </Button>
                          </div>
                          {streamStatus && (
                            <div className="flex items-center space-x-2">
                              {streamStatus.is_live ? (
                                <>
                                  <Radio className="w-4 h-4 text-red-500 animate-pulse" />
                                  <span className="text-red-600 font-medium">配信中</span>
                                  {streamStatus.viewer_count > 0 && (
                                    <span className="text-gray-500 dark:text-gray-400">
                                      (視聴者: {streamStatus.viewer_count}人)
                                    </span>
                                  )}
                                  {streamStatus.duration_seconds && (
                                    <span className="text-gray-500 dark:text-gray-400">
                                      {Math.floor(streamStatus.duration_seconds / 3600)}時間
                                      {Math.floor((streamStatus.duration_seconds % 3600) / 60)}分
                                    </span>
                                  )}
                                </>
                              ) : (
                                <>
                                  <div className="w-4 h-4 rounded-full bg-gray-400" />
                                  <span className="text-gray-500 dark:text-gray-400">オフライン</span>
                                </>
                              )}
                            </div>
                          )}
                        </div>
                      ) : (
                        <div className="flex items-center space-x-2">
                          <span className="text-red-600">
                            ⚠️ {twitchUserInfo.error || '設定エラー'}
                          </span>
                          <Button
                            size="sm"
                            variant="ghost"
                            onClick={verifyTwitchConfig}
                            disabled={verifyingTwitch}
                            className="h-6 px-2 text-xs"
                          >
                            {verifyingTwitch ? '検証中...' : '再検証'}
                          </Button>
                        </div>
                      )}
                    </div>
                  )}
                  {featureStatus.twitch_configured && authStatus?.authenticated && !twitchUserInfo && verifyingTwitch && (
                    <div className="ml-5 text-sm text-gray-500 dark:text-gray-400">
                      検証中...
                    </div>
                  )}
                </div>
                <div className="space-y-1">
                  <div className="flex items-center space-x-2">
                    <div className={`w-3 h-3 rounded-full ${
                      !featureStatus.printer_configured ? 'bg-red-500' : 
                      printerStatusInfo?.connected ? 'bg-green-500' : 'bg-yellow-500'
                    }`} />
                    <span className="font-medium dark:text-gray-200">プリンター</span>
                    <span className="text-sm text-gray-500 dark:text-gray-400">
                      {featureStatus.printer_configured ? '設定済み' : '未設定'}
                    </span>
                  </div>
                  {featureStatus.printer_configured && printerStatusInfo && (
                    <div className="ml-5 text-sm">
                      <div className="flex items-center space-x-2">
                        <span className="text-gray-600 dark:text-gray-300">
                          接続状態: {printerStatusInfo.connected ? '接続中' : '未接続'}
                          {printerStatusInfo.dry_run_mode && ' (DRY-RUN)'}
                        </span>
                        <Button
                          size="sm"
                          variant="ghost"
                          onClick={handlePrinterReconnect}
                          disabled={reconnectingPrinter}
                          className="h-6 px-2 text-xs"
                        >
                          {reconnectingPrinter ? '再接続中...' : '再接続'}
                        </Button>
                      </div>
                    </div>
                  )}
                </div>
                <Tooltip
                  content={
                    featureStatus.warnings.length > 0 ? (
                      <div className="space-y-2">
                        <div className="font-medium text-yellow-200 flex items-center">
                          <span className="w-2 h-2 bg-yellow-400 rounded-full mr-2"></span>
                          システム警告 ({featureStatus.warnings.length}件)
                        </div>
                        <div className="space-y-1">
                          {featureStatus.warnings.map((warning, index) => (
                            <div key={index} className="text-sm pl-4 border-l-2 border-yellow-400/30">
                              {index + 1}. {warning}
                            </div>
                          ))}
                        </div>
                        <div className="text-xs opacity-75 pt-1 border-t border-gray-600">
                          マウスオーバーで詳細を確認
                        </div>
                      </div>
                    ) : (
                      <div className="flex items-center space-x-2">
                        <div className="w-2 h-2 bg-green-400 rounded-full"></div>
                        <span className="text-sm">システムに問題はありません</span>
                      </div>
                    )
                  }
                  position="bottom"
                  delay={200}
                >
                  <div className="flex items-center space-x-2 cursor-help">
                    <div className={`w-3 h-3 rounded-full ${featureStatus.warnings.length === 0 ? 'bg-green-500' : 'bg-yellow-500'}`} />
                    <span className="font-medium dark:text-gray-200">警告</span>
                    <span className="text-sm text-gray-500 dark:text-gray-400">
                      {featureStatus.warnings.length}件
                    </span>
                  </div>
                </Tooltip>
              </div>
              {featureStatus.missing_settings.length > 0 && (
                <div className="mt-4 p-3 bg-yellow-50 dark:bg-yellow-900/20 rounded-lg">
                  <Tooltip
                    content={
                      <div className="space-y-2">
                        <div className="font-medium text-orange-200 flex items-center">
                          <span className="w-2 h-2 bg-orange-400 rounded-full mr-2"></span>
                          未設定項目 ({featureStatus.missing_settings.length}件)
                        </div>
                        <div className="space-y-1">
                          {featureStatus.missing_settings.map((setting, index) => (
                            <div key={index} className="text-sm pl-4 border-l-2 border-orange-400/30">
                              • {setting}
                            </div>
                          ))}
                        </div>
                        <div className="text-xs opacity-75 pt-1 border-t border-gray-600">
                          これらを設定すると全機能が利用できます
                        </div>
                      </div>
                    }
                    position="top"
                    delay={200}
                  >
                    <p className="text-sm text-yellow-800 dark:text-yellow-200 cursor-help">
                      <strong>未設定項目:</strong> {featureStatus.missing_settings.join(', ')}
                    </p>
                  </Tooltip>
                </div>
              )}
            </CardContent>
          </Card>
        )}

        {/* タブコンテンツ */}
        <Tabs value={activeTab} onValueChange={setActiveTab}>
          <TabsList className="grid w-full grid-cols-8 mb-6 dark:bg-gray-800 dark:border-gray-700">
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
            <TabsTrigger value="music" className="flex items-center space-x-2">
              <Music className="w-4 h-4" />
              <span>音楽</span>
            </TabsTrigger>
            <TabsTrigger value="overlay" className="flex items-center space-x-2">
              <Layers className="w-4 h-4" />
              <span>オーバーレイ</span>
            </TabsTrigger>
            <TabsTrigger value="logs" className="flex items-center space-x-2">
              <FileText className="w-4 h-4" />
              <span>ログ</span>
            </TabsTrigger>
            <TabsTrigger value="system" className="flex items-center space-x-2">
              <Server className="w-4 h-4" />
              <span>システム</span>
            </TabsTrigger>
            <TabsTrigger value="api" className="flex items-center space-x-2">
              <Bug className="w-4 h-4" />
              <span>API</span>
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
                      <p className="text-sm text-gray-500 dark:text-gray-400">
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
                      <Label>オフライン時自動ドライラン</Label>
                      <p className="text-sm text-gray-500 dark:text-gray-400">
                        配信オフライン時に自動でドライランモードに切り替えます
                      </p>
                    </div>
                    <Switch
                      checked={getBooleanValue('AUTO_DRY_RUN_WHEN_OFFLINE')}
                      onCheckedChange={(checked) => handleSettingChange('AUTO_DRY_RUN_WHEN_OFFLINE', checked)}
                      disabled={getBooleanValue('DRY_RUN_MODE')}
                    />
                  </div>
                </div>

                <div className="flex items-center justify-between">
                  <div className="space-y-0.5">
                    <Label>デバッグ出力</Label>
                    <p className="text-sm text-gray-500 dark:text-gray-400">
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
                    <p className="text-sm text-gray-500 dark:text-gray-400">
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
                      <p className="text-sm text-gray-500 dark:text-gray-400">
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
                  <Alert className="dark:bg-yellow-900/20 dark:border-yellow-700">
                    <AlertDescription className="text-yellow-700 dark:text-yellow-200">
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
                        <span className="text-sm text-gray-500 dark:text-gray-400">
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
                              className="w-full p-2 border dark:border-gray-600 dark:bg-gray-700 dark:text-white rounded-md min-h-[80px] font-mono text-sm"
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
                            <div className="mt-2 p-4 bg-gray-100 dark:bg-gray-700 rounded">
                              <img 
                                src={previewImage} 
                                alt="Font Preview" 
                                className="max-w-full h-auto border border-gray-300 dark:border-gray-600"
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
                {/* 認証状態の表示 */}
                {authStatus && (
                  <div className="p-4 bg-gray-50 dark:bg-gray-700 rounded-lg border dark:border-gray-600">
                    <div className="flex items-center justify-between">
                      <div>
                        <h3 className="text-sm font-medium dark:text-gray-200">
                          認証状態: {authStatus.authenticated ? (
                            <span className="text-green-600">認証済み</span>
                          ) : (
                            <span className="text-orange-600">未認証</span>
                          )}
                        </h3>
                        {authStatus.error && (
                          <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">{authStatus.error}</p>
                        )}
                        {authStatus.authenticated && authStatus.expiresAt && (
                          <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                            有効期限: {new Date(authStatus.expiresAt * 1000).toLocaleString()}
                          </p>
                        )}
                      </div>
                      {!authStatus.authenticated && (
                        <Button
                          onClick={handleTwitchAuth}
                          variant="default"
                          className="flex items-center space-x-2"
                        >
                          <Wifi className="w-4 h-4" />
                          <span>Twitchで認証</span>
                        </Button>
                      )}
                      {authStatus.authenticated && (
                        <div className="flex items-center space-x-2">
                          <Button
                            onClick={handleTokenRefresh}
                            variant="outline"
                            className="flex items-center space-x-2"
                          >
                            <RefreshCw className="w-4 h-4" />
                            <span>トークンを更新</span>
                          </Button>
                          <Button
                            onClick={handleTwitchAuth}
                            variant="ghost"
                            className="flex items-center space-x-2"
                          >
                            <RefreshCw className="w-4 h-4" />
                            <span>再認証</span>
                          </Button>
                        </div>
                      )}
                    </div>
                  </div>
                )}
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
                        className="absolute right-2 top-1/2 -translate-y-1/2 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300"
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
                        className="absolute right-2 top-1/2 -translate-y-1/2 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300"
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
                        className="absolute right-2 top-1/2 -translate-y-1/2 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300"
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
                        className="absolute right-2 top-1/2 -translate-y-1/2 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300"
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
                      <p className="text-sm text-gray-500 dark:text-gray-400">印刷品質を向上</p>
                    </div>
                    <Switch
                      checked={getBooleanValue('BEST_QUALITY')}
                      onCheckedChange={(checked) => handleSettingChange('BEST_QUALITY', checked)}
                    />
                  </div>

                  <div className="flex items-center justify-between">
                    <div className="space-y-0.5">
                      <Label>ディザリング</Label>
                      <p className="text-sm text-gray-500 dark:text-gray-400">画像の濃淡を改善</p>
                    </div>
                    <Switch
                      checked={getBooleanValue('DITHER')}
                      onCheckedChange={(checked) => handleSettingChange('DITHER', checked)}
                    />
                  </div>

                  <div className="flex items-center justify-between">
                    <div className="space-y-0.5">
                      <Label>自動回転</Label>
                      <p className="text-sm text-gray-500 dark:text-gray-400">画像を自動で回転</p>
                    </div>
                    <Switch
                      checked={getBooleanValue('AUTO_ROTATE')}
                      onCheckedChange={(checked) => handleSettingChange('AUTO_ROTATE', checked)}
                    />
                  </div>

                  <div className="flex items-center justify-between">
                    <div className="space-y-0.5">
                      <Label>印刷回転</Label>
                      <p className="text-sm text-gray-500 dark:text-gray-400">出力を180度回転</p>
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
                  <p className="text-sm text-gray-500 dark:text-gray-400">0-255の値で黒色の判定しきい値を設定</p>
                </div>

                <div className="flex items-center justify-between">
                  <div className="space-y-0.5">
                    <Label>キープアライブ</Label>
                    <p className="text-sm text-gray-500 dark:text-gray-400">
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
                    className="max-w-xs"
                  />
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          {/* 音楽タブ */}
          <TabsContent value="music" className="space-y-6">
            {/* 音楽管理 */}
            <Card>
              <CardHeader>
                <CardTitle>音楽管理</CardTitle>
                <CardDescription>
                  配信中の音楽を管理します。URLパラメータ ?playlist=プレイリスト名 で特定のプレイリストを再生できます。
                </CardDescription>
              </CardHeader>
              <CardContent>
                <MusicManagerEmbed />
              </CardContent>
            </Card>
          </TabsContent>

          {/* オーバーレイタブ */}
          <TabsContent value="overlay">
            <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-6">
              {/* 音楽プレイヤー設定 */}
              <Card>
              <CardHeader>
                <CardTitle>音楽プレイヤー</CardTitle>
                <CardDescription>
                  画面左下に表示される音楽プレイヤーの設定
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="flex items-center justify-between">
                  <Label htmlFor="music-enabled" className="flex flex-col">
                    <span>プレイヤーを表示</span>
                    <span className="text-xs text-gray-500 dark:text-gray-400">
                      音楽プレイヤーの表示/非表示を切り替えます
                    </span>
                  </Label>
                  <Switch
                    id="music-enabled"
                    checked={overlaySettings?.music_enabled ?? true}
                    onCheckedChange={(checked) => 
                      updateOverlaySettings({ music_enabled: checked })
                    }
                  />
                </div>
                
                <div className="flex items-center justify-between">
                  <Label htmlFor="music-auto-play" className="flex flex-col">
                    <span>自動再生</span>
                    <span className="text-xs text-gray-500 dark:text-gray-400">
                      ページ読み込み時に自動的に再生を開始します
                    </span>
                  </Label>
                  <Switch
                    id="music-auto-play"
                    checked={overlaySettings?.music_auto_play ?? false}
                    onCheckedChange={(checked) => 
                      updateOverlaySettings({ music_auto_play: checked })
                    }
                  />
                </div>
              </CardContent>
            </Card>

            {/* 音楽プレイヤーコントロール */}
            <Card>
              <CardHeader>
                <CardTitle>再生コントロール</CardTitle>
                <CardDescription>
                  オーバーレイの音楽プレイヤーをリモート操作します
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                  
                  {/* 現在の曲情報 */}
                  {musicStatus.current_track ? (
                    <div className="p-3 bg-gray-50 dark:bg-gray-800 rounded-lg">
                      <div className="flex items-center gap-3">
                        {/* アートワーク */}
                        <div className="w-12 h-12 flex-shrink-0">
                          {musicStatus.current_track.has_artwork ? (
                            <img
                              src={buildApiUrl(`/api/music/track/${musicStatus.current_track.id}/artwork`)}
                              alt={musicStatus.current_track.title}
                              className="w-full h-full object-cover rounded"
                            />
                          ) : (
                            <div className="w-full h-full bg-gray-200 dark:bg-gray-700 rounded flex items-center justify-center">
                              <Music className="w-5 h-5 text-gray-400" />
                            </div>
                          )}
                        </div>

                        {/* 曲情報 */}
                        <div className="flex-1 min-w-0">
                          <p className="text-sm font-medium truncate">{musicStatus.current_track.title}</p>
                          <p className="text-xs text-gray-500 truncate">
                            {musicStatus.current_track.artist} • {formatTime(musicStatus.current_time)} / {formatTime(musicStatus.duration)}
                          </p>
                        </div>
                      </div>
                    </div>
                  ) : (
                    <div className="p-4 text-center text-gray-500 dark:text-gray-400 bg-gray-50 dark:bg-gray-800 rounded-lg">
                      <Music className="w-8 h-8 mx-auto mb-1 opacity-50" />
                      <p className="text-sm">再生中の曲はありません</p>
                    </div>
                  )}

                  {/* コントロールボタン */}
                  <div className="flex items-center justify-center gap-1">
                    <Button
                      onClick={() => sendMusicControlCommand('previous')}
                      size="sm"
                      variant="outline"
                      disabled={!musicStatus.current_track}
                      className="h-9 w-9"
                    >
                      <SkipBack className="w-3.5 h-3.5" />
                    </Button>
                    
                    <Button
                      onClick={() => sendMusicControlCommand(musicStatus.is_playing ? 'pause' : 'play')}
                      size="sm"
                      className="h-9 w-9"
                      disabled={!musicStatus.current_track && !musicStatus.is_playing}
                    >
                      {musicStatus.is_playing ? (
                        <Pause className="w-4 h-4" />
                      ) : (
                        <Play className="w-4 h-4 ml-0.5" />
                      )}
                    </Button>
                    
                    <Button
                      onClick={() => sendMusicControlCommand('next')}
                      size="sm"
                      variant="outline"
                      disabled={!musicStatus.current_track}
                      className="h-9 w-9"
                    >
                      <SkipForward className="w-3.5 h-3.5" />
                    </Button>
                    
                    <Button
                      onClick={() => sendMusicControlCommand('stop')}
                      size="sm"
                      variant="outline"
                      className="ml-2 h-9 w-9"
                      disabled={!musicStatus.current_track}
                    >
                      <X className="w-3.5 h-3.5" />
                    </Button>
                  </div>

                  {/* シークバー */}
                  {musicStatus.current_track && (
                    <div className="space-y-2">
                      <input
                        type="range"
                        min="0"
                        max={musicStatus.duration}
                        value={musicStatus.current_time}
                        onChange={(e) => handleSeek(Number(e.target.value))}
                        className="w-full"
                      />
                      <div className="flex justify-between text-xs text-gray-500">
                        <span>{formatTime(musicStatus.current_time)}</span>
                        <span>{formatTime(musicStatus.duration)}</span>
                      </div>
                    </div>
                  )}

                  {/* リアルタイムボリューム */}
                  <div className="flex items-center gap-3">
                    <Volume2 className="w-4 h-4 text-gray-500" />
                    <input
                      type="range"
                      min="0"
                      max="100"
                      value={musicStatus.volume}
                      onChange={(e) => {
                        const volume = Number(e.target.value);
                        sendMusicControlCommand('volume', { volume });
                        // 設定にも保存
                        updateOverlaySettings({ music_volume: volume });
                      }}
                      className="flex-1"
                    />
                    <span className="text-sm text-gray-500 w-10 text-right">
                      {musicStatus.volume}%
                    </span>
                  </div>

                  {/* プレイリスト選択 */}
                  <div className="space-y-2">
                    <Label htmlFor="playlist-select">プレイリスト</Label>
                    <Select
                      value={musicStatus.playlist_name || 'all'}
                      onValueChange={(value) => sendMusicControlCommand('load', { playlist: value === 'all' ? undefined : value })}
                    >
                      <SelectTrigger id="playlist-select">
                        <SelectValue placeholder="すべての曲" />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="all">すべての曲</SelectItem>
                        {playlists.map(playlist => (
                          <SelectItem key={playlist.id} value={playlist.name}>
                            {playlist.name} ({playlist.track_count}曲)
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
              </CardContent>
            </Card>

            {/* FAX表示設定 */}
            <Card>
              <CardHeader>
                <CardTitle>FAX表示</CardTitle>
                <CardDescription>
                  FAX受信時のアニメーション設定
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="flex items-center justify-between">
                      <Label htmlFor="fax-enabled" className="flex flex-col">
                        <span>FAXアニメーションを表示</span>
                        <span className="text-xs text-gray-500 dark:text-gray-400">
                          FAX受信時にアニメーションを表示します
                        </span>
                      </Label>
                      <Switch
                        id="fax-enabled"
                        checked={overlaySettings?.fax_enabled ?? true}
                        onCheckedChange={(checked) => 
                          updateOverlaySettings({ fax_enabled: checked })
                        }
                      />
                    </div>

                    <div className="space-y-2">
                      <Label htmlFor="fax-speed">
                        アニメーション速度: {((overlaySettings?.fax_animation_speed ?? 1.0) * 100).toFixed(0)}%
                      </Label>
                      <input
                        type="range"
                        id="fax-speed"
                        min="50"
                        max="200"
                        value={(overlaySettings?.fax_animation_speed ?? 1.0) * 100}
                        onChange={(e) => 
                          updateOverlaySettings({ fax_animation_speed: parseInt(e.target.value) / 100 })
                        }
                        className="w-full"
                      />
                    </div>
              </CardContent>
            </Card>

            {/* 時計表示設定 */}
            <Card>
              <CardHeader>
                <CardTitle>時計表示</CardTitle>
                <CardDescription>
                  画面右上に表示される時計と統計情報の設定
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="flex items-center justify-between">
                      <Label htmlFor="clock-enabled" className="flex flex-col">
                        <span>時計エリアを表示</span>
                        <span className="text-xs text-gray-500 dark:text-gray-400">
                          画面右上の時計エリア全体の表示/非表示
                        </span>
                      </Label>
                      <Switch
                        id="clock-enabled"
                        checked={overlaySettings?.clock_enabled ?? true}
                        onCheckedChange={(checked) => 
                          updateOverlaySettings({ clock_enabled: checked })
                        }
                      />
                    </div>

                    {/* 時計エリアが有効な場合の個別設定 */}
                    {(overlaySettings?.clock_enabled ?? true) && (
                      <div className="space-y-4 pl-4 border-l-2 border-gray-200 dark:border-gray-700">
                        <div className="flex items-center justify-between">
                          <Label htmlFor="clock-show-icons" className="flex flex-col">
                            <span>アイコンを表示</span>
                            <span className="text-xs text-gray-500 dark:text-gray-400">
                              各項目の左側にアイコンを表示します
                            </span>
                          </Label>
                          <Switch
                            id="clock-show-icons"
                            checked={overlaySettings?.clock_show_icons ?? true}
                            onCheckedChange={(checked) => 
                              updateOverlaySettings({ clock_show_icons: checked })
                            }
                          />
                        </div>

                        <div className="flex items-center justify-between">
                          <Label htmlFor="location-enabled">場所 (Tokyo, JP)</Label>
                          <Switch
                            id="location-enabled"
                            checked={overlaySettings?.location_enabled ?? true}
                            onCheckedChange={(checked) => 
                              updateOverlaySettings({ location_enabled: checked })
                            }
                          />
                        </div>

                        <div className="flex items-center justify-between">
                          <Label htmlFor="date-enabled">日付</Label>
                          <Switch
                            id="date-enabled"
                            checked={overlaySettings?.date_enabled ?? true}
                            onCheckedChange={(checked) => 
                              updateOverlaySettings({ date_enabled: checked })
                            }
                          />
                        </div>

                        <div className="flex items-center justify-between">
                          <Label htmlFor="time-enabled">時刻</Label>
                          <Switch
                            id="time-enabled"
                            checked={overlaySettings?.time_enabled ?? true}
                            onCheckedChange={(checked) => 
                              updateOverlaySettings({ time_enabled: checked })
                            }
                          />
                        </div>
                      </div>
                    )}

                    <div className="space-y-2">
                      <Label htmlFor="clock-format">時刻表示形式</Label>
                      <Select
                        value={overlaySettings?.clock_format ?? '24h'}
                        onValueChange={(value: '12h' | '24h') => 
                          updateOverlaySettings({ clock_format: value })
                        }
                      >
                        <SelectTrigger id="clock-format">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="24h">24時間表記</SelectItem>
                          <SelectItem value="12h">12時間表記 (AM/PM)</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>

                    <div className="flex items-center justify-between">
                      <Label htmlFor="stats-enabled" className="flex flex-col">
                        <span>統計情報を表示</span>
                        <span className="text-xs text-gray-500 dark:text-gray-400">
                          時計と一緒に統計情報（体重・財布など）を表示します
                        </span>
                      </Label>
                      <Switch
                        id="stats-enabled"
                        checked={overlaySettings?.stats_enabled ?? true}
                        onCheckedChange={(checked) => 
                          updateOverlaySettings({ stats_enabled: checked })
                        }
                      />
                    </div>

                    {/* 統計情報の値設定 */}
                    {(overlaySettings?.stats_enabled ?? true) && (
                      <div className="space-y-4 pl-4 border-l-2 border-gray-200 dark:border-gray-700">
                        <div className="space-y-2">
                          <Label htmlFor="clock-weight">体重 (kg)</Label>
                          <Input
                            id="clock-weight"
                            type="number"
                            step="0.1"
                            min="0.1"
                            max="999.9"
                            value={getSettingValue('CLOCK_WEIGHT') || '75.4'}
                            onChange={(e) => handleSettingChange('CLOCK_WEIGHT', e.target.value)}
                            placeholder="75.4"
                          />
                        </div>

                        <div className="space-y-2">
                          <Label htmlFor="clock-wallet">財布 (円)</Label>
                          <Input
                            id="clock-wallet"
                            type="number"
                            min="0"
                            max="9999999"
                            value={getSettingValue('CLOCK_WALLET') || '10387'}
                            onChange={(e) => handleSettingChange('CLOCK_WALLET', e.target.value)}
                            placeholder="10387"
                          />
                        </div>
                      </div>
                    )}
              </CardContent>
            </Card>

            {/* デバッグ情報 */}
            <Card>
              <CardHeader>
                <CardTitle>デバッグ情報</CardTitle>
                <CardDescription>
                  開発用のデバッグ機能
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                  <div className="flex items-center justify-between">
                    <Label htmlFor="show-debug-info" className="flex flex-col">
                      <span>デバッグ情報を表示</span>
                      <span className="text-xs text-gray-500 dark:text-gray-400">
                        開発用のデバッグ情報を表示します
                      </span>
                    </Label>
                    <Switch
                      id="show-debug-info"
                      checked={overlaySettings?.show_debug_info ?? false}
                      onCheckedChange={(checked) => 
                        updateOverlaySettings({ show_debug_info: checked })
                      }
                    />
                  </div>
              </CardContent>
            </Card>
            </div>
          </TabsContent>

          {/* ログタブ */}
          <TabsContent value="logs" className="space-y-6">
            <LogViewer embedded={true} />
          </TabsContent>

          {/* システムタブ */}
          <TabsContent value="system" className="space-y-6">
            <Card>
              <CardHeader>
                <CardTitle>サーバー管理</CardTitle>
                <CardDescription>
                  サーバーの再起動や状態確認を行います
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-6">
                {/* サーバー状態 */}
                <div className="space-y-4">
                  <h3 className="text-sm font-medium dark:text-gray-200">サーバー状態</h3>
                  <div className="grid grid-cols-2 gap-4 text-sm dark:text-gray-300">
                    <div className="flex items-center space-x-2">
                      <div className="w-2 h-2 bg-green-500 rounded-full animate-pulse"></div>
                      <span>サーバー稼働中</span>
                    </div>
                    {featureStatus && (
                      <div>
                        サービスモード: {featureStatus.service_mode ? 'Yes' : 'No'}
                      </div>
                    )}
                  </div>
                </div>

                {/* 再起動ボタン */}
                <div className="space-y-4">
                  <h3 className="text-sm font-medium dark:text-gray-200">サービス管理</h3>
                  {restarting ? (
                    <div className="space-y-4">
                      <Alert>
                        <AlertDescription className="flex items-center space-x-2">
                          <RefreshCw className="w-4 h-4 animate-spin" />
                          <span>サーバーを再起動しています...</span>
                          {restartCountdown > 0 && (
                            <span className="font-mono">({restartCountdown}秒)</span>
                          )}
                        </AlertDescription>
                      </Alert>
                      <p className="text-sm text-gray-600 dark:text-gray-400">
                        再起動が完了すると自動的にページがリロードされます。
                      </p>
                    </div>
                  ) : (
                    <div className="space-y-6">
                      {/* アプリケーション再起動 */}
                      <div className="border-b dark:border-gray-700 pb-4">
                        <h4 className="text-sm font-medium mb-2 dark:text-gray-200">アプリケーション</h4>
                        <p className="text-sm text-gray-600 dark:text-gray-400 mb-3">
                          twitch-overlayサービスを再起動します。すべての接続が一時的に切断されます。
                        </p>
                        <div className="flex space-x-2">
                          <Button 
                            onClick={() => handleServerRestart(false)}
                            variant="default"
                            className="flex items-center space-x-2"
                          >
                            <RefreshCw className="w-4 h-4" />
                            <span>サーバーを再起動</span>
                          </Button>
                          <Button 
                            onClick={() => handleServerRestart(true)}
                            variant="destructive"
                            className="flex items-center space-x-2"
                          >
                            <RefreshCw className="w-4 h-4" />
                            <span>強制再起動</span>
                          </Button>
                        </div>
                      </div>

                      {/* Bluetoothサービス再起動 */}
                      <div className="pt-2">
                        <h4 className="text-sm font-medium mb-2 dark:text-gray-200">Bluetoothサービス</h4>
                        <p className="text-sm text-gray-600 dark:text-gray-400 mb-3">
                          HCIエラーやBluetoothデバイスがハングした場合に実行してください。
                        </p>
                        <Button 
                          onClick={handleBluetoothRestart}
                          variant="secondary"
                          className="flex items-center space-x-2"
                        >
                          <Bluetooth className="w-4 h-4" />
                          <span>Bluetoothサービス再起動</span>
                        </Button>
                      </div>
                    </div>
                  )}
                </div>

                {/* 再起動に関する注意事項 */}
                <Alert className="dark:bg-gray-700 dark:border-gray-600">
                  <AlertDescription className="dark:text-gray-300">
                    <strong>注意:</strong> systemdサービスとして動作している場合、
                    サーバーは自動的に再起動されます。通常モードで動作している場合は、
                    新しいプロセスが起動されます。
                  </AlertDescription>
                </Alert>
              </CardContent>
            </Card>
          </TabsContent>

          {/* APIタブ */}
          <TabsContent value="api" className="space-y-6">
            <Card>
              <CardHeader>
                <CardTitle>Music API エンドポイント一覧</CardTitle>
                <CardDescription>
                  音楽機能で利用可能なAPIエンドポイントの一覧です
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-6">
                {/* Track管理API */}
                <div className="space-y-4">
                  <h3 className="text-lg font-semibold dark:text-white">Track管理</h3>
                  <div className="space-y-3">
                    <div className="p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
                      <div className="flex items-center space-x-2 mb-2">
                        <span className="px-2 py-1 bg-green-100 dark:bg-green-900 text-green-800 dark:text-green-200 text-xs font-mono rounded">POST</span>
                        <span className="font-mono text-sm dark:text-gray-300">/api/music/upload</span>
                      </div>
                      <p className="text-sm text-gray-600 dark:text-gray-400">ファイルアップロード (MP3/WAV/M4A/OGG)</p>
                    </div>
                    
                    <div className="p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
                      <div className="flex items-center space-x-2 mb-2">
                        <span className="px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-800 dark:text-blue-200 text-xs font-mono rounded">GET</span>
                        <span className="font-mono text-sm dark:text-gray-300">/api/music/tracks</span>
                      </div>
                      <p className="text-sm text-gray-600 dark:text-gray-400">全トラック取得</p>
                    </div>
                    
                    <div className="p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
                      <div className="flex items-center space-x-2 mb-2">
                        <span className="px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-800 dark:text-blue-200 text-xs font-mono rounded">GET</span>
                        <span className="font-mono text-sm dark:text-gray-300">/api/music/track/{`{id}`}</span>
                      </div>
                      <p className="text-sm text-gray-600 dark:text-gray-400">トラック情報取得</p>
                    </div>
                    
                    <div className="p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
                      <div className="flex items-center space-x-2 mb-2">
                        <span className="px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-800 dark:text-blue-200 text-xs font-mono rounded">GET</span>
                        <span className="font-mono text-sm dark:text-gray-300">/api/music/track/{`{id}`}/audio</span>
                      </div>
                      <p className="text-sm text-gray-600 dark:text-gray-400">音声ファイル取得 (ストリーミング対応)</p>
                    </div>
                    
                    <div className="p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
                      <div className="flex items-center space-x-2 mb-2">
                        <span className="px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-800 dark:text-blue-200 text-xs font-mono rounded">GET</span>
                        <span className="font-mono text-sm dark:text-gray-300">/api/music/track/{`{id}`}/artwork</span>
                      </div>
                      <p className="text-sm text-gray-600 dark:text-gray-400">アートワーク画像取得</p>
                    </div>
                    
                    <div className="p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
                      <div className="flex items-center space-x-2 mb-2">
                        <span className="px-2 py-1 bg-red-100 dark:bg-red-900 text-red-800 dark:text-red-200 text-xs font-mono rounded">DELETE</span>
                        <span className="font-mono text-sm dark:text-gray-300">/api/music/track/{`{id}`}</span>
                      </div>
                      <p className="text-sm text-gray-600 dark:text-gray-400">トラック削除</p>
                    </div>
                    
                    <div className="p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
                      <div className="flex items-center space-x-2 mb-2">
                        <span className="px-2 py-1 bg-red-100 dark:bg-red-900 text-red-800 dark:text-red-200 text-xs font-mono rounded">DELETE</span>
                        <span className="font-mono text-sm dark:text-gray-300">/api/music/track/all</span>
                      </div>
                      <p className="text-sm text-gray-600 dark:text-gray-400">全トラック削除</p>
                    </div>
                  </div>
                </div>

                {/* Playlist管理API */}
                <div className="space-y-4">
                  <h3 className="text-lg font-semibold dark:text-white">Playlist管理</h3>
                  <div className="space-y-3">
                    <div className="p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
                      <div className="flex items-center space-x-2 mb-2">
                        <span className="px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-800 dark:text-blue-200 text-xs font-mono rounded">GET</span>
                        <span className="font-mono text-sm dark:text-gray-300">/api/music/playlists</span>
                      </div>
                      <p className="text-sm text-gray-600 dark:text-gray-400">全プレイリスト取得</p>
                    </div>
                    
                    <div className="p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
                      <div className="flex items-center space-x-2 mb-2">
                        <span className="px-2 py-1 bg-green-100 dark:bg-green-900 text-green-800 dark:text-green-200 text-xs font-mono rounded">POST</span>
                        <span className="font-mono text-sm dark:text-gray-300">/api/music/playlist</span>
                      </div>
                      <p className="text-sm text-gray-600 dark:text-gray-400">プレイリスト作成</p>
                    </div>
                    
                    <div className="p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
                      <div className="flex items-center space-x-2 mb-2">
                        <span className="px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-800 dark:text-blue-200 text-xs font-mono rounded">GET</span>
                        <span className="font-mono text-sm dark:text-gray-300">/api/music/playlist/{`{id}`}</span>
                      </div>
                      <p className="text-sm text-gray-600 dark:text-gray-400">プレイリスト取得</p>
                    </div>
                    
                    <div className="p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
                      <div className="flex items-center space-x-2 mb-2">
                        <span className="px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-800 dark:text-blue-200 text-xs font-mono rounded">GET</span>
                        <span className="font-mono text-sm dark:text-gray-300">/api/music/playlist/{`{id}`}/tracks</span>
                      </div>
                      <p className="text-sm text-gray-600 dark:text-gray-400">プレイリストトラック取得</p>
                    </div>
                    
                    <div className="p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
                      <div className="flex items-center space-x-2 mb-2">
                        <span className="px-2 py-1 bg-yellow-100 dark:bg-yellow-900 text-yellow-800 dark:text-yellow-200 text-xs font-mono rounded">PUT</span>
                        <span className="font-mono text-sm dark:text-gray-300">/api/music/playlist/{`{id}`}</span>
                      </div>
                      <p className="text-sm text-gray-600 dark:text-gray-400">プレイリスト更新 (add_track, remove_track, reorder_track)</p>
                    </div>
                    
                    <div className="p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
                      <div className="flex items-center space-x-2 mb-2">
                        <span className="px-2 py-1 bg-red-100 dark:bg-red-900 text-red-800 dark:text-red-200 text-xs font-mono rounded">DELETE</span>
                        <span className="font-mono text-sm dark:text-gray-300">/api/music/playlist/{`{id}`}</span>
                      </div>
                      <p className="text-sm text-gray-600 dark:text-gray-400">プレイリスト削除</p>
                    </div>
                  </div>
                </div>

                {/* リモート制御API */}
                <div className="space-y-4">
                  <h3 className="text-lg font-semibold dark:text-white">リモート制御</h3>
                  <div className="space-y-3">
                    <div className="p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
                      <div className="flex items-center space-x-2 mb-2">
                        <span className="px-2 py-1 bg-green-100 dark:bg-green-900 text-green-800 dark:text-green-200 text-xs font-mono rounded">POST</span>
                        <span className="font-mono text-sm dark:text-gray-300">/api/music/control/play</span>
                      </div>
                      <p className="text-sm text-gray-600 dark:text-gray-400">音楽再生</p>
                    </div>
                    
                    <div className="p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
                      <div className="flex items-center space-x-2 mb-2">
                        <span className="px-2 py-1 bg-green-100 dark:bg-green-900 text-green-800 dark:text-green-200 text-xs font-mono rounded">POST</span>
                        <span className="font-mono text-sm dark:text-gray-300">/api/music/control/pause</span>
                      </div>
                      <p className="text-sm text-gray-600 dark:text-gray-400">音楽一時停止</p>
                    </div>
                    
                    <div className="p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
                      <div className="flex items-center space-x-2 mb-2">
                        <span className="px-2 py-1 bg-green-100 dark:bg-green-900 text-green-800 dark:text-green-200 text-xs font-mono rounded">POST</span>
                        <span className="font-mono text-sm dark:text-gray-300">/api/music/control/next</span>
                      </div>
                      <p className="text-sm text-gray-600 dark:text-gray-400">次の曲</p>
                    </div>
                    
                    <div className="p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
                      <div className="flex items-center space-x-2 mb-2">
                        <span className="px-2 py-1 bg-green-100 dark:bg-green-900 text-green-800 dark:text-green-200 text-xs font-mono rounded">POST</span>
                        <span className="font-mono text-sm dark:text-gray-300">/api/music/control/previous</span>
                      </div>
                      <p className="text-sm text-gray-600 dark:text-gray-400">前の曲</p>
                    </div>
                    
                    <div className="p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
                      <div className="flex items-center space-x-2 mb-2">
                        <span className="px-2 py-1 bg-green-100 dark:bg-green-900 text-green-800 dark:text-green-200 text-xs font-mono rounded">POST</span>
                        <span className="font-mono text-sm dark:text-gray-300">/api/music/control/volume</span>
                      </div>
                      <p className="text-sm text-gray-600 dark:text-gray-400">音量変更 (body: {`{"volume": 0-100}`})</p>
                    </div>
                    
                    <div className="p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
                      <div className="flex items-center space-x-2 mb-2">
                        <span className="px-2 py-1 bg-green-100 dark:bg-green-900 text-green-800 dark:text-green-200 text-xs font-mono rounded">POST</span>
                        <span className="font-mono text-sm dark:text-gray-300">/api/music/control/load</span>
                      </div>
                      <p className="text-sm text-gray-600 dark:text-gray-400">プレイリスト読み込み (body: {`{"playlist": "name"}`})</p>
                    </div>
                    
                    <div className="p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
                      <div className="flex items-center space-x-2 mb-2">
                        <span className="px-2 py-1 bg-purple-100 dark:bg-purple-900 text-purple-800 dark:text-purple-200 text-xs font-mono rounded">SSE</span>
                        <span className="font-mono text-sm dark:text-gray-300">/api/music/control/events</span>
                      </div>
                      <p className="text-sm text-gray-600 dark:text-gray-400">コマンド受信 (Server-Sent Events)</p>
                    </div>
                    
                    <div className="p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
                      <div className="flex items-center space-x-2 mb-2">
                        <span className="px-2 py-1 bg-green-100 dark:bg-green-900 text-green-800 dark:text-green-200 text-xs font-mono rounded">POST</span>
                        <span className="font-mono text-sm dark:text-gray-300">/api/music/status/update</span>
                      </div>
                      <p className="text-sm text-gray-600 dark:text-gray-400">状態更新 (オーバーレイ→サーバー)</p>
                    </div>
                    
                    <div className="p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
                      <div className="flex items-center space-x-2 mb-2">
                        <span className="px-2 py-1 bg-purple-100 dark:bg-purple-900 text-purple-800 dark:text-purple-200 text-xs font-mono rounded">SSE</span>
                        <span className="font-mono text-sm dark:text-gray-300">/api/music/status/events</span>
                      </div>
                      <p className="text-sm text-gray-600 dark:text-gray-400">状態受信 (Server-Sent Events)</p>
                    </div>
                  </div>
                </div>

                <Alert className="dark:bg-gray-700 dark:border-gray-600">
                  <AlertDescription className="dark:text-gray-300">
                    <strong>ℹ️ 使用方法:</strong>
                    <br />• Track/Playlist管理: Settings画面の音楽タブから操作
                    <br />• リモート制御: Settings画面からオーバーレイの音楽プレイヤーを制御
                    <br />• URLパラメータ: ?playlist=name でプレイリスト指定可能
                  </AlertDescription>
                </Alert>
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>
      </div>
    </div>
  );
};