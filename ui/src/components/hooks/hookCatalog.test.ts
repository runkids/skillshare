import { describe, expect, it } from 'vitest';
import { copyDraft, diffLines, eventLabels, lintNative, lineOf, suggestName } from './hookCatalog';
import type { HookCatalog } from './hookCatalog';
import { emptyBinding, newRow } from './hooksView';

const catalog: HookCatalog = {
  claude: { timeoutUnit: 'seconds', events: [
    { name: 'PreToolUse', description: 'Before a tool runs', matcher: true },
    { name: 'Stop', description: 'When the main agent finishes', matcher: false },
    { name: 'Notification', description: 'When a notification is sent', matcher: true },
  ] },
  gemini: { timeoutUnit: 'milliseconds', events: [
    { name: 'BeforeTool', description: 'Before a tool runs', matcher: true },
    { name: 'AfterAgent', description: 'After the agent replies', matcher: false },
  ] },
  cursor: { timeoutUnit: 'seconds', events: [{ name: 'stop', description: 'When the agent stops', matcher: false }] },
};

const claudeDraft = (...rows: [string, string, string, string][]) => ({
  ...emptyBinding('claude'),
  rows: rows.map(([event, matcher, command, timeout]) => ({ ...newRow('claude', event), matcher, command, timeout })),
});

describe('copyDraft', () => {
  it("maps each event to the target's own name and converts the timeout unit", () => {
    const { draft, notes } = copyDraft(catalog, 'claude', 'gemini', claudeDraft(['PreToolUse', 'Bash', './guard.sh', '10'], ['Stop', '', './done.sh', '']));
    expect(draft.rows.map((r) => [r.event, r.matcher, r.command, r.timeout])).toEqual([['BeforeTool', 'Bash', './guard.sh', '10000'], ['AfterAgent', '', './done.sh', '']]);
    expect(notes).toEqual([{ from: 'PreToolUse', to: 'BeforeTool', kind: 'closest' }, { from: 'Stop', to: 'AfterAgent', kind: 'closest' }]);
  });

  it('leaves an event the target has nothing close to empty and says so', () => {
    const { draft, notes } = copyDraft(catalog, 'claude', 'cursor', claudeDraft(['Notification', '', './ping.sh', '']));
    expect(draft.rows[0]).toMatchObject({ event: '', command: './ping.sh' });
    expect(notes).toEqual([{ from: 'Notification', to: '', kind: 'none' }]);
  });
});

describe('lintNative', () => {
  it('points a JSON syntax error at the line it is on', () => {
    const text = '{\n  "Stop": [\n    { "hooks": [] }\n    { "hooks": [] }\n  ]\n}';
    const [problem] = lintNative(text, 'claude', catalog);
    expect(problem).toMatchObject({ severity: 'error', key: 'hooks.lint.comma' });
    expect(lineOf(text, problem.from)).toBe(4);
  });

  it('flags an event the target does not document and suggests the closest one', () => {
    expect(lintNative('{ "Stopp": [] }', 'claude', catalog)).toEqual([
      expect.objectContaining({ severity: 'warning', key: 'hooks.lint.unknownEventSuggest', params: { event: 'Stopp', suggestion: 'Stop' } }),
    ]);
  });

  it('flags a timeout that is not a number', () => {
    expect(lintNative('{ "Stop": [{ "hooks": [{ "type": "command", "command": "x", "timeout": "30" }] }] }', 'claude', catalog)).toEqual([
      expect.objectContaining({ severity: 'error', key: 'hooks.lint.timeoutType', params: { unit: 'seconds' } }),
    ]);
  });
});

describe('eventLabels', () => {
  it('lists added, updated and removed events in that order', () => {
    expect(eventLabels({ events: { added: ['Stop'], updated: ['PostToolUse'], removed: ['PreToolUse'] } })).toEqual(['+ Stop', '~ PostToolUse', '− PreToolUse']);
  });
});

describe('diffLines', () => {
  it('lists removed lines before added ones within a change', () => {
    expect(diffLines('a\nold\nb', 'a\nnew\nb').map((l) => l.op)).toEqual([' ', '-', '+', ' ']);
  });

  it('keeps a line that only gained a trailing comma as unchanged, showing the new text', () => {
    expect(diffLines('{\n  "A": []\n}', '{\n  "A": [],\n  "B": []\n}')).toEqual([
      { op: ' ', text: '{' }, { op: ' ', text: '  "A": [],' }, { op: '+', text: '  "B": []' }, { op: ' ', text: '}' },
    ]);
  });
});

describe('suggestName', () => {
  const none = () => false;

  it("keeps the CLI's target-event name for an inline command", () => {
    expect(suggestName('claude-stop', 'echo done', none)).toBe('claude-stop');
  });

  it("uses a script's basename when the command runs one by path", () => {
    expect(suggestName('claude-pretooluse', '~/.claude/guard.sh --strict', none)).toBe('guard');
  });

  it('numbers a name that is taken', () => {
    expect(suggestName('claude-stop', 'echo done', (n) => n === 'claude-stop')).toBe('claude-stop-2');
  });
});
