import { describe, expect, it } from 'vitest';
import { initialServerDraft } from './mcpServerDraft';

describe('MCP server drafts', () => {
  it('starts from the Pi settings of a server being edited', () => {
    expect(JSON.parse(initialServerDraft({ command: 'docs', piOptions: { exposure: 'direct' } }, 'docs', ['pi'], false).piOptions)).toEqual({ exposure: 'direct' });
  });

  it('leaves Pi out of the targets a new off switch starts with', () => {
    expect(initialServerDraft(undefined, '', ['claude', 'pi'], true).targets).toEqual(['claude']);
  });
});
