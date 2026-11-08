import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, it, vi } from 'vitest';
import { ApiError } from '../../api/http';
import { mcpCheckApi, MCPToolsAcceptanceError, type MCPCheckReport } from '../../api/mcpCheck';
import { I18nProvider, LOCALE_STORAGE_KEY } from '../../i18n';
import MCPContractReview from './MCPContractReview';
import { MCPRailActions } from './MCPSyncBox';
import { serversFor } from './useMCPCheck';
import { queryKeys } from '../../lib/queryKeys';

vi.mock('../../api/mcpCheck', async (load) => ({ ...await load<typeof import('../../api/mcpCheck')>(), mcpCheckApi: { review: vi.fn(), accept: vi.fn() } }));
beforeEach(() => { vi.resetAllMocks(); localStorage.setItem(LOCALE_STORAGE_KEY, 'en'); });

it('localizes a top-level signed-store refusal in Persian without control escapes for an empty selected scope', async () => {
  vi.mocked(mcpCheckApi.review).mockRejectedValue(new ApiError(400, 'signed MCP tool baseline requires CLI verification with --verify-key'));
  const user = userEvent.setup(); show('', 'fa');
  await user.click(screen.getByRole('button', { name: 'بررسی زندهٔ ابزارها' }));
  const refusal = await screen.findByRole('alert');
  expect(refusal.textContent).toBe('مبنای امضا شدهٔ ابزارهای MCP باید از طریق CLI با --verify-key اعتبارسنجی شود');
  expect(refusal.textContent).not.toContain('\\u200c');
  expect(screen.queryByRole('button', { name: 'اعتماد به این فهرست' })).not.toBeInTheDocument();
  expect(mcpCheckApi.accept).not.toHaveBeenCalled();
});

it.each([
  ['MCP baseline key is invalid or unreadable; use an Ed25519 PEM key with safe file permissions', 'MCP 基準金鑰無效或無法讀取；請使用具有安全檔案權限的 Ed25519 PEM 金鑰'],
  ['signed MCP tool baseline requires CLI verification with --verify-key', '已簽署的 MCP 工具基準必須透過 CLI 使用 --verify-key 驗證'],
  ['MCP tool baseline signature could not be verified; supply the correct external verification key', '無法驗證 MCP 工具基準簽章；請提供正確的外部驗證金鑰'],
  ['unsigned MCP tool baseline requires explicit --sign-unsigned review before signing', '未簽署的 MCP 工具基準必須先明確使用 --sign-unsigned 檢閱，才能簽署'],
  ['accepting signed MCP tool baselines requires a matching --signing-key and --verify-key', '接受已簽署的 MCP 工具基準需要相符的 --signing-key 與 --verify-key'],
  ['--sign-unsigned requires an existing unsigned MCP tool baseline', '--sign-unsigned 需要現有且未簽署的 MCP 工具基準'],
])('shows localized signing refusal with no dashboard trust (%s)', async (reason, translated) => {
  vi.mocked(mcpCheckApi.review).mockResolvedValue({ servers: [{ name: 'docs', ok: false, findings: [{ level: 'error', check: 'tool-change', target: '', message: 'tool contracts could not be compared: ' + reason }], contract: { status: 'error' } }], summary: { errors: 1, warnings: 0 } });
  const user = userEvent.setup(); show('', 'zh-TW');
  await user.click(screen.getByRole('button', { name: '探測即時工具' }));
  expect(await screen.findByText(translated)).toBeInTheDocument();
  expect(screen.getByRole('button', { name: '信任這份清單' })).toBeDisabled();
  expect(mcpCheckApi.accept).not.toHaveBeenCalled();
});

const review = (revision = 'reviewed'): MCPCheckReport => ({ servers: [{
  name: 'docs', project: '/exact/root', ok: true, findings: [], live: { tools: 1 },
  contract: { status: 'untrusted', revision, policy: 'skillshare-contract-v2', identityChanged: true,
    tools: { search: { hash: 'hash', inputHash: 'input', proseHash: 'prose', outputHash: 'output', metadataHash: 'meta', definitionText: JSON.stringify({ name: 'search', description: 'x'.repeat(6000) + 'END\u202e\u001b<script>bad</script>', inputSchema: { type: 'object' }, icons: [{ src: 'https://untrusted.invalid/icon' }] }) } },
    changes: [{ tool: 'search', area: 'input', path: '/inputSchema/required', grade: 'BREAKING', message: 'Required input added' }],
    audit: [{ tool: 'search', path: '/description', ruleId: 'fixture-rule', severity: 'HIGH', message: 'Review instructions' }], omitted: 3,
  },
}], summary: { errors: 0, warnings: 1 } });
function show(root = '/requested/root', locale: 'en' | 'zh-TW' | 'fa' = 'en') {
  localStorage.setItem(LOCALE_STORAGE_KEY, locale);
  const queryClient = new QueryClient();
  return { ...render(<QueryClientProvider client={queryClient}><I18nProvider><MCPContractReview root={root} onClose={vi.fn()} /></I18nProvider></QueryClientProvider>), queryClient };
}

