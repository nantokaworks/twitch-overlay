import { useEffect, useRef, useState } from 'react';

interface MusicVisualizerProps {
  audioElement: HTMLAudioElement | null;
  isPlaying: boolean;
  artworkUrl?: string;
}

const MusicVisualizer = ({ audioElement, isPlaying, artworkUrl }: MusicVisualizerProps) => {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const audioContextRef = useRef<AudioContext | null>(null);
  const analyserRef = useRef<AnalyserNode | null>(null);
  const sourceRef = useRef<MediaElementAudioSourceNode | null>(null);
  const animationIdRef = useRef<number | null>(null);
  const dataArrayRef = useRef<Uint8Array | null>(null);
  const isConnectedRef = useRef<boolean>(false);

  // Visualizer設定
  const BAR_COUNT = 32; // 放射状の線の数
  const DOT_LEVELS = 3; // 3段階表示
  const DOT_SIZE = 3; // ドットのサイズ（3x3ピクセル）
  const DOT_GAP = 3; // ドット間のギャップ
  const RADIUS = 55; // 基準半径（アートワークのサイズに合わせて）
  
  // 色設定（デフォルト値）
  const DEFAULT_COLOR = { r: 255, g: 179, b: 186 }; // デフォルト色
  const [baseColor, setBaseColor] = useState(DEFAULT_COLOR);
  
  // 透明度設定（レベルごと: 内側から外側へ）
  const OPACITY_LEVELS = [0.4, 0.3, 0.2];
  
  // 3段階の閾値（0-1の範囲）
  const THRESHOLD_LOW = 0.2;    // レベル1: 弱い音
  const THRESHOLD_MID = 0.5;    // レベル2: 中程度の音
  const THRESHOLD_HIGH = 0.75;  // レベル3: 強い音

  // アートワークから色を抽出
  const extractDominantColor = async (imageUrl: string) => {
    console.log('🎨 Extracting color from artwork:', imageUrl);
    try {
      const img = new Image();
      img.crossOrigin = 'anonymous';
      img.src = imageUrl;
      
      await new Promise((resolve, reject) => {
        img.onload = resolve;
        img.onerror = reject;
      });
      
      const canvas = document.createElement('canvas');
      const ctx = canvas.getContext('2d');
      if (!ctx) return DEFAULT_COLOR;
      
      // 小さいサイズにリサイズして平均色を取得
      const sampleSize = 10;
      canvas.width = sampleSize;
      canvas.height = sampleSize;
      ctx.drawImage(img, 0, 0, sampleSize, sampleSize);
      
      const imageData = ctx.getImageData(0, 0, sampleSize, sampleSize);
      const data = imageData.data;
      
      let r = 0, g = 0, b = 0;
      let count = 0;
      
      // 全ピクセルの平均色を計算
      for (let i = 0; i < data.length; i += 4) {
        r += data[i];
        g += data[i + 1];
        b += data[i + 2];
        count++;
      }
      
      r = Math.floor(r / count);
      g = Math.floor(g / count);
      b = Math.floor(b / count);
      
      console.log('📊 Raw extracted color:', { r, g, b });
      
      // 明度調整（暗すぎる場合は明るくする）
      const brightness = (r + g + b) / 3;
      console.log('💡 Brightness:', brightness);
      
      if (brightness < 80) {
        const factor = 120 / brightness;
        r = Math.min(255, Math.floor(r * factor));
        g = Math.min(255, Math.floor(g * factor));
        b = Math.min(255, Math.floor(b * factor));
        console.log('🔆 Adjusted for brightness:', { r, g, b });
      }
      
      // 彩度を少し上げる（グレーっぽい色を避ける）
      const max = Math.max(r, g, b);
      const min = Math.min(r, g, b);
      const saturation = max - min;
      console.log('🎨 Saturation:', saturation);
      
      if (saturation < 30) { // 閾値を50から30に下げる
        // 彩度が低い場合はデフォルト色を返す
        console.log('⚠️ Low saturation, using default color');
        return DEFAULT_COLOR;
      }
      
      console.log('✅ Final extracted color:', { r, g, b });
      return { r, g, b };
    } catch (error) {
      console.log('Failed to extract color from artwork:', error);
      return DEFAULT_COLOR;
    }
  };

  // アートワークURL変更時に色を抽出
  useEffect(() => {
    if (artworkUrl) {
      console.log('🖼️ Artwork URL changed:', artworkUrl);
      extractDominantColor(artworkUrl).then(color => {
        console.log('🎯 Setting base color to:', color);
        setBaseColor(color);
      });
    } else {
      console.log('⚠️ No artwork URL, using default color');
      setBaseColor(DEFAULT_COLOR);
    }
  }, [artworkUrl]);

  useEffect(() => {
    if (!audioElement || !canvasRef.current) return;

    // AudioContextの初期化とaudio要素の接続
    if (!isConnectedRef.current && audioElement) {
      console.log('Initializing AudioContext for Visualizer');
      
      // 既存のコンテキストがあればクリーンアップ
      if (audioContextRef.current) {
        audioContextRef.current.close();
      }
      
      const audioContext = new (window.AudioContext || (window as any).webkitAudioContext)();
      audioContextRef.current = audioContext;

      // AnalyserNodeの作成
      const analyser = audioContext.createAnalyser();
      analyser.fftSize = 512; // FFTサイズをさらに増やす
      analyser.smoothingTimeConstant = 0.85; // スムージングを調整
      analyser.minDecibels = -100; // より低い音も拾う
      analyser.maxDecibels = -30; // 最大デシベルも調整
      analyserRef.current = analyser;

      // データ配列の初期化
      const bufferLength = analyser.frequencyBinCount;
      dataArrayRef.current = new Uint8Array(bufferLength);
      console.log('Analyser buffer length:', bufferLength);

      try {
        // audio要素をAudioContextに接続
        const source = audioContext.createMediaElementSource(audioElement);
        source.connect(analyser);
        analyser.connect(audioContext.destination);
        sourceRef.current = source;
        isConnectedRef.current = true;
        console.log('Audio source connected successfully');
      } catch (error) {
        console.error('Failed to connect audio source:', error);
        isConnectedRef.current = false;
      }
    }

    // Canvas設定
    const canvas = canvasRef.current;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    // Canvasサイズ設定
    canvas.width = 140;
    canvas.height = 140;

    // アニメーション関数
    const draw = () => {
      if (!analyserRef.current || !dataArrayRef.current || !ctx) return;

      // 周波数データ取得
      analyserRef.current.getByteFrequencyData(dataArrayRef.current);

      // Canvasクリア
      ctx.clearRect(0, 0, canvas.width, canvas.height);

      // 中心座標
      const centerX = canvas.width / 2;
      const centerY = canvas.height / 2;

      // imageRenderingをpixelatedに設定（シャープなドット）
      ctx.imageSmoothingEnabled = false;
      
      // 各放射状の線をドットで描画
      for (let i = 0; i < BAR_COUNT; i++) {
        // データインデックスを計算（低周波数帯を重視）
        const dataIndex = Math.min(
          Math.floor(i * dataArrayRef.current.length / BAR_COUNT / 3), // 低周波数帯に集中
          dataArrayRef.current.length - 1
        );
        const value = dataArrayRef.current[dataIndex] || 0;
        
        // 音の強度を正規化（感度調整）
        const normalizedValue = Math.pow(value / 255, 0.7); // パワーカーブで感度調整
        
        // 3段階のレベルを決定
        let level = 0;
        if (normalizedValue > THRESHOLD_HIGH) {
          level = DOT_LEVELS; // 強い音：最大ドット数
        } else if (normalizedValue > THRESHOLD_MID) {
          level = 2; // 中程度：2つのドット
        } else if (normalizedValue > THRESHOLD_LOW) {
          level = 1; // 弱い音：1つのドット
        }
        
        // 角度計算
        const angle = (i / BAR_COUNT) * Math.PI * 2;
        
        // レベルに応じてドットを描画
        for (let j = 0; j < level; j++) {
          // ドットの距離を計算（90%に縮小）
          const distance = RADIUS + (j * (DOT_SIZE + DOT_GAP) * 0.9);
          
          // ドットの中心座標
          const dotX = centerX + Math.cos(angle) * distance;
          const dotY = centerY + Math.sin(angle) * distance;
          
          // ドットの左上座標（3x3の正方形なので-1.5）
          const x = Math.floor(dotX - DOT_SIZE / 2);
          const y = Math.floor(dotY - DOT_SIZE / 2);
          
          // ドットの透明度（定数から取得）
          const opacity = OPACITY_LEVELS[j] || OPACITY_LEVELS[OPACITY_LEVELS.length - 1];
          
          // ドットを描画（抽出した色を使用）
          ctx.fillStyle = `rgba(${baseColor.r}, ${baseColor.g}, ${baseColor.b}, ${opacity})`;
          ctx.fillRect(x, y, DOT_SIZE, DOT_SIZE);
        }
      }

      // 次のフレーム
      if (isPlaying) {
        animationIdRef.current = requestAnimationFrame(draw);
      }
    };

    // アニメーション開始/停止
    if (isPlaying) {
      // AudioContextのresume（ブラウザのポリシー対応）
      if (audioContextRef.current?.state === 'suspended') {
        audioContextRef.current.resume();
      }
      draw();
    } else {
      // アニメーション停止
      if (animationIdRef.current) {
        cancelAnimationFrame(animationIdRef.current);
        animationIdRef.current = null;
      }
      // Canvasクリア
      ctx.clearRect(0, 0, canvas.width, canvas.height);
    }

    // クリーンアップ
    return () => {
      if (animationIdRef.current) {
        cancelAnimationFrame(animationIdRef.current);
      }
    };
  }, [audioElement, isPlaying, baseColor]);

  return (
    <canvas
      ref={canvasRef}
      style={{
        position: 'absolute',
        top: '50%',
        left: '50%',
        transform: 'translate(-50%, -50%)',
        pointerEvents: 'none',
        zIndex: 0, // アートワークの背後
        imageRendering: 'pixelated', // ピクセルパーフェクトな描画
      }}
    />
  );
};

export default MusicVisualizer;