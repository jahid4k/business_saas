// src/stores/authStore.ts
import { create } from "zustand";
import type { SafeUser } from "@/types/auth";
import type { Business, MembershipWithRole } from "@/types/org";

interface AuthState {
  user: SafeUser | null;
  currentOrg: Business | null;
  currentMembership: MembershipWithRole | null;
  setUser: (user: SafeUser | null) => void;
  setOrg: (org: Business | null, membership: any) => void;
  reset: () => void;
}

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  currentOrg: null,
  currentMembership: null,

  /**
   * Sets the authenticated user data (nested from /auth/me or signup responses)
   */
  setUser: (user) => set({ user }),

  /**
   * Hydrates the active workspace context after an organization switch
   */
  setOrg: (org, membership) =>
    set({
      currentOrg: org,
      currentMembership: membership,
    }),

  /**
   * Completely clears out the store on logout to prevent state leakage
   */
  reset: () =>
    set({
      user: null,
      currentOrg: null,
      currentMembership: null,
    }),
}));
