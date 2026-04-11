import type { HookItem } from '../api/client';

export type HookDiscoveryGroup = {
  id: string;
  sourceTool: string;
  scope: HookItem['scope'];
  event: string;
  matcher: string;
  hooks: HookItem[];
  collectible: boolean;
  collectReason?: string;
};

export function getHookActionPayload(hook: HookItem): string {
  switch (hook.actionType) {
    case 'command':
      return hook.command ?? '';
    case 'http':
      return hook.url ?? '';
    case 'prompt':
    case 'agent':
      return hook.prompt ?? '';
    default:
      return '';
  }
}

export function getHookDiscoveryGroupId(hook: HookItem): string {
  return hook.groupId ?? `${hook.sourceTool}:${hook.scope}:${hook.event}:${hook.matcher ?? '*'}`;
}

export function formatHookDiscoveryGroupTitle(group: HookDiscoveryGroup): string {
  return group.matcher && group.matcher !== 'All'
    ? `${group.sourceTool} ${group.scope} ${group.event} ${group.matcher}`
    : `${group.sourceTool} ${group.scope} ${group.event}`;
}

export function groupDiscoveredHooks(hooks: HookItem[]): HookDiscoveryGroup[] {
  const grouped = new Map<string, HookDiscoveryGroup>();

  for (const hook of hooks) {
    const id = getHookDiscoveryGroupId(hook);
    const current = grouped.get(id);
    const collectReason = hook.collectReason ?? current?.collectReason;
    const collectible = hook.collectible ?? current?.collectible ?? false;

    if (current) {
      current.hooks.push(hook);
      current.collectible = collectible;
      current.collectReason = collectReason;
      continue;
    }

    grouped.set(id, {
      id,
      sourceTool: hook.sourceTool,
      scope: hook.scope,
      event: hook.event,
      matcher: hook.matcher ?? 'All',
      hooks: [hook],
      collectible,
      collectReason,
    });
  }

  return Array.from(grouped.values());
}
