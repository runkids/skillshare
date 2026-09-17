import { describe, expect, it } from 'vitest';
import { formatAgentDisplayName, formatSkillDisplayName } from './resourceNames';

describe('resourceNames', () => {
  it('formats nested agent flat names as slash paths without markdown suffix', () => {
    expect(formatAgentDisplayName('demo__code-archaeologist.md')).toBe('demo/code-archaeologist');
  });

  it('formats top-level agent flat names without markdown suffix', () => {
    expect(formatAgentDisplayName('reviewer.md')).toBe('reviewer');
  });

  it('formats nested skill flat names as slash paths', () => {
    expect(formatSkillDisplayName('_team__frontend__ui')).toBe('_team/frontend/ui');
  });
});
