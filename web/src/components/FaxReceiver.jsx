import { useEffect, useState } from 'react';
import { useFaxQueue } from '../hooks/useFaxQueue';
import FaxDisplay from './FaxDisplay';

const FaxReceiver = ({ imageType = 'mono' }) => {
  const [isConnected, setIsConnected] = useState(false);
  const { currentFax, addToQueue, onDisplayComplete } = useFaxQueue();
  
  // デバッグモードの判定（URLパラメータまたは環境変数）
  const isDebug = new URLSearchParams(window.location.search).get('debug') === 'true';

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
      <div className="fixed top-0 z-10" style={{ left: '20px', width: '250px', height: '40px' }}>
        <div className="flex items-center h-full px-2">
          <div
            className={`led-dot ${
              isConnected ? 'bg-green-500' : 'bg-red-500'
            }`}
            style={{
              width: '4px',
              height: '20px',
              marginTop: '4px',
              marginRight: '12px'
            }}
          />
          <span className="text-outline" style={{ fontSize: '24px' }}>FAX</span>
        </div>
      </div>

      {/* FAX表示エリア */}
      {currentFax && (
        <FaxDisplay
          faxData={currentFax}
          onComplete={onDisplayComplete}
          imageType={imageType}
        />
      )}

    </div>
  );
};

export default FaxReceiver;