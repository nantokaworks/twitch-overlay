import React, { useState, useRef } from 'react';
import { buildApiUrl } from '../../utils/api';
import type { Track, Playlist } from '../../types/music';

interface FileUploadStatus {
  file: File;
  status: 'pending' | 'uploading' | 'completed' | 'error';
  progress: number;
  error?: string;
  trackId?: string;
}

interface MusicUploadModalProps {
  isOpen: boolean;
  onClose: () => void;
  onUploadComplete: (track: Track) => void;
  playlists?: Playlist[];
  currentPlaylistId?: string | null;
  initialFiles?: File[];
}

const MusicUploadModal = ({ isOpen, onClose, onUploadComplete, playlists = [], currentPlaylistId, initialFiles }: MusicUploadModalProps) => {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [uploadQueue, setUploadQueue] = useState<FileUploadStatus[]>([]);
  const [selectedPlaylistId, setSelectedPlaylistId] = useState<string | null>(currentPlaylistId || null);
  const [isUploading, setIsUploading] = useState(false);
  const [dragActive, setDragActive] = useState(false);
  const uploadingCountRef = useRef(0);
  const MAX_CONCURRENT_UPLOADS = 3;

  // 初期ファイルがある場合は処理
  React.useEffect(() => {
    if (isOpen && initialFiles && initialFiles.length > 0) {
      processFiles(initialFiles);
      // 一時ファイルをクリア
      (window as any).tempUploadFiles = undefined;
    }
  }, [isOpen]);

  // ファイル選択処理
  const handleFileSelect = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(e.target.files || []);
    processFiles(files);
  };

  // ドラッグ&ドロップ処理
  const handleDrag = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (e.type === "dragenter" || e.type === "dragover") {
      setDragActive(true);
    } else if (e.type === "dragleave") {
      setDragActive(false);
    }
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setDragActive(false);

    const files = Array.from(e.dataTransfer.files);
    processFiles(files);
  };

  // ファイル処理
  const processFiles = (files: File[]) => {
    const validFiles: FileUploadStatus[] = [];
    
    for (const file of files) {
      // ファイル形式チェック
      const validTypes = ['audio/mpeg', 'audio/mp3', 'audio/wav', 'audio/x-wav', 'audio/m4a', 'audio/ogg'];
      if (!validTypes.includes(file.type) && !file.name.match(/\.(mp3|wav|m4a|ogg)$/i)) {
        validFiles.push({
          file,
          status: 'error',
          progress: 0,
          error: 'サポートされていないファイル形式です',
        });
        continue;
      }

      // ファイルサイズチェック（50MB）
      if (file.size > 50 * 1024 * 1024) {
        validFiles.push({
          file,
          status: 'error',
          progress: 0,
          error: 'ファイルサイズが50MBを超えています',
        });
        continue;
      }

      validFiles.push({
        file,
        status: 'pending',
        progress: 0,
      });
    }

    setUploadQueue(prev => [...prev, ...validFiles]);
    
    // アップロード開始
    if (!isUploading) {
      startUploadProcess([...validFiles]);
    }
  };

  // アップロードプロセス管理
  const startUploadProcess = async (queue: FileUploadStatus[]) => {
    setIsUploading(true);
    
    const pendingFiles = queue.filter(f => f.status === 'pending');
    
    for (const fileStatus of pendingFiles) {
      // 同時アップロード数チェック
      while (uploadingCountRef.current >= MAX_CONCURRENT_UPLOADS) {
        await new Promise(resolve => setTimeout(resolve, 100));
      }
      
      uploadingCountRef.current++;
      uploadFile(fileStatus);
    }
  };

  // 個別ファイルアップロード
  const uploadFile = async (fileStatus: FileUploadStatus) => {
    // ステータス更新
    setUploadQueue(prev => prev.map(f => 
      f.file === fileStatus.file ? { ...f, status: 'uploading' } : f
    ));

    const formData = new FormData();
    formData.append('file', fileStatus.file);
    
    // プレイリストIDが選択されていれば追加
    if (selectedPlaylistId) {
      formData.append('playlist_id', selectedPlaylistId);
    }

    try {
      const xhr = new XMLHttpRequest();

      // 進捗トラッキング
      xhr.upload.addEventListener('progress', (event) => {
        if (event.lengthComputable) {
          const progress = Math.round((event.loaded / event.total) * 100);
          setUploadQueue(prev => prev.map(f => 
            f.file === fileStatus.file ? { ...f, progress } : f
          ));
        }
      });

      // 完了処理
      xhr.addEventListener('load', () => {
        uploadingCountRef.current--;
        
        if (xhr.status === 200) {
          const track: Track = JSON.parse(xhr.responseText);
          setUploadQueue(prev => prev.map(f => 
            f.file === fileStatus.file 
              ? { ...f, status: 'completed', progress: 100, trackId: track.id } 
              : f
          ));
          onUploadComplete(track);
        } else {
          setUploadQueue(prev => prev.map(f => 
            f.file === fileStatus.file 
              ? { ...f, status: 'error', error: `アップロード失敗: ${xhr.statusText}` } 
              : f
          ));
        }
        
        // 全て完了チェック
        checkAllCompleted();
      });

      // エラー処理
      xhr.addEventListener('error', () => {
        uploadingCountRef.current--;
        setUploadQueue(prev => prev.map(f => 
          f.file === fileStatus.file 
            ? { ...f, status: 'error', error: 'ネットワークエラー' } 
            : f
        ));
        checkAllCompleted();
      });

      // リクエスト送信
      xhr.open('POST', buildApiUrl('/api/music/upload'));
      xhr.send(formData);
    } catch (error) {
      uploadingCountRef.current--;
      setUploadQueue(prev => prev.map(f => 
        f.file === fileStatus.file 
          ? { ...f, status: 'error', error: 'アップロードエラー' } 
          : f
      ));
      checkAllCompleted();
    }
  };

  // 完了チェック
  const checkAllCompleted = () => {
    setTimeout(() => {
      setUploadQueue(prev => {
        const hasUploading = prev.some(f => f.status === 'uploading' || f.status === 'pending');
        if (!hasUploading) {
          setIsUploading(false);
        }
        return prev;
      });
    }, 100);
  };

  // リトライ
  const retryFailed = () => {
    const failedFiles = uploadQueue.filter(f => f.status === 'error');
    const resetFiles: FileUploadStatus[] = failedFiles.map(f => ({ 
      ...f, 
      status: 'pending' as const, 
      progress: 0
    }));
    setUploadQueue(prev => prev.map(f => {
      const reset = resetFiles.find(r => r.file === f.file);
      return reset || f;
    }));
    startUploadProcess(resetFiles);
  };

  // リセット
  const handleClose = () => {
    if (!isUploading) {
      setUploadQueue([]);
      setSelectedPlaylistId(currentPlaylistId || null);
      if (fileInputRef.current) {
        fileInputRef.current.value = '';
      }
      onClose();
    }
  };

  // 統計計算
  const completedCount = uploadQueue.filter(f => f.status === 'completed').length;
  const errorCount = uploadQueue.filter(f => f.status === 'error').length;
  const totalCount = uploadQueue.length;
  const overallProgress = totalCount > 0 
    ? Math.round((completedCount / totalCount) * 100) 
    : 0;

  if (!isOpen) return null;

  return (
    <div style={{
      position: 'fixed',
      top: 0,
      left: 0,
      right: 0,
      bottom: 0,
      backgroundColor: 'rgba(0, 0, 0, 0.8)',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      zIndex: 1000,
    }}>
      <div style={{
        backgroundColor: '#1e1e1e',
        borderRadius: '8px',
        padding: '24px',
        width: '600px',
        maxWidth: '90%',
        maxHeight: '80vh',
        color: 'white',
        display: 'flex',
        flexDirection: 'column',
      }}>
        <h2 style={{ margin: '0 0 20px 0' }}>音楽ファイルのアップロード</h2>

        {/* プレイリスト選択 */}
        {playlists.length > 0 && (
          <div style={{ marginBottom: '20px' }}>
            <label style={{ display: 'block', marginBottom: '8px', fontSize: '14px', color: '#aaa' }}>
              プレイリストに追加（オプション）
            </label>
            <select
              value={selectedPlaylistId || ''}
              onChange={(e) => setSelectedPlaylistId(e.target.value || null)}
              disabled={isUploading}
              style={{
                width: '100%',
                padding: '8px',
                backgroundColor: '#2a2a2a',
                border: '1px solid #444',
                borderRadius: '4px',
                color: 'white',
                cursor: isUploading ? 'not-allowed' : 'pointer',
              }}
            >
              <option value="">なし（すべての曲のみ）</option>
              {playlists.map(playlist => (
                <option key={playlist.id} value={playlist.id}>
                  {playlist.name}
                </option>
              ))}
            </select>
          </div>
        )}

        {/* ファイル選択エリア */}
        <div
          onDragEnter={handleDrag}
          onDragLeave={handleDrag}
          onDragOver={handleDrag}
          onDrop={handleDrop}
          style={{
            border: `2px dashed ${dragActive ? '#1db954' : '#444'}`,
            borderRadius: '8px',
            padding: '40px 20px',
            textAlign: 'center',
            backgroundColor: dragActive ? '#2a2a2a' : 'transparent',
            transition: 'all 0.2s',
            marginBottom: '20px',
          }}
        >
          <input
            ref={fileInputRef}
            type="file"
            accept=".mp3,.wav,.m4a,.ogg"
            multiple
            onChange={handleFileSelect}
            disabled={isUploading}
            style={{ display: 'none' }}
          />
          
          <p style={{ marginBottom: '10px' }}>
            ファイルをドラッグ&ドロップ
          </p>
          <p style={{ marginBottom: '20px', fontSize: '14px', color: '#888' }}>
            または
          </p>
          <button
            onClick={() => fileInputRef.current?.click()}
            disabled={isUploading}
            style={{
              padding: '10px 20px',
              backgroundColor: '#1db954',
              color: 'white',
              border: 'none',
              borderRadius: '20px',
              cursor: isUploading ? 'not-allowed' : 'pointer',
              opacity: isUploading ? 0.5 : 1,
            }}
          >
            ファイルを選択
          </button>
          <p style={{ marginTop: '10px', fontSize: '12px', color: '#666' }}>
            MP3, WAV, M4A, OGG (最大50MB/ファイル)
          </p>
        </div>

        {/* アップロード状況 */}
        {uploadQueue.length > 0 && (
          <div style={{
            flex: 1,
            overflowY: 'auto',
            marginBottom: '20px',
            maxHeight: '300px',
          }}>
            {/* 全体進捗 */}
            {totalCount > 0 && (
              <div style={{ marginBottom: '15px' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '5px' }}>
                  <span>
                    アップロード中 ({completedCount}/{totalCount})
                  </span>
                  {errorCount > 0 && (
                    <button
                      onClick={retryFailed}
                      style={{
                        padding: '2px 10px',
                        fontSize: '12px',
                        backgroundColor: '#d32f2f',
                        color: 'white',
                        border: 'none',
                        borderRadius: '4px',
                        cursor: 'pointer',
                      }}
                    >
                      失敗した{errorCount}件を再試行
                    </button>
                  )}
                </div>
                <div style={{
                  width: '100%',
                  height: '8px',
                  backgroundColor: '#2a2a2a',
                  borderRadius: '4px',
                  overflow: 'hidden',
                }}>
                  <div style={{
                    width: `${overallProgress}%`,
                    height: '100%',
                    backgroundColor: '#1db954',
                    transition: 'width 0.3s ease',
                  }} />
                </div>
              </div>
            )}

            {/* ファイルリスト */}
            {uploadQueue.map((file, index) => (
              <div key={index} style={{
                padding: '10px',
                marginBottom: '8px',
                backgroundColor: '#2a2a2a',
                borderRadius: '4px',
                border: `1px solid ${
                  file.status === 'completed' ? '#1db954' :
                  file.status === 'error' ? '#d32f2f' :
                  '#444'
                }`,
              }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '5px' }}>
                  <span style={{ fontSize: '14px' }}>
                    {file.status === 'completed' && '✓ '}
                    {file.status === 'error' && '✗ '}
                    {file.status === 'uploading' && '⟳ '}
                    {file.status === 'pending' && '○ '}
                    {file.file.name}
                  </span>
                  <span style={{ fontSize: '12px', color: '#888' }}>
                    {(file.file.size / 1024 / 1024).toFixed(1)} MB
                  </span>
                </div>
                
                {file.status === 'uploading' && (
                  <div style={{
                    width: '100%',
                    height: '4px',
                    backgroundColor: '#444',
                    borderRadius: '2px',
                    overflow: 'hidden',
                  }}>
                    <div style={{
                      width: `${file.progress}%`,
                      height: '100%',
                      backgroundColor: '#1db954',
                      transition: 'width 0.2s ease',
                    }} />
                  </div>
                )}
                
                {file.error && (
                  <div style={{
                    fontSize: '12px',
                    color: '#d32f2f',
                    marginTop: '5px',
                  }}>
                    {file.error}
                  </div>
                )}
              </div>
            ))}
          </div>
        )}

        {/* ボタン */}
        <div style={{ textAlign: 'right' }}>
          <button
            onClick={handleClose}
            disabled={isUploading}
            style={{
              padding: '8px 20px',
              backgroundColor: '#444',
              color: 'white',
              border: 'none',
              borderRadius: '4px',
              cursor: isUploading ? 'not-allowed' : 'pointer',
              opacity: isUploading ? 0.5 : 1,
            }}
          >
            {isUploading ? 'アップロード中...' : '閉じる'}
          </button>
        </div>
      </div>
    </div>
  );
};

export default MusicUploadModal;