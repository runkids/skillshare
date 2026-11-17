import { describe, expect, it } from 'vitest';
import { commandSummary, describeChange, formatCommand, nativeEdits, rowPath, rowTitle, sameEvents } from './hooksImport';

const group = (matcher: string, command: string) => ({ matcher, hooks: [{ type: 'command', command }] });

describe('commandSummary', () => {
  it('shows the script a long command ends up running', () => {
    expect(commandSummary('if [ -z "$X" ]; then exit 0; fi; [ -x ~/.claude/hook.sh ] && ~/.claude/hook.sh Stop')).toBe('~/.claude/hook.sh');
  });
  it('keeps the variable a script path starts with', () => {
    expect(commandSummary('"$CLAUDE_PROJECT_DIR"/.claude/hooks/fmt.sh')).toBe('$CLAUDE_PROJECT_DIR/.claude/hooks/fmt.sh');
  });
  it('falls back to the first line', () => {
    expect(commandSummary('\nafplay Glass\necho done')).toBe('afplay Glass');
  });
});

describe('sameEvents', () => {
  it('ignores key order', () => {
    expect(sameEvents({ Stop: [{ a: 1, b: 2 }] }, { Stop: [{ b: 2, a: 1 }] })).toBe(true);
  });
  it('sees a changed command', () => {
    expect(sameEvents({ Stop: [group('', 'a')] }, { Stop: [group('', 'b')] })).toBe(false);
  });
});

describe('nativeEdits', () => {
  it('sends a removal without an after and an edit with one', () => {
    const before = { Stop: [group('', 'a')] };
    const after = { Stop: [group('', 'b')] };
    expect(nativeEdits([
      { target: 'claude', before, change: { remove: true } },
      { target: 'codex', root: '/repo', before, change: { events: after } },
    ])).toEqual([{ target: 'claude', before }, { target: 'codex', root: '/repo', before, after }]);
  });
  it('names the block or file a hook was read from', () => {
    const before = { Stop: [{ command: 'a' }] };
    expect(nativeEdits([{ target: 'antigravity', source: 'lint', before, change: { remove: true } }])).toEqual([{ target: 'antigravity', source: 'lint', before }]);
  });
});

describe('rowTitle and rowPath', () => {
  const row = { id: 'copilot:team.json', target: 'copilot', adopt: true, candidate: { name: 'team', source: 'team.json', problems: [], warnings: [], entry: { bindings: { copilot: { events: { sessionStart: [] } } } } } };
  it('names a Copilot hook by its file, even with one event', () => {
    expect(rowTitle(row)).toBe('team.json');
  });
  it('points an edit at the Copilot file, not its folder', () => {
    expect(rowPath(row, '/home/u/.copilot/hooks')).toBe('/home/u/.copilot/hooks/team.json');
  });
});

describe('describeChange', () => {
  it('names a matcher change', () => {
    expect(describeChange({ PreToolUse: [group('', 'a')] }, { PreToolUse: [group('Bash', 'a')] })).toEqual({ key: 'hooks.importChange.matcher', params: { from: '*', to: 'Bash' } });
  });
  it('names the command that changed', () => {
    expect(describeChange({ Stop: [group('', 'a'), group('', 'b')] }, { Stop: [group('', 'a'), group('', 'c')] })).toEqual({ key: 'hooks.importChange.command', params: { n: '2' } });
  });
  it('says only that anything larger was edited', () => {
    expect(describeChange({ Stop: [group('', 'a')] }, { Stop: [group('', 'a'), group('', 'b')] })).toEqual({ key: 'hooks.importChange.edited' });
  });
});

describe('formatCommand', () => {
  it('puts one statement on a line and indents blocks', () => {
    expect(formatCommand('if [ -z "$X" ]; then case "$Y" in a) exit 0;; esac; fi; [ -x ~/h.sh ] && ~/h.sh Stop')).toBe([
      'if [ -z "$X" ]; then',
      '  case "$Y" in a) exit 0;; esac',
      'fi',
      '[ -x ~/h.sh ] &&',
      '  ~/h.sh Stop',
    ].join('\n'));
  });
  it('leaves separators inside quotes and substitutions alone', () => {
    const escaped = String.raw`find . -exec rm {} \;`;
    expect(formatCommand(`echo "a; b" && x=$(date; id) ; ${escaped}`)).toBe(['echo "a; b" &&', '  x=$(date; id)', escaped].join('\n'));
  });
  it('formats a loop', () => {
    expect(formatCommand('for f in a b; do echo $f; done')).toBe('for f in a b; do\n  echo $f\ndone');
  });
  it.each([
    'cat <<EOF; echo x',
    'echo `date`; ls',
    'ls # list; rm x',
    `echo "open; ls`,
    // A comment also starts right after a separator, and would stop hiding what follows it.
    'echo a;# disabled; rm -f /tmp/x',
    // Quotes nest inside a substitution, so the inner ones do not close the outer string.
    'echo "$(printf "%s;%s" a b)"',
    'echo "${x:-"a;b"}"',
    String.raw`echo $'a\';b'; ls`,
    // An escaped space or line break must not end up next to an inserted line break.
    String.raw`printf "[%s]\n" a\ ;printf "[%s]\n" b`,
    'echo a \\\n  b; echo c',
    // Case fall-through.
    'case $x in a) echo A;& b) echo B;; esac',
    'case $x in a) echo A;;& b) echo B;; esac; echo z',
  ])('returns %s unchanged', (command) => {
    expect(formatCommand(command)).toBe(command);
  });
  it('does not take a command that starts with a keyword for the keyword', () => {
    expect(formatCommand('cd /x; do-backup.sh; else.sh')).toBe('cd /x\ndo-backup.sh\nelse.sh');
  });
  it('is stable on its own output', () => {
    const once = formatCommand('if a; then b; else c; fi');
    expect(once).toBe('if a; then\n  b\nelse\n  c\nfi');
    expect(formatCommand(once)).toBe(once);
  });
});
