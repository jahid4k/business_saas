// src/app/(auth)/login/page.tsx
"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Eye, EyeOff } from "lucide-react";
import gsap from "gsap";

import { login, silentRefresh, getMe } from "@/lib/auth";
import { getToken, setToken } from "@/lib/token";
import { useAuthStore } from "@/stores/authStore";

// ── Constants ─────────────────────────────────────────────────────────
const PURPLE = "#7c3aed";
const PURPLE_HOVER = "#a855f7";
const FONT_SYNE = "var(--font-syne, Syne, sans-serif)";
const FONT_INTER = "var(--font-inter, Inter, sans-serif)";

// ── Validation ────────────────────────────────────────────────────────
const loginSchema = z.object({
  email: z
    .string()
    .min(1, "Email is required")
    .email("Enter a valid email address"),
  password: z.string().min(1, "Password is required"),
});
type LoginValues = z.infer<typeof loginSchema>;

// ── Input style helpers ───────────────────────────────────────────────
function inputStyle(hasError: boolean): React.CSSProperties {
  return {
    width: "100%",
    background: "#161616",
    border: `1px solid ${hasError ? "rgba(239,68,68,0.45)" : "rgba(255,255,255,0.08)"}`,
    borderRadius: "8px",
    padding: "0.75rem 1rem",
    fontSize: "0.875rem",
    color: "white",
    outline: "none",
    transition: "border-color 150ms ease, box-shadow 150ms ease",
    fontFamily: FONT_INTER,
  };
}

function onFocus(e: React.FocusEvent<HTMLInputElement>) {
  e.currentTarget.style.borderColor = PURPLE;
  e.currentTarget.style.boxShadow = "0 0 0 3px rgba(124,58,237,0.14)";
}

function onBlur(e: React.FocusEvent<HTMLInputElement>, hasError: boolean) {
  e.currentTarget.style.borderColor = hasError
    ? "rgba(239,68,68,0.45)"
    : "rgba(255,255,255,0.08)";
  e.currentTarget.style.boxShadow = "none";
}

