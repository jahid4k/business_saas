// lib/auth.ts
import NextAuth from "next-auth";
import type { NextAuthConfig, DefaultSession } from "next-auth";
import Credentials from "next-auth/providers/credentials";

// ----------------------------------------------------------
// Type extensions — next-auth v5 beta style
// ----------------------------------------------------------

declare module "next-auth" {
  interface Session {
    user: {
      id: string;
      email: string;
      name: string;
      accessToken: string;
      activeOrgId: string | null;
      activeOrgSlug: string | null;
      activeRole: string | null;
    } & DefaultSession["user"];
    error?: "RefreshFailed";
  }

  interface User {
    id: string;
    email: string;
    name: string;
    accessToken: string;
    activeOrgId: string | null;
    activeOrgSlug: string | null;
    activeRole: string | null;
  }
}

// NOTE: next-auth v5 beta এ "next-auth/jwt" augmentation কাজ করে না reliably।
// JWT type আমরা internal interface দিয়ে handle করবো।

// ----------------------------------------------------------
// Internal JWT shape — type augmentation ছাড়া
// ----------------------------------------------------------

interface AppJWT {
  accessToken?: string;
  userId?: string;
  activeOrgId?: string | null;
  activeOrgSlug?: string | null;
  activeRole?: string | null;
  error?: "RefreshFailed";
  // next-auth built-in fields
  sub?: string;
  name?: string;
  email?: string;
  picture?: string;
  iat?: number;
  exp?: number;
  jti?: string;
}

// ----------------------------------------------------------
// Auth config
// ----------------------------------------------------------

const backendUrl = process.env.BACKEND_INTERNAL_URL || "http://localhost:8080";

export const authConfig: NextAuthConfig = {
  providers: [
    Credentials({
      id: "credentials",
      name: "Email and password",
      credentials: {
        email: { label: "Email", type: "email" },
        password: { label: "Password", type: "password" },
      },
      async authorize(credentials) {
        if (!credentials?.email || !credentials?.password) return null;

        try {
          const res = await fetch(`${backendUrl}/api/v1/auth/login`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
              email: credentials.email,
              password: credentials.password,
            }),
          });

          if (!res.ok) return null;

          const body = await res.json();
          const data = body?.data;

          if (!data?.access_token || !data?.user) return null;

          return {
            id: String(data.user.id),
            email: String(data.user.email),
            name:
              (data.user.name ??
                `${data.user.first_name ?? ""} ${data.user.last_name ?? ""}`.trim()) ||
              String(data.user.email),
            accessToken: String(data.access_token),
            activeOrgId: null,
            activeOrgSlug: null,
            activeRole: null,
          };
        } catch (err) {
          console.error("[auth] Login failed:", err);
          return null;
        }
      },
    }),
  ],

  pages: {
    signIn: "/login",
    error: "/login",
  },

  session: {
    strategy: "jwt",
    maxAge: 7 * 24 * 60 * 60,
  },

  callbacks: {
    // token এর type কে AppJWT হিসেবে cast করো
    async jwt({ token, user, trigger, session: updateSession }) {
      const t = token as AppJWT;

      // Initial sign-in — user object present
      if (user) {
        t.userId = user.id;
        t.accessToken = user.accessToken;
        t.activeOrgId = user.activeOrgId;
        t.activeOrgSlug = user.activeOrgSlug;
        t.activeRole = user.activeRole;
      }

      // Org switch — frontend calls useSession().update(...)
      if (trigger === "update" && updateSession) {
        const upd = updateSession as Partial<AppJWT>;
        if (upd.accessToken) t.accessToken = upd.accessToken;
        if (upd.activeOrgId !== undefined) t.activeOrgId = upd.activeOrgId;
        if (upd.activeOrgSlug !== undefined)
          t.activeOrgSlug = upd.activeOrgSlug;
        if (upd.activeRole !== undefined) t.activeRole = upd.activeRole;
      }

      return t as typeof token;
    },

    async session({ session, token }) {
      const t = token as AppJWT;

      session.user.id = t.userId ?? "";
      session.user.accessToken = t.accessToken ?? "";
      session.user.activeOrgId = t.activeOrgId ?? null;
      session.user.activeOrgSlug = t.activeOrgSlug ?? null;
      session.user.activeRole = t.activeRole ?? null;

      if (t.error) {
        session.error = t.error;
      }

      return session;
    },
  },

  trustHost: true,
  secret: process.env.NEXTAUTH_SECRET,
};

// next-auth v5 TS2883 fix — separate const then export
const nextAuth = NextAuth(authConfig);
export const { handlers, auth, signIn, signOut } = nextAuth;
