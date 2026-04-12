import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { describe, expect, it, vi } from 'vitest';
import SkillFrontmatterWorkspace, { type FrontmatterWorkspaceEntry } from './SkillFrontmatterWorkspace';

function createEntry(overrides: Partial<FrontmatterWorkspaceEntry> = {}): FrontmatterWorkspaceEntry {
  return {
    id: 'field-1',
    key: 'argument-hint',
    value: '[tag-version]',
    isCustom: false,
    isStructured: false,
    isBoolean: false,
    error: null,
    ...overrides,
  };
}

describe('SkillFrontmatterWorkspace', () => {
  it('shows add controls for unused built-in fields and a Used state for active ones', async () => {
    const user = userEvent.setup();
    const onAddBuiltInField = vi.fn();

    render(
      <SkillFrontmatterWorkspace
        mode="edit"
        builtInEntries={[createEntry()]}
        additionalEntries={[]}
        onAddBuiltInField={onAddBuiltInField}
        onAddCustomField={vi.fn()}
        onRemoveField={vi.fn()}
        onChangeFieldValue={vi.fn()}
        onChangeFieldKey={vi.fn()}
      />,
    );

    expect(screen.getByRole('heading', { name: 'YAML Frontmatter' })).toBeInTheDocument();
    expect(screen.getByText('Used')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /add name/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /add description/i })).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: /add disable-model-invocation/i }));
    expect(onAddBuiltInField).toHaveBeenCalledWith('disable-model-invocation');
  });

  it('emits remove events for active fields and exposes a custom add card', async () => {
    const user = userEvent.setup();
    const onRemoveField = vi.fn();
    const onAddCustomField = vi.fn();

    render(
      <SkillFrontmatterWorkspace
        mode="edit"
        builtInEntries={[createEntry()]}
        additionalEntries={[]}
        onAddBuiltInField={vi.fn()}
        onAddCustomField={onAddCustomField}
        onRemoveField={onRemoveField}
        onChangeFieldValue={vi.fn()}
        onChangeFieldKey={vi.fn()}
      />,
    );

    await user.click(screen.getByRole('button', { name: /remove argument-hint/i }));
    expect(onRemoveField).toHaveBeenCalledWith('field-1');

    await user.click(screen.getByRole('button', { name: /add custom frontmatter/i }));
    expect(onAddCustomField).toHaveBeenCalled();
  });

  it('emits value changes using stable field ids for standard fields', async () => {
    const user = userEvent.setup();
    const onChangeFieldValue = vi.fn();

    function Harness() {
      const [entry, setEntry] = useState(createEntry({ id: 'builtin-1', value: '' }));

      return (
        <SkillFrontmatterWorkspace
          mode="edit"
          builtInEntries={[entry]}
          additionalEntries={[]}
          onAddBuiltInField={vi.fn()}
          onAddCustomField={vi.fn()}
          onRemoveField={vi.fn()}
          onChangeFieldValue={(id, value) => {
            onChangeFieldValue(id, value);
            setEntry((current) => (current.id === id ? { ...current, value } : current));
          }}
          onChangeFieldKey={vi.fn()}
        />
      );
    }

    render(<Harness />);

    await user.type(screen.getByRole('textbox', { name: /argument-hint frontmatter value/i }), 'abc');
    expect(onChangeFieldValue).toHaveBeenLastCalledWith('builtin-1', 'abc');
  });

  it('emits key changes for built-in fields so they can be renamed into custom fields', async () => {
    const user = userEvent.setup();
    const onChangeFieldKey = vi.fn();

    function Harness() {
      const [entry, setEntry] = useState(createEntry({
        id: 'builtin-rename',
        key: 'argument-hint',
      }));

      return (
        <SkillFrontmatterWorkspace
          mode="edit"
          builtInEntries={[entry]}
          additionalEntries={[]}
          onAddBuiltInField={vi.fn()}
          onAddCustomField={vi.fn()}
          onRemoveField={vi.fn()}
          onChangeFieldValue={vi.fn()}
          onChangeFieldKey={(id, nextKey) => {
            onChangeFieldKey(id, nextKey);
            setEntry((current) => (current.id === id ? { ...current, key: nextKey } : current));
          }}
        />
      );
    }

    render(<Harness />);

    const fieldNameInput = screen.getByRole('textbox', { name: 'argument-hint frontmatter key' });
    await user.clear(fieldNameInput);
    await user.type(fieldNameInput, 'renamed-note');

    expect(onChangeFieldKey).toHaveBeenLastCalledWith('builtin-rename', 'renamed-note');
  });

  it('uses the fallback custom value label when the custom key is blank', () => {
    render(
      <SkillFrontmatterWorkspace
        mode="edit"
        builtInEntries={[]}
        additionalEntries={[createEntry({
          id: 'custom-blank',
          key: '',
          value: '',
          isCustom: true,
        })]}
        onAddBuiltInField={vi.fn()}
        onAddCustomField={vi.fn()}
        onRemoveField={vi.fn()}
        onChangeFieldValue={vi.fn()}
        onChangeFieldKey={vi.fn()}
      />,
    );

    expect(screen.getByRole('textbox', { name: 'custom frontmatter value' })).toBeInTheDocument();
  });

  it('renders boolean controls and emits stable-id callbacks when toggled', async () => {
    const user = userEvent.setup();
    const onChangeFieldValue = vi.fn();

    render(
      <SkillFrontmatterWorkspace
        mode="edit"
        builtInEntries={[createEntry({
          id: 'bool-1',
          key: 'disable-model-invocation',
          value: 'false',
          isBoolean: true,
        })]}
        additionalEntries={[]}
        onAddBuiltInField={vi.fn()}
        onAddCustomField={vi.fn()}
        onRemoveField={vi.fn()}
        onChangeFieldValue={onChangeFieldValue}
        onChangeFieldKey={vi.fn()}
      />,
    );

    const trueButton = screen.getByRole('button', { name: 'true' });
    const falseButton = screen.getByRole('button', { name: 'false' });

    expect(trueButton.className).toContain('cursor-pointer');
    expect(falseButton.className).toContain('cursor-pointer');
    expect(trueButton).toHaveAttribute('aria-pressed', 'false');
    expect(falseButton).toHaveAttribute('aria-pressed', 'true');

    await user.click(trueButton);
    expect(onChangeFieldValue).toHaveBeenCalledWith('bool-1', 'true');
  });

  it('leaves blank boolean values unselected until the user chooses true or false', async () => {
    const user = userEvent.setup();
    const onChangeFieldValue = vi.fn();

    render(
      <SkillFrontmatterWorkspace
        mode="edit"
        builtInEntries={[createEntry({
          id: 'bool-blank',
          key: 'disable-model-invocation',
          value: '',
          isBoolean: true,
        })]}
        additionalEntries={[]}
        onAddBuiltInField={vi.fn()}
        onAddCustomField={vi.fn()}
        onRemoveField={vi.fn()}
        onChangeFieldValue={onChangeFieldValue}
        onChangeFieldKey={vi.fn()}
      />,
    );

    const trueButton = screen.getByRole('button', { name: 'true' });
    const falseButton = screen.getByRole('button', { name: 'false' });

    expect(trueButton).toHaveAttribute('aria-pressed', 'false');
    expect(falseButton).toHaveAttribute('aria-pressed', 'false');

    await user.click(trueButton);
    expect(onChangeFieldValue).toHaveBeenCalledWith('bool-blank', 'true');

    await user.click(falseButton);
    expect(onChangeFieldValue).toHaveBeenCalledWith('bool-blank', 'false');
  });

  it('hides edit-only actions in read mode', () => {
    render(
      <SkillFrontmatterWorkspace
        mode="read"
        builtInEntries={[createEntry()]}
        additionalEntries={[]}
        onAddBuiltInField={vi.fn()}
        onAddCustomField={vi.fn()}
        onRemoveField={vi.fn()}
        onChangeFieldValue={vi.fn()}
        onChangeFieldKey={vi.fn()}
      />,
    );

    expect(screen.queryByRole('button', { name: /remove argument-hint/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /add custom frontmatter/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /add disable-model-invocation/i })).not.toBeInTheDocument();
  });

  it('does not switch from input to textarea only because the value becomes long', async () => {
    const user = userEvent.setup();

    function Harness() {
      const [entry, setEntry] = useState<FrontmatterWorkspaceEntry>(
        createEntry({ id: 'builtin-1', value: '' }),
      );

      return (
        <SkillFrontmatterWorkspace
          mode="edit"
          builtInEntries={[entry]}
          additionalEntries={[]}
          onAddBuiltInField={vi.fn()}
          onAddCustomField={vi.fn()}
          onRemoveField={vi.fn()}
          onChangeFieldValue={(id, value) => {
            setEntry((current) => (current.id === id ? { ...current, value } : current));
          }}
          onChangeFieldKey={vi.fn()}
        />
      );
    }

    render(<Harness />);

    const editor = screen.getByRole('textbox', { name: /argument-hint frontmatter value/i });
    expect(editor.tagName).toBe('INPUT');

    await user.type(editor, 'this value gets much longer than sixty characters but should stay a single line editor');

    expect(screen.getByRole('textbox', { name: /argument-hint frontmatter value/i }).tagName).toBe('INPUT');
  });

  it('keeps blank active built-ins visually aligned with the Used reference state', () => {
    render(
      <SkillFrontmatterWorkspace
        mode="edit"
        builtInEntries={[createEntry({ value: '' })]}
        additionalEntries={[]}
        onAddBuiltInField={vi.fn()}
        onAddCustomField={vi.fn()}
        onRemoveField={vi.fn()}
        onChangeFieldValue={vi.fn()}
        onChangeFieldKey={vi.fn()}
      />,
    );

    const usedPill = screen.getByText('Used');
    const referenceCard = usedPill.closest('article');

    expect(referenceCard).toHaveClass('border-muted-dark/40');
    expect(referenceCard).toHaveClass('bg-surface');
    expect(screen.queryByRole('button', { name: /add argument-hint/i })).not.toBeInTheDocument();
  });

  it('can render the agent frontmatter reference schema', async () => {
    const user = userEvent.setup();
    const onAddBuiltInField = vi.fn();

    render(
      <SkillFrontmatterWorkspace
        mode="edit"
        schema="agent"
        builtInEntries={[]}
        additionalEntries={[]}
        onAddBuiltInField={onAddBuiltInField}
        onAddCustomField={vi.fn()}
        onRemoveField={vi.fn()}
        onChangeFieldValue={vi.fn()}
        onChangeFieldKey={vi.fn()}
      />,
    );

    await user.click(screen.getByRole('button', { name: /add permissionmode/i }));
    expect(onAddBuiltInField).toHaveBeenCalledWith('permissionMode');
    expect(screen.queryByRole('button', { name: /add allowed-tools/i })).not.toBeInTheDocument();
  });
});
