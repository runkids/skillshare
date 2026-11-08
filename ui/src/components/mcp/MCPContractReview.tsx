import { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { ApiError } from '../../api/http';
import { mcpCheckApi, MCPToolsAcceptanceError, type MCPCheckFinding, type MCPCheckReport, type MCPCheckServer } from '../../api/mcpCheck';
import { useT } from '../../i18n';
import { invalidate } from '../../lib/queryEvents';
import Button from '../Button';
import DialogShell from '../DialogShell';
import { describeError, describeMessage, describeProbeDetail, identityCredentialMessage, parseToolNotice, pinFindingKey, protocolDetail, scopeCredentialKey, scopeCredentialMessage, toolGapKey, visibleMCPText as visible } from './mcpView';

// Exact Skillshare-owned messages from the shared contract/check producers.
// Unknown diagnostics and canonical descriptors are never translated by substring.
const contractMessages = new Map<string, string>([
  ['MCP source identity is unavailable', 'mcp.contract.message.sourceIdentityUnavailable'],
  ['tool contracts could not be compared: MCP source identity is unavailable', 'mcp.contract.message.sourceIdentityUnavailable'],
  ['MCP baseline key is invalid or unreadable; use an Ed25519 PEM key with safe file permissions', 'mcp.contract.message.signingKey'],
  ['signed MCP tool baseline requires CLI verification with --verify-key', 'mcp.contract.message.verifyRequired'],
  ['MCP tool baseline signature could not be verified; supply the correct external verification key', 'mcp.contract.message.signatureInvalid'],
  ['unsigned MCP tool baseline requires explicit --sign-unsigned review before signing', 'mcp.contract.message.unsignedReview'],
  ['accepting signed MCP tool baselines requires a matching --signing-key and --verify-key', 'mcp.contract.message.signerRequired'],
  ['--sign-unsigned requires an existing unsigned MCP tool baseline', 'mcp.contract.message.migrationRequired'],
  ['MCP tool baseline is invalid or unreadable; restore a valid backup or inspect the state file', 'mcp.contract.message.baselineUnreadable'],
  ['tool contracts could not be compared: MCP baseline key is invalid or unreadable; use an Ed25519 PEM key with safe file permissions', 'mcp.contract.message.signingKey'],
  ['tool contracts could not be compared: signed MCP tool baseline requires CLI verification with --verify-key', 'mcp.contract.message.verifyRequired'],
  ['tool contracts could not be compared: MCP tool baseline signature could not be verified; supply the correct external verification key', 'mcp.contract.message.signatureInvalid'],
  ['tool contracts could not be compared: unsigned MCP tool baseline requires explicit --sign-unsigned review before signing', 'mcp.contract.message.unsignedReview'],
  ['tool contracts could not be compared: accepting signed MCP tool baselines requires a matching --signing-key and --verify-key', 'mcp.contract.message.signerRequired'],
  ['tool contracts could not be compared: --sign-unsigned requires an existing unsigned MCP tool baseline', 'mcp.contract.message.migrationRequired'],
  ['tool added; review required', 'mcp.contract.message.toolAdded'],
  ['tool removed; review required', 'mcp.contract.message.toolRemoved'],
  ['contract changed; comparison unavailable', 'mcp.contract.message.comparisonUnavailable'],
  ['output contract changed; review required', 'mcp.contract.message.outputChanged'],
  ['model-visible text changed; review required', 'mcp.contract.message.proseChanged'],
  ['metadata changed; review required', 'mcp.contract.message.metadataChanged'],
  ['contract fingerprint changed; review required', 'mcp.contract.message.fingerprintChanged'],
  ['schema structure changed; review required', 'mcp.contract.message.schemaStructure'],
  ['schema text changed; review required', 'mcp.contract.message.schemaText'],
  ['object property changed; review required', 'mcp.contract.message.propertyChanged'],
  ['schema constraint changed; review required', 'mcp.contract.message.constraintChanged'],
  ['contract text matched an audit rule; review required', 'mcp.contract.message.auditMatched'],
  ['complete valid tool contracts are required before review', 'mcp.contract.message.completeRequired'],
  ['tool contracts contain a configured credential; no review is available', 'mcp.contract.message.currentCredential'],
  [scopeCredentialMessage, scopeCredentialKey],
  ['stored tool contracts contain a configured credential; no review is available', 'mcp.contract.message.storedCredential'],
  ['tool contract audit was incomplete; review cannot be accepted', 'mcp.contract.message.auditIncomplete'],
  ['baseline trust requires fromEnv references for every environment, header and bearer value', 'mcp.contract.message.referencesRequired'],
  ['tool contract text has audit findings; inspect the complete review', 'mcp.contract.message.auditFindings'],
  ['connection identity is unavailable; run a new complete live review', 'mcp.contract.message.identityUnavailable'],
  ['accepting tool contracts requires a live review of one named server and its revision', 'mcp.contract.message.namedReviewRequired'],
  ['tool contracts have no approved baseline; inspect the complete inventory and revision before accepting', 'mcp.contract.message.noBaseline'],
  ['legacy tool hashes require explicit review of complete contracts and connection identity', 'mcp.contract.message.legacyReview'],
  ['tool contracts or connection identity changed; inspect all changes before accepting', 'mcp.contract.message.driftReview'],
  ['tool baseline was not accepted; a complete valid live observation and current review revision are required', 'mcp.contract.message.acceptanceRequired'],
  ['tool baseline could not be persisted; no acceptance was recorded', 'mcp.contract.message.persistFailed'],
  ['MCP review is stale; run a new live review before accepting', 'mcp.contract.message.staleReview'],
  ['tool contracts could not be compared: MCP tool baseline is invalid or unreadable; restore a valid backup or inspect the state file', 'mcp.contract.message.baselineUnreadable'],
  ['accepted the reviewed complete tool contracts and connection identity', 'mcp.contract.message.acceptedComplete'],
  ['url is not a valid HTTP(S) URL', 'mcp.contract.message.invalidURL'],
  ['not synced yet; the next sync removes it from this Agent', 'mcp.contract.message.removePending'],
  ['sign-in required (HTTP 401); skillshare does not sign in', 'mcp.contract.message.authRequired'],
  ['inventory was not accepted; review the check report', 'mcp.contract.message.requestRefused'],
  ['review is stale; probe again before accepting', 'mcp.contract.message.requestStale'],
  ['name, root and revision are required', 'mcp.contract.message.requestFields'],
  ['invalid MCP check request', 'mcp.contract.message.invalidRequest'],
  ['invalid MCP acceptance response', 'mcp.contract.message.invalidAcceptanceResponse'],
  ['MCP acceptance outcome is unknown; probe again before accepting', 'mcp.contract.message.unknownAcceptance'],
  ['MCP settings are available only when the dashboard is opened by localhost or an IP address, not a domain name', 'mcp.contract.message.localOnly'],
]);
type ReviewT = ReturnType<typeof useT>;
function ownedMessage(t: ReviewT, message: string): string {
  const suffix = '; location truncated, inspect full contract';
  const truncated = message.endsWith(suffix);
  const key = contractMessages.get(truncated ? message.slice(0, -suffix.length) : message);
  return key ? t(key) + (truncated ? t('mcp.contract.message.locationTruncated') : '') : message;
}

function findingText(t: ReviewT, f: MCPCheckFinding): string {
  // This exact unwrapped refusal is emitted by Check's preflight privacy gate.
  if (f.check === 'live' && f.level === 'error' && f.message === identityCredentialMessage) return describeProbeDetail(t, f.message);
  const livePrefix = 'live probe failed: ';
  if (f.check === 'live' && f.level === 'error' && f.message.startsWith(livePrefix)) {
    const detail = f.message.slice(livePrefix.length);
    return t('mcp.contract.protocol.failed', { detail: protocolDetail(t, detail) ?? detail });
  }
  if (f.check === 'tool-change' || f.check === 'tool-audit') return ownedMessage(t, f.message);
  if (f.check === 'env' && f.subject) {
    const suffix = ` reads ${f.subject}, which is not set`;
    if (f.message.endsWith(suffix)) return t('mcp.contract.message.envMissing', { field: f.message.slice(0, -suffix.length), subject: f.subject });
  }
  if (f.check === 'command' && f.subject && f.message === `command ${f.subject} was not found on PATH`) return t('mcp.check.finding.command', { subject: f.subject });
  if (f.check === 'dns' && f.subject && f.message === `host ${f.subject} did not resolve`) return t('mcp.check.finding.dns', { subject: f.subject });
  if (f.check === 'pin' && f.subject) return t(pinFindingKey(f), { subject: f.subject });
  if (f.check === 'sync' && f.message === 'not synced yet; run skillshare sync mcp') return t('mcp.check.finding.notSynced');
  if (f.check === 'tools') {
    const tools = parseToolNotice(f.message);
    if (tools) return t('mcp.tools.notAppliedHere', { parts: tools.gaps.map((gap) => t(toolGapKey(gap))).join(t('mcp.tools.partSeparator')) });
  }
  if (f.check === 'live' && f.subject && f.message === `sign-in required (HTTP 401): resource metadata at ${f.subject}; skillshare does not sign in`) return t('mcp.contract.message.authMetadata', { subject: f.subject });
  if (f.check === 'sync') return describeMessage(t, ownedMessage(t, f.message));
  if (f.check === 'url' || f.check === 'live') return ownedMessage(t, f.message);
  if (f.check === 'client-rule') return describeError(t, describeMessage(t, f.message));
  return f.message;
}

function Snapshot({ name, text }: { name: string; text: string }) {
  const t = useT();
  const [expanded, setExpanded] = useState(false);
  return <div className="ss-box p-3">
    <Button size="sm" variant="ghost" aria-expanded={expanded} onClick={() => setExpanded(!expanded)}>
      {t(expanded ? 'mcp.contract.hideSnapshot' : 'mcp.contract.showSnapshot')} · {visible(name)}
    </Button>
    {expanded && <pre className="ss-pre max-h-[50vh] overflow-auto whitespace-pre-wrap break-all">{visible(text)}</pre>}
  </div>;
}

function ServerReview({ server, blocked, running, accepted, onAccept }: {
  server: MCPCheckServer; blocked: boolean; running: boolean; accepted: boolean; onAccept: () => void;
}) {
  const t = useT();
  const c = server.contract;
  const complete = Boolean(c?.revision && c.tools && server.live && server.ok &&
    ['untrusted', 'legacy', 'unchanged', 'drifted'].includes(c.status) &&
    Object.keys(c.tools).length === server.live.tools &&
    Object.values(c.tools).every((tool) => typeof tool.definitionText === 'string' && tool.definitionText.length > 0));
  return <section className="flex flex-col gap-3">
    <div className="ss-sec"><h3 className="font-mono break-all">{visible(server.name)} · {server.project ? visible(server.project) : t('mcp.contract.ownScope')}</h3></div>
    {c && <>
      <div className="ss-note inf"><span>{t(`mcp.contract.status.${accepted ? 'accepted' : c.status}`)}</span></div>
      {c.policy && <p className="text-sm text-ink-2 break-all">{t('mcp.contract.policy', { policy: visible(c.policy) })}</p>}
      {c.identityChanged && <div className="ss-note warn">{t('mcp.contract.identityChanged')}</div>}
      {c.changes && c.changes.length > 0 && <div className="ss-list">
        {c.changes.map((change, i) => <div className="ss-r flex-wrap gap-2 break-all" key={i}>
          <span className="ss-tag">{visible(change.grade)}</span>
          <span className="font-mono">{visible(change.tool)} · {visible(change.area)} · {visible(change.path)}</span>
          <span>{visible(ownedMessage(t, change.message))}</span>
        </div>)}
      </div>}
      {(c.omitted ?? 0) > 0 && <div className="ss-note warn">{t('mcp.contract.omitted', { count: c.omitted! })}</div>}
      {c.audit && c.audit.length > 0 && <div className="ss-list">
        {c.audit.map((finding, i) => <div className="ss-r flex-wrap gap-2 break-all" key={i}>
          <span className="ss-tag warn">{visible(finding.severity)} · {visible(finding.ruleId)}</span>
          <span className="font-mono">{visible(finding.tool)} · {visible(finding.path)}</span>
          <span>{visible(ownedMessage(t, finding.message))}</span>
        </div>)}
      </div>}
      {Object.entries(c.tools ?? {}).map(([name, tool]) => <Snapshot key={name} name={name} text={tool.definitionText ?? ''} />)}
    </>}
    {server.findings.filter((f) => f.level !== 'info').map((f, i) => <div className={`ss-note ${f.level === 'error' ? 'bad' : 'warn'} break-all`} key={i}>{visible(findingText(t, f))}</div>)}
    {!complete && !accepted && <div className="ss-note warn">{t('mcp.contract.unavailable')}</div>}
    {accepted ? <div className="ss-note inf">{t('mcp.contract.accepted')}</div> :
      <Button className="self-start" disabled={!complete || blocked || running} loading={running} onClick={onAccept}>{t('mcp.contract.trust')}</Button>}
  </section>;
}

/** Opening the dialog has no side effects. Only the probe button starts servers. */
export default function MCPContractReview({ root, onClose }: { root: string; onClose: () => void }) {
  const t = useT();
  const queryClient = useQueryClient();
  const [report, setReport] = useState<MCPCheckReport | null>(null);
  const [running, setRunning] = useState(false);
  const [error, setError] = useState('');
  const [blocked, setBlocked] = useState(false);
  const [acceptedKey, setAcceptedKey] = useState('');
  const keyOf = (server: MCPCheckServer) => JSON.stringify([server.project ?? '', server.name]);

  const probe = async () => {
    setRunning(true); setError(''); setReport(null); setBlocked(true); setAcceptedKey('');
    try {
      setReport(await mcpCheckApi.review(root)); setBlocked(false);
    } catch (e) {
      setError(visible(ownedMessage(t, (e as Error).message)));
    } finally { setRunning(false); }
  };
  const accept = async (server: MCPCheckServer) => {
    if (!server.contract?.revision || blocked || running) return;
    setRunning(true); setError('');
    // A write changes the store digest for every review token. Each remaining
    // server needs a new explicit probe, including after an uncertain failure.
    setBlocked(true);
    try {
      const result = await mcpCheckApi.accept({ name: server.name, root: server.project ?? '', revision: server.contract.revision });
      if (result.accepted !== true || result.report?.toolsAcceptance !== 'accepted') {
        setReport(result.report);
        setError(t('mcp.contract.refused')); return;
      }
      setAcceptedKey(keyOf(server));
    } catch (e) {
      if (e instanceof MCPToolsAcceptanceError) setReport(e.report);
      const message = (e as Error).message;
      const detail = visible(ownedMessage(t, message));
      setError(e instanceof ApiError && e.status === 409 ? t('mcp.contract.stale')
        : e instanceof MCPToolsAcceptanceError && contractMessages.has(message) ? detail
        : `${t('mcp.contract.refused')} ${detail}`);
    } finally {
      void invalidate(queryClient, 'mcpToolsAcceptanceAttempted');
      setRunning(false);
    }
  };

  return <DialogShell open onClose={onClose} padding="none" maxWidth="4xl" preventClose={running} ariaLabel={t('mcp.contract.title')}>
    <div className="dh"><h2 className="ss-h2">{t('mcp.contract.title')}</h2></div>
    <div className="db flex flex-col gap-4">
      <div className="ss-note warn">{t('mcp.contract.confirm')}</div>
      <p className="text-sm text-ink-2">{t('mcp.contract.limits')}</p>
      <p className="text-sm text-ink-2">{t('mcp.contract.credentials')}</p>
      {error && <div className="ss-note bad" role="alert">{error}</div>}
      {report?.servers.length === 0 && <div className="ss-note inf">{t('mcp.contract.empty')}</div>}
      {report?.servers.map((server) => <ServerReview key={keyOf(server)} server={server} running={running} blocked={blocked} accepted={acceptedKey === keyOf(server)} onAccept={() => void accept(server)} />)}
      {acceptedKey && <p className="text-sm text-ink-2">{t('mcp.contract.reprobe')}</p>}
    </div>
    <div className="df">
      <Button variant="secondary" disabled={running} onClick={onClose}>{t('mcp.contract.close')}</Button>
      <Button disabled={running} loading={running} onClick={() => void probe()}>{t('mcp.contract.probe')}</Button>
    </div>
  </DialogShell>;
}
