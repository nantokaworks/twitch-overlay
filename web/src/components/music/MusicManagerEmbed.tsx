import { useState, useEffect, useRef } from 'react';
import { buildApiUrl } from '../../utils/api';
import MusicUploadModal from './MusicUploadModal';
import { Button } from '../ui/button';
import { Upload, Plus, Trash2, Music as MusicIcon, ChevronLeft, ChevronRight, AlertTriangle, ListPlus } from 'lucide-react';
import type { Track, Playlist } from '../../types/music';

const MusicManagerEmbed = () => {
  const [tracks, setTracks] = useState<Track[]>([]);
  const [playlists, setPlaylists] = useState<Playlist[]>([]);
  const [selectedPlaylist, setSelectedPlaylist] = useState<string | null>(null);
  const [isUploadModalOpen, setIsUploadModalOpen] = useState(false);
  const [isCreatingPlaylist, setIsCreatingPlaylist] = useState(false);
  const [newPlaylistName, setNewPlaylistName] = useState('');
  const [isLoading, setIsLoading] = useState(true);
  const [currentPage, setCurrentPage] = useState(1);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  const [playlistTracks, setPlaylistTracks] = useState<Track[]>([]);
  const [activeDropdown, setActiveDropdown] = useState<string | null>(null);
  const [addingToPlaylist, setAddingToPlaylist] = useState<string | null>(null);
  const [selectedTracks, setSelectedTracks] = useState<string[]>([]);
  const [dropdownDirection, setDropdownDirection] = useState<{ [key: string]: 'up' | 'down' }>({});
  const [bulkAddingPlaylist, setBulkAddingPlaylist] = useState<string | null>(null);
  const [tracksPerPage, setTracksPerPage] = useState(20);
  const dropdownRefs = useRef<{ [key: string]: HTMLDivElement | null }>({});
  const buttonRefs = useRef<{ [key: string]: HTMLButtonElement | null }>({});

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
  
  // プレイリストトラックの読み込み
  useEffect(() => {
    if (selectedPlaylist) {
      loadPlaylistTracks(selectedPlaylist);
    } else {
      setPlaylistTracks([]);
    }
  }, [selectedPlaylist]);
  
  // ドロップダウンの外側クリック検知
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (activeDropdown && dropdownRefs.current[activeDropdown]) {
        const rect = dropdownRefs.current[activeDropdown]?.getBoundingClientRect();
        if (rect && (
          event.clientX < rect.left ||
          event.clientX > rect.right ||
          event.clientY < rect.top ||
          event.clientY > rect.bottom
        )) {
          setActiveDropdown(null);
        }
      }
    };
    
    document.addEventListener('click', handleClickOutside);
    return () => document.removeEventListener('click', handleClickOutside);
  }, [activeDropdown]);
  
  // ドロップダウンの位置を計算
  const calculateDropdownPosition = (trackId: string) => {
    const button = buttonRefs.current[trackId];
    if (!button) return;
    
    const rect = button.getBoundingClientRect();
    const dropdownHeight = 250; // 推定高さ
    const spaceBelow = window.innerHeight - rect.bottom;
    
    setDropdownDirection(prev => ({
      ...prev,
      [trackId]: spaceBelow < dropdownHeight ? 'up' : 'down'
    }));
  };

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

  // 全トラック削除
  const handleDeleteAllTracks = async () => {
    try {
      const response = await fetch(buildApiUrl('/api/music/track/all'), {
        method: 'DELETE',
      });
      if (response.ok) {
        setTracks([]);
        setCurrentPage(1);
        setShowDeleteConfirm(false);
      }
    } catch (error) {
      console.error('Failed to delete all tracks:', error);
    }
  };

  // プレイリストトラックを取得
  const loadPlaylistTracks = async (playlistId: string) => {
    try {
      const response = await fetch(buildApiUrl(`/api/music/playlist/${playlistId}/tracks`));
      if (response.ok) {
        const data = await response.json();
        setPlaylistTracks(data.tracks || []);
      }
    } catch (error) {
      console.error('Failed to load playlist tracks:', error);
    }
  };
  
  // トラックをプレイリストに追加
  const handleAddToPlaylist = async (trackId: string, playlistId: string) => {
    setAddingToPlaylist(trackId);
    try {
      const response = await fetch(buildApiUrl(`/api/music/playlist/${playlistId}`), {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          action: 'add_track',
          track_id: trackId,
          position: 0,
        }),
      });
      
      if (response.ok) {
        // プレイリストを再読み込み
        await loadPlaylists();
        if (selectedPlaylist === playlistId) {
          await loadPlaylistTracks(playlistId);
        }
        setActiveDropdown(null);
      }
    } catch (error) {
      console.error('Failed to add track to playlist:', error);
    } finally {
      setAddingToPlaylist(null);
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
    setCurrentPage(1); // 新しいトラックを表示するため最初のページに戻る
    // モーダルは複数ファイル対応のため、自動で閉じない
  };
  
  // 全選択/解除
  const handleSelectAll = () => {
    if (selectedTracks.length === currentTracks.length) {
      setSelectedTracks([]);
    } else {
      setSelectedTracks(currentTracks.map(t => t.id));
    }
  };
  
  // 個別選択
  const handleSelectTrack = (trackId: string, shiftKey: boolean) => {
    setSelectedTracks(prev => {
      if (shiftKey && prev.length > 0) {
        // Shiftキーで範囲選択
        const lastSelected = prev[prev.length - 1];
        const lastIndex = currentTracks.findIndex(t => t.id === lastSelected);
        const currentIndex = currentTracks.findIndex(t => t.id === trackId);
        const start = Math.min(lastIndex, currentIndex);
        const end = Math.max(lastIndex, currentIndex);
        const rangeIds = currentTracks.slice(start, end + 1).map(t => t.id);
        return [...new Set([...prev, ...rangeIds])];
      } else if (prev.includes(trackId)) {
        return prev.filter(id => id !== trackId);
      } else {
        return [...prev, trackId];
      }
    });
  };
  
  // 複数トラックをプレイリストに追加
  const handleBulkAddToPlaylist = async (playlistId: string) => {
    setBulkAddingPlaylist(playlistId);
    let successCount = 0;
    
    for (const trackId of selectedTracks) {
      try {
        const response = await fetch(buildApiUrl(`/api/music/playlist/${playlistId}`), {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            action: 'add_track',
            track_id: trackId,
            position: 0,
          }),
        });
        if (response.ok) successCount++;
      } catch (error) {
        console.error('Failed to add track:', error);
      }
    }
    
    // プレイリストを再読み込み
    await loadPlaylists();
    if (selectedPlaylist === playlistId) {
      await loadPlaylistTracks(playlistId);
    }
    
    setSelectedTracks([]);
    setBulkAddingPlaylist(null);
  };
  
  // 複数トラックを削除
  const handleBulkDelete = async () => {
    if (!confirm(`${selectedTracks.length}曲を削除しますか？`)) return;
    
    for (const trackId of selectedTracks) {
      try {
        await fetch(buildApiUrl(`/api/music/track/${trackId}`), {
          method: 'DELETE',
        });
      } catch (error) {
        console.error('Failed to delete track:', error);
      }
    }
    
    setTracks(prev => prev.filter(t => !selectedTracks.includes(t.id)));
    setSelectedTracks([]);
  };
  
  // アップロードボタンクリック時の処理
  const handleUploadClick = () => {
    // ファイル選択ダイアログを直接開く
    const input = document.createElement('input');
    input.type = 'file';
    input.multiple = true;
    input.accept = '.mp3,.wav,.m4a,.ogg';
    
    input.onchange = (e: Event) => {
      const files = Array.from((e.target as HTMLInputElement).files || []);
      if (files.length > 0) {
        setIsUploadModalOpen(true);
        // モーダルにファイルを渡すために一時的に保存
        (window as any).tempUploadFiles = files;
      }
    };
    
    input.click();
  };
  
  // 表示するトラックリスト（プレイリスト選択時はフィルタリング）
  const displayTracks = selectedPlaylist ? playlistTracks : tracks;
  
  // ページ切り替えまたは表示数変更時に選択をリセット
  useEffect(() => {
    setSelectedTracks([]);
  }, [currentPage, tracksPerPage]);
  
  // ページネーション計算
  const totalPages = Math.ceil(displayTracks.length / tracksPerPage);
  const startIndex = (currentPage - 1) * tracksPerPage;
  const endIndex = startIndex + tracksPerPage;
  const currentTracks = displayTracks.slice(startIndex, endIndex);

  if (isLoading) {
    return <div className="py-8 text-center text-gray-500">読み込み中...</div>;
  }

  return (
    <div>
      {/* アクションボタン */}
      <div className="mb-6 flex justify-between">
        <div className="flex gap-2">
          <Button
            onClick={handleUploadClick}
            variant="default"
            size="sm"
          >
            <Upload className="w-4 h-4 mr-2" />
            音楽をアップロード
          </Button>

          <Button
            onClick={() => setIsCreatingPlaylist(true)}
            variant="outline"
            size="sm"
          >
            <Plus className="w-4 h-4 mr-2" />
            プレイリストを作成
          </Button>
        </div>
        
        {tracks.length > 0 && (
          <Button
            onClick={() => setShowDeleteConfirm(true)}
            variant="destructive"
            size="sm"
          >
            <Trash2 className="w-4 h-4 mr-2" />
            すべて削除
          </Button>
        )}
      </div>

      {/* プレイリスト作成フォーム */}
      {isCreatingPlaylist && (
        <div className="mb-4 p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
          <div className="flex gap-2">
            <input
              type="text"
              placeholder="プレイリスト名"
              value={newPlaylistName}
              onChange={(e) => setNewPlaylistName(e.target.value)}
              onKeyPress={(e) => e.key === 'Enter' && handleCreatePlaylist()}
              className="flex-1 px-3 py-2 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
            <Button
              onClick={handleCreatePlaylist}
              size="sm"
              variant="default"
            >
              作成
            </Button>
            <Button
              onClick={() => {
                setIsCreatingPlaylist(false);
                setNewPlaylistName('');
              }}
              size="sm"
              variant="outline"
            >
              キャンセル
            </Button>
          </div>
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* プレイリスト一覧 */}
        <div className="lg:col-span-1">
          <h3 className="font-medium mb-3 text-gray-900 dark:text-gray-100">プレイリスト</h3>
          <div className="bg-gray-50 dark:bg-gray-800 rounded-lg p-2">
            <div
              onClick={() => setSelectedPlaylist(null)}
              className={`px-3 py-2 cursor-pointer rounded transition-colors ${
                selectedPlaylist === null
                  ? 'bg-white dark:bg-gray-700 shadow-sm'
                  : 'hover:bg-gray-100 dark:hover:bg-gray-700'
              }`}
            >
              すべての曲
            </div>
            {playlists.map(playlist => (
              <div
                key={playlist.id}
                onClick={() => setSelectedPlaylist(playlist.id)}
                className={`px-3 py-2 cursor-pointer rounded transition-colors mt-1 ${
                  selectedPlaylist === playlist.id
                    ? 'bg-white dark:bg-gray-700 shadow-sm'
                    : 'hover:bg-gray-100 dark:hover:bg-gray-700'
                }`}
              >
                <div className="flex justify-between items-center">
                  <span>{playlist.name}</span>
                  <span className="text-xs text-gray-500 dark:text-gray-400">
                    {playlist.track_count}曲
                  </span>
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* トラック一覧 */}
        <div className="lg:col-span-2">
          <h3 className="font-medium mb-3 text-gray-900 dark:text-gray-100">
            {selectedPlaylist ? 
              `プレイリスト: ${playlists.find(p => p.id === selectedPlaylist)?.name} (${displayTracks.length}曲)` : 
              `トラック (${tracks.length}曲)`
            }
          </h3>
          <div className="bg-gray-50 dark:bg-gray-800 rounded-lg p-4">
            {displayTracks.length === 0 ? (
              <div className="text-center py-12 text-gray-500 dark:text-gray-400">
                <MusicIcon className="w-12 h-12 mx-auto mb-3 opacity-50" />
                <p>まだ音楽がアップロードされていません</p>
                <p className="text-sm mt-2">上のボタンから音楽をアップロードしてください</p>
              </div>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full">
                  <thead>
                    <tr className="border-b dark:border-gray-700">
                      <th className="w-10 py-2 align-middle">
                        <input
                          type="checkbox"
                          className="rounded border-gray-300 dark:border-gray-600"
                          checked={selectedTracks.length > 0 && selectedTracks.length === currentTracks.length}
                          onChange={handleSelectAll}
                        />
                      </th>
                      <th className="w-12 py-2"></th>
                      <th className="text-left py-2 font-medium text-gray-700 dark:text-gray-300">タイトル</th>
                      <th className="text-left py-2 font-medium text-gray-700 dark:text-gray-300">アーティスト</th>
                      <th className="text-left py-2 font-medium text-gray-700 dark:text-gray-300 hidden md:table-cell">アルバム</th>
                      <th className="w-24 py-2"></th>
                    </tr>
                  </thead>
                  <tbody>
                    {currentTracks.map(track => (
                      <tr key={track.id} className="border-b dark:border-gray-700/50 group">
                        <td className="py-3">
                          <input
                            type="checkbox"
                            className="rounded border-gray-300 dark:border-gray-600"
                            checked={selectedTracks.includes(track.id)}
                            onClick={(e) => handleSelectTrack(track.id, e.shiftKey)}
                            onChange={() => {}}
                          />
                        </td>
                        <td className="py-3">
                          {/* アートワークサムネイル */}
                          <div className="w-8 h-8 rounded-full overflow-hidden bg-gray-200 dark:bg-gray-700 flex items-center justify-center">
                            {track.has_artwork ? (
                              <img
                                src={buildApiUrl(`/api/music/track/${track.id}/artwork`)}
                                alt=""
                                className="w-full h-full object-cover"
                                onError={(e) => {
                                  const target = e.target as HTMLImageElement;
                                  target.style.display = 'none';
                                  target.parentElement?.classList.add('no-artwork');
                                }}
                              />
                            ) : null}
                            <MusicIcon className="w-4 h-4 text-gray-400 hidden [.no-artwork_&]:block" />
                          </div>
                        </td>
                        <td className="py-3">
                          <span className="text-gray-900 dark:text-gray-100">
                            {track.title}
                          </span>
                        </td>
                        <td className="py-3 text-gray-600 dark:text-gray-400">
                          {track.artist}
                        </td>
                        <td className="py-3 text-gray-600 dark:text-gray-400 hidden md:table-cell">
                          {track.album || '-'}
                        </td>
                        <td className="py-3">
                          <div className="flex items-center justify-end gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                            {/* プレイリスト追加ボタン */}
                            {playlists.length > 0 && (
                              <div className="relative">
                                <Button
                                  ref={el => {
                                    if (el) buttonRefs.current[track.id] = el;
                                  }}
                                  onClick={(e) => {
                                    e.stopPropagation();
                                    calculateDropdownPosition(track.id);
                                    setActiveDropdown(activeDropdown === track.id ? null : track.id);
                                  }}
                                  size="sm"
                                  variant="ghost"
                                  className="text-gray-600 hover:text-gray-900 dark:text-gray-400 dark:hover:text-gray-100"
                                  disabled={addingToPlaylist === track.id}
                                >
                                  <ListPlus className="w-4 h-4" />
                                </Button>
                                
                                {/* プレイリストドロップダウン */}
                                {activeDropdown === track.id && (
                                  <div
                                    ref={el => {
                                      if (el) dropdownRefs.current[track.id] = el;
                                    }}
                                    className={`absolute right-0 ${dropdownDirection[track.id] === 'up' ? 'bottom-8' : 'top-8'} z-10 w-48 bg-white dark:bg-gray-800 rounded-md shadow-lg border border-gray-200 dark:border-gray-700 py-1`}
                                  >
                                    <div className="px-3 py-2 text-xs text-gray-500 dark:text-gray-400 border-b border-gray-200 dark:border-gray-700">
                                      プレイリストに追加
                                    </div>
                                    {playlists.map(playlist => {
                                      const isInPlaylist = selectedPlaylist === playlist.id && 
                                                           playlistTracks.some(t => t.id === track.id);
                                      return (
                                        <button
                                          key={playlist.id}
                                          onClick={() => handleAddToPlaylist(track.id, playlist.id)}
                                          disabled={isInPlaylist}
                                          className="w-full px-3 py-2 text-sm text-left hover:bg-gray-100 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed flex justify-between items-center"
                                        >
                                          <span>{playlist.name}</span>
                                          {isInPlaylist && (
                                            <span className="text-xs text-green-600 dark:text-green-400">✓</span>
                                          )}
                                        </button>
                                      );
                                    })}
                                  </div>
                                )}
                              </div>
                            )}
                            
                            {/* 削除ボタン */}
                            <Button
                              onClick={() => handleDeleteTrack(track.id)}
                              size="sm"
                              variant="ghost"
                              className="text-red-600 hover:text-red-700 hover:bg-red-50 dark:hover:bg-red-900/20"
                            >
                              <Trash2 className="w-4 h-4" />
                            </Button>
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
          
          {/* ページネーションコントロール */}
          {displayTracks.length > 0 && (
            <div className="mt-4 flex flex-col sm:flex-row justify-between items-center gap-4">
              {/* 左側: 表示数選択 */}
              <div className="flex items-center gap-2">
                <span className="text-sm text-gray-600 dark:text-gray-400">表示:</span>
                <select
                  value={tracksPerPage}
                  onChange={(e) => {
                    setTracksPerPage(Number(e.target.value));
                    setCurrentPage(1);
                    setSelectedTracks([]);
                  }}
                  className="px-2 py-1 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                >
                  <option value="10">10曲</option>
                  <option value="20">20曲</option>
                  <option value="50">50曲</option>
                  <option value="100">100曲</option>
                </select>
                <span className="text-sm text-gray-500 dark:text-gray-400">
                  (全{displayTracks.length}曲中 {startIndex + 1}-{Math.min(endIndex, displayTracks.length)}曲を表示)
                </span>
              </div>
              
              {/* 右側: ページ送り */}
              {totalPages > 1 && (
                <div className="flex items-center gap-1">
                  <Button
                    onClick={() => {
                      setCurrentPage(1);
                      setSelectedTracks([]);
                    }}
                    disabled={currentPage === 1}
                    size="sm"
                    variant="outline"
                    className="hidden sm:inline-flex"
                  >
                    最初
                  </Button>
                  <Button
                    onClick={() => {
                      setCurrentPage(prev => Math.max(1, prev - 1));
                      setSelectedTracks([]);
                    }}
                    disabled={currentPage === 1}
                    size="sm"
                    variant="outline"
                  >
                    <ChevronLeft className="w-4 h-4" />
                  </Button>
                  <div className="flex items-center gap-2 px-3">
                    <span className="text-sm font-medium">
                      {currentPage}
                    </span>
                    <span className="text-sm text-gray-500">/</span>
                    <span className="text-sm">
                      {totalPages}
                    </span>
                  </div>
                  <Button
                    onClick={() => {
                      setCurrentPage(prev => Math.min(totalPages, prev + 1));
                      setSelectedTracks([]);
                    }}
                    disabled={currentPage === totalPages}
                    size="sm"
                    variant="outline"
                  >
                    <ChevronRight className="w-4 h-4" />
                  </Button>
                  <Button
                    onClick={() => {
                      setCurrentPage(totalPages);
                      setSelectedTracks([]);
                    }}
                    disabled={currentPage === totalPages}
                    size="sm"
                    variant="outline"
                    className="hidden sm:inline-flex"
                  >
                    最後
                  </Button>
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      <MusicUploadModal
        isOpen={isUploadModalOpen}
        onClose={() => setIsUploadModalOpen(false)}
        onUploadComplete={handleUploadComplete}
        playlists={playlists}
        currentPlaylistId={selectedPlaylist}
        initialFiles={(window as any).tempUploadFiles}
      />
      
      {/* バルクアクションバー */}
      {selectedTracks.length > 0 && (
        <div className="fixed bottom-4 left-1/2 transform -translate-x-1/2 z-50 
          bg-gray-900 text-white p-4 rounded-lg shadow-xl flex items-center gap-4">
          <span className="text-sm">{selectedTracks.length}曲選択中</span>
          
          {/* プレイリストに追加 */}
          {playlists.length > 0 && (
            <div className="relative">
              <Button
                onClick={() => setActiveDropdown('bulk')}
                size="sm"
                variant="secondary"
                disabled={bulkAddingPlaylist !== null}
              >
                <ListPlus className="w-4 h-4 mr-2" />
                プレイリストに追加
              </Button>
              
              {activeDropdown === 'bulk' && (
                <div className="absolute bottom-12 left-0 w-48 bg-white dark:bg-gray-800 
                  rounded-md shadow-lg border border-gray-200 dark:border-gray-700 py-1">
                  <div className="px-3 py-2 text-xs text-gray-500 dark:text-gray-400 
                    border-b border-gray-200 dark:border-gray-700">
                    プレイリストを選択
                  </div>
                  {playlists.map(playlist => (
                    <button
                      key={playlist.id}
                      onClick={() => {
                        handleBulkAddToPlaylist(playlist.id);
                        setActiveDropdown(null);
                      }}
                      disabled={bulkAddingPlaylist === playlist.id}
                      className="w-full px-3 py-2 text-sm text-left text-gray-900 dark:text-gray-100
                        hover:bg-gray-100 dark:hover:bg-gray-700 
                        disabled:opacity-50 disabled:cursor-not-allowed"
                    >
                      {playlist.name}
                      {bulkAddingPlaylist === playlist.id && ' (追加中...)'}
                    </button>
                  ))}
                </div>
              )}
            </div>
          )}
          
          {/* 削除 */}
          <Button
            onClick={handleBulkDelete}
            size="sm"
            variant="destructive"
          >
            <Trash2 className="w-4 h-4 mr-2" />
            削除
          </Button>
          
          {/* キャンセル */}
          <Button
            onClick={() => setSelectedTracks([])}
            size="sm"
            variant="ghost"
            className="text-gray-300 hover:text-white"
          >
            キャンセル
          </Button>
        </div>
      )}
      
      {/* 削除確認モーダル */}
      {showDeleteConfirm && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white dark:bg-gray-800 rounded-lg p-6 max-w-md w-full mx-4">
            <div className="flex items-center mb-4">
              <AlertTriangle className="w-6 h-6 text-red-600 mr-3" />
              <h3 className="text-lg font-medium text-gray-900 dark:text-gray-100">
                すべてのトラックを削除
              </h3>
            </div>
            <p className="text-gray-600 dark:text-gray-400 mb-6">
              {tracks.length}曲のトラックがすべて削除されます。
              この操作は取り消せません。
            </p>
            <div className="flex justify-end gap-2">
              <Button
                onClick={() => setShowDeleteConfirm(false)}
                variant="outline"
                size="sm"
              >
                キャンセル
              </Button>
              <Button
                onClick={handleDeleteAllTracks}
                variant="destructive"
                size="sm"
              >
                <Trash2 className="w-4 h-4 mr-2" />
                すべて削除
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default MusicManagerEmbed;