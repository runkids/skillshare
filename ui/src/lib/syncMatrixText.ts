import type { SyncMatrixEntry } from '../api/client';
import type { TranslationParams } from '../i18n';

export function syncMatrixReasonText(
  entry: SyncMatrixEntry,
  t: (key: string, params?: TranslationParams, fallback?: string) => string,
): string {
  if (entry.reasonCode) {
    return t(entry.reasonCode, entry.reasonParams, entry.reason);
  }
  return entry.reason;
}
