import { useState, useEffect, useRef } from 'react';

const FaxDisplay = ({ faxData, onComplete, imageType, onLabelPositionUpdate, onAnimationStateChange }) => {
  const [imageLoaded, setImageLoaded] = useState(false);
  const [imageHeight, setImageHeight] = useState(0);
  const [animationComplete, setAnimationComplete] = useState(false);
  const [imagePosition, setImagePosition] = useState(-1); // -1 means use 100% (初期位置)
  const [containerPosition, setContainerPosition] = useState(0); // コンテナ全体の位置
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

    // アニメーション開始を通知
    if (onAnimationStateChange) {
      onAnimationStateChange(true);
    }
    
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
          // アニメーション終了を通知（トランジションを有効にする）
          if (onAnimationStateChange) {
            onAnimationStateChange(false);
          }
          // 少し待ってからスライドアニメーションを開始
          setTimeout(() => {
            // FAX表示領域を上にスライドさせる
            setContainerPosition(-290); // 250px (FAX高さ) + 40px (ラベル高さ)
            // ラベルも元の位置に戻す
            if (onLabelPositionUpdate) {
              onLabelPositionUpdate(0);
            }
            setTimeout(() => {
              setAnimationComplete(true);
              onComplete();
            }, 500); // スライドアップ完了後に終了
          }, 50); // トランジション切り替えの遅延
        }, 5000);
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
        left: '20px',
        top: `${containerPosition}px`,
        transition: containerPosition !== 0 ? 'top 0.5s ease-out, opacity 0.5s' : 'opacity 0.5s',
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