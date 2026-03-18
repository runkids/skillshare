import { createContext, useContext, useState, useCallback, useRef, useEffect, type ReactNode } from 'react';
import { X, CheckCircle, AlertTriangle, XCircle, Info } from 'lucide-react';
import { shadows } from '../design';

interface ToastAction {
  label: string;
  onClick: () => void;
}

interface Toast {
  id: number;
  message: string;
  type: 'success' | 'error' | 'warning' | 'info';
  exiting?: boolean;
  action?: ToastAction;
  persistent?: boolean;
  replaceKey?: string;
}

interface ToastContextValue {
  toast: (
    message: string,
    type?: Toast['type'],
    options?: { action?: ToastAction; persistent?: boolean; replaceKey?: string },
  ) => void;
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
  const { persistent, action } = t;
  const Icon = icons[t.type];
  const timerRef = useRef<ReturnType<typeof setTimeout>>(undefined);
  const [paused, setPaused] = useState(false);
  const [exiting, setExiting] = useState(false);
  const remainRef = useRef(TOAST_DURATION);
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
    if (persistent) return;
    startTimer();
    return () => { if (timerRef.current) clearTimeout(timerRef.current); };
  }, [persistent, startTimer]);

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
      onMouseLeave={() => { setPaused(false); if (!persistent) startTimer(); }}
    >
      <Icon size={18} strokeWidth={2.5} className="shrink-0 mt-0.5" />
      <span className="flex-1">{t.message}</span>
      {action && (
        <button
          onClick={() => {
            action.onClick();
            startExit();
          }}
          className="text-sm font-semibold underline underline-offset-2 hover:opacity-80 transition-opacity shrink-0"
        >
          {action.label}
        </button>
      )}
      <button
        onClick={() => startExit()}
        className="shrink-0 opacity-60 hover:opacity-100 transition-opacity"
      >
        <X size={16} strokeWidth={2.5} />
      </button>
      {/* Progress bar */}
      {!persistent && (
        <div className="absolute bottom-0 left-0 right-0 h-0.5 bg-black/5">
          <div
            className={`h-full ${progressColors[t.type]}`}
            style={{
              animation: paused ? 'none' : `toastProgress ${TOAST_DURATION}ms linear forwards`,
            }}
          />
        </div>
      )}
    </div>
  );
}

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);

  const addToast = useCallback(
    (
      message: string,
      type: Toast['type'] = 'info',
      options?: { action?: ToastAction; persistent?: boolean; replaceKey?: string },
    ) => {
      const id = nextId++;
      const { action, persistent, replaceKey } = options ?? {};
      setToasts((prev) => {
        const filtered = replaceKey ? prev.filter((t) => t.replaceKey !== replaceKey) : prev;
        return [...filtered, { id, message, type, action, persistent, replaceKey }];
      });
    },
    [],
  );

  const removeToast = useCallback((id: number) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  return (
    <ToastContext.Provider value={{ toast: addToast }}>
      {children}
      {/* Toast container */}
      <div data-toast-container className="fixed bottom-6 right-6 z-50 flex flex-col gap-3 max-w-sm">
        {toasts.map((t) => (
          <ToastItem key={t.id} toast={t} onRemove={removeToast} />
        ))}
      </div>
    </ToastContext.Provider>
  );
}
