import { useState, useEffect, useRef } from 'react';

const FaxDisplay = ({ faxData, onComplete, imageType }) => {
  const [imageLoaded, setImageLoaded] = useState(false);
  const [imageHeight, setImageHeight] = useState(0);
  const [animationComplete, setAnimationComplete] = useState(false);
  const containerRef = useRef(null);

  useEffect(() => {
    if (!faxData) return;

    // 画像のプリロード
    const img = new Image();
    img.onload = () => {
      setImageLoaded(true);
      // 画像の実際の高さを取得（最大幅200pxでの高さを計算）
      const aspectRatio = img.height / img.width;
      const displayWidth = Math.min(200, window.innerWidth);
      const displayHeight = displayWidth * aspectRatio;
      setImageHeight(displayHeight);
    };
    img.src = `/fax/${faxData.id}/${imageType}`;

    return () => {};
  }, [faxData, imageType]);

  useEffect(() => {
    if (!imageLoaded || !imageHeight) return;

    // スクロールアニメーションの時間を計算（画像の高さに基づく）
    const scrollDuration = Math.max(5000, (imageHeight / 100) * 1000); // 最低5秒、100pxあたり1秒
    
    // アニメーション完了タイマー
    const timer = setTimeout(() => {
      setAnimationComplete(true);
      setTimeout(onComplete, 500); // フェードアウト後に完了を通知
    }, scrollDuration + 5000); // スクロール時間 + 5秒待機

    return () => clearTimeout(timer);
  }, [imageLoaded, imageHeight, onComplete]);

  if (!faxData || !imageLoaded) return null;

  // アニメーション時間を動的に設定
  const animationDuration = Math.max(5, imageHeight / 100); // 100pxあたり1秒

  return (
    <div
      ref={containerRef}
      className={`fixed top-0 transition-opacity duration-500 ${
        animationComplete ? 'opacity-0' : 'opacity-100'
      }`}
      style={{
        left: '20px',
      }}
    >
      <div
        className="relative overflow-hidden"
        style={{
          width: '200px',
          height: '200px',
        }}
      >
        <img
          src={`/fax/${faxData.id}/${imageType}`}
          alt="FAX"
          className="w-full h-auto"
          style={{
            animation: `fax-image-scroll ${animationDuration}s ease-out forwards`,
          }}
        />
      </div>
    </div>
  );
};

export default FaxDisplay;