import { useState, useEffect } from 'react';
import { Button } from '../ui/button';
import { Play, Pause, Square, SkipForward, SkipBack, Volume2, Music } from 'lucide-react';
import { buildApiUrl, buildEventSourceUrl } from '../../utils/api';
import type { Playlist, Track } from '../../types/music';

interface MusicStatus {
  playback_status?: 'playing' | 'paused' | 'stopped';
  is_playing: boolean; // 互換性のため残す
  current_track?: Track;
  progress: number;
  current_time: number;
  duration: number;
  volume: number;
  playlist_name?: string;
}

const MusicPlayerControls = () => {
  const [playlists, setPlaylists] = useState<Playlist[]>([]);
  const [musicStatus, setMusicStatus] = useState<MusicStatus>({
    playback_status: 'stopped',
    is_playing: false,
    progress: 0,
    current_time: 0,
    duration: 0,
    volume: 70
  });

  // プレイリスト一覧を取得
  useEffect(() => {
    fetch(buildApiUrl('/api/music/playlists'))
      .then(res => res.json())
      .then(data => {
        setPlaylists(data.playlists || []);
      })
      .catch(console.error);
  }, []);
  
  // オーバーレイからの音楽状態を受信
  useEffect(() => {
    const eventSource = new EventSource(buildEventSourceUrl('/api/music/status/events'));
    
    eventSource.onmessage = (event) => {
      try {
        const status = JSON.parse(event.data);
        // playback_statusがない場合はis_playingから推測
        if (!status.playback_status) {
          status.playback_status = status.is_playing ? 'playing' : (status.current_track ? 'paused' : 'stopped');
        }
        setMusicStatus(status);
      } catch (error) {
        console.error('Failed to parse music status:', error);
      }
    };
    
    eventSource.onerror = (error) => {
      console.error('Music status SSE error:', error);
    };
    
    return () => {
      eventSource.close();
    };
  }, []);
  
  // リモートコントロール関数
  const sendControlCommand = async (endpoint: string, body?: any) => {
    try {
      const options: RequestInit = {
        method: 'POST'
      };
      
      // Only add headers and body if needed
      if (body) {
        options.headers = { 'Content-Type': 'application/json' };
        options.body = JSON.stringify(body);
      }
      
      await fetch(buildApiUrl(`/api/music/control/${endpoint}`), options);
    } catch (error) {
      console.error(`Failed to send ${endpoint} command:`, error);
    }
  };

  // 時間フォーマット
  const formatTime = (seconds: number) => {
    const mins = Math.floor(seconds / 60);
    const secs = Math.floor(seconds % 60);
    return `${mins}:${secs.toString().padStart(2, '0')}`;
  };

  // 現在の再生状態を取得（playback_statusを優先）
  const playbackStatus = musicStatus.playback_status || 
    (musicStatus.is_playing ? 'playing' : 
     musicStatus.current_track ? 'paused' : 'stopped');

  return (
    <div className="space-y-6">
      {/* 現在の曲情報 */}
      {musicStatus.current_track ? (
        <div className="flex items-start gap-4 p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
          {/* アートワーク */}
          <div className="w-20 h-20 flex-shrink-0">
            {musicStatus.current_track.has_artwork ? (
              <img
                src={buildApiUrl(`/api/music/track/${musicStatus.current_track.id}/artwork`)}
                alt={musicStatus.current_track.title}
                className="w-full h-full object-cover rounded-lg"
              />
            ) : (
              <div className="w-full h-full bg-gray-200 dark:bg-gray-700 rounded-lg flex items-center justify-center">
                <Music className="w-8 h-8 text-gray-400" />
              </div>
            )}
          </div>

          {/* 曲情報 */}
          <div className="flex-1 min-w-0">
            <h3 className="font-semibold text-lg truncate">{musicStatus.current_track.title}</h3>
            <p className="text-gray-600 dark:text-gray-400 truncate">{musicStatus.current_track.artist}</p>
            {musicStatus.current_track.album && (
              <p className="text-sm text-gray-500 dark:text-gray-500 truncate">{musicStatus.current_track.album}</p>
            )}
            <div className="mt-2 text-sm text-gray-500">
              {formatTime(musicStatus.current_time)} / {formatTime(musicStatus.duration)}
            </div>
          </div>
        </div>
      ) : (
        <div className="p-8 text-center text-gray-500 dark:text-gray-400">
          <Music className="w-12 h-12 mx-auto mb-3 opacity-50" />
          <p>{playbackStatus === 'stopped' ? '停止中' : '再生中の曲はありません'}</p>
        </div>
      )}

      {/* コントロールボタン */}
      <div className="flex items-center justify-center gap-2">
        <Button
          onClick={() => sendControlCommand('previous')}
          size="icon"
          variant="outline"
        >
          <SkipBack className="w-4 h-4" />
        </Button>
        
        <Button
          onClick={() => sendControlCommand(playbackStatus === 'playing' ? 'pause' : 'play')}
          size="icon"
          className="w-12 h-12"
        >
          {playbackStatus === 'playing' ? (
            <Pause className="w-5 h-5" />
          ) : (
            <Play className="w-5 h-5 ml-0.5" />
          )}
        </Button>
        
        <Button
          onClick={() => sendControlCommand('next')}
          size="icon"
          variant="outline"
        >
          <SkipForward className="w-4 h-4" />
        </Button>
        
        <Button
          onClick={() => sendControlCommand('stop')}
          size="icon"
          variant="outline"
          title="停止"
        >
          <Square className="w-4 h-4" />
        </Button>
      </div>

      {/* プログレスバー（表示のみ、シークは無効） */}
      {musicStatus.current_track && (
        <div className="space-y-2">
          <div className="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-2 overflow-hidden">
            <div
              className="bg-blue-500 h-full transition-all duration-200"
              style={{ width: `${musicStatus.progress}%` }}
            />
          </div>
          <p className="text-xs text-center text-gray-500">
            シークはオーバーレイ側で操作してください
          </p>
        </div>
      )}

      {/* ボリューム */}
      <div className="flex items-center gap-3">
        <Volume2 className="w-4 h-4 text-gray-500" />
        <input
          type="range"
          min="0"
          max="100"
          value={musicStatus.volume}
          onChange={(e) => sendControlCommand('volume', { volume: Number(e.target.value) })}
          className="flex-1"
        />
        <span className="text-sm text-gray-500 w-10 text-right">
          {musicStatus.volume}%
        </span>
      </div>

      {/* プレイリスト選択 */}
      <div className="space-y-2">
        <label className="text-sm font-medium">プレイリスト</label>
        <select
          value={musicStatus.playlist_name || ''}
          onChange={(e) => sendControlCommand('load', { playlist: e.target.value || undefined })}
          className="w-full px-3 py-2 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
        >
          <option value="">すべての曲</option>
          {playlists.map(playlist => (
            <option key={playlist.id} value={playlist.name}>
              {playlist.name} ({playlist.track_count}曲)
            </option>
          ))}
        </select>
      </div>

      {/* 統計情報 */}
      <div className="grid grid-cols-2 gap-4 text-sm">
        <div>
          <span className="text-gray-500">ステータス:</span>
          <span className="ml-2 font-medium">
            {playbackStatus === 'playing' ? '再生中' : 
             playbackStatus === 'paused' ? '一時停止' : '停止'}
          </span>
        </div>
        <div>
          <span className="text-gray-500">プレイリスト:</span>
          <span className="ml-2 font-medium">{musicStatus.playlist_name || 'すべて'}</span>
        </div>
        <div className="col-span-2">
          <span className="text-gray-500 text-xs">※ オーバーレイ側の音楽プレイヤーをリモート操作中</span>
        </div>
      </div>
    </div>
  );
};

export default MusicPlayerControls;