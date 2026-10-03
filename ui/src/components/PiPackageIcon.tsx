/** Pi's mark in one color (the text color), for Pi packages: Clean keeps package tiles black and white. */
export default function PiPackageIcon({ size = 16 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 800 800" fill="currentColor" aria-hidden="true">
      <path d="M165.29 165.29H517.36V400H400V282.65H165.29Z" />
      <path d="M165.29 282.65H282.65V400H400V517.36H282.65V634.72H165.29Z" />
      <path d="M517.36 400H634.72V634.72H517.36Z" />
    </svg>
  );
}
