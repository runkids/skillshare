import { useEffect } from 'react';
import { useLocation } from 'react-router-dom';
import { postToApp } from '../lib/embeddingApp';

// Tell an embedding app (the desktop shell) which route is showing, so it can reload at the same place.
export function useRouteReport() {
  const { pathname, search } = useLocation();
  useEffect(() => {
    postToApp({ type: 'route-change', path: pathname + search });
  }, [pathname, search]);
}
