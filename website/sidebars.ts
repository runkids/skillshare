import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  docsSidebar: [
    'intro',
    {
      type: 'category',
      label: 'Learn',
      collapsed: false,
      items: [
        'getting-started/index',
        'getting-started/first-sync',
        'getting-started/from-existing-skills',
        'getting-started/quick-reference',
        {
          type: 'category',
          label: 'Quickstarts',
          items: [
            'learn/with-claude-code',
            'learn/with-copilot',
            'learn/with-codex',
            'learn/with-multiple-tools',
            'learn/with-devcontainer',
            'learn/with-playground',
          ],
        },
      ],
    },
    {
      type: 'category',
      label: 'How-To',
      items: [
        'how-to/index',
        {
          type: 'category',
          label: 'Daily Tasks',
          items: [
            'how-to/daily-tasks/daily-workflow',
            'how-to/daily-tasks/skill-discovery',
            'how-to/daily-tasks/backup-restore',
            'how-to/daily-tasks/project-workflow',
            'how-to/daily-tasks/creating-skills',
            'how-to/daily-tasks/organizing-skills',
            'how-to/daily-tasks/best-practices',
          ],
        },
        {
          type: 'category',
          label: 'Sharing & Teams',
          items: [
            'how-to/sharing/project-setup',
            'how-to/sharing/organization-sharing',
            'how-to/sharing/cross-machine-sync',
            'how-to/sharing/hub-index',
          ],
        },
        {
          type: 'category',
          label: 'Advanced',
          items: [
            'how-to/advanced/migration',
            'how-to/advanced/local-first',
            'how-to/advanced/docker-sandbox',
            'how-to/advanced/security',
          ],
        },
        {
          type: 'category',
          label: 'Recipes',
          items: [
            'how-to/recipes/index',
            'how-to/recipes/ci-cd-skill-validation',
            'how-to/recipes/private-enterprise-skills',
            'how-to/recipes/skill-per-project-workflow',
            'how-to/recipes/cross-machine-sync-recipe',
            'how-to/recipes/team-onboarding-recipe',
          ],
        },
      ],
    },
    {
      type: 'category',
      label: 'Understand',
      items: [
        'understand/index',
        'understand/source-and-targets',
        'understand/sync-modes',
        'understand/tracked-repositories',
        'understand/skill-format',
        'understand/project-skills',
        'understand/declarative-manifest',
        {
          type: 'category',
          label: 'Design Philosophy',
          items: [
            'understand/philosophy/why-local-first',
            'understand/philosophy/security-first',
            'understand/philosophy/comparison',
            'understand/philosophy/skill-design',
            'understand/philosophy/sync-modes-explained',
          ],
        },
      ],
    },
    {
      type: 'category',
      label: 'Reference',
      collapsed: true,
      items: [
        'reference/index',
        {
          type: 'category',
          label: 'Commands',
          items: [
            'reference/commands/index',
            {
              type: 'category',
              label: 'Core',
              items: [
                'reference/commands/init',
                'reference/commands/install',
                'reference/commands/uninstall',
                'reference/commands/list',
                'reference/commands/search',
                'reference/commands/sync',
                'reference/commands/status',
              ],
            },
            {
              type: 'category',
              label: 'Skill Management',
              items: [
                'reference/commands/new',
                'reference/commands/check',
                'reference/commands/update',
                'reference/commands/upgrade',
              ],
            },
            {
              type: 'category',
              label: 'Target Management',
              items: [
                'reference/commands/target',
                'reference/commands/diff',
              ],
            },
            {
              type: 'category',
              label: 'Sync Operations',
              items: [
                'reference/commands/collect',
                'reference/commands/backup',
                'reference/commands/restore',
                'reference/commands/trash',
                'reference/commands/push',
                'reference/commands/pull',
              ],
            },
            {
              type: 'category',
              label: 'Security & Utilities',
              items: [
                'reference/commands/audit',
                'reference/commands/hub',
                'reference/commands/log',
                'reference/commands/doctor',
                'reference/commands/ui',
                'reference/commands/version',
              ],
            },
          ],
        },
        {
          type: 'category',
          label: 'Targets',
          items: [
            'reference/targets/index',
            'reference/targets/supported-targets',
            'reference/targets/adding-custom-targets',
            'reference/targets/configuration',
          ],
        },
        {
          type: 'category',
          label: 'Appendix',
          items: [
            'reference/appendix/index',
            'reference/appendix/environment-variables',
            'reference/appendix/file-structure',
            'reference/appendix/url-formats',
          ],
        },
      ],
    },
    {
      type: 'category',
      label: 'Troubleshooting',
      items: [
        'troubleshooting/index',
        'troubleshooting/troubleshooting-workflow',
        'troubleshooting/common-errors',
        'troubleshooting/windows',
        'troubleshooting/faq',
      ],
    },
  ],
};

export default sidebars;
