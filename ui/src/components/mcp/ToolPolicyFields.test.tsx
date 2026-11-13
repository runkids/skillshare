import { useState } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, it, vi } from 'vitest';
import type { MCPToolPolicy } from '../../api/mcp';
import { mcpCheckApi } from '../../api/mcpCheck';
import { I18nProvider, LOCALE_STORAGE_KEY, translate } from '../../i18n';
import ToolPolicyFields from './ToolPolicyFields';

vi.mock('../../api/mcpCheck', async (load) => ({ ...await load<typeof import('../../api/mcpCheck')>(), mcpCheckApi: { probe: vi.fn() } }));
beforeEach(() => { vi.resetAllMocks(); localStorage.setItem(LOCALE_STORAGE_KEY, 'en'); });
const rawName = 'read\u202e_file';
const displayName = 'read\\u202e_file';
const rawRule = 'read\u202e_*';
const displayRule = 'read\\u202e_*';

function show(initial: MCPToolPolicy = {}, error?: string) {
  const changed = vi.fn();
  function Editor() {
    const [tools, setTools] = useState(initial);
    return <ToolPolicyFields tools={tools} onChange={(next) => { changed(next); setTools(next); }} error={error} disabled={false} probe={{ name: 'fixture', server: { command: 'fixture' } }} />;
  }
  return { ...render(<QueryClientProvider client={new QueryClient()}><I18nProvider><Editor /></I18nProvider></QueryClientProvider>), changed };
}

it('shows escaped loaded tool names while policy selection retains the exact advertised name', async () => {
  vi.mocked(mcpCheckApi.probe).mockResolvedValue({ live: { tools: 1, toolNames: [rawName] } });
  const user = userEvent.setup(); const { container, changed } = show();
  await user.click(screen.getByRole('button', { name: 'Load tools' }));
  const checkbox = await screen.findByRole('checkbox', { name: displayName });
  expect(checkbox).toBeChecked();
  expect(container.textContent).not.toContain('\u202e');
  await user.click(checkbox);
  expect(changed).toHaveBeenCalledExactlyOnceWith({ deny: [rawName] });
  expect(changed.mock.calls[0][0].deny).not.toContain(displayName);
});

it('escapes blocking-rule tooltips and rule chips without modifying the stored rule', async () => {
  vi.mocked(mcpCheckApi.probe).mockResolvedValue({ live: { tools: 1, toolNames: [rawName] } });
  const user = userEvent.setup(); const { container, changed } = show({ deny: [rawRule] });
  expect(screen.getByText(displayRule)).toBeInTheDocument();
  const remove = screen.getByRole('button', { name: 'Remove ' + displayRule });
  expect(remove.getAttribute('aria-label')).not.toContain('\u202e');
  await user.click(screen.getByRole('button', { name: 'Load tools' }));
  expect(await screen.findByRole('checkbox', { name: displayName })).toBeDisabled();
  await user.hover(screen.getByText(displayName));
  expect(await screen.findByRole('tooltip')).toHaveTextContent('Blocked by the exclude rule ' + displayRule);
  expect(document.body.textContent).not.toContain('\u202e');
  expect(changed).not.toHaveBeenCalled();
  await user.unhover(screen.getByText(displayName));
  await user.click(remove);
  expect(changed).toHaveBeenCalledExactlyOnceWith({ deny: [] });
  expect(container.textContent).not.toContain('\u202e');
});

it.each([
  ['project scope contains a configured credential; no review is available', '專案範圍含有已設定的憑證；無法提供檢視。'],
  ['connection identity contains a configured credential; use environment-backed references', '連線識別含有已設定的憑證；請使用環境變數參照'],
  ['configured credential appeared in connection identity; use environment references and remove credential-bearing metadata', '連線識別出現已設定的憑證；請使用環境變數參照，並移除含有憑證的中繼資料'],
] as const)('localizes the owned credential refusal in draft probe details: %s', async (message, expected) => {
  localStorage.setItem(LOCALE_STORAGE_KEY, 'zh-TW');
  vi.mocked(mcpCheckApi.probe).mockResolvedValue({ errorKind: 'protocol', error: message });
  const user = userEvent.setup(); const { changed } = show();
  await user.click(screen.getByRole('button', { name: '載入工具' }));
  await user.hover(await screen.findByRole('button', { name: '詳細' }));
  expect(await screen.findByRole('tooltip')).toHaveTextContent(expected);
  expect(screen.getByRole('tooltip')).not.toHaveTextContent(message);
  expect(changed).not.toHaveBeenCalled();
});

