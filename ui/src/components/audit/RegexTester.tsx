import { useState } from 'react';
import { FlaskConical } from 'lucide-react';
import { useRegexTester } from '../../hooks/useRegexTester';
import Badge from '../Badge';

interface RegexTesterProps {
  pattern: string;
  excludePattern?: string;
  onPatternChange: (pattern: string) => void;
}

export default function RegexTester({ pattern, excludePattern, onPatternChange }: RegexTesterProps) {
  const [testInput, setTestInput] = useState('');
  const { matches, error, isGoSpecific } = useRegexTester(pattern, testInput, excludePattern);

  const matchedLines = matches.filter(m => m.matched && !m.excluded);
  const excludedLines = matches.filter(m => m.matched && m.excluded);

  return (
    <div className="flex flex-col gap-2 px-3 py-3 text-sm">
      {/* Header */}
      <div className="flex items-center gap-1.5">
        <FlaskConical size={13} strokeWidth={2} className="text-info flex-shrink-0" />
        <span className="text-xs font-medium text-pencil-light uppercase tracking-wider">Regex Test</span>
      </div>

      {/* Dashed separator */}
      <div className="border-t border-dashed border-pencil-light/30" />

      {/* Pattern textarea */}
      <div className="flex flex-col gap-1">
        <label className="text-[10px] font-medium text-muted-dark uppercase tracking-wider">Pattern</label>
        <textarea
          rows={2}
          className="w-full font-mono text-xs bg-paper border border-dashed border-pencil-light/30 rounded-lg px-2.5 py-2 text-pencil resize-none focus:outline-none focus:border-blue/50 transition-colors"
          value={pattern}
          onChange={e => onPatternChange(e.target.value)}
          placeholder="Enter regex pattern…"
          spellCheck={false}
        />
      </div>

      {/* Test input textarea */}
      <div className="flex flex-col gap-1">
        <label className="text-[10px] font-medium text-muted-dark uppercase tracking-wider">Test input</label>
        <textarea
          rows={4}
          className="w-full font-mono text-xs bg-paper border border-dashed border-pencil-light/30 rounded-lg px-2.5 py-2 text-pencil resize-none focus:outline-none focus:border-blue/50 transition-colors"
          value={testInput}
          onChange={e => setTestInput(e.target.value)}
          placeholder="Paste lines to test…"
          spellCheck={false}
        />
      </div>

      {/* Results */}
      {pattern && (
        <>
          <div className="border-t border-dashed border-pencil-light/30" />
          <div className="flex flex-col gap-1">
            {isGoSpecific ? (
              <Badge variant="info">Go-specific regex — cannot test in browser</Badge>
            ) : error ? (
              <Badge variant="danger">{error}</Badge>
            ) : testInput ? (
              <div className="flex flex-col gap-0.5">
                {matches.map((lm, i) => {
                  let indicator: React.ReactNode;
                  if (lm.matched && !lm.excluded) {
                    indicator = <Badge variant="success" size="sm">✓</Badge>;
                  } else if (lm.matched && lm.excluded) {
                    indicator = <Badge variant="warning" size="sm">~</Badge>;
                  } else {
                    indicator = <Badge variant="danger" size="sm">✗</Badge>;
                  }

                  const content = lm.matched && !lm.excluded && lm.matchStart != null && lm.matchEnd != null ? (
                    <span className="font-mono text-xs text-pencil break-all">
                      {lm.content.slice(0, lm.matchStart)}
                      <mark className="bg-success/20 text-pencil rounded-sm">{lm.content.slice(lm.matchStart, lm.matchEnd)}</mark>
                      {lm.content.slice(lm.matchEnd)}
                    </span>
                  ) : (
                    <span className="font-mono text-xs text-pencil-light break-all">{lm.content}</span>
                  );

                  return (
                    <div key={i} className="flex items-start gap-1.5 py-0.5">
                      <span className="flex-shrink-0 mt-0.5">{indicator}</span>
                      {content}
                    </div>
                  );
                })}
              </div>
            ) : null}
          </div>

          {/* Exclude section */}
          {excludePattern && excludedLines.length > 0 && (
            <>
              <div className="border-t border-dashed border-pencil-light/30" />
              <div className="flex flex-col gap-1">
                <span className="text-[10px] font-medium text-muted-dark uppercase tracking-wider">Exclude</span>
                <div className="flex items-start gap-2">
                  <code className="font-mono text-xs bg-paper border border-dashed border-pencil-light/30 rounded px-1.5 py-0.5 text-pencil-light break-all flex-1">
                    {excludePattern}
                  </code>
                </div>
                <p className="text-xs text-pencil-light">
                  Suppresses {excludedLines.length} line{excludedLines.length !== 1 ? 's' : ''} matched by exclude pattern
                </p>
              </div>
            </>
          )}

          {excludePattern && matchedLines.length === 0 && excludedLines.length === 0 && !isGoSpecific && !error && testInput && (
            <>
              <div className="border-t border-dashed border-pencil-light/30" />
              <div className="flex flex-col gap-1">
                <span className="text-[10px] font-medium text-muted-dark uppercase tracking-wider">Exclude</span>
                <code className="font-mono text-xs bg-paper border border-dashed border-pencil-light/30 rounded px-1.5 py-0.5 text-pencil-light break-all">
                  {excludePattern}
                </code>
                <p className="text-xs text-pencil-light">Suppresses 0 lines</p>
              </div>
            </>
          )}
        </>
      )}
    </div>
  );
}
