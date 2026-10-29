import { useEffect } from 'react';
import { useLocation } from 'react-router-dom';

// Tell an embedding app (the desktop shell) which route is showing, so it can reload at the same place.
export function useRouteReport() {
  const { pathname, search } = useLocation();
  useEffect(() => {
    if (window.parent === window) return;
    window.parent.postMessage({ type: 'route-change', path: pathname + search }, '*');
  }, [pathname, search]);
}
