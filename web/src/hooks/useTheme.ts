import { useState, useEffect } from 'react';

export type Theme = 'light' | 'dark';

// システムのテーマ設定を取得
const getSystemTheme = (): Theme => {
  if (typeof window !== 'undefined' && window.matchMedia) {
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
  }
  return 'light';
};

export const useTheme = () => {
  // 初期テーマの決定: localStorage > システム設定 > デフォルト(light)
  const [theme, setTheme] = useState<Theme>(() => {
    const savedTheme = localStorage.getItem('theme');
    if (savedTheme === 'light' || savedTheme === 'dark') {
      return savedTheme as Theme;
    }
    // localStorageに設定がない場合はシステム設定を使用
    return getSystemTheme();
  });

  // テーマ変更時の処理
  useEffect(() => {
    const root = window.document.documentElement;
    
    // 現在のテーマクラスを削除
    root.classList.remove('light', 'dark');
    
    // 新しいテーマクラスを追加
    root.classList.add(theme);
    
    // localStorageに保存
    localStorage.setItem('theme', theme);
  }, [theme]);

  // システムテーマ変更の監視
  useEffect(() => {
    // localStorageに設定がある場合は監視しない（ユーザーが手動設定済み）
    const savedTheme = localStorage.getItem('theme');
    if (savedTheme) return;

    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
    const handleChange = (e: MediaQueryListEvent) => {
      setTheme(e.matches ? 'dark' : 'light');
    };

    // メディアクエリの変更を監視
    if (mediaQuery.addEventListener) {
      mediaQuery.addEventListener('change', handleChange);
      return () => mediaQuery.removeEventListener('change', handleChange);
    }
  }, []);

  // テーマ切り替え関数
  const toggleTheme = () => {
    setTheme(prevTheme => prevTheme === 'light' ? 'dark' : 'light');
  };

  return {
    theme,
    setTheme,
    toggleTheme,
  };
};