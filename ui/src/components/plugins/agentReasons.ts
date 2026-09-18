import type { PluginCandidate, PluginTarget, PluginTargetDefinition } from '../../api/plugins';
import type { useT } from '../../i18n';

type T = ReturnType<typeof useT>;

/** Why each Agent cannot take this plugin, or '' when it can. One place, so the list and the dialog never disagree. */
export function agentReasons(candidate: PluginCandidate | undefined, definitions: Record<string, PluginTargetDefinition>, isProjectMode: boolean, t: T) {
  return (Object.keys(definitions) as PluginTarget[]).map((target) => {
    const definition = definitions[target];
    const info = candidate?.targetInfo?.[target];
    const reason = (info?.problemKey ? t(info.problemKey, info.problemArgs, info.problem) : info?.problem)
      || (!candidate?.targets.includes(target) ? t('plugins.unsupported')
        : isProjectMode && !definition.project ? t('plugins.globalOnly')
          : !definition.operations.includes('add') ? (definition.reasonKey ? t(definition.reasonKey, undefined, definition.reason) : definition.reason) || t('plugins.unsupported')
            : '');
    return { target, definition, reason };
  });
}
