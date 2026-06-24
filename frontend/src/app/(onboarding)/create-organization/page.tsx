// src/app/(onboarding)/create-organization/page.tsx
"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { ArrowLeft } from "lucide-react";
import gsap from "gsap";

import {
  createOrganization,
  switchOrganization,
  getMyMembership,
} from "@/lib/org";
import { setToken } from "@/lib/token";
import { useAuthStore } from "@/stores/authStore";
import { usePermissionStore } from "@/stores/permissionStore";

const PURPLE = "#7c3aed";
const PURPLE_HOVER = "#a855f7";
const FONT_SYNE = "var(--font-syne, Syne, sans-serif)";
const FONT_INTER = "var(--font-inter, Inter, sans-serif)";

function nameToSlug(name: string): string {
  return name
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9\s-]/g, "")
    .replace(/\s+/g, "-")
    .replace(/-+/g, "-")
    .replace(/^-|-$/g, "")
    .slice(0, 50);
}

const schema = z.object({
  name: z
    .string()
    .min(2, "At least 2 characters")
    .max(100, "Under 100 characters"),
  slug: z
    .string()
    .min(2, "At least 2 characters")
    .max(50, "Under 50 characters")
    .regex(/^[a-z0-9-]+$/, "Lowercase letters, numbers, hyphens only"),
  type: z.string().optional(),
});
type FormValues = z.infer<typeof schema>;

function iStyle(err: boolean): React.CSSProperties {
  return {
    width: "100%",
    background: "#161616",
    border: `1px solid ${err ? "rgba(239,68,68,0.45)" : "rgba(255,255,255,0.08)"}`,
    borderRadius: "8px",
    padding: "0.75rem 1rem",
    fontSize: "0.875rem",
    color: "white",
    outline: "none",
    transition: "border-color 150ms ease, box-shadow 150ms ease",
    fontFamily: FONT_INTER,
  };
}

function iFocus(e: React.FocusEvent<HTMLInputElement | HTMLSelectElement>) {
  e.currentTarget.style.borderColor = PURPLE;
  e.currentTarget.style.boxShadow = "0 0 0 3px rgba(124,58,237,0.14)";
}

function iBlur(
  e: React.FocusEvent<HTMLInputElement | HTMLSelectElement>,
  err: boolean,
) {
  e.currentTarget.style.borderColor = err
    ? "rgba(239,68,68,0.45)"
    : "rgba(255,255,255,0.08)";
  e.currentTarget.style.boxShadow = "none";
}

const ORG_TYPES = [
  { value: "", label: "Select type (optional)" },
  { value: "business", label: "Business" },
  { value: "startup", label: "Startup" },
  { value: "agency", label: "Freelance / Agency" },
  { value: "nonprofit", label: "Non-profit" },
  { value: "personal", label: "Personal project" },
];

