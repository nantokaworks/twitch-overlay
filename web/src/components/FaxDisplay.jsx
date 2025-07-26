import { useState, useEffect, useRef } from 'react';

const FaxDisplay = ({ faxData, onComplete, imageType, onLabelPositionUpdate }) => {
  const [imageLoaded, setImageLoaded] = useState(false);
  const [imageHeight, setImageHeight] = useState(0);
  const [animationComplete, setAnimationComplete] = useState(false);
  const [imagePosition, setImagePosition] = useState(-1); // -1 means use 100% (初期位置)
  const containerRef = useRef(null);
  const animationRef = useRef(null);

  useEffect(() => {
    if (!faxData) return;

    // 画像のプリロード
    const img = new Image();
    img.onload = () => {
      setImageLoaded(true);
      // 画像の実際の高さを取得（最大幅250pxでの高さを計算）
      const aspectRatio = img.height / img.width;
      const displayWidth = Math.min(250, window.innerWidth);
      const displayHeight = displayWidth * aspectRatio;
      setImageHeight(displayHeight);
    };
    img.src = `/fax/${faxData.id}/${imageType}`;

    return () => {};
  }, [faxData, imageType]);

  useEffect(() => {
    if (!imageLoaded || !imageHeight) return;

    // 毎フレームのピクセル移動量を計算
    const pixelsPerFrame = 2; // 毎フレーム2ピクセル移動
    let currentImagePosition = -imageHeight; // 開始位置（画像が完全に上にある状態）
    
    // アニメーションの進行状況を更新
    const updateAnimation = () => {
      currentImagePosition += pixelsPerFrame;
      
      // 画像位置を更新
      setImagePosition(currentImagePosition);
      
      // ラベル位置の計算
      // 画像と同じスピードで動くが、最大250pxまで
      const labelPosition = Math.min(Math.max(0, currentImagePosition + imageHeight), 250);
      
      if (onLabelPositionUpdate) {
        onLabelPositionUpdate(labelPosition);
      }
      
      // 画像が完全に表示されるまで続行
      if (currentImagePosition < 0) {
        animationRef.current = requestAnimationFrame(updateAnimation);
      } else {
        // アニメーション完了、5秒待機
        setImagePosition(0); // 最終位置に固定
        setTimeout(() => {
          setAnimationComplete(true);
          setTimeout(onComplete, 500); // フェードアウト後に完了を通知
        }, 5000);
      }
    };
    
    animationRef.current = requestAnimationFrame(updateAnimation);

    return () => {
      if (animationRef.current) {
        cancelAnimationFrame(animationRef.current);
      }
    };
  }, [imageLoaded, imageHeight, onComplete, onLabelPositionUpdate]);

  if (!faxData || !imageLoaded) return null;

  return (
    <div
      ref={containerRef}
      className={`fixed transition-opacity duration-500 ${
        animationComplete ? 'opacity-0' : 'opacity-100'
      }`}
      style={{
        left: '20px',
        top: '0',
      }}
    >
      <div
        className="relative overflow-hidden"
        style={{
          width: '250px',
          height: '250px',
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