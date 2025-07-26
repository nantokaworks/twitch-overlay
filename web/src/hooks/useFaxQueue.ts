import { useState, useEffect, useCallback } from 'react';
import type { FaxData, UseFaxQueueReturn } from '../types';

export const useFaxQueue = (): UseFaxQueueReturn => {
  const [queue, setQueue] = useState<FaxData[]>([]);
  const [currentFax, setCurrentFax] = useState<FaxData | null>(null);
  const [isDisplaying, setIsDisplaying] = useState<boolean>(false);

  // キューにFAXを追加
  const addToQueue = useCallback((faxData: FaxData) => {
    setQueue((prev) => [...prev, faxData]);
  }, []);

  // 次のFAXを表示
  const displayNext = useCallback(() => {
    setQueue((prev) => {
      if (prev.length > 0 && !isDisplaying) {
        setCurrentFax(prev[0]);
        setIsDisplaying(true);
        return prev.slice(1);
      }
      return prev;
    });
  }, [isDisplaying]);

  // 表示完了を通知
  const onDisplayComplete = useCallback(() => {
    setCurrentFax(null);
    setIsDisplaying(false);
  }, []);

  // キューが空でない場合、次のFAXを表示
  useEffect(() => {
    if (!isDisplaying && queue.length > 0) {
      displayNext();
    }
  }, [queue, isDisplaying, displayNext]);

  return {
    queue,
    currentFax,
    isDisplaying,
    addToQueue,
    onDisplayComplete,
  };
};