import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import FaxReceiver from './components/FaxReceiver';
import { Settings } from './components/Settings';

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<FaxReceiver imageType="mono" />} />
        <Route path="/mono" element={<FaxReceiver imageType="mono" />} />
        <Route path="/color" element={<FaxReceiver imageType="color" />} />
        <Route path="/settings" element={<Settings />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  );
}

export default App;