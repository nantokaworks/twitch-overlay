// FAX関連の型定義

// FAXの状態
export type FaxDisplayState = 'loading' | 'waiting' | 'scrolling' | 'displaying' | 'sliding' | 'sliding-up' | 'complete';

// FAXデータ
export interface FaxData {
  id: string;
  type: 'fax';
  timestamp: number;
  username: string;
  displayName: string;
  message: string;
  imageUrl?: string;
}

// FAX状態オブジェクト
export interface FaxState {
  state: FaxDisplayState;
  progress: number;
}

// 画像タイプ
export type ImageType = 'mono' | 'color';

// FaxDisplayコンポーネントのProps
export interface FaxDisplayProps {
  faxData: FaxData;
  onComplete: () => void;
  imageType: ImageType;
  onLabelPositionUpdate: (position: number) => void;
  onAnimationStateChange: (isAnimating: boolean) => void;
  onStateChange?: (state: FaxState) => void;
}

// FaxReceiverコンポーネントのProps
export interface FaxReceiverProps {
  imageType?: ImageType;
}

// useFaxQueueフックの戻り値
export interface UseFaxQueueReturn {
  queue: FaxData[];
  currentFax: FaxData | null;
  isDisplaying: boolean;
  addToQueue: (faxData: FaxData) => void;
  onDisplayComplete: () => void;
}

// サーバーステータス
export interface ServerStatus {
  printerConnected: boolean;
}

// アニメーション用の動的スタイル
export interface DynamicStyles {
  transform?: string;
  transition?: string;
  top?: string;
  left?: string;
  width?: string;
  height?: string;
  opacity?: number;
  animation?: string;
  backgroundColor?: string;
  fontSize?: string;
  marginRight?: string;
}