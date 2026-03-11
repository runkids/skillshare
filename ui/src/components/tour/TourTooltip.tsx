import { useEffect, useRef, useState } from 'react';
import { useFocusTrap } from '../../hooks/useFocusTrap';
import { radius } from '../../design';
import Button from '../Button';
import { useTour } from './TourProvider';

const MAX_WIDTH = 320;
const ARROW_SIZE = 8;
const OFFSET = 12;

export default function TourTooltip() {
  const { isActive, currentStep, steps, targetRect, isWaiting, nextStep, prevStep, skipTour } = useTour();
  const focusRef = useFocusTrap(isActive && !isWaiting && !!targetRect);
  const tooltipRef = useRef<HTMLDivElement>(null);
  const [pos, setPos] = useState<{ top: number; left: number }>({ top: 0, left: 0 });
  const [isMobile, setIsMobile] = useState(false);

  const step = steps[currentStep];

  useEffect(() => {
    const check = () => setIsMobile(window.innerWidth < 768);
    check();
    window.addEventListener('resize', check);
    return () => window.removeEventListener('resize', check);
  }, []);

  useEffect(() => {
    if (!targetRect || !tooltipRef.current || isMobile) return;
    const tt = tooltipRef.current.getBoundingClientRect();
    const placement = step?.placement ?? 'bottom';
    let top = 0, left = 0;
    switch (placement) {
      case 'bottom': top = targetRect.bottom + OFFSET; left = targetRect.left + targetRect.width / 2 - tt.width / 2; break;
      case 'top': top = targetRect.top - tt.height - OFFSET; left = targetRect.left + targetRect.width / 2 - tt.width / 2; break;
      case 'left': top = targetRect.top + targetRect.height / 2 - tt.height / 2; left = targetRect.left - tt.width - OFFSET; break;
      case 'right': top = targetRect.top + targetRect.height / 2 - tt.height / 2; left = targetRect.right + OFFSET; break;
    }
    left = Math.max(8, Math.min(left, window.innerWidth - tt.width - 8));
    top = Math.max(8, Math.min(top, window.innerHeight - tt.height - 8));
    setPos({ top, left });
  }, [targetRect, step?.placement, currentStep, isMobile]);

  if (!isActive || !step || isWaiting || !targetRect) return null;

  const isFirst = currentStep === 0;
  const isLast = currentStep === steps.length - 1;

  const arrowStyle = (() => {
    const base: React.CSSProperties = { position: 'absolute', width: 0, height: 0 };
    const color = 'var(--color-paper-warm)';
    switch (step.placement) {
      case 'bottom': return { ...base, top: -ARROW_SIZE, left: '50%', transform: 'translateX(-50%)', borderLeft: `${ARROW_SIZE}px solid transparent`, borderRight: `${ARROW_SIZE}px solid transparent`, borderBottom: `${ARROW_SIZE}px solid ${color}` };
      case 'top': return { ...base, bottom: -ARROW_SIZE, left: '50%', transform: 'translateX(-50%)', borderLeft: `${ARROW_SIZE}px solid transparent`, borderRight: `${ARROW_SIZE}px solid transparent`, borderTop: `${ARROW_SIZE}px solid ${color}` };
      case 'left': return { ...base, right: -ARROW_SIZE, top: '50%', transform: 'translateY(-50%)', borderTop: `${ARROW_SIZE}px solid transparent`, borderBottom: `${ARROW_SIZE}px solid transparent`, borderLeft: `${ARROW_SIZE}px solid ${color}` };
      case 'right': return { ...base, left: -ARROW_SIZE, top: '50%', transform: 'translateY(-50%)', borderTop: `${ARROW_SIZE}px solid transparent`, borderBottom: `${ARROW_SIZE}px solid transparent`, borderRight: `${ARROW_SIZE}px solid ${color}` };
    }
  })();

  const content = (
    <div ref={focusRef}>
      <p className="text-pencil font-semibold text-base mb-1">{step.title}</p>
      <p className="text-pencil-light text-sm leading-relaxed mb-3">{step.description}</p>
      <div className="flex items-center gap-1.5 mb-3">
        {steps.map((_, i) => (
          <div key={i} className={`w-1.5 h-1.5 rounded-full transition-colors ${i <= currentStep ? 'bg-accent' : 'bg-muted'}`} />
        ))}
      </div>
      <div className="flex items-center justify-between">
        <Button variant="ghost" size="sm" onClick={skipTour}>Skip</Button>
        <div className="flex items-center gap-2">
          {!isFirst && <Button variant="ghost" size="sm" onClick={prevStep}>Back</Button>}
          <Button variant="primary" size="sm" onClick={nextStep}>{isLast ? 'Done' : 'Next'}</Button>
        </div>
      </div>
    </div>
  );

  if (isMobile) {
    return (
      <div key={currentStep} ref={tooltipRef} className="fixed bottom-0 left-0 right-0 z-[70] bg-paper-warm border-t border-muted p-4 animate-fade-in" style={{ boxShadow: 'var(--shadow-lg)' }} role="dialog" aria-modal="true" aria-label={`Tour step ${currentStep + 1} of ${steps.length}: ${step.title}`}>
        {content}
      </div>
    );
  }

  return (
    <div key={currentStep} ref={tooltipRef} className="fixed z-[70] bg-paper-warm border border-muted p-4 animate-fade-in" style={{ top: pos.top, left: pos.left, maxWidth: MAX_WIDTH, borderRadius: radius.lg, boxShadow: 'var(--shadow-lg)' }} role="dialog" aria-modal="true" aria-label={`Tour step ${currentStep + 1} of ${steps.length}: ${step.title}`}>
      <div style={arrowStyle} />
      {content}
    </div>
  );
}