it('preserves unknown draft probe detail instead of translating matching prose within it', async () => {
  const detail = 'Vendor: configured credential appeared in connection identity; use environment references and remove credential-bearing metadata';
  localStorage.setItem(LOCALE_STORAGE_KEY, 'zh-TW');
  vi.mocked(mcpCheckApi.probe).mockResolvedValue({ errorKind: 'protocol', error: detail });
  const user = userEvent.setup(); const { changed } = show();
  await user.click(screen.getByRole('button', { name: '載入工具' }));
  await user.hover(await screen.findByRole('button', { name: '詳細' }));
  expect(await screen.findByRole('tooltip')).toHaveTextContent(detail);
  expect(screen.getByRole('tooltip')).not.toHaveTextContent('連線識別出現已設定的憑證');
  expect(changed).not.toHaveBeenCalled();
});

it.each([
  ['connection identity contains a configured credential; use environment-backed references', 'هویت اتصال حاوی اطلاعات احراز هویت پیکربندی شده است؛ از ارجاع های مبتنی بر متغیرهای محیطی استفاده کنید'],
  ['vendor diagnostic\u200c\u202e', 'vendor diagnostic\\u200c\\u202e'],
] as const)('keeps Persian probe details readable and server format controls visible: %s', async (message, expected) => {
  localStorage.setItem(LOCALE_STORAGE_KEY, 'fa');
  vi.mocked(mcpCheckApi.probe).mockResolvedValue({ errorKind: 'protocol', error: message });
  const user = userEvent.setup(); const { changed } = show();
  await user.click(screen.getByRole('button', { name: 'بارگذاری ابزارها' }));
  await user.hover(await screen.findByRole('button', { name: 'جزئیات' }));
  const tooltip = await screen.findByRole('tooltip');
  expect(tooltip.textContent).toBe(expected);
  expect(changed).not.toHaveBeenCalled();
});

it.each(['timeout', 'auth', 'invalid-rule', 'deny-all'] as const)('keeps the Persian %s message readable', async (kind) => {
  localStorage.setItem(LOCALE_STORAGE_KEY, 'fa');
  const messages = {
    timeout: 'سرور به موقع پاسخ نداد.',
    auth: 'سرور اطلاعات ورود می خواهد یا آن را رد کرد. توکن، Headers و متغیرهای محیطی آن ها را بررسی کنید.',
    'invalid-rule': 'bad?name نام ابزار نیست. از حروف، ارقام و علائم استفاده کنید؛ * با هر نویسه ای مطابقت دارد.',
    'deny-all': '«رد» همهٔ ابزارهایی را که «اجازه» نگه می دارد حذف می کند، پس سرور هیچ ابزاری نخواهد داشت. یک مورد را از «رد» بردارید یا ابزار دیگری به «اجازه» اضافه کنید.',
  };
  if (kind === 'timeout' || kind === 'auth') vi.mocked(mcpCheckApi.probe).mockResolvedValue({ errorKind: kind, error: 'fixture diagnostic' });
  const user = userEvent.setup();
  const { changed } = kind === 'deny-all' ? show({ allow: ['read'], deny: ['read'] }, translate('fa', 'mcp.tools.denyAll')) : show();
  if (kind === 'timeout' || kind === 'auth') await user.click(screen.getByRole('button', { name: 'بارگذاری ابزارها' }));
  if (kind === 'invalid-rule') await user.type(screen.getByRole('textbox', { name: 'قوانین حذف' }), 'bad?name{Enter}');
  expect((await screen.findByText(messages[kind])).textContent).not.toContain('\\u200c');
  expect(changed).not.toHaveBeenCalled();
});
