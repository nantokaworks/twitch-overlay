import React, { useState, useEffect } from 'react';
import { CalendarIcon, ClockIcon, LocationIcon } from './ClockIcons';
import { useClock } from '../hooks/useClock';
import { buildApiUrl } from '../utils/api';

const ClockDisplay: React.FC = () => {
  const { year, month, date, day, hour, min, flashing } = useClock();
  const [weight, setWeight] = useState<string>('75.4');
  const [wallet, setWallet] = useState<string>('10,387');

  const today = `${year}.${month}.${date} ${day}`;

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
      <div className="clock">
        <LocationIcon />
        <p className="locate">Hyogo,Japan</p>
        <CalendarIcon />
        <p className="clock-date">{today}</p>
        <ClockIcon />
        <p className="clock-hour">{hour}</p>
        <p className="clock-separator" style={{ opacity: flashing ? 1 : 0 }}>
          :
        </p>
        <p className="clock-min">{min}</p>
      </div>
      <div className="stats-container">
        <span className="stats-label">おもさ</span>
        <span className="stats-value">{weight}kg</span>
        <span className="stats-label">さいふ</span>
        <span className="stats-value">{wallet}えん</span>
      </div>
    </div>
  );
};

export default ClockDisplay;