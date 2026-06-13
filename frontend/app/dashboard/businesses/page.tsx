"use client";
// frontend/app/dashboard/businesses/page.tsx
// Create businesses, list them, switch workspace context.

import { useState, useEffect, FormEvent } from "react";
import { useAuth } from "@/hooks/useAuth";
import * as api from "@/lib/api";
import type { MembershipWithRole } from "@/types";

export default function BusinessesPage() {
  const { currentBusiness, doSwitchBusiness } = useAuth();

  const [memberships, setMemberships] = useState<MembershipWithRole[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  // Create form
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState("");
  const [createSuccess, setCreateSuccess] = useState("");

  // Switch state
  const [switching, setSwitching] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function loadInitialBusinesses() {
      try {
        const res = await api.listBusinesses();

        if (cancelled) return;

        if (res.success && res.data) {
          setMemberships(res.data.businesses ?? []);
          setError("");
        } else {
          setMemberships([]);
          setError(res.error?.message || "Failed to load businesses");
        }
      } catch (err) {
        if (cancelled) return;

        setMemberships([]);
        setError(
          err instanceof Error ? err.message : "Failed to load businesses",
        );
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    }

    void loadInitialBusinesses();

    return () => {
      cancelled = true;
    };
  }, []);

  async function reloadBusinesses() {
    setLoading(true);
    setError("");

    try {
      const res = await api.listBusinesses();

      if (res.success && res.data) {
        setMemberships(res.data.businesses ?? []);
        setError("");
      } else {
        setMemberships([]);
        setError(res.error?.message || "Failed to load businesses");
      }
    } catch (err) {
      setMemberships([]);
      setError(
        err instanceof Error ? err.message : "Failed to load businesses",
      );
    } finally {
      setLoading(false);
    }
  }

  // Auto-generate slug from name
  function handleNameChange(val: string) {
    setName(val);
    setSlug(
      val
        .toLowerCase()
        .replace(/[^a-z0-9]+/g, "-")
        .replace(/^-|-$/g, ""),
    );
  }

  async function handleCreate(e: FormEvent) {
    e.preventDefault();

    const createdName = name;

    setCreateError("");
    setCreateSuccess("");
    setCreating(true);

    try {
      const res = await api.createBusiness({ name, slug });

      if (!res.success) {
        setCreateError(res.error?.message || "Failed to create business");
        return;
      }

      setCreateSuccess(`"${createdName}" created successfully!`);
      setName("");
      setSlug("");

      await reloadBusinesses();
    } catch (err) {
      setCreateError(
        err instanceof Error ? err.message : "Failed to create business",
      );
    } finally {
      setCreating(false);
    }
  }

  async function handleSwitch(membership: MembershipWithRole) {
    setSwitching(membership.business.id);
    setError("");

    const err = await doSwitchBusiness(membership.business);

    setSwitching(null);

    if (err) {
      setError(err);
    }
  }

  return (
    <div className="p-8 max-w-4xl">
      <h2 className="text-xl font-semibold text-white mb-1">Businesses</h2>
      <p className="text-gray-400 text-sm mb-8">
        Create workspaces and switch business context
      </p>

      {/* Create form */}
      <section className="mb-8">
        <h3 className="text-xs uppercase tracking-wider text-gray-500 mb-3">
          Create New Business
        </h3>

        <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
          <form onSubmit={handleCreate} className="space-y-4">
            {createError && (
              <div className="bg-red-950 border border-red-800 text-red-300 text-sm rounded-lg px-4 py-3">
                {createError}
              </div>
            )}

            {createSuccess && (
              <div className="bg-green-950 border border-green-800 text-green-300 text-sm rounded-lg px-4 py-3">
                {createSuccess}
              </div>
            )}

            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-sm text-gray-400 mb-1.5">
                  Business name
                </label>
                <input
                  type="text"
                  value={name}
                  onChange={(e) => handleNameChange(e.target.value)}
                  required
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2.5 text-white text-sm placeholder-gray-500 focus:outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500"
                  placeholder="Acme Corp"
                />
              </div>

              <div>
                <label className="block text-sm text-gray-400 mb-1.5">
                  Slug (URL-safe)
                </label>
                <input
                  type="text"
                  value={slug}
                  onChange={(e) => setSlug(e.target.value)}
                  required
                  pattern="[a-z0-9]+(?:-[a-z0-9]+)*"
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2.5 text-white text-sm font-mono placeholder-gray-500 focus:outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500"
                  placeholder="acme-corp"
                />
              </div>
            </div>

            <button
              type="submit"
              disabled={creating}
              className="bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white text-sm font-medium rounded-lg px-5 py-2.5 transition-colors"
            >
              {creating ? "Creating..." : "Create business"}
            </button>
          </form>
        </div>
      </section>

      {/* Business list */}
      <section>
        <h3 className="text-xs uppercase tracking-wider text-gray-500 mb-3">
          Your Businesses
        </h3>

        {loading ? (
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
            <p className="text-gray-500 text-sm">Loading...</p>
          </div>
        ) : error ? (
          <div className="bg-red-950 border border-red-800 text-red-300 text-sm rounded-xl p-5">
            {error}
          </div>
        ) : memberships.length === 0 ? (
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
            <p className="text-gray-400 text-sm">
              No businesses yet. Create one above.
            </p>
          </div>
        ) : (
          <div className="space-y-3">
            {memberships.map((m) => {
              const isActive = currentBusiness?.id === m.business.id;

              return (
                <div
                  key={m.business.id}
                  className={`bg-gray-900 border rounded-xl p-5 flex items-center justify-between ${
                    isActive ? "border-indigo-600" : "border-gray-800"
                  }`}
                >
                  <div>
                    <div className="flex items-center gap-2">
                      <p className="text-white text-sm font-medium">
                        {m.business.name}
                      </p>

                      {isActive && (
                        <span className="bg-indigo-900 text-indigo-300 text-xs px-2 py-0.5 rounded">
                          Active
                        </span>
                      )}
                    </div>

                    <p className="text-gray-500 text-xs mt-0.5">
                      {m.business.slug} · Role:{" "}
                      <span className="capitalize text-gray-400">{m.role}</span>
                    </p>

                    <p className="text-gray-600 text-xs font-mono mt-1">
                      {m.business.id}
                    </p>
                  </div>

                  <button
                    onClick={() => handleSwitch(m)}
                    disabled={isActive || switching === m.business.id}
                    className="bg-gray-800 hover:bg-gray-700 disabled:opacity-40 disabled:cursor-not-allowed text-gray-300 text-xs font-medium rounded-lg px-4 py-2 transition-colors"
                  >
                    {switching === m.business.id
                      ? "Switching..."
                      : isActive
                        ? "Current"
                        : "Switch to"}
                  </button>
                </div>
              );
            })}
          </div>
        )}
      </section>
    </div>
  );
}
