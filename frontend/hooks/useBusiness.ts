"use client";

import { useState, useEffect, useCallback } from "react";
import { api, extractApiError } from "@/lib/api";
import type { Business } from "@/types/business";
import type { ApiSuccess } from "@/types/api";
import { isAuthenticated } from "@/lib/auth";

export function useBusiness() {
  const [businesses, setBusinesses] = useState<Business[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchBusinesses = useCallback(async () => {
    setIsLoading(true);
    setError(null);

    try {
      const { data } = await api.get<ApiSuccess<Business[]>>("/businesses");
      setBusinesses(data.data ?? []);
    } catch (err) {
      setError(extractApiError(err));
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!isAuthenticated()) return;

    let cancelled = false;

    async function loadBusinesses() {
      try {
        const { data } = await api.get<ApiSuccess<Business[]>>("/businesses");

        if (!cancelled) {
          setBusinesses(data.data ?? []);
          setError(null);
        }
      } catch (err) {
        if (!cancelled) {
          setError(extractApiError(err));
        }
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    }

    void loadBusinesses();

    return () => {
      cancelled = true;
    };
  }, []);

  return { businesses, isLoading, error, refetch: fetchBusinesses };
}
