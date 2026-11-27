import { describe, expect, it } from 'vitest';
import {
  commandScriptStem,
  copyDraft,
  defaultImportNames,
  diffLines,
  eventLabels,
  lintNative,
  lineOf,
  suggestName,
} from './hookCatalog';
import type { HookCatalog } from './hookCatalog';
import { emptyBinding, newRow } from './hooksView';

const catalog: HookCatalog = {
  claude: {
    timeoutUnit: 'seconds',
    events: [
      { name: 'PreToolUse', description: 'Before a tool runs', matcher: true },
      { name: 'Stop', description: 'When the main agent finishes', matcher: false },
      { name: 'Notification', description: 'When a notification is sent', matcher: true },
    ],
  },
  gemini: {
    timeoutUnit: 'milliseconds',
    events: [
      { name: 'BeforeTool', description: 'Before a tool runs', matcher: true },
      { name: 'AfterAgent', description: 'After the agent replies', matcher: false },
    ],
  },
  cursor: {
    timeoutUnit: 'seconds',
    events: [{ name: 'stop', description: 'When the agent stops', matcher: false }],
  },
};

const claudeDraft = (...rows: [string, string, string, string][]) => ({
  ...emptyBinding('claude'),
  rows: rows.map(([event, matcher, command, timeout]) => ({
    ...newRow('claude', event),
    matcher,
    command,
    timeout,
  })),
});

describe('copyDraft', () => {
  it("maps each event to the target's own name and converts the timeout unit", () => {
    const { draft, notes } = copyDraft(
      catalog,
      'claude',
      'gemini',
      claudeDraft(['PreToolUse', 'Bash', './guard.sh', '10'], ['Stop', '', './done.sh', '']),
    );
    expect(draft.rows.map((r) => [r.event, r.matcher, r.command, r.timeout])).toEqual([
      ['BeforeTool', 'Bash', './guard.sh', '10000'],
      ['AfterAgent', '', './done.sh', ''],
    ]);
    expect(notes).toEqual([
      { from: 'PreToolUse', to: 'BeforeTool', kind: 'closest' },
      { from: 'Stop', to: 'AfterAgent', kind: 'closest' },
    ]);
  });

  it('leaves an event the target has nothing close to empty and says so', () => {
    const { draft, notes } = copyDraft(
      catalog,
      'claude',
      'cursor',
      claudeDraft(['Notification', '', './ping.sh', '']),
    );
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
      expect.objectContaining({
        severity: 'warning',
        key: 'hooks.lint.unknownEventSuggest',
        params: { event: 'Stopp', suggestion: 'Stop' },
      }),
    ]);
  });

  it('flags a timeout that is not a number', () => {
    expect(
      lintNative(
        '{ "Stop": [{ "hooks": [{ "type": "command", "command": "x", "timeout": "30" }] }] }',
        'claude',
        catalog,
      ),
    ).toEqual([
      expect.objectContaining({
        severity: 'error',
        key: 'hooks.lint.timeoutType',
        params: { unit: 'seconds' },
      }),
    ]);
  });
});

describe('eventLabels', () => {
  it('lists added, updated and removed events in that order', () => {
    expect(
      eventLabels({
        events: { added: ['Stop'], updated: ['PostToolUse'], removed: ['PreToolUse'] },
      }),
    ).toEqual(['+ Stop', '~ PostToolUse', '− PreToolUse']);
  });
});

