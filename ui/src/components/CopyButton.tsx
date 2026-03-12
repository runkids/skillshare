import { useEffect, useRef, useState } from 'react';
import { Check, Copy } from 'lucide-react';
import { useToast } from './Toast';

interface CopyButtonProps {
  value: string;
  title?: string;
  className?: string;
  copiedLabel?: string;
  copiedLabelClassName?: string;
  errorMessage?: string;
  size?: number;
  strokeWidth?: number;
}

const baseClassName = 'inline-flex items-center gap-0.5 text-pencil-light hover:text-pencil transition-all duration-150 cursor-pointer shrink-0 active:scale-95 focus-visible:ring-2 focus-visible:ring-pencil/20 rounded-sm';

export default function CopyButton({
  value,
  title = 'Copy to clipboard',
  className,
  copiedLabel = 'Copied!',
  copiedLabelClassName = 'text-xs',
  errorMessage = 'Failed to copy to clipboard.',
  size = 12,
  strokeWidth = 2.5,
}: CopyButtonProps) {
  const { toast } = useToast();
  const [copied, setCopied] = useState(false);
  const resetTimeoutRef = useRef<number | null>(null);

  useEffect(() => {
    return () => {
      if (resetTimeoutRef.current !== null) {
        window.clearTimeout(resetTimeoutRef.current);
      }
    };
  }, []);

  async function handleCopy() {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      if (resetTimeoutRef.current !== null) {
        window.clearTimeout(resetTimeoutRef.current);
      }
      resetTimeoutRef.current = window.setTimeout(() => {
        setCopied(false);
        resetTimeoutRef.current = null;
      }, 1500);
    } catch {
      setCopied(false);
      if (resetTimeoutRef.current !== null) {
        window.clearTimeout(resetTimeoutRef.current);
        resetTimeoutRef.current = null;
      }
      toast(errorMessage, 'error');
    }
  }

  return (
    <button
      type="button"
      onClick={handleCopy}
      className={className ? `${baseClassName} ${className}` : baseClassName}
      title={title}
      aria-label={title}
    >
      {copied ? (
        <>
          <Check size={size} strokeWidth={strokeWidth} />
          <span className={copiedLabelClassName}>{copiedLabel}</span>
        </>
      ) : (
        <Copy size={size} strokeWidth={strokeWidth} />
      )}
    </button>
  );
}
