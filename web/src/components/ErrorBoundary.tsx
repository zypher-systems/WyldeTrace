import { Component, type ErrorInfo, type ReactNode } from "react";

type Props = { children: ReactNode };
type State = { error: Error | null };

export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("UI error boundary", error, info.componentStack);
  }

  render() {
    if (this.state.error) {
      return (
        <div className="flex min-h-screen flex-col items-center justify-center gap-4 bg-zinc-950 px-6 text-center text-zinc-100">
          <h1 className="text-xl font-semibold">Something went wrong</h1>
          <p className="max-w-md text-sm text-zinc-400">
            The page hit an unexpected error. Reload to continue, or return to the dashboard.
          </p>
          <p className="max-w-lg font-mono text-xs text-zinc-600">{this.state.error.message}</p>
          <div className="flex gap-3">
            <button
              type="button"
              className="wt-btn wt-btn-primary"
              onClick={() => window.location.assign("/")}
            >
              Dashboard
            </button>
            <button
              type="button"
              className="wt-btn wt-btn-secondary"
              onClick={() => window.location.reload()}
            >
              Reload
            </button>
          </div>
        </div>
      );
    }
    return this.props.children;
  }
}
