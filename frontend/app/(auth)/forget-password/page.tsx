"use client";
// frontend/app/(auth)/forgot-password/page.tsx

import { useState, FormEvent } from "react";
import Link from "next/link";
import * as api from "@/lib/api";

export default function ForgotPasswordPage() {
  const [email, setEmail] = useState("");
  const [sent, setSent] = useState(false);
  const [loading, setLoading] = useState(false);

  // Reset confirm state
  const [resetToken, setResetToken] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmed, setConfirmed] = useState(false);
  const [error, setError] = useState("");
  const [confirmLoading, setConfirmLoading] = useState(false);

  async function handleRequest(e: FormEvent) {
    e.preventDefault();
    setLoading(true);
    await api.passwordResetRequest(email);
    setLoading(false);
    setSent(true);
  }

  async function handleConfirm(e: FormEvent) {
    e.preventDefault();
    setError("");
    setConfirmLoading(true);
    const res = await api.passwordResetConfirm({
      token: resetToken,
      new_password: newPassword,
    });
    setConfirmLoading(false);
    if (!res.success) {
      setError(
        res.error?.message || "Reset failed. Token may be invalid or expired.",
      );
    } else {
      setConfirmed(true);
    }
  }

  return (
    <div className="min-h-screen bg-gray-950 flex items-center justify-center px-4">
      <div className="w-full max-w-sm">
        <div className="mb-8 text-center">
          <h1 className="text-2xl font-bold text-white">Password Reset</h1>
          <p className="text-gray-400 text-sm mt-1">
            We&apos;ll send you a reset token
          </p>
        </div>

        <div className="bg-gray-900 border border-gray-800 rounded-xl p-6 space-y-4">
          {confirmed ? (
            <div className="text-center space-y-4">
              <div className="text-green-400 text-sm">
                Password reset successful!
              </div>
              <Link
                href="/login"
                className="block text-indigo-400 hover:text-indigo-300 text-sm"
              >
                Sign in with your new password →
              </Link>
            </div>
          ) : !sent ? (
            <form onSubmit={handleRequest} className="space-y-4">
              <div>
                <label className="block text-sm text-gray-400 mb-1.5">
                  Email address
                </label>
                <input
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  required
                  autoFocus
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2.5 text-white text-sm placeholder-gray-500 focus:outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500"
                  placeholder="you@example.com"
                />
              </div>
              <button
                type="submit"
                disabled={loading}
                className="w-full bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white text-sm font-medium rounded-lg py-2.5 transition-colors"
              >
                {loading ? "Sending..." : "Send reset token"}
              </button>
            </form>
          ) : (
            <form onSubmit={handleConfirm} className="space-y-4">
              <p className="text-sm text-gray-400">
                If <span className="text-white">{email}</span> is registered, a
                reset token was generated. In development, check the backend
                logs for the token.
              </p>

              {error && (
                <div className="bg-red-950 border border-red-800 text-red-300 text-sm rounded-lg px-4 py-3">
                  {error}
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
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2.5 text-white text-sm font-mono placeholder-gray-500 focus:outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500"
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
                disabled={confirmLoading}
                className="w-full bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white text-sm font-medium rounded-lg py-2.5 transition-colors"
              >
                {confirmLoading ? "Resetting..." : "Reset password"}
              </button>
            </form>
          )}

          <div className="text-center pt-1">
            <Link
              href="/login"
              className="text-sm text-gray-400 hover:text-gray-300"
            >
              ← Back to sign in
            </Link>
          </div>
        </div>
      </div>
    </div>
  );
}
