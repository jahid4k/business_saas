// src/app/(auth)/layout.tsx
import type { Metadata } from "next";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";

export const metadata: Metadata = {
  title: "BusinessSAAS",
  description: "Sign in to your workspace",
};

export default function AuthLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div
      className="min-h-screen flex items-center justify-center relative overflow-hidden"
      style={{ background: "#0a0a0a" }}
    >
      {/* Subtle grid */}
      <div
        aria-hidden="true"
        className="fixed inset-0 pointer-events-none select-none"
        style={{
          backgroundImage: `
            linear-gradient(rgba(255,255,255,0.025) 1px, transparent 1px),
            linear-gradient(90deg, rgba(255,255,255,0.025) 1px, transparent 1px)
          `,
          backgroundSize: "56px 56px",
        }}
      />

      {/* Purple top glow */}
      <div
        aria-hidden="true"
        className="fixed pointer-events-none"
        style={{
          top: 0,
          left: "50%",
          transform: "translateX(-50%)",
          width: "100%",
          maxWidth: "900px",
          height: "520px",
          background:
            "radial-gradient(ellipse at 50% 0%, rgba(124,58,237,0.22) 0%, transparent 68%)",
        }}
      />

      {/* Back to Home Button */}
      <Link
        href="/"
        className="absolute top-6 left-6 sm:top-8 sm:left-8 z-20 flex items-center gap-2 text-sm font-medium text-[#888] hover:text-white transition-colors"
        style={{ fontFamily: "var(--font-inter, Inter, sans-serif)" }}
      >
        <ArrowLeft size={16} />
        Back to home
      </Link>

      <div className="relative z-10 w-full">{children}</div>
    </div>
  );
}
