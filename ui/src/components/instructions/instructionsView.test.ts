import { ApiError } from '../../api/client';
import { translate } from '../../i18n';
import { describe, expect, it } from 'vitest';
import type { SharedInstructionsFile, SharedInstructionsTarget } from '../../api/client';
import {
  instructionsErrorMessage,
  instructionsWarningMessage,
  blockOwner,
  connectPlan,
  defaultShareName,
  managedBlocks,
  readChain,
  saveCopiesSummary,
  importDecor,
  importLines,
  previewParts,
  lineCount,
  lineDiff,
  lineRanges,
  modeOptions,
  needsSync,
  overLimit,
  restorePlan,
  rowHint,
  setupPathOf,
  setupPathProblem,
  sharedNameProblem,
  sharedOfImport,
  worstStatus,
} from './instructionsView';

describe('importLines', () => {
  it('finds @path lines outside code fences', () => {
    expect(importLines('# Notes\n@RTK.md\n```\n@not-this\n```\n@AGENTS.md\n')).toEqual([2, 6]);
  });

  it('ignores @ mentions inside prose', () => {
    expect(importLines('Ask @someone before merging.')).toEqual([]);
  });
});

describe('lineRanges', () => {
  it('folds consecutive lines into ranges', () => {
    expect(lineRanges([3, 7, 8, 9])).toBe('3, 7–9');
  });
});

describe('lineDiff', () => {
  it('marks removed and added lines around unchanged ones', () => {
    expect(lineDiff('a\nb\nc\n', 'a\nc\nd\n')).toEqual([
      { kind: 'same', text: 'a' },
      { kind: 'del', text: 'b' },
      { kind: 'same', text: 'c' },
      { kind: 'add', text: 'd' },
    ]);
  });
});

describe('worstStatus', () => {
  it('picks the status that most needs attention', () => {
    expect(
      worstStatus([
        { name: 'a', mode: 'import', status: 'synced' },
        { name: 'b', mode: 'import', status: 'modified' },
      ]),
    ).toBe('modified');
  });
});

describe('overLimit', () => {
  const file = (name: string, chars: number): SharedInstructionsFile => ({
    name,
    file: 'AGENTS.md',
    path: `/x/${name}/AGENTS.md`,
    exists: true,
    size: chars,
    chars,
    targets: 0,
  });
  const windsurf: SharedInstructionsTarget = {
    name: 'windsurf',
    path: '/h/.codeium/windsurf/memories/global_rules.md',
    import: false,
    exists: true,
    max_chars: 6000,
    assigned: [{ name: 'work', mode: 'symlink', status: 'synced' }],
  };

  it('lists only assigned files longer than the target reads', () => {
    expect(
      overLimit(windsurf, [file('work', 7000), file('personal', 9000)]).map((f) => f.name),
    ).toEqual(['work']);
  });
});

describe('sharedNameProblem', () => {
  it('rejects names the server would refuse', () => {
    expect(sharedNameProblem('my file', [])).toBe('invalid');
  });

  it('flags a name another extra already uses', () => {
    expect(sharedNameProblem('team', ['rules', 'team'])).toBe('taken');
  });

  it('treats a name that differs only in case as taken', () => {
    expect(sharedNameProblem('Team', ['team'])).toBe('taken');
  });

  it('accepts a new valid name', () => {
    expect(sharedNameProblem('work-2', ['team'])).toBeNull();
  });
});

describe('setupPathProblem', () => {
  it('accepts a ~/ or absolute file in global mode', () => {
    expect([
      setupPathProblem('~/.myagent/AGENTS.md', false),
      setupPathProblem('/opt/a/AGENTS.md', false),
    ]).toEqual([null, null]);
  });

  it('refuses a relative file in global mode', () => {
    expect(setupPathProblem('AGENTS.md', false)).toBe('relative');
  });

  it('refuses an absolute file in project mode', () => {
    expect(setupPathProblem('~/AGENTS.md', true)).toBe('absolute');
  });

  it('refuses a directory', () => {
    expect(setupPathProblem('.myagent/', true)).toBe('directory');
  });
});

