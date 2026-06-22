import type { Metadata } from "next";
import Link from "next/link";
import { ResetPasswordForm } from "@/features/auth/ResetPasswordForm";

export const metadata: Metadata = {
  title: "Reset password",
};

export default function ResetPasswordPage() {
  return (
    <div className="rounded-xl border bg-card p-8 shadow-sm">
      <div className="mb-6">
        <h1 className="text-xl font-semibold tracking-tight">Reset password</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Enter your email and we&apos;ll send you a reset link
        </p>
      </div>

      <ResetPasswordForm />

      <p className="mt-6 text-center text-sm text-muted-foreground">
        Remember your password?{" "}
        <Link
          href="/login"
          className="font-medium text-primary hover:underline"
        >
          Log in
        </Link>
      </p>
    </div>
  );
}
