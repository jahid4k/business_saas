// lib/query-client.ts
// TanStack Query client with sensible defaults for a SaaS dashboard.

import { QueryClient } from "@tanstack/react-query";
import { ApiError } from "@/types/api";
import { toast } from "sonner";

function makeQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        // Data is considered fresh for 30 seconds — no refetch within this window
        staleTime: 30 * 1000,
        // Keep unused query data in cache for 5 minutes
        gcTime: 5 * 60 * 1000,
        // Retry once on failure (not for auth errors)
        retry: (failureCount, error) => {
          if (error instanceof ApiError && error.status === 401) return false;
          if (error instanceof ApiError && error.status === 403) return false;
          if (error instanceof ApiError && error.status === 404) return false;
          return failureCount < 1;
        },
        refetchOnWindowFocus: true,
        refetchOnReconnect: true,
      },
      mutations: {
        // Show toast on unhandled mutation errors
        onError: (error) => {
          if (error instanceof ApiError) {
            // Field errors are handled inline by the form — skip toast
            if (error.hasFieldErrors) return;
            toast.error(error.message, {
              description:
                error.requestId ? `Reference: ${error.requestId}` : undefined,
            });
          } else {
            toast.error("Something went wrong. Please try again.");
          }
        },
      },
    },
  });
}

// SSR-safe singleton pattern for Next.js App Router
let browserQueryClient: QueryClient | undefined;

export function getQueryClient() {
  if (typeof window === "undefined") {
    // Server: always create a new client
    return makeQueryClient();
  }
  // Browser: create once and reuse
  if (!browserQueryClient) {
    browserQueryClient = makeQueryClient();
  }
  return browserQueryClient;
}
