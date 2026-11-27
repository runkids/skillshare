/** The items a logo stack draws, and how many fell past `max`. No `max` draws them all. */
export function splitStack<T>(items: readonly T[], max = items.length) {
  return { shown: items.slice(0, max), rest: Math.max(0, items.length - max) };
}