it('opens without probing and explicitly confirms starting configured servers', async () => {
  const user = userEvent.setup(); show();
  expect(mcpCheckApi.review).not.toHaveBeenCalled();
  expect(screen.getByText(/starts configured servers/i)).toBeInTheDocument();
  vi.mocked(mcpCheckApi.review).mockResolvedValue(review());
  vi.mocked(mcpCheckApi.accept).mockResolvedValue({ accepted: true, report: { ...review(), toolsAcceptance: 'accepted' } });
  await user.click(screen.getByRole('button', { name: 'Probe live tools' }));
  await waitFor(() => expect(mcpCheckApi.review).toHaveBeenCalledExactlyOnceWith('/requested/root'));
  await user.click(await screen.findByRole('button', { name: 'Trust this inventory' }));
  expect(mcpCheckApi.accept).toHaveBeenCalledExactlyOnceWith({ name: 'docs', root: '/exact/root', revision: 'reviewed' });
});

it('shows full descriptor text, visible controls, changes and audit without active content', async () => {
  vi.mocked(mcpCheckApi.review).mockResolvedValue(review());
  const user = userEvent.setup();const { container } = show();
  await user.click(screen.getByRole('button', { name: 'Probe live tools' }));
  await user.click(await screen.findByRole('button', { name: /Show full snapshot/ }));
  const snapshot = container.ownerDocument.querySelector('pre');
  expect(snapshot?.textContent).toContain('x'.repeat(6000));
  expect(snapshot?.textContent).toContain('END\\u202e\\u001b<script>bad</script>');
  expect(snapshot?.textContent).not.toContain('\u202e');
  expect(container.ownerDocument.querySelector('script, img, a[href*="untrusted"]')).toBeNull();
  expect(screen.getByText(/Required input added/)).toBeInTheDocument();
  expect(screen.getByText(/fixture-rule/)).toBeInTheDocument();
  expect(screen.getByText(/3 change details omitted/)).toBeInTheDocument();
});

it.each(['', undefined])('disables approval without a complete revision (%s)', async (revision) => {
  const report = review(''); report.servers[0].contract!.revision = revision;
  vi.mocked(mcpCheckApi.review).mockResolvedValue(report);
  const user = userEvent.setup(); show();
  await user.click(screen.getByRole('button', { name: 'Probe live tools' }));
  expect(await screen.findByRole('button', { name: 'Trust this inventory' })).toBeDisabled();
});





it('disables acceptance for a failed scan', async () => {
  const report = review(); report.servers[0].ok = false;report.servers[0].contract!.status = 'error'; report.servers[0].contract!.revision = '';
  vi.mocked(mcpCheckApi.review).mockResolvedValue(report);
  const user = userEvent.setup(); show();
  await user.click(screen.getByRole('button', { name: 'Probe live tools' }));
  expect(await screen.findByRole('button', { name: 'Trust this inventory' })).toBeDisabled();
});

it('retains exact numeric lexemes from the canonical snapshot', async () => {
  const report = review();
  report.servers[0].contract!.tools!.search.definitionText = '{"name":"search","inputSchema":{"type":"object","default":900719925474099312345}}';
  vi.mocked(mcpCheckApi.review).mockResolvedValue(report);
  const user = userEvent.setup(); show();
  await user.click(screen.getByRole('button', { name: 'Probe live tools' }));
  await user.click(await screen.findByRole('button', { name: /Show full snapshot/ }));
  expect(document.querySelector('pre')?.textContent).toContain('900719925474099312345');
});

it('disables trust when exact snapshot text is unavailable', async () => {
  const report = review(); report.servers[0].contract!.tools!.search.definitionText = '';
  vi.mocked(mcpCheckApi.review).mockResolvedValue(report);
  const user = userEvent.setup(); show();
  await user.click(screen.getByRole('button', { name: 'Probe live tools' }));
  expect(await screen.findByRole('button', { name: 'Trust this inventory' })).toBeDisabled();
});

it('allows review of a complete empty inventory', async () => {
  const report = review(); report.servers[0].live = { tools: 0 }; report.servers[0].contract!.tools = {};
  vi.mocked(mcpCheckApi.review).mockResolvedValue(report);
  const user = userEvent.setup(); show();
  await user.click(screen.getByRole('button', { name: 'Probe live tools' }));
  expect(await screen.findByRole('button', { name: 'Trust this inventory' })).toBeEnabled();
});

