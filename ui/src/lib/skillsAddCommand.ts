/** A pasted `npx skills add` command (vercel-labs/skills), reduced to what the install dialog uses. */
export type SkillsAddCommand = {
  source: string;
  /** Skill names picked with --skill; empty means no pick (or '*'). */
  skills: string[];
};

const RUNNERS = [['npx'], ['pnpx'], ['bunx'], ['pnpm', 'dlx'], ['yarn', 'dlx'], ['bun', 'x']];

// Shell-style words: quotes group, and a backslash escapes only whitespace, a quote or a backslash,
// so `my\ skill` stays one word while Windows paths such as C:\Users keep their backslashes.
function tokens(value: string): string[] {
  const words: string[] = [];
  let word: string | null = null;
  let quote: string | null = null;
  for (let i = 0; i < value.length; i++) {
    const c = value[i];
    if (c === '\\' && quote !== "'" && /[\s"'\\]/.test(value[i + 1] ?? '')) {
      word = (word ?? '') + value[++i];
    } else if (quote) {
      if (c === quote) quote = null;
      else word += c;
    } else if (/\s/.test(c)) {
      if (word !== null) words.push(word);
      word = null;
    } else {
      word ??= '';
      if (c === '"' || c === "'") quote = c;
      else word += c;
    }
  }
  if (word !== null) words.push(word);
  return words;
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
  // GitHub shorthand owner/repo@skill names one skill, as upstream's source parser reads it.
  const shorthand = source.match(/^([^/:]+\/[^/@]+)@(.+)$/);
  if (shorthand) {
    source = shorthand[1];
    skills.push(shorthand[2]);
  }
  return { source, skills: skills.includes('*') ? [] : skills };
}

/**
 * The discovered items a pasted command asks to preselect, or null to keep the default selection:
 * no command, a different source, no --skill (or '*'), or no name that matches.
 * Names match case-insensitively, as upstream does.
 */
export function requestedSkills<T extends { name: string }>(command: SkillsAddCommand | null, source: string, items: T[]): T[] | null {
  if (command?.source !== source) return null;
  const wanted = new Set(command.skills.map((s) => s.toLowerCase()));
  const picked = items.filter((i) => wanted.has(i.name.toLowerCase()));
  return picked.length > 0 ? picked : null;
}
