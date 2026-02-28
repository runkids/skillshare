import { Component, type ReactNode } from 'react';
import { AlertTriangle } from 'lucide-react';
import Card from './Card';
import HandButton from './HandButton';

interface Props {
  children: ReactNode;
}
interface State {
  hasError: boolean;
  error: Error | null;
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false, error: null };

  static getDerivedStateFromError(error: Error) {
    return { hasError: true, error };
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className="p-8 max-w-lg mx-auto mt-16">
          <Card variant="accent">
            <div className="flex items-start gap-3">
              <AlertTriangle size={24} className="text-danger shrink-0 mt-0.5" />
              <div>
                <h2
                  className="text-xl font-bold text-pencil mb-2"
                  style={{ fontFamily: 'var(--font-heading)' }}
                >
                  Something went wrong
                </h2>
                <p
                  className="text-pencil-light mb-4"
                  style={{ fontFamily: 'var(--font-hand)' }}
                >
                  {this.state.error?.message || 'An unexpected error occurred.'}
                </p>
                <HandButton variant="secondary" onClick={() => window.location.reload()}>
                  Reload page
                </HandButton>
              </div>
            </div>
          </Card>
        </div>
      );
    }
    return this.props.children;
  }
}
