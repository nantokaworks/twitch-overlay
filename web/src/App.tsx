import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import FaxReceiver from './components/FaxReceiver';
import { SettingsPage } from './components/SettingsPage';
import { Toaster } from 'sonner';
import { MusicPlayerProvider } from './contexts/MusicPlayerContext';

function App() {
  return (
    <BrowserRouter>
      <Routes>
        {/* オーバーレイページ（MusicPlayerProvider付き） */}
        <Route path="/" element={
          <MusicPlayerProvider>
            <FaxReceiver imageType="mono" />
            <Toaster position="top-right" richColors expand={true} duration={3000} />
          </MusicPlayerProvider>
        } />
        <Route path="/mono" element={
          <MusicPlayerProvider>
            <FaxReceiver imageType="mono" />
            <Toaster position="top-right" richColors expand={true} duration={3000} />
          </MusicPlayerProvider>
        } />
        <Route path="/color" element={
          <MusicPlayerProvider>
            <FaxReceiver imageType="color" />
            <Toaster position="top-right" richColors expand={true} duration={3000} />
          </MusicPlayerProvider>
        } />
        
        {/* Settings画面（MusicPlayerProvider無し） */}
        <Route path="/settings" element={<SettingsPage />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  );
}

export default App;