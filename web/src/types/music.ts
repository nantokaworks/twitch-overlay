export interface Track {
  id: string;
  filename: string;
  title: string;
  artist: string;
  album: string;
  duration: number;
  has_artwork: boolean;
  created_at: string;
}

export interface Playlist {
  id: string;
  name: string;
  description: string;
  created_at: string;
  track_count: number;
}

export interface PlaylistTrack extends Track {
  position: number;
}

export type PlaybackStatus = 'playing' | 'paused' | 'stopped';

export interface MusicPlayerState {
  playbackStatus: PlaybackStatus;
  isPlaying: boolean; // 互換性のため残す（playbackStatus === 'playing'と同じ）
  currentTrack: Track | null;
  playlist: Track[];
  playlistName: string | null;
  progress: number; // 0-100
  currentTime: number;
  duration: number;
  volume: number; // 0-100
  isLoading: boolean;
  playHistory: string[]; // 再生履歴（ランダム再生用）
}

export interface MusicUploadProgress {
  isUploading: boolean;
  progress: number;
  error: string | null;
}