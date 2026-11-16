import { useState } from 'react';
import { createPortal } from 'react-dom';
import { useQuery } from '@tanstack/react-query';
import { ArrowLeft, Trash2 } from 'lucide-react';
import { hooksApi } from '../../api/hooks';
import { useT } from '../../i18n';
import { shortenHome } from '../../lib/paths';
import { queryKeys } from '../../lib/queryKeys';
import Button from '../Button';
import IconButton from '../IconButton';
import HookBindingEditor from './HookBindingEditor';
import { sameEvents } from './hooksImport';
import type { HookEvents } from './hooksImport';
import { bindingInvalid, bindingToDraft, checkBinding, draftToBinding } from './hooksView';

interface Props {
  agent: string;
  /** The target's label and the hook's title, shown as "Claude / PostToolUse". */
  label: string;
  title: string;
  path?: string;
  /** The dialog's footer, where this editor's actions go while it is open. */
  footerHost: HTMLElement | null;
  /** The hook as the dialog holds it now, and as it was read from the target. */
  events: HookEvents;
  original: HookEvents;
  onCancel: () => void;
  /** The edited events, or null when they are back to what the target has. */
  onDone: (events: HookEvents | null) => void;
  onRemove: () => void;
}

/** Edits a hook in place of the list. Nothing is written here: the result becomes a pending change of the import dialog. */
export default function HooksImportEditor({ agent, label, title, path, footerHost, events, original, onCancel, onDone, onRemove }: Props) {
  const t = useT();
  const { data: catalog } = useQuery({ queryKey: queryKeys.hooksCatalog, queryFn: () => hooksApi.catalog(), staleTime: Infinity });
  const [draft, setDraft] = useState(() => bindingToDraft(agent, { events }));
  const [tabsHost, setTabsHost] = useState<HTMLElement | null>(null);
  const check = checkBinding(agent, draft);
  const done = () => {
    const edited = draftToBinding(agent, draft).events ?? {};
    // The editor may write the same hook in another shape, so compare against the original taken through it too.
    const untouched = draftToBinding(agent, bindingToDraft(agent, { events: original })).events ?? {};
    onDone(sameEvents(edited, untouched) || sameEvents(edited, original) ? null : edited);
  };
  return (
    <section aria-label={title} className="ss-list flex min-h-0 min-w-0 flex-1 animate-[ss-fade-in_.15s_ease-out] flex-col">
      <div className="flex h-[50px] shrink-0 items-center gap-2 pl-2.5 pr-3">
        <IconButton icon={<ArrowLeft size={16} />} label={t('common.back')} onClick={onCancel} />
        <span className="min-w-0 flex-1 truncate">
          <span className="text-ink-2">{label} / </span>
          <span className="font-mono text-[13px] font-semibold">{title}</span>
        </span>
        <span ref={setTabsHost} className="shrink-0" />
      </div>
      <div className="flex min-h-0 flex-1 flex-col overflow-y-auto">
        <HookBindingEditor cards tabsHost={tabsHost} agent={agent} name={title} draft={draft} check={check} onChange={setDraft} disabled={false} catalog={catalog} />
      </div>
      {footerHost && createPortal(
        <>
          <Button variant="ghost" className="!text-bad" onClick={onRemove}><Trash2 size={15} />{t('hooks.importRemoveHook')}</Button>
          <span className="min-w-0 flex-1 truncate text-[13px] text-ink-3">{t('hooks.importEditNote', { path: path ? shortenHome(path) : label })}</span>
          <Button variant="ghost" onClick={onCancel}>{t('common.cancel')}</Button>
          <Button variant="primary" disabled={bindingInvalid(check)} onClick={done}>{t('hooks.importDone')}</Button>
        </>,
        footerHost,
      )}
    </section>
  );
}
