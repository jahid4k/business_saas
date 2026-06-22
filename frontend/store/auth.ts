// store/auth.ts
// Client-side user state. Populated from next-auth session on mount.
// Does NOT store the access token — that lives in lib/api.ts (_accessToken).

import { create } from "zustand";
import type { User } from "@/types/domain";

interface AuthStore {
  user: User | null;
  isHydrated: boolean;
  setUser: (user: User) => void;
  clearUser: () => void;
  setHydrated: () => void;
}

export const useAuthStore = create<AuthStore>((set) => ({
  user: null,
  isHydrated: false,

  setUser: (user) => set({ user }),
  clearUser: () => set({ user: null }),
  setHydrated: () => set({ isHydrated: true }),
}));
