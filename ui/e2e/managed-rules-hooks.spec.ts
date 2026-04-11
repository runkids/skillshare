import { expect, test, type Page } from '@playwright/test';

async function installBrowserMocks(page: Page) {
  await page.addInitScript(() => {
    const originalFetch = window.fetch.bind(window);

    const json = (body: unknown) =>
      new Response(JSON.stringify(body), {
        status: 200,
        headers: {
          'Content-Type': 'application/json',
        },
      });

    const overview = {
      source: '/tmp/home',
      skillCount: 4,
      topLevelCount: 2,
      targetCount: 3,
      mode: 'project',
      version: '1.0.0',
      managedRulesCount: 1,
      managedHooksCount: 1,
      trackedRepos: [],
      isProjectMode: true,
    };

    const managedHookDetail = {
      hook: {
        id: 'claude/pre-tool-use/bash.yaml',
        tool: 'claude',
        event: 'PreToolUse',
        matcher: 'Bash',
        handlers: [
          {
            type: 'command',
            command: './bin/check',
          },
        ],
      },
      previews: [
        {
          target: 'claude',
          files: [
            {
              path: '/tmp/home/.claude/settings.json',
              content: '{"hooks":{}}',
              format: 'json',
            },
          ],
          warnings: [],
        },
      ],
    };

    const managedRulesList = {
      rules: [
        {
          id: 'claude/backend.md',
          tool: 'claude',
          name: 'backend.md',
          relativePath: 'claude/backend.md',
          content: '# Backend',
        },
      ],
    };

    const discoveredRulesList = {
      warnings: [],
      rules: [
        {
          id: 'claude:project:backend',
          name: 'backend-rule',
          sourceTool: 'claude',
          scope: 'project',
          path: '/tmp/project/.claude/rules/backend.md',
          exists: true,
          collectible: true,
          collectReason: 'Ready to import',
          content: '# Backend Rule\n\nFollow the backend checklist.',
          size: 41,
          isScoped: false,
          stats: {
            wordCount: 6,
            lineCount: 3,
            tokenCount: 12,
          },
        },
      ],
    };

    const skill = {
      name: 'cli-e2e-test',
      flatName: 'cli-e2e-test',
      relPath: 'cli-e2e-test',
      sourcePath: '/tmp/skills/cli-e2e-test',
      isInRepo: false,
      targets: ['claude', 'codex'],
    };

    const auditSummary = {
      total: 0,
      passed: 0,
      warning: 0,
      failed: 0,
      critical: 0,
      high: 1,
      medium: 0,
      low: 0,
      info: 0,
      threshold: 'warn',
      riskScore: 24,
      riskLabel: 'high',
    };

    const managedRuleDetail = {
      rule: {
        id: 'claude/backend.md',
        tool: 'claude',
        name: 'backend.md',
        relativePath: 'claude/backend.md',
        content: '# Backend',
      },
      previews: [
        {
          target: 'claude',
          files: [
            {
              path: '/tmp/home/.claude/rules/backend.md',
              content: '# Backend',
              format: 'markdown',
            },
          ],
          warnings: [],
        },
      ],
    };

    class MockEventSource {
      url: string;
      listeners = new Map<string, Set<(event: MessageEvent) => void>>();
      closed = false;

      constructor(url: string) {
        this.url = url;
        setTimeout(() => {
          this.dispatch('start', { total: 0 });
          this.dispatch('done', { diffs: [] });
        }, 0);
      }

      addEventListener(type: string, listener: (event: MessageEvent) => void) {
        if (!this.listeners.has(type)) {
          this.listeners.set(type, new Set());
        }
        this.listeners.get(type)?.add(listener);
      }

      removeEventListener(type: string, listener: (event: MessageEvent) => void) {
        this.listeners.get(type)?.delete(listener);
      }

      close() {
        this.closed = true;
      }

      dispatch(type: string, data: unknown) {
        if (this.closed) return;
        const event = new MessageEvent(type, { data: JSON.stringify(data) });
        for (const listener of this.listeners.get(type) ?? []) {
          listener(event);
        }
      }
    }

    // @ts-expect-error test-only browser shim
    window.EventSource = MockEventSource;

    window.fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
      const requestUrl = typeof input === 'string' || input instanceof URL ? input.toString() : input.url;
      const url = new URL(requestUrl, window.location.origin);
      const method = (init?.method ?? (typeof input === 'string' || input instanceof URL ? 'GET' : input.method ?? 'GET')).toUpperCase();

      if (!url.pathname.startsWith('/api/')) {
        return originalFetch(input, init);
      }

      if (method === 'GET' && url.pathname === '/api/overview') {
        return json(overview);
      }

      if (method === 'GET' && url.pathname === '/api/managed/hooks') {
        return json({
          hooks: [
            {
              id: 'claude/pre-tool-use/bash.yaml',
              tool: 'claude',
              event: 'PreToolUse',
              matcher: 'Bash',
              handlers: [
                {
                  type: 'command',
                  command: './bin/check',
                },
              ],
            },
          ],
        });
      }

      if (method === 'GET' && url.pathname === '/api/managed/rules') {
        return json(managedRulesList);
      }

      if (method === 'GET' && url.pathname === '/api/rules') {
        return json(discoveredRulesList);
      }

      if (method === 'POST' && url.pathname === '/api/managed/rules') {
        return json(managedRuleDetail);
      }

      if (method === 'POST' && url.pathname === '/api/managed/rules/collect') {
        return json({
          created: ['claude/backend.md'],
          overwritten: [],
          skipped: [],
        });
      }

      if (method === 'GET' && url.pathname === '/api/hooks') {
        return json({
          warnings: [],
          hooks: [
            {
              groupId: 'claude:project:PreToolUse:Bash',
              sourceTool: 'claude',
              scope: 'project',
              event: 'PreToolUse',
              matcher: 'Bash',
              actionType: 'command',
              command: './bin/check',
              path: '/tmp/project/.claude/settings.json',
              collectible: true,
              collectReason: 'Can be collected into managed hooks',
            },
          ],
        });
      }

      if (method === 'GET' && url.pathname.startsWith('/api/managed/hooks/')) {
        return json(managedHookDetail);
      }

      if (method === 'GET' && url.pathname.startsWith('/api/managed/rules/')) {
        return json(managedRuleDetail);
      }

      if (method === 'GET' && url.pathname === '/api/skills') {
        return json({ skills: [skill] });
      }

      if (method === 'GET' && url.pathname === '/api/skills/cli-e2e-test') {
        return json({
          skill,
          skillMdContent: '---\nname: skillshare-cli-e2e-test\ndescription: Run isolated E2E tests in devcontainer.\n---\n\n# Flow\n\nRun isolated E2E tests in devcontainer.',
          files: ['SKILL.md'],
          stats: {
            wordCount: 14,
            lineCount: 6,
            tokenCount: 32,
          },
        });
      }

      if (method === 'GET' && url.pathname === '/api/audit/cli-e2e-test') {
        return json({
          result: {
            skillName: 'cli-e2e-test',
            findings: [],
            riskScore: 24,
            riskLabel: 'high',
            threshold: 'warn',
            isBlocked: false,
          },
          summary: auditSummary,
        });
      }

      if (method === 'GET' && url.pathname === '/api/diff') {
        return json({ diffs: [] });
      }

      if (method === 'POST' && url.pathname === '/api/managed/hooks/collect') {
        return json({
          created: ['claude/pre-tool-use/bash.yaml'],
          overwritten: [],
          skipped: [],
        });
      }

      if (method === 'POST' && url.pathname === '/api/sync') {
        return json({
          results: [
            {
              resource: 'skills',
              target: 'claude',
              linked: ['a'],
              updated: [],
              skipped: [],
              pruned: [],
            },
            {
              resource: 'rules',
              target: 'claude',
              linked: ['claude/backend.md'],
              updated: [],
              skipped: [],
              pruned: [],
            },
            {
              resource: 'hooks',
              target: 'codex',
              linked: ['codex/pre-tool-use/bash.yaml'],
              updated: [],
              skipped: [],
              pruned: [],
            },
          ],
        });
      }

      return new Response(JSON.stringify({ error: `Unhandled mock: ${method} ${url.pathname}` }), {
        status: 404,
        headers: {
          'Content-Type': 'application/json',
        },
      });
    };
  });
}

