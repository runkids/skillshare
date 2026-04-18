import { NavLink, Outlet, useNavigate, useLocation } from 'react-router-dom';
import { useState, useCallback, useEffect } from 'react';
import {
  LayoutDashboard,
  Layers,
  Target,
  FolderPlus,
  RefreshCw,
  ArrowDownToLine,
  Archive,
  Trash2,
  GitBranch,
  Search,
  Download,
  ArrowUpCircle,
  ShieldCheck,
  ScrollText,
  Settings,
  Menu,
  X,
  Keyboard,
  Compass,
  ChevronUp,
  ChevronDown,
  Stethoscope,
  BarChart3,
} from 'lucide-react';
import { radius } from '../design';
import { useAppContext } from '../context/AppContext';
import { useGlobalShortcuts } from '../hooks/useGlobalShortcuts';
import KeyboardShortcutsModal from './KeyboardShortcutsModal';
import ShortcutHUD from './ShortcutHUD';
import ThemePopover from './ThemePopover';
import LanguagePopover from './LanguagePopover';
import { useTour } from './tour';
import UpdateDialog from './UpdateDialog';
import { useT } from '../i18n';

interface NavItem {
  to: string;
  icon: React.ElementType;
  labelKey: string;
  hideInProject?: boolean;
}

interface NavGroup {
  labelKey?: string;
  items: NavItem[];
}

const navGroups: NavGroup[] = [
  {
    items: [
      { to: '/', icon: LayoutDashboard, labelKey: 'layout.nav.dashboard' },
    ],
  },
  {
    labelKey: 'layout.group.manage',
    items: [
      { to: '/resources', icon: Layers, labelKey: 'layout.nav.resources' },
      { to: '/extras', icon: FolderPlus, labelKey: 'layout.nav.extras' },
      { to: '/targets', icon: Target, labelKey: 'layout.nav.targets' },
      { to: '/search', icon: Search, labelKey: 'layout.nav.search' },
    ],
  },
  {
    labelKey: 'layout.group.operations',
    items: [
      { to: '/sync', icon: RefreshCw, labelKey: 'layout.nav.sync' },
      { to: '/collect', icon: ArrowDownToLine, labelKey: 'layout.nav.collect' },
      { to: '/install', icon: Download, labelKey: 'layout.nav.install' },
      { to: '/update', icon: ArrowUpCircle, labelKey: 'layout.nav.update' },
      { to: '/uninstall', icon: Trash2, labelKey: 'layout.nav.uninstall' },
    ],
  },
  {
    labelKey: 'layout.group.securityMaintenance',
    items: [
      { to: '/audit', icon: ShieldCheck, labelKey: 'layout.nav.audit' },
      { to: '/analyze', icon: BarChart3, labelKey: 'layout.nav.analyze' },
      { to: '/git', icon: GitBranch, labelKey: 'layout.nav.gitSync', hideInProject: true },
      { to: '/backup', icon: Archive, labelKey: 'layout.nav.backup', hideInProject: true },
      { to: '/trash', icon: Trash2, labelKey: 'layout.nav.trash' },
    ],
  },
  {
    labelKey: 'layout.group.system',
    items: [
      { to: '/log', icon: ScrollText, labelKey: 'layout.nav.log' },
      { to: '/config', icon: Settings, labelKey: 'layout.nav.config' },
      { to: '/doctor', icon: Stethoscope, labelKey: 'layout.nav.healthCheck' },
    ],
  },
];

