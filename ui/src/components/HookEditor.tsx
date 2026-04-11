import { useEffect, useRef, useState, type ReactNode } from 'react';
import Card from './Card';
import HandButton from './HandButton';
import { HandInput, HandSelect, HandTextarea } from './HandInput';
import {
  createEmptyHookEditorHandlerValue,
  type HookEditorHandlerValue,
  type HookEditorValue,
} from './hookEditorState';

interface HookEditorProps {
  value: HookEditorValue;
  onChange: (value: HookEditorValue) => void;
  onSave: () => void;
  saving?: boolean;
  status?: string | null;
  submitLabel?: string;
  deleteAction?: ReactNode;
}

const handlerTypeOptions = [
  { value: 'command', label: 'command' },
  { value: 'http', label: 'http' },
  { value: 'prompt', label: 'prompt' },
  { value: 'agent', label: 'agent' },
] as const;

const REMOVE_ANIMATION_MS = 180;

export default function HookEditor({
  value,
  onChange,
  onSave,
  saving = false,
  status,
  submitLabel = 'Save Hook',
  deleteAction,
}: HookEditorProps) {
  const [removingHandlerIndex, setRemovingHandlerIndex] = useState<number | null>(null);
  const removalTimerRef = useRef<number | null>(null);
  const valueRef = useRef(value);

  useEffect(() => {
    valueRef.current = value;
  }, [value]);

  useEffect(() => {
    return () => {
      if (removalTimerRef.current !== null) {
        window.clearTimeout(removalTimerRef.current);
      }
    };
  }, []);

  const updateHandler = (index: number, next: HookEditorHandlerValue) => {
    onChange({
      ...value,
      handlers: value.handlers.map((handler, handlerIndex) => (handlerIndex === index ? next : handler)),
    });
  };

  const addHandler = () => {
    onChange({
      ...value,
      handlers: [...value.handlers, createEmptyHookEditorHandlerValue()],
    });
  };

  const removeHandler = (index: number) => {
    if (removalTimerRef.current !== null) {
      return;
    }

    setRemovingHandlerIndex(index);
    removalTimerRef.current = window.setTimeout(() => {
      const currentValue = valueRef.current;
      const nextHandlers = currentValue.handlers.length === 1
        ? [createEmptyHookEditorHandlerValue()]
        : currentValue.handlers.filter((_, handlerIndex) => handlerIndex !== index);

      removalTimerRef.current = null;
      setRemovingHandlerIndex(null);
      onChange({
        ...currentValue,
        handlers: nextHandlers,
      });
    }, REMOVE_ANIMATION_MS);
  };

  return (
    <Card className="space-y-4">
      <form
        className="space-y-4"
        onSubmit={(event) => {
          event.preventDefault();
          onSave();
        }}
      >
        <div className="grid gap-4 md:grid-cols-3">
          <HandInput
            label="Tool"
            value={value.tool}
            onChange={(event) => onChange({ ...value, tool: event.target.value })}
            placeholder="claude"
          />
          <HandInput
            label="Event"
            value={value.event}
            onChange={(event) => onChange({ ...value, event: event.target.value })}
            placeholder="PreToolUse"
          />
          <HandInput
            label="Matcher"
            value={value.matcher}
            onChange={(event) => onChange({ ...value, matcher: event.target.value })}
            placeholder="Bash"
          />
        </div>

        <div className="space-y-4">
          <div className="flex items-center justify-between gap-3">
            <h3 className="text-lg text-pencil" style={{ fontFamily: 'var(--font-heading)' }}>
              Handlers
            </h3>
            <HandButton type="button" size="sm" variant="secondary" onClick={addHandler}>
              Add Handler
            </HandButton>
          </div>

          <div className="space-y-4">
            {value.handlers.map((handler, index) => (
              <Card
                key={index}
                variant="outlined"
                className={`space-y-4 animate-sketch-in ${
                  removingHandlerIndex === index ? 'pointer-events-none opacity-0 translate-x-3 scale-[0.98]' : ''
                }`}
                style={{
                  transition: `opacity ${REMOVE_ANIMATION_MS}ms ease, transform ${REMOVE_ANIMATION_MS}ms ease, box-shadow 100ms ease`,
                  ...(removingHandlerIndex === index
                    ? { opacity: 0, transform: 'translateX(12px) scale(0.98)' }
                    : {}),
                }}
              >
                <div className="flex items-center justify-between gap-3">
                  <h4 className="text-base text-pencil" style={{ fontFamily: 'var(--font-heading)' }}>
                    Handler {index + 1}
                  </h4>
                  <HandButton
                    type="button"
                    size="sm"
                    variant="ghost"
                    disabled={removingHandlerIndex !== null}
                    onClick={() => removeHandler(index)}
                  >
                    Remove
                  </HandButton>
                </div>

                <div className="grid gap-4 md:grid-cols-2">
                  <HandSelect
                    label="Type"
                    value={handler.type}
                    onChange={(type) => updateHandler(index, { ...handler, type: type as HookEditorHandlerValue['type'] })}
                    options={[...handlerTypeOptions]}
                  />
                  <HandInput
                    label="Status Message"
                    value={handler.statusMessage}
                    onChange={(event) => updateHandler(index, { ...handler, statusMessage: event.target.value })}
                    placeholder="Hook finished"
                  />
                </div>

                <div className="grid gap-4 md:grid-cols-2">
                  <HandInput
                    label="Command"
                    value={handler.command}
                    onChange={(event) => updateHandler(index, { ...handler, command: event.target.value })}
                    placeholder="./bin/check"
                  />
                  <HandInput
                    label="URL"
                    value={handler.url}
                    onChange={(event) => updateHandler(index, { ...handler, url: event.target.value })}
                    placeholder="https://example.com/hook"
                  />
                </div>

                <HandTextarea
                  label="Prompt"
                  value={handler.prompt}
                  onChange={(event) => updateHandler(index, { ...handler, prompt: event.target.value })}
                  placeholder="Review the action"
                  rows={5}
                  className="resize-y"
                  style={{ fontFamily: 'var(--font-hand)' }}
                />

                <div className="grid gap-4 md:grid-cols-2">
                  <HandInput
                    label="Timeout"
                    value={handler.timeout}
                    onChange={(event) => updateHandler(index, { ...handler, timeout: event.target.value })}
                    placeholder="30s"
                  />
                  <HandInput
                    label="Timeout Sec"
                    type="text"
                    value={handler.timeoutSec}
                    onChange={(event) => updateHandler(index, { ...handler, timeoutSec: event.target.value })}
                    placeholder="30"
                    inputMode="numeric"
                    pattern="[0-9]*"
                  />
                </div>
              </Card>
            ))}
          </div>
        </div>

        {status && <p className="text-sm text-pencil-light">{status}</p>}

        <div className="flex flex-wrap gap-3">
          <HandButton type="submit" size="sm" disabled={saving}>
            {saving ? 'Working...' : submitLabel}
          </HandButton>
          {deleteAction}
        </div>
      </form>
    </Card>
  );
}
