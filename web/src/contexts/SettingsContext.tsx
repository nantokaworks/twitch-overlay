import React, { createContext, useContext, useEffect, useState, useCallback } from 'react';
import { buildApiUrl } from '../utils/api';

interface OverlaySettings {
  // 音楽プレイヤー設定
  music_playlist: string | null;
  music_volume: number;
  
  // FAX表示設定
  fax_enabled: boolean;
  fax_animation_speed: number;
  fax_image_type: 'mono' | 'color';
  
  // 時計表示設定
  clock_enabled: boolean;
  clock_format: '12h' | '24h';
  clock_show_icons: boolean;
  location_enabled: boolean;
  date_enabled: boolean;
  time_enabled: boolean;
  stats_enabled: boolean;
  
  updated_at: string;
}

interface SettingsContextType {
  settings: OverlaySettings | null;
  updateSettings: (updates: Partial<OverlaySettings>) => Promise<void>;
  isLoading: boolean;
  error: string | null;
}

const SettingsContext = createContext<SettingsContextType | undefined>(undefined);

export const useSettings = () => {
  const context = useContext(SettingsContext);
  if (!context) {
    throw new Error('useSettings must be used within SettingsProvider');
  }
  return context;
};

export const SettingsProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [settings, setSettings] = useState<OverlaySettings | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // 初期設定を取得
  useEffect(() => {
    const fetchSettings = async () => {
      try {
        const response = await fetch(buildApiUrl('/api/settings/overlay'));
        if (response.ok) {
          const data = await response.json();
          setSettings(data);
        } else {
          setError('Failed to load settings');
        }
      } catch (err) {
        setError('Failed to connect to server');
        console.error('Failed to fetch settings:', err);
      } finally {
        setIsLoading(false);
      }
    };

    fetchSettings();
  }, []);

  // SSEで設定変更を監視
  useEffect(() => {
    const eventSource = new EventSource(buildApiUrl('/api/settings/overlay/events'));

    eventSource.onmessage = (event) => {
      try {
        const updatedSettings = JSON.parse(event.data);
        console.log('📡 Settings updated via SSE:', updatedSettings);
        setSettings(updatedSettings);
      } catch (err) {
        console.error('Failed to parse SSE data:', err);
      }
    };

    eventSource.onerror = (err) => {
      console.error('SSE connection error:', err);
      // 再接続は自動的に行われる
    };

    return () => {
      eventSource.close();
    };
  }, []);

  // 設定を更新
  const updateSettings = useCallback(async (updates: Partial<OverlaySettings>) => {
    if (!settings) return;

    const newSettings = { ...settings, ...updates };
    
    try {
      const response = await fetch(buildApiUrl('/api/settings/overlay'), {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(newSettings),
      });

      if (response.ok) {
        // サーバーが成功したら、SSE経由で更新が来るのを待つ
        // 楽観的更新を行う
        setSettings(newSettings);
      } else {
        throw new Error('Failed to update settings');
      }
    } catch (err) {
      console.error('Failed to update settings:', err);
      setError('Failed to update settings');
      throw err;
    }
  }, [settings]);

  return (
    <SettingsContext.Provider value={{ settings, updateSettings, isLoading, error }}>
      {children}
    </SettingsContext.Provider>
  );
};