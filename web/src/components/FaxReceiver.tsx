import { useEffect, useState } from 'react';
import { useFaxQueue } from '../hooks/useFaxQueue';
import FaxDisplay from './FaxDisplay';
import DebugPanel from './DebugPanel';
import ClockDisplay from './ClockDisplay';
import MusicPlayer from './music/MusicPlayer';
import { LAYOUT } from '../constants/layout';
import { buildApiUrl, buildEventSourceUrl } from '../utils/api';
import { useSettings } from '../contexts/SettingsContext';
import type { FaxData, FaxState, ServerStatus, DynamicStyles } from '../types';

const FaxReceiver = () => {
  const [isConnected, setIsConnected] = useState<boolean>(false);
  const [isPrinterConnected, setIsPrinterConnected] = useState<boolean>(false);
  const [labelPosition, setLabelPosition] = useState<number>(0);
  const [isAnimating, setIsAnimating] = useState<boolean>(false);
  const [faxState, setFaxState] = useState<FaxState | null>(null);
  const [isShaking, setIsShaking] = useState<boolean>(false);
  const { currentFax, addToQueue, onDisplayComplete } = useFaxQueue();
  
  // ラベル位置をリセット
  useEffect(() => {
    if (!currentFax) {
      setLabelPosition(0);
      setFaxState(null);
    }
  }, [currentFax]);
  
  // Settings from context
  const { settings } = useSettings();
  
  // URLパラメータからデバッグモードだけ取得
  const params = new URLSearchParams(window.location.search);
  const isDebug = params.get('debug') === 'true';
  
  // 設定から表示状態を取得（設定がない場合はデフォルト値）
  const showFax = settings?.fax_enabled ?? true;
  const showClock = settings?.clock_enabled ?? true;
  const playlistName = settings?.music_playlist || undefined;
  
  // 時計表示用
  const showLocation = settings?.location_enabled ?? true;
  const showDate = settings?.date_enabled ?? true;
  const showTime = settings?.time_enabled ?? true;
  const showStats = settings?.stats_enabled ?? true;
  
  // デバッグ情報をコンソールに出力
  useEffect(() => {
    if (isDebug && faxState) {
      console.log('FAX State:', faxState.state, 'Progress:', faxState.progress + '%');
    }
  }, [faxState, isDebug]);
  
  // 震え制御
  useEffect(() => {
    if (faxState) {
      setIsShaking(faxState.state === 'waiting' || faxState.state === 'scrolling');
    } else {
      setIsShaking(false);
    }
  }, [faxState]);

  // プリンター状態のポーリング
  useEffect(() => {
    const checkPrinterStatus = async () => {
      try {
        const response = await fetch(buildApiUrl('/status'));
        if (response.ok) {
          const data: ServerStatus = await response.json();
          setIsPrinterConnected(data.printerConnected);
        }
      } catch (error) {
        console.error('Failed to check printer status:', error);
      }
    };

    // 初回チェック
    checkPrinterStatus();

    // 5秒ごとにチェック
    const interval = setInterval(checkPrinterStatus, 5000);

    return () => clearInterval(interval);
  }, []);

  useEffect(() => {
    let reconnectTimeout: NodeJS.Timeout | null = null;
    let eventSource: EventSource | null = null;

    const connect = () => {
      eventSource = new EventSource(buildEventSourceUrl('/events'));

      eventSource.onopen = () => {
        setIsConnected(true);
        console.log('SSE connection opened');
        if (reconnectTimeout) {
          clearTimeout(reconnectTimeout);
          reconnectTimeout = null;
        }
      };

      eventSource.onmessage = (event: MessageEvent) => {
        try {
          const data: FaxData = JSON.parse(event.data);
          if (data.type === 'fax') {
            addToQueue(data);
          }
        } catch (error) {
          console.error('Failed to parse SSE message:', error);
        }
      };

      eventSource.onerror = (error: Event) => {
        console.error('SSE connection error:', error);
        setIsConnected(false);
        eventSource?.close();
        
        // 再接続を試みる
        reconnectTimeout = setTimeout(() => {
          console.log('Attempting to reconnect...');
          connect();
        }, 3000);
      };
    };

    connect();

    return () => {
      if (reconnectTimeout) {
        clearTimeout(reconnectTimeout);
      }
      if (eventSource) {
        eventSource.close();
      }
    };
  }, [addToQueue]);

  // 背景スタイル
  const backgroundStyle: DynamicStyles = { 
    backgroundColor: isDebug ? '#374151' : 'transparent' 
  };

  // ラベルのスタイル
  const labelStyle: DynamicStyles = { 
    left: `${LAYOUT.LABEL_LEFT_MARGIN}px`, 
    width: `${LAYOUT.FAX_WIDTH}px`, 
    height: `${LAYOUT.LABEL_HEIGHT}px`,
    top: `${labelPosition}px`,
    transition: 'none' // 常にJavaScriptアニメーションを使用
  };

  // LED のスタイル
  const ledStyle: DynamicStyles = {
    fontSize: `${LAYOUT.FONT_SIZE}px`,
    marginRight: `${LAYOUT.LED_RIGHT_MARGIN}px`
  };

  // FAXテキストのスタイル
  const faxTextStyle: DynamicStyles = { 
    fontSize: `${LAYOUT.FONT_SIZE}px`,
    animation: isShaking ? `shake ${LAYOUT.SHAKE_DURATION} infinite` : 'none'
  };

  return (
    <div className="h-screen text-white relative overflow-hidden" style={backgroundStyle}>
      {/* コントロールパネル */}
      {showFax && (
        <div 
          className="fixed z-10" 
          style={labelStyle}
        >
          <div className="flex items-center h-full px-2">
            <span
              className={`text-outline ${
                !isConnected ? 'text-red-500' : 
                !isPrinterConnected ? 'text-yellow-500' : 
                'text-green-500'
              }`}
              style={ledStyle}
            >
              ◆
            </span>
            <span 
              className="text-outline" 
              style={faxTextStyle}
            >
              FAX
            </span>
          </div>
        </div>
      )}

      {/* Clock Display */}
      {(showLocation || showDate || showTime || showStats) && (
        <div className="fixed top-0 right-0 z-10">
          <ClockDisplay 
            showLocation={showLocation}
            showDate={showDate}
            showTime={showTime}
            showStats={showStats}
          />
        </div>
      )}

      {/* FAX表示エリア */}
      {showFax && currentFax && (
        <FaxDisplay
          faxData={currentFax}
          onComplete={onDisplayComplete}
          imageType={settings?.fax_image_type ?? 'mono'}
          onLabelPositionUpdate={setLabelPosition}
          onAnimationStateChange={setIsAnimating}
          onStateChange={setFaxState}
        />
      )}

      {/* デバッグパネル（デバッグモード時のみ表示） */}
      {isDebug && (
        <DebugPanel onSendFax={addToQueue} />
      )}

      {/* 音楽プレイヤー */}
      <MusicPlayer 
        playlist={playlistName || undefined}
      />
    </div>
  );
};

export default FaxReceiver;