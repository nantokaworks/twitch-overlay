import { useEffect, useRef, useState } from 'react';
import { LAYOUT } from '../constants/layout';

const FaxDisplay = ({ faxData, onComplete, imageType, onLabelPositionUpdate, onAnimationStateChange, onStateChange }) => {
  const [imageLoaded, setImageLoaded] = useState(false);
  const [imageHeight, setImageHeight] = useState(0);
  const [animationComplete, setAnimationComplete] = useState(false);
  const [imagePosition, setImagePosition] = useState(-1); // -1 means use 100% (初期位置)
  const [containerPosition, setContainerPosition] = useState(0); // コンテナ全体の位置
  const [displayState, setDisplayState] = useState('loading'); // 'loading', 'waiting', 'scrolling', 'displaying', 'sliding', 'complete'
  const [scrollProgress, setScrollProgress] = useState(0); // 0-100%
  const containerRef = useRef(null);
  const animationRef = useRef(null);
  
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
    img.src = `/fax/${faxData.id}/${imageType}`;

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
      setScrollProgress(progress);
      
      // ラベル位置の計算
      // 画像と同じスピードで動くが、最大FAX高さまで
      const labelPosition = Math.min(Math.max(0, currentImagePosition + imageHeight), LAYOUT.FAX_HEIGHT);
      
      if (onLabelPositionUpdate) {
        onLabelPositionUpdate(labelPosition);
      }
      
      // 画像が完全に表示されるまで続行
      if (currentImagePosition < 0) {
        animationRef.current = requestAnimationFrame(updateAnimation);
      } else {
        // アニメーション完了、表示時間待機
        setImagePosition(0); // 最終位置に固定
        setScrollProgress(100);
        setDisplayState('displaying');
        setTimeout(() => {
          setDisplayState('sliding');
          // アニメーション終了を通知（トランジションを有効にする）
          if (onAnimationStateChange) {
            onAnimationStateChange(false);
          }
          // 少し待ってからスライドアニメーションを開始
          setTimeout(() => {
            // FAX表示領域を上にスライドさせる
            setContainerPosition(LAYOUT.SLIDE_UP_DISTANCE);
            // ラベルも元の位置に戻す
            if (onLabelPositionUpdate) {
              onLabelPositionUpdate(0);
            }
            setTimeout(() => {
              setAnimationComplete(true);
              setDisplayState('complete');
              onComplete();
            }, LAYOUT.FADE_DURATION); // スライドアップ完了後に終了
          }, LAYOUT.TRANSITION_DELAY); // トランジション切り替えの遅延
        }, LAYOUT.DISPLAY_DURATION);
      }
    };
    
    animationRef.current = requestAnimationFrame(updateAnimation);

    return () => {
      if (animationRef.current) {
        cancelAnimationFrame(animationRef.current);
      }
    };
  }, [imageLoaded, imageHeight, onComplete, onLabelPositionUpdate, onAnimationStateChange]);

  if (!faxData || !imageLoaded) return null;

  return (
    <div
      ref={containerRef}
      className={`fixed ${
        animationComplete ? 'opacity-0' : 'opacity-100'
      }`}
      style={{
        left: `${LAYOUT.LEFT_MARGIN}px`,
        top: `${containerPosition}px`,
        transition: containerPosition !== 0 ? `top ${LAYOUT.FADE_DURATION}ms ease-out, opacity ${LAYOUT.FADE_DURATION}ms` : `opacity ${LAYOUT.FADE_DURATION}ms`,
      }}
    >
      <div
        className="relative overflow-hidden"
        style={{
          width: `${LAYOUT.FAX_WIDTH}px`,
          height: `${LAYOUT.FAX_HEIGHT}px`,
        }}
      >
        <img
          src={`/fax/${faxData.id}/${imageType}`}
          alt="FAX"
          className="w-full h-auto"
          style={{
            transform: `translateY(${imagePosition === -1 ? '-100%' : `${imagePosition}px`})`,
          }}
        />
      </div>
    </div>
  );
};

export default FaxDisplay;