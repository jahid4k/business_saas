"use client";
// frontend/app/dashboard/page.tsx
// Overview: system health, current user, business context indicator.

import { useState, useEffect } from "react";
import { useAuth } from "@/hooks/useAuth";
import * as api from "@/lib/api";

interface HealthData {
  status?: string;
  postgres?: string;
  redis?: string;
}

export default function DashboardPage() {
  const { user, currentBusiness, myMembership } = useAuth();
  const [health, setHealth] = useState<HealthData | null>(null);
  const [healthOk, setHealthOk] = useState<boolean | null>(null);

  useEffect(() => {
    api
      .getHealth()
      .then((res) => {
        if (res.success && res.data) {
          setHealth(res.data as HealthData);
          setHealthOk(true);
        } else {
          setHealthOk(false);
        }
      })
      .catch(() => setHealthOk(false));
  }, []);

  return (
    <div className="p-8 max-w-4xl">
      <h2 className="text-xl font-semibold text-white mb-1">Overview</h2>
      <p className="text-gray-400 text-sm mb-8">
        System status and current session
      </p>

      {/* System health */}
      <section className="mb-8">
        <h3 className="text-xs uppercase tracking-wider text-gray-500 mb-3">
          System Health
        </h3>
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
          <div className="flex items-center gap-2 mb-4">
            <span
              className={`w-2 h-2 rounded-full ${healthOk === null ? "bg-gray-600" : healthOk ? "bg-green-400" : "bg-red-400"}`}
            />
            <span className="text-sm text-gray-300">
              {healthOk === null
                ? "Checking..."
                : healthOk
                  ? "Backend reachable"
                  : "Backend unreachable"}
            </span>
          </div>
          {health && (
            <div className="grid grid-cols-3 gap-4">
              {Object.entries(health).map(([key, val]) => (
                <div key={key} className="bg-gray-800 rounded-lg p-3">
                  <p className="text-gray-500 text-xs capitalize mb-1">{key}</p>
                  <p className="text-white text-sm font-medium">
                    {String(val)}
                  </p>
                </div>
              ))}
            </div>
          )}
        </div>
      </section>

      {/* Current user */}
      <section className="mb-8">
        <h3 className="text-xs uppercase tracking-wider text-gray-500 mb-3">
          Current User
        </h3>
        {user ? (
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-5 grid grid-cols-2 gap-4">
            <Field label="User ID" value={user.id} mono />
            <Field label="Email" value={user.email} />
            <Field
              label="Name"
              value={`${user.first_name} ${user.last_name}`}
            />
            <Field label="Verified" value={user.is_verified ? "Yes" : "No"} />
            <Field
              label="Member since"
              value={new Date(user.created_at).toLocaleDateString()}
            />
          </div>
        ) : (
          <EmptyCard text="No user data" />
        )}
      </section>

      {/* Business context */}
      <section className="mb-8">
        <h3 className="text-xs uppercase tracking-wider text-gray-500 mb-3">
          Active Workspace
        </h3>
        {currentBusiness ? (
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-5 grid grid-cols-2 gap-4">
            <Field label="Business ID" value={currentBusiness.id} mono />
            <Field label="Name" value={currentBusiness.name} />
            <Field label="Slug" value={currentBusiness.slug} />
            <Field label="Role" value={myMembership?.role ?? "—"} badge />
          </div>
        ) : (
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
            <p className="text-gray-400 text-sm">No workspace selected.</p>
            <p className="text-gray-500 text-xs mt-1">
              Go to <strong className="text-gray-400">Businesses</strong> to
              create or switch into a workspace. Business-scoped features
              (Tasks, Members) require an active workspace.
            </p>
          </div>
        )}
      </section>

      {/* Permissions */}
      {myMembership && (
        <section>
          <h3 className="text-xs uppercase tracking-wider text-gray-500 mb-3">
            Your Permissions
          </h3>
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
            <div className="flex flex-wrap gap-2">
              {myMembership.permissions.map((p) => (
                <span
                  key={p}
                  className="bg-gray-800 text-gray-300 text-xs font-mono px-2.5 py-1 rounded-md"
                >
                  {p}
                </span>
              ))}
            </div>
          </div>
        </section>
      )}
    </div>
  );
}

function Field({
  label,
  value,
  mono,
  badge,
}: {
  label: string;
  value: string;
  mono?: boolean;
  badge?: boolean;
}) {
  return (
    <div>
      <p className="text-gray-500 text-xs mb-1">{label}</p>
      {badge ? (
        <span className="inline-block bg-indigo-900 text-indigo-300 text-xs font-medium px-2.5 py-1 rounded-md capitalize">
          {value}
        </span>
      ) : (
        <p className={`text-white text-sm ${mono ? "font-mono text-xs" : ""}`}>
          {value}
        </p>
      )}
    </div>
  );
}

function EmptyCard({ text }: { text: string }) {
  return (
    <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
      <p className="text-gray-500 text-sm">{text}</p>
    </div>
  );
}
