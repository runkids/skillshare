import { api } from '../api/client';

/** Preserve the UI downloaded by upgrade and reload only after the server responds. */
export async function upgradeAndReconnect(
  onRestart: () => void,
  reload = () => window.location.reload(),
) {
  if ((await api.upgradeApp()).devMode) return 'dev';
  onRestart();
  await api.restartApp({ clearCache: false });
  for (let attempt = 0; attempt < 40; attempt++) {
    await new Promise((resolve) => setTimeout(resolve, 500));
    try {
      await api.health();
    } catch {
      continue;
    }
    reload();
    return 'reloaded';
  }
  return 'manual';
}
