import { Component, ErrorInfo, ReactNode } from 'react';
import Link from 'next/link';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type FallbackRenderer = (props: { error: Error; reset: () => void }) => ReactNode;

interface ErrorBoundaryProps {
    children?: ReactNode;
    /** Optional custom fallback UI. Receives the error and a reset function. */
    fallback?: ReactNode | FallbackRenderer;
    /** Optional error reporting callback (e.g. Sentry, Datadog). */
    onError?: (error: Error, errorInfo: ErrorInfo) => void;
}

interface ErrorBoundaryState {
    hasError: boolean;
    error: Error | null;
    errorInfo: ErrorInfo | null;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
    constructor(props: ErrorBoundaryProps) {
        super(props);
        this.state = { hasError: false, error: null, errorInfo: null };
        this.reset = this.reset.bind(this);
    }

    static getDerivedStateFromError(error: Error): ErrorBoundaryState {
        return { hasError: true, error, errorInfo: null };
    }

    componentDidCatch(error: Error, errorInfo: ErrorInfo) {
        this.setState({ errorInfo });

        // Call optional external reporter (e.g. Sentry.captureException)
        this.props.onError?.(error, errorInfo);

        if (process.env.NODE_ENV === 'development') {
            console.error('ErrorBoundary caught an error:', error, errorInfo);
        }
    }

    reset() {
        this.setState({ hasError: false, error: null, errorInfo: null });
    }

    render() {
        const { children = null, fallback } = this.props;
        const { hasError, error, errorInfo } = this.state;

        if (!hasError) {
            return children;
        }

        // Custom fallback — function form
        if (typeof fallback === 'function') {
            return (fallback as FallbackRenderer)({ error: error!, reset: this.reset });
        }

        // Custom fallback — static ReactNode
        if (fallback) {
            return fallback;
        }

        // Default fallback UI
        return (
            <div className="flex min-h-screen flex-col items-center justify-center bg-white p-6 text-center">
                <div className="mb-6 flex size-16 items-center justify-center rounded-full bg-red-100">
                    <svg
                        className="size-8 text-red-500"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                        strokeWidth={2}
                    >
                        <path
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            d="M12 9v2m0 4h.01M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z"
                        />
                    </svg>
                </div>

                <h1 className="mb-2 text-2xl font-semibold text-gray-900">
                    Something went wrong
                </h1>

                <p className="mb-6 max-w-md text-sm text-gray-500">
                    An unexpected error occurred. You can try again.
                </p>

                {/* Dev-only error details */}
                {process.env.NODE_ENV === 'development' && error && (
                    <details className="mb-6 w-full max-w-lg rounded-lg border border-red-200 bg-red-50 p-4 text-left">
                        <summary className="cursor-pointer text-sm font-medium text-red-700">
                            Error details
                        </summary>
                        <pre className="mt-2 overflow-auto whitespace-pre-wrap break-words text-xs text-red-600">
							{error.toString()}
                            {errorInfo?.componentStack}
						</pre>
                    </details>
                )}

                <div className="flex gap-3">
                    <button
                        type="button"
                        onClick={this.reset}
                        className="rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2"
                    >
                        Try again
                    </button>
                    <button
                        type="button"
                        onClick={() => window.location.reload()}
                        className="rounded-md border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2"
                    >
                        Refresh page
                    </button>
            </div>
    </div>
    );
    }
}

export default ErrorBoundary;