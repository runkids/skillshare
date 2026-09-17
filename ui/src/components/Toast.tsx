import { createContext, useContext, useState, useCallback, useRef, useEffect, type ReactNode } from 'react';
import { X, CheckCircle, AlertTriangle, XCircle, Info } from 'lucide-react';

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

// The toast is always dark, so icon colours are fixed light tones rather than theme tokens.
const iconColors = {
  success: 'text-[#8FD6A4]',
  error: 'text-[#FF9C8F]',
  warning: 'text-[#FFD37A]',
  info: 'text-[#A9C8FF]',
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
      className={`ss-toast relative !flex !h-auto min-h-[42px] !items-start py-2.5 overflow-hidden ${exiting ? 'animate-toast-out' : 'animate-fade-in'}`}
      onMouseEnter={() => { setPaused(true); pauseTimer(); }}
      onMouseLeave={() => { setPaused(false); startTimer(); }}
    >
      <Icon size={16} className={`shrink-0 mt-px ${iconColors[t.type]}`} />
      <div className="flex-1 min-w-0">
        {t.title && <p className="font-bold break-words leading-snug">{t.title}</p>}
        <span className="block whitespace-pre-line break-words leading-normal">{t.message}</span>
      </div>
      <button
        onClick={() => startExit()}
        type="button"
        className="shrink-0 grid place-items-center w-6 h-6 -my-0.5 rounded-md opacity-60 hover:opacity-100 hover:bg-white/10 cursor-pointer"
      >
        <X size={14} />
      </button>
      {/* Progress bar */}
      <div className="absolute bottom-0 left-0 right-0 h-0.5">
        <div
          className="h-full bg-white/25"
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
      <div data-toast-container className="fixed bottom-7 left-1/2 -translate-x-1/2 z-[70] flex flex-col items-center gap-2 w-[min(36rem,calc(100vw-3rem))]">
        {toasts.map((t) => (
          <ToastItem key={t.id} toast={t} onRemove={removeToast} />
        ))}
      </div>
    </ToastContext.Provider>
  );
}
