import { useState } from 'react';
import { buildApiUrl } from '../utils/api';
import type { FaxData } from '../types';

interface DebugPanelProps {
  onSendFax?: (faxData: FaxData) => void;
}

const DebugPanel = ({}: DebugPanelProps) => {
  const [isSubmitting, setIsSubmitting] = useState<boolean>(false);
  const [username, setUsername] = useState<string>('DebugUser');
  const [userInput, setUserInput] = useState<string>('');
  const [isExpanded, setIsExpanded] = useState<boolean>(false);
  
  // Additional event parameters
  const [bits, setBits] = useState<number>(100);
  const [viewers, setViewers] = useState<number>(10);
  const [months, setMonths] = useState<number>(3);
  const [resubMessage, setResubMessage] = useState<string>('デバッグ再サブスクメッセージ');
  const [fromBroadcaster, setFromBroadcaster] = useState<string>('DebugRaider');

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    
    if (!userInput.trim() || isSubmitting) return;

    setIsSubmitting(true);

    try {
      // バックエンドのデバッグエンドポイントに送信
      // HandleChannelPointsCustomRedemptionAddと同じ処理をエミュレート
      const response = await fetch(buildApiUrl('/debug/channel-points'), {
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

  const handleClock = async () => {
    setIsSubmitting(true);

    try {
      const response = await fetch(buildApiUrl('/debug/clock'), {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          withStats: true,  // リーダーボード情報を含む
        }),
      });

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`Failed to trigger clock: ${response.statusText} - ${errorText}`);
      }
      
      // 成功時はアラートを表示しない（エラー時のみ表示）
    } catch (error) {
      console.error('Failed to trigger clock:', error);
      if (error instanceof Error) {
        alert(`時計印刷の実行に失敗しました:\n${error.message}`);
      } else {
        alert('時計印刷の実行に失敗しました。サーバーが起動しているか確認してください。');
      }
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleClockEmpty = async () => {
    setIsSubmitting(true);

    try {
      const response = await fetch(buildApiUrl('/debug/clock'), {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          withStats: true,
          emptyLeaderboard: true,  // 空のリーダーボードをテスト
        }),
      });

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`Failed to trigger clock: ${response.statusText} - ${errorText}`);
      }
      
      // 成功時はアラートを表示しない（エラー時のみ表示）
    } catch (error) {
      console.error('Failed to trigger clock with empty leaderboard:', error);
      if (error instanceof Error) {
        alert(`時計印刷（空のリーダーボード）の実行に失敗しました:\n${error.message}`);
      } else {
        alert('時計印刷（空のリーダーボード）の実行に失敗しました。サーバーが起動しているか確認してください。');
      }
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleTwitchEvent = async (endpoint: string, data?: any) => {
    setIsSubmitting(true);

    try {
      const response = await fetch(buildApiUrl(`/debug/${endpoint}`), {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(data || {}),
      });

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`Failed to trigger ${endpoint}: ${response.statusText} - ${errorText}`);
      }
    } catch (error) {
      console.error(`Failed to trigger ${endpoint}:`, error);
      if (error instanceof Error) {
        alert(`イベント実行に失敗しました:\n${error.message}`);
      } else {
        alert('イベント実行に失敗しました。サーバーが起動しているか確認してください。');
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
        <div className="bg-gray-800 rounded-lg shadow-xl p-4" style={{ width: '350px', maxHeight: '80vh', overflowY: 'auto', fontFamily: 'system-ui, -apple-system, sans-serif' }}>
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
          
          {/* チャンネルポイント */}
          <div className="mb-4 pb-4 border-b border-gray-700">
            <h4 className="text-gray-300 text-sm font-semibold mb-3">📝 チャンネルポイント</h4>
            <form onSubmit={handleSubmit} className="space-y-3">
              <div>
                <label className="block text-gray-300 text-xs mb-1">
                  ユーザー名
                </label>
                <input
                  type="text"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  className="w-full px-3 py-1.5 bg-gray-700 text-white rounded border border-gray-600 focus:border-blue-500 focus:outline-none"
                  style={{ fontSize: '13px' }}
                  required
                />
              </div>
              
              <div>
                <label className="block text-gray-300 text-xs mb-1">
                  メッセージ
                </label>
                <textarea
                  value={userInput}
                  onChange={(e) => setUserInput(e.target.value)}
                  className="w-full px-3 py-1.5 bg-gray-700 text-white rounded border border-gray-600 focus:border-blue-500 focus:outline-none resize-none"
                  style={{ fontSize: '13px' }}
                  rows={2}
                  placeholder="FAXに送信するメッセージ..."
                  required
                />
              </div>
              
              <button
                type="submit"
                disabled={isSubmitting}
                className={`w-full py-1.5 rounded transition-colors font-medium ${
                  isSubmitting 
                    ? 'bg-gray-600 text-gray-400 cursor-not-allowed' 
                    : 'bg-blue-600 text-white hover:bg-blue-700'
                }`}
                style={{ fontSize: '13px' }}
              >
                {isSubmitting ? '送信中...' : 'チャンネルポイントを使用'}
              </button>
            </form>
          </div>

          {/* 時計印刷 */}
          <div className="mb-4 pb-4 border-b border-gray-700">
            <h4 className="text-gray-300 text-sm font-semibold mb-3">🕐 時計印刷</h4>
            <div className="space-y-2">
              <button
                onClick={handleClock}
                disabled={isSubmitting}
                className={`w-full py-1.5 rounded transition-colors font-medium ${
                  isSubmitting 
                    ? 'bg-gray-600 text-gray-400 cursor-not-allowed' 
                    : 'bg-purple-600 text-white hover:bg-purple-700'
                }`}
                style={{ fontSize: '13px' }}
              >
                {isSubmitting ? '実行中...' : 'リーダーボード付き'}
              </button>
              
              <button
                onClick={handleClockEmpty}
                disabled={isSubmitting}
                className={`w-full py-1.5 rounded transition-colors font-medium ${
                  isSubmitting 
                    ? 'bg-gray-600 text-gray-400 cursor-not-allowed' 
                    : 'bg-orange-600 text-white hover:bg-orange-700'
                }`}
                style={{ fontSize: '13px' }}
              >
                {isSubmitting ? '実行中...' : '空のリーダーボード'}
              </button>
            </div>
          </div>

          {/* サブスク関連イベント */}
          <div className="mb-4 pb-4 border-b border-gray-700">
            <h4 className="text-gray-300 text-sm font-semibold mb-3">⭐ サブスク関連</h4>
            <div className="space-y-2">
              <button
                onClick={() => handleTwitchEvent('subscribe', { username })}
                disabled={isSubmitting}
                className={`w-full py-1.5 rounded transition-colors font-medium ${
                  isSubmitting 
                    ? 'bg-gray-600 text-gray-400 cursor-not-allowed' 
                    : 'bg-pink-600 text-white hover:bg-pink-700'
                }`}
                style={{ fontSize: '13px' }}
              >
                サブスクライブ
              </button>

              <button
                onClick={() => handleTwitchEvent('gift-sub', { username, isAnonymous: false })}
                disabled={isSubmitting}
                className={`w-full py-1.5 rounded transition-colors font-medium ${
                  isSubmitting 
                    ? 'bg-gray-600 text-gray-400 cursor-not-allowed' 
                    : 'bg-pink-600 text-white hover:bg-pink-700'
                }`}
                style={{ fontSize: '13px' }}
              >
                サブギフト（通常）
              </button>

              <button
                onClick={() => handleTwitchEvent('gift-sub', { username: '匿名さん', isAnonymous: true })}
                disabled={isSubmitting}
                className={`w-full py-1.5 rounded transition-colors font-medium ${
                  isSubmitting 
                    ? 'bg-gray-600 text-gray-400 cursor-not-allowed' 
                    : 'bg-pink-600 text-white hover:bg-pink-700'
                }`}
                style={{ fontSize: '13px' }}
              >
                サブギフト（匿名）
              </button>

              <div className="space-y-1">
                <div className="flex gap-2">
                  <input
                    type="number"
                    value={months}
                    onChange={(e) => setMonths(parseInt(e.target.value) || 1)}
                    className="flex-1 px-2 py-1 bg-gray-700 text-white rounded border border-gray-600 focus:border-blue-500 focus:outline-none"
                    style={{ fontSize: '12px' }}
                    placeholder="月数"
                    min="1"
                  />
                  <input
                    type="text"
                    value={resubMessage}
                    onChange={(e) => setResubMessage(e.target.value)}
                    className="flex-1 px-2 py-1 bg-gray-700 text-white rounded border border-gray-600 focus:border-blue-500 focus:outline-none"
                    style={{ fontSize: '12px' }}
                    placeholder="メッセージ"
                  />
                </div>
                <button
                  onClick={() => handleTwitchEvent('resub', { 
                    username, 
                    cumulativeMonths: months,
                    message: resubMessage 
                  })}
                  disabled={isSubmitting}
                  className={`w-full py-1.5 rounded transition-colors font-medium ${
                    isSubmitting 
                      ? 'bg-gray-600 text-gray-400 cursor-not-allowed' 
                      : 'bg-pink-600 text-white hover:bg-pink-700'
                  }`}
                  style={{ fontSize: '13px' }}
                >
                  再サブスク（{months}ヶ月）
                </button>
              </div>
            </div>
          </div>

          {/* その他のイベント */}
          <div className="mb-4 pb-4 border-b border-gray-700">
            <h4 className="text-gray-300 text-sm font-semibold mb-3">🎉 その他のイベント</h4>
            <div className="space-y-2">
              <button
                onClick={() => handleTwitchEvent('follow', { username })}
                disabled={isSubmitting}
                className={`w-full py-1.5 rounded transition-colors font-medium ${
                  isSubmitting 
                    ? 'bg-gray-600 text-gray-400 cursor-not-allowed' 
                    : 'bg-green-600 text-white hover:bg-green-700'
                }`}
                style={{ fontSize: '13px' }}
              >
                フォロー
              </button>

              <div className="space-y-1">
                <input
                  type="number"
                  value={bits}
                  onChange={(e) => setBits(parseInt(e.target.value) || 100)}
                  className="w-full px-2 py-1 bg-gray-700 text-white rounded border border-gray-600 focus:border-blue-500 focus:outline-none"
                  style={{ fontSize: '12px' }}
                  placeholder="ビッツ数"
                  min="1"
                />
                <button
                  onClick={() => handleTwitchEvent('cheer', { username, bits })}
                  disabled={isSubmitting}
                  className={`w-full py-1.5 rounded transition-colors font-medium ${
                    isSubmitting 
                      ? 'bg-gray-600 text-gray-400 cursor-not-allowed' 
                      : 'bg-yellow-600 text-white hover:bg-yellow-700'
                  }`}
                  style={{ fontSize: '13px' }}
                >
                  チアー（{bits}ビッツ）
                </button>
              </div>

              <div className="space-y-1">
                <div className="flex gap-2">
                  <input
                    type="text"
                    value={fromBroadcaster}
                    onChange={(e) => setFromBroadcaster(e.target.value)}
                    className="flex-1 px-2 py-1 bg-gray-700 text-white rounded border border-gray-600 focus:border-blue-500 focus:outline-none"
                    style={{ fontSize: '12px' }}
                    placeholder="配信者名"
                  />
                  <input
                    type="number"
                    value={viewers}
                    onChange={(e) => setViewers(parseInt(e.target.value) || 10)}
                    className="w-20 px-2 py-1 bg-gray-700 text-white rounded border border-gray-600 focus:border-blue-500 focus:outline-none"
                    style={{ fontSize: '12px' }}
                    placeholder="人数"
                    min="1"
                  />
                </div>
                <button
                  onClick={() => handleTwitchEvent('raid', { fromBroadcaster, viewers })}
                  disabled={isSubmitting}
                  className={`w-full py-1.5 rounded transition-colors font-medium ${
                    isSubmitting 
                      ? 'bg-gray-600 text-gray-400 cursor-not-allowed' 
                      : 'bg-red-600 text-white hover:bg-red-700'
                  }`}
                  style={{ fontSize: '13px' }}
                >
                  レイド（{viewers}人）
                </button>
              </div>

              <button
                onClick={() => handleTwitchEvent('shoutout', { fromBroadcaster })}
                disabled={isSubmitting}
                className={`w-full py-1.5 rounded transition-colors font-medium ${
                  isSubmitting 
                    ? 'bg-gray-600 text-gray-400 cursor-not-allowed' 
                    : 'bg-indigo-600 text-white hover:bg-indigo-700'
                }`}
                style={{ fontSize: '13px' }}
              >
                シャウトアウト
              </button>
            </div>
          </div>

          {/* 配信状態 */}
          <div className="mb-4 pb-4 border-b border-gray-700">
            <h4 className="text-gray-300 text-sm font-semibold mb-3">📡 配信状態</h4>
            <div className="space-y-2">
              <button
                onClick={() => handleTwitchEvent('stream-online')}
                disabled={isSubmitting}
                className={`w-full py-1.5 rounded transition-colors font-medium ${
                  isSubmitting 
                    ? 'bg-gray-600 text-gray-400 cursor-not-allowed' 
                    : 'bg-teal-600 text-white hover:bg-teal-700'
                }`}
                style={{ fontSize: '13px' }}
              >
                配信開始
              </button>

              <button
                onClick={() => handleTwitchEvent('stream-offline')}
                disabled={isSubmitting}
                className={`w-full py-1.5 rounded transition-colors font-medium ${
                  isSubmitting 
                    ? 'bg-gray-600 text-gray-400 cursor-not-allowed' 
                    : 'bg-gray-500 text-white hover:bg-gray-600'
                }`}
                style={{ fontSize: '13px' }}
              >
                配信終了
              </button>
            </div>
          </div>
          
          <p className="text-gray-400 text-xs">
            ※バックエンドでoutput.PrintOutが実行されます
          </p>
        </div>
      )}
    </div>
  );
};

export default DebugPanel;