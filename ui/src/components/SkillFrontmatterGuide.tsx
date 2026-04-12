import type { ReactNode } from 'react';
import { Info } from 'lucide-react';
import type { SkillFrontmatter } from '../lib/skillMarkdown';
import {
  buildFrontmatterTemplate,
  formatFrontmatterValue,
  type FrontmatterSchema,
  getAdditionalFrontmatterEntries,
  getReferenceFrontmatterEntries,
} from '../lib/skillFrontmatter';

type SkillFrontmatterGuideProps = {
  frontmatter?: SkillFrontmatter;
  headingLevel?: 'h2' | 'h3' | 'h4';
  className?: string;
  showCurrentValues?: boolean;
  excludeKeys?: string[];
  schema?: FrontmatterSchema;
  isReferenceEntryActive?: (entry: ReturnType<typeof getReferenceFrontmatterEntries>[number]) => boolean;
  renderReferenceAccessory?: (entry: ReturnType<typeof getReferenceFrontmatterEntries>[number]) => ReactNode;
};

export default function SkillFrontmatterGuide({
  frontmatter = {},
  headingLevel = 'h3',
  className = '',
  showCurrentValues = false,
  excludeKeys,
  schema = 'skill',
  isReferenceEntryActive,
  renderReferenceAccessory,
}: SkillFrontmatterGuideProps) {
  const HeadingTag = headingLevel;
  const referenceEntries = getReferenceFrontmatterEntries(frontmatter, { excludeKeys, schema });
  const extraEntries = getAdditionalFrontmatterEntries(frontmatter, schema);
  const isAgentSchema = schema === 'agent';

  return (
    <section className={`space-y-4 ${className}`}>
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="space-y-1">
          <HeadingTag className="text-lg font-bold text-pencil">Reference</HeadingTag>
          <p className="text-sm text-pencil-light max-w-2xl">
            {isAgentSchema ? (
              <>
                Name and description are required. The markdown body below the frontmatter becomes the subagent system prompt. Use
                {' '}
                <code>tools</code>
                {' '}
                and
                {' '}
                <code>disallowedTools</code>
                {' '}
                to control capabilities.
              </>
            ) : (
              <>
                All fields are optional. Description is recommended. Use
                {' '}
                <code>context: fork</code>
                {' '}
                with
                {' '}
                <code>agent</code>
                {' '}
                when you want the skill to run in an isolated subagent context.
              </>
            )}
          </p>
        </div>
      </div>

      <div className="rounded-[var(--radius-md)] border border-dashed border-muted-dark/40 bg-paper/40 p-4">
        <div className="mb-2 inline-flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.18em] text-muted-dark">
          <Info size={12} strokeWidth={2.5} />
          Template
        </div>
        <pre className="overflow-x-auto rounded-[var(--radius-sm)] border border-muted/80 bg-surface px-3 py-3 text-xs text-pencil-light">
          <code>{buildFrontmatterTemplate({ excludeKeys, schema })}</code>
        </pre>
      </div>

      <div className="grid grid-cols-1 gap-3 xl:grid-cols-2">
        {referenceEntries.map((entry) => {
          const isActive = isReferenceEntryActive ? isReferenceEntryActive(entry) : entry.isSet;

          return (
          <article
            key={entry.key}
            className={`rounded-[var(--radius-md)] border p-3 ${
              isActive
                ? 'border-muted-dark/40 bg-surface'
                : 'border-muted/80 bg-paper/40'
            }`}
          >
            <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
              <code className="text-sm font-semibold text-pencil">{entry.key}</code>
              {renderReferenceAccessory ? renderReferenceAccessory(entry) : (
                <span className={`rounded-full px-2 py-0.5 text-[11px] font-medium ${
                  entry.required === 'Recommended'
                    ? 'border border-blue/30 bg-blue/10 text-blue'
                    : 'border border-muted-dark/30 bg-muted/40 text-pencil-light'
                }`}
                >
                  {entry.required}
                </span>
              )}
            </div>
            <p className="mb-3 text-sm leading-relaxed text-pencil-light">{entry.description}</p>
            {showCurrentValues ? (
              <>
                <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-muted-dark">Current value</div>
                <pre className="mt-1 overflow-x-auto whitespace-pre-wrap break-words rounded-[var(--radius-sm)] bg-paper/80 px-2.5 py-2 text-xs text-pencil">
                  <code>{formatFrontmatterValue(entry.value)}</code>
                </pre>
              </>
            ) : null}
          </article>
          );
        })}
      </div>

      {showCurrentValues && extraEntries.length > 0 ? (
        <div className="space-y-2 rounded-[var(--radius-md)] border border-muted-dark/40 bg-surface p-4">
          <div className="text-sm font-semibold text-pencil">Additional fields in this {isAgentSchema ? 'agent' : 'skill'}</div>
          <div className="space-y-2">
            {extraEntries.map((entry) => (
              <div key={entry.key} className="rounded-[var(--radius-sm)] border border-muted/80 bg-paper/60 px-3 py-2">
                <code className="text-sm font-semibold text-pencil">{entry.key}</code>
                <pre className="mt-1 overflow-x-auto whitespace-pre-wrap break-words text-xs text-pencil-light">
                  <code>{formatFrontmatterValue(entry.value)}</code>
                </pre>
              </div>
            ))}
          </div>
        </div>
      ) : null}
    </section>
  );
}
