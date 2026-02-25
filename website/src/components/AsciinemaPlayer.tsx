import {useEffect, useRef} from 'react';
import 'asciinema-player/dist/bundle/asciinema-player.css';

interface Props {
  src: string;
  cols?: number;
  rows?: number;
  autoPlay?: boolean;
  speed?: number;
  theme?: string;
  fit?: string;
}

export default function AsciinemaPlayer({
  src,
  cols = 120,
  rows = 30,
  autoPlay = false,
  speed = 1.5,
  theme = 'monokai',
  fit = 'width',
}: Props) {
  const ref = useRef<HTMLDivElement>(null);
  const playerRef = useRef<any>(null);

  useEffect(() => {
    import('asciinema-player').then((AsciinemaPlayerLib) => {
      if (ref.current && !playerRef.current) {
        playerRef.current = AsciinemaPlayerLib.create(src, ref.current, {
          cols,
          rows,
          autoPlay,
          speed,
          theme,
          fit,
        });
      }
    });

    return () => {
      playerRef.current?.dispose();
      playerRef.current = null;
    };
  }, [src]);

  return (
    <div
      style={{
        border: '2px solid var(--ifm-color-emphasis-300)',
        borderRadius: '12px 8px 14px 10px',
        boxShadow: '4px 4px 0 var(--ifm-color-emphasis-200)',
        overflow: 'hidden',
        margin: '1.5rem 0',
      }}
      ref={ref}
    />
  );
}