describe('connect and restore plans', () => {
  const general: SharedInstructionsFile = {
    name: 'general',
    file: 'AGENTS.md',
    path: '/x/general/AGENTS.md',
    exists: true,
    size: 7000,
    chars: 7000,
    targets: 0,
  };
  const tg = (
    name: string,
    rest: Partial<SharedInstructionsTarget> = {},
  ): SharedInstructionsTarget => ({
    name,
    path: `/h/${name}/AGENTS.md`,
    import: false,
    exists: true,
    assigned: [],
    ...rest,
  });
  const claude = tg('claude', {
    import: true,
    assigned: [{ name: 'team', mode: 'import', status: 'synced' }],
  });
  const opencode = tg('opencode', {
    assigned: [{ name: 'team', mode: 'symlink', status: 'synced' }],
  });
  const windsurf = tg('windsurf', { max_chars: 6000 });
  const antigravity = tg('antigravity', { same_as: 'gemini' });

  it('adds an import line and keeps the other files of an import target', () => {
    expect(connectPlan([claude], general)).toEqual([
      { target: 'claude', extras: ['team', 'general'], note: 'import' },
    ]);
  });

  it('adds a managed block for a target without @import and keeps its other blocks', () => {
    const cursor = tg('cursor', {
      assigned: [{ name: 'team', mode: 'prepend', status: 'synced' }],
    });
    expect(connectPlan([cursor], general)).toEqual([
      { target: 'cursor', extras: ['team', 'general'], note: 'block' },
    ]);
  });

  it('offers prepend and append to a target without @import, with import listed as blocked', () => {
    const cursor = tg('cursor', {
      assigned: [
        { name: 'team', mode: 'prepend', status: 'synced' },
        { name: 'general', mode: 'append', status: 'synced' },
      ],
    });
    const options = modeOptions(cursor, true);
    expect(options.map((o) => o.mode)).toEqual(['import', 'prepend', 'append', 'symlink', 'copy']);
    expect(options.find((o) => o.isDefault)?.mode).toBe('prepend');
    expect(options.map((o) => o.blocked)).toEqual([
      'noImport',
      undefined,
      undefined,
      'several',
      'several',
    ]);
  });

  it('keeps a link as the default for a target without @import that holds nothing yet', () => {
    expect(modeOptions(tg('cursor'), true).find((o) => o.isDefault)?.mode).toBe('symlink');
  });

  it('removes only the block when restoring a block target that holds another file', () => {
    const cursor = tg('cursor', {
      assigned: [
        { name: 'team', mode: 'prepend', status: 'synced' },
        { name: 'general', mode: 'append', status: 'synced' },
      ],
    });
    expect(restorePlan([cursor], 'general')).toEqual([
      { target: 'cursor', note: 'blockKeep', others: ['team'] },
    ]);
  });

  it('skips a target another file holds by link, saying which', () => {
    expect(connectPlan([opencode], general)).toEqual([
      { target: 'opencode', extras: ['general'], note: 'held', other: 'team' },
    ]);
  });

  it('skips a target another file holds by copy', () => {
    const pi = tg('pi', { assigned: [{ name: 'team', mode: 'copy', status: 'synced' }] });
    expect(connectPlan([pi], general)[0]).toMatchObject({ note: 'held', other: 'team' });
  });

  it('warns when a target reads less than the file holds', () => {
    expect(connectPlan([windsurf], general)[0]).toMatchObject({ note: 'tooLong', max: 6000 });
  });

  // Its file is another shared file's source: an import line written there would land in that file.
  it('skips a target whose file is another shared file, saying which', () => {
    const linked = tg('claude-unno', {
      import: true,
      linked_shared: 'team',
      assigned: [{ name: 'team', mode: 'symlink', status: 'synced' }],
    });
    expect(connectPlan([linked], general)).toEqual([
      { target: 'claude-unno', extras: ['team', 'general'], note: 'held', other: 'team' },
    ]);
  });

  it("never includes targets that read another target's file", () => {
    expect(connectPlan([antigravity], general)).toEqual([]);
  });

  it('keeps the other import lines when restoring an import target', () => {
    const both = tg('claude', {
      import: true,
      assigned: [
        { name: 'team', mode: 'import', status: 'synced' },
        { name: 'general', mode: 'import', status: 'synced' },
      ],
    });
    expect(restorePlan([both], 'general')).toEqual([
      { target: 'claude', note: 'importKeep', others: ['team'] },
    ]);
  });

  it('warns that outside edits are backed up before restoring', () => {
    const edited = tg('gemini', {
      assigned: [{ name: 'general', mode: 'symlink', status: 'modified' }],
    });
    expect(restorePlan([edited], 'general')).toEqual([{ target: 'gemini', note: 'modified' }]);
  });

  it('counts only connected targets whose file or link is missing or off as needing sync', () => {
    const lost = tg('codex', {
      assigned: [{ name: 'general', mode: 'symlink', status: 'not synced' }],
    });
    const fine = tg('amp', { assigned: [{ name: 'general', mode: 'symlink', status: 'synced' }] });
    expect(needsSync([lost, fine, opencode], 'general').map((t) => t.name)).toEqual(['codex']);
  });

  it('tells a link target on another file which one it uses now', () => {
    expect(rowHint(opencode, general)).toEqual({ kind: 'usesOther', name: 'team' });
  });

  it('tells an import target which other files it also uses', () => {
    expect(rowHint(claude, general)).toEqual({ kind: 'alsoUses', names: ['team'] });
  });

  it('finds the managed blocks in a target file and the shared file each came from', () => {
    const content =
      '<!-- skillshare:extra src="/h/.config/skillshare/extras/security/AGENTS.md" sha256=0123456789abcdef -->\n# S\n<!-- /skillshare:extra -->\n\n<!-- skillshare:extra src="../extras/team/AGENTS.md" sha256=0123456789abcdef -->\n# T\nrule\n<!-- /skillshare:extra -->\n\n# Mine\n';
    const blocks = managedBlocks(content);
    expect(blocks.map((b) => [b.start, b.end])).toEqual([
      [1, 3],
      [5, 8],
    ]);
    expect(blocks.map((b) => blockOwner(b, ['team', 'security']))).toEqual(['security', 'team']);
  });

  it('tells a target holding another file as a block that it also uses it, since a block joins instead of replacing', () => {
    const held = tg('codex', { assigned: [{ name: 'team', mode: 'prepend', status: 'synced' }] });
    expect(rowHint(held, general)).toEqual({ kind: 'alsoUses', names: ['team'] });
    const both = tg('codex', {
      assigned: [
        { name: 'team', mode: 'prepend', status: 'synced' },
        { name: 'general', mode: 'append', status: 'synced' },
      ],
    });
    expect(rowHint(both, general)).toEqual({ kind: 'alsoUses', names: ['team'] });
  });
});

