// src/app/(auth)/signup/page.tsx
"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Eye, EyeOff } from "lucide-react";
import gsap from "gsap";
import { toast } from "sonner";

import { signup, login, silentRefresh, getMe } from "@/lib/auth";
import { getToken, setToken } from "@/lib/token";
import { useAuthStore } from "@/stores/authStore";

// ── Constants ─────────────────────────────────────────────────────────
const PURPLE = "#7c3aed";
const PURPLE_HOVER = "#a855f7";
const FONT_SYNE = "var(--font-syne, Syne, sans-serif)";
const FONT_INTER = "var(--font-inter, Inter, sans-serif)";

// ── Validation ────────────────────────────────────────────────────────
// Mirrors backend validateSignupRequest (internal/auth/handler.go):
// email required (≤255 chars), password 8–72 chars (bcrypt's hard limit).
// firstName/lastName aren't enforced server-side but are required here
// since the signup form collects them for the profile.
const signupSchema = z.object({
  firstName: z
    .string()
    .min(1, "First name is required")
    .max(100, "First name is too long"),
  lastName: z
    .string()
    .min(1, "Last name is required")
    .max(100, "Last name is too long"),
  email: z
    .string()
    .min(1, "Email is required")
    .email("Enter a valid email address")
    .max(255, "Email is too long"),
  password: z
    .string()
    .min(8, "Password must be at least 8 characters")
    .max(72, "Password must not exceed 72 characters"),
});
type SignupValues = z.infer<typeof signupSchema>;

// ── Input style helpers (matches login/page.tsx) ────────────────────
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
export default function SignupPage() {
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
  } = useForm<SignupValues>({ resolver: zodResolver(signupSchema) });

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
  const onSubmit = async (values: SignupValues) => {
    setServerError(null);
    const email = values.email.trim().toLowerCase();

    // Step 1 — create the account
    try {
      await signup({
        email,
        password: values.password,
        first_name: values.firstName.trim(),
        last_name: values.lastName.trim(),
      });
    } catch (err: unknown) {
      const apiErr = err as {
        response?: { data?: { error?: { code?: string; message?: string } } };
      };
      const code = apiErr?.response?.data?.error?.code;
      const msg =
        code === "EMAIL_ALREADY_EXISTS"
          ? "An account with this email already exists."
          : (apiErr?.response?.data?.error?.message ??
            "Something went wrong. Please try again.");
      setServerError(msg);
      return;
    }

    // Step 2 — signup doesn't issue tokens (backend returns only the
    // user), so log in immediately with the same credentials to bootstrap
    // the session exactly like the login page does. If this step fails
    // for any reason, the account still exists — send them to /login.
    try {
      const tokenData = await login(email, values.password);
      setToken(tokenData.access_token);
      const user = await getMe();
      setUser(user);
      router.replace("/select-organization");
    } catch {
      toast.success("Account created. Please sign in to continue.");
      router.replace("/login");
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
        Create your account
      </h1>
      <p
        ref={subRef}
        className="text-center text-sm mb-8"
        style={{ color: "#888", fontFamily: FONT_INTER }}
      >
        Start building your workspace
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

          {/* First / Last name */}
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <label
                htmlFor="firstName"
                className="block text-sm font-medium"
                style={{ color: "#d0d0d0", fontFamily: FONT_INTER }}
              >
                First name
              </label>
              <input
                id="firstName"
                type="text"
                autoComplete="given-name"
                placeholder="Ada"
                {...register("firstName")}
                style={inputStyle(!!errors.firstName)}
                onFocus={onFocus}
                onBlur={(e) => onBlur(e, !!errors.firstName)}
              />
              {errors.firstName && (
                <p
                  className="text-xs"
                  style={{ color: "#f87171", fontFamily: FONT_INTER }}
                >
                  {errors.firstName.message}
                </p>
              )}
            </div>

            <div className="space-y-1.5">
              <label
                htmlFor="lastName"
                className="block text-sm font-medium"
                style={{ color: "#d0d0d0", fontFamily: FONT_INTER }}
              >
                Last name
              </label>
              <input
                id="lastName"
                type="text"
                autoComplete="family-name"
                placeholder="Lovelace"
                {...register("lastName")}
                style={inputStyle(!!errors.lastName)}
                onFocus={onFocus}
                onBlur={(e) => onBlur(e, !!errors.lastName)}
              />
              {errors.lastName && (
                <p
                  className="text-xs"
                  style={{ color: "#f87171", fontFamily: FONT_INTER }}
                >
                  {errors.lastName.message}
                </p>
              )}
            </div>
          </div>

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
            <label
              htmlFor="password"
              className="block text-sm font-medium"
              style={{ color: "#d0d0d0", fontFamily: FONT_INTER }}
            >
              Password
            </label>
            <div className="relative">
              <input
                id="password"
                type={showPassword ? "text" : "password"}
                autoComplete="new-password"
                placeholder="At least 8 characters"
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
                Creating account…
              </span>
            ) : (
              "Create account"
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
        Already have an account?{" "}
        <Link
          href="/login"
          className="font-medium"
          style={{ color: PURPLE_HOVER }}
          onMouseEnter={(e) => (e.currentTarget.style.color = "#c084fc")}
          onMouseLeave={(e) => (e.currentTarget.style.color = PURPLE_HOVER)}
        >
          Sign in
        </Link>
      </div>
    </div>
  );
}
