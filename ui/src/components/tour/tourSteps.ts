export interface TourStep {
  id: string;
  page: string;
  targetSelector: string;
  title: string;
  description: string;
  emptyDescription?: string;
  placement: 'top' | 'bottom' | 'left' | 'right';
}

const ALL_STEPS: TourStep[] = [
  { id: 'stats-grid', page: '/', targetSelector: "[data-tour='stats-grid']", title: 'Dashboard Overview', description: 'Real-time stats for skills, targets, and sync status. Zeros are normal — numbers update after installing skills.', placement: 'bottom' },
  { id: 'quick-actions', page: '/', targetSelector: "[data-tour='quick-actions']", title: 'Quick Actions', description: 'Shortcuts for common operations: one-click sync, security scan, browse skills, batch update.', placement: 'top' },
  { id: 'skills-view', page: '/skills', targetSelector: "[data-tour='skills-view']", title: 'Skills Management', description: 'Browse all installed skills. Supports grid, grouped, and table views with search and filters.', emptyDescription: 'No skills yet. After the tour, try installing your first skill from Search or Install!', placement: 'bottom' },
  { id: 'extras-list', page: '/extras', targetSelector: "[data-tour='extras-list']", title: 'Extras', description: 'Manage non-skill extra file directories (hooks, snippets, etc.) synced to targets.', emptyDescription: 'No extras yet. After the tour, try adding an extra directory (hooks, snippets, etc.) to sync alongside your skills!', placement: 'bottom' },
  { id: 'targets-grid', page: '/targets', targetSelector: "[data-tour='targets-grid']", title: 'Targets', description: 'Your AI CLI tools (Claude, Cursor, etc.). Each target can be configured with its own sync mode.', placement: 'bottom' },
  { id: 'skill-filters', page: '/targets', targetSelector: "[data-tour='skill-filters']", title: 'Skill Filters', description: 'Use Include/Exclude patterns to control which skills sync to each target. For example, exclude large skills from lightweight tools.', placement: 'bottom' },
  { id: 'search-input', page: '/search', targetSelector: "[data-tour='search-input']", title: 'Search Skills', description: 'Search community-shared skills from GitHub and Hubs. Install directly from results.', placement: 'bottom' },
  { id: 'sync-actions', page: '/sync', targetSelector: "[data-tour='sync-actions']", title: 'Sync Operations', description: 'Sync skills from source to all targets. Preview with Diff before executing.', placement: 'bottom' },
  { id: 'collect-scan', page: '/collect', targetSelector: "[data-tour='collect-scan']", title: 'Collect Local Skills', description: 'Scan targets for manually created skills and collect them back to source for unified management.', placement: 'bottom' },
  { id: 'install-form', page: '/install', targetSelector: "[data-tour='install-form']", title: 'Install Skills', description: 'Enter a GitHub repo URL to install skills. Track mode (--track) enables future updates.', placement: 'bottom' },
  { id: 'audit-summary', page: '/audit', targetSelector: "[data-tour='audit-summary']", title: 'Security Audit', description: 'Scan all skills for security risks, graded by severity (Critical → Info). Run regularly.', placement: 'bottom' },
  { id: 'git-actions', page: '/git', targetSelector: "[data-tour='git-actions']", title: 'Git Sync', description: 'Back up and sync skill configs via Git. Push to upload, Pull to download.', placement: 'bottom' },
  { id: 'log-filters', page: '/log', targetSelector: "[data-tour='log-filters']", title: 'Operation Log', description: 'View all operation history. Filter by command type and time range.', placement: 'bottom' },
  { id: 'shortcuts-btn', page: '/log', targetSelector: "[data-tour='shortcuts-btn']", title: 'Keyboard Shortcuts', description: 'Press ? to see all shortcuts. Use Cmd+S (Ctrl+S on Windows/Linux) to quick-sync, g+d to jump to Dashboard, g+s to Skills, and more. Hold Cmd/Ctrl to see a shortcut HUD overlay.', placement: 'left' },
];

interface BuildStepsOptions {
  isProjectMode: boolean;
  skillCount: number;
}

export function buildSteps({ isProjectMode, skillCount }: BuildStepsOptions): TourStep[] {
  let steps = ALL_STEPS;
  if (isProjectMode) {
    steps = steps.filter((s) => s.id !== 'git-actions');
  }
  if (skillCount === 0) {
    steps = steps.map((s) => s.emptyDescription ? { ...s, description: s.emptyDescription } : s);
  }
  return steps;
}

export const TOUR_STORAGE_KEY = 'skillshare.tour.completed';