describe('lineCount', () => {
  it('does not count the trailing newline as a line', () => {
    expect(lineCount('# A\n\nb\n')).toBe(3);
  });
});

describe('modeOptions', () => {
  const tg = (rest: Partial<SharedInstructionsTarget>): SharedInstructionsTarget => ({
    name: 'codex',
    path: '/h/.codex/AGENTS.md',
    import: false,
    exists: true,
    assigned: [{ name: 'team', mode: 'symlink', status: 'synced' }],
    ...rest,
  });

  it('offers import first and as the default where the tool reads @import lines', () => {
    expect(modeOptions(tg({ import: true }), true)).toEqual([
      { mode: 'import', isDefault: true },
      { mode: 'prepend', isDefault: false },
      { mode: 'append', isDefault: false },
      { mode: 'symlink', isDefault: false },
      { mode: 'copy', isDefault: false },
    ]);
  });

  it('lists import as blocked for other tools, with symlink as the default', () => {
    expect(modeOptions(tg({}), true)).toEqual([
      { mode: 'import', isDefault: false, blocked: 'noImport' },
      { mode: 'prepend', isDefault: false },
      { mode: 'append', isDefault: false },
      { mode: 'symlink', isDefault: true },
      { mode: 'copy', isDefault: false },
    ]);
  });

  it('blocks symlink and makes copy the default without file links', () => {
    expect(
      modeOptions(tg({}), false).filter((o) => o.mode === 'symlink' || o.mode === 'copy'),
    ).toEqual([
      { mode: 'symlink', isDefault: false, blocked: 'fileLinks' },
      { mode: 'copy', isDefault: true },
    ]);
  });

  it('keeps an import target on several shared files to import', () => {
    const both = tg({
      import: true,
      assigned: [
        { name: 'team', mode: 'import', status: 'synced' },
        { name: 'work', mode: 'import', status: 'synced' },
      ],
    });
    expect(
      modeOptions(both, true)
        .filter((o) => o.blocked)
        .map((o) => o.mode),
    ).toEqual(['symlink', 'copy']);
  });

  it('warns about a folder link the tool cannot read', () => {
    const file: SharedInstructionsFile = {
      name: 'team',
      file: 'AGENTS.md',
      path: '/x/team/AGENTS.md',
      exists: true,
      size: 1,
      chars: 1,
      targets: 1,
    };
    expect(
      rowHint(
        tg({
          assigned: [{ name: 'team', mode: 'symlink', status: 'drift', reason: 'folder_link' }],
        }),
        file,
      ),
    ).toEqual({ kind: 'folderLink' });
    expect(
      rowHint(
        tg({ assigned: [{ name: 'team', mode: 'symlink', status: 'drift', reason: 'directory' }] }),
        file,
      ),
    ).toEqual({ kind: 'directory' });
  });
});

