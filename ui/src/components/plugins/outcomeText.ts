import type { useT } from '../../i18n';

type T = ReturnType<typeof useT>;

const statusKeys: Record<string, string> = {
  installed: 'plugins.outcome.installed',
  unchanged: 'plugins.outcome.unchanged',
  skipped: 'plugins.outcome.skipped',
  saved: 'plugins.outcome.saved',
  removed: 'plugins.outcome.removed',
  excluded: 'plugins.outcome.excluded',
  imported: 'plugins.outcome.imported',
  failed: 'plugins.outcome.failed',
};

// Mirrors the fixed outcome messages in internal/plugin/lifecycle.go; native errors stay intact.
const messageKeys: Record<string, string> = {
  'Native installation recorded. Reload the Agent and complete any required login or hook trust.': 'plugins.outcome.installHelp',
  'Added to Skillshare. Choose Agents when you want to install it.': 'plugins.outcome.addHelp',
  'Removed from Skillshare.': 'plugins.outcome.removeHelp',
  'Native plugin removed; shared marketplaces are retained.': 'plugins.outcome.nativeRemoveHelp',
  'Sync selection saved. Run sync to apply installation changes.': 'plugins.outcome.selectionHelp',
  'Removed from this target; plugin definition retained.': 'plugins.outcome.excludeHelp',
  'Existing installation adopted without changing its enabled state.': 'plugins.outcome.importHelp',
  'Native operation completed, but recording its result failed; inspect status before retrying.': 'plugins.outcome.recordFailedHelp',
};

export const outcomeStatus = (t: T, status: string) => statusKeys[status] ? t(statusKeys[status], undefined, status) : status;
export const outcomeMessage = (t: T, message: string) => messageKeys[message] ? t(messageKeys[message], undefined, message) : message;
// A message the server names with a key, such as why an update skipped an Agent.
export const keyedMessage = (t: T, m: { message?: string; messageKey?: string; messageArgs?: Record<string, string> }) =>
  m.messageKey ? t(m.messageKey, m.messageArgs, m.message) : outcomeMessage(t, m.message ?? '');
