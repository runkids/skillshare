import { lazy, Suspense } from 'react';
import { createBrowserRouter, RouterProvider, Routes, Route, Navigate, useRouteError, useParams, useSearchParams } from 'react-router-dom';
import { QueryClientProvider } from '@tanstack/react-query';
import { ReactQueryDevtools } from '@tanstack/react-query-devtools';
import { queryClient } from './lib/queryClient';
import { ToastProvider } from './components/Toast';
import { ThemeProvider } from './context/ThemeContext';
import { AppProvider } from './context/AppContext';
import { I18nProvider } from './i18n';
import { PageSkeleton } from './components/Skeleton';
import { ErrorBoundary } from './components/ErrorBoundary';
import Layout from './components/Layout';
import { TourProvider, TourOverlay, TourTooltip } from './components/tour';
import TruncateTip from './components/TruncateTip';
import DashboardPage from './pages/DashboardPage';
import { BASE_PATH } from './lib/basePath';

const HubPage = lazy(() => import('./pages/HubPage'));
const ResourcesPage = lazy(() => import('./pages/ResourcesPage'));
const ResourceDetailPage = lazy(() => import('./pages/ResourceDetailPage'));
const TargetsPage = lazy(() => import('./pages/TargetsPage'));
const ExtrasPage = lazy(() => import('./pages/ExtrasPage'));
const PluginsPage = lazy(() => import('./pages/PluginsPage'));
const MCPPage = lazy(() => import('./pages/MCPPage'));
const SyncPage = lazy(() => import('./pages/SyncPage'));
const BackupPage = lazy(() => import('./pages/BackupPage'));
const GitSyncPage = lazy(() => import('./pages/GitSyncPage'));
const AuditPage = lazy(() => import('./pages/AuditPage'));
const AuditRulesPage = lazy(() => import('./pages/AuditRulesPage'));
const LogPage = lazy(() => import('./pages/LogPage'));
const ConfigPage = lazy(() => import('./pages/ConfigPage'));
const SettingsPage = lazy(() => import('./pages/SettingsPage'));
const TargetDetailPage = lazy(() => import('./pages/TargetDetailPage'));
const NewSkillPage = lazy(() => import('./pages/NewSkillPage'));
const DoctorPage = lazy(() => import('./pages/DoctorPage'));

function Lazy({ children }: { children: React.ReactNode }) {
  return <Suspense fallback={<PageSkeleton />}>{children}</Suspense>;
}

/** Old /resources URLs (bookmarks) → /skills or /agents. */
function LegacyResourceRedirect() {
  const { name } = useParams();
  const [params] = useSearchParams();
  const base = params.get('kind') === 'agent' || params.get('tab') === 'agents' ? '/agents' : '/skills';
  return <Navigate to={name ? `${base}/${encodeURIComponent(name)}` : base} replace />;
}

/** Filter Studio and the Collect page became tabs and a dialog on the target page. */
function LegacyTargetRedirect() {
  const { name } = useParams();
  const [params] = useSearchParams();
  const target = name ?? params.get('target');
  if (!target) return <Navigate to="/targets" replace />;
  const agents = params.get('kind') === 'agent' || params.get('scope') === 'agent';
  return <Navigate to={`/targets/${encodeURIComponent(target)}${agents ? '?tab=agents' : ''}`} replace />;
}

function AppRoutes() {
  return (
    <ErrorBoundary>
      <TourProvider>
        <TourOverlay />
        <TourTooltip />
        <TruncateTip />
        <Routes>
          <Route element={<Layout />}>
            <Route index element={<DashboardPage />} />
            <Route path="skills" element={<Lazy><ResourcesPage key="skill" kind="skill" /></Lazy>} />
            <Route path="hubs" element={<Lazy><HubPage /></Lazy>} />
            <Route path="skills/new" element={<Lazy><NewSkillPage /></Lazy>} />
            <Route path="skills/:name" element={<Lazy><ResourceDetailPage /></Lazy>} />
            <Route path="agents" element={<Lazy><ResourcesPage key="agent" kind="agent" /></Lazy>} />
            <Route path="agents/:name" element={<Lazy><ResourceDetailPage /></Lazy>} />
            <Route path="resources" element={<LegacyResourceRedirect />} />
            <Route path="resources/new" element={<Navigate to="/skills/new" replace />} />
            <Route path="resources/:name" element={<LegacyResourceRedirect />} />
            <Route path="uninstall" element={<Navigate to="/skills" replace />} />
            <Route path="targets" element={<Lazy><TargetsPage /></Lazy>} />
            <Route path="targets/:name" element={<Lazy><TargetDetailPage /></Lazy>} />
            <Route path="targets/:name/filters" element={<LegacyTargetRedirect />} />
            <Route path="extras" element={<Lazy><ExtrasPage /></Lazy>} />
            <Route path="plugins" element={<Lazy><PluginsPage /></Lazy>} />
            <Route path="mcp" element={<Lazy><MCPPage /></Lazy>} />
            <Route path="sync" element={<Lazy><SyncPage /></Lazy>} />
            <Route path="collect" element={<LegacyTargetRedirect />} />
            <Route path="backup" element={<Lazy><BackupPage /></Lazy>} />
            <Route path="trash" element={<Navigate to="/skills?tab=trash" replace />} />
            <Route path="git" element={<Lazy><GitSyncPage /></Lazy>} />
            <Route path="search" element={<Navigate to="/skills?install=search" replace />} />
            <Route path="install" element={<Navigate to="/skills?install=url" replace />} />
            <Route path="update" element={<Navigate to="/skills?tab=updates" replace />} />
            <Route path="audit" element={<Lazy><AuditPage /></Lazy>} />
            <Route path="audit/rules" element={<Lazy><AuditRulesPage /></Lazy>} />
            <Route path="analyze" element={<Navigate to="/skills?tab=analyze" replace />} />
            <Route path="log" element={<Lazy><LogPage /></Lazy>} />
            <Route path="settings" element={<Lazy><SettingsPage /></Lazy>} />
            <Route path="config" element={<Lazy><ConfigPage /></Lazy>} />
            <Route path="doctor" element={<Lazy><DoctorPage /></Lazy>} />
          </Route>
        </Routes>
      </TourProvider>
    </ErrorBoundary>
  );
}

// Forward router failures to the same fallback as render failures.
function RouteError(): never { throw useRouteError(); }

const router = createBrowserRouter([{ path: "*", element: <AppRoutes />, errorElement: <ErrorBoundary><RouteError /></ErrorBoundary> }], { basename: BASE_PATH });

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>
        <I18nProvider>
          <ToastProvider>
            <AppProvider>
              <RouterProvider router={router} />
            </AppProvider>
          </ToastProvider>
        </I18nProvider>
      </ThemeProvider>
      {/* Top right, so the dev-only toggle does not cover the scroll-to-top button */}
      <ReactQueryDevtools initialIsOpen={false} buttonPosition="top-right" />
    </QueryClientProvider>
  );
}
