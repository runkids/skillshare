type Placement = 'top' | 'bottom' | 'left' | 'right';

export interface TourStep {
  id: string;
  page: string;
  targetSelector: string;
  placement: Placement;
  /** Show the copy written for a library with nothing in it yet. */
  empty?: boolean;
}

// Every step reads its title and description from `tour.steps.<id>` in the locale files.
// Anchors live on controls that are always rendered, not on lists that disappear when empty:
// a missing target makes the step time out and skip itself.
const step = (id: string, page: string, placement: Placement): TourStep => ({ id, page, placement, targetSelector: `[data-tour='${id}']` });

// Steps that also have an `emptyDescription` for a fresh install
const HAS_EMPTY_COPY = new Set(['skills-view', 'extras-list']);

const ALL_STEPS: TourStep[] = [
  step('stats-grid', '/', 'bottom'),
  step('quick-actions', '/', 'bottom'),
  step('skills-view', '/skills', 'bottom'),
  step('install-button', '/skills', 'left'),
  step('extras-list', '/extras', 'left'),
  step('mcp-actions', '/mcp', 'bottom'),
  step('targets-grid', '/targets', 'left'),
  step('sync-actions', '/sync', 'bottom'),
  step('audit-summary', '/audit', 'bottom'),
  step('git-actions', '/git', 'bottom'),
  step('log-filters', '/log', 'bottom'),
  step('shortcuts-btn', '/log', 'left'),
];

interface BuildStepsOptions {
  isProjectMode: boolean;
  skillCount: number;
}

export function buildSteps({ isProjectMode, skillCount }: BuildStepsOptions): TourStep[] {
  // A project keeps its skills in its own repository, so there is no Git Sync page.
  let steps = isProjectMode ? ALL_STEPS.filter((s) => s.id !== 'git-actions') : ALL_STEPS;
  if (skillCount === 0) {
    steps = steps.map((s) => (HAS_EMPTY_COPY.has(s.id) ? { ...s, empty: true } : s));
  }
  return steps;
}

export const TOUR_STORAGE_KEY = 'skillshare.tour.completed';
