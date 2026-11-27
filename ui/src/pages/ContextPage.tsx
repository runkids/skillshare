import { useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import SkillCostPanel from '../components/context/SkillCostPanel';
import MCPCostPanel from '../components/context/MCPCostPanel';
import RouterPanel from '../components/context/RouterPanel';
import PageHeader from '../components/PageHeader';
import { useT } from '../i18n';
import { segSlide } from '../lib/segSlide';

type Tab = 'skills' | 'mcp' | 'router';
const TABS: Tab[] = ['skills', 'mcp', 'router'];
const INTRO: Record<Tab, string> = {
  skills: 'analyze.intro',
  mcp: 'context.mcp.intro',
  router: 'context.router.intro',
};

/** What skills, MCP servers and a folder's context router cost a model's context. Each tab has its own list: skill targets, MCP targets and router folders differ. */
export default function ContextPage() {
  const t = useT();
  const [params, setParams] = useSearchParams();
  const wanted = params.get('tab');
  const tab: Tab = wanted === 'mcp' || wanted === 'router' ? wanted : 'skills';
  const setTab = (next: Tab) =>
    setParams(
      (p) => {
        if (next === 'skills') p.delete('tab');
        else p.set('tab', next);
        // The Router tab's folder means nothing to the other tabs.
        if (next !== 'router') {
          p.delete('root');
          p.delete('gone');
        }
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
        subtitle={t(INTRO[tab])}
        actionsOnTitleRow
        actions={
          <>
            {/* Tabs on the title row, each tab's picker under them: the tabs stay put whatever the picker's width. */}
            <div ref={segSlide} className="ss-seg" role="tablist">
              {TABS.map((key) => (
                <button
                  key={key}
                  type="button"
                  role="tab"
                  aria-selected={tab === key}
                  className={tab === key ? 'on' : ''}
                  onClick={() => setTab(key)}
                >
                  {t(`context.tab.${key}`)}
                </button>
              ))}
            </div>
            <div ref={setActions} className="flex flex-wrap items-center justify-end gap-2" />
          </>
        }
      />
      {/* Keyed, so switching tabs fades the new content in. */}
      <div key={tab} className="animate-page-in">
        {tab === 'mcp' ? (
          <MCPCostPanel actions={actions} />
        ) : tab === 'router' ? (
          <RouterPanel actions={actions} />
        ) : (
          <SkillCostPanel actions={actions} />
        )}
      </div>
    </div>
  );
}
