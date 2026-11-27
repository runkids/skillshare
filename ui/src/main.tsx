import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import './index.css';
import App from './App.tsx';
import { BASE_PATH } from './lib/basePath';

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);

if ('serviceWorker' in navigator) {
  const swPath = `${BASE_PATH.replace(/\/$/, '')}/sw.js`;
  window.addEventListener('load', () => {
    navigator.serviceWorker.register(swPath).catch(() => undefined);
  });
}
