import { useTour } from './TourProvider';

export default function TourOverlay() {
  const { isActive, currentStep, targetRect, isWaiting, skipTour } = useTour();

  if (!isActive) return null;

  const handleClick = (e: React.MouseEvent) => {
    if (e.target === e.currentTarget) skipTour();
  };

  const PADDING = 8;
  const showSpinner = isWaiting && currentStep === 0;

  return (
    <div className="fixed inset-0 z-[60]" onClick={handleClick} aria-hidden="true">
      {!targetRect || isWaiting ? (
        <div className="absolute inset-0 bg-black/50 transition-opacity duration-200">
          {showSpinner && (
            <div className="absolute inset-0 flex items-center justify-center">
              <div className="w-6 h-6 border-2 border-pencil-light border-t-transparent rounded-full animate-spin" />
            </div>
          )}
        </div>
      ) : (
        <div
          className="absolute pointer-events-none transition-all duration-300 ease-out"
          style={{
            top: targetRect.top - PADDING,
            left: targetRect.left - PADDING,
            width: targetRect.width + PADDING * 2,
            height: targetRect.height + PADDING * 2,
            borderRadius: '8px',
            boxShadow: '0 0 0 9999px rgba(0, 0, 0, 0.5)',
          }}
        />
      )}
    </div>
  );
}
