import { useState, type ReactNode } from 'react';
import { Info, X } from 'lucide-react';
import { mcpApi, mcpTargets, type MCPOverride, type MCPToolPolicy } from '../../api/mcp';
import { useT } from '../../i18n';
import { shortenHome } from '../../lib/paths';
import { scalarText } from '../../lib/scalarText';
import Button from '../Button';
import DialogShell from '../DialogShell';
import IconButton from '../IconButton';
import { Select } from '../Input';
import { useToast } from '../Toast';
import { TargetToggles } from './TargetPicker';
import ToolPolicyFields from './ToolPolicyFields';
import {
  cleanToolPolicy,
  describeEndpoint,
  describeError,
  hasToolPolicy,
  piExposures,
  targetLabel,
  toolSummary,
} from './mcpView';

type MCPList = Awaited<ReturnType<typeof mcpApi.list>>;

interface Props {
  /** The shared server's name. */
  name: string;
  /** The mcp.projects root the override is for. */
  root: string;
  source: MCPList['source'];
  /** Each Agent's config file in this project; only these Agents are offered. */
  paths: Record<string, string>;
  onClose: () => void;
  onSaved: () => void;
}

/**
 * What one project changes in a shared server it uses. The connection stays the MCP page's,
 * so it is shown, not edited; the project chooses its Agents, Pi's tool exposure and tools.
 */