it('shows persisted success and invalidates every token after a write', async () => {
  const report = review();
  report.servers.push({ ...report.servers[0], name: 'other' });
  vi.mocked(mcpCheckApi.review).mockResolvedValue(report);
  vi.mocked(mcpCheckApi.accept).mockResolvedValue({ accepted: true, report: { ...review(), toolsAcceptance: 'accepted' } });
  const user = userEvent.setup(); show();
  await user.click(screen.getByRole('button', { name: 'Probe live tools' }));
  const buttons = await screen.findAllByRole('button', { name: 'Trust this inventory' });
  await user.click(buttons[0]);
  expect(await screen.findByText('Inventory accepted')).toBeInTheDocument();
  expect(screen.queryByText(/Untrusted: no inventory/)).not.toBeInTheDocument();

  expect(screen.getByRole('button', { name: 'Trust this inventory' })).toBeDisabled();
  expect(mcpCheckApi.review).toHaveBeenCalledTimes(1);
});

it.each(['', '/project'])('keeps the rail static check separate from explicit live review for root %s', async (root) => {
  const user = userEvent.setup(); const run = vi.fn();
  render(<QueryClientProvider client={new QueryClient()}><I18nProvider><MCPRailActions check={{ running: false, run }} reviewRoot={root} /></I18nProvider></QueryClientProvider>);
  await user.click(screen.getByRole('button', { name: 'Review live tools' }));
  expect(run).not.toHaveBeenCalled();
  expect(mcpCheckApi.review).not.toHaveBeenCalled();
  vi.mocked(mcpCheckApi.review).mockResolvedValue(review());
  await user.click(screen.getByRole('button', { name: 'Probe live tools' }));
  expect(mcpCheckApi.review).toHaveBeenCalledExactlyOnceWith(root);
});

it('keeps current baseline errors without a revision visible and cannot approve them', async () => {
  const report = review(''); report.servers[0].contract!.status = 'error';report.servers[0].ok = false;
  report.servers[0].findings = [{ level: 'error', check: 'tool-change', target: '', message: 'tool contracts could not be compared: MCP tool baseline is invalid or unreadable; restore a valid backup or inspect the state file' }];
  vi.mocked(mcpCheckApi.review).mockResolvedValue(report);
  const user = userEvent.setup(); show();
  await user.click(screen.getByRole('button', { name: 'Probe live tools' }));
  expect(await screen.findByText('tool contracts could not be compared: MCP tool baseline is invalid or unreadable; restore a valid backup or inspect the state file')).toBeInTheDocument();
});






it('localizes owned drift, audit and actionable findings in zh-TW while preserving descriptor and endpoint text', async () => {
  const report = review();
  const c = report.servers[0].contract!;
  c.changes = [{ tool: 'search', area: 'prose', path: '/description', grade: 'DESCRIPTION', message: 'model-visible text changed; review required' },
    { tool: 'search', area: 'input', path: '/inputSchema/long…', grade: 'UNKNOWN', message: 'schema constraint changed; review required; location truncated, inspect full contract' }];
  c.audit = [{ tool: 'search', path: '/description', ruleId: 'fixture-rule', severity: 'HIGH', message: 'contract text matched an audit rule; review required' }];
  c.tools!.search.definitionText = '{"name":"search","description":"model-visible text changed; review required","inputSchema":{"type":"object"}}';
  report.servers[0].findings = [
    { level: 'warning', check: 'tool-change', target: '', message: 'tool contracts have no approved baseline; inspect the complete inventory and revision before accepting' },
    { level: 'warning', check: 'tool-audit', target: '', message: 'tool contract text has audit findings; inspect the complete review' },
    { level: 'error', check: 'env', target: '', subject: 'FIXTURE_TOKEN', message: 'header X-Auth reads FIXTURE_TOKEN, which is not set' },
    { level: 'error', check: 'live', target: '', message: 'Endpoint diagnostic: model-visible text changed; review required' },
    { level: 'error', check: 'tool-change', target: '', message: 'tool baseline could not be persisted; no acceptance was recorded' },
    { level: 'error', check: 'tool-change', target: '', message: 'MCP review is stale; run a new live review before accepting' },
    { level: 'error', check: 'tool-change', target: '', message: 'model-visible text changed; review required; vendor detail' },
  ];
  vi.mocked(mcpCheckApi.review).mockResolvedValue(report);
  const user = userEvent.setup(); show('', 'zh-TW');
  await user.click(screen.getByRole('button', { name: '探測即時工具' }));
  expect(await screen.findByText('模型可見的文字已變更；必須檢視。')).toBeInTheDocument();
  expect(screen.getByText('合約文字符合稽核規則；必須檢視。')).toBeInTheDocument();
  expect(screen.getByText('工具合約尚無已核准的基準；接受前請檢視完整清單與版本。')).toBeInTheDocument();
  expect(screen.getByText('工具合約文字有稽核發現；請檢視完整報告。')).toBeInTheDocument();
  expect(screen.getByText('header X-Auth 讀取 FIXTURE_TOKEN，但此變數尚未設定。')).toBeInTheDocument();
  expect(screen.getByText('結構描述的限制已變更；必須檢視。位置已截短，請檢視完整合約。')).toBeInTheDocument();
  expect(screen.getByText('Endpoint diagnostic: model-visible text changed; review required')).toBeInTheDocument();
  expect(screen.getByText('無法儲存工具基準；未記錄任何接受結果。')).toBeInTheDocument();
  expect(screen.getByText('MCP 檢視版本已過期；接受前請重新進行即時檢視。')).toBeInTheDocument();
  expect(screen.getByText('model-visible text changed; review required; vendor detail')).toBeInTheDocument();
  await user.click(screen.getByRole('button', { name: /顯示完整快照/ }));
  expect(document.querySelector('pre')?.textContent).toBe(c.tools!.search.definitionText);
});

