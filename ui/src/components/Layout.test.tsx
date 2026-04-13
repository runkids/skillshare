import { describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import Layout from './Layout';

vi.mock('../context/AppContext', () => ({
  useAppContext: () => ({
    isProjectMode: false,
  }),
}));

vi.mock('../hooks/useGlobalShortcuts', () => ({
  useGlobalShortcuts: () => ({
    modifierHeld: false,
  }),
}));

vi.mock('./ThemePopover', () => ({
  default: () => <div data-testid="theme-popover" />,
}));

vi.mock('./KeyboardShortcutsModal', () => ({
  default: () => null,
}));

vi.mock('./ShortcutHUD', () => ({
  default: () => null,
}));

vi.mock('./UpdateDialog', () => ({
  default: () => null,
}));

vi.mock('./tour', () => ({
  useTour: () => ({
    startTour: vi.fn(),
  }),
}));

describe('Layout', () => {
  function renderLayout() {
    return render(
      <MemoryRouter initialEntries={['/']}>
        <Routes>
          <Route path="/" element={<Layout />}>
            <Route index element={<div>Home</div>} />
          </Route>
        </Routes>
      </MemoryRouter>,
    );
  }

  it('shows resources in the main sidebar without standalone rules or hooks links', () => {
    renderLayout();

    expect(screen.getByRole('link', { name: 'Resources' })).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Rules' })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Hooks' })).not.toBeInTheDocument();
  });
});
