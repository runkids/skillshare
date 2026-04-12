const SUBSTITUTION_TOKEN_SOURCE = String.raw`\$(?:[0-9](?![0-9A-Za-z])|[A-Za-z_][A-Za-z0-9_]*(?:\[[A-Za-z0-9_]+\])?|\{[A-Za-z_][A-Za-z0-9_]*(?:\[[A-Za-z0-9_]+\])?\})`;

const substitutionTokenPattern = new RegExp(SUBSTITUTION_TOKEN_SOURCE, 'g');
const fullSubstitutionTokenPattern = new RegExp(`^${SUBSTITUTION_TOKEN_SOURCE}$`);

export type SubstitutionTokenMatch = {
  start: number;
  end: number;
  value: string;
};

export function getSubstitutionTokenMatches(text: string): SubstitutionTokenMatch[] {
  if (!text) return [];

  return Array.from(text.matchAll(substitutionTokenPattern)).map((match) => ({
    start: match.index ?? 0,
    end: (match.index ?? 0) + match[0].length,
    value: match[0],
  }));
}

export function getFirstSubstitutionTokenMatch(text: string): SubstitutionTokenMatch | null {
  return getSubstitutionTokenMatches(text)[0] ?? null;
}

export function isFullSubstitutionToken(text: string): boolean {
  return fullSubstitutionTokenPattern.test(text.trim());
}
