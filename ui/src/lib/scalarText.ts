/** Text for scalar values in loosely typed metadata; never coerce objects to "[object Object]". */
export function scalarText(value: unknown): string {
  return typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean'
    ? String(value)
    : '';
}
