"use client";

import { useState } from "react";
import Link from "next/link";
import { useAuth } from "@/hooks/useAuth";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";

export function LoginForm() {
  const { login, isLoading, error } = useAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    await login({ email: email.trim().toLowerCase(), password });
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <Input
        label="Email address"
        type="email"
        value={email}
        onChange={(e) => setEmail(e.target.value)}
        placeholder="you@company.com"
        autoComplete="email"
        required
      />
      <Input
        label="Password"
        type="password"
        value={password}
        onChange={(e) => setPassword(e.target.value)}
        placeholder="••••••••"
        autoComplete="current-password"
        required
      />

      {error && (
        <div className="px-3 py-2.5 bg-error-light border border-error/20 rounded text-sm text-error">
          {error}
        </div>
      )}

      <Button type="submit" className="w-full" size="lg" isLoading={isLoading}>
        Sign in
      </Button>

      <p className="text-sm text-gray-500 text-center">
        Don&apos;t have an account?{" "}
        <Link
          href="/signup"
          className="text-brand-600 hover:text-brand-700 font-medium"
        >
          Sign up
        </Link>
      </p>
    </form>
  );
}
