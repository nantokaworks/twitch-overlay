import { useEffect, useRef, useState } from 'react';
import { LAYOUT } from '../constants/layout';
import type { DynamicStyles, FaxDisplayProps, FaxDisplayState } from '../types';
import { buildApiUrl } from '../utils/api';

const FaxDisplay = ({ faxData, onComplete, imageType, onLabelPositionUpdate, onAnimationStateChange, onStateChange }: FaxDisplayProps) => {
  const [imageLoaded, setImageLoaded] = useState<boolean>(false);
  const [imageHeight, setImageHeight] = useState<number>(0);
  const [animationComplete, setAnimationComplete] = useState<boolean>(false);
  const [imagePosition, setImagePosition] = useState<number>(-1); // -1 means use 100% (初期位置)
  const [containerPosition, setContainerPosition] = useState<number>(0); // コンテナ全体の位置
  const [displayState, setDisplayState] = useState<FaxDisplayState>('loading'); // 'loading', 'waiting', 'scrolling', 'displaying', 'sliding', 'sliding-up', 'complete'
  const [scrollProgress, setScrollProgress] = useState<number>(0); // 0-100%
  const [currentLabelPosition, setCurrentLabelPosition] = useState<number>(0); // ラベルの現在位置を保持
  const containerRef = useRef<HTMLDivElement>(null);
  const animationRef = useRef<number | null>(null);
  const slideUpAnimationRef = useRef<number | null>(null);
  
  // 状態変更を通知
  useEffect(() => {
    if (onStateChange) {
      onStateChange({
        state: displayState,
        progress: scrollProgress
      });
    }
  }, [displayState, scrollProgress, onStateChange]);

  useEffect(() => {
    if (!faxData) return;

    setDisplayState('loading');
    // 画像のプリロード
    const img = new Image();
    img.onload = () => {
      // 画像の実際の高さを取得（最大幅でのFAX幅での高さを計算）
      const aspectRatio = img.height / img.width;
      const displayWidth = Math.min(LAYOUT.FAX_WIDTH, window.innerWidth);
      const displayHeight = displayWidth * aspectRatio;
      setImageHeight(displayHeight);
      
      setDisplayState('waiting');
      // 待機時間
      setTimeout(() => {
        setImageLoaded(true);
      }, LAYOUT.LAG_DURATION);
    };
    img.src = buildApiUrl(`/fax/${faxData.id}/${imageType}`);

    return () => {};
  }, [faxData, imageType]);

  useEffect(() => {
    if (!imageLoaded || !imageHeight) return;

    setDisplayState('scrolling');
    // アニメーション開始を通知
    if (onAnimationStateChange) {
      onAnimationStateChange(true);
    }
    
    // 毎フレームのピクセル移動量を計算
    const pixelsPerFrame = LAYOUT.PIXELS_PER_FRAME;
    let currentImagePosition = -imageHeight; // 開始位置（画像が完全に上にある状態）
    
    // アニメーションの進行状況を更新
    const updateAnimation = () => {
      currentImagePosition += pixelsPerFrame;
      
      // 画像位置を更新
      setImagePosition(currentImagePosition);
      
      // スクロール進捗を計算（0-100%）
      const progress = Math.min(100, Math.max(0, ((currentImagePosition + imageHeight) / imageHeight) * 100));
      setScrollProgress(Math.round(progress));
      
      // ラベル位置の計算
      // 画像と同じスピードで動くが、最大FAX高さまで（FAX文字とコンテンツ間にマージンを確保）
      const labelPosition = Math.min(Math.max(0, currentImagePosition + imageHeight + LAYOUT.FAX_CONTENT_TOP_MARGIN), LAYOUT.FAX_HEIGHT);
      
      setCurrentLabelPosition(labelPosition); // 現在位置を保存
      if (onLabelPositionUpdate) {
        onLabelPositionUpdate(Math.round(labelPosition));
      }
      
      // 画像が完全に表示されるまで続行
      if (currentImagePosition < 0) {
        animationRef.current = requestAnimationFrame(updateAnimation);
      } else {
        // アニメーション完了、表示時間待機
        setImagePosition(0); // 最終位置に固定
        setScrollProgress(100);
        setDisplayState('displaying');
        
        // 時計印刷かどうかで表示時間を分岐
        const displayDuration = faxData.username === "🕐 Clock" 
          ? LAYOUT.CLOCK_DISPLAY_DURATION 
          : LAYOUT.DISPLAY_DURATION;
        
        setTimeout(() => {
          setDisplayState('sliding');
          // スライドアップアニメーションを開始
          setTimeout(() => {
            setDisplayState('sliding-up');
            // この時点でのラベル位置を確実に取得
            // 高さが低いFAXの場合は実際の画像高さまでしか移動していない（マージンを考慮）
            const currentLabelPos = Math.min(imageHeight + LAYOUT.FAX_CONTENT_TOP_MARGIN, LAYOUT.FAX_HEIGHT);
            startSlideUpAnimation(currentLabelPos);
          }, LAYOUT.TRANSITION_DELAY);
        }, displayDuration);
      }
    };
    
    animationRef.current = requestAnimationFrame(updateAnimation);

    return () => {
      if (animationRef.current) {
        cancelAnimationFrame(animationRef.current);
      }
      if (slideUpAnimationRef.current) {
        cancelAnimationFrame(slideUpAnimationRef.current);
      }
    };
  }, [imageLoaded, imageHeight, onComplete, onLabelPositionUpdate, onAnimationStateChange]);

  // ease-out関数（3次ベジェ曲線）
  const easeOut = (t: number): number => {
    return 1 - Math.pow(1 - t, 3);
  };

  // スライドアップアニメーション
  const startSlideUpAnimation = (startLabelPosition: number) => {
    // 6px/フレームで移動
    const pixelsPerFrame = 6; // 6px/frame
    const targetDistance = LAYOUT.SLIDE_UP_DISTANCE; // -320px
    const totalDistance = Math.abs(targetDistance); // 320px
    const frames = totalDistance / pixelsPerFrame; // 約53フレーム
    const duration = frames * (1000 / 60); // 60fpsで約889ms
    
    const startTime = performance.now();
    const initialLabelPosition = startLabelPosition; // 引数から取得
    
    const animate = (currentTime: number) => {
      const elapsed = currentTime - startTime;
      const progress = Math.min(elapsed / duration, 1);
      // イージングなしで一定速度
      const easedProgress = progress;
      
      // コンテナとラベルを同時に更新
      const currentPosition = targetDistance * easedProgress;
      setContainerPosition(currentPosition);
      
      // ラベルはコンテナと同じ速度で移動
      // 単純に同じピクセル数だけ上に移動
      const labelPos = initialLabelPosition + currentPosition; // currentPositionは負の値
      
      // デバッグ用ログ（最初と最後のフレームのみ）
      if (progress === 0 || progress >= 0.99) {
        console.log('SlideUp:', {
          progress,
          initialLabelPosition,
          currentPosition,
          labelPos,
          finalLabelPos: Math.round(Math.max(0, labelPos))
        });
      }
      
      if (onLabelPositionUpdate) {
        onLabelPositionUpdate(Math.round(Math.max(0, labelPos)));
      }
      
      if (progress < 1) {
        slideUpAnimationRef.current = requestAnimationFrame(animate);
      } else {
        // アニメーション完了
        if (onAnimationStateChange) {
          onAnimationStateChange(false);
        }
        setAnimationComplete(true);
        setDisplayState('complete');
        onComplete();
      }
    };
    
    slideUpAnimationRef.current = requestAnimationFrame(animate);
  };

  if (!faxData || !imageLoaded) return null;

  // コンテナのスタイル
  const containerStyle: DynamicStyles = {
    left: `${LAYOUT.FAX_LEFT_MARGIN}px`,
    top: `${containerPosition}px`,
    transition: `opacity ${LAYOUT.FADE_DURATION}ms`,
  };

  // FAX表示エリアのスタイル
  const displayAreaStyle: DynamicStyles = {
    width: `${LAYOUT.FAX_WIDTH}px`,
    height: `${LAYOUT.FAX_HEIGHT}px`,
  };

  // 画像のスタイル
  const imageStyle: DynamicStyles = {
    transform: `translateY(${imagePosition === -1 ? '-100%' : `${imagePosition}px`})`,
    padding: '5px',
    backgroundColor: 'white',
    boxSizing: 'border-box',
  };

  return (
    <div
      ref={containerRef}
      className={`fixed ${
        animationComplete ? 'opacity-0' : 'opacity-100'
      }`}
      style={containerStyle}
    >
      <div
        className="relative overflow-hidden"
        style={displayAreaStyle}
      >
        <img
          src={buildApiUrl(`/fax/${faxData.id}/${imageType}`)}
          alt="FAX"
          className="w-full h-auto"
          style={imageStyle}
        />
      </div>
    </div>
  );
};

export default FaxDisplay;