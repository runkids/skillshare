import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import type { ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { TrashedSkill } from '../api/client';
import { api } from '../api/client';
import { ToastProvider } from '../components/Toast';
import { I18nProvider } from '../i18n';
import TrashPage from './TrashPage';

vi.mock('react-virtuoso', () => ({
  Virtuoso: ({
    data,
    itemContent,
  }: {
    data: TrashedSkill[];
    itemContent: (index: number, item: TrashedSkill) => ReactNode;
  }) => (
    <div data-testid="trash-virtual-list">
      {data.slice(0, 2).map((item, index) => (
        <div key={`${item.name}-${item.timestamp}`}>{itemContent(index, item)}</div>
      ))}
    </div>
  ),
}));

vi.mock('../api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api/client')>();
  return {
    ...actual,
    api: {
      ...actual.api,
      listTrash: vi.fn(),
    },
  };
});

function renderTrashPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <I18nProvider>
        <ToastProvider>
          <TrashPage />
        </ToastProvider>
      </I18nProvider>
    </QueryClientProvider>,
  );
}

describe('TrashPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('virtualizes large trash lists instead of rendering every item at once', async () => {
    vi.mocked(api.listTrash).mockResolvedValue({
      totalSize: 30,
      items: [
        trashItem('alpha'),
        trashItem('beta'),
        trashItem('gamma'),
      ],
    });

    renderTrashPage();

    expect(await screen.findByTestId('trash-virtual-list')).toBeInTheDocument();
    expect(screen.getByText('alpha')).toBeInTheDocument();
    expect(screen.getByText('beta')).toBeInTheDocument();
    expect(screen.queryByText('gamma')).not.toBeInTheDocument();
  });
});

function trashItem(name: string): TrashedSkill {
  return {
    name,
    kind: 'skill',
    timestamp: `2026-06-03_10-00-0${name.length}`,
    date: '2026-06-03T10:00:00Z',
    size: 10,
    path: `/trash/${name}`,
  };
}
