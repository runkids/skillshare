import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Outlet, useBlocker } from 'react-router-dom';
import App from './App';

vi.mock('@tanstack/react-query-devtools', () => ({
  ReactQueryDevtools: () => null,
}));

vi.mock('./context/ThemeContext', () => ({
  ThemeProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

vi.mock('./context/AppContext', () => ({
  AppProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  useAppContext: () => ({ isProjectMode: false }),
}));

vi.mock('./components/Toast', async () => {
  const actual = await vi.importActual<typeof import('./components/Toast')>('./components/Toast');
  return {
    ...actual,
    ToastProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  };
});

vi.mock('./components/Layout', () => ({
  default: function MockLayout() {
    return <Outlet />;
  },
}));

vi.mock('./pages/DashboardPage', () => ({
  default: () => <div>dashboard page</div>,
}));

vi.mock('./pages/ResourcesPage', () => ({
  default: () => <div>resources page</div>,
}));

vi.mock('./pages/ResourceDetailPage', () => ({
  default: function MockResourceDetailPage() {
    useBlocker(false);
    return <div>resource detail page</div>;
  },
}));

vi.mock('./pages/TargetsPage', () => ({ default: () => <div>targets page</div> }));
vi.mock('./pages/ExtrasPage', () => ({ default: () => <div>extras page</div> }));
vi.mock('./pages/SyncPage', () => ({ default: () => <div>sync page</div> }));
vi.mock('./pages/CollectPage', () => ({ default: () => <div>collect page</div> }));
vi.mock('./pages/BackupPage', () => ({ default: () => <div>backup page</div> }));
vi.mock('./pages/GitSyncPage', () => ({ default: () => <div>git page</div> }));
vi.mock('./pages/SearchPage', () => ({ default: () => <div>search page</div> }));
vi.mock('./pages/InstallPage', () => ({ default: () => <div>install page</div> }));
vi.mock('./pages/UpdatePage', () => ({ default: () => <div>update page</div> }));
vi.mock('./pages/TrashPage', () => ({ default: () => <div>trash page</div> }));
vi.mock('./pages/AuditPage', () => ({ default: () => <div>audit page</div> }));
vi.mock('./pages/AuditRulesPage', () => ({ default: () => <div>audit rules page</div> }));
vi.mock('./pages/LogPage', () => ({ default: () => <div>log page</div> }));
vi.mock('./pages/ConfigPage', () => ({ default: () => <div>config page</div> }));
vi.mock('./pages/FilterStudioPage', () => ({ default: () => <div>filter studio page</div> }));
vi.mock('./pages/NewSkillPage', () => ({ default: () => <div>new skill page</div> }));
vi.mock('./pages/BatchUninstallPage', () => ({ default: () => <div>batch uninstall page</div> }));
vi.mock('./pages/DoctorPage', () => ({ default: () => <div>doctor page</div> }));
vi.mock('./pages/AnalyzePage', () => ({ default: () => <div>analyze page</div> }));

describe('App', () => {
  beforeEach(() => {
    window.history.replaceState({}, '', '/resources/test-skill');
    vi.spyOn(console, 'error').mockImplementation(() => {});
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders the resource detail route without a router compatibility error', async () => {
    render(<App />);

    expect(await screen.findByText('resource detail page')).toBeInTheDocument();
    expect(screen.queryByText(/useBlocker must be used within a data router/i)).not.toBeInTheDocument();
    expect(screen.queryByText('Something went wrong')).not.toBeInTheDocument();
  });
});
