import { useEffect, useState } from 'react';
import { useToast } from '../components/Toast';

interface EditableFileOptions {
  // Writes the edited text. Return the text actually written when it differs
  // from the edit (config.yaml is normalized on save) so the editor shows it.
  save: (value: string) => Promise<string | void>;
  // Keep the editor as it is when the fetched text is empty.
  skipEmpty?: boolean;
  // Show a failed save somewhere other than a toast.
  onError?: (error: Error) => void;
}

// useEditableFile holds the editor state for one server file: it loads the
// fetched text, tracks whether the edit differs from it, saves, and resets.
export function useEditableFile(data: { raw?: string } | undefined, { save, skipEmpty = false, onError }: EditableFileOptions) {
  const { toast } = useToast();
  const [value, setValue] = useState('');
  const [dirty, setDirty] = useState(false);
  const [saving, setSaving] = useState(false);
  const original = data?.raw ?? '';

  useEffect(() => {
    if (data && (!skipEmpty || data.raw)) {
      setValue(data.raw ?? '');
      setDirty(false);
    }
  }, [data, skipEmpty]);

  // Returns whether the new text differs from the fetched file.
  const change = (next: string) => {
    setValue(next);
    const changed = next !== original;
    setDirty(changed);
    return changed;
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      const written = await save(value);
      if (written !== undefined) setValue(written);
      setDirty(false);
    } catch (e: unknown) {
      if (onError) onError(e as Error);
      else toast((e as Error).message, 'error');
    } finally {
      setSaving(false);
    }
  };

  const reset = () => {
    setValue(original);
    setDirty(false);
  };

  return { value, dirty, saving, change, save: handleSave, reset };
}
