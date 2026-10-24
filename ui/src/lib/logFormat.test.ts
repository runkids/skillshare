import { describe, expect, it } from 'vitest';
import type { LogEntry } from '../api/client';
import { formatLogDetail } from './logFormat';

const entry = (cmd: string, args: Record<string, unknown>, msg?: string): LogEntry =>
  ({ ts: '2026-01-01T00:00:00Z', cmd, args, status: 'ok', msg } as LogEntry);

describe('formatLogDetail', () => {
  it('shows which project changed and how', () => {
    expect(formatLogDetail(entry('project', { action: 'edit', root: '~/work/app', scope: 'cli' }))).toBe('edit ~/work/app, scope=cli');
  });

  it('lists the targets a project conversion found', () => {
    expect(formatLogDetail(entry('project', { action: 'convert', root: '~/work/app', targets: ['claude', 'cursor'], scope: 'ui' }, 'partial')))
      .toBe('convert ~/work/app, targets=claude, cursor, scope=ui — partial');
  });

  it('names the project a sync touched', () => {
    expect(formatLogDetail(entry('sync', { action: 'sync', root: '~/work/app', targets_total: 2, scope: 'cli' })))
      .toBe('root=~/work/app, targets=2, scope=cli');
  });
});