export default function Layout() {
  const t = useT();
  const [mobileOpen, setMobileOpen] = useState(false);
  const [shortcutsOpen, setShortcutsOpen] = useState(false);
  const [toolsOpen, setToolsOpen] = useState(() => {
    try { return localStorage.getItem('ss-sidebar-tools') !== 'closed'; } catch { return true; }
  });
  useEffect(() => {
    try { localStorage.setItem('ss-sidebar-tools', toolsOpen ? 'open' : 'closed'); } catch {}
  }, [toolsOpen]);
  const { isProjectMode } = useAppContext();
  const { startTour } = useTour();

  const nav = useNavigate();
  const location = useLocation();
  const toggleShortcuts = useCallback(() => setShortcutsOpen((v) => !v), []);
  const handleSync = useCallback(() => nav('/sync'), [nav]);

  const { modifierHeld } = useGlobalShortcuts({
    onToggleHelp: toggleShortcuts,
    onSync: handleSync,
  });

  const filteredGroups = navGroups.map((group) => ({
    ...group,
    items: group.items.filter((item) => !(isProjectMode && item.hideInProject)),
  })).filter((group) => group.items.length > 0);

  return (
    <div className="min-h-screen">
      {/* Mobile menu button */}
      <button
        onClick={() => setMobileOpen(!mobileOpen)}
        className="fixed top-4 left-4 z-50 md:hidden w-10 h-10 flex items-center justify-center bg-surface border-2 border-pencil cursor-pointer"
        style={{ borderRadius: radius.sm }}
        aria-label={mobileOpen ? t('layout.mobile.closeMenu') : t('layout.mobile.openMenu')}
      >
        {mobileOpen ? <X size={20} strokeWidth={2.5} /> : <Menu size={20} strokeWidth={2.5} />}
      </button>

      {/* Mobile overlay */}
      {mobileOpen && (
        <div
          className="fixed inset-0 bg-pencil/30 z-30 md:hidden"
          onClick={() => setMobileOpen(false)}
        />
      )}

      {/* Sidebar — always fixed, never participates in document flow */}
      <aside
        className={`
          fixed top-0 left-0 z-40 h-screen w-60
          bg-paper-warm border-r border-muted
          flex flex-col
          transition-transform duration-200 md:translate-x-0
          ${mobileOpen ? 'translate-x-0' : '-translate-x-full'}
        `}
      >
        {/* Logo */}
        <div className="p-5 pb-4 border-b border-muted">
          <h1
            className="text-2xl font-bold text-pencil tracking-wide"

          >
            {t('app.name')}
          </h1>
          <div className="flex items-center gap-2 mt-0.5">
            <p
              className="text-sm text-pencil-light"
                         >
              {t('app.subtitle')}
            </p>
            {isProjectMode && (
              <span
                className="text-xs px-1.5 py-0.5 bg-info-light text-blue border border-blue font-medium"
                style={{ borderRadius: radius.sm, fontFamily: 'var(--font-hand)' }}
              >
                {t('app.project')}
              </span>
            )}
          </div>
        </div>

        {/* Navigation */}
        <nav className="flex-1 min-h-0 overflow-y-auto py-2 px-2">
          {filteredGroups.map((group, groupIdx) => (
            <div key={groupIdx}>
              {group.labelKey && (
                <div className="px-3 pt-4 pb-1 text-xs font-medium tracking-wider text-muted-dark uppercase">
                  {t(group.labelKey)}
                </div>
              )}
              {group.items.map(({ to, icon: Icon, labelKey }) => (
                <NavLink
                  key={to}
                  to={to}
                  end={to === '/'}
                  onClick={() => setMobileOpen(false)}
                  className={({ isActive }) =>
                    `flex items-center gap-3 px-3 py-2 mb-0.5 text-sm transition-colors duration-100 ${
                      isActive
                        ? 'bg-muted/40 text-pencil font-semibold'
                        : 'text-pencil-light hover:text-pencil hover:bg-muted/20'
                    }`
                  }

                >
                  <Icon size={16} strokeWidth={2.5} />
                  {t(labelKey)}
                </NavLink>
              ))}
            </div>
          ))}
        </nav>

        {/* Bottom bar — collapsible tools */}
        <div className="mt-auto border-t border-muted">
          <button
            onClick={() => setToolsOpen((v) => !v)}
            className="w-full flex items-center justify-between px-4 py-1.5 text-xs font-medium tracking-wider text-muted-dark uppercase hover:text-pencil-light transition-colors cursor-pointer"
            aria-expanded={toolsOpen}
            aria-label={toolsOpen ? t('layout.tools.collapse') : t('layout.tools.expand')}
          >
            {t('common.tools')}
            {toolsOpen
              ? <ChevronDown size={14} strokeWidth={2.5} />
              : <ChevronUp size={14} strokeWidth={2.5} />}
          </button>
          {toolsOpen && (
            <div className="px-2 pb-2 flex flex-col gap-0.5">
              <ThemePopover />
              <LanguagePopover />
              <button
                onClick={startTour}
                className="flex items-center gap-3 px-3 py-1.5 text-sm text-pencil-light hover:text-pencil hover:bg-muted/20 transition-colors cursor-pointer"
                aria-label={t('layout.tools.quickTour')}
              >
                <Compass size={16} strokeWidth={2.5} />
                {t('layout.tools.quickTour')}
              </button>
              <button
                data-tour="shortcuts-btn"
                onClick={toggleShortcuts}
                className="flex items-center gap-3 px-3 py-1.5 text-sm text-pencil-light hover:text-pencil hover:bg-muted/20 transition-colors cursor-pointer"
                aria-label={t('shortcuts.title')}
                aria-keyshortcuts="?"
              >
                <Keyboard size={16} strokeWidth={2.5} />
                {t('layout.tools.shortcuts')}
              </button>
            </div>
          )}
        </div>
      </aside>

      {/* Main content — offset by sidebar width on desktop */}
      <main className="md:ml-60 min-w-0 p-4 md:p-8 pt-16 md:pt-8">
        <div className="max-w-6xl mx-auto">
          <Outlet />
        </div>
      </main>

      {/* Keyboard shortcuts modal */}
      <KeyboardShortcutsModal open={shortcutsOpen} onClose={() => setShortcutsOpen(false)} />

      {/* Modifier-held HUD overlay — hidden on Config page where Cmd+S means Save */}
      <ShortcutHUD visible={modifierHeld && !location.pathname.startsWith('/config')} />

      <UpdateDialog />
    </div>
  );
}
