// Feature Map data: every command grouped by the job it serves.
// Shared by the homepage teaser and the /features page.

export type FeatureItem = {
  cmd: string;
  what: string;
  kw: string;
  href: string;
};

export type FeatureGroup = {
  id: string;
  num: string;
  title: string;
  when: string;
  teaser: string;
  items: FeatureItem[];
  guides: {label: string; href: string}[];
};

const C = '/docs/reference/commands/';
const D = '/docs/';

export const COMMAND_COUNT = 34;
export const TARGET_COUNT = 66;

export const FEATURE_GROUPS: FeatureGroup[] = [
  {
    id: 'setup',
    num: '01',
    title: 'Get set up',
    when: 'first time, a new machine, or a new AI tool',
    teaser: 'init · install · sync',
    items: [
      {cmd: 'init', what: 'Detect installed tools and create the source folder', kw: 'start begin setup detect config', href: C + 'init'},
      {cmd: 'install', what: 'Add skills from GitHub, any git URL or a local path', kw: 'add get download repo clone', href: C + 'install'},
      {cmd: 'sync', what: 'Symlink the source into every target', kw: 'link symlink apply', href: C + 'sync'},
      {cmd: 'doctor', what: 'Check config, targets and symlink health', kw: 'diagnose broken health fix', href: C + 'doctor'},
      {cmd: 'completion', what: 'Shell completions for zsh, bash and fish', kw: 'shell tab autocomplete', href: C + 'completion'},
      {cmd: 'upgrade', what: 'Upgrade the skillshare binary itself', kw: 'version latest self update', href: C + 'upgrade'},
    ],
    guides: [
      {label: 'First sync in 5 minutes', href: D + 'getting-started/first-sync'},
      {label: 'From existing skills', href: D + 'getting-started/from-existing-skills'},
      {label: 'Windows notes', href: D + 'troubleshooting/windows'},
    ],
  },
  {
    id: 'sync',
    num: '02',
    title: 'Keep tools in sync',
    when: 'daily: you changed a skill, added a tool, something drifted',
    teaser: 'status · diff · collect',
    items: [
      {cmd: 'status', what: 'What is linked, local or out of date, per target', kw: 'state overview drift', href: C + 'status'},
      {cmd: 'diff', what: 'Differences between source and targets', kw: 'compare changed', href: C + 'diff'},
      {cmd: 'collect', what: 'Pull skills created inside a tool back to the source', kw: 'reverse import bidirectional cursor claude', href: C + 'collect'},
      {cmd: 'check', what: 'Which tracked repos have updates upstream', kw: 'outdated remote', href: C + 'check'},
      {cmd: 'update', what: 'Update tracked repos and installed skills', kw: 'refresh pull latest', href: C + 'update'},
      {cmd: 'uninstall', what: 'Remove a skill from source and all targets', kw: 'delete remove', href: C + 'uninstall'},
    ],
    guides: [
      {label: 'Daily workflow', href: D + 'how-to/daily-tasks/daily-workflow'},
      {label: 'Sync modes: merge vs symlink', href: D + 'understand/sync-modes'},
      {label: 'Source and targets', href: D + 'understand/source-and-targets'},
    ],
  },
  {
    id: 'find',
    num: '03',
    title: 'Find & install skills',
    when: 'looking for something someone already wrote',
    teaser: 'search · hub · list',
    items: [
      {cmd: 'search', what: 'Search GitHub for skills and install from the results', kw: 'discover github lookup', href: C + 'search'},
      {cmd: 'hub', what: 'Saved hub sources for organization-wide discovery', kw: 'index catalog registry org', href: C + 'hub'},
      {cmd: 'list', what: 'What you have installed, with filters', kw: 'show installed ls', href: C + 'list'},
      {cmd: 'tui', what: 'Toggle the interactive terminal UI globally', kw: 'interactive terminal browse', href: C + 'tui'},
      {cmd: 'ui', what: 'Web dashboard on localhost:19420', kw: 'browser web dashboard gui', href: C + 'ui'},
    ],
    guides: [
      {label: 'Skill discovery', href: D + 'how-to/daily-tasks/skill-discovery'},
      {label: 'Hub index', href: D + 'how-to/sharing/hub-index'},
      {label: 'URL formats', href: D + 'reference/appendix/url-formats'},
    ],
  },
  {
    id: 'share',
    num: '04',
    title: 'Share & teams',
    when: 'same skills across machines, a team, or one project',
    teaser: 'push · pull · --track',
    items: [
      {cmd: 'push / pull', what: 'Git-sync the source folder with any remote', kw: 'git remote machine laptop desktop github gitlab', href: C + 'push'},
      {cmd: 'commit', what: 'Commit source changes locally without pushing', kw: 'git save', href: C + 'commit'},
      {cmd: 'install --track', what: 'Follow an organization repo; updates flow in with update', kw: 'team org standards tracked company', href: D + 'understand/tracked-repositories'},
      {cmd: '-p / project', what: 'Skills that ship with a repo, in .skillshare/', kw: 'project mode local repo team', href: D + 'understand/project-skills'},
      {cmd: 'target', what: 'Add, remove or configure sync targets', kw: 'custom tool path add', href: C + 'target'},
    ],
    guides: [
      {label: 'Organization-wide skills', href: D + 'how-to/sharing/organization-sharing'},
      {label: 'Cross-machine sync', href: D + 'how-to/sharing/cross-machine-sync'},
      {label: 'Team onboarding recipe', href: D + 'how-to/recipes/team-onboarding-recipe'},
    ],
  },
  {
    id: 'safe',
    num: '05',
    title: 'Stay safe',
    when: 'before you trust a skill, or after something went wrong',
    teaser: 'audit · backup · trash',
    items: [
      {cmd: 'audit', what: 'Scan for prompt injection, exfiltration and destructive commands', kw: 'security scan threat malicious', href: C + 'audit'},
      {cmd: 'audit-rules', what: 'Tune thresholds and the rules that block installs', kw: 'security configure severity', href: C + 'audit-rules'},
      {cmd: 'backup / restore', what: 'Snapshot targets and roll back', kw: 'undo snapshot recover', href: C + 'backup'},
      {cmd: 'trash', what: 'Soft-deleted skills with TTL expiry', kw: 'recover deleted restore', href: C + 'trash'},
      {cmd: 'log', what: 'Operation and audit history', kw: 'history what happened oplog', href: C + 'log'},
    ],
    guides: [
      {label: 'Security guide', href: D + 'how-to/advanced/security'},
      {label: 'Audit engine', href: D + 'understand/audit-engine'},
      {label: 'Backup and restore', href: D + 'how-to/daily-tasks/backup-restore'},
    ],
  },
  {
    id: 'write',
    num: '06',
    title: 'Write & organize',
    when: 'authoring your own, or keeping a big set tidy',
    teaser: 'new · extras · mcp',
    items: [
      {cmd: 'new', what: 'Scaffold a skill with a valid SKILL.md', kw: 'create author scaffold template', href: C + 'new'},
      {cmd: 'analyze', what: 'Context window usage and skill quality per target', kw: 'tokens size budget quality', href: C + 'analyze'},
      {cmd: 'enable / disable', what: 'Turn skills off without removing them', kw: 'toggle off pause', href: C + 'enable'},
      {cmd: 'extras', what: 'Sync rules, commands and prompts alongside skills', kw: 'rules prompts agents non-skill', href: C + 'extras'},
      {cmd: 'mcp', what: 'Define an MCP server once, write it into each tool\'s own config', kw: 'model context protocol server connection json toml import', href: C + 'mcp'},
      {cmd: 'plugin', what: 'Install a complete plugin and choose which tools receive it', kw: 'package marketplace hooks bundle claude codex', href: C + 'plugin'},
      {cmd: '.skillignore', what: 'Include, exclude and filter what gets synced', kw: 'filter ignore exclude include', href: D + 'reference/filtering'},
    ],
    guides: [
      {label: 'Creating skills', href: D + 'how-to/daily-tasks/creating-skills'},
      {label: 'Organizing skills', href: D + 'how-to/daily-tasks/organizing-skills'},
      {label: 'Skill format', href: D + 'understand/skill-format'},
    ],
  },
];