it('escapes controls in every rendered identity, location and diagnostic field', async () => {
  const report = review(); const c = report.servers[0].contract!;
  report.servers[0].name = 'docs\u202e';
  report.servers[0].project = '/root\u001b';
  c.policy = 'policy\u2066';
  c.changes = [{ tool: 'tool\u202d', area: 'area\u001b', path: '/path\u202e', grade: 'UNKNOWN\u2066', message: 'change\u001b' }];
  c.audit = [{ tool: 'auditTool\u202e', path: '/auditPath\u001b', ruleId: 'rule\u2066', severity: 'HIGH\u202d', message: 'audit\u001b' }];
  report.servers[0].findings = [{ level: 'error', check: 'live', target: '', message: 'diagnostic\u001b]52;c;payload\u0007\u202e' }];
  vi.mocked(mcpCheckApi.review).mockResolvedValue(report);
  const user = userEvent.setup(); show();
  await user.click(screen.getByRole('button', { name: 'Probe live tools' }));
  await screen.findByText(/diagnostic/);
  const dialog = screen.getByRole('dialog');
  expect(dialog.textContent).not.toMatch(/[\u001b\u0007\u202d\u202e\u2066]/u);
  for (const name of ['docs', '/root', 'policy', 'tool', 'area', '/path', 'UNKNOWN', 'auditTool', '/auditPath', 'rule', 'HIGH', 'change', 'audit', 'diagnostic']) {
    expect(dialog.textContent).toContain(name + '\\u');
  }
  expect(document.querySelector('a[href], img, script')).toBeNull();
});

it.each([
  ['tool-change', '', 'baseline trust requires fromEnv references for every environment, header and bearer value', 'اعتماد به مبنا به ارجاع های fromEnv برای همهٔ مقادیر محیط، سرآیند و توکن Bearer نیاز دارد'],
  ['sync', 'claude', 'not synced yet; run skillshare sync mcp', 'هنوز همگام سازی نشده است. بعد از زدن «همگام سازی MCP» نوشته می شود.'],
] as const)('keeps Persian %s findings readable', async (check, target, message, expected) => {
  const report = review(''); report.servers[0].findings = [{ level: 'warning', check, target, message }];
  vi.mocked(mcpCheckApi.review).mockResolvedValue(report);
  const user = userEvent.setup(); show('', 'fa');
  await user.click(screen.getByRole('button', { name: 'بررسی زندهٔ ابزارها' }));
  expect((await screen.findByText(expected)).textContent).not.toContain('\\u200c');
});

it('requires an explicit new review and second trust click after a stale failure', async () => {
  vi.mocked(mcpCheckApi.review).mockResolvedValueOnce(review('first')).mockResolvedValueOnce(review('second'));
  vi.mocked(mcpCheckApi.accept).mockRejectedValueOnce(new ApiError(409, 'stale')).mockResolvedValueOnce({ accepted: true, report: { ...review('second'), toolsAcceptance: 'accepted' } });
  const user = userEvent.setup(); show();
  await user.click(screen.getByRole('button', { name: 'Probe live tools' }));
  await user.click(await screen.findByRole('button', { name: 'Trust this inventory' }));
  await screen.findByText(/Review is stale/);
  expect(screen.getByText(/Probe again to review/)).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Trust this inventory' })).toBeDisabled();
  await user.click(screen.getByRole('button', { name: 'Probe live tools' }));
  const trust = await screen.findByRole('button', { name: 'Trust this inventory' });
  expect(mcpCheckApi.accept).toHaveBeenCalledTimes(1);
  expect(trust).toBeEnabled();
  await user.click(trust);
  expect(mcpCheckApi.accept).toHaveBeenLastCalledWith({ name: 'docs', root: '/exact/root', revision: 'second' });
  expect(await screen.findByText('Inventory accepted')).toBeInTheDocument();
});


