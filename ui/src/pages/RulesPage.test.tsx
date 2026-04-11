import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import RulesPage from './RulesPage';

describe('RulesPage', () => {
  function LocationProbe() {
    const location = useLocation();
    return <div>{location.pathname}{location.search}</div>;
  }

  function renderPage(initialEntry = '/rules') {
    return render(
      <MemoryRouter initialEntries={[initialEntry]}>
        <Routes>
          <Route path="/rules" element={<RulesPage />} />
          <Route path="/resources" element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>,
    );
  }

  it('redirects /rules to /resources?tab=rules preserving mode', async () => {
    renderPage('/rules?mode=discovered');

    expect(await screen.findByText('/resources?tab=rules&mode=discovered')).toBeInTheDocument();
  });
});
