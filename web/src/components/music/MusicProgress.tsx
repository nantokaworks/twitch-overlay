import { CSSProperties } from 'react';

interface MusicProgressProps {
  progress: number;
  isPlaying: boolean;
}

const MusicProgress = ({ progress, isPlaying }: MusicProgressProps) => {
  const containerStyle: CSSProperties = {
    position: 'fixed',
    bottom: 0,
    left: 0,
    width: '100%',
    height: '3px',
    backgroundColor: 'rgba(255, 255, 255, 0.1)',
    zIndex: 100,
    overflow: 'hidden',
  };

  const barStyle: CSSProperties = {
    height: '100%',
    width: `${progress}%`,
    backgroundColor: isPlaying ? '#1db954' : '#ffffff',
    transition: 'width 0.1s linear',
    boxShadow: isPlaying ? '0 0 10px rgba(29, 185, 84, 0.5)' : 'none',
  };

  return (
    <div style={containerStyle}>
      <div style={barStyle} />
    </div>
  );
};

export default MusicProgress;