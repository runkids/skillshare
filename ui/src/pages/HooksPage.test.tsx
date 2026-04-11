import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import HooksPage from './HooksPage';

describe('HooksPage', () => {
  function LocationProbe() {
    const location = useLocation();
    return <div>{location.pathname}{location.search}</div>;
  }

  function renderPage(initialEntry = '/hooks') {
    return render(
      <MemoryRouter initialEntries={[initialEntry]}>
        <Routes>
          <Route path="/hooks" element={<HooksPage />} />
          <Route path="/resources" element={<LocationProbe />} />
        </Routes>
      </MemoryRouter>,
    );
  }

  it('redirects /hooks to /resources?tab=hooks preserving mode', async () => {
    renderPage('/hooks?mode=discovered');

    expect(await screen.findByText('/resources?tab=hooks&mode=discovered')).toBeInTheDocument();
  });
});
