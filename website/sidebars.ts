import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  docsSidebar: [
    'intro',
    {
      type: 'category',
      label: 'Getting Started',
      collapsed: false,
      items: [
        'getting-started/index',
        'getting-started/first-sync',
        'getting-started/from-existing-skills',
        'getting-started/quick-reference',
      ],
    },
    {
      type: 'category',
      label: 'Concepts',
      items: [
        'concepts/index',
        'concepts/source-and-targets',
        'concepts/sync-modes',
        'concepts/tracked-repositories',
        'concepts/skill-format',
        'concepts/project-skills',
      ],
    },
    {
      type: 'category',
      label: 'Workflows',
      items: [
        'workflows/index',
        'workflows/daily-workflow',
        'workflows/skill-discovery',
        'workflows/backup-restore',
        'workflows/troubleshooting-workflow',
        'workflows/project-workflow',
      ],
    },
    {
      type: 'category',
      label: 'Guides',
      items: [
        'guides/index',
        'guides/creating-skills',
        'guides/organization-sharing',
        'guides/cross-machine-sync',
        'guides/migration',
        'guides/organizing-skills',
        'guides/best-practices',
        'guides/project-setup',
        'guides/docker-sandbox',
        'guides/comparison',
      ],
    },
    {
      type: 'category',
      label: 'Commands',
      items: [
        'commands/index',
        {
          type: 'category',
          label: 'Core',
          items: [
            'commands/init',
            'commands/install',
            'commands/uninstall',
            'commands/list',
            'commands/search',
            'commands/sync',
            'commands/status',
          ],
        },
        {
          type: 'category',
          label: 'Skill Management',
          items: [
            'commands/new',
            'commands/check',
            'commands/update',
            'commands/upgrade',
          ],
        },
        {
          type: 'category',
          label: 'Target Management',
          items: [
            'commands/target',
            'commands/diff',
          ],
        },
        {
          type: 'category',
          label: 'Sync Operations',
          items: [
            'commands/collect',
            'commands/backup',
            'commands/restore',
            'commands/trash',
            'commands/push',
            'commands/pull',
          ],
        },
        {
          type: 'category',
          label: 'Security & Utilities',
          items: [
            'commands/audit',
            'commands/log',
            'commands/doctor',
            'commands/ui',
          ],
        },
      ],
    },
    {
      type: 'category',
      label: 'Targets',
      items: [
        'targets/index',
        'targets/supported-targets',
        'targets/adding-custom-targets',
        'targets/configuration',
      ],
    },
    {
      type: 'category',
      label: 'Troubleshooting',
      items: [
        'troubleshooting/index',
        'troubleshooting/common-errors',
        'troubleshooting/windows',
        'troubleshooting/faq',
      ],
    },
    {
      type: 'category',
      label: 'Reference',
      items: [
        'reference/index',
        'reference/environment-variables',
        'reference/file-structure',
      ],
    },
  ],
};

export default sidebars;
