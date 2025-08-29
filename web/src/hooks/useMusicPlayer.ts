import { useState, useRef, useEffect, useCallback } from 'react';
import type { Track, MusicPlayerState } from '../types/music';
import { buildApiUrl } from '../utils/api';

interface UseMusicPlayerReturn extends MusicPlayerState {
  play: () => void;
  pause: () => void;
  next: () => void;
  previous: () => void;
  seek: (time: number) => void;
  setVolume: (volume: number) => void;
  loadPlaylist: (playlistName?: string) => Promise<void>;
  loadTrack: (track: Track) => void;
  clearHistory: () => void;
  audioElement: HTMLAudioElement | null;
}

// localStorage キー
const STORAGE_KEYS = {
  PLAYLIST_NAME: 'musicPlayer.playlistName',
  VOLUME: 'musicPlayer.volume',
  CURRENT_TRACK_ID: 'musicPlayer.currentTrackId',
  PLAY_HISTORY: 'musicPlayer.playHistory',
  WAS_PLAYING: 'musicPlayer.wasPlaying',
} as const;

// localStorageから値を安全に取得
const getFromStorage = <T>(key: string, defaultValue: T): T => {
  try {
    const stored = localStorage.getItem(key);
    return stored ? JSON.parse(stored) : defaultValue;
  } catch {
    return defaultValue;
  }
};

// localStorageに値を保存
const saveToStorage = (key: string, value: any): void => {
  try {
    localStorage.setItem(key, JSON.stringify(value));
  } catch (error) {
    console.error('Failed to save to localStorage:', error);
  }
};

