"use client";
// frontend/app/dashboard/profile/page.tsx
// Profile management: update name, logout, logout all, password reset.

import { useState, FormEvent } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/hooks/useAuth";
import * as api from "@/lib/api";

export default function ProfilePage() {
  const router = useRouter();
  const { user, doUpdateProfile, doLogout, doLogoutAll } = useAuth();

  // Profile update
  const [firstName, setFirstName] = useState(user?.first_name ?? "");
  const [lastName, setLastName] = useState(user?.last_name ?? "");
  const [updateLoading, setUpdateLoading] = useState(false);
  const [updateError, setUpdateError] = useState("");
  const [updateSuccess, setUpdateSuccess] = useState("");

  // Password reset
  const [resetEmail, setResetEmail] = useState(user?.email ?? "");
  const [resetSent, setResetSent] = useState(false);
  const [resetToken, setResetToken] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [resetLoading, setResetLoading] = useState(false);
  const [resetError, setResetError] = useState("");
  const [resetSuccess, setResetSuccess] = useState("");

  async function handleUpdateProfile(e: FormEvent) {
    e.preventDefault();
    setUpdateError("");
    setUpdateSuccess("");
    setUpdateLoading(true);
    const err = await doUpdateProfile({
      first_name: firstName,
      last_name: lastName,
    });
    setUpdateLoading(false);
    if (err) {
      setUpdateError(err);
    } else {
      setUpdateSuccess("Profile updated successfully");
    }
  }

  async function handlePasswordResetRequest(e: FormEvent) {
    e.preventDefault();
    setResetLoading(true);
    await api.passwordResetRequest(resetEmail);
    setResetLoading(false);
    setResetSent(true);
  }

  async function handlePasswordResetConfirm(e: FormEvent) {
    e.preventDefault();
    setResetError("");
    setResetLoading(true);
    const res = await api.passwordResetConfirm({
      token: resetToken,
      new_password: newPassword,
    });
    setResetLoading(false);
    if (!res.success) {
      setResetError(
        res.error?.message || "Reset failed. Token may be invalid or expired.",
      );
    } else {
      setResetSuccess("Password changed. Please sign in again.");
      setTimeout(async () => {
        await doLogout();
        router.push("/login");
      }, 2000);
    }
  }

  async function handleLogout() {
    await doLogout();
    router.push("/login");
  }

  async function handleLogoutAll() {
    await doLogoutAll();
    router.push("/login");
  }

  return (
    <div className="p-8 max-w-2xl">
      <h2 className="text-xl font-semibold text-white mb-1">Profile</h2>
      <p className="text-gray-400 text-sm mb-8">Manage your account</p>

      {/* Account info */}
      <section className="mb-8">
        <h3 className="text-xs uppercase tracking-wider text-gray-500 mb-3">
          Account Info
        </h3>
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-5 space-y-3">
          <div>
            <p className="text-gray-500 text-xs mb-1">User ID</p>
            <p className="text-white text-xs font-mono">{user?.id}</p>
          </div>
          <div>
            <p className="text-gray-500 text-xs mb-1">Email</p>
            <p className="text-white text-sm">{user?.email}</p>
          </div>
          <div>
            <p className="text-gray-500 text-xs mb-1">Email verified</p>
            <p className="text-white text-sm">
              {user?.is_verified ? "Yes" : "No"}
            </p>
          </div>
        </div>
      </section>

      {/* Update profile */}
      <section className="mb-8">
        <h3 className="text-xs uppercase tracking-wider text-gray-500 mb-3">
          Update Name
        </h3>
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
          <form onSubmit={handleUpdateProfile} className="space-y-4">
            {updateError && (
              <div className="bg-red-950 border border-red-800 text-red-300 text-sm rounded-lg px-4 py-3">
                {updateError}
              </div>
            )}
            {updateSuccess && (
              <div className="bg-green-950 border border-green-800 text-green-300 text-sm rounded-lg px-4 py-3">
                {updateSuccess}
              </div>
            )}
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-sm text-gray-400 mb-1.5">
                  First name
                </label>
                <input
                  type="text"
                  value={firstName}
                  onChange={(e) => setFirstName(e.target.value)}
                  required
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2.5 text-white text-sm placeholder-gray-500 focus:outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500"
                />
              </div>
              <div>
                <label className="block text-sm text-gray-400 mb-1.5">
                  Last name
                </label>
                <input
                  type="text"
                  value={lastName}
                  onChange={(e) => setLastName(e.target.value)}
                  required
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2.5 text-white text-sm placeholder-gray-500 focus:outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500"
                />
              </div>
            </div>
            <button
              type="submit"
              disabled={updateLoading}
              className="bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white text-sm font-medium rounded-lg px-5 py-2.5 transition-colors"
            >
              {updateLoading ? "Saving..." : "Save changes"}
            </button>
          </form>
        </div>
      </section>

      {/* Password reset */}
      <section className="mb-8">
        <h3 className="text-xs uppercase tracking-wider text-gray-500 mb-3">
          Change Password
        </h3>
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
          {resetSuccess ? (
            <p className="text-green-400 text-sm">{resetSuccess}</p>
          ) : !resetSent ? (
            <form onSubmit={handlePasswordResetRequest} className="space-y-4">
              <p className="text-gray-500 text-xs">
                We&apos;ll generate a reset token. In development, copy it from
                the backend logs.
              </p>
              <div>
                <label className="block text-sm text-gray-400 mb-1.5">
                  Your email
                </label>
                <input
                  type="email"
                  value={resetEmail}
                  onChange={(e) => setResetEmail(e.target.value)}
                  required
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2.5 text-white text-sm placeholder-gray-500 focus:outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500"
                />
              </div>
              <button
                type="submit"
                disabled={resetLoading}
                className="bg-gray-700 hover:bg-gray-600 disabled:opacity-50 text-white text-sm font-medium rounded-lg px-5 py-2.5 transition-colors"
              >
                {resetLoading ? "Sending..." : "Send reset token"}
              </button>
            </form>
          ) : (
            <form onSubmit={handlePasswordResetConfirm} className="space-y-4">
              <p className="text-gray-400 text-xs">
                Token sent. Check backend logs in development.
              </p>
              {resetError && (
                <div className="bg-red-950 border border-red-800 text-red-300 text-sm rounded-lg px-4 py-3">
                  {resetError}
                </div>
              )}
              <div>
                <label className="block text-sm text-gray-400 mb-1.5">
                  Reset token
                </label>
                <input
                  type="text"
                  value={resetToken}
                  onChange={(e) => setResetToken(e.target.value)}
                  required
                  autoFocus
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2.5 text-white text-xs font-mono placeholder-gray-500 focus:outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500"
                  placeholder="Paste token from backend logs"
                />
              </div>
              <div>
                <label className="block text-sm text-gray-400 mb-1.5">
                  New password
                </label>
                <input
                  type="password"
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  required
                  minLength={8}
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2.5 text-white text-sm placeholder-gray-500 focus:outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500"
                  placeholder="Min 8 characters"
                />
              </div>
              <button
                type="submit"
                disabled={resetLoading}
                className="bg-gray-700 hover:bg-gray-600 disabled:opacity-50 text-white text-sm font-medium rounded-lg px-5 py-2.5 transition-colors"
              >
                {resetLoading ? "Resetting..." : "Reset password"}
              </button>
            </form>
          )}
        </div>
      </section>

      {/* Session management */}
      <section>
        <h3 className="text-xs uppercase tracking-wider text-gray-500 mb-3">
          Sessions
        </h3>
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-5 space-y-3">
          <div>
            <p className="text-gray-400 text-sm font-medium mb-1">
              Sign out this device
            </p>
            <p className="text-gray-500 text-xs mb-3">
              Revokes your current refresh token only.
            </p>
            <button
              onClick={handleLogout}
              className="bg-gray-800 hover:bg-gray-700 text-white text-sm font-medium rounded-lg px-5 py-2.5 transition-colors"
            >
              Sign out
            </button>
          </div>
          <div className="border-t border-gray-800 pt-3">
            <p className="text-gray-400 text-sm font-medium mb-1">
              Sign out all devices
            </p>
            <p className="text-gray-500 text-xs mb-3">
              Revokes all active sessions everywhere.
            </p>
            <button
              onClick={handleLogoutAll}
              className="bg-red-950 hover:bg-red-900 border border-red-800 text-red-300 text-sm font-medium rounded-lg px-5 py-2.5 transition-colors"
            >
              Sign out all devices
            </button>
          </div>
        </div>
      </section>
    </div>
  );
}
