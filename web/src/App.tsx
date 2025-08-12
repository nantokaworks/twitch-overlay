import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import FaxReceiver from './components/FaxReceiver';
import { SettingsPage } from './components/SettingsPage';
import { Toaster } from 'sonner';

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<FaxReceiver imageType="mono" />} />
        <Route path="/mono" element={<FaxReceiver imageType="mono" />} />
        <Route path="/color" element={<FaxReceiver imageType="color" />} />
        <Route path="/settings" element={<SettingsPage />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
      <Toaster 
        position="top-right"
        richColors
        expand={true}
        duration={3000}
      />
    </BrowserRouter>
  );
}

export default App;