import { describe, expect, it } from 'vitest';
import { parseFindings } from './auditMessage';

describe('parseFindings', () => {
  it('splits the lowered-severity note off the message and keeps the location', () => {
    const [f] = parseFindings(['audit HIGH: Reading accessing AWS credentials (lowered from CRITICAL: Python never runs comments or docstrings) (hook.py:3)'], false);
    expect(f).toMatchObject({ severity: 'HIGH', message: 'Reading accessing AWS credentials', where: 'hook.py:3', lowered: true });
  });
});
