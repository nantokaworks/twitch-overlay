// APIのベースURLを取得
export function getApiBaseUrl(): string {
  // 環境変数が設定されている場合はそれを使用
  if (import.meta.env.VITE_API_BASE_URL) {
    return import.meta.env.VITE_API_BASE_URL;
  }
  
  // 本番環境では現在のホストとポートを使用（相対パス）
  if (import.meta.env.PROD) {
    // 本番環境: 現在のページと同じホスト・ポートを使用
    return '';
  }
  
  // 開発環境: Viteプロキシは通常のfetch用。EventSourceには完全なURLが必要
  return '';
}

// API URLを構築（通常のfetch用）
export function buildApiUrl(path: string): string {
  // 環境変数が設定されている場合
  if (import.meta.env.VITE_API_BASE_URL) {
    return `${import.meta.env.VITE_API_BASE_URL}${path}`;
  }
  
  // 本番環境
  if (import.meta.env.PROD) {
    return path;
  }
  
  // 開発環境: Viteプロキシが機能しない場合があるので、完全なURLを返す
  const backendPort = import.meta.env.VITE_BACKEND_PORT || '8080';
  return `http://localhost:${backendPort}${path}`;
}

// EventSource用のURLを構築（開発環境では完全なURLが必要）
export function buildEventSourceUrl(path: string): string {
  // 環境変数が設定されている場合
  if (import.meta.env.VITE_API_BASE_URL) {
    return `${import.meta.env.VITE_API_BASE_URL}${path}`;
  }
  
  // 本番環境
  if (import.meta.env.PROD) {
    return path;
  }
  
  // 開発環境: バックエンドポートを使用して完全なURLを構築
  const backendPort = import.meta.env.VITE_BACKEND_PORT || '8080';
  return `http://localhost:${backendPort}${path}`;
}