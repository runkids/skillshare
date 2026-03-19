import { NavLink, Outlet, useNavigate, useLocation } from 'react-router-dom';
import { useState, useCallback, useEffect } from 'react';
import {
  LayoutDashboard,
  Puzzle,
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
} from 'lucide-react';
import { radius } from '../design';
import { useAppContext } from '../context/AppContext';
import { useGlobalShortcuts } from '../hooks/useGlobalShortcuts';
import KeyboardShortcutsModal from './KeyboardShortcutsModal';
import ShortcutHUD from './ShortcutHUD';
import ThemePopover from './ThemePopover';
import { useTour } from './tour';

interface NavItem {
  to: string;
  icon: React.ElementType;
  label: string;
  hideInProject?: boolean;
}

interface NavGroup {
  label?: string;
  items: NavItem[];
}

const navGroups: NavGroup[] = [
  {
    items: [
      { to: '/', icon: LayoutDashboard, label: 'Dashboard' },
    ],
  },
  {
    label: 'MANAGE',
    items: [
      { to: '/skills', icon: Puzzle, label: 'Skills' },
      { to: '/extras', icon: FolderPlus, label: 'Extras' },
      { to: '/targets', icon: Target, label: 'Targets' },
      { to: '/search', icon: Search, label: 'Search' },
    ],
  },
  {
    label: 'OPERATIONS',
    items: [
      { to: '/sync', icon: RefreshCw, label: 'Sync' },
      { to: '/collect', icon: ArrowDownToLine, label: 'Collect' },
      { to: '/install', icon: Download, label: 'Install' },
      { to: '/update', icon: ArrowUpCircle, label: 'Update' },
      { to: '/skills/batch-uninstall', icon: Trash2, label: 'Uninstall' },
    ],
  },
  {
    label: 'SECURITY & MAINTENANCE',
    items: [
      { to: '/audit', icon: ShieldCheck, label: 'Audit' },
      { to: '/git', icon: GitBranch, label: 'Git Sync', hideInProject: true },
      { to: '/backup', icon: Archive, label: 'Backup', hideInProject: true },
      { to: '/trash', icon: Trash2, label: 'Trash' },
    ],
  },
  {
    label: 'SYSTEM',
    items: [
      { to: '/log', icon: ScrollText, label: 'Log' },
      { to: '/config', icon: Settings, label: 'Config' },
      { to: '/doctor', icon: Stethoscope, label: 'Health Check' },
    ],
  },
];

export default function Layout() {
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
        aria-label={mobileOpen ? 'Close menu' : 'Open menu'}
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
            skillshare
          </h1>
          <div className="flex items-center gap-2 mt-0.5">
            <p
              className="text-sm text-pencil-light"
                         >
              Web Dashboard
            </p>
            {isProjectMode && (
              <span
                className="text-xs px-1.5 py-0.5 bg-info-light text-blue border border-blue font-medium"
                style={{ borderRadius: radius.sm, fontFamily: 'var(--font-hand)' }}
              >
                Project
              </span>
            )}
          </div>
        </div>

        {/* Navigation */}
        <nav className="flex-1 min-h-0 overflow-y-auto py-2 px-2">
          {filteredGroups.map((group, groupIdx) => (
            <div key={groupIdx}>
              {group.label && (
                <div className="px-3 pt-4 pb-1 text-xs font-medium tracking-wider text-muted-dark uppercase">
                  {group.label}
                </div>
              )}
              {group.items.map(({ to, icon: Icon, label }) => (
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
                  {label}
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
            aria-label={toolsOpen ? 'Collapse tools' : 'Expand tools'}
          >
            Tools
            {toolsOpen
              ? <ChevronDown size={14} strokeWidth={2.5} />
              : <ChevronUp size={14} strokeWidth={2.5} />}
          </button>
          {toolsOpen && (
            <div className="px-2 pb-2 flex flex-col gap-0.5">
              <ThemePopover />
              <button
                onClick={startTour}
                className="flex items-center gap-3 px-3 py-1.5 text-sm text-pencil-light hover:text-pencil hover:bg-muted/20 transition-colors cursor-pointer"
                aria-label="Quick Tour"
              >
                <Compass size={16} strokeWidth={2.5} />
                Quick Tour
              </button>
              <button
                data-tour="shortcuts-btn"
                onClick={toggleShortcuts}
                className="flex items-center gap-3 px-3 py-1.5 text-sm text-pencil-light hover:text-pencil hover:bg-muted/20 transition-colors cursor-pointer"
                aria-label="Keyboard shortcuts"
                aria-keyshortcuts="?"
              >
                <Keyboard size={16} strokeWidth={2.5} />
                Shortcuts
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
    </div>
  );
}
