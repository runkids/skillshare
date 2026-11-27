import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { I18nProvider } from '../../i18n';
import SharedNameNotices from './SharedNameNotices';

describe('Shared name notices', () => {
  it('names the target, the paths and the shared name, and says which sync under full path names', () => {
    render(
      <I18nProvider>
        <SharedNameNotices
          items={[
            { target: 'claude', name: 'dev', paths: ['backend/dev', 'frontend/dev'], same_name: true },
            { target: 'cursor', name: 'repo-ui', paths: ['_repo/ui', 'repo-ui'], same_name: false },
          ]}
        />
      </I18nProvider>,
    );

    expect(screen.getByText(/claude: backend\/dev and frontend\/dev share the name dev\. All but a top-level one/)).toBeInTheDocument();
    expect(screen.getByText(/cursor: _repo\/ui and repo-ui would both sync as repo-ui/)).toBeInTheDocument();
  });

  it('renders nothing without shared names', () => {
    const { container } = render(
      <I18nProvider>
        <SharedNameNotices items={[]} />
      </I18nProvider>,
    );

    expect(container).toBeEmptyDOMElement();
  });
});
