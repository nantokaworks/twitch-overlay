import React, { useEffect, useState } from 'react';
import { useClock } from '../hooks/useClock';
import { buildApiUrl } from '../utils/api';
import { CalendarIcon, ClockIcon, LocationIcon } from './ClockIcons';

interface ClockDisplayProps {
  showLocation?: boolean;
  showDate?: boolean;
  showTime?: boolean;
  showStats?: boolean;
}

const ClockDisplay: React.FC<ClockDisplayProps> = ({ 
  showLocation = true, 
  showDate = true, 
  showTime = true, 
  showStats = true 
}) => {
  const { year, month, date, day, hour, min, flashing } = useClock();
  const [weight, setWeight] = useState<string>('75.4');
  const [wallet, setWallet] = useState<string>('10,387');

  const today = `${year}.${month}.${date} ${day}`;
  
  // URLパラメータからアイコン表示設定を取得
  const params = new URLSearchParams(window.location.search);
  const showIcons = params.get('icons') !== 'false';

  useEffect(() => {
    const fetchSettings = async () => {
      try {
        const response = await fetch(buildApiUrl('/api/settings/v2'));
        if (response.ok) {
          const data = await response.json();
          const settings = data.settings;
          
          const weightValue = settings['CLOCK_WEIGHT']?.value || '75.4';
          const walletValue = settings['CLOCK_WALLET']?.value || '10387';
          
          setWeight(weightValue);
          setWallet(parseInt(walletValue).toLocaleString());
        }
      } catch (error) {
        console.error('Failed to fetch clock settings:', error);
      }
    };

    fetchSettings();
    
    // 30秒ごとに設定を更新
    const interval = setInterval(fetchSettings, 30000);
    return () => clearInterval(interval);
  }, []);

  return (
    <div className="clock-container text-2xl">
      {(showLocation || showDate || showTime) && (
        <div className="clock">
          {showLocation && (
            <>
              {showIcons ? <LocationIcon /> : <div className="icon-placeholder" />}
              <p className="locate">Hyogo,Japan</p>
            </>
          )}
          {showDate && (
            <>
              {showIcons ? <CalendarIcon /> : <div className="icon-placeholder" />}
              <p className="clock-date">{today}</p>
            </>
          )}
          {showTime && (
            <>
              {showIcons ? <ClockIcon /> : <div className="icon-placeholder" />}
              <p className="clock-hour">{hour}</p>
              <p className="clock-separator" style={{ opacity: flashing ? 1 : 0 }}>
                :
              </p>
              <p className="clock-min">{min}</p>
            </>
          )}
        </div>
      )}
      {showStats && (
        <div className="stats-container">
          <span className="stats-label">おもさ</span>
          <span className="stats-value">{weight}kg</span>
          <span className="stats-label">さいふ</span>
          <span className="stats-value">{wallet}えん</span>
        </div>
      )}
    </div>
  );
};

export default ClockDisplay;