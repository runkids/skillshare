import { Navigate, useSearchParams } from 'react-router-dom';

export default function HooksPage() {
  const [searchParams] = useSearchParams();
  const params = new URLSearchParams();
  params.set('tab', 'hooks');
  for (const [key, value] of searchParams.entries()) {
    if (key === 'tab') continue;
    params.append(key, value);
  }
  return <Navigate replace to={`/resources?${params.toString()}`} />;
}
