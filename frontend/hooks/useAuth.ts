"use client";
// frontend/hooks/useAuth.ts
// Central auth hook. All components use this to read auth state and trigger actions.
// State lives in module-level variables so it's shared across all hook instances.

import { useState, useEffect, useCallback } from "react";
import { store } from "@/lib/store";
import * as api from "@/lib/api";
import type { User, Business, MyMembership } from "@/types";

// Module-level listeners so all hook instances stay in sync
type Listener = () => void;
const listeners = new Set<Listener>();

function notify() {
  listeners.forEach((fn) => fn());
}

export function useAuth() {
  const [, setTick] = useState(0);

  useEffect(() => {
    const fn = () => setTick((t) => t + 1);
    listeners.add(fn);
    return () => {
      listeners.delete(fn);
    };
  }, []);

  const user = store.getUser();
  const accessToken = store.getAccessToken();
  const currentBusiness = store.getCurrentBusiness();
  const myMembership = store.getMyMembership();
  const isAuthenticated = store.isAuthenticated();

  const hasPermission = useCallback(
    (permission: string) => {
      return store.hasPermission(permission);
    },
    [myMembership],
  ); // eslint-disable-line react-hooks/exhaustive-deps

  const doLogin = useCallback(
    async (email: string, password: string): Promise<string | null> => {
      const res = await api.login({ email, password });
      if (!res.success || !res.data) {
        return res.error?.message || "Login failed";
      }
      store.setTokens(res.data.access_token, res.data.refresh_token);

      // Load user profile
      const meRes = await api.getMe();
      if (meRes.success && meRes.data) {
        store.setUser(meRes.data.user);
      }

      notify();
      return null;
    },
    [],
  );

  const doSignup = useCallback(
    async (data: {
      email: string;
      password: string;
      first_name: string;
      last_name: string;
    }): Promise<string | null> => {
      const res = await api.signup(data);
      if (!res.success) {
        return res.error?.message || "Signup failed";
      }
      return null;
    },
    [],
  );

  const doLogout = useCallback(async () => {
    const refreshToken = store.getRefreshToken();
    if (refreshToken) {
      await api.logout(refreshToken);
    }
    store.clear();
    notify();
  }, []);

  const doLogoutAll = useCallback(async () => {
    await api.logoutAll();
    store.clear();
    notify();
  }, []);

  const doSwitchBusiness = useCallback(
    async (business: Business): Promise<string | null> => {
      const res = await api.switchBusiness(business.id);
      if (!res.success || !res.data) {
        return res.error?.message || "Failed to switch business";
      }

      // Replace access token with the new one that has business_id embedded
      store.setAccessToken(res.data.access_token);
      store.setCurrentBusiness(business);

      // Load membership + permissions for this business
      const memberRes = await api.getMyMembership();
      if (memberRes.success && memberRes.data) {
        store.setMyMembership(memberRes.data.membership);
      }

      notify();
      return null;
    },
    [],
  );

  const doUpdateProfile = useCallback(
    async (data: {
      first_name: string;
      last_name: string;
    }): Promise<string | null> => {
      const res = await api.updateMe(data);
      if (!res.success || !res.data) {
        return res.error?.message || "Update failed";
      }
      store.setUser(res.data.user);
      notify();
      return null;
    },
    [],
  );

  return {
    user,
    accessToken,
    currentBusiness,
    myMembership,
    isAuthenticated,
    hasPermission,
    doLogin,
    doSignup,
    doLogout,
    doLogoutAll,
    doSwitchBusiness,
    doUpdateProfile,
  };
}