export default function CreateOrganizationPage() {
  const router = useRouter();
  const { setOrg } = useAuthStore();
  const { setPermissions } = usePermissionStore();
  const [serverError, setServerError] = useState<string | null>(null);
  const [slugEdited, setSlugEdited] = useState(false);

  const wrapRef = useRef<HTMLDivElement>(null);
  const cardRef = useRef<HTMLDivElement>(null);

  const {
    register,
    handleSubmit,
    setValue,
    watch,
    formState: { errors, isSubmitting },
  } = useForm<FormValues>({ resolver: zodResolver(schema) });

  const nameVal = watch("name", "");

  // Auto-generate slug from name (unless user edited slug manually)
  useEffect(() => {
    if (!slugEdited && nameVal) {
      setValue("slug", nameToSlug(nameVal), { shouldValidate: false });
    }
  }, [nameVal, slugEdited, setValue]);

  useEffect(() => {
    const ctx = gsap.context(() => {
      gsap.set(cardRef.current, { opacity: 0, y: 20 });
      gsap.to(cardRef.current, {
        opacity: 1,
        y: 0,
        duration: 0.5,
        ease: "power3.out",
        delay: 0.05,
      });
    }, wrapRef);
    return () => ctx.revert();
  }, []);

  const onSubmit = async (values: FormValues) => {
    setServerError(null);
    try {
      // 1. Create org
      const org = await createOrganization({
        name: values.name,
        slug: values.slug,
        type: values.type || undefined,
      });

      // 2. Switch to it → JWT gets business_id
      const switchData = await switchOrganization(org.id);
      setToken(switchData.access_token);

      // 3. Fetch permissions
      const membership = await getMyMembership();
      setOrg(org, membership);
      setPermissions(membership.permissions);

      // 4. Enter workspace
      router.push(`/${org.id}`);
    } catch (err: unknown) {
      const apiErr = (
        err as {
          response?: { data?: { error?: { code?: string; message?: string } } };
        }
      )?.response?.data?.error;
      if (apiErr?.code === "SLUG_TAKEN") {
        setServerError("That slug is already taken. Please choose another.");
      } else {
        setServerError(
          apiErr?.message ?? "Failed to create workspace. Please try again.",
        );
      }
    }
  };

  return (
    <div ref={wrapRef} className="min-h-screen py-14 px-4">
      <div className="max-w-[460px] mx-auto">
        {/* ── Back ──────────────────────────── */}
        <Link
          href="/select-organization"
          className="inline-flex items-center gap-2 text-sm mb-10 transition-colors"
          style={{ color: "#555", fontFamily: FONT_INTER }}
          onMouseEnter={(e) => (e.currentTarget.style.color = "#999")}
          onMouseLeave={(e) => (e.currentTarget.style.color = "#555")}
        >
          <ArrowLeft size={13} />
          Back to workspaces
        </Link>

        {/* ── Heading ───────────────────────── */}
        <h1
          className="font-bold text-white mb-1.5"
          style={{
            fontFamily: FONT_SYNE,
            fontSize: "1.75rem",
            letterSpacing: "-0.025em",
            lineHeight: 1.2,
          }}
        >
          Create a workspace
        </h1>
        <p
          className="text-sm mb-8"
          style={{ color: "#666", fontFamily: FONT_INTER }}
        >
          Set up your team's home in BusinessSAAS
        </p>

        {/* ── Card ──────────────────────────── */}
        <div
          ref={cardRef}
          className="rounded-xl"
          style={{
            background: "#0f0f0f",
            border: "1px solid rgba(255,255,255,0.07)",
            boxShadow: "0 20px 40px rgba(0,0,0,0.4)",
          }}
        >
          <form
            onSubmit={handleSubmit(onSubmit)}
            className="p-8 space-y-5"
            noValidate
          >
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

            {/* Name */}
            <div className="space-y-1.5">
              <label
                className="block text-sm font-medium"
                style={{ color: "#d0d0d0", fontFamily: FONT_INTER }}
              >
                Workspace name <span style={{ color: "#f87171" }}>*</span>
              </label>
              <input
                type="text"
                placeholder="Acme Inc."
                autoFocus
                {...register("name")}
                style={iStyle(!!errors.name)}
                onFocus={iFocus}
                onBlur={(e) => iBlur(e, !!errors.name)}
              />
              {errors.name && (
                <p
                  className="text-xs"
                  style={{ color: "#f87171", fontFamily: FONT_INTER }}
                >
                  {errors.name.message}
                </p>
              )}
            </div>

            {/* Slug */}
            <div className="space-y-1.5">
              <label
                className="block text-sm font-medium"
                style={{ color: "#d0d0d0", fontFamily: FONT_INTER }}
              >
                Slug <span style={{ color: "#f87171" }}>*</span>
              </label>
              <div className="relative">
                <span
                  className="absolute left-3.5 top-1/2 -translate-y-1/2 text-xs select-none pointer-events-none"
                  style={{ color: "#3a3a3a", fontFamily: FONT_INTER }}
                >
                  app /
                </span>
                <input
                  type="text"
                  placeholder="acme-inc"
                  {...register("slug")}
                  style={{ ...iStyle(!!errors.slug), paddingLeft: "3rem" }}
                  onFocus={iFocus}
                  onBlur={(e) => iBlur(e, !!errors.slug)}
                  onChange={(e) => {
                    setSlugEdited(true);
                    register("slug").onChange(e);
                  }}
                />
              </div>
              {errors.slug ? (
                <p
                  className="text-xs"
                  style={{ color: "#f87171", fontFamily: FONT_INTER }}
                >
                  {errors.slug.message}
                </p>
              ) : (
                <p
                  className="text-xs"
                  style={{ color: "#444", fontFamily: FONT_INTER }}
                >
                  Lowercase letters, numbers, and hyphens only
                </p>
              )}
            </div>

            {/* Type */}
            <div className="space-y-1.5">
              <label
                className="block text-sm font-medium"
                style={{ color: "#d0d0d0", fontFamily: FONT_INTER }}
              >
                Type
                <span
                  className="ml-2 text-xs font-normal"
                  style={{ color: "#444" }}
                >
                  optional
                </span>
              </label>
              <select
                {...register("type")}
                style={{
                  ...iStyle(false),
                  appearance: "none",
                  cursor: "pointer",
                }}
                onFocus={iFocus}
                onBlur={(e) => iBlur(e, false)}
              >
                {ORG_TYPES.map((t) => (
                  <option
                    key={t.value}
                    value={t.value}
                    style={{ background: "#1a1a1a", color: "#ccc" }}
                  >
                    {t.label}
                  </option>
                ))}
              </select>
            </div>

            {/* Submit */}
            <button
              type="submit"
              disabled={isSubmitting}
              className="w-full rounded-lg py-3 text-sm font-semibold text-white"
              style={{
                marginTop: "0.5rem",
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
                  Creating workspace…
                </span>
              ) : (
                "Create workspace"
              )}
            </button>
          </form>
        </div>
      </div>
    </div>
  );
}