describe('setupPathOf', () => {
  it('writes a global file from the home folder with ~/', () => {
    expect(setupPathOf('/home/me/.codex/AGENTS.md', false)).toBe('~/.codex/AGENTS.md');
  });

  it('writes a project file relative to the project root', () => {
    expect(setupPathOf('/work/app/.codex/AGENTS.md', true, '/work/app')).toBe('.codex/AGENTS.md');
    expect(
      setupPathOf(String.raw`C:\work\app\.codex\AGENTS.md`, true, String.raw`C:\work\app`),
    ).toBe('.codex/AGENTS.md');
  });
});

describe('importDecor', () => {
  const lines = [
    '<!-- skillshare:instructions:begin -->',
    '@/h/.config/skillshare/extras/personal/AGENTS.md',
    '<!-- skillshare:instructions:end -->',
    '',
    '# Mine',
  ];

  it('tints the managed block and notes its import line', () => {
    expect(importDecor(lines, (_, inBlock) => (inBlock ? 'managed' : 'own'))).toEqual([
      { block: true },
      { block: true, note: 'managed' },
      { block: true },
      null,
      null,
    ]);
  });

  it('names the shared file an import line of the block points at', () => {
    expect(sharedOfImport(lines[1], ['work', 'personal'])).toBe('personal');
  });
});

describe('previewParts', () => {
  it('shows the managed import block as the shared files it imports', () => {
    const content =
      '<!-- skillshare:instructions:begin -->\n@/h/.config/skillshare/extras/personal/AGENTS.md\n<!-- skillshare:instructions:end -->\n\n# Mine\n';
    expect(previewParts(content, ['personal'])).toEqual({
      before: '',
      imports: ['personal'],
      after: '\n# Mine\n',
    });
  });

  it('drops HTML comments that Markdown would show as text', () => {
    expect(previewParts('# A\n<!-- note -->\nB', []).before).toBe('# A\n\nB');
  });
});

