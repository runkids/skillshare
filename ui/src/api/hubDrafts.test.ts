import { describe, expect, it } from 'vitest';
import { hubAddCommand } from './hubDrafts';

describe('Hub sharing command', () => {
  it('quotes shell metacharacters as data', () => {
    expect(hubAddCommand('https://host/hub.json', "team's $(touch /tmp/no)"))
      .toBe(`skillshare hub add 'https://host/hub.json' --label 'team'"'"'s $(touch /tmp/no)'`);
  });
});