it.each([
  ['existing entry is not managed; import it to explicitly adopt it', 'Agent 已有同名項目，尚未交給 skillshare 管理'],
  ['managed by another Skillshare config: /fixture\u202e/config.yaml', '由另一份 skillshare 設定管理: /fixture\\u202e/config.yaml'],
  ['left over from a Skillshare config that was removed; import it or explicitly replace this entry: /fixture/config.yaml', '殘留自一份已被移除的 skillshare 設定；請匯入或以來源覆寫: /fixture/config.yaml'],
  ['Agent setting changed since sync; import it before removing the cleared setting', 'Agent 設定在 skillshare 之外被修改過'],
  ['Agent configuration changed; import it or explicitly replace this entry', 'Agent 設定在 skillshare 之外被修改過'],
  ['existing entry is a Pi project override of a global server; replace it, or remove the override with /mcp in Pi', '這是在 Pi 用 /mcp 建立的專案覆寫，覆寫的是 global server。請取代它，或在 Pi 移除這個覆寫'],
  ['entry changed after the backup; restore would overwrite newer changes', '備份後項目又被修改過，還原會覆蓋較新的變更'],
  ['a local scope server of the same name in ~/.claude.json overrides this one in this project; remove it with: claude mcp remove fixture', '~/.claude.json 裡有同名的 local scope 伺服器，在這個專案會蓋過這一筆。用這個指令移除： claude mcp remove fixture'],
])('uses the existing zh-TW conflict presenter for sync findings: %s', async (message, expected) => {
  const report = review(''); report.servers[0].ok = false;
  report.servers[0].findings = [{ level: 'error', check: 'sync', target: 'claude', message }];
  vi.mocked(mcpCheckApi.review).mockResolvedValue(report);
  const user = userEvent.setup(); show('', 'zh-TW');
  await user.click(screen.getByRole('button', { name: '探測即時工具' }));
  expect(await screen.findByText(expected)).toBeInTheDocument();
  expect(screen.getByRole('dialog').textContent).not.toContain('\u202e');
  expect(screen.getByRole('button', { name: '信任這份清單' })).toBeDisabled();
});

it('preserves unknown sync diagnostics and credential-like text that is not an exact owned message', async () => {
  const report = review('');
  const sync = 'Vendor: existing entry is not managed; import it to explicitly adopt it';
  const credential = 'stored tool contracts contain a configured credential; no review is available (vendor detail)';
  report.servers[0].findings = [
    { level: 'error', check: 'sync', target: '', message: sync },
    { level: 'error', check: 'tool-change', target: '', message: credential },
  ];
  vi.mocked(mcpCheckApi.review).mockResolvedValue(report);
  const user = userEvent.setup(); show('', 'zh-TW');
  await user.click(screen.getByRole('button', { name: '探測即時工具' }));
  expect(await screen.findByText(sync)).toBeInTheDocument();
  expect(screen.getByText(credential)).toBeInTheDocument();
});


it.each([
  ['tool inventory contains duplicate names', '工具清單含有重複名稱'],
  ['tools/list must return an array of tools', 'tools/list 必須傳回工具陣列'],
  ['tool inventory exceeds its byte or tool limit', '工具清單超過位元組或工具數量上限'],
  ['canonical tool inventory exceeds its byte limit', '正規化工具清單超過位元組上限'],
  ['tool inventory repeated a pagination cursor', '工具清單重複使用分頁游標'],
  ['tool inventory is incomplete at the page limit', '工具清單達到分頁上限，仍未完整取得'],
  ['invalid JSON-RPC envelope', 'JSON-RPC 封套無效'],
  ['ambiguous JSON-RPC fields', 'JSON-RPC 欄位有歧義'],
  ['tools/list: tools/list returned a non-complete result type', 'tools/list: tools/list 傳回未完成的結果類型'],
  ['tools/list: tools/list returned an invalid pagination cursor', 'tools/list: tools/list 傳回無效的分頁游標'],
  ['initialize: initialize returned ambiguous or invalid fields', 'initialize: initialize 傳回有歧義或無效的欄位'],
  ['initialize: initialize returned invalid server information', 'initialize: initialize 傳回無效的伺服器資訊'],
  ['server/discover returned invalid server information', 'server/discover 傳回無效的伺服器資訊'],
  ['tools/list: the response is not a JSON-RPC response', 'tools/list: 回覆不是有效的 JSON-RPC 回應'],
  ['the event stream ended before the response event was complete', '事件串流在回應事件完整傳回前已結束'],
  ['tool inventory rejected: invalid MCP tool contract', '工具清單遭拒：MCP 工具合約無效'],
  ['connection identity contains a configured credential; use environment-backed references', '連線識別含有已設定的憑證；請使用環境變數參照'],
])('localizes exact owned live failure reasons in their probe context: %s', async (detail, expected) => {
  const report = review(''); report.servers[0].ok = false;
  report.servers[0].findings = [{ level: 'error', check: 'live', target: '', message: 'live probe failed: ' + detail }];
  vi.mocked(mcpCheckApi.review).mockResolvedValue(report);
  const user = userEvent.setup(); show('', 'zh-TW');
  await user.click(screen.getByRole('button', { name: '探測即時工具' }));
  expect(await screen.findByText('即時探測失敗：' + expected)).toBeInTheDocument();
  expect(screen.getByRole('button', { name: '信任這份清單' })).toBeDisabled();
});

