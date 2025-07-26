import { useEffect, useState } from 'react';
import { useFaxQueue } from '../hooks/useFaxQueue';
import FaxDisplay from './FaxDisplay';

const FaxReceiver = () => {
  const [isConnected, setIsConnected] = useState(false);
  const [imageType, setImageType] = useState('mono');
  const { currentFax, addToQueue, onDisplayComplete } = useFaxQueue();

  useEffect(() => {
    const eventSource = new EventSource('/events');

    eventSource.onopen = () => {
      setIsConnected(true);
      console.log('SSE connection opened');
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
    };

    return () => {
      eventSource.close();
    };
  }, [addToQueue]);

  return (
    <div className="h-screen text-white relative overflow-hidden" style={{ backgroundColor: 'transparent' }}>
      {/* コントロールパネル */}
      <div className="absolute top-4 right-4 bg-gray-800 rounded-lg p-4 shadow-lg z-10">
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-2">
            <div
              className={`w-3 h-3 rounded-full ${
                isConnected ? 'bg-green-500' : 'bg-red-500'
              }`}
            />
            <span className="text-sm">
              {isConnected ? '接続中' : '切断'}
            </span>
          </div>
          
          <div className="flex items-center gap-2">
            <label className="text-sm">画像タイプ:</label>
            <select
              value={imageType}
              onChange={(e) => setImageType(e.target.value)}
              className="bg-gray-700 text-white rounded px-2 py-1 text-sm"
            >
              <option value="mono">モノクロ</option>
              <option value="color">カラー</option>
            </select>
          </div>
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