export function updateCheckTitle(failed: boolean, checked: boolean, available: number) {
  if (failed) return 'update.check.error';
  if (!checked) return 'update.tab.notChecked';
  return available > 0 ? 'update.hero.available' : 'update.hero.allCurrent';
}
