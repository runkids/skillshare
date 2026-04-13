import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import type { ManagedPreview } from '../api/client';
import CompiledPreviewCard from './CompiledPreviewCard';

describe('CompiledPreviewCard', () => {
  it('renders clean previews even when the API omits warnings', () => {
    const preview = {
      target: 'claude',
      files: [
        {
          path: '/tmp/home/.claude/rules/e2e-ui-test.md',
          content: '# E2E UI Test',
          format: 'markdown',
        },
      ],
    } as ManagedPreview;

    render(<CompiledPreviewCard preview={preview} />);

    expect(screen.getByText('Preview for claude')).toBeInTheDocument();
    expect(screen.getByText('1 file')).toBeInTheDocument();
    expect(screen.getByText('/tmp/home/.claude/rules/e2e-ui-test.md')).toBeInTheDocument();
  });
});
