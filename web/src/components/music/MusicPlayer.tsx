import { useEffect, useState, useRef } from 'react';
import { useMusicPlayer } from '../../hooks/useMusicPlayer';
import MusicProgress from './MusicProgress';
import MusicArtwork from './MusicArtwork';
import { buildEventSourceUrl, buildApiUrl } from '../../utils/api';

interface MusicPlayerProps {
  playlist?: string | undefined;
  enabled?: boolean;
}

const MusicPlayer = ({ playlist, enabled = true }: MusicPlayerProps) => {
  const player = useMusicPlayer();
  const playerRef = useRef(player);
  const [sseStatus, setSseStatus] = useState<'connecting' | 'connected' | 'error' | 'disconnected'>('disconnected');
  const [lastCommand, setLastCommand] = useState<string>('');
  
  // デバッグモードの確認
  const isDebug = new URLSearchParams(window.location.search).get('debug') === 'true';
  
  // playerの参照を更新
  useEffect(() => {
    playerRef.current = player;
  }, [player]);

  // 初期化時に保存された状態を復元
  useEffect(() => {
    if (enabled && !playlist) {
      // URLパラメータでプレイリストが指定されていない場合、保存されたプレイリストを復元
      const savedPlaylistName = localStorage.getItem('musicPlayer.playlistName');
      if (savedPlaylistName) {
        const parsedName = JSON.parse(savedPlaylistName);
        console.log('🔄 Restoring saved playlist:', parsedName || 'All tracks');
        player.loadPlaylist(parsedName);
      } else {
        // 初回起動時はすべてのトラックを読み込む
        player.loadPlaylist(undefined);
      }
    } else if (enabled && playlist) {
      // URLパラメータで指定されている場合はそれを優先
      player.loadPlaylist(playlist);
    }
  }, [enabled]); // 初回のみ実行
  
  // プレイリストの変更を監視
  useEffect(() => {
    if (enabled && playlist !== undefined) {
      player.loadPlaylist(playlist);
    }
  }, [playlist]);

  // 手動スタートのため、自動再生は無効化
  // useEffect(() => {
  //   if (enabled && player.playlist.length > 0 && !player.currentTrack) {
  //     // 少し遅延を入れて自動再生
  //     const timer = setTimeout(() => {
  //       player.play();
  //     }, 1000);
  //     return () => clearTimeout(timer);
  //   }
  // }, [enabled, player.playlist.length]);
  
  // SSEでリモート制御を受信（オーバーレイ側のみ）
  useEffect(() => {
    if (!enabled) return;
    
    let reconnectTimer: NodeJS.Timeout;
    let reconnectCount = 0;
    const maxReconnectAttempts = 5;
    
    const connectSSE = () => {
      const sseUrl = buildEventSourceUrl('/api/music/control/events');
      if (reconnectCount === 0) {
        console.log('🔗 Connecting to music control SSE:', sseUrl);
        setSseStatus('connecting');
      }
      
      const eventSource = new EventSource(sseUrl);
      
      eventSource.onopen = () => {
        if (reconnectCount > 0) {
          console.log('✅ Music control SSE reconnected after', reconnectCount, 'attempts');
        } else {
          console.log('✅ Music control SSE connection established');
        }
        reconnectCount = 0; // リセット
        setSseStatus('connected');
      };
      
      eventSource.onmessage = (event) => {
        try {
          const command = JSON.parse(event.data);
          console.log('Music control command received in overlay:', command);
          setLastCommand(`${command.type} (${new Date().toLocaleTimeString()})`);
        
        switch (command.type) {
          case 'play':
            console.log('🎵 Executing PLAY command');
            playerRef.current.play();
            break;
          case 'pause':
            console.log('⏸️ Executing PAUSE command');
            playerRef.current.pause();
            break;
          case 'stop':
            console.log('⏹️ Executing STOP command (same as pause)');
            playerRef.current.pause();
            break;
          case 'next':
            console.log('⏭️ Executing NEXT command');
            playerRef.current.next();
            break;
          case 'previous':
            console.log('⏮️ Executing PREVIOUS command');
            playerRef.current.previous();
            break;
          case 'volume':
            if (typeof command.value === 'number') {
              console.log(`🔊 Executing VOLUME command: ${command.value}%`);
              playerRef.current.setVolume(command.value);
            }
            break;
          case 'load_playlist':
            if (command.playlist !== undefined) {
              console.log(`📂 Executing LOAD_PLAYLIST command: ${command.playlist || 'All tracks'}`);
              playerRef.current.loadPlaylist(command.playlist || undefined);
            }
            break;
          default:
            console.warn(`❌ Unknown music command type: ${command.type}`);
        }
      } catch (error) {
        console.error('Failed to process music control command in overlay:', error);
      }
    };
    
      eventSource.onerror = (error) => {
        setSseStatus('error');
        if (reconnectCount < maxReconnectAttempts) {
          reconnectCount++;
          const delay = Math.min(1000 * Math.pow(2, reconnectCount - 1), 10000); // 指数バックオフ（最大10秒）
          
          console.warn(`⚠️ Music control SSE error (attempt ${reconnectCount}/${maxReconnectAttempts}), reconnecting in ${delay}ms`);
          
          eventSource.close();
          setSseStatus('connecting');
          reconnectTimer = setTimeout(connectSSE, delay);
        } else {
          console.error('❌ Music control SSE failed after max attempts:', error);
          console.log('SSE readyState:', eventSource.readyState);
        }
      };
      
      return eventSource;
    };
    
    const eventSource = connectSSE();
    
    return () => {
      console.log('🔌 Closing music control SSE connection');
      setSseStatus('disconnected');
      if (reconnectTimer) {
        clearTimeout(reconnectTimer);
      }
      eventSource?.close();
    };
  }, [enabled]); // playerを依存から削除して無限ループを防ぐ
  
  // 音楽状態をサーバーに送信
  useEffect(() => {
    if (!enabled) return;
    
    const sendMusicStatus = async () => {
      try {
        await fetch(buildApiUrl('/api/music/status/update'), {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            is_playing: player.isPlaying,
            current_track: player.currentTrack,
            progress: player.progress,
            current_time: player.currentTime,
            duration: player.duration,
            volume: player.volume,
            playlist_name: player.playlistName
          })
        });
      } catch (error) {
        // サイレントに失敗（Settingsが開いていない場合など）
      }
    };
    
    // 状態が変化したときに送信
    sendMusicStatus();
    
    // 定期的に進捗状態を送信
    let interval: NodeJS.Timeout | null = null;
    if (player.isPlaying) {
      interval = setInterval(sendMusicStatus, 1000);
    }
    
    return () => {
      if (interval) clearInterval(interval);
    };
  }, [enabled, player.isPlaying, player.currentTrack?.id, player.progress, player.volume, player.playlistName, buildApiUrl]);

  if (!enabled) return null;

  const getStatusColor = (status: typeof sseStatus) => {
    switch (status) {
      case 'connected': return '#10b981'; // green
      case 'connecting': return '#f59e0b'; // yellow  
      case 'error': return '#ef4444'; // red
      case 'disconnected': return '#6b7280'; // gray
      default: return '#6b7280';
    }
  };

  const getStatusText = (status: typeof sseStatus) => {
    switch (status) {
      case 'connected': return '接続中';
      case 'connecting': return '接続中...';
      case 'error': return 'エラー';
      case 'disconnected': return '切断';
      default: return '不明';
    }
  };

  return (
    <>
      {/* デバッグ情報 - 右上 */}
      {isDebug && (
        <div
          style={{
            position: 'fixed',
            top: '10px',
            right: '10px',
            zIndex: 100,
            backgroundColor: 'rgba(0,0,0,0.8)',
            color: 'white',
            padding: '8px 12px',
            borderRadius: '6px',
            fontSize: '12px',
            fontFamily: 'monospace',
            border: `2px solid ${getStatusColor(sseStatus)}`,
          }}
        >
          <div>SSE: <span style={{ color: getStatusColor(sseStatus) }}>{getStatusText(sseStatus)}</span></div>
          {lastCommand && <div>Last: {lastCommand}</div>}
          <div>Playing: {player.isPlaying ? '▶️' : '⏸️'}</div>
        </div>
      )}
      
      {/* プログレスバー - 最下部 */}
      <MusicProgress
        progress={player.progress}
        isPlaying={player.isPlaying}
      />
      
      {/* アートワーク＋トラック情報 - 左下 */}
      {player.currentTrack && (
        <>
          <MusicArtwork
            track={player.currentTrack}
            isPlaying={player.isPlaying}
            onPlayPause={() => player.isPlaying ? player.pause() : player.play()}
          />
          
          {/* トラック情報 */}
          <div
            className="text-outline"
            style={{
              position: 'fixed',
              bottom: '10px',
              left: '130px',
              zIndex: 99,
              color: 'white',
              fontSize: '24px',
            }}
          >
            <div style={{ fontWeight: 'bold' }}>
              {player.currentTrack.title}
            </div>
            <div style={{ fontSize: '18px' }}>
              {player.currentTrack.artist}
            </div>
          </div>
        </>
      )}
    </>
  );
};

export default MusicPlayer;