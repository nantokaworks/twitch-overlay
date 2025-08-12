import React, { useState, useEffect, useRef } from 'react';
import { buildApiUrl } from '../utils/api';
import { 
  Setting, 
  FeatureStatus, 
  BluetoothDevice, 
  ScanResponse, 
  TestResponse, 
  SettingsResponse,
  UpdateSettingsRequest,
  UpdateSettingsResponse
} from '../types';

interface FontInfo {
  hasCustomFont: boolean;
  filename?: string;
  fileSize?: number;
  modifiedAt?: string;
}

interface AuthInfo {
  authUrl: string;
  authenticated: boolean;
  expiresAt?: number | null;
  error?: string | null;
}

interface SettingsProps {
  onClose?: () => void;
}

export const Settings: React.FC<SettingsProps> = ({ onClose }) => {
  const [fontInfo, setFontInfo] = useState<FontInfo>({ hasCustomFont: false });
  const [authInfo, setAuthInfo] = useState<AuthInfo | null>(null);
  const [uploading, setUploading] = useState(false);
  const [previewText, setPreviewText] = useState('サンプルテキスト Sample Text 123');
  const [previewImage, setPreviewImage] = useState<string>('');
  const [dragActive, setDragActive] = useState(false);
  const [error, setError] = useState<string>('');
  const [success, setSuccess] = useState<string>('');
  const fileInputRef = useRef<HTMLInputElement>(null);

  // 新しい設定管理関連の状態
  const [settings, setSettings] = useState<Record<string, Setting>>({});
  const [featureStatus, setFeatureStatus] = useState<FeatureStatus | null>(null);
  const [activeTab, setActiveTab] = useState<'general' | 'twitch' | 'printer' | 'behavior' | 'font'>('general');
  const [bluetoothDevices, setBluetoothDevices] = useState<BluetoothDevice[]>([]);
  const [scanning, setScanning] = useState(false);
  const [testing, setTesting] = useState(false);
  const [selectedDevice, setSelectedDevice] = useState('');
  const [unsavedChanges, setUnsavedChanges] = useState<UpdateSettingsRequest>({});

  // 設定情報を取得
  useEffect(() => {
    const initialize = async () => {
      await fetchAllSettings();
      await fetchAuthStatus();
      // 初期表示時のプレビュー生成エラーは無視（後でリトライ）
      try {
        await generatePreview(undefined, false);
      } catch (err) {
        console.log('Initial preview generation failed, will retry later');
      }
    };
    initialize();
  }, []);

  // 全ての設定を取得
  const fetchAllSettings = async () => {
    try {
      const response = await fetch(buildApiUrl('/api/settings/v2'));
      if (!response.ok) throw new Error('Failed to fetch settings');
      const data: SettingsResponse = await response.json();
      
      setSettings(data.settings);
      setFeatureStatus(data.status);
      setFontInfo(data.font || { hasCustomFont: false });
      
      // プリンターアドレスを選択状態に設定
      if (data.settings.PRINTER_ADDRESS?.value) {
        setSelectedDevice(data.settings.PRINTER_ADDRESS.value);
      }
    } catch (err) {
      console.error('Failed to fetch settings:', err);
      setError('設定の取得に失敗しました');
    }
  };

  const fetchSettings = async () => {
    try {
      const response = await fetch(buildApiUrl('/api/settings'));
      if (!response.ok) throw new Error('Failed to fetch settings');
      const data = await response.json();
      setFontInfo(data.font || { hasCustomFont: false });
    } catch (err) {
      console.error('Failed to fetch settings:', err);
      setError('設定の取得に失敗しました');
    }
  };

  const fetchAuthStatus = async () => {
    try {
      const response = await fetch(buildApiUrl('/api/settings/auth/status'));
      if (!response.ok) throw new Error('Failed to fetch auth status');
      const data = await response.json();
      setAuthInfo(data);
    } catch (err) {
      console.error('Failed to fetch auth status:', err);
    }
  };

  const generatePreview = async (text?: string, showError: boolean = true) => {
    try {
      const response = await fetch(buildApiUrl('/api/settings/font/preview'), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ text: text || previewText }),
      });
      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`Failed to generate preview: ${errorText}`);
      }
      const data = await response.json();
      setPreviewImage(data.image);
      // 成功したらエラーをクリア
      if (error.includes('プレビュー生成エラー')) {
        setError('');
      }
    } catch (err) {
      console.error('Failed to generate preview:', err);
      // プレビュー生成に失敗した場合、showErrorがtrueの場合のみユーザーに通知
      if (showError && err instanceof Error) {
        setError(`プレビュー生成エラー: ${err.message}`);
      }
      throw err; // エラーを再スロー
    }
  };

  const handleFileUpload = async (file: File) => {
    // ファイルタイプチェック
    const ext = file.name.toLowerCase().split('.').pop();
    if (ext !== 'ttf' && ext !== 'otf') {
      setError('TTFまたはOTFファイルのみアップロード可能です');
      return;
    }

    // ファイルサイズチェック（50MB）
    if (file.size > 50 * 1024 * 1024) {
      setError('ファイルサイズは50MB以下にしてください');
      return;
    }

    setUploading(true);
    setError('');
    setSuccess('');

    const formData = new FormData();
    formData.append('font', file);

    try {
      const response = await fetch(buildApiUrl('/api/settings/font'), {
        method: 'POST',
        body: formData,
      });

      if (!response.ok) {
        const text = await response.text();
        throw new Error(text || 'Upload failed');
      }

      const data = await response.json();
      setFontInfo(data.font);
      setSuccess('フォントのアップロードに成功しました');
      
      // プレビューを再生成
      await generatePreview();
    } catch (err: any) {
      console.error('Upload failed:', err);
      setError(err.message || 'アップロードに失敗しました');
    } finally {
      setUploading(false);
    }
  };

  const handleDeleteFont = async () => {
    if (!confirm('カスタムフォントを削除してデフォルトに戻しますか？')) {
      return;
    }

    try {
      const response = await fetch(buildApiUrl('/api/settings/font'), {
        method: 'DELETE',
      });

      if (!response.ok) throw new Error('Delete failed');
      
      setFontInfo({ hasCustomFont: false });
      setSuccess('カスタムフォントを削除しました');
      
      // プレビューを再生成
      await generatePreview();
    } catch (err) {
      console.error('Delete failed:', err);
      setError('削除に失敗しました');
    }
  };

  // ドラッグ＆ドロップハンドラー
  const handleDrag = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (e.type === 'dragenter' || e.type === 'dragover') {
      setDragActive(true);
    } else if (e.type === 'dragleave') {
      setDragActive(false);
    }
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setDragActive(false);

    if (e.dataTransfer.files && e.dataTransfer.files[0]) {
      handleFileUpload(e.dataTransfer.files[0]);
    }
  };

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files[0]) {
      handleFileUpload(e.target.files[0]);
    }
  };

  const formatFileSize = (bytes: number) => {
    if (bytes < 1024) return bytes + ' B';
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
    return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
  };

  // プリンタースキャン機能
  const handleScan = async () => {
    setScanning(true);
    setError('');
    try {
      const response = await fetch(buildApiUrl('/api/printer/scan'), { 
        method: 'POST' 
      });
      if (!response.ok) throw new Error('Scan request failed');
      
      const data: ScanResponse = await response.json();
      if (data.status === 'success') {
        setBluetoothDevices(data.devices);
        setSuccess(`${data.devices.length}台のデバイスが見つかりました`);
      } else {
        throw new Error(data.message || 'Scan failed');
      }
    } catch (err: any) {
      console.error('Scan failed:', err);
      setError('デバイススキャンに失敗しました: ' + err.message);
    } finally {
      setScanning(false);
    }
  };

  // プリンター接続テスト
  const handleTest = async (macAddress: string) => {
    setTesting(true);
    setError('');
    try {
      const response = await fetch(buildApiUrl('/api/printer/test'), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ mac_address: macAddress }),
      });
      
      const data: TestResponse = await response.json();
      if (data.success) {
        setSuccess('プリンターとの接続に成功しました');
      } else {
        setError('プリンター接続テスト失敗: ' + data.message);
      }
    } catch (err: any) {
      console.error('Test failed:', err);
      setError('接続テストでエラーが発生しました: ' + err.message);
    } finally {
      setTesting(false);
    }
  };

  // 設定値の変更を一時保存
  const handleSettingChange = (key: string, value: string) => {
    setUnsavedChanges(prev => ({
      ...prev,
      [key]: value
    }));
  };

  // 設定を保存
  const handleSaveSettings = async () => {
    if (Object.keys(unsavedChanges).length === 0) {
      setError('変更された設定がありません');
      return;
    }

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
      if (data.success) {
        setSuccess(data.message);
        setFeatureStatus(data.status);
        setUnsavedChanges({});
        // 設定を再取得して最新状態に同期
        await fetchAllSettings();
      } else {
        throw new Error('設定の保存に失敗しました');
      }
    } catch (err: any) {
      console.error('Save failed:', err);
      setError('設定の保存に失敗しました: ' + err.message);
    }
  };

  // 設定値を取得（表示用）
  const getSettingValue = (key: string): string => {
    // 未保存の変更があればそれを優先
    if (key in unsavedChanges) {
      return unsavedChanges[key];
    }
    // 設定値が存在すればそれを返す
    return settings[key]?.value || '';
  };

  // 設定値のタイプ変換（boolean, number用）
  const getBooleanSetting = (key: string): boolean => {
    return getSettingValue(key) === 'true';
  };

  const getNumberSetting = (key: string): number => {
    return parseInt(getSettingValue(key)) || 0;
  };

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4 z-50" style={{ fontFamily: 'system-ui, -apple-system, sans-serif' }}>
      <div className="bg-white rounded-lg max-w-4xl w-full max-h-[90vh] overflow-y-auto" style={{ fontFamily: 'system-ui, -apple-system, sans-serif' }}>
        <div className="p-6">
          <div className="flex justify-between items-center mb-6">
            <h2 className="text-2xl font-bold">設定</h2>
            {onClose && (
              <button
                onClick={onClose}
                className="text-gray-500 hover:text-gray-700"
              >
                <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            )}
          </div>

          {/* エラー/成功メッセージ */}
          {error && (
            <div className="mb-4 p-3 bg-red-100 border border-red-400 text-red-700 rounded">
              {error}
            </div>
          )}
          {success && (
            <div className="mb-4 p-3 bg-green-100 border border-green-400 text-green-700 rounded">
              {success}
            </div>
          )}

          {/* フォント設定セクション */}
          <div className="mb-8">
            <h3 className="text-lg font-semibold mb-4">フォント設定</h3>
            
            {/* 現在のフォント情報 */}
            <div className="mb-4 p-4 bg-gray-50 rounded">
              <div className="text-sm text-gray-600">現在のフォント:</div>
              <div className="font-medium">
                {fontInfo.hasCustomFont ? (
                  <>
                    {fontInfo.filename} ({formatFileSize(fontInfo.fileSize || 0)})
                    <div className="text-xs text-gray-500 mt-1">
                      更新日時: {fontInfo.modifiedAt}
                    </div>
                  </>
                ) : (
                  'デフォルトフォント (システムフォント)'
                )}
              </div>
            </div>

            {/* アップロードエリア */}
            <div
              className={`border-2 border-dashed rounded-lg p-8 text-center transition-colors ${
                dragActive ? 'border-blue-500 bg-blue-50' : 'border-gray-300'
              } ${uploading ? 'opacity-50 pointer-events-none' : ''}`}
              onDragEnter={handleDrag}
              onDragLeave={handleDrag}
              onDragOver={handleDrag}
              onDrop={handleDrop}
            >
              <input
                ref={fileInputRef}
                type="file"
                accept=".ttf,.otf"
                onChange={handleFileSelect}
                className="hidden"
              />
              
              <svg className="mx-auto h-12 w-12 text-gray-400" stroke="currentColor" fill="none" viewBox="0 0 48 48">
                <path d="M28 8H12a4 4 0 00-4 4v20m32-12v8m0 0v8a4 4 0 01-4 4H12a4 4 0 01-4-4v-4m32-4l-3.172-3.172a4 4 0 00-5.656 0L28 28M8 32l9.172-9.172a4 4 0 015.656 0L28 28m0 0l4 4m4-24h8m-4-4v8m-12 4h.02" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
              
              <p className="mt-2 text-sm text-gray-600">
                {uploading ? (
                  'アップロード中...'
                ) : (
                  <>
                    <button
                      onClick={() => fileInputRef.current?.click()}
                      className="font-medium text-blue-600 hover:text-blue-500"
                    >
                      ファイルを選択
                    </button>
                    またはドラッグ＆ドロップ
                  </>
                )}
              </p>
              <p className="text-xs text-gray-500 mt-1">
                TTF, OTF (最大50MB)
              </p>
            </div>

            {/* カスタムフォント削除ボタン */}
            {fontInfo.hasCustomFont && (
              <button
                onClick={handleDeleteFont}
                className="mt-4 px-4 py-2 bg-red-600 text-white rounded hover:bg-red-700 transition-colors"
              >
                カスタムフォントを削除
              </button>
            )}
          </div>

          {/* プレビューセクション */}
          <div className="mb-8">
            <h3 className="text-lg font-semibold mb-4">フォントプレビュー</h3>
            
            <div className="mb-4">
              <input
                type="text"
                value={previewText}
                onChange={(e) => setPreviewText(e.target.value)}
                onBlur={() => generatePreview()}
                className="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-blue-500"
                placeholder="プレビューテキストを入力"
              />
            </div>

            {previewImage ? (
              <div className="border rounded p-4 bg-gray-50">
                <img
                  src={previewImage}
                  alt="Font preview"
                  className="max-w-full h-auto"
                  style={{ imageRendering: 'pixelated' }}
                />
              </div>
            ) : (
              <div className="border rounded p-4 bg-gray-50 text-gray-500 text-center">
                <div>プレビューを生成できません</div>
                <button
                  onClick={() => generatePreview()}
                  className="mt-2 text-blue-600 hover:text-blue-800 underline"
                >
                  再試行
                </button>
              </div>
            )}
          </div>

          {/* Twitch認証セクション */}
          <div className="mb-8">
            <h3 className="text-lg font-semibold mb-4">Twitch認証</h3>
            
            {authInfo && (
              <div className="border rounded p-4 bg-gray-50">
                {authInfo.authenticated ? (
                  <div>
                    <div className="flex items-center mb-2">
                      <div className="w-3 h-3 bg-green-500 rounded-full mr-2"></div>
                      <span className="text-green-700 font-medium">認証済み</span>
                    </div>
                    {authInfo.expiresAt && (
                      <p className="text-sm text-gray-600">
                        有効期限: {new Date(authInfo.expiresAt * 1000).toLocaleString('ja-JP')}
                      </p>
                    )}
                  </div>
                ) : (
                  <div>
                    <div className="flex items-center mb-3">
                      <div className="w-3 h-3 bg-red-500 rounded-full mr-2"></div>
                      <span className="text-red-700 font-medium">
                        {authInfo.error === 'No token found' ? '未認証' : 'トークン期限切れ'}
                      </span>
                    </div>
                    <p className="text-sm text-gray-600 mb-3">
                      Twitchアカウントを連携して、FAX機能を使用できるようにしてください。
                    </p>
                    <a
                      href={authInfo.authUrl}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="inline-flex items-center px-4 py-2 bg-purple-600 text-white rounded hover:bg-purple-700 transition-colors"
                    >
                      <svg className="w-5 h-5 mr-2" fill="currentColor" viewBox="0 0 24 24">
                        <path d="M11.571 4.714h1.715v5.143H11.57zm4.715 0H18v5.143h-1.714zM6 0L1.714 4.286v15.428h5.143V24l4.286-4.286h3.428L22.286 12V0zm14.571 11.143l-3.428 3.428h-3.429l-3 3v-3H6.857V1.714h13.714Z"/>
                      </svg>
                      Twitchでログイン
                    </a>
                  </div>
                )}
              </div>
            )}
            
            {authInfo && authInfo.authenticated && (
              <button
                onClick={fetchAuthStatus}
                className="mt-3 text-sm text-blue-600 hover:text-blue-800 underline"
              >
                認証状態を更新
              </button>
            )}
          </div>

          {/* 閉じるボタン */}
          {onClose && (
            <div className="flex justify-end">
              <button
                onClick={onClose}
                className="px-6 py-2 bg-gray-600 text-white rounded hover:bg-gray-700 transition-colors"
              >
                閉じる
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};