export function updateCheckTitle(failed: boolean, checked: boolean, available: number) {
  // Updates lead even when some checks failed; the failures get their own line.
  if (available > 0) return 'update.hero.available';
  if (failed) return 'update.check.error';
  if (!checked) return 'update.tab.notChecked';
  return 'update.hero.allCurrent';
}
