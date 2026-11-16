/** The check that ends a run: drawn once, short arm first. Lucide's path runs the other way. */
export default function DrawnCheck({ size = 20 }: { size?: number }) {
  return (
    <svg className="ss-draw" width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2.6} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M4 12l5 5L20 6" />
    </svg>
  );
}