it('preserves unrecognized live details, unknown method wrappers and text outside the owned live context', async () => {
  const report = review('');
  report.servers[0].findings = [
    { level: 'error', check: 'live', target: '', message: 'live probe failed: vendor/do: tool inventory contains duplicate names' },
    { level: 'error', check: 'live', target: '', message: 'live probe failed: tool inventory contains duplicate names (vendor detail)' },
    { level: 'error', check: 'live', target: '', message: 'tool inventory contains duplicate names' },
    { level: 'error', check: 'tool-change', target: '', message: 'live probe failed: tool inventory contains duplicate names' },
  ];
  vi.mocked(mcpCheckApi.review).mockResolvedValue(report);
  const user = userEvent.setup(); show('', 'zh-TW');
  await user.click(screen.getByRole('button', { name: '探測即時工具' }));
  expect(await screen.findByText('即時探測失敗：vendor/do: tool inventory contains duplicate names')).toBeInTheDocument();
  expect(screen.getByText('即時探測失敗：tool inventory contains duplicate names (vendor detail)')).toBeInTheDocument();
  expect(screen.getByText('tool inventory contains duplicate names')).toBeInTheDocument();
  expect(screen.getByText('live probe failed: tool inventory contains duplicate names')).toBeInTheDocument();
});

it('retains protocol parameters and escapes controls after localizing owned labels', async () => {
  const report = review('');
  report.servers[0].findings = [
    { level: 'error', check: 'live', target: '', message: 'live probe failed: JSON-RPC error -32022' },
    { level: 'error', check: 'live', target: '', message: 'live probe failed: no answer within 1m2.5s' },
    { level: 'error', check: 'live', target: '', message: 'live probe failed: server supports protocol version(s) vendor\u202e; skillshare probes 2026-07-28 or the initialize handshake' },
    { level: 'error', check: 'live', target: '', message: 'live probe failed: invalid JSON-RPC response; server diagnostic output omitted' },
  ];
  vi.mocked(mcpCheckApi.review).mockResolvedValue(report);
  const user = userEvent.setup(); show('', 'zh-TW');
  await user.click(screen.getByRole('button', { name: '探測即時工具' }));
  expect(await screen.findByText('即時探測失敗：JSON-RPC 錯誤 -32022')).toBeInTheDocument();
  expect(screen.getByText('即時探測失敗：1m2.5s 內未收到回應')).toBeInTheDocument();
  expect(screen.getByText('即時探測失敗：伺服器支援的協定版本為 vendor\\u202e；skillshare 探測 2026-07-28 或使用 initialize 交握')).toBeInTheDocument();
  expect(screen.getByText('即時探測失敗：JSON-RPC 回應無效；已省略伺服器診斷輸出')).toBeInTheDocument();
});

it.each(['accepted', 'refused', 'stale', 'connection-error'] as const)('invalidates log lists and statistics after every acceptance attempt: %s', async (outcome) => {
  vi.mocked(mcpCheckApi.review).mockResolvedValue(review());
  if (outcome === 'stale') vi.mocked(mcpCheckApi.accept).mockRejectedValue(new ApiError(409, 'stale'));
  else if (outcome === 'connection-error') vi.mocked(mcpCheckApi.accept).mockRejectedValue(new ApiError(0, 'lost connection'));
  else if (outcome === 'refused') vi.mocked(mcpCheckApi.accept).mockRejectedValue(new MCPToolsAcceptanceError(422, 'inventory was not accepted', { ...review(), toolsAcceptance: 'error' }));
  else vi.mocked(mcpCheckApi.accept).mockResolvedValue({ accepted: true, report: { ...review(), toolsAcceptance: 'accepted' } });
  const user = userEvent.setup(); const { queryClient } = show();
  const logKeys = [queryKeys.log('ops', 100)];
  for (const key of [...logKeys, queryKeys.mcp, queryKeys.audit.all()]) queryClient.setQueryData(key, { fixture: true });
  expect(logKeys.every((key) => !queryClient.getQueryState(key)?.isInvalidated)).toBe(true);
  await user.click(screen.getByRole('button', { name: 'Probe live tools' }));
  const trust = await screen.findByRole('button', { name: 'Trust this inventory' });
  expect(logKeys.every((key) => !queryClient.getQueryState(key)?.isInvalidated)).toBe(true);
  await user.click(trust);
  await waitFor(() => expect(logKeys.every((key) => queryClient.getQueryState(key)?.isInvalidated)).toBe(true));
  expect(queryClient.getQueryState(queryKeys.mcp)?.isInvalidated).toBe(false);
  expect(queryClient.getQueryState(queryKeys.audit.all())?.isInvalidated).toBe(false);
});

