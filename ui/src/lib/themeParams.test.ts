import { describe, expect, it } from 'vitest';
import { themeFromSearch } from './themeParams';

describe('themeFromSearch', () => {
  it('reads a mode from ?theme= and leaves the style alone', () => {
    expect(themeFromSearch('?theme=dark')).toEqual({ mode: 'dark' });
  });

  it('reads a style from ?theme= and implies light mode', () => {
    expect(themeFromSearch('?theme=Playful')).toEqual({ style: 'playful', mode: 'light' });
  });

  it('reads Dark + Clean from ?mode= and ?style=', () => {
    expect(themeFromSearch('?mode=dark&style=clean')).toEqual({ mode: 'dark', style: 'clean' });
  });

  it('lets ?mode= and ?style= win over ?theme=', () => {
    expect(themeFromSearch('?theme=playful&mode=dark')).toEqual({ style: 'playful', mode: 'dark' });
    expect(themeFromSearch('?theme=dark&style=clean')).toEqual({ mode: 'dark', style: 'clean' });
  });

  it('reads Custom only from ?style=', () => {
    expect(themeFromSearch('?style=custom')).toEqual({ style: 'custom' });
    expect(themeFromSearch('?theme=custom')).toEqual({});
  });

  it('ignores unknown values and other params', () => {
    expect(themeFromSearch('?tab=updates&mode=sepia&style=neon&theme=x')).toEqual({});
  });
});
