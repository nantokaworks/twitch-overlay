import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import FaxReceiver from './components/FaxReceiver';
import { SettingsPage } from './components/SettingsPage';
import { Toaster } from 'sonner';
import { MusicPlayerProvider } from './contexts/MusicPlayerContext';
import { SettingsProvider } from './contexts/SettingsContext';

function App() {
  return (
    <BrowserRouter>
      <SettingsProvider>
        <Routes>
          {/* オーバーレイページ（MusicPlayerProvider付き） */}
          <Route path="/" element={
            <MusicPlayerProvider>
              <FaxReceiver />
              <Toaster position="top-right" richColors expand={true} duration={3000} />
            </MusicPlayerProvider>
          } />
          
          {/* Settings画面 */}
          <Route path="/settings" element={<SettingsPage />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </SettingsProvider>
    </BrowserRouter>
  );
}

export default App;