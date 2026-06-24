// src/lib/token.ts

// In-memory variable to hold the short-lived access token securely.
// This will reset on a hard page refresh, which immediately triggers
// a silent refresh call to fetch a new token seamlessly.
let memoryToken: string | null = null;

/**
 * Retrieves the current in-memory access token.
 */
export function getToken(): string | null {
  return memoryToken;
}

/**
 * Updates or clears the in-memory access token.
 * * @param token - The new JWT string, or null to clear the session context.
 */
export function setToken(token: string | null): void {
  memoryToken = token;
}
