"use client";

// TokenHydrator.tsx
// Runs on the client after server render.
// Loads the Go backend access token from the next-auth session
// into the in-memory store (lib/api.ts _accessToken).
//
// This is safe because:
//   - It only runs in the browser (client component)
//   - The token is never written to localStorage or any persistent storage
//   - The session carrying the token is encrypted by next-auth

import { useEffect } from "react";
import { setAccessToken } from "@/lib/api";

interface TokenHydratorProps {
  accessToken: string;
}

export function TokenHydrator({ accessToken }: TokenHydratorProps) {
  useEffect(() => {
    if (accessToken) {
      setAccessToken(accessToken);
    }
  }, [accessToken]);

  return null; // renders nothing
}
