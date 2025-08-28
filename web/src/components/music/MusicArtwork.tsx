import { CSSProperties, useState, useEffect } from 'react';
import { buildApiUrl } from '../../utils/api';
import type { Track } from '../../types/music';

interface MusicArtworkProps {
  track: Track;
  isPlaying: boolean;
  onPlayPause: () => void;
}

const MusicArtwork = ({ track, isPlaying, onPlayPause }: MusicArtworkProps) => {
  const [imageError, setImageError] = useState(false);

  useEffect(() => {
    setImageError(false);
  }, [track.id]);

  const containerStyle: CSSProperties = {
    position: 'fixed',
    bottom: '20px',
    left: '20px',
    width: '100px',
    height: '100px',
    borderRadius: '50%',
    overflow: 'hidden',
    backgroundColor: '#282828',
    boxShadow: '0 4px 12px rgba(0, 0, 0, 0.5)',
    zIndex: 99,
    cursor: 'pointer',
    animation: isPlaying ? 'rotate 20s linear infinite' : 'none',
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

  // CSSアニメーション定義
  useEffect(() => {
    const style = document.createElement('style');
    style.textContent = `
      @keyframes rotate {
        from { transform: rotate(0deg); }
        to { transform: rotate(360deg); }
      }
    `;
    document.head.appendChild(style);
    return () => {
      document.head.removeChild(style);
    };
  }, []);

  return (
    <div style={containerStyle} onClick={onPlayPause}>
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
  );
};

export default MusicArtwork;