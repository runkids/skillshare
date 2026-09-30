import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { I18nProvider } from '../../i18n';
import MCPNotices from './MCPNotices';

const show = (notices: string[]) => render(<I18nProvider><MCPNotices notices={notices} /></I18nProvider>);

describe('MCP notices', () => {
  it('words a known notice for the dashboard, naming its servers and a project server by its root', () => {
    show(['piExtension is ignored since 0.23.0 (docs, wiki (/work/app)): Pi always uses its built-in MCP (mcp.json), sync moves these servers there and removes the entries Skillshare wrote to mcp-adapter.json; saving the config drops the field']);
    expect(screen.getByText(/^Pi now always uses its built-in MCP \(mcp\.json\): docs, wiki \(\/work\/app\)\./)).toBeInTheDocument();
    expect(screen.queryByText(/piExtension/)).not.toBeInTheDocument();
  });

  it('names each server once in a notice that lists several settings', () => {
    show(["Pi's built-in MCP does not read these pi-mcp-adapter piOptions, so they are ignored since 0.23.0: excludeTools (docs); lifecycle (docs, wiki); saving the config drops them"]);
    expect(screen.getByText(/so they are ignored: docs, wiki\./)).toBeInTheDocument();
  });

  it("groups tool policy notices by server, naming each Agent and what it leaves out", () => {
    show(['tool policy not applied for opencode: allow, deny (docs)', 'tool policy not applied for copilot: deny (docs, wiki)']);
    expect(screen.getByText('Tool settings of docs are not applied everywhere. OpenCode: allow list, deny list; Copilot CLI: deny list.')).toBeInTheDocument();
    expect(screen.getByText('Tool settings of wiki are not applied everywhere. Copilot CLI: deny list.')).toBeInTheDocument();
  });

  it('never shows a notice it does not know in English field names', () => {
    show(['somethingNew is ignored since 0.23.0 (docs)']);
    expect(screen.getByText('Some settings in the config no longer apply since 0.23.0. The CLI lists them when you sync.')).toBeInTheDocument();
  });
});
