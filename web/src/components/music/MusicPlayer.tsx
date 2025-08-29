import { useEffect, useState } from 'react';
import { useMusicPlayerContext } from '../../contexts/MusicPlayerContext';
import { buildApiUrl } from '../../utils/api';
import { useSettings } from '../../contexts/SettingsContext';
import MusicArtwork from './MusicArtwork';
import MusicProgress from './MusicProgress';

interface MusicPlayerProps {
  playlist?: string | undefined;
  enabled?: boolean;
}

const MusicPlayer = ({ playlist: propPlaylist, enabled: propEnabled }: MusicPlayerProps) => {
  const player = useMusicPlayerContext();
  const { settings } = useSettings();
  const [debugPanelPosition, setDebugPanelPosition] = useState({ x: window.innerWidth - 200, y: 10 });
  const [isDragging, setIsDragging] = useState(false);
  const [dragStart, setDragStart] = useState({ x: 0, y: 0 });
  
  // デバッグモードの確認
  const isDebug = new URLSearchParams(window.location.search).get('debug') === 'true';
  
  // Settings からプレイリストと有効状態を取得（propが優先）
  const enabled = propEnabled ?? (settings?.music_enabled ?? true);
  const playlist = propPlaylist ?? settings?.music_playlist ?? undefined;

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
  
  // ドラッグハンドラー
  const handleMouseDown = (e: React.MouseEvent) => {
    setIsDragging(true);
    setDragStart({
      x: e.clientX - debugPanelPosition.x,
      y: e.clientY - debugPanelPosition.y
    });
  };

  useEffect(() => {
    if (!isDragging) return;

    const handleMouseMove = (e: MouseEvent) => {
      setDebugPanelPosition({
        x: e.clientX - dragStart.x,
        y: e.clientY - dragStart.y
      });
    };

    const handleMouseUp = () => {
      setIsDragging(false);
    };

    document.addEventListener('mousemove', handleMouseMove);
    document.addEventListener('mouseup', handleMouseUp);

    return () => {
      document.removeEventListener('mousemove', handleMouseMove);
      document.removeEventListener('mouseup', handleMouseUp);
    };
  }, [isDragging, dragStart]);
  
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
      {/* デバッグ情報 - ドラッグ可能 */}
      {isDebug && (
        <div
          onMouseDown={handleMouseDown}
          style={{
            position: 'fixed',
            top: `${debugPanelPosition.y}px`,
            left: `${debugPanelPosition.x}px`,
            zIndex: 100,
            backgroundColor: 'rgba(0,0,0,0.8)',
            color: 'white',
            padding: '8px 12px',
            borderRadius: '6px',
            fontSize: '12px',
            fontFamily: 'monospace',
            border: '2px solid #10b981',
            cursor: isDragging ? 'grabbing' : 'grab',
            userSelect: 'none',
            opacity: isDragging ? 0.8 : 1,
            transition: isDragging ? 'none' : 'opacity 0.2s',
          }}
        >
          <div>Playing: {player.isPlaying ? '▶️' : '⏸️'}</div>
          <div>Track: {player.currentTrack?.title || 'None'}</div>
          <div>Volume: {player.volume}%</div>
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
            audioElement={player.audioElement}
          />
          
          {/* トラック情報 */}
          <div
            className="text-outline"
            style={{
              position: 'fixed',
              bottom: '26px',
              left: '140px',
              zIndex: 99,
              color: 'white',
              fontSize: '24px',
            }}
          >
            <div style={{ fontWeight: 'bold' }}>
              {player.currentTrack.title}
            </div>
            <div style={{ fontSize: '12px', marginTop: '10px' }}>
              {player.currentTrack.artist}
            </div>
          </div>
        </>
      )}
    </>
  );
};

export default MusicPlayer;