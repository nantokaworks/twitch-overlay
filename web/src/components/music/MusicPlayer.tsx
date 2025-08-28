import { useEffect } from 'react';
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

  // プレイリストの読み込み
  useEffect(() => {
    if (enabled) {
      player.loadPlaylist(playlist);
    }
  }, [playlist, enabled]);

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
    
    const eventSource = new EventSource(buildEventSourceUrl('/api/music/control/events'));
    
    eventSource.onmessage = (event) => {
      try {
        const command = JSON.parse(event.data);
        console.log('Music control command received in overlay:', command);
        
        switch (command.type) {
          case 'play':
            player.play();
            break;
          case 'pause':
            player.pause();
            break;
          case 'next':
            player.next();
            break;
          case 'previous':
            player.previous();
            break;
          case 'volume':
            if (typeof command.value === 'number') {
              player.setVolume(command.value);
            }
            break;
          case 'load_playlist':
            if (command.playlist !== undefined) {
              player.loadPlaylist(command.playlist || undefined);
            }
            break;
        }
      } catch (error) {
        console.error('Failed to process music control command in overlay:', error);
      }
    };
    
    eventSource.onerror = (error) => {
      console.error('Music control SSE error in overlay:', error);
    };
    
    return () => {
      eventSource.close();
    };
  }, [enabled, player]);
  
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

  return (
    <>
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
            style={{
              position: 'fixed',
              bottom: '10px',
              left: '130px',
              zIndex: 99,
              color: 'white',
              textShadow: '2px 2px 4px rgba(0,0,0,0.8)',
            }}
          >
            <div style={{ fontSize: '16px', fontWeight: 'bold' }}>
              {player.currentTrack.title}
            </div>
            <div style={{ fontSize: '14px', opacity: 0.8 }}>
              {player.currentTrack.artist}
            </div>
          </div>
        </>
      )}
    </>
  );
};

export default MusicPlayer;