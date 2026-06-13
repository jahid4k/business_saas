"use client";

import { useState } from "react";
import Link from "next/link";
import { useAuth } from "@/hooks/useAuth";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";

export function SignupForm() {
  const { signup, isLoading, error } = useAuth();
  const [form, setForm] = useState({
    first_name: "jahid",
    last_name: "mridha",
    email: "jahid4k@gmail.com",
    password: "@@VapoRub2026@@",
  });

  const set =
    (field: keyof typeof form) => (e: React.ChangeEvent<HTMLInputElement>) =>
      setForm((f) => ({ ...f, [field]: e.target.value }));

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    await signup({ ...form, email: form.email.trim().toLowerCase() });
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div className="grid grid-cols-2 gap-3">
        <Input
          label="First name"
          type="text"
          value={form.first_name}
          onChange={set("first_name")}
          placeholder="Ada"
          required
        />
        <Input
          label="Last name"
          type="text"
          value={form.last_name}
          onChange={set("last_name")}
          placeholder="Lovelace"
          required
        />
      </div>
      <Input
        label="Email address"
        type="email"
        value={form.email}
        onChange={set("email")}
        placeholder="you@company.com"
        autoComplete="email"
        required
      />
      <Input
        label="Password"
        type="password"
        value={form.password}
        onChange={set("password")}
        placeholder="Minimum 8 characters"
        autoComplete="new-password"
        minLength={8}
        required
        hint="At least 8 characters"
      />

      {error && (
        <div className="px-3 py-2.5 bg-error-light border border-error/20 rounded text-sm text-error">
          {error}
        </div>
      )}

      <Button type="submit" className="w-full" size="lg" isLoading={isLoading}>
        Create account
      </Button>

      <p className="text-sm text-gray-500 text-center">
        Already have an account?{" "}
        <Link
          href="/login"
          className="text-brand-600 hover:text-brand-700 font-medium"
        >
          Sign in
        </Link>
      </p>
    </form>
  );
}