describe('defaultShareName', () => {
  it('names the shared file after the target', () => {
    expect(defaultShareName('claude', ['personal'])).toBe('claude');
  });

  it('adds a number when that name is taken, ignoring case', () => {
    expect(defaultShareName('claude', ['Claude', 'claude-2'])).toBe('claude-3');
  });

  it('turns a target name the server would refuse into a valid one', () => {
    expect(defaultShareName('.my tool', [])).toBe('my-tool');
  });
});

describe('saveCopiesSummary', () => {
  it('lists updated copies and names the target of each problem', () => {
    expect(
      saveCopiesSummary([
        { target: 'pi', warnings: ['backed up ~/.pi/agent/AGENTS.md before replacing it'] },
        { target: 'grok', error: 'permission denied' },
      ]),
    ).toEqual({
      updated: ['pi'],
      warnings: ['pi: backed up ~/.pi/agent/AGENTS.md before replacing it'],
      errors: ['grok: permission denied'],
    });
  });
});

describe('readChain', () => {
  it('leaves out an empty rules folder but keeps a main file that does not exist yet', () => {
    expect(
      readChain([
        { path: '/h/.claude/CLAUDE.md', kind: 'main', exists: false, read: false },
        { path: '/h/.claude/rules', kind: 'rules', exists: false, read: false, count: 0 },
        { path: '/h/AGENTS.md', kind: 'unread', exists: false, read: false },
      ]).map((e) => e.kind),
    ).toEqual(['main']);
  });

  it('keeps a rules folder that holds files', () => {
    expect(
      readChain([{ path: '/h/.claude/rules', kind: 'rules', exists: true, read: true, count: 3 }]),
    ).toHaveLength(1);
  });
});

describe('instruction messages', () => {
  const t = (key: string, params?: Parameters<typeof translate>[2], fallback?: string) =>
    translate('zh-TW', key, params, fallback);
  it('translates warnings and shortens interpolated paths', () => {
    expect(
      instructionsWarningMessage(
        {
          code: 'backed_up',
          params: { path: '/home/sim/.codex/AGENTS.md' },
          message: 'English backup',
        },
        t,
      ),
    ).toBe('已先備份 ~/.codex/AGENTS.md 再覆蓋。');
  });
  it('translates coded errors and preserves mode names', () => {
    expect(
      instructionsErrorMessage(
        new ApiError(409, 'English directory', {
          code: 'instructions_target_directory',
          params: { path: '/Users/sim/.codex/AGENTS.md' },
        }),
        t,
      ),
    ).toBe('~/.codex/AGENTS.md 是資料夾，沒有覆蓋。');
    expect(
      instructionsErrorMessage(
        new ApiError(400, 'English mode', {
          code: 'instructions_invalid_mode',
          params: { mode: 'merge' },
        }),
        t,
      ),
    ).toContain('import、symlink 或 copy');
  });
  it('falls back to English for unknown codes and uncoded errors', () => {
    expect(
      instructionsWarningMessage({ code: 'future', params: {}, message: 'Future warning' }, t),
    ).toBe('Future warning');
    expect(
      instructionsErrorMessage(
        new ApiError(409, 'Future error', { code: 'instructions_future' }),
        t,
      ),
    ).toBe('Future error');
    expect(instructionsErrorMessage(new Error('Offline'), t)).toBe('Offline');
    expect(
      instructionsErrorMessage(
        new ApiError(500, 'read /home/sim/AGENTS.md failed', { code: 'instructions_future' }),
        t,
      ),
    ).toBe('read ~/AGENTS.md failed');
    expect(
      instructionsErrorMessage(
        new ApiError(500, 'write failed', {
          code: 'instructions_write_failed',
          params: { detail: 'open /Users/sim/AGENTS.md: denied' },
        }),
        t,
      ),
    ).toBe('無法寫入檔案：open ~/AGENTS.md: denied');
  });
});