describe('diffLines', () => {
  it('lists removed lines before added ones within a change', () => {
    expect(diffLines('a\nold\nb', 'a\nnew\nb').map((l) => l.op)).toEqual([' ', '-', '+', ' ']);
  });

  it('shows a line that only gained a trailing comma as removed and added, removals first', () => {
    expect(diffLines('{\n  "A": []\n}', '{\n  "A": [],\n  "B": []\n}')).toEqual([
      { op: ' ', text: '{' },
      { op: '-', text: '  "A": []' },
      { op: '+', text: '  "A": [],' },
      { op: '+', text: '  "B": []' },
      { op: ' ', text: '}' },
    ]);
  });

  it('shows a line that only lost a trailing comma as removed and added', () => {
    expect(diffLines('[\n  1,\n  2\n]', '[\n  1\n]').map((l) => `${l.op}${l.text}`)).toEqual([
      ' [',
      '-  1,',
      '-  2',
      '+  1',
      ' ]',
    ]);
  });

  it('adds an event after the last one as a comma pair plus the new lines, closing brackets untouched', () => {
    const before = '{\n  "hooks": {\n    "A": [\n      {}\n    ]\n  }\n}';
    const after =
      '{\n  "hooks": {\n    "A": [\n      {}\n    ],\n    "B": [\n      {}\n    ]\n  }\n}';
    expect(
      diffLines(before, after)
        .filter((l) => l.op !== ' ')
        .map((l) => `${l.op}${l.text}`),
    ).toEqual(['-    ]', '+    ],', '+    "B": [', '+      {}', '+    ]']);
  });

  it('keeps unrelated lines aligned when a comma changes', () => {
    const before = '{\n  "A": [],\n  "B": [],\n  "C": []\n}';
    const after = '{\n  "A": [],\n  "B": [],\n  "C": [],\n  "D": []\n}';
    expect(
      diffLines(before, after)
        .filter((l) => l.op !== ' ')
        .map((l) => `${l.op}${l.text}`),
    ).toEqual(['-  "C": []', '+  "C": [],', '+  "D": []']);
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
    expect(suggestName('claude-stop', 'echo done', (n) => n === 'claude-stop')).toBe(
      'claude-stop-2',
    );
  });

  it('skips versioned interpreters, switches, and runner subcommands to find the script stem', () => {
    expect(
      suggestName(
        'cursor-stop',
        '/opt/homebrew/opt/python@3.14/bin/python3.14 ~/.local/share/tool/check.py cursor stop',
        none,
      ),
    ).toBe('check');
    expect(suggestName('claude-stop', '/bin/bash -c "echo done"', none)).toBe('claude-stop');
    expect(suggestName('claude-stop', '/bin/bash -lc "echo done"', none)).toBe('claude-stop');
    expect(commandScriptStem('/bin/bash -e ~/.claude/guard.sh')).toBe('guard');
    expect(commandScriptStem('/usr/bin/env python3 ~/.claude/audit.py')).toBe('audit');
    expect(commandScriptStem('/usr/bin/env NODE_ENV=production node ~/.claude/check.js')).toBe(
      'check',
    );
    expect(commandScriptStem('/usr/bin/perl5.34 ~/.claude/filter.pl')).toBe('filter');
    expect(commandScriptStem('/usr/bin/php8.3 ~/.claude/hook.php')).toBe('hook');
    expect(commandScriptStem('python -u ./hook.py')).toBe('hook');
    expect(commandScriptStem('python3 -I -P ./isolated.py')).toBe('isolated');
    expect(commandScriptStem('/usr/bin/env -S uv run -p 3.12 scripts/verify.py')).toBe('verify');
    expect(commandScriptStem('uv run --python 3.12 scripts/verify.py')).toBe('verify');
    expect(commandScriptStem('uv run -p 3.12 scripts/verify.py')).toBe('verify');
    expect(commandScriptStem('node -r ./bootstrap.js ./hook.js')).toBe('hook');
    expect(commandScriptStem('node --import ./instrument.mjs ./audit.mjs')).toBe('audit');
    expect(commandScriptStem('ts-node -P ./tsconfig.json ./lint.ts')).toBe('lint');
    expect(commandScriptStem('tsx --tsconfig ./tsconfig.hooks.json ./lint.ts')).toBe('lint');
    expect(commandScriptStem('ruby -I ./lib -r ./setup.rb ./guard.rb')).toBe('guard');
    expect(commandScriptStem('bash --rcfile ~/.bashrc ~/.claude/guard.sh')).toBe('guard');
    expect(commandScriptStem('php -c /etc/php.ini -e ./hook.php')).toBe('hook');
    expect(commandScriptStem('php -r "echo 1;"')).toBe('');
    expect(commandScriptStem('node -p "1 + 1"')).toBe('');
    expect(commandScriptStem('pwsh -ExecutionPolicy Bypass -File C:\\hooks\\guard.ps1')).toBe(
      'guard',
    );
    expect(commandScriptStem('cmd.exe /d /s /c C:\\hooks\\guard.bat')).toBe('guard');
  });

  it('handles quoted paths with spaces and quoted env prefixes', () => {
    expect(commandScriptStem('"/Users/me/My Scripts/guard.sh"')).toBe('guard');
    expect(
      commandScriptStem('"C:\\Program Files\\nodejs\\node.exe" "C:\\My Hooks\\check.js"'),
    ).toBe('check');
    expect(commandScriptStem('"$CLAUDE_PROJECT_DIR"/.claude/hooks/lint.sh')).toBe('lint');
  });

  it('falls back to candidate instead of numbering when the script stem is already taken', () => {
    expect(
      suggestName(
        'gemini-aftertool',
        '~/.local/bin/hyp hook gemini after-tool',
        (n) => n === 'hyp',
      ),
    ).toBe('gemini-aftertool');
  });

  it('assigns <target>-<event> names across rows that share the same dispatcher binary', () => {
    expect(
      defaultImportNames(
        [
          {
            id: 'claude:claude-pretooluse',
            candidate: 'claude-pretooluse',
            command: '~/.local/bin/hyp hook claude pre-tool-use',
          },
          {
            id: 'claude:claude-stop',
            candidate: 'claude-stop',
            command: '~/.local/bin/hyp hook claude stop',
          },
          {
            id: 'claude:claude-notify',
            candidate: 'claude-notify',
            command: '~/.claude/hooks/notify.sh',
          },
        ],
        none,
      ),
    ).toEqual({
      'claude:claude-pretooluse': 'claude-pretooluse',
      'claude:claude-stop': 'claude-stop',
      'claude:claude-notify': 'notify',
    });
  });
});