// ── Page ──────────────────────────────────────────────────────────────
export default function LoginPage() {
  const router = useRouter();
  const { setUser } = useAuthStore();

  const [showPassword, setShowPassword] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);
  // Shows spinner while we check if the user already has a valid session
  const [checkingSession, setCheckingSession] = useState(true);

  // GSAP refs
  const wrapRef = useRef<HTMLDivElement>(null);
  const logoRef = useRef<HTMLDivElement>(null);
  const headRef = useRef<HTMLHeadingElement>(null);
  const subRef = useRef<HTMLParagraphElement>(null);
  const cardRef = useRef<HTMLDivElement>(null);
  const footRef = useRef<HTMLDivElement>(null);

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<LoginValues>({ resolver: zodResolver(loginSchema) });

  // ── Session check on mount ────────────────────────────────────────
  // If a valid refresh cookie exists → redirect away, never show the form
  useEffect(() => {
    const check = async () => {
      if (getToken()) {
        router.replace("/select-organization");
        return;
      }
      try {
        const token = await silentRefresh();
        setToken(token);
        const user = await getMe();
        setUser(user);
        router.replace("/select-organization");
      } catch {
        // No valid session — show the form
        setCheckingSession(false);
      }
    };
    check();
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  // ── GSAP entrance — only after session check resolves ────────────
  useEffect(() => {
    if (checkingSession) return;

    const targets = [
      logoRef.current,
      headRef.current,
      subRef.current,
      cardRef.current,
      footRef.current,
    ];

    const ctx = gsap.context(() => {
      gsap.set(targets, { opacity: 0, y: 22 });
      gsap.to(targets, {
        opacity: 1,
        y: 0,
        duration: 0.6,
        stagger: 0.08,
        ease: "power3.out",
      });
    }, wrapRef);

    return () => ctx.revert();
  }, [checkingSession]);

  // ── Submit ────────────────────────────────────────────────────────
  const onSubmit = async (values: LoginValues) => {
    setServerError(null);
    try {
      const tokenData = await login(values.email, values.password);
      setToken(tokenData.access_token);
      const user = await getMe();
      setUser(user);
      router.replace("/select-organization");
    } catch (err: unknown) {
      const msg =
        (
          err as {
            response?: { data?: { error?: { message?: string } } };
          }
        )?.response?.data?.error?.message ?? "Invalid email or password";
      setServerError(msg);
    }
  };

  // ── Loading state while checking session ─────────────────────────
  if (checkingSession) {
    return (
      <div
        className="flex items-center justify-center"
        style={{ minHeight: "100vh", background: "#0a0a0a" }}
      >
        <span
          className="w-5 h-5 rounded-full animate-spin block"
          style={{
            border: "2px solid rgba(124,58,237,0.2)",
            borderTopColor: PURPLE,
          }}
        />
      </div>
    );
  }

  // ── Form ──────────────────────────────────────────────────────────
  return (
    <div ref={wrapRef} className="w-full max-w-[420px] mx-auto px-5 py-12">
      {/* ── Logo ─────────────────────────────────── */}
      <div ref={logoRef} className="flex justify-center mb-10">
        <div className="flex items-center gap-2.5">
          <div
            className="shrink-0 flex items-center justify-center rounded-lg"
            style={{
              width: 36,
              height: 36,
              background: "linear-gradient(135deg, #7c3aed 0%, #a855f7 100%)",
            }}
          >
            <svg
              width="18"
              height="18"
              viewBox="0 0 18 18"
              fill="none"
              aria-hidden="true"
            >
              <rect x="2" y="2" width="6" height="6" rx="1.5" fill="white" />
              <rect
                x="10"
                y="2"
                width="6"
                height="6"
                rx="1.5"
                fill="white"
                fillOpacity="0.5"
              />
              <rect
                x="2"
                y="10"
                width="6"
                height="6"
                rx="1.5"
                fill="white"
                fillOpacity="0.5"
              />
              <rect x="10" y="10" width="6" height="6" rx="1.5" fill="white" />
            </svg>
          </div>
          <span
            className="text-xl font-bold text-white"
            style={{ fontFamily: FONT_SYNE, letterSpacing: "-0.02em" }}
          >
            BusinessSAAS
          </span>
        </div>
      </div>

      {/* ── Heading ──────────────────────────────── */}
      <h1
        ref={headRef}
        className="text-center font-bold text-white mb-2"
        style={{
          fontFamily: FONT_SYNE,
          fontSize: "2rem",
          lineHeight: 1.2,
          letterSpacing: "-0.025em",
        }}
      >
        Welcome back
      </h1>
      <p
        ref={subRef}
        className="text-center text-sm mb-8"
        style={{ color: "#888", fontFamily: FONT_INTER }}
      >
        Sign in to continue to your workspace
      </p>

      {/* ── Card ─────────────────────────────────── */}
      <div
        ref={cardRef}
        className="rounded-xl"
        style={{
          background: "#0f0f0f",
          border: "1px solid rgba(255,255,255,0.07)",
          boxShadow:
            "0 0 0 1px rgba(255,255,255,0.02), 0 24px 48px rgba(0,0,0,0.55)",
        }}
      >
        <form
          onSubmit={handleSubmit(onSubmit)}
          className="p-8 space-y-5"
          noValidate
        >
          {/* Server error banner */}
          {serverError && (
            <div
              className="rounded-lg px-4 py-3 text-sm"
              style={{
                background: "rgba(239,68,68,0.07)",
                border: "1px solid rgba(239,68,68,0.2)",
                color: "#f87171",
                fontFamily: FONT_INTER,
              }}
            >
              {serverError}
            </div>
          )}

          {/* Email */}
          <div className="space-y-1.5">
            <label
              htmlFor="email"
              className="block text-sm font-medium"
              style={{ color: "#d0d0d0", fontFamily: FONT_INTER }}
            >
              Email address
            </label>
            <input
              id="email"
              type="email"
              autoComplete="email"
              placeholder="you@company.com"
              {...register("email")}
              style={inputStyle(!!errors.email)}
              onFocus={onFocus}
              onBlur={(e) => onBlur(e, !!errors.email)}
            />
            {errors.email && (
              <p
                className="text-xs"
                style={{ color: "#f87171", fontFamily: FONT_INTER }}
              >
                {errors.email.message}
              </p>
            )}
          </div>

          {/* Password */}
          <div className="space-y-1.5">
            <div className="flex items-center justify-between">
              <label
                htmlFor="password"
                className="block text-sm font-medium"
                style={{ color: "#d0d0d0", fontFamily: FONT_INTER }}
              >
                Password
              </label>
              <Link
                href="/forgot-password"
                className="text-xs transition-colors"
                style={{ color: PURPLE, fontFamily: FONT_INTER }}
                onMouseEnter={(e) =>
                  (e.currentTarget.style.color = PURPLE_HOVER)
                }
                onMouseLeave={(e) => (e.currentTarget.style.color = PURPLE)}
              >
                Forgot password?
              </Link>
            </div>
            <div className="relative">
              <input
                id="password"
                type={showPassword ? "text" : "password"}
                autoComplete="current-password"
                placeholder="••••••••"
                {...register("password")}
                style={{
                  ...inputStyle(!!errors.password),
                  paddingRight: "2.75rem",
                }}
                onFocus={onFocus}
                onBlur={(e) => onBlur(e, !!errors.password)}
              />
              <button
                type="button"
                onClick={() => setShowPassword((v) => !v)}
                className="absolute right-3.5 top-1/2 -translate-y-1/2 p-1 rounded transition-colors"
                style={{ color: "#555" }}
                tabIndex={-1}
                aria-label={showPassword ? "Hide password" : "Show password"}
                onMouseEnter={(e) => (e.currentTarget.style.color = "#aaa")}
                onMouseLeave={(e) => (e.currentTarget.style.color = "#555")}
              >
                {showPassword ? (
                  <EyeOff size={15} strokeWidth={1.75} />
                ) : (
                  <Eye size={15} strokeWidth={1.75} />
                )}
              </button>
            </div>
            {errors.password && (
              <p
                className="text-xs"
                style={{ color: "#f87171", fontFamily: FONT_INTER }}
              >
                {errors.password.message}
              </p>
            )}
          </div>

          {/* Submit */}
          <button
            type="submit"
            disabled={isSubmitting}
            className="w-full rounded-lg py-3 text-sm font-semibold text-white"
            style={{
              marginTop: "0.25rem",
              background: PURPLE,
              cursor: isSubmitting ? "not-allowed" : "pointer",
              opacity: isSubmitting ? 0.72 : 1,
              transition: "background 150ms ease, opacity 150ms ease",
              fontFamily: FONT_INTER,
            }}
            onMouseEnter={(e) => {
              if (!isSubmitting)
                e.currentTarget.style.background = PURPLE_HOVER;
            }}
            onMouseLeave={(e) => {
              if (!isSubmitting) e.currentTarget.style.background = PURPLE;
            }}
          >
            {isSubmitting ? (
              <span className="flex items-center justify-center gap-2.5">
                <span
                  className="w-4 h-4 rounded-full animate-spin inline-block"
                  style={{
                    border: "2px solid rgba(255,255,255,0.25)",
                    borderTopColor: "white",
                  }}
                />
                Signing in…
              </span>
            ) : (
              "Sign in"
            )}
          </button>
        </form>
      </div>

      {/* ── Footer ───────────────────────────────── */}
      <div
        ref={footRef}
        className="text-center mt-6 text-sm"
        style={{ color: "#666", fontFamily: FONT_INTER }}
      >
        New to BusinessSAAS?{" "}
        <Link
          href="/signup"
          className="font-medium"
          style={{ color: PURPLE_HOVER }}
          onMouseEnter={(e) => (e.currentTarget.style.color = "#c084fc")}
          onMouseLeave={(e) => (e.currentTarget.style.color = PURPLE_HOVER)}
        >
          Create an account
        </Link>
      </div>
    </div>
  );
}
