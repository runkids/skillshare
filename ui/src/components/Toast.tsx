import { createContext, useContext, useState, useCallback, useRef, useEffect, type ReactNode } from 'react';
import { X, CheckCircle, AlertTriangle, XCircle, Info } from 'lucide-react';
import { shadows } from '../design';

interface Toast {
  id: number;
  message: string;
  type: 'success' | 'error' | 'warning' | 'info';
  title?: string;
  exiting?: boolean;
}

interface ToastOptions {
  title?: string;
}

interface ToastContextValue {
  toast: (message: string, type?: Toast['type'], opts?: ToastOptions) => void;
}

const ToastContext = createContext<ToastContextValue | null>(null);

let nextId = 0;

export function useToast() {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error('useToast must be used within ToastProvider');
  return ctx;
}

const icons = {
  success: CheckCircle,
  error: XCircle,
  warning: AlertTriangle,
  info: Info,
};

const typeStyles = {
  success: 'bg-success-light border-success text-success',
  error: 'bg-danger-light border-danger text-danger',
  warning: 'bg-warning-light border-warning text-warning',
  info: 'bg-info-light border-blue text-blue',
};

const progressColors = {
  success: 'bg-success',
  error: 'bg-danger',
  warning: 'bg-warning',
  info: 'bg-blue',
};

const TOAST_DURATION = 4000;
const EXIT_DURATION = 300;

function ToastItem({
  toast: t,
  onRemove,
}: {
  toast: Toast;
  onRemove: (id: number) => void;
}) {
  const Icon = icons[t.type];
  // Errors/warnings often carry longer text — give them more time to read.
  const duration = t.type === 'error' || t.type === 'warning' ? 8000 : TOAST_DURATION;
  const timerRef = useRef<ReturnType<typeof setTimeout>>(undefined);
  const [paused, setPaused] = useState(false);
  const [exiting, setExiting] = useState(false);
  const remainRef = useRef(duration);
  const startRef = useRef(Date.now());

  const startExit = useCallback(() => {
    setExiting(true);
    setTimeout(() => onRemove(t.id), EXIT_DURATION);
  }, [t.id, onRemove]);

  const startTimer = useCallback(() => {
    startRef.current = Date.now();
    timerRef.current = setTimeout(startExit, remainRef.current);
  }, [startExit]);

  const pauseTimer = useCallback(() => {
    if (timerRef.current) {
      clearTimeout(timerRef.current);
      remainRef.current -= Date.now() - startRef.current;
    }
  }, []);

  useEffect(() => {
    startTimer();
    return () => { if (timerRef.current) clearTimeout(timerRef.current); };
  }, [startTimer]);

  return (
    <div
      className={`
        ss-toast
        relative flex items-start gap-3 px-4 py-3 border-2 text-base overflow-hidden
        rounded-[var(--radius-sm)]
        ${exiting ? 'animate-toast-out' : 'animate-fade-in'}
        ${typeStyles[t.type]}
      `}
      style={{
        boxShadow: shadows.md,
      }}
      onMouseEnter={() => { setPaused(true); pauseTimer(); }}
      onMouseLeave={() => { setPaused(false); startTimer(); }}
    >
      <Icon size={18} strokeWidth={2.5} className="shrink-0 mt-0.5" />
      <div className="flex-1 min-w-0">
        {t.title && <p className="font-bold break-words leading-snug">{t.title}</p>}
        <span className="block whitespace-pre-line break-words leading-relaxed">{t.message}</span>
      </div>
      <button
        onClick={() => startExit()}
        className="shrink-0 opacity-60 hover:opacity-100 transition-opacity"
      >
        <X size={16} strokeWidth={2.5} />
      </button>
      {/* Progress bar */}
      <div className="absolute bottom-0 left-0 right-0 h-0.5 bg-black/5">
        <div
          className={`h-full ${progressColors[t.type]}`}
          style={{
            animation: paused ? 'none' : `toastProgress ${duration}ms linear forwards`,
          }}
        />
      </div>
    </div>
  );
}

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);

  const addToast = useCallback((message: string, type: Toast['type'] = 'info', opts?: ToastOptions) => {
    const id = nextId++;
    setToasts((prev) => [...prev, { id, message, type, title: opts?.title }]);
  }, []);

  const removeToast = useCallback((id: number) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  return (
    <ToastContext.Provider value={{ toast: addToast }}>
      {children}
      {/* Toast container */}
      <div data-toast-container className="fixed bottom-6 right-6 z-[60] flex flex-col gap-3 w-[min(32rem,calc(100vw-3rem))]">
        {toasts.map((t) => (
          <ToastItem key={t.id} toast={t} onRemove={removeToast} />
        ))}
      </div>
    </ToastContext.Provider>
  );
}
