import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  docsSidebar: [
    'intro',
    {
      type: 'category',
      label: 'Commands',
      items: [
        'commands/init',
        'commands/sync',
        'commands/install',
        'commands/new',
        'commands/search',
      ],
    },
    {
      type: 'category',
      label: 'Guides',
      items: [
        'guides/team-edition',
        'guides/targets',
        'guides/cross-machine',
        'guides/configuration',
      ],
    },
    'faq',
  ],
};

export default sidebars;
