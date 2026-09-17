import { Link, Outlet, useNavigate, useLocation } from 'react-router-dom';
import { useState, useCallback } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  LayoutDashboard,
  RefreshCw,
  GitBranch,
  Puzzle,
  Bot,
  FolderPlus,
  Plug,
  Target,
  ShieldCheck,
  Settings,
  Keyboard,
  Compass,
} from 'lucide-react';
import { api, type AuditAllResponse } from '../api/client';
import { mcpApi } from '../api/mcp';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { pendingCount } from './sync/syncView';
import { useAppContext } from '../context/AppContext';
import { useGlobalShortcuts } from '../hooks/useGlobalShortcuts';
import KeyboardShortcutsModal from './KeyboardShortcutsModal';
import ShortcutHUD from './ShortcutHUD';
import ScrollToTop from './ScrollToTop';
import ThemePopover from './ThemePopover';
import LanguagePopover from './LanguagePopover';
import { useTour } from './tour';
import UpdateDialog from './UpdateDialog';
import { useT } from '../i18n';
import { shortenHome } from '../lib/paths';

interface NavItem {
  to: string;
  icon: React.ElementType;
  labelKey: string;
  /** Legacy routes that belong to this item until their pages move under it. */
  also?: string[];
  hideInProject?: boolean;
}

const navGroups: { labelKey?: string; items: NavItem[] }[] = [
  {
    items: [
      { to: '/', icon: LayoutDashboard, labelKey: 'layout.nav.dashboard' },
      { to: '/sync', icon: RefreshCw, labelKey: 'layout.nav.sync' },
      { to: '/git', icon: GitBranch, labelKey: 'layout.nav.gitSync', hideInProject: true },
    ],
  },
  {
    labelKey: 'layout.group.library',
    items: [
      { to: '/skills', icon: Puzzle, labelKey: 'layout.nav.skills' },
      { to: '/agents', icon: Bot, labelKey: 'layout.nav.agents' },
      { to: '/extras', icon: FolderPlus, labelKey: 'layout.nav.extras' },
      { to: '/mcp', icon: Plug, labelKey: 'mcp.title' },
    ],
  },
  {
    labelKey: 'layout.group.destinations',
    items: [{ to: '/targets', icon: Target, labelKey: 'layout.nav.targets', also: ['/collect'] }],
  },
  {
    labelKey: 'layout.group.maintain',
    items: [
      { to: '/audit', icon: ShieldCheck, labelKey: 'layout.nav.audit' },
      { to: '/settings', icon: Settings, labelKey: 'layout.nav.settings', also: ['/config', '/backup', '/log', '/doctor'] },
    ],
  },
];

function isActive(item: NavItem, pathname: string): boolean {
  if (item.to === '/') return pathname === '/';
  return [item.to, ...(item.also ?? [])].some((p) => pathname === p || pathname.startsWith(`${p}/`));
}

