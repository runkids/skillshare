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
  'Removed from Skillshare.': 'plugins.outcome.removeHelp',
  'Native plugin removed; shared marketplaces are retained.': 'plugins.outcome.nativeRemoveHelp',
  'Removed from this target; plugin definition retained.': 'plugins.outcome.excludeHelp',
  'Existing installation adopted without changing its enabled state.': 'plugins.outcome.importHelp',
  'Native operation completed, but recording its result failed; inspect status before retrying.': 'plugins.outcome.recordFailedHelp',
};

export const outcomeStatus = (t: T, status: string) => statusKeys[status] ? t(statusKeys[status], undefined, status) : status;
export const outcomeMessage = (t: T, message: string) => messageKeys[message] ? t(messageKeys[message], undefined, message) : message;
/**
 * The backend keys its fixed sentences; a message it assembled at runtime has no key
 * and is shown as it came, which is also what the CLI prints.
 */
export const keyedText = (t: T, key: string | undefined, text: string | undefined, args?: Record<string, string>) => (key ? t(key, args, text) : text ?? '');
// An outcome's message: keyed as above, else one of the fixed sentences older outcomes carry.
export const keyedMessage = (t: T, m: { message?: string; messageKey?: string; messageArgs?: Record<string, string> }) =>
  m.messageKey ? keyedText(t, m.messageKey, m.message, m.messageArgs) : outcomeMessage(t, m.message ?? '');
