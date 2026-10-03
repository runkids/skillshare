/** A pasted `npx skills add` command (vercel-labs/skills), reduced to what the install dialog uses. */
export type SkillsAddCommand = {
  source: string;
  /** Skill names picked with --skill; empty means no pick (or '*'). */
  skills: string[];
};

const RUNNERS = [['npx'], ['pnpx'], ['bunx'], ['pnpm', 'dlx'], ['yarn', 'dlx'], ['bun', 'x']];

function tokens(value: string): string[] {
  return [...value.matchAll(/"([^"]*)"|'([^']*)'|(\S+)/g)].map((m) => m[1] ?? m[2] ?? m[3]);
}

/**
 * Parses `npx skills add <source> --skill a --skill=b -s c d`. Install-location flags
 * (-g, -a, -y, --copy, --all) are dropped: targets decide those in skillshare.
 * Returns null when the value is not such a command.
 */
export function parseSkillsAddCommand(value: string): SkillsAddCommand | null {
  let args = tokens(value.trim().replace(/^\$\s+/, ''));
  const runner = RUNNERS.find((r) => r.every((word, i) => args[i] === word));
  if (runner) {
    args = args.slice(runner.length);
    while (args[0] === '-y' || args[0] === '--yes') args = args.slice(1);
  }
  if (!/^skills(@\S+)?$/.test(args[0] ?? '') || args[1] !== 'add') return null;

  let source = '';
  const skills: string[] = [];
  // Which variadic flag the following bare words belong to.
  let list: 'skill' | 'agent' | null = null;
  for (const arg of args.slice(2)) {
    if (arg.startsWith('-')) {
      const [flag, inline] = arg.split(/=(.*)/s);
      list = flag === '--skill' || flag === '-s' ? 'skill' : flag === '--agent' || flag === '-a' ? 'agent' : null;
      if (inline !== undefined && list === 'skill') skills.push(inline);
      if (inline !== undefined) list = null;
    } else if (list === 'skill') {
      skills.push(arg);
    } else if (list !== 'agent' && !source) {
      source = arg;
    }
  }
  if (!source) return null;
  return { source, skills: skills.includes('*') ? [] : skills };
}
