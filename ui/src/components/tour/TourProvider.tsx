import { createContext, useContext, useState, useCallback, useEffect, useRef, type ReactNode } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { useAppContext } from '../../context/AppContext';
import type { Overview } from '../../api/client';
import { queryKeys } from '../../lib/queryKeys';
import { buildSteps, TOUR_STORAGE_KEY, type TourStep } from './tourSteps';

interface TourContextValue {
  isActive: boolean;
  currentStep: number;
  steps: TourStep[];
  targetRect: DOMRect | null;
  isWaiting: boolean;
  startTour: () => void;
  nextStep: () => void;
  prevStep: () => void;
  skipTour: () => void;
}

const TourContext = createContext<TourContextValue | null>(null);

export function useTour() {
  const ctx = useContext(TourContext);
  if (!ctx) throw new Error('useTour must be used within TourProvider');
  return ctx;
}

const POLL_INTERVAL = 50;
const POLL_TIMEOUT = 3000;

export function TourProvider({ children }: { children: ReactNode }) {
  const navigate = useNavigate();
  const location = useLocation();
  const queryClient = useQueryClient();
  const { isProjectMode } = useAppContext();

  const [isActive, setIsActive] = useState(false);
  const [currentStep, setCurrentStep] = useState(0);
  const [steps, setSteps] = useState<TourStep[]>([]);
  const [targetRect, setTargetRect] = useState<DOMRect | null>(null);
  const [isWaiting, setIsWaiting] = useState(false);

  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const skippingRef = useRef(false);

  const clearPolling = useCallback(() => {
    if (pollRef.current) { clearInterval(pollRef.current); pollRef.current = null; }
    if (timeoutRef.current) { clearTimeout(timeoutRef.current); timeoutRef.current = null; }
  }, []);

  const findTarget = useCallback((step: TourStep) => {
    setIsWaiting(true);
    clearPolling();

    const tryFind = () => {
      const el = document.querySelector(step.targetSelector);
      if (el) {
        clearPolling();
        el.scrollIntoView({ behavior: 'smooth', block: 'center' });
        setTimeout(() => {
          setTargetRect(el.getBoundingClientRect());
          setIsWaiting(false);
        }, 300);
        return true;
      }
      return false;
    };

    if (tryFind()) return;

    pollRef.current = setInterval(() => { tryFind(); }, POLL_INTERVAL);

    timeoutRef.current = setTimeout(() => {
      clearPolling();
      setIsWaiting(false);
      if (skippingRef.current) return;
      skippingRef.current = true;
      console.warn(`[Tour] Target not found: ${step.targetSelector}, skipping`);
      setCurrentStep((prev) => {
        const next = prev + 1;
        return next < steps.length ? next : prev;
      });
      setTimeout(() => { skippingRef.current = false; }, 100);
    }, POLL_TIMEOUT);
  }, [clearPolling, steps.length]);

  // Single useEffect for navigation + target finding
  useEffect(() => {
    if (!isActive || steps.length === 0) return;
    const step = steps[currentStep];
    if (!step) return;
    if (location.pathname !== step.page) {
      navigate(step.page);
    } else {
      findTarget(step);
    }
  }, [isActive, currentStep, location.pathname, steps, navigate, findTarget]);

  // Update targetRect on resize/scroll
  useEffect(() => {
    if (!isActive) return;
    const updateRect = () => {
      const step = steps[currentStep];
      if (!step) return;
      const el = document.querySelector(step.targetSelector);
      if (el) setTargetRect(el.getBoundingClientRect());
    };
    window.addEventListener('resize', updateRect);
    window.addEventListener('scroll', updateRect, true);
    return () => {
      window.removeEventListener('resize', updateRect);
      window.removeEventListener('scroll', updateRect, true);
    };
  }, [isActive, currentStep, steps]);

  // Add/remove body class for toast z-index promotion
  useEffect(() => {
    if (isActive) { document.body.classList.add('tour-active'); }
    else { document.body.classList.remove('tour-active'); }
    return () => document.body.classList.remove('tour-active');
  }, [isActive]);

  const startTour = useCallback(() => {
    // Use cached overview from react-query; fall back to API call only if not cached
    const cached = queryClient.getQueryData<Overview>(queryKeys.overview);
    const skillCount = cached?.skillCount ?? 0;

    const builtSteps = buildSteps({ isProjectMode, skillCount });
    setSteps(builtSteps);
    setCurrentStep(0);
    setIsActive(true);
    const first = builtSteps[0];
    if (first && location.pathname !== first.page) navigate(first.page);
  }, [isProjectMode, location.pathname, navigate, queryClient]);

  const nextStepFn = useCallback(() => {
    const next = currentStep + 1;
    if (next >= steps.length) {
      setIsActive(false);
      setTargetRect(null);
      clearPolling();
      localStorage.setItem(TOUR_STORAGE_KEY, 'true');
      return;
    }
    setCurrentStep(next);
  }, [currentStep, steps.length, clearPolling]);

  const prevStepFn = useCallback(() => {
    if (currentStep <= 0) return;
    setCurrentStep(currentStep - 1);
  }, [currentStep]);

  const skipTourFn = useCallback(() => {
    setIsActive(false);
    setTargetRect(null);
    clearPolling();
    localStorage.setItem(TOUR_STORAGE_KEY, 'true');
  }, [clearPolling]);

  // Stable refs for keyboard handler (declared AFTER functions)
  const nextRef = useRef(nextStepFn);
  const prevRef = useRef(prevStepFn);
  const skipRef = useRef(skipTourFn);
  nextRef.current = nextStepFn;
  prevRef.current = prevStepFn;
  skipRef.current = skipTourFn;

  useEffect(() => {
    if (!isActive) return;
    const handler = (e: KeyboardEvent) => {
      switch (e.key) {
        case 'Escape': e.preventDefault(); skipRef.current(); break;
        case 'ArrowRight': case 'Enter': e.preventDefault(); nextRef.current(); break;
        case 'ArrowLeft': e.preventDefault(); prevRef.current(); break;
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [isActive]);

  useEffect(() => { return () => clearPolling(); }, [clearPolling]);

  return (
    <TourContext.Provider value={{ isActive, currentStep, steps, targetRect, isWaiting, startTour, nextStep: nextStepFn, prevStep: prevStepFn, skipTour: skipTourFn }}>
      {children}
    </TourContext.Provider>
  );
}