test.beforeEach(async ({ page }) => {
  await installBrowserMocks(page);
});

test('smokes dashboard, hooks, and sync parity flows', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'Dashboard' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Rules', exact: true })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Hooks', exact: true })).toBeVisible();

  await page.goto('/rules/new');
  const ruleForm = page.locator('form').first();
  await ruleForm.getByLabel('Tool').fill('claude');
  await ruleForm.getByLabel('Relative Path').fill('claude/backend.md');
  await ruleForm.getByLabel('Content').fill('# Backend');
  await ruleForm.getByRole('button', { name: /save rule/i }).click();
  await expect(page).toHaveURL(/\/rules\/manage\/claude\/backend\.md$/);
  await expect(page.getByRole('button', { name: /delete rule/i })).toBeVisible();
  await expect(page.getByText('/tmp/home/.claude/rules/backend.md')).toBeVisible();

  await page.goto('/hooks');
  await expect(page.getByRole('heading', { name: 'Hooks' })).toBeVisible();
  await expect(page.getByRole('tab', { name: /hooks/i })).toBeVisible();
  await expect(page.getByText('claude/pre-tool-use/bash.yaml')).toBeVisible();

  await page.getByRole('tab', { name: /discovered/i }).click();
  await expect(page.getByRole('checkbox', { name: /collect bash/i })).toBeVisible();
  await page.getByRole('link', { name: 'claude project PreToolUse Bash' }).click();
  await expect(page).toHaveURL(/\/hooks\/discovered\//);
  await expect(page.getByRole('heading', { name: 'claude project PreToolUse Bash' })).toBeVisible();
  await expect(page.getByText('./bin/check')).toBeVisible();
  await page.getByRole('button', { name: /collect & edit/i }).click();
  await expect(page).toHaveURL(/\/hooks\/manage\/claude\/pre-tool-use\/bash\.yaml$/);

  await page.goto('/rules?mode=discovered');
  await expect(page.getByRole('heading', { name: 'Rules' })).toBeVisible();
  await expect(page.getByRole('tab', { name: /discovered \(1\)/i })).toBeVisible();
  await page.getByRole('button', { name: /view rule/i }).click();
  await expect(page).toHaveURL(/\/rules\/discovered\//);
  await expect(page.getByRole('heading', { name: 'backend-rule' })).toBeVisible();
  await expect(page.getByText('41 bytes')).toBeVisible();
  await page.getByRole('button', { name: /collect & edit/i }).click();
  await expect(page).toHaveURL(/\/rules\/manage\/claude\/backend\.md$/);
  await expect(page.getByRole('heading', { name: 'backend.md' })).toBeVisible();
  await expect(page.getByText('/tmp/home/.claude/rules/backend.md')).toBeVisible();

  await page.goto('/skills/cli-e2e-test');
  await expect(page.getByRole('heading', { name: 'cli-e2e-test' })).toBeVisible();
  await expect(page.getByText(/tokens/i)).toBeVisible();

  await page.goto('/hooks/manage/claude/pre-tool-use/bash.yaml');
  await expect(page.getByRole('heading', { name: 'claude/pre-tool-use/bash.yaml' })).toBeVisible();
  await expect(page.getByText(/compiled preview/i)).toBeVisible();
  await expect(page.getByText('/tmp/home/.claude/settings.json')).toBeVisible();

  await page.goto('/sync');
  await expect(page.getByRole('heading', { name: 'Sync' })).toBeVisible();
  await page.getByRole('button', { name: /sync now/i }).click();
  await expect(page.getByRole('heading', { name: /^skills$/i })).toBeVisible();
  await expect(page.getByRole('heading', { name: /^rules$/i })).toBeVisible();
  await expect(page.getByRole('heading', { name: /^hooks$/i })).toBeVisible();
});