it('replaces a valid preview with the actual HTTP refusal report and disables the old approval', async () => {
  vi.mocked(mcpCheckApi.review).mockResolvedValue(review('old-reviewed-inventory'));
  const refused: MCPCheckReport = { toolsAcceptance: 'error', servers: [{ name: 'docs', project: '/exact/root', ok: false, findings: [{ level: 'error', check: 'live', target: '', message: 'live probe failed: tool inventory contains duplicate names' }], contract: { status: 'error' } }], summary: { errors: 1, warnings: 0 } };
  vi.mocked(mcpCheckApi.accept).mockRejectedValue(new MCPToolsAcceptanceError(422, 'inventory was not accepted; review the check report', refused));
  const user = userEvent.setup(); const { queryClient } = show();
  queryClient.setQueryData(queryKeys.log('ops', 100), { fixture: true });
  await user.click(screen.getByRole('button', { name: 'Probe live tools' }));
  const trust = await screen.findByRole('button', { name: 'Trust this inventory' });
  expect(trust).toBeEnabled();
  expect(screen.getByRole('button', { name: /Show full snapshot/ })).toBeInTheDocument();
  await user.click(trust);
  expect(await screen.findByText('live probe failed: tool inventory contains duplicate names')).toBeInTheDocument();
  expect(screen.queryByRole('button', { name: /Show full snapshot/ })).not.toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Trust this inventory' })).toBeDisabled();
  expect(mcpCheckApi.accept).toHaveBeenCalledExactlyOnceWith({ name: 'docs', root: '/exact/root', revision: 'old-reviewed-inventory' });
  expect(mcpCheckApi.review).toHaveBeenCalledTimes(1);
  expect(queryClient.getQueryState(queryKeys.log('ops', 100))?.isInvalidated).toBe(true);
});

it.each([
  ['en', 'Probe live tools', 'Trust this inventory', 'MCP acceptance outcome is unknown; probe again before accepting'],
  ['zh-TW', '探測即時工具', '信任這份清單', '無法確認 MCP 接受結果；再次接受前請重新探測'],
] as const)('shows only the unknown outcome and clears the old inventory in %s', async (locale, probeLabel, trustLabel, expected) => {
  vi.mocked(mcpCheckApi.review).mockResolvedValue(review());
  vi.mocked(mcpCheckApi.accept).mockRejectedValue(new MCPToolsAcceptanceError(0, 'MCP acceptance outcome is unknown; probe again before accepting', null));
  const user = userEvent.setup(); show('', locale);
  await user.click(screen.getByRole('button', { name: probeLabel }));
  await user.click(await screen.findByRole('button', { name: trustLabel }));
  const alert = await screen.findByRole('alert');
  expect(alert.textContent).toBe(expected);
  expect(alert).not.toHaveTextContent(/not accepted|未接受/i);
  expect(screen.queryByRole('button', { name: trustLabel })).not.toBeInTheDocument();
  expect(screen.queryByRole('button', { name: /Show full snapshot|顯示完整快照/ })).not.toBeInTheDocument();
  expect(mcpCheckApi.review).toHaveBeenCalledTimes(1);
  expect(mcpCheckApi.accept).toHaveBeenCalledTimes(1);
});

