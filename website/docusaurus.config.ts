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

  url: 'https://skillshare.runkids.work',
  baseUrl: '/',

  organizationName: 'runkids',
  projectName: 'skillshare',

  onBrokenLinks: 'throw',

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

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
      } satisfies Preset.Options,
    ],
  ],

  themeConfig: {
    image: 'img/social-card.png',
    colorMode: {
      defaultMode: 'dark',
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
          href: 'https://github.com/runkids/skillshare',
          label: 'GitHub',
          position: 'right',
        },
      ],
    },
    footer: {
      style: 'dark',
      links: [
        {
          title: 'Documentation',
          items: [
            {label: 'Getting Started', to: '/docs/intro'},
            {label: 'Commands', to: '/docs/commands/init'},
            {label: 'Team Edition', to: '/docs/guides/team-edition'},
          ],
        },
        {
          title: 'More',
          items: [
            {
              label: 'GitHub',
              href: 'https://github.com/runkids/skillshare',
            },
            {
              label: 'Releases',
              href: 'https://github.com/runkids/skillshare/releases',
            },
          ],
        },
      ],
      copyright: `MIT License · Built with Docusaurus`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
      additionalLanguages: ['bash', 'powershell', 'yaml'],
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
