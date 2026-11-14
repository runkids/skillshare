import type { ResolvedMode, Style } from '../context/ThemeContext';

/** The theme a URL asks for.
 *  `?theme=` is the single-value form: `dark`/`light` set the mode; `clean`/`playful` set the
 *  style and imply light mode. `?mode=` and `?style=` name one axis each and win over `?theme=`,
 *  so an embedding app can ask for Dark + Clean or Playful dark. */
export function themeFromSearch(search: string): { style?: Style; mode?: ResolvedMode } {
  const params = new URLSearchParams(search);
  const get = (key: string) => params.get(key)?.toLowerCase();
  const asked: { style?: Style; mode?: ResolvedMode } = {};

  const theme = get('theme');
  if (theme === 'dark' || theme === 'light') asked.mode = theme;
  if (theme === 'clean' || theme === 'playful') {
    asked.style = theme;
    asked.mode = 'light';
  }

  const mode = get('mode');
  if (mode === 'dark' || mode === 'light') asked.mode = mode;
  const style = get('style');
  if (style === 'clean' || style === 'playful') asked.style = style;
  return asked;
}
