import { useState } from 'react';
import type { FaxData } from '../types';

interface DebugPanelProps {
  onSendFax: (faxData: FaxData) => void;
}

const DebugPanel = ({ onSendFax }: DebugPanelProps) => {
  const [username, setUsername] = useState<string>('DebugUser');
  const [message, setMessage] = useState<string>('');
  const [imageUrl, setImageUrl] = useState<string>('');
  const [isExpanded, setIsExpanded] = useState<boolean>(false);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    
    if (!message.trim()) return;

    // メッセージから画像URLを抽出する処理（サーバーと同じロジック）
    let finalImageUrl = imageUrl.trim();
    if (!finalImageUrl) {
      // URLパターンを検索（http/https画像URL）
      const urlPattern = /(https?:\/\/[^\s]+\.(jpg|jpeg|png|gif|webp)(\?[^\s]*)?)/gi;
      const matches = message.match(urlPattern);
      if (matches && matches.length > 0) {
        finalImageUrl = matches[0];
      }
    }

    const faxData: FaxData = {
      id: `debug-${Date.now()}`,
      type: 'fax',
      timestamp: Date.now(),
      username: username.toLowerCase(),
      displayName: username,
      message: message.trim(),
      imageUrl: finalImageUrl || undefined,
    };

    onSendFax(faxData);
    
    // フォームをリセット
    setMessage('');
    setImageUrl('');
  };

  return (
    <div className="fixed bottom-4 right-4 z-50">
      {!isExpanded ? (
        <button
          onClick={() => setIsExpanded(true)}
          className="bg-gray-800 text-white px-4 py-2 rounded-lg shadow-lg hover:bg-gray-700 transition-colors"
          style={{ fontSize: '14px', fontFamily: 'system-ui, -apple-system, sans-serif' }}
        >
          Debug Panel
        </button>
      ) : (
        <div className="bg-gray-800 rounded-lg shadow-xl p-4" style={{ width: '300px', fontFamily: 'system-ui, -apple-system, sans-serif' }}>
          <div className="flex justify-between items-center mb-4">
            <h3 className="text-white font-bold" style={{ fontSize: '16px' }}>Debug Panel</h3>
            <button
              onClick={() => setIsExpanded(false)}
              className="text-gray-400 hover:text-white"
              style={{ fontSize: '20px' }}
            >
              ×
            </button>
          </div>
          
          <form onSubmit={handleSubmit} className="space-y-3">
            <div>
              <label className="block text-gray-300 text-sm mb-1">
                ユーザー名
              </label>
              <input
                type="text"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                className="w-full px-3 py-2 bg-gray-700 text-white rounded border border-gray-600 focus:border-blue-500 focus:outline-none"
                style={{ fontSize: '14px' }}
                required
              />
            </div>
            
            <div>
              <label className="block text-gray-300 text-sm mb-1">
                メッセージ <span className="text-gray-500">(必須)</span>
              </label>
              <textarea
                value={message}
                onChange={(e) => setMessage(e.target.value)}
                className="w-full px-3 py-2 bg-gray-700 text-white rounded border border-gray-600 focus:border-blue-500 focus:outline-none resize-none"
                style={{ fontSize: '14px' }}
                rows={3}
                placeholder="FAXメッセージを入力..."
                required
              />
            </div>
            
            <div>
              <label className="block text-gray-300 text-sm mb-1">
                画像URL <span className="text-gray-500">(オプション)</span>
              </label>
              <input
                type="url"
                value={imageUrl}
                onChange={(e) => setImageUrl(e.target.value)}
                className="w-full px-3 py-2 bg-gray-700 text-white rounded border border-gray-600 focus:border-blue-500 focus:outline-none"
                style={{ fontSize: '14px' }}
                placeholder="https://example.com/image.jpg"
              />
            </div>
            
            <button
              type="submit"
              className="w-full bg-blue-600 text-white py-2 rounded hover:bg-blue-700 transition-colors font-medium"
              style={{ fontSize: '14px' }}
            >
              FAX送信 (TRIGGER_CUSTOM_REWORD_ID)
            </button>
          </form>
          
          <div className="mt-3 pt-3 border-t border-gray-700">
            <p className="text-gray-400 text-xs">
              このパネルはカスタムリワードIDでの<br />
              FAX送信をエミュレートします。<br />
              <br />
              ※メッセージ内の画像URLは自動検出されます
            </p>
          </div>
        </div>
      )}
    </div>
  );
};

export default DebugPanel;