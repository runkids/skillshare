import type { ProjectList } from '../../api/client';
import { useT } from '../../i18n';
import AgentIcon from '../AgentIcon';
import { Select } from '../Select';
import { targetLabel } from '../mcp/mcpView';

interface Props {
  tools: ProjectList['tools'];
  /** Tools listed before the rest: the ones found on this machine */
  common: string[];
  selected: string[];
  onChange: (next: string[]) => void;
  disabled?: boolean;
}

/** The tools used in one project, from every tool that has a project path. */
export default function ProjectTools({ tools, common, selected, onChange, disabled }: Props) {
  const t = useT();
  const names = tools.map((x) => x.name);
  // The tools on this machine come first; the search box finds the rest.
  const ordered = [...names.filter((x) => common.includes(x)), ...names.filter((x) => !common.includes(x))];
  return (
    <Select
      ariaLabel={t('projects.targets')}
      values={selected}
      onChangeValues={(next) => onChange(names.filter((x) => next.includes(x)))}
      placeholder={t('projects.tools.choose')}
      options={ordered.map((name) => ({ value: name, label: targetLabel(name), icon: <AgentIcon target={name} size={13} /> }))}
      disabled={disabled}
      search={{ placeholder: t('projects.tools.search'), empty: t('projects.tools.noMatch'), clear: t('resources.search.clear') }}
    />
  );
}
