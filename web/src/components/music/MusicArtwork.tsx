import { CSSProperties, useEffect, useState } from 'react';
import type { Track } from '../../types/music';
import { buildApiUrl } from '../../utils/api';
import MusicVisualizer from './MusicVisualizer';

interface MusicArtworkProps {
  track: Track;
  isPlaying: boolean;
  onPlayPause: () => void;
  audioElement?: HTMLAudioElement | null;
  rotation?: number;
}

const MusicArtwork = ({ track, isPlaying, onPlayPause, audioElement, rotation = 0 }: MusicArtworkProps) => {
  const [imageError, setImageError] = useState(false);

  useEffect(() => {
    setImageError(false);
  }, [track.id]);

  const containerStyle: CSSProperties = {
    position: 'relative',
    bottom: '20px',
    left: '20px',
    width: '100px',
    height: '100px',
    zIndex: 99,
    cursor: 'pointer',
  };
  
  const rotatingContainerStyle: CSSProperties = {
    position: 'relative',
    width: '100%',
    height: '100%',
    transform: `rotate(${rotation}deg)`,
  };

  const artworkContainerStyle: CSSProperties = {
    position: 'absolute',
    width: '100%',
    height: '100%',
    overflow: 'hidden',
    backgroundColor: '#282828',
    maskImage: 'url(/dot_record_mask.svg)',
    maskSize: '100% 100%',
    maskRepeat: 'no-repeat',
    maskPosition: 'center',
    WebkitMaskImage: 'url(/dot_record_mask.svg)',
    WebkitMaskSize: '100% 100%',
    WebkitMaskRepeat: 'no-repeat',
    WebkitMaskPosition: 'center',
    zIndex: 1,
  };

  const innerFrameStyle: CSSProperties = {
    position: 'absolute',
    width: '100%',
    height: '100%',
    imageRendering: 'pixelated',
    zIndex: 2,
    pointerEvents: 'none', // クリックイベントを透過
  };

  const imageStyle: CSSProperties = {
    width: '100%',
    height: '100%',
    objectFit: 'cover',
  };

  const placeholderStyle: CSSProperties = {
    width: '100%',
    height: '100%',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: '#404040',
    color: '#808080',
    fontSize: '40px',
  };


  return (
    <div style={containerStyle} onClick={onPlayPause}>
      {/* Visualizer（固定位置、回転しない） */}
      {audioElement && (
        <MusicVisualizer 
          audioElement={audioElement} 
          isPlaying={isPlaying}
          artworkUrl={track.has_artwork && !imageError ? buildApiUrl(`/api/music/track/${track.id}/artwork`) : undefined}
        />
      )}
      
      {/* 回転するコンテナ（アートワークとフレーム） */}
      <div style={rotatingContainerStyle}>
        {/* アートワーク（SVGマスク適用） */}
        <div style={artworkContainerStyle}>
          {track.has_artwork && !imageError ? (
            <img
              src={buildApiUrl(`/api/music/track/${track.id}/artwork`)}
              alt={`${track.title} artwork`}
              style={imageStyle}
              onError={() => setImageError(true)}
            />
          ) : (
            <div style={placeholderStyle}>
              ♪
            </div>
          )}
        </div>
        
        {/* ドット絵レコード内側装飾（最上層） */}
        <img
          src="/dot_record_inner.svg"
          alt="Record inner frame"
          style={innerFrameStyle}
        />
      </div>
    </div>
  );
};

export default MusicArtwork;