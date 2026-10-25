import { describe, expect, it } from 'vitest';
import { buildSteps } from './tourSteps';

describe('buildSteps', () => {
  it('omits the hidden sidebar shortcut target on mobile only', () => {
    const options = { isProjectMode: false, skillCount: 1 };
    expect(buildSteps(options).some((s) => s.id === 'shortcuts-btn')).toBe(true);
    expect(buildSteps({ ...options, isMobile: true }).some((s) => s.id === 'shortcuts-btn')).toBe(false);
  });
});
