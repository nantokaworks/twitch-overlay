import React, { createContext, useContext, useEffect } from 'react';
import { useMusicPlayer } from '../hooks/useMusicPlayer';
import { buildEventSourceUrl } from '../utils/api';
import type { Track, MusicPlayerState } from '../types/music';

interface MusicPlayerContextValue extends MusicPlayerState {
  play: () => void;
  pause: () => void;
  next: () => void;
  previous: () => void;
  seek: (time: number) => void;
  setVolume: (volume: number) => void;
  loadPlaylist: (playlistName?: string) => Promise<void>;
  loadTrack: (track: Track) => void;
  clearHistory: () => void;
}

const MusicPlayerContext = createContext<MusicPlayerContextValue | null>(null);

export const MusicPlayerProvider = ({ children }: { children: React.ReactNode }) => {
  const player = useMusicPlayer();

  // APIからの制御を受け付ける
  useEffect(() => {
    const eventSource = new EventSource(buildEventSourceUrl('/api/music/control/events'));

    eventSource.onmessage = (event) => {
      try {
        const command = JSON.parse(event.data);
        console.log('Music control command received:', command);
        
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
            if (command.playlist) {
              player.loadPlaylist(command.playlist);
            }
            break;
        }
      } catch (error) {
        console.error('Failed to process music control command:', error);
      }
    };

    eventSource.onerror = (error) => {
      console.error('Music control SSE error:', error);
    };

    return () => {
      eventSource.close();
    };
  }, []);

  return (
    <MusicPlayerContext.Provider value={player}>
      {children}
    </MusicPlayerContext.Provider>
  );
};

export const useMusicPlayerContext = () => {
  const context = useContext(MusicPlayerContext);
  if (!context) {
    throw new Error('useMusicPlayerContext must be used within MusicPlayerProvider');
  }
  return context;
};