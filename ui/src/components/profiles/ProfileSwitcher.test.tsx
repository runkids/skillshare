import { fireEvent, render, screen } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import ProfileSwitcher from './ProfileSwitcher';

vi.mock('../../i18n', () => ({ useT: () => (key: string) => key }));

it('hides without profiles or state and opens a preview instead of writing', () => {
  // jsdom has no scrollIntoView, which the dropdown calls on its focused option.
  HTMLElement.prototype.scrollIntoView = vi.fn();
  const onChoose = vi.fn();
  const data = { active: '', state: 'none' as const, pending: false, profiles: [] };
  const { rerender } = render(<ProfileSwitcher data={data} onChoose={onChoose} />);
  expect(screen.queryByRole('combobox', { name: 'profiles.choose' })).toBeNull();
  rerender(<ProfileSwitcher data={{ ...data, profiles: [{ name: 'work', description: '', skills: null, selected: 2, unmatched: [], usable: true }] }} onChoose={onChoose} />);
  fireEvent.click(screen.getByRole('combobox', { name: 'profiles.choose' }));
  fireEvent.mouseDown(screen.getByRole('option', { name: /work/ }));
  expect(onChoose).toHaveBeenCalledExactlyOnceWith('work');
});
