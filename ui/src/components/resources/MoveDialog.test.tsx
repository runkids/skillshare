import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '../../api/client';
import type { MoveItemResult, MoveResult, Skill } from '../../api/client';
import { I18nProvider } from '../../i18n';
import { MoveDialog } from './MoveDialog';

vi.mock('../../api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../api/client')>();
  return { ...actual, api: { ...actual.api, moveResources: vi.fn() } };
});

const skill = (relPath: string, extra: Partial<Skill> = {}): Skill => ({
  name: relPath.split('/').pop()!,
  kind: 'skill',
  flatName: relPath.replace(/\//g, '__'),
  relPath,
  sourcePath: `/s/${relPath}`,
  isInRepo: false,
  ...extra,
});

const ALL = [skill('frontend/pdf'), skill('frontend/docx'), skill('archive/old'), skill('solo')];

const ok = (name: string, to: string, extra: Partial<MoveItemResult> = {}): MoveItemResult =>
  ({ name, success: true, from: name.replace(/__/g, '/'), to, flatName: to.replace(/\//g, '__'), record: true, ...extra });

const result = (results: MoveItemResult[], dryRun = false, warnings: string[] = []): MoveResult => ({
  results,
  summary: { succeeded: results.filter((r) => r.success).length, failed: results.filter((r) => !r.success).length },
  warnings,
  dryRun,
});

function mount(props: Partial<React.ComponentProps<typeof MoveDialog>> = {}) {
  const onClose = vi.fn();
  const onMoved = vi.fn();
  render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <I18nProvider>
        <MoveDialog skills={[ALL[0], ALL[1]]} all={ALL} onClose={onClose} onMoved={onMoved} {...props} />
      </I18nProvider>
    </QueryClientProvider>,
  );
  return { onClose, onMoved };
}

async function pick(user: ReturnType<typeof userEvent.setup>, option: RegExp | string) {
  await user.click(screen.getByRole('combobox', { name: 'Destination' }));
  await user.click(await screen.findByRole('option', { name: option }));
}

describe('MoveDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    HTMLElement.prototype.scrollIntoView = vi.fn();
  });

  it('previews with a dry run, then sends only the chosen destination and names', async () => {
    vi.mocked(api.moveResources).mockImplementation(async (opts) =>
      result(opts.names.map((n) => ok(n, `archive/${n.split('__')[1]}`)), !!opts.dryRun));
    const user = userEvent.setup();
    mount();

    expect(screen.getByRole('button', { name: /^Move 2 skills$/ })).toBeDisabled();
    await pick(user, /^archive/);

    await waitFor(() => expect(api.moveResources).toHaveBeenCalledWith({ names: ['frontend__pdf', 'frontend__docx'], dest: 'archive', dryRun: true }));
    await user.click(await screen.findByRole('button', { name: /^Move 2 skills$/ }));

    await waitFor(() => expect(api.moveResources).toHaveBeenLastCalledWith({ names: ['frontend__pdf', 'frontend__docx'], dest: 'archive', force: undefined }));
  });

  it('says how many skills nested inside the selected ones move with them', async () => {
    vi.mocked(api.moveResources).mockImplementation(async (opts) => result(opts.names.map((n) => ok(n, `archive/${n}`)), !!opts.dryRun));
    const user = userEvent.setup();
    mount({ skills: [skill('suite')], all: [...ALL, skill('suite'), skill('suite/inner'), skill('suite/deep/more')] });

    expect(screen.getByText('2 skills nested inside the selected ones move with them')).toBeInTheDocument();
    await pick(user, /^archive/);
    expect(await screen.findByRole('button', { name: /^Move 3 skills$/ })).toBeEnabled();
  });

  it('sends only the outer skill when one selected skill is inside another', async () => {
    vi.mocked(api.moveResources).mockImplementation(async (opts) => result(opts.names.map((n) => ok(n, `archive/${n}`)), !!opts.dryRun));
    const user = userEvent.setup();
    const outer = skill('suite');
    const inner = skill('suite/inner');
    mount({ skills: [outer, inner, skill('solo')], all: [...ALL, outer, inner] });
    await pick(user, /^archive/);

    await waitFor(() => expect(api.moveResources).toHaveBeenCalledWith({ names: ['suite', 'solo'], dest: 'archive', dryRun: true }));
  });

  it('counts the skills under a moved folder, not the one result', async () => {
    vi.mocked(api.moveResources).mockImplementation(async (opts) => result([ok('frontend', 'archive/frontend', { skills: 4 })], !!opts.dryRun));
    const user = userEvent.setup();
    mount({ skills: undefined, folder: 'frontend', all: ALL });
    await pick(user, /^archive/);
    await user.click(await screen.findByRole('button', { name: 'Move folder' }));

    expect(await screen.findByText('Moved 4 skills')).toBeInTheDocument();
  });

  it('locks Later and Sync now while a "Move anyway" retry runs', async () => {
    let release: (r: MoveResult) => void = () => undefined;
    vi.mocked(api.moveResources).mockImplementation((opts) => {
      if (opts.dryRun) return Promise.resolve(result(opts.names.map((n) => ok(n, `archive/${n}`)), true));
      if (opts.force) return new Promise<MoveResult>((resolve) => { release = resolve; });
      return Promise.resolve(result([
        ok('frontend__pdf', 'archive/pdf'),
        { name: 'frontend__docx', success: false, error: 'y', error_code: 'name_collision' },
      ]));
    });
    const user = userEvent.setup();
    mount();
    await pick(user, /^archive/);
    await user.click(await screen.findByRole('button', { name: /^Move 2 skills$/ }));
    await user.click(await screen.findByRole('button', { name: 'Move anyway' }));

    expect(screen.getByRole('button', { name: 'Later' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Sync Now' })).toBeDisabled();
    release(result([ok('frontend__docx', 'archive/docx')]));
    await waitFor(() => expect(screen.getByRole('button', { name: 'Later' })).toBeEnabled());
  });

  it('sends "." for the source root', async () => {
    vi.mocked(api.moveResources).mockImplementation(async (opts) => result(opts.names.map((n) => ok(n, 'pdf')), !!opts.dryRun));
    const user = userEvent.setup();
    mount({ skills: [ALL[0]] });

    await pick(user, /^Root/);

    await waitFor(() => expect(api.moveResources).toHaveBeenCalledWith({ names: ['frontend__pdf'], dest: '.', dryRun: true }));
  });

  it('shows what the preview refuses and leaves those items out of the move', async () => {
    vi.mocked(api.moveResources).mockImplementation(async (opts) => result([
      ok('frontend__pdf', 'archive/pdf'),
      { name: 'frontend__docx', success: false, error: 'dest exists', error_code: 'dest_exists' },
    ], !!opts.dryRun));
    const user = userEvent.setup();
    mount();

    await pick(user, /^archive/);

    expect(await screen.findByText(/already has an item with this name/)).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: /^Move 1 skill$/ }));
    await waitFor(() => expect(api.moveResources).toHaveBeenLastCalledWith({ names: ['frontend__pdf'], dest: 'archive', force: undefined }));
  });

  it('warns about target filters from the preview without showing backend text', async () => {
    vi.mocked(api.moveResources).mockResolvedValue(result([ok('frontend__pdf', 'archive/pdf')], true, ['target cursor: include "frontend__pdf" will stop matching']));
    const user = userEvent.setup();
    mount({ skills: [ALL[0]] });

    await pick(user, /^archive/);

    expect(await screen.findByText(/may match differently once links are renamed \(1\)/)).toBeInTheDocument();
    expect(screen.queryByText(/will stop matching/)).toBeNull();
  });

  it('offers "Move anyway" only for a name collision, and retries only that item', async () => {
    vi.mocked(api.moveResources).mockImplementation(async (opts) => {
      if (opts.dryRun) return result(opts.names.map((n) => ok(n, `archive/${n}`)), true);
      if (opts.force) return result([ok('frontend__docx', 'archive/docx')]);
      return result([
        { name: 'frontend__pdf', success: false, error: 'x', error_code: 'dest_exists' },
        { name: 'frontend__docx', success: false, error: 'y', error_code: 'name_collision' },
      ]);
    });
    const user = userEvent.setup();
    mount();
    await pick(user, /^archive/);
    await user.click(await screen.findByRole('button', { name: /^Move 2 skills$/ }));

    await screen.findByText('Moved 0, 2 failed');
    const retry = screen.getByRole('button', { name: 'Move anyway' });
    await user.click(retry);

    await waitFor(() => expect(api.moveResources).toHaveBeenLastCalledWith({ names: ['frontend__docx'], dest: 'archive', force: true }));
    await screen.findByText('Moved 1, 1 failed');
    expect(screen.queryByRole('button', { name: 'Move anyway' })).toBeNull();
  });

  it('has no "Move anyway" when nothing collided', async () => {
    vi.mocked(api.moveResources).mockImplementation(async (opts) => opts.dryRun
      ? result(opts.names.map((n) => ok(n, `archive/${n}`)), true)
      : result([{ name: 'frontend__pdf', success: false, error: 'x', error_code: 'dest_exists' }]));
    const user = userEvent.setup();
    mount({ skills: [ALL[0]] });
    await pick(user, /^archive/);
    await user.click(await screen.findByRole('button', { name: /^Move 1 skill$/ }));

    await screen.findByText('Moved 0, 1 failed');
    expect(screen.queryByRole('button', { name: 'Move anyway' })).toBeNull();
    expect(screen.getByRole('button', { name: 'Later' })).toBeInTheDocument();
  });

  it('reports the moved skills and hands them to onMoved', async () => {
    vi.mocked(api.moveResources).mockImplementation(async (opts) => result(opts.names.map((n) => ok(n, 'archive/pdf')), !!opts.dryRun));
    const user = userEvent.setup();
    const { onMoved } = mount({ skills: [ALL[0]] });
    await pick(user, /^archive/);
    await user.click(await screen.findByRole('button', { name: /^Move 1 skill$/ }));

    expect(await screen.findByText('Moved 1 skill')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Sync Now' })).toBeInTheDocument();
    expect(onMoved).toHaveBeenCalledWith([expect.objectContaining({ flatName: 'archive__pdf' })]);
  });

  it('makes a new destination in a step, and Escape goes back to the form instead of closing', async () => {
    vi.mocked(api.moveResources).mockImplementation(async (opts) => result(opts.names.map((n) => ok(n, 'fresh/pdf')), !!opts.dryRun));
    const user = userEvent.setup();
    const { onClose } = mount({ skills: [ALL[0]] });

    await user.click(screen.getByRole('combobox', { name: 'Destination' }));
    await user.click(await screen.findByRole('option', { name: /new folder/i }));
    await user.keyboard('{Escape}');
    expect(await screen.findByRole('combobox', { name: 'Destination' })).toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();

    await user.click(screen.getByRole('combobox', { name: 'Destination' }));
    await user.click(await screen.findByRole('option', { name: /new folder/i }));
    await user.type(screen.getByRole('textbox', { name: /^name$/i }), 'fresh');
    await user.click(screen.getByRole('button', { name: /use this folder/i }));

    await waitFor(() => expect(api.moveResources).toHaveBeenCalledWith({ names: ['frontend__pdf'], dest: 'fresh', dryRun: true }));
  });

  it('keeps the rows that already moved when a retry fails', async () => {
    vi.mocked(api.moveResources).mockImplementation(async (opts) => {
      if (opts.dryRun) return result(opts.names.map((n) => ok(n, `archive/${n}`)), true);
      if (opts.force) throw new Error('network down');
      return result([
        ok('frontend__pdf', 'archive/pdf'),
        { name: 'frontend__docx', success: false, error: 'y', error_code: 'name_collision' },
      ]);
    });
    const user = userEvent.setup();
    mount();
    await pick(user, /^archive/);
    await user.click(await screen.findByRole('button', { name: /^Move 2 skills$/ }));
    await screen.findByText('Moved 1, 1 failed');

    await user.click(screen.getByRole('button', { name: 'Move anyway' }));

    expect(await screen.findByText('The move did not go through.')).toBeInTheDocument();
    expect(screen.getByText('Moved 1, 1 failed')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Sync Now' })).toBeInTheDocument();
  });

  describe('whole folder', () => {
    const REPO = skill('frontend/_acme/skills/x', { isInRepo: true, repoPath: 'frontend/_acme' });

    it('lists what moves with the folder and disables the folder itself as a destination', async () => {
      vi.mocked(api.moveResources).mockImplementation(async (opts) => result([ok('frontend', 'archive/frontend', { skills: 2 })], !!opts.dryRun));
      const user = userEvent.setup();
      mount({ skills: undefined, folder: 'frontend', all: ALL });

      expect(screen.getByRole('dialog', { name: 'Move folder frontend' })).toBeInTheDocument();
      await user.click(screen.getByRole('combobox', { name: 'Destination' }));
      expect(await screen.findByRole('option', { name: /^frontend/ })).toHaveAttribute('aria-disabled', 'true');
      await user.click(screen.getByRole('option', { name: /^archive/ }));

      await waitFor(() => expect(api.moveResources).toHaveBeenCalledWith({ names: ['frontend'], dest: 'archive', dryRun: true }));
      const dialog = screen.getByRole('dialog');
      expect(await within(dialog).findByText('archive/frontend/pdf')).toBeInTheDocument();
      await user.click(within(dialog).getByRole('button', { name: 'Move folder' }));
      await waitFor(() => expect(api.moveResources).toHaveBeenLastCalledWith({ names: ['frontend'], dest: 'archive', force: undefined }));
    });

    it('stays disabled when the destination is where the folder already is', async () => {
      vi.mocked(api.moveResources).mockImplementation(async (opts) => result([
        { name: 'ui', success: false, error: 'same', error_code: 'same_folder' },
      ], !!opts.dryRun));
      const user = userEvent.setup();
      mount({ skills: undefined, folder: 'frontend/ui', all: [...ALL, skill('frontend/ui/x')] });
      await user.click(screen.getByRole('combobox', { name: 'Destination' }));
      const parent = (await screen.findAllByRole('option')).find((o) => o.querySelector('.truncate')?.textContent === 'frontend')!;
      await user.click(parent);

      await waitFor(() => expect(api.moveResources).toHaveBeenCalledWith({ names: ['frontend/ui'], dest: 'frontend', dryRun: true }));
      expect(screen.getByRole('button', { name: 'Move folder' })).toBeDisabled();
    });

    it('is blocked, with a note, when a tracked repo sits inside', async () => {
      vi.mocked(api.moveResources).mockImplementation(async (opts) => result([
        { name: 'frontend', success: false, error: 'tracked', error_code: 'inside_tracked_repo' },
      ], !!opts.dryRun));
      const user = userEvent.setup();
      mount({ skills: undefined, folder: 'frontend', all: [...ALL, REPO] });

      const dialog = screen.getByRole('dialog');
      expect(within(dialog).getByText('_acme')).toBeInTheDocument();
      expect(within(dialog).getByText(/Take _acme out first with skillshare uninstall/)).toBeInTheDocument();
      await pick(user, /^archive/);
      await waitFor(() => expect(api.moveResources).toHaveBeenCalled());
      expect(within(dialog).getByRole('button', { name: 'Move folder' })).toBeDisabled();
    });

    it('is blocked by a refusal from the preview even when the list shows nothing blocked', async () => {
      vi.mocked(api.moveResources).mockImplementation(async (opts) => result([
        { name: 'frontend', success: false, error: 'dest', error_code: 'dest_is_skill' },
      ], !!opts.dryRun));
      const user = userEvent.setup();
      mount({ skills: undefined, folder: 'frontend', all: ALL });
      await pick(user, /^archive/);

      expect(await screen.findByText('The destination is inside another skill.')).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Move folder' })).toBeDisabled();
    });
  });
});
