// hooks/useAuth.ts
// Thin wrapper over next-auth session.
// Returns the current user and a logout helper.

"use client";

import { useSession, signOut } from "next-auth/react";
import { useRouter } from "next/navigation";
import { clearAccessToken } from "@/lib/api";

export function useAuth() {
  const { data: session, status } = useSession();
  const router = useRouter();

  const user = session?.user
    ? {
        id: session.user.id,
        email: session.user.email ?? "",
        name: session.user.name ?? "",
        activeOrgId: session.user.activeOrgId ?? null,
        activeOrgSlug: session.user.activeOrgSlug ?? null,
        activeRole: session.user.activeRole ?? null,
      }
    : null;

  async function logout() {
    clearAccessToken();
    await signOut({ redirect: false });
    router.push("/login");
  }

  return {
    user,
    isLoading: status === "loading",
    isAuthenticated: status === "authenticated",
    logout,
  };
}
