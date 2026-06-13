// frontend/lib/store.ts
// In-memory auth state. Never touches localStorage or sessionStorage.
// State is lost on page refresh — that's intentional for this test frontend.
// Production will use httpOnly cookies for refresh tokens.

import type { AuthState, User, Business, MyMembership } from "@/types";

const state: AuthState = {
  user: null,
  accessToken: null,
  refreshToken: null,
  currentBusiness: null,
  myMembership: null,
};

export const store = {
  getAccessToken: () => state.accessToken,
  getRefreshToken: () => state.refreshToken,
  getUser: () => state.user,
  getCurrentBusiness: () => state.currentBusiness,
  getMyMembership: () => state.myMembership,

  setTokens: (accessToken: string, refreshToken: string) => {
    state.accessToken = accessToken;
    state.refreshToken = refreshToken;
  },

  setUser: (user: User) => {
    state.user = user;
  },

  setCurrentBusiness: (business: Business | null) => {
    state.currentBusiness = business;
  },

  setMyMembership: (membership: MyMembership | null) => {
    state.myMembership = membership;
  },

  setAccessToken: (token: string) => {
    state.accessToken = token;
  },

  clear: () => {
    state.user = null;
    state.accessToken = null;
    state.refreshToken = null;
    state.currentBusiness = null;
    state.myMembership = null;
  },

  isAuthenticated: () => !!state.accessToken && !!state.user,

  hasPermission: (permission: string) => {
    if (!state.myMembership) return false;
    return state.myMembership.permissions.includes(permission);
  },
};
