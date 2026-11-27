import { ApiError } from '../api/client';
import type { useT } from '../i18n';

/** The API error code for a file whose frontmatter the target editor will not touch. */
export const TARGET_FRONTMATTER = 'target_frontmatter';
export const isTargetRefusal = (err: unknown) =>
  err instanceof ApiError && err.code === TARGET_FRONTMATTER;

/** One item a target change left as it was: `detail` is its name when refused, else its error message. */
export type TargetFailure = { detail: string; refused: boolean };

/** Toast text for a target change that failed for some items: the counts, which items, and the fix for a refused file. */
export function targetFailureText(
  t: ReturnType<typeof useT>,
  updated: number,
  failures: TargetFailure[],
) {
  const values = {
    updated,
    failed: failures.length,
    details: failures.map((f) => f.detail).join('; '),
  };
  return failures.some((f) => f.refused)
    ? t('resources.targets.toast.partialRefused', {
        ...values,
        hint: t('resources.targets.error.target_frontmatter'),
      })
    : t('resources.targets.toast.partialDetails', values);
}
