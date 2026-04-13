import { useEffect } from 'react';
import { X } from 'lucide-react';
import Markdown, { type Components } from 'react-markdown';
import remarkGfm from 'remark-gfm';
import Card from './Card';
import CopyButton from './CopyButton';
import HandButton from './HandButton';
import { useFocusTrap } from '../hooks/useFocusTrap';
import { wobbly } from '../design';
import type { RuleItem } from '../api/client';

interface RuleViewerModalProps {
  rule: RuleItem;
  onClose: () => void;
}

export default function RuleViewerModal({ rule, onClose }: RuleViewerModalProps) {
  const trapRef = useFocusTrap(true);
  const markdownComponents: Components = {
    a: ({ children }) => (
      <span className="underline decoration-muted-dark underline-offset-2">
        {children}
      </span>
    ),
    img: ({ alt }) => (
      <span className="italic text-pencil-light">
        {alt ? `[image: ${alt}]` : '[image]'}
      </span>
    ),
  };

  useEffect(() => {
    const handleKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose();
    };

    document.addEventListener('keydown', handleKey);
    return () => document.removeEventListener('keydown', handleKey);
  }, [onClose]);

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4"
      role="dialog"
      aria-modal="true"
      aria-label={rule.name}
      onClick={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
    >
      <div className="absolute inset-0 bg-pencil/30" onClick={onClose} />

      <div
        ref={trapRef}
        className="relative w-full max-w-3xl max-h-[85vh] animate-sketch-in"
        style={{ borderRadius: wobbly.md }}
      >
        <Card decoration="tape" className="flex flex-col h-full overflow-hidden">
          <div className="flex items-start justify-between gap-3 mb-4 pt-2">
            <div className="min-w-0">
              <h3
                className="text-xl font-bold text-pencil"
                style={{ fontFamily: 'var(--font-heading)' }}
              >
                {rule.name}
              </h3>
              <div className="flex items-center gap-1.5 mt-1 min-w-0">
                <p
                  className="text-sm text-pencil-light truncate"
                  style={{ fontFamily: "'Courier New', monospace" }}
                >
                  {rule.path}
                </p>
                <CopyButton
                  value={rule.path}
                  title="Copy rule path"
                  copiedLabelClassName="text-xs font-normal"
                />
              </div>
            </div>

            <HandButton
              variant="ghost"
              size="sm"
              onClick={onClose}
              className="shrink-0"
              aria-label="Close rule viewer"
            >
              <X size={16} strokeWidth={2.5} />
            </HandButton>
          </div>

          <div className="overflow-auto flex-1 min-h-0 -mx-4 -mb-4 px-4 pb-4">
            <div className="prose-hand max-w-none">
              <Markdown remarkPlugins={[remarkGfm]} components={markdownComponents}>
                {rule.content}
              </Markdown>
            </div>
          </div>
        </Card>
      </div>
    </div>
  );
}
