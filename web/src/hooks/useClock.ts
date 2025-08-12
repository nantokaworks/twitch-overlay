import { useEffect, useState } from 'react';

interface TimeState {
  year: string;
  month: string;
  date: string;
  day: string;
  hour: string;
  min: string;
  second: number;
  flashing: boolean;
}

export const useClock = (): TimeState => {
  const [time, setTime] = useState<TimeState>({
    year: '',
    month: '',
    date: '',
    day: '',
    hour: '',
    min: '',
    second: 0,
    flashing: false
  });

  useEffect(() => {
    const updateClock = (): void => {
      const d = new Date();

      const year = d.getFullYear();
      let month = d.getMonth() + 1;
      let date = d.getDate();
      const dayNum = d.getDay();
      const weekday = ["SUN", "MoN", "TUE", "WED", "THU", "FRI", "SAT"];
      const day = weekday[dayNum];
      let hour = d.getHours();
      let min = d.getMinutes();
      const second = d.getSeconds();

      const monthStr = month < 10 ? "0" + month : month.toString();
      const dateStr = date < 10 ? "0" + date : date.toString();
      const hourStr = hour < 10 ? "0" + hour : hour.toString();
      const minStr = min < 10 ? "0" + min : min.toString();

      const flashing = second % 2 === 0;

      setTime({
        year: year.toString(),
        month: monthStr,
        date: dateStr,
        day,
        hour: hourStr,
        min: minStr,
        second,
        flashing
      });
    };

    // 初回実行
    updateClock();

    // 1秒ごとに更新
    const interval = setInterval(updateClock, 1000);

    // クリーンアップ
    return () => clearInterval(interval);
  }, []);

  return time;
};