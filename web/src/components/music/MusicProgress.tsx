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
    height: '4px',
    backgroundColor: 'rgba(255, 255, 255, 0.15)',
    zIndex: 100,
    overflow: 'hidden',
  };

  const barStyle: CSSProperties = {
    height: '100%',
    width: `${progress}%`,
    backgroundColor: isPlaying ? '#50fa7b' : '#ffffff',
    transition: isPlaying ? 'width 0.5s linear, background-color 0.3s ease' : 'width 0.3s ease-out, background-color 0.3s ease',
    boxShadow: isPlaying ? '0 0 10px rgba(29, 185, 84, 0.5)' : 'none',
    transform: 'translateZ(0)', // ハードウェアアクセラレーションを有効化
    willChange: 'width', // ブラウザに変更を事前通知
  };

  return (
    <div style={containerStyle}>
      <div style={barStyle} />
    </div>
  );
};

export default MusicProgress;