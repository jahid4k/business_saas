"use client";

import { ErrorBoundary } from "react-error-boundary";
import { AlertCircle } from "lucide-react";
import { Button } from "@/components/ui/button";

function ErrorFallback({
  error,
  resetErrorBoundary,
  moduleName,
}: {
  error: Error;
  resetErrorBoundary: () => void;
  moduleName: string;
}) {
  return (
    <div className="flex flex-col items-center justify-center py-16 text-center">
      <div className="mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-destructive/10">
        <AlertCircle className="h-7 w-7 text-destructive" />
      </div>

      <h3 className="mb-1 text-sm font-semibold">{moduleName} failed to load</h3>

      <p className="mb-2 max-w-sm text-sm text-muted-foreground">
        Something went wrong in this section. This hasn&apos;t affected the rest of
        the app.
      </p>

      {process.env.NODE_ENV === "development" && (
        <p className="mb-4 max-w-sm font-mono text-xs text-destructive">
          {error.message}
        </p>
      )}

      <Button size="sm" variant="outline" onClick={resetErrorBoundary}>
        Try again
      </Button>
    </div>
  );
}

interface ModuleErrorBoundaryProps {
  moduleName: string;
  children: React.ReactNode;
}

export function ModuleErrorBoundary({
  moduleName,
  children,
}: ModuleErrorBoundaryProps) {
  return (
    <ErrorBoundary
      fallbackRender={({ error, resetErrorBoundary }) => (
        <ErrorFallback
          error={error}
          resetErrorBoundary={resetErrorBoundary}
          moduleName={moduleName}
        />
      )}
      onError={(error) => {
        console.error(`[${moduleName}] Error boundary caught:`, error);
      }}
    >
      {children}
    </ErrorBoundary>
  );
}
