import { createContext, useContext } from 'react';
import type { ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import { api } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';

interface AppContextValue {
  isProjectMode: boolean;
  projectRoot?: string;
}

const AppContext = createContext<AppContextValue>({ isProjectMode: false });

export function useAppContext() {
  return useContext(AppContext);
}

export function AppProvider({ children }: { children: ReactNode }) {
  const { data } = useQuery({
    queryKey: queryKeys.overview,
    queryFn: () => api.getOverview(),
    staleTime: staleTimes.overview,
  });

  const value: AppContextValue = {
    isProjectMode: data?.isProjectMode ?? false,
    projectRoot: data?.projectRoot,
  };

  return <AppContext.Provider value={value}>{children}</AppContext.Provider>;
}
