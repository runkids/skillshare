import { expect, test, type Page } from '@playwright/test';

async function installBrowserMocks(page: Page) {
  await page.addInitScript(() => {
    const originalFetch = window.fetch.bind(window);

    const json = (body: unknown, status = 200) =>
      new Response(JSON.stringify(body), {
        status,
        headers: {
          'Content-Type': 'application/json',
        },
      });

    const skill = {
      kind: 'skill',
      name: 'cli-e2e-test',
      flatName: 'cli-e2e-test',
      relPath: 'cli-e2e-test',
      sourcePath: '/tmp/skills/cli-e2e-test',
      isInRepo: false,
      targets: ['codex'],
    };

    const initialSkillMarkdown = [
      '---',
      'name: cli-e2e-test',
      'description: Browser test fixture',
      '---',
      '',
      '# Overview',
      '',
      'Original preview copy.',
      '',
      'See [notes](docs/notes.md).',
    ].join('\n');

    const fileContents: Record<string, string> = {
      'SKILL.md': initialSkillMarkdown,
      'docs/notes.md': [
        '# Notes',
        '',
        'Original sidebar markdown content.',
      ].join('\n'),
    };

    const contentTypes: Record<string, string> = {
      'SKILL.md': 'text/markdown',
      'docs/notes.md': 'text/markdown',
    };

    const buildSkillDetail = () => ({
      resource: skill,
      skillMdContent: fileContents['SKILL.md'],
      files: ['SKILL.md', 'docs/notes.md'],
    });

    const buildSkillFile = (filepath: string) => ({
      filename: filepath.split('/').pop() ?? filepath,
      content: fileContents[filepath],
      contentType: contentTypes[filepath] ?? 'text/plain',
    });

    const readBody = async (input: RequestInfo | URL, init?: RequestInit) => {
      if (typeof init?.body === 'string') return init.body;
      if (init?.body && typeof init.body === 'object' && 'text' in init.body && typeof init.body.text === 'function') {
        return init.body.text();
      }
      if (typeof input !== 'string' && !(input instanceof URL)) {
        return input.text();
      }
      return '';
    };

    window.fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
      const requestUrl = typeof input === 'string' || input instanceof URL ? input.toString() : input.url;
      const url = new URL(requestUrl, window.location.origin);
      const method = (init?.method ?? (typeof input === 'string' || input instanceof URL ? 'GET' : input.method ?? 'GET')).toUpperCase();

      if (!url.pathname.startsWith('/api/')) {
        return originalFetch(input, init);
      }

      if (method === 'GET' && url.pathname === '/api/overview') {
        return json({
          source: '/tmp/skills',
          skillCount: 1,
          topLevelCount: 1,
          targetCount: 1,
          mode: 'project',
          version: '1.0.0',
          managedRulesCount: 0,
          managedHooksCount: 0,
          trackedRepos: [],
          isProjectMode: true,
        });
      }

      if (method === 'GET' && url.pathname === '/api/resources') {
        return json({ resources: [skill] });
      }

      if (method === 'GET' && url.pathname === '/api/resources/cli-e2e-test') {
        return json(buildSkillDetail());
      }

      if (method === 'GET' && url.pathname === '/api/audit/cli-e2e-test') {
        return json({
          result: {
            skillName: 'cli-e2e-test',
            findings: [],
            riskScore: 0,
            riskLabel: 'low',
            threshold: 'warn',
            isBlocked: false,
          },
          summary: {
            total: 0,
            passed: 0,
            warning: 0,
            failed: 0,
            critical: 0,
            high: 0,
            medium: 0,
            low: 0,
            info: 0,
            threshold: 'warn',
            riskScore: 0,
            riskLabel: 'low',
          },
        });
      }

      if (method === 'GET' && url.pathname === '/api/diff') {
        return json({
          diffs: [
            {
              target: 'codex',
              items: [
                {
                  skill: 'cli-e2e-test',
                  action: 'linked',
                },
              ],
            },
          ],
        });
      }

      if (method === 'GET' && url.pathname === '/api/sync-matrix') {
        return json({
          entries: [
            {
              skill: 'cli-e2e-test',
              target: 'codex',
              status: 'synced',
            },
          ],
        });
      }

      const fileMatch = url.pathname.match(/^\/api\/resources\/cli-e2e-test\/files\/(.+)$/);
      if (fileMatch) {
        const filepath = decodeURIComponent(fileMatch[1]);
        if (!(filepath in fileContents)) {
          return json({ error: `Unknown file: ${filepath}` }, 404);
        }

        if (method === 'GET') {
          return json(buildSkillFile(filepath));
        }

        if (method === 'PUT') {
          const rawBody = await readBody(input, init);
          const body = JSON.parse(rawBody || '{}') as { content?: string };
          fileContents[filepath] = body.content ?? '';
          return json({
            filename: filepath.split('/').pop() ?? filepath,
            content: fileContents[filepath],
          });
        }
      }

      return json({ error: `Unhandled mock: ${method} ${url.pathname}` }, 404);
    };
  });
}

test.beforeEach(async ({ page }) => {
  await installBrowserMocks(page);
});

test('edits SKILL.md and sidebar markdown files from the skill detail route', async ({ page }) => {
  const updatedSkillText = 'Updated preview from Playwright flow.';
  const updatedSidebarText = 'Sidebar markdown saved from modal.';

  await page.goto('/resources/cli-e2e-test');

  await expect(page.getByRole('heading', { name: 'cli-e2e-test' })).toBeVisible();
  await expect(page.getByText('Original preview copy.')).toBeVisible();

  await page.getByRole('button', { name: 'Edit', exact: true }).click();
  await page.getByRole('button', { name: 'Open Raw' }).click();

  const skillEditor = page.getByLabel('Raw markdown editor');
  await expect(skillEditor).toBeVisible();
  await skillEditor.fill([
    '---',
    'name: cli-e2e-test',
    'description: Browser test fixture',
    '---',
    '',
    '# Overview',
    '',
    updatedSkillText,
    '',
    'See [notes](docs/notes.md).',
  ].join('\n'));

  await expect(page.getByRole('button', { name: 'Save', exact: true })).toBeEnabled();
  await page.getByRole('button', { name: 'Save', exact: true }).click();
  await expect(page.getByText('SKILL.md saved.')).toBeVisible();

  await page.getByRole('button', { name: 'Read', exact: true }).click();
  await expect(page.getByText(updatedSkillText)).toBeVisible();
  await expect(page.getByText('Original preview copy.')).toHaveCount(0);

  await page.getByRole('button', { name: 'Preview file docs/notes.md', exact: true }).click();
  const dialog = page.getByRole('dialog');
  await expect(dialog).toBeVisible();
  await expect(dialog.getByText('Original sidebar markdown content.')).toBeVisible();

  await dialog.getByRole('button', { name: 'Edit', exact: true }).click();
  await dialog.getByRole('button', { name: 'Open Raw' }).click();

  const modalEditor = dialog.getByLabel('Raw markdown editor');
  await expect(modalEditor).toBeVisible();
  await modalEditor.fill([
    '# Notes',
    '',
    updatedSidebarText,
  ].join('\n'));

  await dialog.getByRole('button', { name: 'Save', exact: true }).click();
  await dialog.getByRole('button', { name: 'Read', exact: true }).click();
  await expect(dialog.getByText(updatedSidebarText)).toBeVisible();
  await expect(dialog.getByText('Original sidebar markdown content.')).toHaveCount(0);
});
