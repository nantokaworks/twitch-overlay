import { useState, useEffect } from 'react';
import { buildApiUrl } from '../../utils/api';
import MusicUploadModal from './MusicUploadModal';
import type { Track, Playlist } from '../../types/music';

const MusicManager = () => {
  const [tracks, setTracks] = useState<Track[]>([]);
  const [playlists, setPlaylists] = useState<Playlist[]>([]);
  const [selectedPlaylist, setSelectedPlaylist] = useState<string | null>(null);
  const [isUploadModalOpen, setIsUploadModalOpen] = useState(false);
  const [isCreatingPlaylist, setIsCreatingPlaylist] = useState(false);
  const [newPlaylistName, setNewPlaylistName] = useState('');
  const [isLoading, setIsLoading] = useState(true);

  // トラック一覧を取得
  const loadTracks = async () => {
    try {
      const response = await fetch(buildApiUrl('/api/music/tracks'));
      if (response.ok) {
        const data = await response.json();
        setTracks(data.tracks || []);
      }
    } catch (error) {
      console.error('Failed to load tracks:', error);
    }
  };

  // プレイリスト一覧を取得
  const loadPlaylists = async () => {
    try {
      const response = await fetch(buildApiUrl('/api/music/playlists'));
      if (response.ok) {
        const data = await response.json();
        setPlaylists(data.playlists || []);
      }
    } catch (error) {
      console.error('Failed to load playlists:', error);
    }
  };

  // 初期読み込み
  useEffect(() => {
    Promise.all([loadTracks(), loadPlaylists()]).then(() => {
      setIsLoading(false);
    });
  }, []);

  // トラック削除
  const handleDeleteTrack = async (trackId: string) => {
    if (!confirm('このトラックを削除しますか？')) return;

    try {
      const response = await fetch(buildApiUrl(`/api/music/track/${trackId}`), {
        method: 'DELETE',
      });
      if (response.ok) {
        setTracks(prev => prev.filter(t => t.id !== trackId));
      }
    } catch (error) {
      console.error('Failed to delete track:', error);
    }
  };

  // プレイリスト作成
  const handleCreatePlaylist = async () => {
    if (!newPlaylistName.trim()) return;

    try {
      const response = await fetch(buildApiUrl('/api/music/playlist'), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: newPlaylistName,
          description: '',
          track_ids: [],
        }),
      });
      
      if (response.ok) {
        const playlist = await response.json();
        setPlaylists(prev => [...prev, playlist]);
        setNewPlaylistName('');
        setIsCreatingPlaylist(false);
      }
    } catch (error) {
      console.error('Failed to create playlist:', error);
    }
  };

  // アップロード完了時の処理
  const handleUploadComplete = (track: Track) => {
    setTracks(prev => [track, ...prev]);
    setIsUploadModalOpen(false);
  };

  if (isLoading) {
    return <div style={{ padding: '20px', color: 'white' }}>読み込み中...</div>;
  }

  return (
    <div style={{ padding: '20px', color: 'white', backgroundColor: '#121212', minHeight: '100vh' }}>
      <h1>音楽管理</h1>

      <div style={{ marginBottom: '30px' }}>
        <button
          onClick={() => setIsUploadModalOpen(true)}
          style={{
            padding: '10px 20px',
            backgroundColor: '#1db954',
            color: 'white',
            border: 'none',
            borderRadius: '20px',
            cursor: 'pointer',
            fontSize: '16px',
            marginRight: '10px',
          }}
        >
          音楽をアップロード
        </button>

        <button
          onClick={() => setIsCreatingPlaylist(true)}
          style={{
            padding: '10px 20px',
            backgroundColor: '#444',
            color: 'white',
            border: 'none',
            borderRadius: '20px',
            cursor: 'pointer',
            fontSize: '16px',
          }}
        >
          プレイリストを作成
        </button>
      </div>

      {isCreatingPlaylist && (
        <div style={{ marginBottom: '20px', padding: '15px', backgroundColor: '#1e1e1e', borderRadius: '8px' }}>
          <input
            type="text"
            placeholder="プレイリスト名"
            value={newPlaylistName}
            onChange={(e) => setNewPlaylistName(e.target.value)}
            onKeyPress={(e) => e.key === 'Enter' && handleCreatePlaylist()}
            style={{
              padding: '8px',
              marginRight: '10px',
              backgroundColor: '#2a2a2a',
              border: '1px solid #444',
              borderRadius: '4px',
              color: 'white',
              width: '200px',
            }}
          />
          <button
            onClick={handleCreatePlaylist}
            style={{
              padding: '8px 16px',
              backgroundColor: '#1db954',
              color: 'white',
              border: 'none',
              borderRadius: '4px',
              cursor: 'pointer',
              marginRight: '10px',
            }}
          >
            作成
          </button>
          <button
            onClick={() => {
              setIsCreatingPlaylist(false);
              setNewPlaylistName('');
            }}
            style={{
              padding: '8px 16px',
              backgroundColor: '#444',
              color: 'white',
              border: 'none',
              borderRadius: '4px',
              cursor: 'pointer',
            }}
          >
            キャンセル
          </button>
        </div>
      )}

      <div style={{ display: 'flex', gap: '30px' }}>
        {/* プレイリスト一覧 */}
        <div style={{ flex: '0 0 250px' }}>
          <h2>プレイリスト</h2>
          <div style={{ backgroundColor: '#1e1e1e', borderRadius: '8px', padding: '10px' }}>
            <div
              onClick={() => setSelectedPlaylist(null)}
              style={{
                padding: '10px',
                cursor: 'pointer',
                backgroundColor: selectedPlaylist === null ? '#444' : 'transparent',
                borderRadius: '4px',
                marginBottom: '5px',
              }}
            >
              すべての曲
            </div>
            {playlists.map(playlist => (
              <div
                key={playlist.id}
                onClick={() => setSelectedPlaylist(playlist.id)}
                style={{
                  padding: '10px',
                  cursor: 'pointer',
                  backgroundColor: selectedPlaylist === playlist.id ? '#444' : 'transparent',
                  borderRadius: '4px',
                  marginBottom: '5px',
                }}
              >
                {playlist.name} ({playlist.track_count}曲)
              </div>
            ))}
          </div>
        </div>

        {/* トラック一覧 */}
        <div style={{ flex: 1 }}>
          <h2>トラック ({tracks.length}曲)</h2>
          <div style={{ backgroundColor: '#1e1e1e', borderRadius: '8px', padding: '15px' }}>
            {tracks.length === 0 ? (
              <p style={{ textAlign: 'center', color: '#666' }}>
                まだ音楽がアップロードされていません
              </p>
            ) : (
              <table style={{ width: '100%', borderCollapse: 'collapse' }}>
                <thead>
                  <tr style={{ borderBottom: '1px solid #333' }}>
                    <th style={{ textAlign: 'left', padding: '10px' }}>タイトル</th>
                    <th style={{ textAlign: 'left', padding: '10px' }}>アーティスト</th>
                    <th style={{ textAlign: 'left', padding: '10px' }}>アルバム</th>
                    <th style={{ padding: '10px' }}>アクション</th>
                  </tr>
                </thead>
                <tbody>
                  {tracks.map(track => (
                    <tr key={track.id} style={{ borderBottom: '1px solid #222' }}>
                      <td style={{ padding: '10px' }}>
                        {track.has_artwork && '🎵'} {track.title}
                      </td>
                      <td style={{ padding: '10px', color: '#aaa' }}>{track.artist}</td>
                      <td style={{ padding: '10px', color: '#aaa' }}>{track.album || '-'}</td>
                      <td style={{ padding: '10px', textAlign: 'center' }}>
                        <button
                          onClick={() => handleDeleteTrack(track.id)}
                          style={{
                            padding: '4px 12px',
                            backgroundColor: '#d32f2f',
                            color: 'white',
                            border: 'none',
                            borderRadius: '4px',
                            cursor: 'pointer',
                            fontSize: '12px',
                          }}
                        >
                          削除
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </div>
      </div>

      <MusicUploadModal
        isOpen={isUploadModalOpen}
        onClose={() => setIsUploadModalOpen(false)}
        onUploadComplete={handleUploadComplete}
      />
    </div>
  );
};

export default MusicManager;