it.each([
  ['tool-change', 'zh-TW', '探測即時工具', '信任這份清單', '工具合約含有已設定的憑證；無法提供檢視。', 'tool contracts contain a configured credential; no review is available'],
  ['tool-change', 'zh-TW', '探測即時工具', '信任這份清單', '已儲存的工具合約含有已設定的憑證；無法提供檢視。', 'stored tool contracts contain a configured credential; no review is available'],
  ['live', 'zh-TW', '探測即時工具', '信任這份清單', '連線識別含有已設定的憑證；請使用環境變數參照', 'connection identity contains a configured credential; use environment-backed references'],
  ['live', 'en', 'Probe live tools', 'Trust this inventory', 'project scope contains a configured credential; no review is available', 'project scope contains a configured credential; no review is available'],
  ['tool-change', 'en', 'Probe live tools', 'Trust this inventory', 'project scope contains a configured credential; no review is available', 'project scope contains a configured credential; no review is available'],
  ['live', 'zh-TW', '探測即時工具', '信任這份清單', '專案範圍含有已設定的憑證；無法提供檢視。', 'project scope contains a configured credential; no review is available'],
  ['tool-change', 'zh-TW', '探測即時工具', '信任這份清單', '專案範圍含有已設定的憑證；無法提供檢視。', 'project scope contains a configured credential; no review is available'],
] as const)('localizes scope credential refusal from %s in %s', async (check, locale, probeLabel, trustLabel, expected, message) => {
  const report = review(''); const server = report.servers[0];
  server.ok = false; server.live = undefined;
  server.contract = { status: 'error', policy: 'skillshare-contract-v2' };
  server.findings = [{ level: 'error', check, target: '', message }];
  vi.mocked(mcpCheckApi.review).mockResolvedValue(report);
  const user = userEvent.setup(); show('', locale);
  await user.click(screen.getByRole('button', { name: probeLabel }));
  expect(await screen.findByText(expected)).toBeInTheDocument();
  expect(screen.getByRole('button', { name: trustLabel })).toBeDisabled();
  expect(mcpCheckApi.accept).not.toHaveBeenCalled();
});


it.each([
  ['en', 'Probe live tools', 'Trust this inventory', 'inventory was not accepted; review the check report'],
  ['zh-TW', '探測即時工具', '信任這份清單', '未接受工具清單；請檢視檢查報告。'],
] as const)('does not duplicate a known owned adapter refusal in %s', async (locale, probeLabel, trustLabel, expected) => {
  vi.mocked(mcpCheckApi.review).mockResolvedValue(review());
  const refused: MCPCheckReport = { toolsAcceptance: 'error', servers: [{ name: 'docs', project: '/exact/root', ok: false, findings: [], contract: { status: 'error' } }], summary: { errors: 1, warnings: 0 } };
  vi.mocked(mcpCheckApi.accept).mockRejectedValue(new MCPToolsAcceptanceError(422, 'inventory was not accepted; review the check report', refused));
  const user = userEvent.setup(); show('', locale);
  await user.click(screen.getByRole('button', { name: probeLabel }));
  await user.click(await screen.findByRole('button', { name: trustLabel }));
  expect((await screen.findByRole('alert')).textContent).toBe(expected);
  expect(screen.getByRole('button', { name: trustLabel })).toBeDisabled();
  expect(mcpCheckApi.review).toHaveBeenCalledTimes(1);
});

it('keeps refusal context and visible controls for unknown adapter diagnostics', async () => {
  vi.mocked(mcpCheckApi.review).mockResolvedValue(review());
  const refused: MCPCheckReport = { toolsAcceptance: 'error', servers: [{ name: 'docs', project: '/exact/root', ok: false, findings: [], contract: { status: 'error' } }], summary: { errors: 1, warnings: 0 } };
  vi.mocked(mcpCheckApi.accept).mockRejectedValue(new MCPToolsAcceptanceError(422, 'vendor diagnostic\u202e', refused));
  const user = userEvent.setup(); show();
  await user.click(screen.getByRole('button', { name: 'Probe live tools' }));
  await user.click(await screen.findByRole('button', { name: 'Trust this inventory' }));
  const alert = await screen.findByRole('alert');
  expect(alert).toHaveTextContent(/^Inventory was not accepted/);
  expect(alert.textContent).toContain('vendor diagnostic\\u202e');
  expect(alert.textContent).not.toContain('\u202e');
  expect(screen.getByRole('button', { name: 'Trust this inventory' })).toBeDisabled();
});


it('associates and approves the exact project root while escaping it for display', async () => {
  const root = '/project\u202e\u001b';
  const report = review(); report.servers[0].project = root;
  expect(serversFor(report, root)).toHaveLength(1);
  expect(serversFor(report, '/project\\u202e\\u001b')).toHaveLength(0);
  vi.mocked(mcpCheckApi.review).mockResolvedValue(report);
  vi.mocked(mcpCheckApi.accept).mockResolvedValue({ accepted: true, report: { ...report, toolsAcceptance: 'accepted' } });
  const user = userEvent.setup(); show(root);
  await user.click(screen.getByRole('button', { name: 'Probe live tools' }));
  const trust = await screen.findByRole('button', { name: 'Trust this inventory' });
  expect(screen.getByRole('dialog').textContent).toContain('/project\\u202e\\u001b');
  await user.click(trust);
  expect(mcpCheckApi.accept).toHaveBeenCalledExactlyOnceWith({ name: 'docs', root, revision: 'reviewed' });
});
