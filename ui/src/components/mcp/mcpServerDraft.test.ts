import { describe, expect, it } from 'vitest';
import { initialServerDraft } from './mcpServerDraft';

describe('MCP server drafts', () => {
  it('starts from the Pi settings of a server being edited', () => {
    expect(JSON.parse(initialServerDraft({ command: 'docs', piOptions: { exposure: 'direct' } }, 'docs', ['pi'], false).piOptions)).toEqual({ exposure: 'direct' });
  });

  it('starts from the tool policy of a server being edited', () => {
    expect(initialServerDraft({ command: 'docs', tools: { deny: ['delete_*'] } }, 'docs', ['pi'], false).tools).toEqual({ deny: ['delete_*'] });
  });

  it('keeps only Agents with a switch in the targets a new off switch starts with', () => {
    expect(initialServerDraft(undefined, '', ['claude', 'cursor', 'pi'], true).targets).toEqual(['claude', 'pi']);
  });
});
