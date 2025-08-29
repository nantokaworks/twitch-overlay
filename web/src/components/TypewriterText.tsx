import { useEffect, useState } from 'react';

interface TypewriterTextProps {
  text: string;
  speed?: number; // ミリ秒/文字
  delay?: number; // 開始前の遅延（ミリ秒）
  onComplete?: () => void;
  className?: string;
  style?: React.CSSProperties;
}

const TypewriterText = ({ 
  text, 
  speed = 40, 
  delay = 0,
  onComplete, 
  className = '',
  style = {}
}: TypewriterTextProps) => {
  const [displayedText, setDisplayedText] = useState('');
  const [currentIndex, setCurrentIndex] = useState(0);
  const [isStarted, setIsStarted] = useState(false);

  // テキストが変更されたらリセット
  useEffect(() => {
    setDisplayedText('');
    setCurrentIndex(0);
    setIsStarted(false);
    
    // 遅延後に開始
    if (text) {
      const startTimer = setTimeout(() => {
        setIsStarted(true);
      }, delay);
      
      return () => clearTimeout(startTimer);
    }
  }, [text, delay]);

  // タイピングアニメーション
  useEffect(() => {
    if (!isStarted || currentIndex >= text.length) {
      if (currentIndex >= text.length && onComplete) {
        onComplete();
      }
      return;
    }

    const timer = setTimeout(() => {
      setDisplayedText(prev => prev + text[currentIndex]);
      setCurrentIndex(prev => prev + 1);
    }, speed);

    return () => clearTimeout(timer);
  }, [currentIndex, text, speed, isStarted, onComplete]);

  return (
    <span className={className} style={style}>
      {displayedText || '\u00A0'}
    </span>
  );
};

export default TypewriterText;