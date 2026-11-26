import { useCallback } from 'react';
import { useT } from '../i18n';
import { PRESETS, editedFrom, themeName, type CustomTheme } from '../lib/customTheme';

/** A custom theme's name to show: its own or its preset's, "Pine (modified)" once a preset is
 *  edited, otherwise "Custom". */
export function useThemeLabel() {
  const t = useT();
  return useCallback(
    (theme: CustomTheme) => {
      const own = themeName(theme);
      if (own) return own;
      const from = editedFrom(theme);
      return from
        ? t('settings.custom.modifiedName', { name: PRESETS[from].name })
        : t('settings.custom.name');
    },
    [t],
  );
}
