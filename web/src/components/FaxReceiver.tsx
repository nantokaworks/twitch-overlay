import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useFaxQueue } from '../hooks/useFaxQueue';
import FaxDisplay from './FaxDisplay';
import DebugPanel from './DebugPanel';
import { LAYOUT } from '../constants/layout';
import { buildApiUrl, buildEventSourceUrl } from '../utils/api';
import type { FaxReceiverProps, FaxData, FaxState, ServerStatus, DynamicStyles } from '../types';

const FaxReceiver = ({ imageType = 'mono' }: FaxReceiverProps) => {
  const navigate = useNavigate();
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
  
  // デバッグモードの判定（URLパラメータまたは環境変数）
  const isDebug = new URLSearchParams(window.location.search).get('debug') === 'true';
  
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
          {/* 設定ボタン（デバッグモード時のみ表示） */}
          {isDebug && (
            <button
              onClick={() => navigate('/settings')}
              className="ml-4 p-1 hover:bg-white hover:bg-opacity-20 rounded transition-colors"
              title="設定"
            >
              <svg className="w-6 h-6" fill="currentColor" viewBox="0 0 20 20">
                <path fillRule="evenodd" d="M11.49 3.17c-.38-1.56-2.6-1.56-2.98 0a1.532 1.532 0 01-2.286.948c-1.372-.836-2.942.734-2.106 2.106.54.886.061 2.042-.947 2.287-1.561.379-1.561 2.6 0 2.978a1.532 1.532 0 01.947 2.287c-.836 1.372.734 2.942 2.106 2.106a1.532 1.532 0 012.287.947c.379 1.561 2.6 1.561 2.978 0a1.533 1.533 0 012.287-.947c1.372.836 2.942-.734 2.106-2.106a1.533 1.533 0 01.947-2.287c1.561-.379 1.561-2.6 0-2.978a1.532 1.532 0 01-.947-2.287c.836-1.372-.734-2.942-2.106-2.106a1.532 1.532 0 01-2.287-.947zM10 13a3 3 0 100-6 3 3 0 000 6z" clipRule="evenodd" />
              </svg>
            </button>
          )}
        </div>
      </div>

      {/* FAX表示エリア */}
      {currentFax && (
        <FaxDisplay
          faxData={currentFax}
          onComplete={onDisplayComplete}
          imageType={imageType}
          onLabelPositionUpdate={setLabelPosition}
          onAnimationStateChange={setIsAnimating}
          onStateChange={setFaxState}
        />
      )}

      {/* デバッグパネル（デバッグモード時のみ表示） */}
      {isDebug && (
        <DebugPanel onSendFax={addToQueue} />
      )}

    </div>
  );
};

export default FaxReceiver;