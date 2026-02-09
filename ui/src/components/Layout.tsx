import { NavLink, Outlet } from 'react-router-dom';
import { useState, useMemo } from 'react';
import {
  LayoutDashboard,
  Puzzle,
  Target,
  RefreshCw,
  ArrowDownToLine,
  Archive,
  Trash2,
  GitBranch,
  Search,
  Download,
  ShieldCheck,
  ScrollText,
  Settings,
  Menu,
  X,
} from 'lucide-react';
import { wobbly, shadows } from '../design';
import { useAppContext } from '../context/AppContext';

const allNavItems = [
  { to: '/', icon: LayoutDashboard, label: 'Dashboard' },
  { to: '/skills', icon: Puzzle, label: 'Skills' },
  { to: '/audit', icon: ShieldCheck, label: 'Audit' },
  { to: '/log', icon: ScrollText, label: 'Log' },
  { to: '/install', icon: Download, label: 'Install' },
  { to: '/targets', icon: Target, label: 'Targets' },
  { to: '/sync', icon: RefreshCw, label: 'Sync' },
  { to: '/collect', icon: ArrowDownToLine, label: 'Collect' },
  { to: '/backup', icon: Archive, label: 'Backup' },
  { to: '/trash', icon: Trash2, label: 'Trash' },
  { to: '/git', icon: GitBranch, label: 'Git Sync' },
  { to: '/search', icon: Search, label: 'Search' },
  { to: '/config', icon: Settings, label: 'Config' },
];

export default function Layout() {
  const [mobileOpen, setMobileOpen] = useState(false);
  const { isProjectMode } = useAppContext();

  const navItems = useMemo(() => {
    if (isProjectMode) {
      return allNavItems.filter((item) => item.to !== '/git' && item.to !== '/backup');
    }
    return allNavItems;
  }, [isProjectMode]);

  return (
    <div className="flex min-h-screen">
      {/* Mobile menu button */}
      <button
        onClick={() => setMobileOpen(!mobileOpen)}
        className="fixed top-4 left-4 z-50 md:hidden w-10 h-10 flex items-center justify-center bg-white border-2 border-pencil"
        style={{
          borderRadius: wobbly.sm,
          boxShadow: shadows.sm,
        }}
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

      {/* Sidebar */}
      <aside
        className={`
          fixed md:sticky top-0 left-0 z-40 h-screen w-60 shrink-0
          bg-paper-warm border-r-2 border-dashed border-pencil-light
          flex flex-col
          transition-transform duration-200 md:translate-x-0
          ${mobileOpen ? 'translate-x-0' : '-translate-x-full'}
        `}
      >
        {/* Logo */}
        <div className="p-5 pb-4 border-b-2 border-dashed border-muted-dark">
          <h1
            className="text-2xl font-bold text-pencil tracking-wide"
            style={{ fontFamily: 'var(--font-heading)' }}
          >
            skillshare
          </h1>
          <div className="flex items-center gap-2 mt-0.5">
            <p
              className="text-sm text-pencil-light"
              style={{ fontFamily: 'var(--font-hand)' }}
            >
              Web Dashboard
            </p>
            {isProjectMode && (
              <span
                className="text-xs px-1.5 py-0.5 bg-info-light text-blue border border-blue font-medium"
                style={{ borderRadius: wobbly.sm, fontFamily: 'var(--font-hand)' }}
              >
                Project
              </span>
            )}
          </div>
        </div>

        {/* Navigation */}
        <nav className="flex-1 py-3 px-2">
          {navItems.map(({ to, icon: Icon, label }) => (
            <NavLink
              key={to}
              to={to}
              end={to === '/'}
              onClick={() => setMobileOpen(false)}
              className={({ isActive }) =>
                `flex items-center gap-3 px-4 py-2.5 mb-1 text-base transition-all duration-100 ${
                  isActive
                    ? 'bg-white border-2 border-pencil text-pencil font-medium'
                    : 'text-pencil-light hover:text-pencil hover:bg-white/60 border-2 border-transparent'
                }`
              }
              style={({ isActive }) => ({
                borderRadius: wobbly.sm,
                boxShadow: isActive ? shadows.sm : 'none',
                fontFamily: 'var(--font-hand)',
              })}
            >
              <Icon size={18} strokeWidth={2.5} />
              {label}
            </NavLink>
          ))}
        </nav>

      </aside>

      {/* Main content */}
      <main className="flex-1 min-w-0 p-4 md:p-8 pt-16 md:pt-8">
        <div className="max-w-5xl mx-auto">
          <Outlet />
        </div>
      </main>
    </div>
  );
}
