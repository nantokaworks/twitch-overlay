import { useState } from 'react';
import type { FaxData } from '../types';

interface DebugPanelProps {
  onSendFax: (faxData: FaxData) => void;
}

const DebugPanel = ({ onSendFax }: DebugPanelProps) => {
  const [isSubmitting, setIsSubmitting] = useState<boolean>(false);
  const [username, setUsername] = useState<string>('DebugUser');
  const [userInput, setUserInput] = useState<string>('');
  const [isExpanded, setIsExpanded] = useState<boolean>(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    
    if (!userInput.trim() || isSubmitting) return;

    setIsSubmitting(true);

    try {
      // バックエンドのデバッグエンドポイントに送信
      // HandleChannelPointsCustomRedemptionAddと同じ処理をエミュレート
      const response = await fetch('/debug/channel-points', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          username: username.toLowerCase(),
          displayName: username,
          userInput: userInput.trim(),
        }),
      });

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`Failed to send debug channel points: ${response.statusText} - ${errorText}`);
      }
      // フォームをリセット
      setUserInput('');
    } catch (error) {
      console.error('Failed to send debug channel points:', error);
      if (error instanceof Error) {
        alert(`デバッグチャンネルポイントの送信に失敗しました:\n${error.message}`);
      } else {
        alert('デバッグチャンネルポイントの送信に失敗しました。サーバーが起動しているか確認してください。');
      }
    } finally {
      setIsSubmitting(false);
    }
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
                メッセージ
              </label>
              <textarea
                value={userInput}
                onChange={(e) => setUserInput(e.target.value)}
                className="w-full px-3 py-2 bg-gray-700 text-white rounded border border-gray-600 focus:border-blue-500 focus:outline-none resize-none"
                style={{ fontSize: '14px' }}
                rows={3}
                placeholder="FAXに送信するメッセージ..."
                required
              />
            </div>
            
            <button
              type="submit"
              disabled={isSubmitting}
              className={`w-full py-2 rounded transition-colors font-medium ${
                isSubmitting 
                  ? 'bg-gray-600 text-gray-400 cursor-not-allowed' 
                  : 'bg-blue-600 text-white hover:bg-blue-700'
              }`}
              style={{ fontSize: '14px' }}
            >
              {isSubmitting ? '送信中...' : 'チャンネルポイントを使用'}
            </button>
          </form>
          
          <div className="mt-3 pt-3 border-t border-gray-700">
            <p className="text-gray-400 text-xs">
              TRIGGER_CUSTOM_REWORD_IDで設定された<br />
              チャンネルポイント報酬をエミュレート<br />
              <br />
              ※バックエンドでoutput.PrintOutが実行されます
            </p>
          </div>
        </div>
      )}
    </div>
  );
};

export default DebugPanel;