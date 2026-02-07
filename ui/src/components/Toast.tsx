import { createContext, useContext, useState, useCallback, type ReactNode } from 'react';
import { X, CheckCircle, AlertTriangle, XCircle, Info } from 'lucide-react';
import { wobbly } from '../design';

interface Toast {
  id: number;
  message: string;
  type: 'success' | 'error' | 'warning' | 'info';
}

interface ToastContextValue {
  toast: (message: string, type?: Toast['type']) => void;
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

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);

  const addToast = useCallback((message: string, type: Toast['type'] = 'info') => {
    const id = nextId++;
    setToasts((prev) => [...prev, { id, message, type }]);
    setTimeout(() => {
      setToasts((prev) => prev.filter((t) => t.id !== id));
    }, 4000);
  }, []);

  const removeToast = useCallback((id: number) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  return (
    <ToastContext.Provider value={{ toast: addToast }}>
      {children}
      {/* Toast container */}
      <div className="fixed bottom-6 right-6 z-50 flex flex-col gap-3 max-w-sm">
        {toasts.map((t) => {
          const Icon = icons[t.type];
          return (
            <div
              key={t.id}
              className={`flex items-start gap-3 px-4 py-3 border-2 font-[var(--font-hand)] text-base animate-sketch-in ${typeStyles[t.type]}`}
              style={{
                borderRadius: wobbly.sm,
                boxShadow: '3px 3px 0px 0px rgba(45, 45, 45, 0.15)',
                fontFamily: 'var(--font-hand)',
              }}
            >
              <Icon size={18} strokeWidth={2.5} className="shrink-0 mt-0.5" />
              <span className="flex-1">{t.message}</span>
              <button
                onClick={() => removeToast(t.id)}
                className="shrink-0 opacity-60 hover:opacity-100 transition-opacity"
              >
                <X size={16} strokeWidth={2.5} />
              </button>
            </div>
          );
        })}
      </div>
    </ToastContext.Provider>
  );
}
