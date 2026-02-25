import {themes as prismThemes} from 'prism-react-renderer';
import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

const config: Config = {
  title: 'skillshare',
  tagline: 'One source of truth for AI CLI skills. Sync everywhere with one command.',
  favicon: 'img/favicon.png',

  future: {
    v4: true,
  },

  url: 'https://skillshare.runkids.cc',
  baseUrl: '/',

  organizationName: 'runkids',
  projectName: 'skillshare',

  onBrokenLinks: 'throw',

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  markdown: {
    mermaid: true,
  },

  themes: [
    '@docusaurus/theme-mermaid',
    [
      require.resolve('@easyops-cn/docusaurus-search-local'),
      {
        hashed: true,
        indexBlog: true,
        language: ['en'],
        highlightSearchTermsOnTargetPage: true,
        searchResultLimits: 8,
      },
    ],
  ],

  presets: [
    [
      'classic',
      {
        docs: {
          sidebarPath: './sidebars.ts',
          editUrl: 'https://github.com/runkids/skillshare/tree/main/website/',
        },
        blog: {
          showReadingTime: true,
          blogSidebarCount: 10,
          feedOptions: {
            type: 'all',
            copyright: `MIT License`,
          },
          editUrl: 'https://github.com/runkids/skillshare/tree/main/website/',
        },
        theme: {
          customCss: './src/css/custom.css',
        },
        sitemap: {
          changefreq: 'weekly',
          priority: 0.5,
          filename: 'sitemap.xml',
        },
      } satisfies Preset.Options,
    ],
  ],

  plugins: [
    [
      '@docusaurus/plugin-client-redirects',
      {
        redirects: [
          // Workflows → How-To / Daily Tasks
          {from: '/docs/workflows', to: '/docs/how-to/daily-tasks'},
          {from: '/docs/workflows/daily-workflow', to: '/docs/how-to/daily-tasks/daily-workflow'},
          {from: '/docs/workflows/skill-discovery', to: '/docs/how-to/daily-tasks/skill-discovery'},
          {from: '/docs/workflows/backup-restore', to: '/docs/how-to/daily-tasks/backup-restore'},
          {from: '/docs/workflows/project-workflow', to: '/docs/how-to/daily-tasks/project-workflow'},
          // Guides → How-To
          {from: '/docs/guides', to: '/docs/how-to'},
          {from: '/docs/guides/creating-skills', to: '/docs/how-to/daily-tasks/creating-skills'},
          {from: '/docs/guides/skill-design', to: '/docs/understand/philosophy/skill-design'},
          {from: '/docs/guides/organizing-skills', to: '/docs/how-to/daily-tasks/organizing-skills'},
          {from: '/docs/guides/best-practices', to: '/docs/how-to/daily-tasks/best-practices'},
          {from: '/docs/guides/project-setup', to: '/docs/how-to/sharing/project-setup'},
          {from: '/docs/guides/organization-sharing', to: '/docs/how-to/sharing/organization-sharing'},
          {from: '/docs/guides/cross-machine-sync', to: '/docs/how-to/sharing/cross-machine-sync'},
          {from: '/docs/guides/hub-index', to: '/docs/how-to/sharing/hub-index'},
          {from: '/docs/guides/migration', to: '/docs/how-to/advanced/migration'},
          {from: '/docs/guides/comparison', to: '/docs/understand/philosophy/comparison'},
          {from: '/docs/guides/local-first', to: '/docs/how-to/advanced/local-first'},
          {from: '/docs/guides/docker-sandbox', to: '/docs/how-to/advanced/docker-sandbox'},
          {from: '/docs/guides/security', to: '/docs/how-to/advanced/security'},
          // Commands → Reference / Commands
          {from: '/docs/commands', to: '/docs/reference/commands'},
          {from: '/docs/commands/init', to: '/docs/reference/commands/init'},
          {from: '/docs/commands/install', to: '/docs/reference/commands/install'},
          {from: '/docs/commands/uninstall', to: '/docs/reference/commands/uninstall'},
          {from: '/docs/commands/list', to: '/docs/reference/commands/list'},
          {from: '/docs/commands/search', to: '/docs/reference/commands/search'},
          {from: '/docs/commands/sync', to: '/docs/reference/commands/sync'},
          {from: '/docs/commands/status', to: '/docs/reference/commands/status'},
          {from: '/docs/commands/new', to: '/docs/reference/commands/new'},
          {from: '/docs/commands/check', to: '/docs/reference/commands/check'},
          {from: '/docs/commands/update', to: '/docs/reference/commands/update'},
          {from: '/docs/commands/upgrade', to: '/docs/reference/commands/upgrade'},
          {from: '/docs/commands/target', to: '/docs/reference/commands/target'},
          {from: '/docs/commands/diff', to: '/docs/reference/commands/diff'},
          {from: '/docs/commands/collect', to: '/docs/reference/commands/collect'},
          {from: '/docs/commands/backup', to: '/docs/reference/commands/backup'},
          {from: '/docs/commands/restore', to: '/docs/reference/commands/restore'},
          {from: '/docs/commands/trash', to: '/docs/reference/commands/trash'},
          {from: '/docs/commands/push', to: '/docs/reference/commands/push'},
          {from: '/docs/commands/pull', to: '/docs/reference/commands/pull'},
          {from: '/docs/commands/audit', to: '/docs/reference/commands/audit'},
          {from: '/docs/commands/hub', to: '/docs/reference/commands/hub'},
          {from: '/docs/commands/log', to: '/docs/reference/commands/log'},
          {from: '/docs/commands/doctor', to: '/docs/reference/commands/doctor'},
          {from: '/docs/commands/ui', to: '/docs/reference/commands/ui'},
          {from: '/docs/commands/version', to: '/docs/reference/commands/version'},
          // Targets → Reference / Targets
          {from: '/docs/targets', to: '/docs/reference/targets'},
          {from: '/docs/targets/supported-targets', to: '/docs/reference/targets/supported-targets'},
          {from: '/docs/targets/adding-custom-targets', to: '/docs/reference/targets/adding-custom-targets'},
          {from: '/docs/targets/configuration', to: '/docs/reference/targets/configuration'},
          // Concepts → Understand
          {from: '/docs/concepts', to: '/docs/understand'},
          {from: '/docs/concepts/source-and-targets', to: '/docs/understand/source-and-targets'},
          {from: '/docs/concepts/sync-modes', to: '/docs/understand/sync-modes'},
          {from: '/docs/concepts/tracked-repositories', to: '/docs/understand/tracked-repositories'},
          {from: '/docs/concepts/skill-format', to: '/docs/understand/skill-format'},
          {from: '/docs/concepts/project-skills', to: '/docs/understand/project-skills'},
          {from: '/docs/concepts/declarative-manifest', to: '/docs/understand/declarative-manifest'},
          // Reference appendix
          {from: '/docs/reference/environment-variables', to: '/docs/reference/appendix/environment-variables'},
          {from: '/docs/reference/file-structure', to: '/docs/reference/appendix/file-structure'},
          {from: '/docs/reference/url-formats', to: '/docs/reference/appendix/url-formats'},
        ],
      },
    ],
  ],

  themeConfig: {
    image: 'img/social-card.png',
    colorMode: {
      defaultMode: 'light',
      respectPrefersColorScheme: false,
    },
    navbar: {
      title: 'skillshare',
      logo: {
        alt: 'skillshare',
        src: 'img/logo.png',
      },
      items: [
        {
          type: 'docSidebar',
          sidebarId: 'docsSidebar',
          position: 'left',
          label: 'Learn',
        },
        {
          to: '/docs/how-to',
          label: 'How-To',
          position: 'left',
        },
        {
          to: '/docs/reference',
          label: 'Reference',
          position: 'left',
        },
        {
          to: '/blog',
          label: 'Blog',
          position: 'left',
        },
        {
          to: '/changelog',
          label: 'Changelog',
          position: 'left',
        },
        {
          href: 'https://github.com/runkids/skillshare',
          label: 'GitHub',
          position: 'right',
        },
      ],
    },
    footer: {
      style: 'light',
      links: [
        {
          title: 'Learn',
          items: [
            {label: 'Getting Started', to: '/docs/getting-started'},
            {label: 'FAQ', to: '/docs/troubleshooting/faq'},
          ],
        },
        {
          title: 'How-To',
          items: [
            {label: 'Daily Workflow', to: '/docs/how-to/daily-tasks/daily-workflow'},
            {label: 'Team Sharing', to: '/docs/how-to/sharing/organization-sharing'},
            {label: 'Backup & Restore', to: '/docs/how-to/daily-tasks/backup-restore'},
          ],
        },
        {
          title: 'Reference',
          items: [
            {label: 'Commands', to: '/docs/reference/commands'},
            {label: 'Targets', to: '/docs/reference/targets'},
            {label: 'Environment Variables', to: '/docs/reference/appendix/environment-variables'},
            {label: 'Troubleshooting', to: '/docs/troubleshooting'},
          ],
        },
        {
          title: 'Community',
          items: [
            {label: 'GitHub', href: 'https://github.com/runkids/skillshare'},
            {label: 'Issues', href: 'https://github.com/runkids/skillshare/issues'},
            {label: 'Discussions', href: 'https://github.com/runkids/skillshare/discussions'},
            {label: 'Blog', to: '/blog'},
          ],
        },
      ],
      copyright: `MIT License · Built with Docusaurus`,
    },
    mermaid: {
      theme: {light: 'default', dark: 'dark'},
      options: {
        look: 'handDrawn',
        handDrawnSeed: 42,
        flowchart: {curve: 'basis', padding: 20},
      },
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
      additionalLanguages: ['bash', 'powershell', 'yaml'],
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
