// APIのベースURLを取得
export function getApiBaseUrl(): string {
  // 環境変数が設定されている場合はそれを使用
  if (import.meta.env.VITE_API_BASE_URL) {
    return import.meta.env.VITE_API_BASE_URL;
  }
  
  // 本番環境では現在のホストとポートを使用（相対パス）
  // 開発環境ではViteのプロキシが処理するため空文字列でOK
  if (import.meta.env.PROD) {
    // 本番環境: 現在のページと同じホスト・ポートを使用
    return '';
  }
  
  // 開発環境: プロキシを使用
  return '';
}

// API URLを構築
export function buildApiUrl(path: string): string {
  const baseUrl = getApiBaseUrl();
  if (baseUrl) {
    return `${baseUrl}${path}`;
  }
  return path;
}