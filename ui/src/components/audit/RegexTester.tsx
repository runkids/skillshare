import { useState } from 'react';
import { useT } from '../../i18n';
import { useRegexTester } from '../../hooks/useRegexTester';

interface RegexTesterProps {
  pattern: string;
  excludePattern?: string;
  onPatternChange: (pattern: string) => void;
}

/** Try a rule's regex against pasted lines before saving it. */
export default function RegexTester({ pattern, excludePattern, onPatternChange }: RegexTesterProps) {
  const t = useT();
  const [testInput, setTestInput] = useState('');
  const { matches, error, isGoSpecific } = useRegexTester(pattern, testInput, excludePattern);
  const excludedLines = matches.filter((m) => m.matched && m.excluded);

  return (
    <div className="flex flex-col gap-3.5">
      <label className="ss-fld">
        <span>Pattern</span>
        <textarea
          rows={2}
          className="ss-inp area font-mono !text-[12.5px]"
          value={pattern}
          onChange={(e) => onPatternChange(e.target.value)}
          placeholder={t('auditRules.regex.patternPlaceholder')}
          spellCheck={false}
        />
      </label>

      <label className="ss-fld">
        <span>{t('auditRules.regex.testInput')}</span>
        <textarea
          rows={4}
          className="ss-inp area font-mono !text-[12.5px]"
          value={testInput}
          onChange={(e) => setTestInput(e.target.value)}
          placeholder={t('auditRules.regex.testInputPlaceholder')}
          spellCheck={false}
        />
      </label>

      {pattern && isGoSpecific && <span className="ss-st warn wrap">{t('auditRules.regex.goSpecific')}</span>}
      {pattern && !isGoSpecific && error && <span className="ss-st bad wrap">{error}</span>}

      {pattern && !isGoSpecific && !error && testInput && (
        <div className="ss-list !shadow-none">
          {matches.map((lm, i) => (
            <div key={i} className="ss-r !min-h-[30px] !px-2.5">
              <span className={`ss-st ${lm.matched && !lm.excluded ? 'ok' : lm.matched ? 'warn' : 'off'}`}>
                {lm.matched && !lm.excluded ? 'Match' : lm.matched ? 'Excluded' : 'No match'}
              </span>
              {lm.matched && !lm.excluded && lm.matchStart != null && lm.matchEnd != null ? (
                <span className="min-w-0 flex-1 truncate font-mono text-[12.5px]">
                  {lm.content.slice(0, lm.matchStart)}
                  <mark className="rounded-sm bg-ok-bg text-ok">{lm.content.slice(lm.matchStart, lm.matchEnd)}</mark>
                  {lm.content.slice(lm.matchEnd)}
                </span>
              ) : (
                <span className="min-w-0 flex-1 truncate font-mono text-[12.5px] text-ink-3">{lm.content}</span>
              )}
            </div>
          ))}
        </div>
      )}

      {pattern && excludePattern && (
        <div className="ss-kv">
          <span>Exclude</span>
          <code className="font-mono text-[12.5px] break-all">{excludePattern}</code>
          <span>{t(excludedLines.length === 1 ? 'auditRules.regex.suppressed.one' : 'auditRules.regex.suppressed.other', { count: excludedLines.length })}</span>
        </div>
      )}
    </div>
  );
}