export const useMusicPlayer = (initialVolume?: number): UseMusicPlayerReturn => {
  const audioRef = useRef<HTMLAudioElement | null>(null);
  const handleNextRef = useRef<(() => void) | null>(null);
  const isInitializedRef = useRef(false);
  
  // 保存された値を初期値として使用（Settingsからの音量を優先）
  const [state, setState] = useState<MusicPlayerState>({
    isPlaying: false,
    currentTrack: null,
    playlist: [],
    playlistName: getFromStorage(STORAGE_KEYS.PLAYLIST_NAME, null),
    progress: 0,
    currentTime: 0,
    duration: 0,
    volume: initialVolume ?? getFromStorage(STORAGE_KEYS.VOLUME, 70),
    isLoading: false,
    playHistory: getFromStorage(STORAGE_KEYS.PLAY_HISTORY, []),
  });

  // 状態をlocalStorageに保存
  useEffect(() => {
    if (!isInitializedRef.current) return;
    
    // プレイリスト名を保存
    if (state.playlistName !== undefined) {
      saveToStorage(STORAGE_KEYS.PLAYLIST_NAME, state.playlistName);
    }
    
    // 音量を保存
    saveToStorage(STORAGE_KEYS.VOLUME, state.volume);
    
    // 現在のトラックIDを保存
    if (state.currentTrack) {
      saveToStorage(STORAGE_KEYS.CURRENT_TRACK_ID, state.currentTrack.id);
    }
    
    // 再生履歴を保存
    saveToStorage(STORAGE_KEYS.PLAY_HISTORY, state.playHistory);
    
    // 再生状態を保存（ページ離脱時の復元用）
    saveToStorage(STORAGE_KEYS.WAS_PLAYING, state.isPlaying);
  }, [state.playlistName, state.volume, state.currentTrack?.id, state.playHistory, state.isPlaying]);
  
  // Settingsからの音量変更を反映
  useEffect(() => {
    if (initialVolume !== undefined && audioRef.current) {
      audioRef.current.volume = initialVolume / 100;
      setState(prev => ({ ...prev, volume: initialVolume }));
    }
  }, [initialVolume]);

  // オーディオ要素の初期化
  useEffect(() => {
    audioRef.current = new Audio();
    audioRef.current.crossOrigin = 'anonymous'; // CORS対応
    audioRef.current.volume = (initialVolume ?? state.volume) / 100;

    // イベントリスナー設定
    const audio = audioRef.current;

    const handleTimeUpdate = () => {
      if (audio.duration) {
        setState(prev => ({
          ...prev,
          currentTime: audio.currentTime,
          duration: audio.duration,
          progress: (audio.currentTime / audio.duration) * 100,
        }));
      }
    };

    const handleEnded = () => {
      // 2秒のインターバルを設けてから次の曲へ
      setState(prev => ({ ...prev, isPlaying: false }));
      setTimeout(() => {
        if (handleNextRef.current) {
          handleNextRef.current();
        }
      }, 2000);
    };

    const handleLoadedMetadata = () => {
      setState(prev => ({
        ...prev,
        duration: audio.duration,
        isLoading: false,
      }));
    };

    const handleError = (e: Event) => {
      console.error('Audio playback error:', e);
      setState(prev => ({
        ...prev,
        isPlaying: false,
        isLoading: false,
      }));
    };

    audio.addEventListener('timeupdate', handleTimeUpdate);
    audio.addEventListener('ended', handleEnded);
    audio.addEventListener('loadedmetadata', handleLoadedMetadata);
    audio.addEventListener('error', handleError);

    return () => {
      audio.removeEventListener('timeupdate', handleTimeUpdate);
      audio.removeEventListener('ended', handleEnded);
      audio.removeEventListener('loadedmetadata', handleLoadedMetadata);
      audio.removeEventListener('error', handleError);
      audio.pause();
      audio.src = '';
    };
  }, []);

  // ランダムで次のトラックを取得（履歴管理付き）
  const getNextRandomTrack = useCallback((): Track | null => {
    if (state.playlist.length === 0) return null;
    
    // 未再生のトラックを取得
    const unplayedTracks = state.playlist.filter(
      track => !state.playHistory.includes(track.id)
    );

    // 全て再生済みの場合は履歴をリセット
    if (unplayedTracks.length === 0) {
      setState(prev => ({ ...prev, playHistory: [] }));
      // 現在のトラックと異なるトラックを選択
      const availableTracks = state.playlist.filter(
        track => track.id !== state.currentTrack?.id
      );
      if (availableTracks.length === 0) return state.playlist[0];
      const randomIndex = Math.floor(Math.random() * availableTracks.length);
      return availableTracks[randomIndex];
    }

    // ランダムに未再生トラックを選択
    const randomIndex = Math.floor(Math.random() * unplayedTracks.length);
    return unplayedTracks[randomIndex];
  }, [state.playlist, state.playHistory, state.currentTrack]);

  // トラックを読み込む
  const loadTrack = useCallback((track: Track) => {
    if (!audioRef.current) return;

    setState(prev => ({
      ...prev,
      currentTrack: track,
      isLoading: true,
      currentTime: 0,
      progress: 0,
    }));

    audioRef.current.src = buildApiUrl(`/api/music/track/${track.id}/audio`);
    audioRef.current.load();

    // 自動再生が有効な場合
    if (state.isPlaying) {
      audioRef.current.play().catch(err => {
        console.error('Failed to auto-play:', err);
        setState(prev => ({ ...prev, isPlaying: false }));
      });
    }
  }, [state.isPlaying]);

  // 再生
  const play = useCallback(() => {
    if (!audioRef.current) return;

    // トラックが選択されていない場合は最初のトラックを選択
    if (!state.currentTrack && state.playlist.length > 0) {
      const firstTrack = getNextRandomTrack();
      if (firstTrack) {
        loadTrack(firstTrack);
      }
    }

    audioRef.current.play().then(() => {
      setState(prev => ({ ...prev, isPlaying: true }));
    }).catch(err => {
      console.error('Failed to play:', err);
    });
  }, [state.currentTrack, state.playlist, getNextRandomTrack, loadTrack]);

  // 一時停止
  const pause = useCallback(() => {
    if (!audioRef.current) return;
    audioRef.current.pause();
    setState(prev => ({ ...prev, isPlaying: false }));
  }, []);

  // 次の曲
  const handleNext = useCallback(() => {
    // 一旦停止して少し待つ
    if (audioRef.current) {
      audioRef.current.pause();
    }
    setState(prev => ({ ...prev, isPlaying: false }));
    
    setTimeout(() => {
      const nextTrack = getNextRandomTrack();
      if (nextTrack) {
        // 現在のトラックを履歴に追加
        if (state.currentTrack) {
          setState(prev => ({
            ...prev,
            playHistory: [...prev.playHistory, state.currentTrack!.id],
            isPlaying: true,
          }));
        } else {
          setState(prev => ({ ...prev, isPlaying: true }));
        }
        loadTrack(nextTrack);
      }
    }, 500); // next/prevボタンは少し短めのインターバル
  }, [getNextRandomTrack, loadTrack, state.currentTrack]);
  
  // handleNextの参照を更新
  useEffect(() => {
    handleNextRef.current = handleNext;
  }, [handleNext]);

  // 前の曲（履歴から）
  const previous = useCallback(() => {
    // 一旦停止して少し待つ
    if (audioRef.current) {
      audioRef.current.pause();
    }
    setState(prev => ({ ...prev, isPlaying: false }));
    
    setTimeout(() => {
      if (state.playHistory.length > 0) {
        const lastTrackId = state.playHistory[state.playHistory.length - 1];
        const track = state.playlist.find(t => t.id === lastTrackId);
        if (track) {
          setState(prev => ({
            ...prev,
            playHistory: prev.playHistory.slice(0, -1),
            isPlaying: true,
          }));
          loadTrack(track);
        }
      }
    }, 500); // next/prevボタンは少し短めのインターバル
  }, [state.playHistory, state.playlist, loadTrack]);

  // シーク
  const seek = useCallback((time: number) => {
    if (!audioRef.current) return;
    audioRef.current.currentTime = time;
    setState(prev => ({
      ...prev,
      currentTime: time,
      progress: (time / prev.duration) * 100,
    }));
  }, []);

  // ボリューム設定
  const setVolume = useCallback((volume: number) => {
    if (!audioRef.current) return;
    const clampedVolume = Math.max(0, Math.min(100, volume));
    audioRef.current.volume = clampedVolume / 100;
    setState(prev => ({ ...prev, volume: clampedVolume }));
  }, []);

  // サーバーから再生状態を復元
  const restoreServerState = useCallback(async (tracks: Track[]) => {
    try {
      const response = await fetch(buildApiUrl('/api/music/state/get'));
      if (!response.ok) return;
      
      const savedState = await response.json();
      console.log('🔄 Restoring server playback state:', savedState);
      
      // 保存されたトラックを探す
      const savedTrack = tracks.find(t => t.id === savedState.track_id);
      if (savedTrack && audioRef.current) {
        console.log('🎵 Found saved track:', savedTrack.title);
        console.log('📍 Saved position:', savedState.position);
        
        // stateを直接更新（loadTrackを経由しない）
        setState(prev => ({
          ...prev,
          currentTrack: savedTrack,
          isLoading: true,
          // 位置をリセットしない
          currentTime: savedState.position || 0,
          progress: savedState.duration ? (savedState.position / savedState.duration) * 100 : 0,
          duration: savedState.duration || 0
        }));
        
        // audio要素を直接操作
        audioRef.current.src = buildApiUrl(`/api/music/track/${savedTrack.id}/audio`);
        audioRef.current.load();
        
        // メタデータ読み込み後に位置を復元
        audioRef.current.addEventListener('loadedmetadata', () => {
          if (savedState.position > 0 && audioRef.current && 
              savedState.position < audioRef.current.duration) {
            audioRef.current.currentTime = savedState.position;
            console.log(`⏯️ Resuming from ${savedState.position.toFixed(1)}s`);
            
            // stateも更新
            setState(prev => ({
              ...prev,
              isLoading: false,
              duration: audioRef.current!.duration
            }));
          }
          
          // 音量も復元
          if (savedState.volume !== undefined && audioRef.current) {
            audioRef.current.volume = savedState.volume / 100;
            setState(prev => ({ ...prev, volume: savedState.volume }));
          }
        }, { once: true });
        
        // エラー時の処理
        audioRef.current.addEventListener('error', () => {
          setState(prev => ({ ...prev, isLoading: false }));
        }, { once: true });
      }
    } catch (error) {
      console.log('No saved playback state or failed to restore:', error);
    }
  }, []);

  // プレイリスト読み込み
  const loadPlaylist = useCallback(async (playlistName?: string) => {
    setState(prev => ({ ...prev, isLoading: true }));

    try {
      let tracks: Track[] = [];

      if (playlistName) {
        // 指定されたプレイリストを読み込む
        const response = await fetch(buildApiUrl(`/api/music/playlist/${playlistName}/tracks`));
        if (response.ok) {
          const data = await response.json();
          tracks = data.tracks || [];
          setState(prev => ({ ...prev, playlistName }));
        }
      } else {
        // 全トラックを読み込む
        const response = await fetch(buildApiUrl('/api/music/tracks'));
        if (response.ok) {
          const data = await response.json();
          tracks = data.tracks || [];
          setState(prev => ({ ...prev, playlistName: null }));
        }
      }

      setState(prev => ({
        ...prev,
        playlist: tracks,
        isLoading: false,
        playHistory: prev.playHistory.filter(id => 
          tracks.some(track => track.id === id)
        ), // プレイリストに存在するトラックのみ履歴に保持
      }));

      // 保存されたトラックを復元、もしくは最初の曲を選択
      if (tracks.length > 0) {
        // サーバーから状態を復元を優先
        if (!isInitializedRef.current) {
          await restoreServerState(tracks);
          isInitializedRef.current = true;
        } else {
          // 既に初期化済みの場合はローカルストレージから復元
          const savedTrackId = getFromStorage(STORAGE_KEYS.CURRENT_TRACK_ID, null);
          const wasPlaying = getFromStorage(STORAGE_KEYS.WAS_PLAYING, false);
          
          if (savedTrackId) {
            const savedTrack = tracks.find(t => t.id === savedTrackId);
            if (savedTrack) {
              console.log('🔄 Restoring saved track:', savedTrack.title);
              loadTrack(savedTrack);
              // 前回再生中だった場合は自動再生（ブラウザポリシーで制限される可能性あり）
              if (wasPlaying) {
                setTimeout(() => {
                  audioRef.current?.play().catch(() => {
                    console.log('Auto-play blocked by browser policy');
                  });
                }, 500);
              }
            } else {
              // 保存されたトラックが見つからない場合はランダム選択
              const firstTrack = tracks[Math.floor(Math.random() * tracks.length)];
              loadTrack(firstTrack);
            }
          } else if (!state.currentTrack) {
            // 現在のトラックがない場合はランダム選択
            const firstTrack = tracks[Math.floor(Math.random() * tracks.length)];
            loadTrack(firstTrack);
          }
        }
      }
    } catch (error) {
      console.error('Failed to load playlist:', error);
      setState(prev => ({ ...prev, isLoading: false }));
    }
  }, [loadTrack]);

  // 履歴クリア
  const clearHistory = useCallback(() => {
    setState(prev => ({ ...prev, playHistory: [] }));
  }, []);

  // サーバーに再生状態を送信
  const updateServerState = useCallback(async () => {
    if (!state.currentTrack || !audioRef.current) return;
    
    try {
      await fetch(buildApiUrl('/api/music/state/update'), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          track_id: state.currentTrack.id,
          position: audioRef.current.currentTime,
          duration: audioRef.current.duration,
          is_playing: state.isPlaying,
          volume: state.volume,
          playlist_name: state.playlistName
        })
      });
    } catch (error) {
      // サイレントに失敗（ログのみ）
      console.log('Failed to update server state:', error);
    }
  }, [state.currentTrack, state.isPlaying, state.volume, state.playlistName]);

  // 定期的に状態を送信（再生中のみ）
  useEffect(() => {
    if (state.isPlaying && state.currentTrack) {
      const interval = setInterval(updateServerState, 10000); // 10秒ごと
      return () => clearInterval(interval);
    }
  }, [state.isPlaying, state.currentTrack, updateServerState]);

  // 一時停止時にも状態を送信
  const pauseWithStateUpdate = useCallback(() => {
    pause();
    updateServerState();
  }, [pause, updateServerState]);

  // トラック変更時にも状態を送信
  useEffect(() => {
    if (state.currentTrack) {
      updateServerState();
    }
  }, [state.currentTrack?.id]); // IDが変わったときのみ

  return {
    ...state,
    play,
    pause: pauseWithStateUpdate, // 一時停止時に状態も送信
    next: handleNext,
    previous,
    seek,
    setVolume,
    loadPlaylist,
    loadTrack,
    clearHistory,
    audioElement: audioRef.current,
  };
};