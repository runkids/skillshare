import { lazy, Suspense } from 'react';
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { QueryClientProvider } from '@tanstack/react-query';
import { ReactQueryDevtools } from '@tanstack/react-query-devtools';
import { queryClient } from './lib/queryClient';
import { ToastProvider } from './components/Toast';
import { ThemeProvider } from './context/ThemeContext';
import { AppProvider } from './context/AppContext';
import { PageSkeleton } from './components/Skeleton';
import { ErrorBoundary } from './components/ErrorBoundary';
import Layout from './components/Layout';
import { TourProvider, TourOverlay, TourTooltip } from './components/tour';
import DashboardPage from './pages/DashboardPage';
import { BASE_PATH } from './lib/basePath';

const SkillsPage = lazy(() => import('./pages/SkillsPage'));
const SkillDetailPage = lazy(() => import('./pages/SkillDetailPage'));
const TargetsPage = lazy(() => import('./pages/TargetsPage'));
const ExtrasPage = lazy(() => import('./pages/ExtrasPage'));
const SyncPage = lazy(() => import('./pages/SyncPage'));
const CollectPage = lazy(() => import('./pages/CollectPage'));
const BackupPage = lazy(() => import('./pages/BackupPage'));
const GitSyncPage = lazy(() => import('./pages/GitSyncPage'));
const SearchPage = lazy(() => import('./pages/SearchPage'));
const InstallPage = lazy(() => import('./pages/InstallPage'));
const UpdatePage = lazy(() => import('./pages/UpdatePage'));
const TrashPage = lazy(() => import('./pages/TrashPage'));
const AuditPage = lazy(() => import('./pages/AuditPage'));
const AuditRulesPage = lazy(() => import('./pages/AuditRulesPage'));
const LogPage = lazy(() => import('./pages/LogPage'));
const ConfigPage = lazy(() => import('./pages/ConfigPage'));
const FilterStudioPage = lazy(() => import('./pages/FilterStudioPage'));
const NewSkillPage = lazy(() => import('./pages/NewSkillPage'));
const BatchUninstallPage = lazy(() => import('./pages/BatchUninstallPage'));
const DoctorPage = lazy(() => import('./pages/DoctorPage'));

function Lazy({ children }: { children: React.ReactNode }) {
  return <Suspense fallback={<PageSkeleton />}>{children}</Suspense>;
}

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>
      <ToastProvider>
        <AppProvider>
          <BrowserRouter basename={BASE_PATH}>
            <ErrorBoundary>
            <TourProvider>
            <TourOverlay />
            <TourTooltip />
            <Routes>
              <Route element={<Layout />}>
                <Route index element={<DashboardPage />} />
                <Route path="skills" element={<Lazy><SkillsPage /></Lazy>} />
                <Route path="skills/new" element={<Lazy><NewSkillPage /></Lazy>} />
                <Route path="skills/batch-uninstall" element={<Lazy><BatchUninstallPage /></Lazy>} />
                <Route path="skills/:name" element={<Lazy><SkillDetailPage /></Lazy>} />
                <Route path="targets" element={<Lazy><TargetsPage /></Lazy>} />
                <Route path="targets/:name/filters" element={<Lazy><FilterStudioPage /></Lazy>} />
                <Route path="extras" element={<Lazy><ExtrasPage /></Lazy>} />
                <Route path="sync" element={<Lazy><SyncPage /></Lazy>} />
                <Route path="collect" element={<Lazy><CollectPage /></Lazy>} />
                <Route path="backup" element={<Lazy><BackupPage /></Lazy>} />
                <Route path="trash" element={<Lazy><TrashPage /></Lazy>} />
                <Route path="git" element={<Lazy><GitSyncPage /></Lazy>} />
                <Route path="search" element={<Lazy><SearchPage /></Lazy>} />
                <Route path="install" element={<Lazy><InstallPage /></Lazy>} />
                <Route path="update" element={<Lazy><UpdatePage /></Lazy>} />
                <Route path="audit" element={<Lazy><AuditPage /></Lazy>} />
                <Route path="audit/rules" element={<Lazy><AuditRulesPage /></Lazy>} />
                <Route path="log" element={<Lazy><LogPage /></Lazy>} />
                <Route path="config" element={<Lazy><ConfigPage /></Lazy>} />
                <Route path="doctor" element={<Lazy><DoctorPage /></Lazy>} />
              </Route>
            </Routes>
            </TourProvider>
            </ErrorBoundary>
          </BrowserRouter>
        </AppProvider>
      </ToastProvider>
      </ThemeProvider>
      <ReactQueryDevtools initialIsOpen={false} />
    </QueryClientProvider>
  );
}
