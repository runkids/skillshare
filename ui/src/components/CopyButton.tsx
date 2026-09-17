import { useEffect, useRef, useState } from 'react';
import { Check, Copy } from 'lucide-react';
import { useToast } from './Toast';

interface CopyButtonProps {
  value: string;
  title?: string;
  className?: string;
  copiedLabel?: string;
  /** Shown next to the icon before copying; without it the button is icon-only. */
  label?: string;
  /** Drop the icon-button styling so `className` alone decides how it looks. */
  unstyled?: boolean;
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
  label,
  unstyled = false,
  copiedLabelClassName = unstyled ? '' : 'text-xs',
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
      className={unstyled ? className : className ? `${baseClassName} ${className}` : baseClassName}
      // A visible label says what the button does, so a hover tooltip would only repeat it
      title={label ? undefined : title}
      aria-label={title}
    >
      {copied ? (
        <>
          <Check size={size} strokeWidth={strokeWidth} />
          <span className={copiedLabelClassName}>{copiedLabel}</span>
        </>
      ) : (
        <>
          <Copy size={size} strokeWidth={strokeWidth} />
          {label && <span className={copiedLabelClassName}>{label}</span>}
        </>
      )}
    </button>
  );
}
