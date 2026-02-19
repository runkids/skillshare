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
        indexBlog: false,
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
        blog: false,
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
          label: 'Docs',
        },
        {
          to: '/docs/commands',
          label: 'Commands',
          position: 'left',
        },
        {
          to: '/docs/guides',
          label: 'Guides',
          position: 'left',
        },
        {
          to: '/docs/targets',
          label: 'Targets',
          position: 'left',
        },
        {
          href: 'https://github.com/runkids/skillshare/releases',
          label: 'Changelog',
          position: 'right',
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
          title: 'Documentation',
          items: [
            {label: 'Getting Started', to: '/docs/getting-started'},
            {label: 'Commands', to: '/docs/commands'},
            {label: 'Guides', to: '/docs/guides'},
            {label: 'FAQ', to: '/docs/troubleshooting/faq'},
          ],
        },
        {
          title: 'Commands',
          items: [
            {label: 'init', to: '/docs/commands/init'},
            {label: 'sync', to: '/docs/commands/sync'},
            {label: 'install', to: '/docs/commands/install'},
            {label: 'doctor', to: '/docs/commands/doctor'},
          ],
        },
        {
          title: 'Community',
          items: [
            {
              label: 'GitHub',
              href: 'https://github.com/runkids/skillshare',
            },
            {
              label: 'Issues',
              href: 'https://github.com/runkids/skillshare/issues',
            },
            {
              label: 'Discussions',
              href: 'https://github.com/runkids/skillshare/discussions',
            },
          ],
        },
        {
          title: 'More',
          items: [
            {
              label: 'Releases',
              href: 'https://github.com/runkids/skillshare/releases',
            },
            {
              label: 'Contributing',
              href: 'https://github.com/runkids/skillshare/blob/main/CONTRIBUTING.md',
            },
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
