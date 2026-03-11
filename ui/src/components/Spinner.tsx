import { Loader2 } from 'lucide-react';

interface SpinnerProps {
  size?: 'sm' | 'md' | 'lg';
  className?: string;
}

const sizeMap = {
  sm: 14,
  md: 18,
  lg: 24,
};

export default function Spinner({ size = 'md', className = '' }: SpinnerProps) {
  return (
    <Loader2
      size={sizeMap[size]}
      strokeWidth={2.5}
      className={`animate-spin text-pencil-light ${className}`}
    />
  );
}
