import { useEffect, useState } from 'react';
import { useFaxQueue } from '../hooks/useFaxQueue';
import FaxDisplay from './FaxDisplay';
import { LAYOUT } from '../constants/layout';

const FaxReceiver = ({ imageType = 'mono' }) => {
  const [isConnected, setIsConnected] = useState(false);
  const [labelPosition, setLabelPosition] = useState(0);
  const [isAnimating, setIsAnimating] = useState(false);
  const [faxState, setFaxState] = useState(null);
  const [isShaking, setIsShaking] = useState(false);
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

  useEffect(() => {
    let reconnectTimeout;
    let eventSource;

    const connect = () => {
      eventSource = new EventSource('/events');

      eventSource.onopen = () => {
        setIsConnected(true);
        console.log('SSE connection opened');
        if (reconnectTimeout) {
          clearTimeout(reconnectTimeout);
          reconnectTimeout = null;
        }
      };

      eventSource.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          if (data.type === 'fax') {
            addToQueue(data);
          }
        } catch (error) {
          console.error('Failed to parse SSE message:', error);
        }
      };

      eventSource.onerror = (error) => {
        console.error('SSE connection error:', error);
        setIsConnected(false);
        eventSource.close();
        
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

  return (
    <div className="h-screen text-white relative overflow-hidden" style={{ backgroundColor: isDebug ? '#374151' : 'transparent' }}>
      {/* コントロールパネル */}
      <div 
        className="fixed z-10" 
        style={{ 
          left: `${LAYOUT.LEFT_MARGIN}px`, 
          width: `${LAYOUT.FAX_WIDTH}px`, 
          height: `${LAYOUT.LABEL_HEIGHT}px`,
          top: `${labelPosition}px`,
          transition: isAnimating ? 'none' : `top ${LAYOUT.FADE_DURATION}ms ease-out`
        }}
      >
        <div className="flex items-center h-full px-2">
          <span
            className={`text-outline ${
              isConnected ? 'text-green-500' : 'text-red-500'
            }`}
            style={{
              fontSize: `${LAYOUT.FONT_SIZE}px`,
              marginRight: `${LAYOUT.LED_RIGHT_MARGIN}px`
            }}
          >
            ◆
          </span>
          <span 
            className="text-outline" 
            style={{ 
              fontSize: `${LAYOUT.FONT_SIZE}px`,
              animation: isShaking ? `shake ${LAYOUT.SHAKE_DURATION} infinite` : 'none'
            }}
          >
            FAX
          </span>
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

    </div>
  );
};

export default FaxReceiver;