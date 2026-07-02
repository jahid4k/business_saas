// src/components/providers/QueryProvider.tsx
"use client";

import { useState } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ReactQueryDevtools } from "@tanstack/react-query-devtools";

export default function QueryProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  // useState (not a module-level singleton) — each browser session gets its
  // own QueryClient instance. This is the official Next.js App Router
  // pattern and is what keeps the door open if any route ever starts doing
  // server-side prefetching later.
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            staleTime: 30 * 1000, // 30s — avoids refetch storms on remount, still feels live
            retry: 1, // fail fast into a designed error state instead of a long spinner
            refetchOnWindowFocus: true, // per ADR-0007: refresh stale dashboards on tab focus
          },
          mutations: {
            retry: 0, // never auto-retry a write — could duplicate a side effect
          },
        },
      }),
  );

  return (
    <QueryClientProvider client={queryClient}>
      {children}
      {process.env.NODE_ENV === "development" && (
        <ReactQueryDevtools
          initialIsOpen={false}
          buttonPosition="bottom-left"
        />
      )}
    </QueryClientProvider>
  );
}
