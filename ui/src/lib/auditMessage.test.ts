import { describe, expect, it } from 'vitest';
import { parseFindings, statusNote } from './auditMessage';

describe('parseFindings', () => {
  it('splits the lowered-severity note off the message and keeps the location', () => {
    const [f] = parseFindings(['audit HIGH: Reading accessing AWS credentials (lowered from CRITICAL: Python never runs comments or docstrings) (hook.py:3)'], false);
    expect(f).toMatchObject({ severity: 'HIGH', message: 'Reading accessing AWS credentials', where: 'hook.py:3', lowered: true });
  });
});

describe('statusNote', () => {
  it('reads the skill name and value off a known audit status line', () => {
    expect(statusNote('pdf: audit findings at/above block threshold (CRITICAL); proceeding due to --force')).toMatchObject({ name: 'pdf', key: 'forced', value: 'CRITICAL' });
  });

  it('keeps an unknown line as it is', () => {
    expect(statusNote('pdf: failed to write metadata: disk full')).toEqual({ text: 'pdf: failed to write metadata: disk full' });
  });
});