export default function MCPOverrideDialog({ name, root, source, paths, onClose, onSaved }: Props) {
  const t = useT();
  const { toast } = useToast();
  const server = source.servers[name] ?? {};
  const project = source.projects?.[root];
  // Settings the dialog does not edit, such as other Agents' options, are kept as they are.
  const saved: MCPOverride = project?.overrides?.[name] ?? {};
  // Global-only Agents have no file in a project, so the project cannot send the server there.
  const offered: readonly string[] = mcpTargets.filter((x) => paths[x]);
  const inherited = server.targets ?? project?.targets ?? source.targets ?? [];
  const [targets, setTargets] = useState<string[] | undefined>(saved.targets);
  const [exposure, setExposure] = useState(scalarText(saved.piOptions?.exposure));
  const [tools, setTools] = useState<MCPToolPolicy | undefined>(saved.tools);
  const [busy, setBusy] = useState(false);

  const loaded = targets ?? inherited;
  // Agents saved earlier but not offered here, such as ones picked on the other page, stay listed and saved.
  const shown = [...offered, ...(targets ?? []).filter((x) => !offered.includes(x))];
  const sharedExposure = scalarText(server.piOptions?.exposure ?? 'codemode');
  // Pi refuses a tool policy next to toolExposure, and an override cannot take a key away.
  const toolsLocked =
    (saved.piOptions?.toolExposure ?? server.piOptions?.toolExposure) !== undefined;
  const files = loaded.filter((x) => paths[x]).map((x) => shortenHome(paths[x]));

  const next: MCPOverride = { ...saved };
  if (targets) next.targets = shown.filter((x) => targets.includes(x));
  else delete next.targets;
  const piOptions = { ...saved.piOptions };
  if (exposure) piOptions.exposure = exposure;
  else delete piOptions.exposure;
  if (Object.keys(piOptions).length > 0) next.piOptions = piOptions;
  else delete next.piOptions;
  const policy = tools && cleanToolPolicy(tools);
  if (policy) next.tools = policy;
  else delete next.tools;

  const save = async () => {
    setBusy(true);
    try {
      await mcpApi.save({ project: root, name, override: next });
      toast(t('mcp.override.saved', { name, project: shortenHome(root) }), 'success');
      onSaved();
      onClose();
    } catch (e) {
      toast(describeError(t, (e as Error).message), 'error');
    } finally {
      setBusy(false);
    }
  };

  const headers = Object.entries(server.headers ?? {}).map(([key, value]) =>
    typeof value === 'object' ? t('mcp.override.fromEnv', { key, env: value.fromEnv }) : key,
  );
  const env = Object.keys(server.env ?? {});

  return (
    <DialogShell
      open
      onClose={onClose}
      preventClose={busy}
      padding="none"
      maxWidth="2xl"
      ariaLabel={t('mcp.override.label', { name, project: shortenHome(root) })}
    >
      <div className="dh">
        <h2 className="ss-h2 min-w-0 truncate">
          <span className="font-mono">{name}</span>
          <span className="font-normal text-ink-3"> · {shortenHome(root)}</span>
        </h2>
        <IconButton icon={<X size={16} />} label={t('common.close')} onClick={onClose} />
      </div>
      <div className="db flex flex-col gap-5">
        <div className="ss-note inf">
          <Info size={15} className="shrink-0" aria-hidden="true" />
          <span className="flex-1">{t('mcp.override.connectionNote', { name })}</span>
        </div>
        <section className="flex flex-col gap-2">
          <h3 className="text-[13px] font-semibold">{t('mcp.override.inherited')}</h3>
          <dl className="ss-kv">
            <dt>{t('mcp.override.connection')}</dt>
            <dd className="font-mono">{describeEndpoint(server)}</dd>
            {headers.length > 0 && (
              <>
                <dt>{t('mcp.override.headers')}</dt>
                <dd className="font-mono">{headers.join(', ')}</dd>
              </>
            )}
            {env.length > 0 && (
              <>
                <dt>{t('mcp.override.env')}</dt>
                <dd className="font-mono">{env.join(', ')}</dd>
              </>
            )}
          </dl>
        </section>
        <section className="flex flex-col gap-4 pt-4 [border-top:var(--sep)]">
          <h3 className="text-[13px] font-semibold">{t('mcp.override.thisProject')}</h3>
          <Field
            label={t('mcp.targets')}
            set={Boolean(targets)}
            onReset={() => setTargets(undefined)}
            disabled={busy}
          >
            <Select
              ariaLabel={t('mcp.targets')}
              value={targets ? 'choose' : ''}
              disabled={busy}
              // Only Agents this project offers: an account in mcp.targets cannot be an override's target.
              onChange={(value) =>
                setTargets(value ? inherited.filter((x) => offered.includes(x)) : undefined)
              }
              options={[
                {
                  value: '',
                  label: t('mcp.override.follow', {
                    value: inherited.map(targetLabel).join(', ') || t('mcp.override.noAgents'),
                  }),
                },
                { value: 'choose', label: t('mcp.override.chooseAgents') },
              ]}
            />
            {targets && (
              <div className="flex flex-wrap gap-x-6 gap-y-3.5 pt-1">
                <TargetToggles
                  offered={shown}
                  selected={targets}
                  http={Boolean(server.url)}
                  disabled={busy}
                  onToggle={(target, on) =>
                    setTargets(on ? [...targets, target] : targets.filter((x) => x !== target))
                  }
                />
              </div>
            )}
          </Field>
          {(loaded.includes('pi') || exposure) && (
            <Field
              label={t('mcp.piExposure')}
              set={Boolean(exposure)}
              onReset={() => setExposure('')}
              disabled={busy}
              hint={
                exposure ? t('mcp.override.exposureHint', { value: sharedExposure }) : undefined
              }
            >
              <Select
                ariaLabel={t('mcp.piExposure')}
                value={exposure}
                disabled={busy}
                onChange={setExposure}
                options={[
                  { value: '', label: t('mcp.override.follow', { value: sharedExposure }) },
                  ...piExposures.map((value) => ({
                    value,
                    label: value,
                    note: `· ${t(`mcp.piExposure.${value}`)}`,
                  })),
                ]}
              />
            </Field>
          )}
          <Field
            label={t('mcp.tools.title')}
            set={Boolean(policy)}
            onReset={() => setTools(undefined)}
            disabled={busy}
            hint={toolsLocked ? t('mcp.override.toolsLocked') : undefined}
          >
            <Select
              ariaLabel={t('mcp.tools.title')}
              value={tools ? 'choose' : ''}
              disabled={busy || toolsLocked}
              onChange={(value) => setTools(value ? (server.tools ?? {}) : undefined)}
              options={[
                {
                  value: '',
                  label: t('mcp.override.follow', {
                    value: hasToolPolicy(server.tools)
                      ? toolSummary(t, server.tools)
                      : t('mcp.tools.all'),
                  }),
                },
                { value: 'choose', label: t('mcp.override.chooseTools') },
              ]}
            />
            {tools && (
              <ToolPolicyFields
                tools={tools}
                onChange={setTools}
                disabled={busy}
                probe={{ project: root, name, server: { ...server, global: undefined } }}
              />
            )}
          </Field>
        </section>
      </div>
      <div className="df">
        <span className="mr-auto min-w-0 truncate text-[13px] text-ink-2" title={files.join(', ')}>
          {files.length > 0
            ? t('mcp.override.files', { files: files.join(', ') })
            : t('mcp.override.noFiles')}
        </span>
        <Button variant="ghost" onClick={onClose} disabled={busy}>
          {t('common.cancel')}
        </Button>
        <Button variant="primary" onClick={() => void save()} loading={busy}>
          {t('common.save')}
        </Button>
      </div>
    </DialogShell>
  );
}

/** One setting the project can override: its label, a mark and a reset while it is set. */
function Field({
  label,
  set,
  onReset,
  disabled,
  hint,
  children,
}: {
  label: string;
  set: boolean;
  onReset: () => void;
  disabled: boolean;
  hint?: string;
  children: ReactNode;
}) {
  const t = useT();
  return (
    <div className="ss-fld">
      <span className="flex items-center gap-2">
        <span className="text-[13px] font-semibold">{label}</span>
        {set && <span className="ss-tag inf">{t('mcp.override.tag')}</span>}
        {set && (
          <Button variant="link" className="ml-auto min-h-6" onClick={onReset} disabled={disabled}>
            {t('mcp.override.reset')}
          </Button>
        )}
      </span>
      {children}
      {hint && <span className="hp">{hint}</span>}
    </div>
  );
}
