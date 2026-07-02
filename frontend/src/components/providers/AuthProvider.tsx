// src/components/providers/AuthProvider.tsx
"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { getToken, setToken } from "@/lib/token";
import { silentRefresh, getMe } from "@/lib/auth";
import { useAuthStore } from "@/stores/authStore";

export default function AuthProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  const [checking, setChecking] = useState(true);
  const router = useRouter();
  const { setUser } = useAuthStore();

  useEffect(() => {
    // Token already in memory = client-side navigation within same session.
    // Don't refresh again — token is valid until it expires (Axios handles 401).
    if (getToken()) {
      setChecking(false);
      return;
    }

    const init = async () => {
      try {
        const newToken = await silentRefresh();
        setToken(newToken);
        const user = await getMe();
        setUser(user);
      } catch {
        // Refresh failed = no valid session
        setToken(null);
        router.replace("/login");
      } finally {
        setChecking(false);
      }
    };

    init();
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  if (checking) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-white dark:bg-[#0a0a0a]">
        <div className="flex items-center gap-3">
          <span className="w-5 h-5 rounded-full animate-spin block border-2 border-purple-600/20 border-t-purple-600" />
          <span className="text-sm text-gray-400 dark:text-[#555]">
            Loading workspace…
          </span>
        </div>
      </div>
    );
  }

  return <>{children}</>;
}