export default function Layout() {
  const t = useT();
  const [shortcutsOpen, setShortcutsOpen] = useState(false);
  const { isProjectMode, projectRoot } = useAppContext();
  const { startTour } = useTour();
  const nav = useNavigate();
  const location = useLocation();
  const toggleShortcuts = useCallback(() => setShortcutsOpen((v) => !v), []);
  const handleSync = useCallback(() => nav('/sync'), [nav]);
  const { modifierHeld } = useGlobalShortcuts({ onToggleHelp: toggleShortcuts, onSync: handleSync });

  const { data: overview } = useQuery({
    queryKey: queryKeys.overview,
    queryFn: () => api.getOverview(),
    staleTime: staleTimes.overview,
  });
  const version = overview?.version;
  const home = isProjectMode ? projectRoot : overview?.configDir;
  const counts = useNavCounts(isProjectMode);

  return (
    <div className="min-h-screen">
      <aside className="ss-side fixed top-0 left-0 z-30 h-screen">
        <div className="ss-wm">
          <b>{t('app.name')}</b>
          <svg className="ss-squig ss-only-playful" width="112" height="7" viewBox="0 0 112 7" aria-hidden="true">
            <path d="M1 4 Q 8 0 15 4 T 29 4 T 43 4 T 57 4 T 71 4 T 85 4 T 99 4 T 111 4" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" />
          </svg>
          <span className="truncate" title={home}>{t(isProjectMode ? 'app.project' : 'app.global')}{home && ` · ${shortenHome(home)}`}</span>
        </div>

        <nav className="flex-1 min-h-0 overflow-y-auto -mx-1 px-1">
          {navGroups.map((group, i) => {
            const items = group.items.filter((item) => !(isProjectMode && item.hideInProject));
            return (
              <div key={i}>
                {group.labelKey && <div className="ss-nvg">{t(group.labelKey)}</div>}
                {items.map((item) => {
                  const Icon = item.icon;
                  const active = isActive(item, location.pathname);
                  return (
                    <Link key={item.to} to={item.to} className={`ss-nv ${active ? 'on' : ''}`} aria-current={active ? 'page' : undefined}>
                      <Icon size={16} />
                      <span>{t(item.labelKey)}</span>
                      {counts[item.to] > 0 && <span className="n">{counts[item.to]}</span>}
                    </Link>
                  );
                })}
              </div>
            );
          })}
        </nav>

        <div className="ss-sidefoot">
          <span className="flex items-center gap-0.5">
            <ThemePopover />
            <LanguagePopover />
            <button type="button" className="ss-ib" data-tour="shortcuts-btn" onClick={toggleShortcuts} aria-label={t('shortcuts.title')} title={t('shortcuts.title')} aria-keyshortcuts="?">
              <Keyboard size={16} />
            </button>
            <button type="button" className="ss-ib" onClick={startTour} aria-label={t('layout.tools.quickTour')} title={t('layout.tools.quickTour')}>
              <Compass size={16} />
            </button>
          </span>
          {version && <span className="font-mono text-[11px] text-ink-3">{/^\d/.test(version) ? `v${version}` : version}</span>}
        </div>
      </aside>

      <main className="ml-[232px] min-w-0 px-12 py-10">
        <div className="max-w-[1080px] mx-auto">
          <Outlet />
        </div>
      </main>

      <KeyboardShortcutsModal open={shortcutsOpen} onClose={() => setShortcutsOpen(false)} />
      {/* Hidden on Config where Cmd+S means Save */}
      <ScrollToTop />
      <ShortcutHUD visible={modifierHeld && !location.pathname.startsWith('/config')} />
      <UpdateDialog />
    </div>
  );
}

// ponytail: polling keeps the badges (and the pages sharing these queries) current after CLI or
// file changes; server push would avoid the requests if they ever get heavy.
const live = { refetchInterval: 15_000, refetchOnWindowFocus: true } as const;

/** Sidebar badges: pending sync changes, git work to share, and skills a cached audit scan blocks. */
function useNavCounts(isProjectMode: boolean): Record<string, number> {
  const targets = useQuery({ queryKey: queryKeys.targets.all, queryFn: () => api.listTargets(), staleTime: staleTimes.targets, ...live });
  const diff = useQuery({ queryKey: queryKeys.diff(), queryFn: () => api.diff(), staleTime: staleTimes.diff, ...live });
  const extras = useQuery({ queryKey: queryKeys.extrasDiff(), queryFn: () => api.diffExtras(), staleTime: staleTimes.extras, ...live });
  const mcp = useQuery({ queryKey: queryKeys.mcp, queryFn: () => mcpApi.list(), staleTime: staleTimes.extras, ...live });
  const git = useQuery({ queryKey: queryKeys.gitStatus, queryFn: () => api.gitStatus(), staleTime: staleTimes.gitStatus, enabled: !isProjectMode, ...live });
  // Scans are expensive, so the badge only reads one the Audit page already ran.
  const auditSkills = useQuery<AuditAllResponse>({ queryKey: queryKeys.audit.all('skills'), queryFn: () => api.auditAll('skills'), enabled: false });
  const auditAgents = useQuery<AuditAllResponse>({ queryKey: queryKeys.audit.all('agents'), queryFn: () => api.auditAll('agents'), enabled: false });

  const status = git.data;
  return {
    '/sync': diff.data ? pendingCount(diff.data.diffs, targets.data?.targets ?? [], extras.data?.extras ?? [], mcp.data?.plan) : 0,
    '/git': !status?.isRepo ? 0 : status.isDirty ? status.files.length : status.hasRemote ? status.ahead : 0,
    '/audit': (auditSkills.data?.summary.failed ?? 0) + (auditAgents.data?.summary.failed ?? 0),
  };
}
