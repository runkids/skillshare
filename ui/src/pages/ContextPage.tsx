import { useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import SkillCostPanel from '../components/context/SkillCostPanel';
import MCPCostPanel from '../components/context/MCPCostPanel';
import PageHeader from '../components/PageHeader';
import { useT } from '../i18n';
import { segSlide } from '../lib/segSlide';

/** What skills and MCP servers cost a model's context. Each tab has its own list: skill targets and MCP targets differ. */
export default function ContextPage() {
  const t = useT();
  const [params, setParams] = useSearchParams();
  const tab = params.get('tab') === 'mcp' ? 'mcp' : 'skills';
  const setTab = (next: 'skills' | 'mcp') =>
    setParams(
      (p) => {
        if (next === 'mcp') p.set('tab', 'mcp');
        else p.delete('tab');
        return p;
      },
      { replace: true },
    );
  // Each panel puts its own picker in the header's actions, since the picker needs the panel's data.
  const [actions, setActions] = useState<HTMLDivElement | null>(null);

  return (
    <div className="ss-wrap animate-page-in">
      <PageHeader
        title={t('context.title')}
        subtitle={t(tab === 'mcp' ? 'context.mcp.intro' : 'analyze.intro')}
        actionsOnTitleRow
        actions={
          <>
            {/* Tabs on the title row, each tab's picker under them: the tabs stay put whatever the picker's width. */}
            <div ref={segSlide} className="ss-seg" role="tablist">
              {(['skills', 'mcp'] as const).map((key) => (
                <button
                  key={key}
                  type="button"
                  role="tab"
                  aria-selected={tab === key}
                  className={tab === key ? 'on' : ''}
                  onClick={() => setTab(key)}
                >
                  {t(key === 'mcp' ? 'context.tab.mcp' : 'context.tab.skills')}
                </button>
              ))}
            </div>
            <div ref={setActions} className="flex flex-wrap items-center justify-end gap-2" />
          </>
        }
      />
      {/* Keyed, so switching tabs fades the new content in. */}
      <div key={tab} className="animate-page-in">
        {tab === 'mcp' ? <MCPCostPanel actions={actions} /> : <SkillCostPanel actions={actions} />}
      </div>
    </div>
  );
}
